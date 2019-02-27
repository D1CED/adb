package adb

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

// DeviceState represents one of the 3 possible states adb will report devices.
// A device can be communicated with when it's in StateOnline.
// A USB device will make the following state transitions:
//
//     Plugged in: StateDisconnected->StateOffline->StateOnline
//     Unplugged:  StateOnline->StateDisconnected
//
//go:generate stringer -type=DeviceState
type DeviceState uint8

const (
	StateInvalid DeviceState = iota
	StateUnauthorized
	StateDisconnected
	StateOffline
	StateOnline
)

func parseDeviceState(str string) DeviceState {
	switch str {
	case "":
		return StateDisconnected
	case "offline":
		return StateOffline
	case "device":
		return StateOnline
	case "unauthorized":
		return StateUnauthorized
	default:
		return StateInvalid
	}
}

// DeviceStateChangedEvent represents a device state transition.
// Contains the device’s old and new states, but also provides methods to
// query the type of state transition.
type DeviceStateChangedEvent struct {
	Serial   string
	OldState DeviceState
	NewState DeviceState
}

// CameOnline returns true if this event represents a device coming online.
func (e DeviceStateChangedEvent) CameOnline() bool {
	return e.OldState != StateOnline && e.NewState == StateOnline
}

// WentOffline returns true if this event represents a device going offline.
func (e DeviceStateChangedEvent) WentOffline() bool {
	return e.OldState == StateOnline && e.NewState != StateOnline
}

// DeviceWatcher publishes device status change events.
// If the server dies while listening for events, it restarts the server.
type DeviceWatcher struct {
	server *Server

	conn net.Conn

	// If an error occurs, it is stored here and eventChan is close immediately after.
	err atomic.Value

	eventChan chan DeviceStateChangedEvent

	cancel chan struct{}
}

func newDeviceWatcher(s *Server) (*DeviceWatcher, error) {
	conn, err := dial(s.address)
	if err != nil {
		return nil, err
	}
	_, err = conn.Write([]byte("host:track-devices"))
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := wantStatus(conn); err != nil {
		conn.Close()
		return nil, err
	}

	watcher := &DeviceWatcher{
		server:    s,
		eventChan: make(chan DeviceStateChangedEvent),
		cancel:    make(chan struct{}),
		conn:      conn,
	}
	// runtime.SetFinalizer(watcher, func(watcher *DeviceWatcher) { watcher.Shutdown() })
	go publishDevices(watcher)
	return watcher, nil
}

// C returns a channel than can be received on to get events.
// If an unrecoverable error occurs, or Shutdown is called, the channel will be closed.
func (w *DeviceWatcher) C() <-chan DeviceStateChangedEvent {
	return w.eventChan
}

// Err returns the error that caused the channel returned by C to be closed,
// if C is closed. If C is not closed, its return value is undefined.
func (w *DeviceWatcher) Err() error {
	err, _ := w.err.Load().(error)
	return err
}

// Shutdown stops the watcher from listening for events and closes the channel
// returned from C.
func (w *DeviceWatcher) Shutdown() {
	close(w.cancel)
}

// EDIT(jmh): Will recursively restart. Revisit this.
func publishDevices(watcher *DeviceWatcher) {
	defer close(watcher.eventChan)

	// pre do it for no async mess
	defer watcher.conn.Close()

	err := publishDevicesUntilError(watcher.conn, watcher.eventChan, watcher.cancel)

	if errors.Cause(err) == ErrConnectionReset {
		// The server died, restart and reconnect.

		// Delay by a random [0ms, 500ms) in case multiple
		// DeviceWatchers are trying to start the same server.
		delay := time.Duration(rand.Intn(500)) * time.Millisecond
		fmt.Printf("[DeviceWatcher] server died, restarting in %s…", delay)
		time.Sleep(delay)

		if err := start(watcher.server); err != nil {
			fmt.Println("[DeviceWatcher] error restarting server, giving up")
			watcher.err.Store(err)
			return
		}
		publishDevices(watcher)
	} else if err != nil {
		// Unknown error, don't retry.
		watcher.err.Store(err)
	}
}

func publishDevicesUntilError(r io.Reader, eventChan chan<- DeviceStateChangedEvent, cancel <-chan struct{}) error {
	lastState := make(map[string]DeviceState)
	devState := make([]serialDeviceState, 0, 8)
	for {
		select {
		case <-cancel:
			close(eventChan)
			return nil
		default:
		}
		msg, err := readBytes(r)
		if err != nil {
			return err
		}
		deviceStates := parseDeviceStates(bytes.NewReader(msg), devState)
		calculateStateDiffs(deviceStates, lastState, eventChan)
	}
}

type serialDeviceState struct {
	serial string
	state  DeviceState
}

// for even fewer allocations take sdss as pointer to slice so that enlargments
// propagate
func parseDeviceStates(r io.Reader, sdss []serialDeviceState) []serialDeviceState {
	sdss = sdss[0:0:cap(sdss)]
	scan := bufio.NewScanner(r)

	for scan.Scan() {
		line := scan.Text()

		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			continue
		}
		serial, stateString := fields[0], fields[1]
		state := parseDeviceState(stateString)
		sdss = append(sdss, serialDeviceState{serial, state})
	}
	return sdss
}

func calculateStateDiffs(
	lastState []serialDeviceState,
	deviceStates map[string]DeviceState,
	ch chan<- DeviceStateChangedEvent) {

	for _, sta := range lastState {
		if old := deviceStates[sta.serial]; sta.state != old {
			ch <- DeviceStateChangedEvent{
				Serial:   sta.serial,
				OldState: old,
				NewState: sta.state,
			}
			deviceStates[sta.serial] = sta.state
		}
	}
	// check opposite case
	// what fields are in map but not in slice
	// send event and delete from map
}
