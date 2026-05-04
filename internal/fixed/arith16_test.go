package fixed

import "testing"

func TestAdd(t *testing.T) {
	tests := []struct {
		name string
		a, b Word16
		want Word16
	}{
		{"zero+zero", 0, 0, 0},
		{"pos+pos", 100, 200, 300},
		{"pos+neg", 100, -50, 50},
		{"neg+neg", -100, -200, -300},
		{"saturate high", 30000, 30000, Max16},
		{"saturate low", -30000, -30000, Min16},
		{"at max no overflow", Max16, 0, Max16},
		{"max + 1 saturates", Max16, 1, Max16},
		{"min - 1 saturates", Min16, -1, Min16},
		{"min + max is -1", Min16, Max16, -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Add(tc.a, tc.b); got != tc.want {
				t.Errorf("Add(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestSub(t *testing.T) {
	tests := []struct {
		name string
		a, b Word16
		want Word16
	}{
		{"zero-zero", 0, 0, 0},
		{"pos-pos", 200, 50, 150},
		{"neg-pos", -100, 50, -150},
		{"pos-neg", 100, -50, 150},
		{"saturate high", Max16, Min16, Max16},
		{"saturate low", Min16, Max16, Min16},
		{"a - a", 1234, 1234, 0},
		{"min - 0", Min16, 0, Min16},
		{"0 - min saturates", 0, Min16, Max16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sub(tc.a, tc.b); got != tc.want {
				t.Errorf("Sub(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestNegate(t *testing.T) {
	tests := []struct {
		in, want Word16
	}{
		{0, 0},
		{1, -1},
		{-1, 1},
		{100, -100},
		{-100, 100},
		{Max16, -Max16},
		{Min16, Max16},
	}
	for _, tc := range tests {
		if got := Negate(tc.in); got != tc.want {
			t.Errorf("Negate(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestAbsS(t *testing.T) {
	tests := []struct {
		in, want Word16
	}{
		{0, 0},
		{100, 100},
		{-100, 100},
		{Max16, Max16},
		{Min16, Max16},
		{-1, 1},
	}
	for _, tc := range tests {
		if got := AbsS(tc.in); got != tc.want {
			t.Errorf("AbsS(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
