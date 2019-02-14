package adb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func ParseDeviceList(t *testing.T) {
	devs, err := parseDeviceList(`192.168.56.101:5555	device
05856558`, parseDeviceShort)

	assert.NoError(t, err)
	assert.Len(t, devs, 2)
	assert.Equal(t, "192.168.56.101:5555", devs[0].Serial)
	assert.Equal(t, "05856558", devs[1].Serial)
}

func TestParseDevice(t *testing.T) {
	var tests = []struct {
		name      string
		parameter string
		want      DeviceInfo
	}{{
		name: "Short",
		parameter: "192.168.56.101:5555	device\n",
		want: DeviceInfo{Serial: "192.168.56.101:5555"},
	}, {
		name:      "Long",
		parameter: "SERIAL    device product:PRODUCT model:MODEL device:DEVICE\n",
		want: DeviceInfo{
			Serial:     "SERIAL",
			Product:    "PRODUCT",
			Model:      "MODEL",
			DeviceInfo: "DEVICE"},
	}, {
		name:      "LongUSB",
		parameter: "SERIAL    device usb:1234 product:PRODUCT model:MODEL device:DEVICE \n",
		want: DeviceInfo{
			Serial:     "SERIAL",
			Product:    "PRODUCT",
			Model:      "MODEL",
			DeviceInfo: "DEVICE",
			USB:        "1234"},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev, err := parseDeviceShort(test.parameter)
			if err != nil {
				t.Errorf("got unexpected error: %v", err)
			}
			if *dev != test.want {
				t.Fail()
			}
		})
	}
}
