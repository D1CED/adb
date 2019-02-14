package wire

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

// maxMessageLength is set to because the official implementation of adb
// imposes an undocumented 255-byte limit on messages.
// TODO(JMH): Check if correct. My guess would be 0xFFFF, so 255 * 255
const maxMessageLength = 255

// Status holds a StatusCode. If the StatusCode is "FAIL" it holds the
// corresponding error.
type Status struct {
	status [4]byte
	Error  error
}

func (s Status) String() string {
	return string(s.status[:])
}

// StatusCodes are returned by the server. If the code indicates failure, the
// next message will be the error.
var (
	StatusSuccess = Status{status: [4]byte{'O', 'K', 'A', 'Y'}} // "OKAY"

	StatusSyncData = Status{status: [4]byte{'D', 'A', 'T', 'A'}} // "DATA"
	StatusSyncDone = Status{status: [4]byte{'D', 'O', 'N', 'E'}} // "DONE"

	statusFail = [4]byte{'F', 'A', 'I', 'L'}
)

// StatusFailure reports true if the actual status is FAIL or if the status
// is unknown but carried with an error meassage.
func StatusFailure(s Status) bool {
	return s.status == statusFail || (s.status == [4]byte{} && s.Error != nil)
}

/*
Conn is a normal connection to an adb server.

For most cases, usage looks something like:
	conn := wire.Dial()
	conn.Write(data)
	conn.Status().Error != nil
	conn.Read(buffer)

For some messages, the server will return more than one message (but still a
single status). Generally, after calling ReadStatus once, you should call
ReadMessage until it returns an io.EOF error. Note: the protocol docs seem to
suggest that connections will be kept open for multiple commands, but this is
not the case. The official client closes a connection immediately after its
read the response, in most cases. The docs might be referring to the connection
between the adb server and the device, but I haven't confirmed that.
*/
// TODO(JMH): Consider removing Conn. It really just wraps io.ReadWriter.
// It is a short lived object just intended for one single request-response cycle.
type Conn struct {
	rw io.ReadWriter
	// bytes currently on the connection
	n int
}

// Status reads the status, and if failure, reads the message and returns
// it as an error. If the status is success, doesn't read the message.
func (c *Conn) status() Status {
	status, err := ReadTetra(c.rw)
	if err != nil {
		return Status{Error: errors.Wrap(err, "error reading status")}
	}
	if status == statusFail {
		b := &strings.Builder{}
		_, err := c.writeTo(b)
		if err != nil {
			return Status{status, errors.Wrap(err,
				"server returned error, but couldn't read the error message")}
		}
		return Status{status, errors.Errorf(b.String())}
	}
	return Status{status, nil}
}

// read reads a message from the server. Read until you get EOF.
// This is for regular connection mode.
func (c *Conn) read(b []byte) (int, error) {
	if c.n != 0 {
		o, err := ReadTetra(c.rw)
		if err != nil {
			return 0, err
		}
		c.n = HexTetraToLen(o)
	}
	n, err := c.rw.Read(b)
	c.n -= n
	if c.n == 0 {
		return n, io.EOF
	}
	return n, errors.WithMessage(err, "error reading message data")
}

// writeTo reads data from server writung to w. Returns a nil error on succsess.
func (c *Conn) writeTo(w io.Writer) (int64, error) {
	o, err := ReadTetra(c.rw)
	if err != nil {
		return 0, err
	}
	length := HexTetraToLen(o)
	n, err := io.Copy(w, c.rw)
	if err != nil {
		return n, errors.Wrap(err, "error reading message data")
	}
	if n != int64(length) {
		return n, errors.New("incomplete read")
	}
	return n, nil
}

// write sends the buffer msg to the server. Messages limit is 0xFFFF bytes.
func (c *Conn) write(msg []byte) (int64, error) {
	if len(msg) > maxMessageLength {
		return 0, errors.Errorf("message length exceeds maximum: %d > %d",
			len(msg), maxMessageLength)
	}
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "%04x", len(msg))
	b.Write(msg)
	return io.Copy(c.rw, b)
}
