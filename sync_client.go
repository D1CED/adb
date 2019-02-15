package adb

import (
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
)

const (
	StatusSuccess  string = "OKAY"
	StatusFailure         = "FAIL"
	StatusSyncData        = "DATA"
	StatusSyncDone        = "DONE"
	StatusNone            = ""

	// Chunks cannot be longer than 64k.
	SyncMaxChunkSize = 64 * 1024
)

var zeroTime = time.Unix(0, 0).UTC()

func stat(conn io.ReadWriter, path string) (*DirEntry, error) {
	if _, err := conn.Write([]byte("STAT")); err != nil {
		return nil, err
	}
	if _, err := sendMessage(conn, path); err != nil {
		return nil, err
	}

	id, err := readTetra(conn)
	if err != nil {
		return nil, err
	}
	status := tetraToString(id)
	if status != "STAT" {
		return nil, errors.Errorf("expected stat ID 'STAT', but got '%s'", status)
	}

	return readStat(conn)
}

func listDirEntries(conn io.ReadWriteCloser, path string) (*DirEntries, error) {
	if _, err := conn.Write([]byte("LIST")); err != nil {
		return nil, err
	}
	if _, err := sendMessage(conn, path); err != nil {
		return nil, err
	}
	return &DirEntries{scanner: conn}, nil
}

func receiveFile(conn io.ReadWriteCloser, path string) (io.ReadCloser, error) {
	if _, err := conn.Write([]byte("RECV")); err != nil {
		return nil, err
	}
	if _, err := sendMessage(conn, path); err != nil {
		return nil, err
	}
	return newSyncFileReader(conn)
}

// sendFile returns a WriteCloser than will write to the file at path on device.
// The file will be created with permissions specified by mode.
// The file's modified time will be set to mtime, unless mtime is 0, in which case the time the writer is
// closed will be used.
func sendFile(conn io.ReadWriteCloser, path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	if _, err := conn.Write([]byte("SEND")); err != nil {
		return nil, err
	}
	pathAndMode := encodePathAndMode(path, mode)
	if _, err := sendMessage(conn, string(pathAndMode)); err != nil {
		return nil, err
	}
	return newSyncFileWriter(conn, mtime), nil
}

func readStat(s io.Reader) (*DirEntry, error) {
	t, err := readTetra(s)
	if err != nil {
		return nil, errors.Wrap(err, "error reading file mode")
	}
	mode := tetraToFileMode(t)

	t, err = readTetra(s)
	if err != nil {
		return nil, errors.Wrap(err, "error reading file size")
	}
	size := int32(tetraToInt(t))

	t, err = readTetra(s)
	if err != nil {
		return nil, errors.Wrap(err, "error reading file time")
	}
	mtime := tetraToTime(t)

	// adb doesn't indicate when a file doesn't exist, but will return all zeros.
	// Theoretically this could be an actual file, but that's very unlikely.
	if mode == os.FileMode(0) && size == 0 && mtime == zeroTime {
		return nil, ErrFileNotExist
	}
	return &DirEntry{
		Mode:       mode,
		Size:       size,
		ModifiedAt: mtime,
	}, nil
}
