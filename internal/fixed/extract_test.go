package fixed

import "testing"

func TestExtractH(t *testing.T) {
	tests := []struct {
		in   Word32
		want Word16
	}{
		{0, 0},
		{0x00010000, 1},
		{0x7FFF0000, Max16},
		{-0x00010000, -1},
		{0x7FFFFFFF, Max16},
		{Min32, Min16},
	}
	for _, tc := range tests {
		if got := ExtractH(tc.in); got != tc.want {
			t.Errorf("ExtractH(%#x) = %d, want %d", uint32(tc.in), got, tc.want)
		}
	}
}

func TestExtractL(t *testing.T) {
	tests := []struct {
		in   Word32
		want Word16
	}{
		{0, 0},
		{1, 1},
		{-1, -1},
		{0x00007FFF, 32767},
		{0x00008000, -32768},
		{int32(int64(0x80008000) - int64(1)<<32), -32768},
		{0x12345678, 0x5678},
	}
	for _, tc := range tests {
		if got := ExtractL(tc.in); got != tc.want {
			t.Errorf("ExtractL(%#x) = %d, want %d", uint32(tc.in), got, tc.want)
		}
	}
}

func TestLDepositH(t *testing.T) {
	tests := []struct {
		in   Word16
		want Word32
	}{
		{0, 0},
		{1, 0x00010000},
		{-1, int32(-0x00010000)},
		{Max16, 0x7FFF0000},
		{Min16, Min32},
	}
	for _, tc := range tests {
		if got := LDepositH(tc.in); got != tc.want {
			t.Errorf("LDepositH(%d) = %#x, want %#x", tc.in, uint32(got), uint32(tc.want))
		}
	}
}

func TestLDepositL(t *testing.T) {
	tests := []struct {
		in   Word16
		want Word32
	}{
		{0, 0},
		{1, 1},
		{-1, -1},
		{Max16, 32767},
		{Min16, -32768},
	}
	for _, tc := range tests {
		if got := LDepositL(tc.in); got != tc.want {
			t.Errorf("LDepositL(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
