package adb

type Interface interface {
	Version() string
	Kill() error
	Devices()
	TrackDevices()
	Emulator(port int)
	Transport(method, serialNumber string)
	HostSerial(method, serialNumber string)
	SerialNumber()
	DevicePath()
	State()
	Forward()
	KillForward()
	ListForward()
}
