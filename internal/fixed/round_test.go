package fixed

import "testing"

func TestRound(t *testing.T) {
	tests := []struct {
		name string
		in   Word32
		want Word16
	}{
		{"zero", 0, 0},
		{"small rounds to zero", 0x00007FFF, 0},
		{"half rounds up", 0x00008000, 1},
		{"just above one high", 0x00018000, 2},
		{"neg half rounds toward zero", -0x00008000, 0},
		{"neg rounds negative", -0x00018000, -1},
		{"max32 saturates", Max32, Max16},
		{"min32 stays min16", Min32, Min16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Round(tc.in); got != tc.want {
				t.Errorf("Round(%#x) = %d, want %d", uint32(tc.in), got, tc.want)
			}
		})
	}
}
