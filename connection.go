package adb

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/yosemite-open/go-adb/wire"
)

var (
	// _ Connection = &wire.Conn{}
	_ SyncScanner = wire.SyncScanner{}
	_ Conn        = &wire.Conn{}
)

type Conn interface {
	io.ReadCloser
	ReadStatus(string) (string, error)

	SendMessage([]byte) error
	ReadMessage() ([]byte, error)

	// NewSyncConn() *wire.SyncConn
}

// RoundTripSingleResponse sends a message to the server, and reads a single
// message response. If the reponse has a failure status code, returns it as an error.
func roundTripSingleResponseConn(c Conn, req []byte) (resp []byte, err error) {
	if err = c.SendMessage(req); err != nil {
		return nil, err
	}
	if _, err = c.ReadStatus(string(req)); err != nil {
		return nil, err
	}
	return c.ReadMessage()
}

// Dial knows how to create connections to an adb server.
type Dial func(address string) (Conn, error)

// TCPDial connects to the adb server on the host and port set on the netDialer.
// The zero-value will connect to the default, localhost:5037.
func TCPDial(address string) (Conn, error) {
	netConn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, errors.Wrapf(err, "error dialing %s", address)
	}

	// Better close it manually.
	/*
		// net.Conn can't be closed more than once, but wire.Conn will try to close both sender and scanner
		// so we need to wrap it to make it safe.
		safeConn := wire.MultiCloseable(netConn)

		// Prevent leaking the network connection, not sure if TCPConn does this itself.
		// Note that the network connection may still be in use after the conn isn't (scanners/senders
		// can give their underlying connections to other scanner/sender types), so we can't
		// set the finalizer on conn.
		runtime.SetFinalizer(safeConn, func(conn io.ReadWriteCloser) {
			conn.Close()
		})
	*/

	return &wire.Conn{
		Scanner: wire.Scanner{netConn},
		Sender:  wire.Sender{netConn},
	}, nil
}

type SyncScanner interface {
	Close() error
	ReadStatus(string) (string, error)

	ReadBytes() (io.Reader, error)
	ReadFileMode() (os.FileMode, error)
	ReadInt32() (int32, error)
	ReadString() (string, error)
	ReadTime() (time.Time, error)
}

/*
	SendBytes(data []byte) error
	SendFileMode(os.FileMode) error
	SendInt32(int32) error
	SendOctetString(string) error
	SendTime(time.Time) error
*/
