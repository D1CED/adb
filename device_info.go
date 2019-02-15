package adb

import (
	"bufio"
	"io"
	"strings"

	"github.com/pkg/errors"
)

type DeviceInfo struct {
	// Always set.
	Serial string
	// Product, device, and model are not set in the short form.
	Product    string
	Model      string
	DeviceInfo string
	// Only set for devices connected via USB.
	USB string
}

func newDevice(serial string, attrs map[string]string) (DeviceInfo, error) {
	if serial == "" {
		return DeviceInfo{}, errors.Wrap(ErrAssertionViolation, "device serial cannot be blank")
	}
	return DeviceInfo{
		Serial:     serial,
		Product:    attrs["product"],
		Model:      attrs["model"],
		DeviceInfo: attrs["device"],
		USB:        attrs["usb"],
	}, nil
}

// IsUSB returns true if the device is connected via USB.
func (d DeviceInfo) IsUSB() bool {
	return d.USB != ""
}

func parseDeviceList(list io.Reader, lineParseFunc func(string) (DeviceInfo, error)) ([]DeviceInfo, error) {
	devices := []DeviceInfo{}
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
		return DeviceInfo{}, errors.Wrapf(ErrParsing,
			"malformed device line, expected 2 fields but found %d", len(fields))
	}
	return newDevice(fields[0], map[string]string{})
}

func parseDeviceLong(line string) (DeviceInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return DeviceInfo{}, errors.Wrapf(ErrParsing,
			"malformed device line, expected at least 5 fields but found %d", len(fields))
	}

	attrs := parseDeviceAttributes(fields[2:])
	return newDevice(fields[0], attrs)
}

func parseDeviceAttributes(fields []string) map[string]string {
	attrs := map[string]string{}
	for _, field := range fields {
		key, val := parseKeyVal(field)
		attrs[key] = val
	}
	return attrs
}

// Parses a key:val pair and returns key, val.
func parseKeyVal(pair string) (string, string) {
	split := strings.Split(pair, ":")
	if len(split) != 2 {
		return "", ""
	}
	return split[0], split[1]
}
