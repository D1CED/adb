package adb

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/pkg/errors"
)

/*
Remove this. This is a bad/unessecarry abstraction. Use something like
WriteFile(string, io.Reader) instead.

And close method can be omitted!

This has less types, less state, better composition.
It can be easily transformed into an io.Reader or better io.WriterTo via
a closure and a wrapper.

Example:
    type WriterFunc func([]byte) (int, error)

    func (wf WriterFunc) Write(b []byte) (int, error) {
        return wf(b)
    }

    var F WriterFunc = func(b []byte) (int, error) {
        return d.WriteFile("myfile", bytes.NewBuffer(b))
    }
*/

// syncFileWriter wraps a SyncConn that has requested to send a file.
type syncFileWriter struct {
	// The modification time to write in the footer.
	// If 0, use the current time.
	modTime time.Time

	// Reader used to read data from the adb connection.
	sender io.WriteCloser
}

var _ io.WriteCloser = &syncFileWriter{}

/*
encodePathAndMode encodes a path and file mode as required for starting a send file stream.

From https://android.googlesource.com/platform/system/core/+/master/adb/SYNC.TXT:
	The remote file name is split into two parts separated by the last
	comma (","). The first part is the actual path, while the second is a decimal
	encoded file mode containing the permissions of the file on device.
*/

// Write writes the min of (len(buf), 64k).
func (w *syncFileWriter) Write(buf []byte) (n int, err error) {
	written := 0

	// If buf > 64k we'll have to send multiple chunks.
	// TODO Refactor this into something that can coalesce smaller writes into a single chukn.
	for len(buf) > 0 {
		// Writes < 64k have a one-to-one mapping to chunks.
		// If buffer is larger than the max, we'll return the max size and leave it up to the
		// caller to handle correctly.
		partialBuf := buf
		if len(partialBuf) > syncMaxChunkSize {
			partialBuf = partialBuf[:syncMaxChunkSize]
		}

		if _, err := w.Write([]byte(statusSyncData)); err != nil {
			return written, err
		}
		// correct?
		length := make([]byte, 4)
		binary.LittleEndian.PutUint32(length, uint32(len(partialBuf)))
		if _, err := w.sender.Write(length[:]); err != nil {
			return written, err
		}
		n, err := w.sender.Write(partialBuf)
		if err != nil {
			return written + n, err
		}

		written += n
		buf = buf[n:]
	}

	return written, nil
}

// TODO(jmh): implement
func (w *syncFileWriter) ReadFrom(r io.Reader) (int64, error) {
	return 0, ErrNotImplemented
}

func (w *syncFileWriter) Close() error {
	if w.modTime == (time.Time{}) {
		w.modTime = time.Now()
	}
	// cancelation check?

	buf := make([]byte, 8)
	copy(buf[:4], statusSyncDone)
	binary.LittleEndian.PutUint32(buf[4:], uint32(w.modTime.Unix()))
	_, err := w.sender.Write(buf)
	if err != nil {
		return err
	}
	return errors.WithMessage(w.sender.Close(), "error closing FileWriter")
}
