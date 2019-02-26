package adb

import (
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
	var deviceStateStrings = map[string]DeviceState{
		"":             StateDisconnected,
		"offline":      StateOffline,
		"device":       StateOnline,
		"unauthorized": StateUnauthorized,
	}
	return deviceStateStrings[str]
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
func (s DeviceStateChangedEvent) CameOnline() bool {
	return s.OldState != StateOnline && s.NewState == StateOnline
}

// WentOffline returns true if this event represents a device going offline.
func (s DeviceStateChangedEvent) WentOffline() bool {
	return s.OldState == StateOnline && s.NewState != StateOnline
}

// DeviceWatcher publishes device status change events.
// If the server dies while listening for events, it restarts the server.
type DeviceWatcher struct {
	server *Server

	conn net.Conn

	// If an error occurs, it is stored here and eventChan is close immediately after.
	err atomic.Value

	eventChan chan DeviceStateChangedEvent
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
	if err, ok := w.err.Load().(error); ok {
		return err
	}
	return nil
}

// Shutdown stops the watcher from listening for events and closes the channel
// returned from C.
// TODO(z): Implement.
func (w *DeviceWatcher) Shutdown() {
}

/*
publishDevices reads device lists from scanner, calculates diffs, and publishes
events on eventChan.
Returns when scanner returns an error.
Doesn't refer directly to a *DeviceWatcher so it can be GCed (which will, in
turn, close Scanner and stop this goroutine).

TODO(z): to support shutdown, spawn a new goroutine each time a server connection
is established. This goroutine should read messages and send them to a
message channel. Can write errors directly to errVal. publishDevicesUntilError
should take the msg chan and the scanner and select on the msg chan and stop
chan, and if the stop chan sends, close the scanner and return true.
If the msg chan closes, just return false. publishDevices can look at ret val:
if false and err == EOF, reconnect. If false and other error, report err and
abort. If true, report no error and stop.

EDIT(jmh): Will recursively restart. Revisit this.
*/
func publishDevices(watcher *DeviceWatcher) {
	defer close(watcher.eventChan)

	// pre do it for no async mess
	defer watcher.conn.Close()

	err := publishDevicesUntilError(watcher.conn, watcher.eventChan)

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

func publishDevicesUntilError(r io.Reader, eventChan chan<- DeviceStateChangedEvent) error {
	lastState := make(map[string]DeviceState)
	for {
		msg, err := readBytes(r)
		if err != nil {
			return err
		}
		deviceStates, err := parseDeviceStates(string(msg))
		if err != nil {
			return err
		}
		for _, event := range calculateStateDiffs(lastState, deviceStates) {
			eventChan <- event
		}
		lastState = deviceStates
	}
}

func parseDeviceStates(msg string) (map[string]DeviceState, error) {
	// PERF(jmh): change this to slice, don't allocate map in loop!
	// maybe pass it in as argument.
	// use io.Reader instead of string for further optimizations.
	states := make(map[string]DeviceState)
	for lineNum, line := range strings.Split(msg, "\n") {
		if len(line) == 0 {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			return nil, errors.Errorf("invalid device state line %d: %s", lineNum, line)
		}
		serial, stateString := fields[0], fields[1]
		state := parseDeviceState(stateString)
		states[serial] = state
	}
	return states, nil
}

func calculateStateDiffs(oldStates, newStates map[string]DeviceState) []DeviceStateChangedEvent {
	events := make([]DeviceStateChangedEvent, 0, len(newStates))
	for serial, oldState := range oldStates {
		newState, ok := newStates[serial]

		if oldState != newState {
			if ok {
				// Device present in both lists: state changed.
				events = append(events, DeviceStateChangedEvent{serial, oldState, newState})
			} else {
				// Device only present in old list: device removed.
				events = append(events, DeviceStateChangedEvent{serial, oldState, StateDisconnected})
			}
		}
	}

	for serial, newState := range newStates {
		if _, ok := oldStates[serial]; !ok {
			// Device only present in new list: device added.
			events = append(events, DeviceStateChangedEvent{serial, StateDisconnected, newState})
		}
	}

	return events
}
