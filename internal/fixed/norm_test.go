package fixed

import "testing"

func TestNormS(t *testing.T) {
	tests := []struct {
		in   Word16
		want Word16
	}{
		{0, 0},
		{1, 14},
		{2, 13},
		{Max16, 0},
		{Min16, 0},
		{-1, 15},
		{16384, 0},
		{16383, 1},
		{100, 8},
	}
	for _, tc := range tests {
		if got := NormS(tc.in); got != tc.want {
			t.Errorf("NormS(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestNormL(t *testing.T) {
	tests := []struct {
		in   Word32
		want Word16
	}{
		{0, 0},
		{1, 30},
		{Max32, 0},
		{Min32, 0},
		{-1, 31},
		{0x40000000, 0},
		{0x3FFFFFFF, 1},
		{100, 24},
	}
	for _, tc := range tests {
		if got := NormL(tc.in); got != tc.want {
			t.Errorf("NormL(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
