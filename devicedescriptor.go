package adb

import "fmt"

type DeviceDescriptor struct {
	descriptor uint8
	// Only used if Type is DeviceSerial.
	serial string
}

const (
	device = iota
	usbDevice
	localDevice
	serialDevice
)

var (
	// AnyDevice represents host:transport-any and host:<request>
	AnyDevice = DeviceDescriptor{device, ""}
	// AnyUSBDevice represents host:transport-usb and host-usb:<request>
	AnyUSBDevice = DeviceDescriptor{usbDevice, ""}
	// AnyLocalDevice represents host:transport-local and host-local:<request>
	AnyLocalDevice = DeviceDescriptor{localDevice, ""}
)

// AnyDeviceSerial represents host:transport:<serial> and host-serial:<serial>:<request>
func AnyDeviceSerial(serial string) DeviceDescriptor {
	return DeviceDescriptor{serialDevice, serial}
}

func (d DeviceDescriptor) String() string {
	switch d.descriptor {
	case device:
		return "Device"
	case usbDevice:
		return "DeviceUSB"
	case localDevice:
		return "DeviceLocal"
	case serialDevice:
		return fmt.Sprintf("DeviceSerial[%s]", d.serial)
	default:
		return "<invalid DeviceDescriptor>"
	}
}

func (d DeviceDescriptor) getHostPrefix() string {
	switch d.descriptor {
	case device:
		return "host"
	case usbDevice:
		return "host-usb"
	case localDevice:
		return "host-local"
	case serialDevice:
		return fmt.Sprintf("host-serial:%s", d.serial)
	default:
		return "<invalid DeviceDescriptor>"
	}
}

func (d DeviceDescriptor) getTransportDescriptor() string {
	switch d.descriptor {
	case device:
		return "transport-any"
	case usbDevice:
		return "transport-usb"
	case localDevice:
		return "transport-local"
	case serialDevice:
		return fmt.Sprintf("transport:%s", d.serial)
	default:
		return "<invalid DeviceDescriptor>"
	}
}
