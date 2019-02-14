package wire

import (
	"bytes"
	"io"

	"github.com/pkg/errors"
)

/*
SyncConn is a connection to the adb server in sync mode.
Assumes the connection has been put into sync mode (by sending "sync" in transport mode).

The adb sync protocol is defined at
https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT.

Unlike the normal adb protocol (implemented in Conn), the sync protocol is binary.
Lengths are binary-encoded (little-endian) instead of hex.

Notes on Encoding

Length headers and other integers are encoded in little-endian, with 32 bits.

File mode seems to be encoded as POSIX file mode.

Modification time seems to be the Unix timestamp format, i.e. seconds since Epoch UTC.
*/

// syncMaxChunkSize cannot be longer than 64k.
const syncMaxChunkSize = 64 * 1024

// request types
const (
	listRequest    = "LIST"
	retriveRequest = "RECV"
	sendRequest    = "SEND"
	statRequest    = "STAT"
)

func (c *Conn) syncWrite(requestType string, msg string) (int64, error) {
	if len(requestType) != 4 {
		return 0, errors.Errorf("malformed requestType: %s", requestType)
	}
	t := Uint32ToTetra(uint32(len(msg)))
	b := &bytes.Buffer{}
	b.WriteString(requestType)
	b.Write(t[:])
	b.WriteString(msg)
	return io.Copy(c.rw, b)
}

func (c *Conn) syncStatus() Status {
	t, err := ReadTetra(c.rw)
	return Status{t, err}
}

func (c *Conn) syncRead(buf []byte) (int, error) {
	return c.rw.Read(buf)
}
