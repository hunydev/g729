package fixed

import "testing"

func TestDivS(t *testing.T) {
	tests := []struct {
		name     string
		num, den Word16
		want     Word16
	}{
		{"zero over anything", 0, 1, 0},
		{"equal", 1000, 1000, Max16},
		{"half", 16384, 32768 - 1, 16384},
		{"quarter", 8192, 32767, 8192},
		{"one third approx", 10922, 32766, 10922},
		{"num > den returns max", 100, 50, Max16},
		{"num negative returns max", -1, 100, Max16},
		{"den zero returns max", 100, 0, Max16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DivS(tc.num, tc.den)
			diff := int32(got) - int32(tc.want)
			if diff < 0 {
				diff = -diff
			}
			if diff > 1 {
				t.Errorf("DivS(%d, %d) = %d, want %d (+/-1)", tc.num, tc.den, got, tc.want)
			}
		})
	}
}

func TestDivSExactCases(t *testing.T) {
	cases := []struct {
		num, den Word16
		want     Word16
	}{
		{0, 1, 0},
		{0, 32767, 0},
		{1, 1, Max16},
		{100, 100, Max16},
	}
	for _, tc := range cases {
		if got := DivS(tc.num, tc.den); got != tc.want {
			t.Errorf("DivS(%d, %d) = %d, want %d", tc.num, tc.den, got, tc.want)
		}
	}
}
