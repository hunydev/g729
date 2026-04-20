package fixed

import "testing"

func TestLAdd(t *testing.T) {
	tests := []struct {
		name string
		a, b Word32
		want Word32
	}{
		{"zero", 0, 0, 0},
		{"small", 100, 200, 300},
		{"negative", -100, -200, -300},
		{"saturate high", 2_000_000_000, 2_000_000_000, Max32},
		{"saturate low", -2_000_000_000, -2_000_000_000, Min32},
		{"max + 1 saturates", Max32, 1, Max32},
		{"min + -1 saturates", Min32, -1, Min32},
		{"min + max", Min32, Max32, -1},
		{"mixed signs no saturation", Max32, -100, Max32 - 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LAdd(tc.a, tc.b); got != tc.want {
				t.Errorf("LAdd(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestLSub(t *testing.T) {
	tests := []struct {
		name string
		a, b Word32
		want Word32
	}{
		{"zero", 0, 0, 0},
		{"small", 300, 100, 200},
		{"saturate high", Max32, Min32, Max32},
		{"saturate low", Min32, Max32, Min32},
		{"0 - min saturates", 0, Min32, Max32},
		{"min - 0", Min32, 0, Min32},
		{"a - a", 123456, 123456, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LSub(tc.a, tc.b); got != tc.want {
				t.Errorf("LSub(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestLNegate(t *testing.T) {
tests := []struct {
in, want Word32
}{
{0, 0},
{1, -1},
{-1, 1},
{Max32, -Max32},
{Min32, Max32},
}
for _, tc := range tests {
if got := LNegate(tc.in); got != tc.want {
t.Errorf("LNegate(%d) = %d, want %d", tc.in, got, tc.want)
}
}
}

func TestLAbs(t *testing.T) {
tests := []struct {
in, want Word32
}{
{0, 0},
{100, 100},
{-100, 100},
{Max32, Max32},
{Min32, Max32},
}
for _, tc := range tests {
if got := LAbs(tc.in); got != tc.want {
t.Errorf("LAbs(%d) = %d, want %d", tc.in, got, tc.want)
}
}
}
