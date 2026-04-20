package fixed

import "testing"

func TestLShl(t *testing.T) {
	tests := []struct {
		name string
		a    Word32
		n    Word16
		want Word32
	}{
		{"zero shift", 1000, 0, 1000},
		{"shift left 1", 1000, 1, 2000},
		{"shift left 10", 1000, 10, 1_024_000},
		{"neg shift left", -1000, 1, -2000},
		{"saturate high", 1_000_000_000, 2, Max32},
		{"saturate low", -1_000_000_000, 2, Min32},
		{"n negative -> LShr", 1000, -1, 500},
		{"large n saturates pos", 1, 32, Max32},
		{"large n saturates neg", -1, 32, Min32},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LShl(tc.a, tc.n); got != tc.want {
				t.Errorf("LShl(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}

func TestLShr(t *testing.T) {
	tests := []struct {
		name string
		a    Word32
		n    Word16
		want Word32
	}{
		{"zero shift", 1000, 0, 1000},
		{"shift right 1", 1000, 1, 500},
		{"shift right 10", 1_024_000, 10, 1000},
		{"negative arithmetic", -2, 1, -1},
		{"n negative -> LShl", 1000, -1, 2000},
		{"n >= 31 pos", 1000000, 31, 0},
		{"n >= 31 neg", -1000000, 31, -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LShr(tc.a, tc.n); got != tc.want {
				t.Errorf("LShr(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}

func TestLShrR(t *testing.T) {
	tests := []struct {
		name string
		a    Word32
		n    Word16
		want Word32
	}{
		{"n=0", 1000, 0, 1000},
		{"rounds at half", 3, 1, 2},
		{"below half", 4, 2, 1},
		{"at half", 6, 2, 2},
		{"neg rounding", -3, 1, -1},
		{"n=31 nonneg", Max32, 31, 1},
		{"n=31 neg", Min32, 31, -1},
		{"n>31", 12345, 32, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LShrR(tc.a, tc.n); got != tc.want {
				t.Errorf("LShrR(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}
