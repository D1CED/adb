package adb

import (
	"encoding/binary"
	"io"
	"os"
	"path"
	"time"

	"github.com/pkg/errors"
)

// DirEntry holds information about a directory entry on a device.
type DirEntry struct {
	FName      string
	FMode      os.FileMode
	FSize      uint32
	ModifiedAt time.Time
}

var _ os.FileInfo = DirEntry{}

func (de DirEntry) IsDir() bool {
	return de.FMode.IsDir()
}

func (de DirEntry) ModTime() time.Time {
	return de.ModifiedAt
}

func (de DirEntry) Mode() os.FileMode {
	return de.FMode
}

func (de DirEntry) Name() string {
	return path.Base(de.FName)
}

func (de DirEntry) Size() int64 {
	return int64(de.FSize)
}

func (de DirEntry) Sys() interface{} {
	return nil
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
		FName:      string(body),
		FMode:      mode,
		FSize:      size,
		ModifiedAt: time,
	}, nil
}
