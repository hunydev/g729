package fixed

import "testing"

func TestShl(t *testing.T) {
	tests := []struct {
		name string
		a    Word16
		n    Word16
		want Word16
	}{
		{"zero shift", 100, 0, 100},
		{"shift left 1", 100, 1, 200},
		{"shift left 3", 100, 3, 800},
		{"neg shift left 1", -100, 1, -200},
		{"saturate high", 20000, 2, Max16},
		{"saturate low", -20000, 2, Min16},
		{"max shifted saturates", Max16, 1, Max16},
		{"min shifted saturates", Min16, 1, Min16},
		{"negative n -> right shift", 100, -1, 50},
		{"negative n large -> 0", 100, -20, 0},
		{"neg a with negative n large -> -1", -100, -20, -1},
		{"large n saturates", 1, 20, Max16},
		{"large n neg saturates", -1, 20, Min16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Shl(tc.a, tc.n); got != tc.want {
				t.Errorf("Shl(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}

func TestShr(t *testing.T) {
	tests := []struct {
		name string
		a    Word16
		n    Word16
		want Word16
	}{
		{"zero shift", 100, 0, 100},
		{"shift right 1", 100, 1, 50},
		{"shift right 3", 800, 3, 100},
		{"neg shift right", -100, 1, -50},
		{"neg arithmetic", -2, 1, -1},
		{"neg close to zero", -1, 1, -1},
		{"negative n -> left shift", 100, -1, 200},
		{"n>=15 nonneg a", 32767, 15, 0},
		{"n>=15 neg a", -32768, 15, -1},
		{"n>15 neg a", -1, 20, -1},
		{"n>15 pos a", 100, 20, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Shr(tc.a, tc.n); got != tc.want {
				t.Errorf("Shr(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}

func TestShrR(t *testing.T) {
	tests := []struct {
		name string
		a, n Word16
		want Word16
	}{
		{"n=0", 100, 0, 100},
		{"rounds up exact half", 3, 1, 2},
		{"rounds down below half", 4, 2, 1},
		{"rounds up at half", 6, 2, 2},
		{"negative rounding", -3, 1, -1},
		{"large n", 100, 8, 0},
		{"large n neg", -100, 8, 0},
		{"n negative acts as Shl", 100, -1, 200},
		{"n>=15 nonneg", 32767, 15, 1},
		{"n>=15 neg", -32768, 15, -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShrR(tc.a, tc.n); got != tc.want {
				t.Errorf("ShrR(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}
