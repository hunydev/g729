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

func TestLMac(t *testing.T) {
tests := []struct {
name string
acc  Word32
a, b Word16
want Word32
}{
{"zero acc", 0, 100, 200, 40_000},
{"acc add", 1000, 100, 200, 41_000},
{"acc subtract", 100_000, 100, -200, 60_000},
{"saturate acc high", Max32 - 10, 100, 100, Max32},
{"saturate on mult only", 0, Min16, Min16, Max32},
}
for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
if got := LMac(tc.acc, tc.a, tc.b); got != tc.want {
t.Errorf("LMac(%d, %d, %d) = %d, want %d", tc.acc, tc.a, tc.b, got, tc.want)
}
})
}
}

func TestLMsu(t *testing.T) {
tests := []struct {
name string
acc  Word32
a, b Word16
want Word32
}{
{"zero acc", 0, 100, 200, -40_000},
{"acc minus", 100_000, 100, 200, 60_000},
{"acc minus neg prod", 1000, 100, -200, 41_000},
{"saturate low", Min32 + 10, 100, 100, Min32},
}
for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
if got := LMsu(tc.acc, tc.a, tc.b); got != tc.want {
t.Errorf("LMsu(%d, %d, %d) = %d, want %d", tc.acc, tc.a, tc.b, got, tc.want)
}
})
}
}

func TestMult(t *testing.T) {
tests := []struct {
name string
a, b Word16
want Word16
}{
{"zero", 0, 0, 0},
{"half times half", 16384, 16384, 8192},
{"one-ish times max", Max16, Max16, 32766},
{"saturate", Min16, Min16, Max16},
{"pos * neg", 16384, -16384, -8192},
}
for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
if got := Mult(tc.a, tc.b); got != tc.want {
t.Errorf("Mult(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
}
})
}
}

func TestMultR(t *testing.T) {
tests := []struct {
name string
a, b Word16
want Word16
}{
{"zero", 0, 0, 0},
{"half times half rounds", 16384, 16384, 8192},
{"saturates like Mult", Min16, Min16, Max16},
{"rounds up", 32767, 2, 2},
}
for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
if got := MultR(tc.a, tc.b); got != tc.want {
t.Errorf("MultR(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
}
})
}
}
