package adb

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

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

// IsUSB returns true if the device is connected via USB.
// remove?
func (d DeviceInfo) IsUSB() bool {
	return d.USB != ""
}

func FormatDeviceInfo(dd []DeviceInfo) string {
	f := new(strings.Builder)
	fmt.Fprintln(f, "Serial Product Model Device USB")
	for _, d := range dd {
		fmt.Fprintf(f, "%6s %7s %5s %6s %3s\n",
			d.Serial, d.Product, d.Model, d.Device, d.USB)
	}
	return f.String()
}

func parseDeviceList(list io.Reader, lineParseFunc func(string) (DeviceInfo, error)) ([]DeviceInfo, error) {
	devices := make([]DeviceInfo, 0, 4)
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
	return DeviceInfo{Serial: fields[0]}, nil
}

func parseDeviceLong(line string) (DeviceInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return DeviceInfo{}, errors.Errorf(
			"malformed device line, expected at least 5 fields but found %d", len(fields))
	}
	di := DeviceInfo{Serial: fields[0]}
	for _, field := range fields[2:] {
		split := strings.Split(field, ":")
		if len(split) != 2 {
			continue
		}
		switch s := split[1]; split[0] {
		case "product":
			di.Product = s
		case "model":
			di.Model = s
		case "device":
			di.Device = s
		case "usb":
			di.USB = s
		}
	}
	return di, nil
}
