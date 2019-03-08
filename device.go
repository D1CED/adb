package adb

import (
	"github.com/pkg/errors"
)

// Device communicates with a specific Android device.
// To get an instance, call Server.Device(serial).
type Device struct {
	server *Server
	serial string
}

// String returns the devices serial-number.
func (d *Device) String() string {
	// return d.descriptor.String()
	// s, _ := d.Serial()
	return d.serial
}

// getAttribute returns the message send by the server when requesting
// <host-prefix>:<attr>, where host-prefix is d.
func (d *Device) requestResponseString(attr string) ([]byte, error) {
	return requestResponseBytes(d.server.address, "host-serial:"+d.serial+":"+attr)
}

func (d *Device) send(attr string) error {
	return send(d.server.address, "host-serial:"+d.serial+":"+attr)
}

// get-product is documented, but not implemented, in the server.
// TODO(z): Make product exported if get-product is ever implemented in adb.
func (d *Device) product() (string, error) {
	attr, err := d.requestResponseString("get-product")
	return string(attr), errors.WithMessage(err, "Product")
}

// Serial returns the devices serial number.
// unnecessary?!
func (d *Device) Serial() (string, error) {
	attr, err := d.requestResponseString("get-serialno")
	return string(attr), errors.WithMessage(err, "Serial")
}

// DevicePath returns the current devices path.
func (d *Device) DevicePath() (string, error) {
	attr, err := d.requestResponseString("get-devpath")
	return string(attr), errors.WithMessage(err, "DevicePath")
}

func (d *Device) State() (DeviceState, error) {
	attr, err := d.requestResponseString("get-state")
	state := parseDeviceState(string(attr))
	return state, errors.WithMessage(err, "State")
}

// DeviceInfo queries the server for information on the current device.
func (d *Device) DeviceInfo() (DeviceInfo, error) {
	// adb doesn't actually provide a way to get this for an individual device,
	// so we have to just list devices and find ourselves.

	devices, err := d.server.ListDevices()
	if err != nil {
		return DeviceInfo{}, errors.Wrap(err, "Server.ListDevices")
	}

	for _, deviceInfo := range devices {
		if deviceInfo.Serial == d.serial {
			return deviceInfo, nil
		}
	}

	return DeviceInfo{}, errors.Errorf("device list doesn't contain serial %s", d.serial)
}

/*
remount : from the official adb command’s docs:
	Ask adbd to remount the device's filesystem in read-write mode,
	instead of read-only. This is usually necessary before performing
	an "adb sync" or "adb push" request.
	This request may not succeed on certain builds which do not allow
	that.
Source: https://android.googlesource.com/platform/system/core/+/master/adb/SERVICES.TXT
TODO(jmh): actually use this!
TODO(jmh): investigate respnse type
*/
func (d *Device) remount() error {
	_, err := d.requestResponseString("remount")
	return errors.WithMessage(err, "Remount")
}
