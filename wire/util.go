package wire

import (
	"io"
	"os"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
)

// DeviceNotFoundMessagePattern matches all possible error messages returned by adb servers to
// report that a matching device was not found. Used to set the DeviceNotFound error code on
// error values.
//
// Old servers send "device not found", and newer ones "device 'serial' not found".
var DeviceNotFoundMessagePattern = regexp.MustCompile(`device( '.*')? not found`)

func ReadTetra(r io.Reader) ([4]byte, error) {
	var octet [4]byte
	n, err := r.Read(octet[:])
	if n != 4 || err != nil {
		return [4]byte{}, errors.Wrap(err, "octet read failed")
	}
	return octet, nil
}

func TetraToString(tetra [4]byte) string {
	return string(tetra[:])
}

func HexTetraToLen(octet [4]byte) int {
	len, err := strconv.ParseInt(string(octet[:]), 16, 32)
	if err != nil {
		return -1
	}
	return int(len)
}

// TetraToUint32 converts an octet to an uint32 in little endian form.
func TetraToUint32(octet [4]byte) uint32 {
	var i uint32
	for j := range octet {
		i |= uint32(octet[j]) << (uint(j) * 8)
	}
	return i
}

func Uint32ToTetra(u uint32) [4]byte {
	var t [4]byte
	for i := range t {
		t[i] = byte(u >> uint(i) * 8)
	}
	return t
}

// ADB file modes seem to only be 16 bits.
// Values are taken from http://linux.die.net/include/bits/stat.h.
// These numbers are octal.
const (
	ModeDir        = 0040000
	ModeSymlink    = 0120000
	ModeSocket     = 0140000
	ModeFifo       = 0010000
	ModeCharDevice = 0020000
)

// ADBFileMode parses the mode returned by sync
func ADBFileMode(mode uint32) os.FileMode {
	// The ADB filemode uses the permission bits defined in Go's os package, but
	// we need to parse the other bits manually.
	var filemode os.FileMode
	switch {
	case mode&ModeSymlink == ModeSymlink:
		filemode = os.ModeSymlink
	case mode&ModeDir == ModeDir:
		filemode = os.ModeDir
	case mode&ModeSocket == ModeSocket:
		filemode = os.ModeSocket
	case mode&ModeFifo == ModeFifo:
		filemode = os.ModeNamedPipe
	case mode&ModeCharDevice == ModeCharDevice:
		filemode = os.ModeCharDevice
	}
	filemode |= os.FileMode(mode).Perm()
	return filemode
}
