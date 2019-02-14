package adb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yosemite-open/go-adb/internal/errors"
	"github.com/yosemite-open/go-adb/wire"
)

func TestGetAttribute(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"value"},
	}
	client := s.Device(DeviceWithSerial("serial"))

	v, err := client.getAttribute("attr")
	assert.Equal(t, "host-serial:serial:attr", s.Requests[0])
	assert.NoError(t, err)
	assert.Equal(t, "value", v)
}

func TestGetDeviceInfo(t *testing.T) {
	deviceLister := func() ([]*DeviceInfo, error) {
		return []*DeviceInfo{
			&DeviceInfo{
				Serial:  "abc",
				Product: "Foo",
			},
			&DeviceInfo{
				Serial:  "def",
				Product: "Bar",
			},
		}, nil
	}

	client := newDeviceClientWithDeviceLister("abc", deviceLister)
	device, err := client.DeviceInfo()
	assert.NoError(t, err)
	assert.Equal(t, "Foo", device.Product)

	client = newDeviceClientWithDeviceLister("def", deviceLister)
	device, err = client.DeviceInfo()
	assert.NoError(t, err)
	assert.Equal(t, "Bar", device.Product)

	client = newDeviceClientWithDeviceLister("serial", deviceLister)
	device, err = client.DeviceInfo()
	assert.True(t, HasErrCode(err, DeviceNotFound))
	assert.EqualError(t, err.(*errors.Err).Cause,
		"DeviceNotFound: device list doesn't contain serial serial")
	assert.Nil(t, device)
}

func newDeviceClientWithDeviceLister(serial string, deviceLister func() ([]*DeviceInfo, error)) *Device {
	client := (&ADB{&MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{serial},
	}}).Device(DeviceWithSerial(serial))
	client.deviceListFunc = deviceLister
	return client
}

func TestRunCommandNoArgs(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"output"},
	}
	client := (&ADB{s}).Device(AnyDevice())

	v, err := client.RunCommand("cmd")
	assert.Equal(t, "host:transport-any", s.Requests[0])
	assert.Equal(t, "shell:cmd", s.Requests[1])
	assert.NoError(t, err)
	assert.Equal(t, "output", v)
}

func TestForward(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{""},
	}
	client := (&ADB{s}).Device(DeviceWithSerial("abc"))
	err := client.Forward(ForwardSpec{"tcp", "8999"}, ForwardSpec{"localabstract", "demo"})
	assert.Equal(t, "host-serial:abc:forward:tcp:8999;localabstract:demo", s.Requests[0])
	assert.NoError(t, err)
}

func TestForwardList(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"serial tcp:8999 tcp:d1\nabc tcp:8994 udp:d2\nabc tcp:8995 udp:d3"},
	}
	client := (&Adb{s}).Device(DeviceWithSerial("abc"))
	fws, err := client.ForwardList()
	assert.NoError(t, err)
	assert.Equal(t, "host-serial:abc:list-forward", s.Requests[0])
	assert.Equal(t, 2, len(fws))
	assert.Equal(t, fws[0].Serial, "abc")
	assert.Equal(t, fws[0].Local.Protocol, "tcp")
	assert.Equal(t, fws[0].Local.PortOrName, "8994")
	assert.Equal(t, fws[0].Remote.Protocol, "udp")
	assert.Equal(t, fws[0].Remote.PortOrName, "d2")
	assert.Equal(t, fws[1].Remote.PortOrName, "d3")
}

func TestForwardRemove(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{""},
	}
	client := (&Adb{s}).Device(DeviceWithSerial("abc"))
	err := client.ForwardRemove(ForwardSpec{"tcp", "8999"})
	assert.Equal(t, "host-serial:abc:killforward:tcp:8999", s.Requests[0])
	assert.NoError(t, err)
}

func TestForwardRemoveAll(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{""},
	}
	client := (&Adb{s}).Device(DeviceWithSerial("abc"))
	err := client.ForwardRemoveAll()
	assert.Equal(t, "host-serial:abc:killforward-all", s.Requests[0])
	assert.NoError(t, err)
}

func TestProperties(t *testing.T) {
	s := &MockServer{
		Status:   wire.StatusSuccess,
		Messages: []string{"[wifi.interface]: [wlan0]\r\n[wlan.driver.ath]: [0]\r\n"},
	}
	client := (&Adb{s}).Device(AnyDevice())
	props, err := client.Properties()
	assert.NoError(t, err)
	assert.Equal(t, len(props), 2)
	assert.Equal(t, props["wifi.interface"], "wlan0")
	assert.Equal(t, props["wlan.driver.ath"], "0")
}

func TestPrepareCommandLineNoArgs(t *testing.T) {
	result, err := prepareCommandLine("cmd")
	assert.NoError(t, err)
	assert.Equal(t, "cmd", result)
}

func TestPrepareCommandLineEmptyCommand(t *testing.T) {
	_, err := prepareCommandLine("")
	assert.Equal(t, errors.AssertionError, code(err))
	assert.Equal(t, "command cannot be empty", message(err))
}

func TestPrepareCommandLineBlankCommand(t *testing.T) {
	_, err := prepareCommandLine("  ")
	assert.Equal(t, errors.AssertionError, code(err))
	assert.Equal(t, "command cannot be empty", message(err))
}

func TestPrepareCommandLineCleanArgs(t *testing.T) {
	result, err := prepareCommandLine("cmd", "arg1", "arg2")
	assert.NoError(t, err)
	assert.Equal(t, "cmd arg1 arg2", result)
}

func TestPrepareCommandLineArgWithWhitespaceQuotes(t *testing.T) {
	result, err := prepareCommandLine("cmd", "arg with spaces")
	assert.NoError(t, err)
	assert.Equal(t, "cmd \"arg with spaces\"", result)
}

func TestPrepareCommandLineArgWithDoubleQuoteFails(t *testing.T) {
	_, err := prepareCommandLine("cmd", "quoted\"arg")
	assert.Equal(t, errors.ParseError, code(err))
	assert.Equal(t, "arg at index 0 contains an invalid double quote: quoted\"arg", message(err))
}

func code(err error) errors.ErrCode {
	return err.(*errors.Err).Code
}

func message(err error) string {
	return err.(*errors.Err).Message
}
