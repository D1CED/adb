package adb

import (
	"bufio"
	"io"
	"strings"

	"github.com/pkg/errors"
)

// change this to not use maps!
// map allocation in loop bad!!!

type DeviceInfo struct {
	// Must be always set.
	Serial string
	// Product, device, and model are not set in the short form.
	Product string
	Model   string
	Device  string
	// Only set for devices connected via USB.
	USB string
}

func newDevice(serial string, attrs map[string]string) (DeviceInfo, error) {
	if serial == "" {
		return DeviceInfo{}, errors.Wrap(ErrAssertionViolation, "device serial cannot be blank")
	}
	return DeviceInfo{
		Serial:  serial,
		Product: attrs["product"],
		Model:   attrs["model"],
		Device:  attrs["device"],
		USB:     attrs["usb"],
	}, nil
}

// IsUSB returns true if the device is connected via USB.
// remove?
func (d DeviceInfo) IsUSB() bool {
	return d.USB != ""
}

func parseDeviceList(list io.Reader, lineParseFunc func(string) (DeviceInfo, error)) ([]DeviceInfo, error) {
	devices := make([]DeviceInfo, 0, 5)
	scanner := bufio.NewScanner(list)

	for scanner.Scan() {
		device, err := lineParseFunc(scanner.Text())
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func parseDeviceShort(line string) (DeviceInfo, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return DeviceInfo{}, errors.Errorf(
			"malformed device line, expected 2 fields but found %d", len(fields))
	}
	return newDevice(fields[0], map[string]string{})
}

func parseDeviceLong(line string) (DeviceInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return DeviceInfo{}, errors.Errorf(
			"malformed device line, expected at least 5 fields but found %d", len(fields))
	}
	attrs := make(map[string]string)
	for _, field := range fields[2:] {
		split := strings.Split(field, ":")
		if len(split) != 2 {
			continue
		}
		attrs[split[0]] = split[1]
	}
	return newDevice(fields[0], attrs)
}
