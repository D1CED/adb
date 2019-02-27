package adb

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
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
		b := new(bytes.Buffer)
		err := sendMessage(b, test.in)
		if err != nil {
			t.Errorf("got unexpected error: %v", err)
		}
		if b.String() != test.out {
			t.Errorf("want %s, got %s", test.out, b.String())
		}
		// r := strings.NewReader(test.out)
		// if s, err := readBytes(r); string(s) != test.in {
		//	t.Errorf("want %s, got %s, with err: %v", test.in, s, err)
		// }
	}
}

func TestWantStatus(t *testing.T) {
	var compErrors = func(l, r error) bool {
		if l == nil && r == nil {
			return true
		} else if l == nil || r == nil {
			return false
		} else {
			return l.Error() == r.Error()
		}
	}

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
		err := wantStatus(conn, test.status)
		if !compErrors(err, test.err) {
			t.Errorf("want '%v', got '%v'", test.err, err)
		}
	}
}

func TestServerVersion(t *testing.T) {
	d := dial
	dial = mockDial(t, "000chost:version", "OKAY00040002")
	defer func() { dial = d }()

	s := &Server{
		path:    "mock-path",
		address: "mock-address",
	}
	v, err := s.Version()
	if err != nil || v != 2 {
		t.Errorf("%d - %d, err: %v", v, 2, err)
	}
}
