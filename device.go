package adb

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// MtimeOfClose should be passed to OpenWrite to set the file modification time
// to the time the Close method is called.
var MtimeOfClose = time.Time{}

// Device communicates with a specific Android device.
// To get an instance, call Device() on an Adb.
type Device struct {
	server     *Server
	descriptor DeviceDescriptor
}

// openConn switches the connection to communicate directly with the device
// by requesting the transport defined by the DeviceDescriptor.
func (d *Device) openConn() error {
	// or maybe close and reopen?
	if d.server.conn != nil {
		return nil
	}
	err := d.server.openConn()
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("host:%s", d.descriptor.transportDescriptor())
	_, err = sendMessage(d.server.conn, msg)
	if err != nil {
		d.server.Close()
		return errors.Wrapf(err, "error connecting to device '%s'", d.descriptor)
	}
	if err = wantStatus("OKAY", d.server.conn); err != nil {
		d.server.Close()
		return err
	}
	return nil
}

func (d *Device) openSyncConn() error {
	if d.server.conn == nil {
		err := d.server.openConn()
		if err != nil {
			return err
		}
	}

	// Switch the connection to sync mode.
	if _, err := sendMessage(d.server.conn, "sync:"); err != nil {
		return err
	}
	if err := wantStatus("OKAY", d.server.conn); err != nil {
		return err
	}
	return nil
}

func (d *Device) String() string {
	return d.descriptor.String()
}

// get-product is documented, but not implemented, in the server.
// TODO(z): Make product exported if get-product is ever implemented in adb.
func (d *Device) product() (string, error) {
	attr, err := d.getAttribute("get-product")
	return attr, errors.Wrap(err, "Product")
}

func (d *Device) Serial() (string, error) {
	attr, err := d.getAttribute("get-serialno")
	return attr, errors.Wrap(err, "Serial")
}

func (d *Device) DevicePath() (string, error) {
	attr, err := d.getAttribute("get-devpath")
	return attr, errors.Wrap(err, "DevicePath")
}

func (d *Device) State() (DeviceState, error) {
	attr, err := d.getAttribute("get-state")
	state := parseDeviceState(attr)
	return state, errors.Wrap(err, "State")
}

const (
	FProtocolTcp        = "tcp"
	FProtocolAbstract   = "localabstract"
	FProtocolReserved   = "localreserved"
	FProtocolFilesystem = "localfilesystem"
)

type ForwardSpec struct {
	Protocol   string
	PortOrName string
}

func (f ForwardSpec) String() string {
	return fmt.Sprintf("%s:%s", f.Protocol, f.PortOrName)
}

func (f ForwardSpec) Port() (int, error) {
	if f.Protocol != FProtocolTcp {
		return 0, fmt.Errorf("protocal is not tcp")
	}
	return strconv.Atoi(f.PortOrName)
}

func (f *ForwardSpec) parseString(s string) error {
	fields := strings.Split(s, ":")
	if len(fields) != 2 {
		return fmt.Errorf("expect string contains only one ':', str = %s", s)
	}
	f.Protocol = fields[0]
	f.PortOrName = fields[1]
	return nil
}

type ForwardPair struct {
	Serial string
	Local  ForwardSpec
	Remote ForwardSpec
}

// ForwardList returns list with struct ForwardPair
// If no device serial specified all devices's forward list will returned
func (d *Device) ForwardList() (fs []ForwardPair, err error) {
	attr, err := d.getAttribute("list-forward")
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(attr)
	if len(fields)%3 != 0 {
		return nil, fmt.Errorf("list forward parse error")
	}
	fs = make([]ForwardPair, 0)
	for i := 0; i < len(fields)/3; i++ {
		var local, remote ForwardSpec
		var serial = fields[i*3]
		// skip other device serial forwards
		if d.descriptor == AnyDeviceSerial(serial) {
			continue
		}
		if err = local.parseString(fields[i*3+1]); err != nil {
			return nil, err
		}
		if err = remote.parseString(fields[i*3+2]); err != nil {
			return nil, err
		}
		fs = append(fs, ForwardPair{serial, local, remote})
	}
	return fs, nil
}

// ForwardRemove specified forward
func (d *Device) ForwardRemove(local ForwardSpec) error {
	err := roundTripSingleNoResponse(d.server,
		fmt.Sprintf("%s:killforward:%v", d.descriptor.hostPrefix(), local))
	return errors.Wrap(err, "ForwardRemove")
}

// ForwardRemoveAll cancel all exists forwards
func (d *Device) ForwardRemoveAll() error {
	err := roundTripSingleNoResponse(d.server,
		fmt.Sprintf("%s:killforward-all", d.descriptor.hostPrefix()))
	return errors.Wrap(err, "ForwardRemoveAll")
}

// Forward remote connection to local
func (d *Device) Forward(local, remote ForwardSpec) error {
	err := roundTripSingleNoResponse(d.server,
		fmt.Sprintf("%s:forward:%v;%v", d.descriptor.hostPrefix(), local, remote))
	return errors.Wrap(err, "Forward")
}

// ForwardToFreePort return random generated port
// If forward already exists, just return current forworded port
func (d *Device) ForwardToFreePort(remote ForwardSpec) (port int, err error) {
	fws, err := d.ForwardList()
	if err != nil {
		return
	}
	for _, fw := range fws {
		if fw.Remote == remote {
			return fw.Local.Port()
		}
	}
	port = getFreePort()
	if err != nil {
		return
	}
	err = d.Forward(ForwardSpec{FProtocolTcp, strconv.Itoa(port)}, remote)
	return
}

