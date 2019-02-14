package wire

import (
	"bytes"
	"reflect"
	"testing"
)

func TestConnReadWrite(t *testing.T) {
	tests := []struct {
		inp  []byte
		want []byte
	}{{
		[]byte("hello, world!"), []byte("000dhello, world!"),
	}, {
		[]byte("!"), []byte("0001!"),
	}}
	for _, test := range tests {
		buf := &bytes.Buffer{}
		c := Conn{ReadWriter: buf}
		c.Write(test.inp)
		if !reflect.DeepEqual(buf.Bytes(), test.want) {
			t.Errorf("want %s, got %s", test.want, buf.Bytes())
		}
		buf2 := &bytes.Buffer{}
		n, _ := c.WriteTo(buf2)
		if int(n) != len(test.inp) {
			t.Errorf("mismatched length; want %d, got %d", len(test.inp), n)
		}
		if !reflect.DeepEqual(buf2.Bytes(), test.inp) {
			t.Errorf("want %s, got %s", test.inp, buf2.Bytes())
		}
	}
}
