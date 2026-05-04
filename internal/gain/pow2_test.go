package gain

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
)

func approxEqualPow(t *testing.T, name string, got, want, tol fixed.Word32) {
	t.Helper()
	d := got - want
	if d < 0 {
		d = -d
	}
	if d > tol {
		t.Errorf("%s: got %d, want %d (±%d)", name, got, want, tol)
	}
}

func TestPow2Fixed_Integers(t *testing.T) {
	cases := []struct {
		x    fixed.Word32 // Q10
		want fixed.Word32 // 2^(x/1024) Q0
	}{
		{0, 1},
		{1024, 2},
		{2048, 4},
		{10240, 1024},
		{20480, 1 << 20},
	}
	for _, c := range cases {
		got := pow2Fixed(c.x)
		approxEqualPow(t, "pow2", got, c.want, 4)
	}
}

func TestPow2Fixed_Fractional(t *testing.T) {
	// 2^0.5 ≈ 1.4142 → at Q10 input 512: result=2^(512/1024)=√2≈1
	// To get meaningful Q0 output: 2^10.5 ≈ 1448
	cases := []struct {
		x    fixed.Word32
		want fixed.Word32
		tol  fixed.Word32
	}{
		{10240 + 512, 1448, 4}, // 2^10.5
		{10240 + 256, 1217, 4}, // 2^10.25
		{20480 + 512, 1482910, 256},
	}
	for _, c := range cases {
		got := pow2Fixed(c.x)
		approxEqualPow(t, "pow2", got, c.want, c.tol)
	}
}

func TestPow2Fixed_RoundTripWithLog2(t *testing.T) {
	// pow2Fixed(log2Fixed(x)) ≈ x within a few percent for moderate x
	for _, x := range []fixed.Word32{1, 8, 100, 1024, 12345, 1 << 20} {
		round := pow2Fixed(log2Fixed(x))
		// allow ~1% tolerance from interpolation in both directions
		tol := x / 64
		if tol < 4 {
			tol = 4
		}
		approxEqualPow(t, "pow(log(x))", round, x, tol)
	}
}

func TestPow2Fixed_Negative(t *testing.T) {
	// 2^-1 = 0.5 → in Q0 truncates to 0
	if got := pow2Fixed(-1024); got != 0 {
		t.Errorf("pow2(-1024) = %d, want 0", got)
	}
	if got := pow2Fixed(-100000); got != 0 {
		t.Errorf("pow2(very negative) = %d, want 0", got)
	}
}
