package adb

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
)

const (
	statusOK   = "OKAY"
	statusFail = "FAIL"
)

// use this only for short writes
func sendMessage(w io.Writer, msg string) error {
	n, err := w.Write([]byte(fmt.Sprintf("%04x%s", len(msg), msg)))
	if err != nil {
		return err
	}
	if n != len(msg)+4 {
		return io.ErrShortWrite
	}
	return nil
}

func sendSyncMessage(w io.Writer, status, msg string) error {
	if len(msg) > syncMaxChunkSize {
		return errors.New("maximum message length exceded")
	}
	buf := make([]byte, len(status)+4+len(msg))
	buf = append(buf, status...)
	binary.LittleEndian.PutUint32(buf, uint32(len(msg)))
	buf = append(buf, msg...)
	n, err := w.Write(buf)
	if err != nil {
		return err
	}
	if n != len(msg)+4 {
		return io.ErrShortWrite
	}
	return nil
}

// wantStatus errors when the connection responds with a different status
// than status. If 'FAIL' is reportet the returned error will carry the
// error given by the server.
// When acceptStatus is left empty defaults to statusOK
func wantStatus(r io.Reader, acceptStatus ...string) error {
	buf := make([]byte, 4)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return err
	}
	s := string(buf)

	if s == statusFail {
		_, err = io.ReadFull(r, buf)
		if err != nil {
			return err
		}
		length := binary.LittleEndian.Uint32(buf)
		errMsg := make([]byte, length)
		_, err := io.ReadFull(r, errMsg)
		if err != nil {
			return err
		}
		return errors.New(string(errMsg))
	}

	if len(acceptStatus) == 0 {
		if s != statusOK {
			return &UnexpectedStatusError{acceptStatus, s}
		}
	} else {
		success := false
		for _, status := range acceptStatus {
			if s == status {
				success = true
				break
			}
		}
		if !success {
			return &UnexpectedStatusError{acceptStatus, s}
		}
	}
	return nil
}

// readString reads a message from r and returns it as a string.
// It will report an error for any status code that is not listed
// in acceptStatus or if none supplied 'OKAY'.
func readBytes(r io.Reader, acceptStatus ...string) ([]byte, error) {
	head := make([]byte, 8)
	_, err := io.ReadFull(r, head)
	if err != nil {
		return nil, err
	}
	status := string(head[:4])

	if status == statusFail {
		buf := make([]byte, binary.LittleEndian.Uint32(head[4:]))
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(buf))
	}

	if len(acceptStatus) == 0 {
		if status != statusOK {
			return nil, &UnexpectedStatusError{acceptStatus, status}
		}
	} else {
		ok := false
		for _, s := range acceptStatus {
			if s == status {
				ok = true
				break
			}
		}
		if !ok {
			return nil, &UnexpectedStatusError{acceptStatus, status}
		}
	}
	buf := make([]byte, binary.LittleEndian.Uint32(head[4:]))
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func requestResponseBytes(address, msg string) ([]byte, error) {
	conn, err := dial(address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	err = sendMessage(conn, msg)
	if err != nil {
		return nil, err
	}

	return readBytes(conn)
}

func send(address, msg string) error {
	conn, err := dial(address)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	err = sendMessage(conn, msg)
	if err != nil {
		return err
	}

	return wantStatus(conn)
}
