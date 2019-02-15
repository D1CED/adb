package adb

// DeviceState represents one of the 3 possible states adb will report devices.
// A device can be communicated with when it's in StateOnline.
// A USB device will make the following state transitions:
// 	Plugged in: StateDisconnected->StateOffline->StateOnline
// 	Unplugged:  StateOnline->StateDisconnected
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
	if state, ok := deviceStateStrings[str]; ok {
		return state
	} else {
		return StateInvalid
	}
}
