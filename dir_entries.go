package adb

import (
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
)

// DirEntry holds information about a directory entry on a device.
type DirEntry struct {
	Name       string
	Mode       os.FileMode
	Size       int32
	ModifiedAt time.Time
}

// DirEntries iterates over directory entries.
type DirEntries struct {
	scanner io.ReadCloser

	currentEntry *DirEntry
	err          error
}

// ReadAllDirEntries reads all the remaining directory entries into a slice,
// closes self, and returns any error.
// If err is non-nil, result will contain any entries read until the error occurred.
func (entries *DirEntries) ReadAll() (result []*DirEntry, err error) {
	defer entries.Close()

	for entries.Next() {
		result = append(result, entries.Entry())
	}
	err = entries.Err()

	return
}

func (entries *DirEntries) Next() bool {
	if entries.err != nil {
		return false
	}

	entry, done, err := readNextDirListEntry(entries.scanner)
	if err != nil {
		entries.err = err
		entries.Close()
		return false
	}

	entries.currentEntry = entry
	if done {
		entries.Close()
		return false
	}

	return true
}

func (entries *DirEntries) Entry() *DirEntry {
	return entries.currentEntry
}

func (entries *DirEntries) Err() error {
	return entries.err
}

// Close closes the connection to the adb.
// Next() will call Close() before returning false.
func (entries *DirEntries) Close() error {
	return entries.scanner.Close()
}

func readNextDirListEntry(s io.Reader) (*DirEntry, bool, error) {
	t, err := readTetra(s)
	if err != nil {
		return nil, false, err
	}
	status := tetraToString(t)

	if status == "DONE" {
		return nil, true, nil
	} else if status != "DENT" {
		return nil, false, errors.Errorf("error reading dir entries: expected dir entry ID 'DENT', but got '%s'", status)
	}

	t, err = readTetra(s)
	if err != nil {
		return nil, false, errors.Wrap(err, "error reading dir entries: error reading file mode")
	}
	mode := tetraToFileMode(t)

	t, err = readTetra(s)
	if err != nil {
		return nil, false, errors.Wrap(err, "error reading dir entries: error reading file size")
	}
	size := int32(tetraToInt(t))

	t, err = readTetra(s)
	if err != nil {
		return nil, false, errors.Wrap(err, "error reading dir entries: error reading file time")
	}
	mtime := tetraToTime(t)

	name, err := readMessage(s)
	if err != nil {
		return nil, false, errors.Wrap(err, "error reading dir entries: error reading file name")
	}

	return &DirEntry{
		Name:       name,
		Mode:       mode,
		Size:       size,
		ModifiedAt: mtime,
	}, false, nil
}
