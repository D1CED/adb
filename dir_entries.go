package adb

import (
	"encoding/binary"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
)

// DirEntry holds information about a directory entry on a device.
// TODO(jmh): implement os.FileInfo for this.
type DirEntry struct {
	Name       string
	Mode       os.FileMode
	Size       uint32
	ModifiedAt time.Time
}

// ReadAllDirEntries reads directory entries into a slice,
// closes self, and returns any error.
// If err is non-nil, result will contain any entries read until the error occurred.
func ReadAllDirEntries(r io.Reader) ([]DirEntry, error) {
	result := make([]DirEntry, 0, 4)
	de, err := readNextDirListEntry(r)
	for err == nil {
		result = append(result, de)
		de, err = readNextDirListEntry(r)
	}
	if err != done {
		return result, err
	}
	return result, nil
}

// done signals successful completion
var done = errors.New("DONE")

func readNextDirListEntry(r io.Reader) (DirEntry, error) {
	header := make([]byte, 4*5)
	n, err := io.ReadFull(r, header)
	if err == io.ErrUnexpectedEOF {
		if n == 4 && string(header[:4]) == statusSyncDone {
			return DirEntry{}, done
		} else {
			return DirEntry{}, errors.New("unexpected status")
		}
	}
	if string(header[:4]) != statusSyncDent {
		return DirEntry{}, errors.New("unexpected status")
	}

	var (
		mode   = os.FileMode(binary.LittleEndian.Uint32(header[4:8]))
		size   = binary.LittleEndian.Uint32(header[8:12])
		time   = time.Unix(int64(binary.LittleEndian.Uint32(header[12:16])), 0)
		length = binary.LittleEndian.Uint32(header[16:20])
	)

	body := make([]byte, length)
	_, err = io.ReadFull(r, body)
	if err != nil {
		return DirEntry{}, nil
	}

	return DirEntry{
		Name:       string(body),
		Mode:       mode,
		Size:       size,
		ModifiedAt: time,
	}, nil
}
