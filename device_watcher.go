package adb

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"

	"github.com/pkg/errors"
)

// DeviceState represents one of the 3 possible states adb will report devices.
// A device can be communicated with when it's in StateOnline.
// A USB device will make the following state transitions:
//
//     Plugged in: StateDisconnected->StateOffline->StateOnline
//     Unplugged:  StateOnline->StateDisconnected
//
type DeviceState uint8

//go:generate stringer -type=DeviceState

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
	server    *Server
	conn      net.Conn
	err       atomic.Value
	eventChan chan DeviceStateChangedEvent
	cancel    chan struct{}
}

// NewDeviceWatcher starts a new device watcher.
func (s *Server) NewDeviceWatcher() (*DeviceWatcher, error) {
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
		eventChan: make(chan DeviceStateChangedEvent, 1),
		cancel:    make(chan struct{}),
		conn:      conn,
	}
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

// Close stops the watcher from listening for events and closes the channel
// returned from C. Don't double close a DeviceWatcher.
func (w *DeviceWatcher) Close() error {
	w.cancel <- struct{}{}
	<-w.cancel
	err, _ := w.err.Load().(error)
	return err
}

func publishDevices(dw *DeviceWatcher) {
	defer func() {
		err := dw.conn.Close()
		if err != nil && dw.err.Load() == nil {
			dw.err.Store(err)
		}
	}()
	defer close(dw.cancel)

	lastState := make(map[string]DeviceState)
	devState := make([]serialDeviceState, 0, 8)
	for {
		select {
		case <-dw.cancel:
			close(dw.eventChan)
			return
		default:
		}
		msg, err := readBytes(dw.conn)
		if errors.Cause(err) == ErrConnectionReset {
			// The server died, restart and reconnect.
			err := start(dw.server)
			if err != nil {
				fmt.Println("[DeviceWatcher] error restarting server, giving up")
				dw.err.Store(err)
				return
			}
			continue
		} else if err != nil {
			// Unknown error, don't retry.
			dw.err.Store(err)
			close(dw.eventChan)
			return
		}
		deviceStates := parseDeviceStates(bytes.NewReader(msg), devState)
		calculateStateDiffs(deviceStates, lastState, dw.eventChan)
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

	var notContains = func(s string) bool {
		for _, t := range lastState {
			if t.serial == s {
				return false
			}
		}
		return true
	}

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

	for s := range deviceStates {
		if notContains(s) {
			ch <- DeviceStateChangedEvent{
				Serial:   s,
				OldState: deviceStates[s],
				NewState: StateDisconnected,
			}
			delete(deviceStates, s)
		}
	}
}
