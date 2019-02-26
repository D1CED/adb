package adb

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSendReadMessage(t *testing.T) {
	var tests = []struct {
		in, out string
	}{{
		"A", "0001A",
	}, {
		"Hello, World!", "000dHello, World!",
	}, {
		"Siebenhundertsiebenundzwanzig", "001dSiebenhundertsiebenundzwanzig",
	}}
	for _, test := range tests {
		b := &bytes.Buffer{}
		_, err := sendMessage(b, test.in)
		if err != nil {
			t.Errorf("got unexpected error: %v", err)
		}
		if b.String() != test.out {
			t.Errorf("want %s, got %s", test.out, b.String())
		}
		r := strings.NewReader(test.out)
		if s, _ := readMessage(r); s != test.in {
			t.Errorf("want %s, got %s", test.in, s)
		}
	}
}

func TestInt32ToTetra(t *testing.T) {
	var tests = []struct {
		tetra [4]byte
		i     int32
	}{{
		[4]byte{1, 0, 0, 0}, 1,
	}, {
		[4]byte{2, 0, 0, 0}, 2,
	}, {
		[4]byte{100, 0, 0, 0}, 100,
	}, {
		[4]byte{255, 0, 0, 0}, 255,
	}, {
		[4]byte{0, 1, 0, 0}, 256,
	}, {
		[4]byte{1, 1, 0, 0}, 257,
	}, {
		[4]byte{255, 255, 255, 1}, 0x01FFFFFF,
	}}
	for _, test := range tests {
		i := tetraToInt(test.tetra)
		if int32(i) != test.i {
			t.Errorf("%d, %d", i, test.i)
		}

		tetra := int32ToTetra(test.i)
		if tetra != test.tetra {
			t.Errorf("%v, %v", tetra, test.tetra)
		}
	}
}

func TestWantStatus(t *testing.T) {
	var tests = []struct {
		status string
		input  string
		err    error
	}{{
		"OKAY", "OKAY", nil,
	}, {
		"DATA", "DATA", nil,
	}, {
		"FAIL", "FAIL001aThis is the error message!", errors.New("This is the error message!"),
	}, {
		"FAIL", "FAIL001bThis is the error message!", io.ErrUnexpectedEOF,
	}}
	for _, test := range tests {
		conn := strings.NewReader(test.input)
		err := wantStatus(test.status, conn)
		if err != test.err {
			t.Errorf("want '%v', got '%v'", test.err, err)
		}
	}
}

func TestSendTime(t *testing.T) {
	var tests = []struct {
		time         time.Time
		intermediate int32
	}{{
		time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), 0,
	}, {
		time.Date(1970, 1, 1, 0, 0, 1, 0, time.UTC), 1,
	}, {
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), 0,
	}}
	b := &bytes.Buffer{}
	for _, test := range tests {
		_ = sendTime(b, test.time)
		te, _ := readTetra(b)
		ti := tetraToTime(te)
		if test.time != ti {
			t.Errorf("want eq: %v == %v", test.time, ti)
		}
	}
}

func TestFileMode(t *testing.T) {
	var tests = []struct {
		fmode os.FileMode
	}{{
		0777,
	}}
	b := &bytes.Buffer{}
	for _, test := range tests {
		te := int32ToTetra(int32(test.fmode))
		n, _ := b.Write(te[:])
		if n != 4 {
			t.Fail()
		}
	}
}

func TestServerVersion(t *testing.T) {
	var s = Server{
		path:    "mock-path",
		address: "mock-address",
		dial:    mockDial(t, "000chost:version", "OKAY00040002"),
	}
	v, err := s.Version()
	if err != nil {
		t.Error(err)
	}
	if v != 2 {
		t.Errorf("%d - %d", v, 2)
	}
}
