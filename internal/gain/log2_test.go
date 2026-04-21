package gain

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
)

// approxEqual checks |got-want| <= tol.
func approxEqualLog(t *testing.T, name string, got, want, tol fixed.Word32) {
	t.Helper()
	d := got - want
	if d < 0 {
		d = -d
	}
	if d > tol {
		t.Errorf("%s: got %d, want %d (±%d)", name, got, want, tol)
	}
}

func TestLog2Fixed_PowersOfTwo(t *testing.T) {
	cases := []struct {
		x    fixed.Word32
		want fixed.Word32 // log2(x) in Q10
	}{
		{1, 0},
		{2, 1024},
		{4, 2048},
		{1024, 10240},
		{1 << 20, 20480},
		{1 << 30, 30720},
	}
	for _, c := range cases {
		got := log2Fixed(c.x)
		approxEqualLog(t, "log2", got, c.want, 4)
	}
}

func TestLog2Fixed_NonPowers(t *testing.T) {
	// log2(3) ≈ 1.585  → Q10 ≈ 1623
	// log2(1.5) = 0.585 → Q10 ≈ 599 — but log2Fixed wants integer arg
	// log2(10) ≈ 3.3219 → Q10 ≈ 3402
	// log2(1000) ≈ 9.9658 → Q10 ≈ 10205
	cases := []struct {
		x    fixed.Word32
		want fixed.Word32
		tol  fixed.Word32
	}{
		{3, 1623, 8},
		{10, 3402, 8},
		{1000, 10205, 8},
		{1 << 15, 15360, 4},
	}
	for _, c := range cases {
		got := log2Fixed(c.x)
		approxEqualLog(t, "log2", got, c.want, c.tol)
	}
}

func TestLog2Fixed_NonPositive(t *testing.T) {
	if got := log2Fixed(0); got != 0 {
		t.Errorf("log2(0) = %d, want 0", got)
	}
	if got := log2Fixed(-5); got != 0 {
		t.Errorf("log2(-5) = %d, want 0", got)
	}
}