func (d *Device) DeviceInfo() (DeviceInfo, error) {
	// adb doesn't actually provide a way to get this for an individual device,
	// so we have to just list devices and find ourselves.

	serial, err := d.Serial()
	if err != nil {
		return DeviceInfo{}, errors.Wrap(err, "GetDeviceInfo(GetSerial)")
	}

	devices, err := d.server.ListDevices()
	if err != nil {
		return DeviceInfo{}, errors.Wrap(err, "DeviceInfo(ListDevices)")
	}

	for _, deviceInfo := range devices {
		if deviceInfo.Serial == serial {
			return deviceInfo, nil
		}
	}

	err = errors.Wrapf(ErrDeviceNotFound, "device list doesn't contain serial %s", serial)
	return DeviceInfo{}, errors.Wrap(err, "DeviceInfo")
}

/*
RunCommand runs the specified commands on a shell on the device.

From the Android docs:
	Run 'command arg1 arg2 ...' in a shell on the device, and return
	its output and error streams. Note that arguments must be separated
	by spaces. If an argument contains a space, it must be quoted with
	double-quotes. Arguments cannot contain double quotes or things
	will go very wrong.

	Note that this is the non-interactive version of "adb shell"
Source: https://android.googlesource.com/platform/system/core/+/master/adb/SERVICES.TXT

This method quotes the arguments for you, and will return an error if any of them
contain double quotes.

Because the adb shell converts all "\n" into "\r\n",
so here we convert it back (maybe not good for binary output)
*/
func (d *Device) RunCommand(cmd string, args ...string) (string, error) {
	err := d.OpenCommand(cmd, args...)
	if err != nil {
		return "", err
	}
	b := &strings.Builder{}
	_, err = io.Copy(b, d.server.conn)
	return b.String(), errors.WithMessage(err, "RunCommand")
}

// OpenCommand the connection to accept commands.
func (d *Device) OpenCommand(cmd string, args ...string) error {
	cmd, err := prepareCommandLine(cmd, args...)
	if err != nil {
		return err
	}
	err = d.openConn()
	if err != nil {
		return err
	}

	req := fmt.Sprintf("shell:%s", cmd)

	// Shell responses are special, they don't include a length header.
	// We read until the stream is closed.
	// So, we can't use conn.RoundTripSingleResponse.
	if _, err = sendMessage(d.server.conn, req); err != nil {
		return err
	}
	if err = wantStatus("OKAY", d.server.conn); err != nil {
		return err
	}
	return nil
}

/*
Remount, from the official adb command’s docs:
	Ask adbd to remount the device's filesystem in read-write mode,
	instead of read-only. This is usually necessary before performing
	an "adb sync" or "adb push" request.
	This request may not succeed on certain builds which do not allow
	that.
Source: https://android.googlesource.com/platform/system/core/+/master/adb/SERVICES.TXT
*/
func (d *Device) Remount() (string, error) {
	b := &strings.Builder{}
	_, err := d.server.requestResponse("remount", b)
	return b.String(), errors.WithMessage(err, "Remount")
}

func (d *Device) ListDirEntries(path string) (*DirEntries, error) {
	err := d.openSyncConn()
	if err != nil {
		return nil, errors.Wrapf(err, "ListDirEntries(%s)", path)
	}
	defer d.server.Close()

	entries, err := listDirEntries(d.server.conn, path)
	return entries, errors.Wrapf(err, "ListDirEntries(%s)", path)
}

func (d *Device) Stat(path string) (*DirEntry, error) {
	err := d.openSyncConn()
	if err != nil {
		return nil, errors.Wrapf(err, "Stat(%s)", path)
	}
	defer d.server.Close()

	entry, err := stat(d.server.conn, path)
	return entry, errors.Wrapf(err, "Stat(%s)", path)
}

func (d *Device) OpenRead(path string) (io.ReadCloser, error) {
	err := d.openSyncConn()
	if err != nil {
		return nil, errors.Wrapf(err, "OpenRead(%s)", path)
	}

	reader, err := receiveFile(d.server.conn, path)
	return reader, errors.Wrapf(err, "OpenRead(%s)", path)
}

// OpenWrite opens the file at path on the device, creating it with the permissions specified
// by perms if necessary, and returns a writer that writes to the file.
// The files modification time will be set to mtime when the WriterCloser is closed. The zero value
// is TimeOfClose, which will use the time the Close method is called as the modification time.
func (d *Device) OpenWrite(path string, perms os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	err := d.openSyncConn()
	if err != nil {
		return nil, errors.Wrapf(err, "OpenWrite(%s)", path)
	}

	writer, err := sendFile(d.server.conn, path, perms, mtime)
	return writer, errors.Wrapf(err, "OpenWrite(%s)", path)
}

// getAttribute returns the first message returned by the server by running
// <host-prefix>:<attr>, where host-prefix is determined from the DeviceDescriptor.
func (c *Device) getAttribute(attr string) (string, error) {
	resp, err := roundTripSingleResponse(c.server,
		fmt.Sprintf("%s:%s", c.descriptor.hostPrefix(), attr))
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

// prepareCommandLine validates the command and argument strings, quotes
// arguments if required, and joins them into a valid adb command string.
func prepareCommandLine(cmd string, args ...string) (string, error) {
	if isBlank(cmd) {
		return "", errors.Wrap(ErrAssertionViolation, "command cannot be empty")
	}

	for i, arg := range args {
		if strings.ContainsRune(arg, '"') {
			return "", errors.Wrapf(ErrParsing, "arg at index %d contains an invalid double quote: %s", i, arg)
		}
		if containsWhitespace(arg) {
			args[i] = fmt.Sprintf("\"%s\"", arg)
		}
	}

	// Prepend the command to the args array.
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}

	return cmd, nil
}
