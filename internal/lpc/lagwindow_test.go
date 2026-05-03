package lpc

import (
	"math"
	"testing"
)

// TestLagWindowLUT_MatchesSpecOracle asserts the precomputed Q15
// LUT entries match exp(-0.5·(2π·60·k/8000)²) for k=1..10 within
// ±2 (§3.2.1 eq. 6, lines 692–699). The LUT is indexed lagWindow[k-1].
func TestLagWindowLUT_MatchesSpecOracle(t *testing.T) {
	for k := 1; k <= 10; k++ {
		arg := 2 * math.Pi * 60 * float64(k) / 8000
		want := int32(math.Round(math.Exp(-0.5*arg*arg) * 32768))
		got := int32(lagWindow[k-1])
		if d := got - want; d > 2 || d < -2 {
			t.Errorf("lagWindow[%d] = %d, want %d (±2)", k-1, got, want)
		}
	}
}

// TestApplyLagWindow_FlatInputDividesByLUT asserts that for a flat
// r[k] = 1<<24 input, applyLagWindow scales each k=1..10 entry by
// lagWindow[k-1]/2^15 (§3.2.1 eq. 6).
func TestApplyLagWindow_FlatInputDividesByLUT(t *testing.T) {
	var r [11]int32
	for k := 0; k <= 10; k++ {
		r[k] = 1 << 24
	}
	applyLagWindow(&r)
	for k := 1; k <= 10; k++ {
		want := int32((int64(1<<24) * int64(lagWindow[k-1])) >> 15)
		if r[k] != want {
			t.Errorf("r[%d] = %d, want %d", k, r[k], want)
		}
	}
}

// TestApplyLagWindow_NoiseFloor asserts r'(0) = r(0) + r(0)>>13,
// the §3.2.1 eq. 7 white-noise correction approximation of
// ×1.0001 (1 + 2^-13 ≈ 1.000122; spec constant 1.0001 = 1 + 1·10^-4
// rounded up to the nearest dyadic ratio).
func TestApplyLagWindow_NoiseFloor(t *testing.T) {
	cases := []int32{0, 1, 1 << 13, 1 << 24, 1<<31 - 1}
	for _, r0 := range cases {
		var r [11]int32
		r[0] = r0
		applyLagWindow(&r)
		// Saturating add models §3.2.1 eq. 7 on the AC-1 shared
		// Word32 scale; for r0 = MaxInt32 the exact sum overflows
		// and must clamp.
		var want int32
		sum := int64(r0) + int64(r0>>13)
		switch {
		case sum > math.MaxInt32:
			want = math.MaxInt32
		case sum < math.MinInt32:
			want = math.MinInt32
		default:
			want = int32(sum)
		}
		if r[0] != want {
			t.Errorf("r0=%d: got r'(0)=%d, want %d", r0, r[0], want)
		}
	}
}
