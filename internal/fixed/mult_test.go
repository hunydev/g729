package fixed

import "testing"

func TestLMult(t *testing.T) {
	tests := []struct {
		name string
		a, b Word16
		want Word32
	}{
		{"zero", 0, 0, 0},
		{"zero times anything", 0, 12345, 0},
		{"one times one", 1, 1, 2},
		{"pos * pos", 100, 200, 40_000},
		{"pos * neg", 100, -200, -40_000},
		{"max * one", Max16, 1, 2 * int32(Max16)},
		{"max * max", Max16, Max16, 2 * int32(Max16) * int32(Max16)},
		{"min * min saturates", Min16, Min16, Max32},
		{"min * max", Min16, Max16, 2 * int32(Min16) * int32(Max16)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LMult(tc.a, tc.b); got != tc.want {
				t.Errorf("LMult(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
