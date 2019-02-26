package adb

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
)

type mockAddr string

func (a mockAddr) String() string  { return string(a[3:]) }
func (a mockAddr) Network() string { return string(a[:3]) }

type mockConn struct {
	out      []byte
	in       []byte
	addr     string
	open     bool
	mtx      sync.Mutex
	deadline time.Time
	t        *testing.T
}

func mockDial(t *testing.T, in, out string) func(n, a string) (net.Conn, error) {
	return func(network, address string) (net.Conn, error) {
		return &mockConn{
			out:  []byte(out),
			in:   []byte(in),
			addr: address,
			open: true,
			mtx:  sync.Mutex{},
			t:    t,
		}, nil
	}
}

func (mc *mockConn) Read(b []byte) (int, error) {
	if time.Now().After(mc.deadline) {
		return 0, errors.New("timeout")
	}
	if len(mc.out) == 0 {
		return 0, io.EOF
	}
	mc.mtx.Lock()
	n := copy(b, mc.out)
	mc.out = mc.out[n:]
	mc.mtx.Unlock()
	return n, nil
}

func (mc *mockConn) Write(b []byte) (int, error) {
	if time.Now().After(mc.deadline) {
		return 0, errors.New("timeout")
	}
	mc.mtx.Lock()
	for i, v := range b {
		if len(mc.in) == 0 {
			return i, io.EOF
		}
		if v == mc.in[0] {
			mc.in = mc.in[1:]
		} else {
			mc.t.Errorf("mismatched write. Want %x, got %v", mc.in[0], b)
		}
	}
	mc.mtx.Unlock()
	return len(b), nil
}

func (mc *mockConn) Close() error {
	if !mc.open {
		return errors.New("conn double close")
	}
	mc.mtx.Lock()
	mc.open = false
	mc.mtx.Unlock()
	return nil
}

func (mc *mockConn) LocalAddr() net.Addr  { return mockAddr("tcp0.0.0.0") }
func (mc *mockConn) RemoteAddr() net.Addr { return mc.LocalAddr() }

func (mc *mockConn) SetDeadline(t time.Time) error {
	if time.Now().After(t) {
		return errors.New("time past")
	}
	mc.mtx.Lock()
	mc.deadline = t
	mc.mtx.Unlock()
	return nil
}

func (mc *mockConn) SetReadDeadline(t time.Time) error  { return mc.SetDeadline(t) }
func (mc *mockConn) SetWriteDeadline(t time.Time) error { return mc.SetDeadline(t) }
