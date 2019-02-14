package adb

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	// ADBExecutableName is the name in Path
	adbExecutableName = "adb"
	// ADBPort is the default port for the adb server to listens on.
	adbPort = 5037
)

var (
	lookPath         = exec.LookPath
	isExecutableFile = func(path string) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("not a regular file")
		}
		return isExecutable(path)
	}
	cmdCombinedOutput = func(name string, arg ...string) ([]byte, error) {
		return exec.Command(name, arg...).CombinedOutput()
	}
)

// Server communicates with host services on the adb server.
// Use New or NewWithConf
// TODO(z): Finish implementing host services.
type Server struct {
	// path to the adb executable. If empty, the PATH environment variable will be searched.
	path string
	// host and port the adb server is listening on.
	// If not specified, will use the default port on localhost.
	host string
	port int
	// dialer used to connect to the adb server.
	dialer Dial
}

// New creates a new Adb client that uses the default ServerConfig.
func New() (Server, error) {
	// "127.0.0.1" as port on windows recommended
	return NewWithConfig(ADBExecutableName, "localhost", ADBPort, TCPDial)
}

func NewConfig(path, host string, port int, dialer Dial) (Server, error) {
	s := Server{
		path:   path,
		host:   host,
		port:   port,
		dialer: dialer,
	}
	if err := isExecutableFile(path); err != nil {
		return Server{}, errors.Wrapf(err, "invalid adb executable: %s", path)
	}
	return s, nil
}

// Dial tries to connect to the server. If the first attempt fails, tries
// starting the server before retrying. If the second attempt fails, returns the error.
func (s Server) Dial() (Conn, error) {
	conn, err := s.dialer(s.address())
	if err != nil {
		// Attempt to start the server and try again.
		if err = s.Start(); err != nil {
			return nil, errors.Wrap(err, "error starting server for dial")
		}
		return s.dialer(s.address())
	}
	return conn, nil
}

// Start ensures there is a server running.
func (s Server) Start() error {
	output, err := cmdCombinedOutput(s.path, "start-server")
	outputStr := strings.TrimSpace(string(output))
	return errors.Wrapf(err, "error starting server: %s\noutput:\n%s", err, outputStr)
}

func (s Server) Device(d DeviceDescriptor) *Device {
	return &Device{
		server:         s,
		descriptor:     d,
		deviceListFunc: s.ListDevices,
	}
}

func (s Server) NewDeviceWatcher() *DeviceWatcher {
	return newDeviceWatcher(&s)
}

// Version asks the ADB server for its internal version number.
func (s Server) Version() (int, error) {

	parseServerVersion := func(versionRaw []byte) (int, error) {
		versionStr := string(versionRaw)
		version, err := strconv.ParseInt(versionStr, 16, 32)
		if err != nil {
			return 0, errors.Wrapf(err, "error parsing server version: %s", versionStr)
		}
		return int(version), nil
	}

	resp, err := roundTripSingleResponse(s, "host:version")
	if err != nil {
		return 0, wrapClientError(err, s, "GetServerVersion")
	}
	version, err := parseServerVersion(resp)
	if err != nil {
		return 0, wrapClientError(err, s, "GetServerVersion")
	}
	return version, nil
}

// Kill tells the server to quit immediately.
//
// Corresponds to the command:
//     adb kill-server
func (s Server) Kill() error {
	conn, err := s.Dial()
	if err != nil {
		return wrapClientError(err, s, "KillServer")
	}
	defer conn.Close()

	if err = conn.SendMessage([]byte("host:kill")); err != nil {
		return wrapClientError(err, s, "KillServer")
	}

	return nil
}

// ListDeviceSerials returns the serial numbers of all attached devices.
//
// Corresponds to the command:
//     adb devices
func (s Server) ListDeviceSerials() ([]string, error) {
	resp, err := roundTripSingleResponse(s, "host:devices")
	if err != nil {
		return nil, wrapClientError(err, s, "ListDeviceSerials")
	}

	devices, err := parseDeviceList(string(resp), parseDeviceShort)
	if err != nil {
		return nil, wrapClientError(err, s, "ListDeviceSerials")
	}

	serials := make([]string, len(devices))
	for i, dev := range devices {
		serials[i] = dev.Serial
	}
	return serials, nil
}

// ListDevices returns the list of connected devices.
//
// Corresponds to the command:
//     adb devices -l
func (s Server) ListDevices() ([]DeviceInfo, error) {
	resp, err := roundTripSingleResponse(s, "host:devices-l")
	if err != nil {
		return nil, wrapClientError(err, s, "ListDevices")
	}

	devices, err := parseDeviceList(string(resp), parseDeviceLong)
	if err != nil {
		return nil, wrapClientError(err, s, "ListDevices")
	}
	return devices, nil
}

func roundTripSingleResponse(adb Server, req string) ([]byte, error) {
	conn, err := adb.Dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return roundTripSingleResponseConn(conn, []byte(req))
}

func roundTripSingleNoResponse(s Server, req string) error {

	// RoundTripSingleResponse sends a message to the server
	// Only read status
	roundTripSingleNoResponse2 := func(c Conn, req []byte) error {
		err := c.SendMessage(req)
		if err != nil {
			return err
		}
		_, err = c.ReadStatus(string(req))
		return err
	}

	conn, err := s.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	return roundTripSingleNoResponse2(conn, []byte(req))
}

func (s Server) address() string {
	return s.host + ":" + strconv.Itoa(s.port)
}
