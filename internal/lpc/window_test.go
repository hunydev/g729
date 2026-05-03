package lpc

import (
	"math"
	"testing"
)

// oracleHammingCosine computes w_lp(n) per ITU-T G.729 (06/2012)
// §3.2.1 eq. 3 in Q15 as a free-floating float64 reference. This
// oracle is intentionally test-only — production carries the literal
// LUT generated from the same recipe (Phase 1a CosLSP precedent).
func oracleHammingCosine(n int) int16 {
	var v float64
	switch {
	case n < 200:
		v = 0.54 - 0.46*math.Cos(2.0*math.Pi*float64(n)/399.0)
	default:
		v = math.Cos(2.0*math.Pi*float64(n-200)/159.0)
	}
	q := math.Round(v * 32768.0)
	if q > 32767 {
		q = 32767
	}
	if q < -32768 {
		q = -32768
	}
	return int16(q)
}

func TestLPAnalysisWindow_Length(t *testing.T) {
	if got := len(lpAnalysisWindow); got != 240 {
		t.Fatalf("lpAnalysisWindow length = %d, want 240", got)
	}
}

func TestLPAnalysisWindow_HandComputedSamples(t *testing.T) {
	// Hand-computed checkpoints per §3.2.1 eq. 3 with absolute
	// tolerance ±2 in Q15 (matches plan §2 Task 2a-W-1 step 1).
	cases := []struct {
		n    int
		want int16
	}{
		{0, 2621},   // 0.08 * 32768 ≈ 2621
		{100, 17754}, // 0.54 - 0.46·cos(2π·100/399) ≈ 0.5418
		{199, 32767}, // ≈ 0.99999 → saturates
		{200, 32767}, // cos(0) = 1.0 → saturates
	}
	for _, tc := range cases {
		got := lpAnalysisWindow[tc.n]
		diff := int(got) - int(tc.want)
		if diff < -2 || diff > 2 {
			t.Errorf("lpAnalysisWindow[%d] = %d, want %d (±2)", tc.n, got, tc.want)
		}
	}
}

func TestLPAnalysisWindow_OracleAllIndices(t *testing.T) {
	for n := 0; n < 240; n++ {
		want := oracleHammingCosine(n)
		got := lpAnalysisWindow[n]
		diff := int(got) - int(want)
		if diff < -2 || diff > 2 {
			t.Errorf("lpAnalysisWindow[%d] = %d, oracle = %d (diff > ±2)", n, got, want)
		}
	}
}

func TestLPAnalysisWindow_MonotonicDecayOnCosineSegment(t *testing.T) {
	// On [200, 239] the window is cos(2π(n-200)/159) which is
	// strictly decreasing because (n-200)/159 ∈ [0, 39/159] ⊂ [0, 1/4]
	// so the cosine argument stays in the first quadrant.
	for n := 200; n < 239; n++ {
		if lpAnalysisWindow[n] < lpAnalysisWindow[n+1] {
			t.Errorf("non-monotonic decay at n=%d: w[%d]=%d, w[%d]=%d",
				n, n, lpAnalysisWindow[n], n+1, lpAnalysisWindow[n+1])
		}
	}
}

func TestLPAnalysisWindow_W0EqualsZeroPointZeroEight(t *testing.T) {
	// Endpoint check: w_lp(0) = 0.54 - 0.46 = 0.08; in Q15 that is
	// round(0.08 * 32768) = 2621.
	if lpAnalysisWindow[0] != 2621 {
		t.Fatalf("lpAnalysisWindow[0] = %d, want 2621", lpAnalysisWindow[0])
	}
}
