package wire

import (
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
)

// StatusCodes are returned by the server. If the code indicates failure, the
// next message will be the error.
type StatusCode string

const (
	StatusSuccess StatusCode = "OKAY"
	StatusFailure            = "FAIL"
	StatusNone               = ""
)

func (status StatusCode) IsSuccess() bool {
	return status == StatusSuccess
}

/*
Scanner reads tokens from a server.
See Conn for more details.
*/
type Scanner interface {
	ReadStatus() (StatusCode, error)
	ReadMessage() ([]byte, error)
	ReadUntilEof() ([]byte, error)

	NewSyncScanner() SyncScanner

	Close() error
}

type realScanner struct {
	reader io.ReadCloser
}

func NewScanner(r io.ReadCloser) Scanner {
	return &realScanner{r}
}

func ReadMessageString(s Scanner) (string, error) {
	msg, err := s.ReadMessage()
	if err != nil {
		return string(msg), err
	}
	return string(msg), nil
}

func (s *realScanner) ReadStatus() (StatusCode, error) {
	status := make([]byte, 4)
	n, err := io.ReadFull(s.reader, status)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", err
	} else if err == io.ErrUnexpectedEOF {
		return StatusCode(status), incompleteMessage("status", n, 4)
	}

	return StatusCode(status), nil
}

func (s *realScanner) ReadMessage() ([]byte, error) {
	length, err := s.readLength()
	if err != nil {
		return nil, err
	}

	data := make([]byte, length)
	n, err := io.ReadFull(s.reader, data)
	if err != nil && err != io.ErrUnexpectedEOF {
		return data, fmt.Errorf("error reading message data: %v", err)
	} else if err == io.ErrUnexpectedEOF {
		return data, incompleteMessage("message data", n, length)
	}
	return data, nil
}

func (s *realScanner) ReadUntilEof() ([]byte, error) {
	return ioutil.ReadAll(s.reader)
}

func (s *realScanner) NewSyncScanner() SyncScanner {
	return NewSyncScanner(s.reader)
}

// Reads the status, and if failure, reads the message and returns it as an error.
// If the status is success, doesn't read the message.
// req is just used to populate the AdbError, and can be nil.
func ReadStatusFailureAsError(s Scanner, req []byte) error {
	status, err := s.ReadStatus()
	if err != nil {
		return err
	}

	if !status.IsSuccess() {
		msg, err := s.ReadMessage()
		if err != nil {
			return err
		}

		return &AdbError{
			Request:   req,
			ServerMsg: string(msg),
		}
	}

	return nil
}

func (s *realScanner) Close() error {
	return s.reader.Close()
}

func (s *realScanner) readLength() (int, error) {
	lengthHex := make([]byte, 4)
	n, err := io.ReadFull(s.reader, lengthHex)
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0, err
	} else if err == io.ErrUnexpectedEOF {
		return 0, incompleteMessage("length", n, 4)
	}

	length, err := strconv.ParseInt(string(lengthHex), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hex length: %v", err)
	}

	// Clip the length to 255, as per the Google implementation.
	if length > MaxMessageLength {
		length = MaxMessageLength
	}

	return int(length), nil
}

var _ Scanner = &realScanner{}