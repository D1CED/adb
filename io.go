package adb

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func sendTime(w io.Writer, t time.Time) error {
	i := int32(t.Unix())
	j := int32ToTetra(i)
	_, err := w.Write(j[:])
	return err
}

func sendStatus(w io.Writer, status string) error {
	_, err := w.Write([]byte(status))
	return err
}

func sendMessage(w io.Writer, msg string) (int, error) {
	return w.Write([]byte(fmt.Sprintf("%04x%s", len(msg), msg)))
}

func readMessage(r io.Reader) (string, error) {
	t, err := readTetra(r)
	if err != nil {
		return "", err
	}
	length := hexTetraToInt(t)
	b := &strings.Builder{}
	_, err = io.CopyN(b, r, int64(length))
	return b.String(), err
}

func readTetra(r io.Reader) ([4]byte, error) {
	var t [4]byte
	_, err := r.Read(t[:])
	if err != nil {
		return [4]byte{}, err
	}
	return t, nil
}

func readStatus(r io.Reader) error {
	t, err := readTetra(r)
	if err != nil {
		return err
	}
	if tetraToString(t) != "FAIL" {
		return nil
	}
	t, err = readTetra(r)
	if err != nil {
		return err
	}
	length := hexTetraToInt(t)
	b := make([]byte, length)
	n, err := r.Read(b)
	if err != nil {
		return err
	}
	if n != length {
		return errors.New("incomplete read")
	}
	return errors.New(string(b))
}

func wantStatus(status string, r io.Reader) error {
	t, err := readTetra(r)
	if err != nil {
		return err
	}
	s := tetraToString(t)
	if s == "FAIL" {
		t, err := readTetra(r)
		if err != nil {
			return err
		}
		length := hexTetraToInt(t)
		b := &strings.Builder{}
		io.CopyN(b, r, int64(length))
		return errors.New(b.String())
	} else if s != status {
		err := errors.New(s)
		return errors.Wrapf(err, "got unexpected status, want %s", status)
	}
	return nil
}

func int32ToTetra(i int32) [4]byte {
	var t [4]byte
	for j := range t {
		t[i] = byte(i >> uint(j*8))
	}
	return t
}

func tetraToTime(t [4]byte) time.Time {
	i := tetraToInt(t)
	return time.Unix(i, 0)
}

func tetraToFileMode(t [4]byte) os.FileMode {
	i := tetraToInt(t)
	return os.FileMode(i)
}

func tetraToString(t [4]byte) string {
	return string(t[:])
}

func hexTetraToInt(t [4]byte) int {
	i, _ := strconv.ParseUint(string(t[:]), 16, 32)
	return int(i)
}

func tetraToInt(t [4]byte) int64 {
	var i uint32
	for j := range t {
		i |= uint32(t[j]) << uint(j*8)
	}
	return int64(i)
}
