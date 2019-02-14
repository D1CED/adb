package wire

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func assertEOF(t *testing.T, s Scanner) {
	msg, err := s.ReadMessage()
	assert.True(t, errors.Cause(err) == io.EOF)
	assert.Nil(t, msg)
}

func assertNotEOF(t *testing.T, r io.Reader) {
	n, err := r.Read(make([]byte, 1))
	assert.Equal(t, 1, n)
	assert.NoError(t, err)
}

// newEOFReader returns an io.ReadCloser of str that returns an EOF error
// at the end of input, instead of just returning 0 bytes read.
func newEOFReader(str string) io.ReadCloser {
	limitReader := io.LimitReader(bytes.NewBufferString(str), int64(len(str)))
	bufReader := bufio.NewReader(limitReader)
	return ioutil.NopCloser(bufReader)
}

func TestReadStatusOkay(t *testing.T) {
	s := Scanner{newEOFReader("OKAY~")}
	err := s.ReadStatusError()
	assert.NoError(t, err)
	assertNotEOF(t, s)
}

func TestReadIncompleteStatus(t *testing.T) {
	s := Scanner{newEOFReader("oka")}
	err := s.ReadStatusError()
	assert.EqualError(t, err, "NetworkError: error reading status for ")
	assert.Equal(t, errIncompleteMessage("", 3, 4), err.(*errors.Err).Cause)
	assertEOF(t, s)
}

func TestReadFailureIncompleteStatus(t *testing.T) {
	s := newEOFReader("FAIL")
	_, err := readStatusFailureAsError(s, "req", readHexLength)
	assert.EqualError(t, err, "NetworkError: server returned error for req, but couldn't read the error message")
	assert.Error(t, err.(*errors.Err).Cause)
	assertEOF(t, s)
}

func TestReadFailureEmptyStatus(t *testing.T) {
	s := newEOFReader("FAIL0000")
	_, err := readStatusFailureAsError(s, "", readHexLength)
	assert.EqualError(t, err, "AdbError: server error:  ({Request: ServerMsg:})")
	assert.NoError(t, err.(*errors.Err).Cause)
	assertEOF(t, s)
}

func TestReadFailureStatus(t *testing.T) {
	s := newEOFReader("FAIL0004fail")
	_, err := readStatusFailureAsError(s, "", readHexLength)
	assert.EqualError(t, err, "AdbError: server error: fail ({Request: ServerMsg:fail})")
	assert.NoError(t, err.(*errors.Err).Cause)
	assertEOF(t, s)
}

func TestReadMessage(t *testing.T) {
	s := newEOFReader("0005hello")
	msg, err := readMessage(s, readHexLength)
	assert.NoError(t, err)
	assert.Len(t, msg, 5)
	assert.Equal(t, "hello", string(msg))
	assertEOF(t, s)
}

func TestReadMessageWithExtraData(t *testing.T) {
	s := newEOFReader("0005hellothere")
	msg, err := readMessage(s, readHexLength)
	assert.NoError(t, err)
	assert.Len(t, msg, 5)
	assert.Equal(t, "hello", string(msg))
	assertNotEOF(t, s)
}

func TestReadLongerMessage(t *testing.T) {
	s := newEOFReader("001b192.168.56.101:5555	device\n")
	msg, err := readMessage(s, readHexLength)
	assert.NoError(t, err)
	assert.Len(t, msg, 27)
	assert.Equal(t, "192.168.56.101:5555	device\n", string(msg))
	assertEOF(t, s)
}

func TestReadEmptyMessage(t *testing.T) {
	s := newEOFReader("0000")
	msg, err := readMessage(s, readHexLength)
	assert.NoError(t, err)
	assert.Equal(t, "", string(msg))
	assertEOF(t, s)
}

func TestReadIncompleteMessage(t *testing.T) {
	s := newEOFReader("0005hel")
	msg, err := readMessage(s, readHexLength)
	assert.Error(t, err)
	assert.Equal(t, errIncompleteMessage("message data", 3, 5), err)
	assert.Equal(t, "hel\000\000", string(msg))
	assertEOF(t, s)
}

func TestReadLength(t *testing.T) {
	s := newEOFReader("000a")
	l, err := readHexLength(s)
	assert.NoError(t, err)
	assert.Equal(t, 10, l)
	assertEOF(t, s)
}

func TestReadLengthIncompleteLength(t *testing.T) {
	s := newEOFReader("aaa")
	_, err := readHexLength(s)
	assert.Equal(t, errIncompleteMessage("length", 3, 4), err)
	assertEOF(t, s)
}
