package wire

import "testing"

func TestTetraToUint32(t *testing.T) {
	tests := []struct {
		inp  [4]byte
		want uint32
	}{{
		[4]byte{0, 0, 0, 0}, 0,
	}, {
		[4]byte{1, 0, 0, 0}, 1,
	}, {
		[4]byte{2, 0, 0, 0}, 2,
	}, {
		[4]byte{100, 0, 0, 0}, 100,
	}, {
		[4]byte{255, 0, 0, 0}, 255,
	}, {
		[4]byte{0, 1, 0, 0}, 1 << 8,
	}, {
		[4]byte{0, 0, 0, 1}, 1 << 24,
	}, {
		[4]byte{255, 255, 255, 255}, (1 << 32) - 1,
	}}
	for _, test := range tests {
		got := tetraToUint32(test.inp)
		if test.want != got {
			t.Errorf("for %v, want %d, got %d", test.inp, test.want, got)
		}
		back := uint32ToTetra(got)
		if test.inp != back {
			t.Errorf("want %v, got %v", test.inp, back)
		}
	}
}
