package adb

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

const (
	statusSyncDent string = "DENT"
	statusSyncSend string = "SEND"
	statusSyncData string = "DATA"
	statusSyncDone string = "DONE"
	statusSyncStat string = "STAT"
	statusSyncList string = "LIST"
	statusSyncRecv string = "RECV"

	// Chunks cannot be longer than 64k.
	syncMaxChunkSize = 64 * 1024
)

var zeroTime = time.Unix(0, 0).UTC()

func readStat(r io.Reader) (DirEntry, error) {
	buf := make([]byte, 12)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return DirEntry{}, nil
	}

	var (
		mode  = os.FileMode(binary.LittleEndian.Uint32(buf[0:4]))
		size  = binary.LittleEndian.Uint32(buf[4:8])
		mtime = time.Unix(int64(binary.LittleEndian.Uint32(buf[8:12])), 0)
	)

	// adb doesn't indicate when a file doesn't exist, but will return all zeros.
	// Theoretically this could be an actual file, but that's very unlikely.
	if mode == os.FileMode(0) && size == 0 && mtime == zeroTime {
		return DirEntry{}, ErrFileNotExist
	}
	return DirEntry{
		FMode:      mode,
		FSize:      size,
		ModifiedAt: mtime,
	}, nil
}

func stat(conn io.ReadWriter, path string) (DirEntry, error) {
	err := sendSyncMessage(conn, statusSyncStat, path)
	if err != nil {
		return DirEntry{}, err
	}
	err = wantStatus(conn, "STAT")
	if err != nil {
		return DirEntry{}, err
	}
	return readStat(conn)
}

// sendFile returns a WriteCloser than will write to the file at path on device.
// The file will be created with permissions specified by mode.
// The file's modified time will be set to mtime, unless mtime is 0, in which case the time the writer is
// closed will be used.
func sendFile(conn io.ReadWriteCloser, path string, mode os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	pathAndMode := path + "," + strconv.Itoa(int(mode.Perm()))
	err := sendSyncMessage(conn, statusSyncSend, pathAndMode)
	if err != nil {
		return nil, err
	}
	return &syncFileWriter{mtime, conn}, nil
}

func openSyncConn(address, serial string) (net.Conn, error) {
	conn, err := dial(address)
	if err != nil {
		return nil, err
	}
	_, err = conn.Write([]byte("host-serial:" + serial + ":sync:"))
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// List lists the directory contents of path on file.
// move this to sync
func (d *Device) List(path string) ([]DirEntry, error) {
	conn, err := openSyncConn(d.server.address, d.serial)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	err = sendSyncMessage(conn, statusSyncList, path)
	if err != nil {
		return nil, err
	}
	return ReadAllDirEntries(conn)
}

// Stat returns filestats of path on device.
func (d *Device) Stat(path string) (DirEntry, error) {
	conn, err := openSyncConn(d.server.address, d.serial)
	if err != nil {
		return DirEntry{}, errors.Wrapf(err, "Stat(%s)", path)
	}
	defer conn.Close()

	entry, err := stat(conn, path)
	return entry, errors.WithMessagef(err, "Stat(%s)", path)
}

// ReadFile returns a a reader for the given path on the device.
func (d *Device) ReadFile(path string) (io.ReadCloser, error) {
	conn, err := openSyncConn(d.server.address, d.serial)
	if err != nil {
		return nil, errors.Wrapf(err, "OpenRead(%s)", path)
	}
	// don't close as syncfilereader get ioreadcloser
	err = sendSyncMessage(conn, statusSyncRecv, path)
	if err != nil {
		return nil, err
	}
	return newSyncFileReader(conn)
}

// OpenWrite opens the file at path on the device, creating it with the permissions specified
// by perms if necessary, and returns a writer that writes to the file.
// The files modification time will be set to mtime when the WriterCloser is closed. The zero value
// is TimeOfClose, which will use the time the Close method is called as the modification time.
// Deprecate this. Use CopyFile instead!
func (d *Device) OpenWrite(path string, perms os.FileMode, mtime time.Time) (io.WriteCloser, error) {
	conn, err := openSyncConn(d.server.address, d.serial)
	if err != nil {
		return nil, errors.Wrapf(err, "OpenWrite(%s)", path)
	}
	err = d.remount()
	if err != nil {
		conn.Close()
		return nil, errors.New("remount failed")
	}

	writer, err := sendFile(conn, path, perms, mtime)
	return writer, errors.WithMessagef(err, "OpenWrite(%s)", path)
}

// CopyFile copies the contents of r writing them to path on the device.
func (d *Device) CopyFile(path string, r io.Reader, perms os.FileMode, modtime time.Time) (int, error) {
	conn, err := openSyncConn(d.server.address, d.serial)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	pathAndMode := path + "," + strconv.Itoa(int(perms.Perm()))
	// sync message!
	err = sendSyncMessage(conn, statusSyncSend, pathAndMode)
	if err != nil {
		return 0, err
	}

	var (
		buf         = make([]byte, 64*1024)
		er          error
		nr, written int
	)

	// TODO(jmh): write time and close
	for er != nil {
		nr, er = r.Read(buf)
		wbuf := buf[:nr]
		for len(wbuf) > 0 {
			_, ew := conn.Write([]byte(statusSyncData))
			if ew != nil {
				return written, ew
			}
			// correct?
			length := make([]byte, 4)
			binary.LittleEndian.PutUint32(length, uint32(len(wbuf)))
			_, ew = conn.Write(length[:])
			if ew != nil {
				return written, ew
			}
			nw, ew := conn.Write(wbuf)
			if ew != nil {
				return written + nw, ew
			}

			written += nw
			wbuf = wbuf[nw:]
		}
		if er != nil {
			return written, er
		}
	}
	if er != io.EOF {
		return written, er
	}
	return written, nil
}
