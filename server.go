package adb

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	// DefaultExecutableName is the name of the ADB-Server on the Path
	DefaultExecutableName = "adb"
	// DefaultPort is the default port for the ADB-Server to listens on.
	DefaultPort = 5037
)

// Server holds information needed to connect to a server repeatedly.
// Use New or NewDefault to create one.
type Server struct {
	path    string
	address string

	conn net.Conn

	// dialer used to connect to the adb server.
	// Default is the regular Dialer form net.
	// This exist only for easier mocking.
	dial func(network, address string) (net.Conn, error)
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
		address: fmt.Sprintf("%s:%d", host, port),
		dial:    net.Dial,
	}
	err := s.start()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) start() error {
	out, err := exec.Command(s.path, "start-server").CombinedOutput()
	return errors.WithMessagef(err, "error starting server. Output:\n%s", out)
}

// requestResponse sends msg to server and writes the result into w.
// The connection is closed.
func (s *Server) requestResponse(msg string, w io.Writer) (int64, error) {
	conn, err := s.dial("tcp", s.address)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, err = sendMessage(conn, msg)
	if err != nil {
		return 0, err
	}
	err = readStatus(conn)
	if err != nil {
		return 0, err
	}
	t, err := readTetra(conn)
	if err != nil {
		return 0, err
	}
	length := hexTetraToInt(t)
	return io.CopyN(w, conn, int64(length))
}

// send sends msg to server reads status then closes the connection.
func (s *Server) send(msg string) error {
	conn, err := s.dial("tcp", s.address)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, err = sendMessage(conn, msg)
	if err != nil {
		return err
	}
	return wantStatus("OKAY", conn)
}

// Opens a connection to the adb server.
func (s *Server) openConn() error {
	conn, err := s.dial("tcp", s.address)
	if err != nil {
		return err
	}
	s.conn = conn
	return nil
}

func (s *Server) Close() error {
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// Version asks the adb server for its internal version number.
func (s *Server) Version() (int, error) {
	// TODO(JMH): Check server version format
	var parseVersion = func(versionStr string) (int, error) {
		version, err := strconv.ParseInt(versionStr, 16, 32)
		if err != nil {
			return 0, errors.Wrapf(err, "error parsing server version: %s", versionStr)
		}
		return int(version), nil
	}

	b := &strings.Builder{}
	_, err := s.requestResponse("host:version", b)
	if err != nil {
		return 0, err
	}
	v, err := parseVersion(b.String())
	if err != nil {
		return 0, err
	}
	return v, nil
}

// Kill tells the server to quit immediately.
//
// Corresponds to the command:
//     adb kill-server
func (s *Server) Kill() error {
	return s.send("host:kill")
}

// ListDevices returns the list of connected devices.
//
// Corresponds to the command:
//     adb devices -l
func (s *Server) ListDevices() ([]DeviceInfo, error) {
	b := &bytes.Buffer{}
	_, err := s.requestResponse("host:devices-l", b)
	if err != nil {
		return nil, err
	}
	devices, err := parseDeviceList(b, parseDeviceLong)
	if err != nil {
		return nil, err
	}
	return devices, nil
}

// ListDeviceSerials returns the serial numbers of all attached devices.
//
// Corresponds to the command:
//     adb devices
func (s *Server) ListDeviceSerials() ([]string, error) {
	b := &bytes.Buffer{}
	_, err := s.requestResponse("host:devices", b)
	if err != nil {
		return nil, err
	}
	devices, err := parseDeviceList(b, parseDeviceShort)
	if err != nil {
		return nil, err
	}

	serials := make([]string, len(devices))
	for i, dev := range devices {
		serials[i] = dev.Serial
	}
	return serials, nil
}

func (s *Server) Device(d DeviceDescriptor) *Device {
	return &Device{
		server:     &(*s),
		descriptor: d,
	}
}

func (s *Server) NewDeviceWatcher() *DeviceWatcher {
	return newDeviceWatcher(s)
}

// marked for removal. Use Server.requestResponse.
func roundTripSingleResponse(s *Server, req string) ([]byte, error) {
	b := &bytes.Buffer{}
	_, err := s.requestResponse(req, b)
	return b.Bytes(), err
}

// marked for removal. Use Server.send.
func roundTripSingleNoResponse(s *Server, req string) error {
	return s.send(req)
}
