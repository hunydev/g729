package fixed

import "testing"

func TestSaturate(t *testing.T) {
	tests := []struct {
		name string
		in   Word32
		want Word16
	}{
		{"zero", 0, 0},
		{"small positive", 100, 100},
		{"small negative", -100, -100},
		{"at max16", 32767, 32767},
		{"at min16", -32768, -32768},
		{"above max16", 32768, Max16},
		{"far above max16", 2000000000, Max16},
		{"below min16", -32769, Min16},
		{"far below min16", -2000000000, Min16},
		{"max32", Max32, Max16},
		{"min32", Min32, Min16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Saturate(tc.in); got != tc.want {
				t.Errorf("Saturate(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
