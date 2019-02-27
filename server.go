package adb

import (
	"bytes"
	"net"
	"os/exec"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

const (
	// DefaultExecutableName is the name of the ADB-Server on the Path
	DefaultExecutableName = "adb"
	// DefaultPort is the default port for the ADB-Server to listens on.
	DefaultPort = 5037

	// use statusOK
	statusSuccess string = "OKAY"
	// use statusFail
	statusFailure string = "FAIL"
)

// dialer used to connect to the adb server.
// Default is the regular Dialer form net.
// This exist only for easier mocking.
var dial = func(address string) (net.Conn, error) { return net.Dial("tcp", address) }

// Server holds information needed to connect to a server repeatedly.
// Use New or NewDefault to create one.
type Server struct {
	path    string
	address string
}

// NewDefault creates a new Adb client that uses the default ServerConfig.
func NewDefault() (*Server, error) {
	return New(DefaultExecutableName, "localhost", DefaultPort)
}

// New creates a new Server.
func New(path, host string, port int) (*Server, error) {
	// maybe add path search for adb?
	s := &Server{
		path:    path,
		address: host + ":" + strconv.Itoa(port),
	}
	err := start(s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func start(s *Server) error {
	out, err := exec.Command(s.path, "start-server").CombinedOutput()
	return errors.WithMessagef(err, "error starting server. Output:\n%s", out)
}

// requestResponseBytes sends msg to server and returns the response.
// The connection is closed. It prepends "host:" to the message.
// The connection times out after 10 seconds.
func (s *Server) requestResponseBytes(msg string) ([]byte, error) {
	conn, err := dial(s.address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	err = sendMessage(conn, "host:", msg)
	if err != nil {
		return nil, err
	}

	return readBytes(conn)
}

// send sends msg to server reads status then closes the connection.
// prepends 'host:'
func (s *Server) send(msg string) error {
	conn, err := dial(s.address)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	err = sendMessage(conn, "host:", msg)
	if err != nil {
		return err
	}

	return wantStatus(conn)
}

// Version asks the adb server for its internal version number.
// TODO(jmh): Check server version format
func (s *Server) Version() (int, error) {
	b, err := s.requestResponseBytes("version")
	if err != nil {
		return 0, err
	}

	v, _ := strconv.ParseInt(string(b), 16, 32)
	return int(v), nil
}

// Kill tells the server to quit immediately.
func (s *Server) Kill() error {
	return s.send("kill")
}

// ListDevices returns the list of connected devices.
func (s *Server) ListDevices() ([]DeviceInfo, error) {
	b, err := s.requestResponseBytes("devices-l")
	if err != nil {
		return nil, err
	}
	return parseDeviceList(bytes.NewBuffer(b), parseDeviceLong)
}

// ListDeviceSerials returns the serial numbers of all attached devices.
func (s *Server) ListDeviceSerials() ([]string, error) {
	b, err := s.requestResponseBytes("devices")
	if err != nil {
		return nil, err
	}
	devices, err := parseDeviceList(bytes.NewBuffer(b), parseDeviceShort)
	if err != nil {
		return nil, err
	}

	serials := make([]string, len(devices))
	for i, dev := range devices {
		serials[i] = dev.Serial
	}
	return serials, nil
}

// Device takes a devices serial number and returns it.
func (s *Server) Device(serial string) *Device {
	return &Device{
		server: &(*s), // copy server
		serial: serial,
	}
}

// NewDeviceWatcher starts a new device watcher.
func (s *Server) NewDeviceWatcher() (*DeviceWatcher, error) {
	return newDeviceWatcher(s)
}
