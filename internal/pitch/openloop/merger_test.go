package openloop

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
)

// scoreTriple constructs a (rsq, e) pair such that R²/E equals the
// supplied num/den ratio (in arbitrary units), making it trivial to
// build mergeThreeRanges inputs whose normalized eq. A.5 ordering is
// fully predictable. Because compareNormalized only ever cross-
// multiplies R²·E across two candidates, scaling rsq²·e by the same
// constant on every candidate preserves the ordering.
func scoreTriple(rsq, e int32) (fixed.Word32, fixed.Word32) {
	return fixed.Word32(rsq), fixed.Word32(e)
}

// TestMergeThreeRanges_AllEqual_PicksLowestLag covers §A.3.4 lines
// 2109-2111: three identical normalized correlations at lags 20, 40,
// 80 (40 = 2·20, 80 = 4·20 — both sub-multiples of the shorter lag)
// must collapse to the [20,39] winner via the sub-multiple lift.
func TestMergeThreeRanges_AllEqual_PicksLowestLag(t *testing.T) {
	r, e := scoreTriple(1000, 1000)
	got := mergeThreeRanges(r, e, 20, r, e, 40, r, e, 80)
	if got != 20 {
		t.Fatalf("mergeThreeRanges(equal scores at 20/40/80) = %d, want 20", got)
	}
}

// TestMergeThreeRanges_LongRangeDominates covers the case where the
// [80,143] winner has a normalized correlation strong enough to beat
// the lifted shorter-range scores. With R²/E ratios 1:1:100, even the
// 4/3 OQ-1 lift cannot close the gap.
func TestMergeThreeRanges_LongRangeDominates(t *testing.T) {
	rWeak, eWeak := scoreTriple(1, 1000)     // R'² = 1e-6
	rStrong, eStrong := scoreTriple(100, 10) // R'² = 1000
	got := mergeThreeRanges(rWeak, eWeak, 25, rWeak, eWeak, 50, rStrong, eStrong, 100)
	if got != 100 {
		t.Fatalf("mergeThreeRanges(weak/weak/strong at 25/50/100) = %d, want 100", got)
	}
}

// TestMergeThreeRanges_SubMultipleLift covers the canonical OL-4
// scenario from plan §6 OL-4 step 1.3: lags 30, 60 (= 2·30), 90
// (= 3·30) with identical normalized correlations. The sub-multiple
// lift must select 30 as T_op.
func TestMergeThreeRanges_SubMultipleLift(t *testing.T) {
	r, e := scoreTriple(2000, 500)
	got := mergeThreeRanges(r, e, 30, r, e, 60, r, e, 90)
	if got != 30 {
		t.Fatalf("mergeThreeRanges(equal scores at 30/60/90) = %d, want 30", got)
	}
}

// TestMergeThreeRanges_TieBreakLowerLag covers the §A.3.4 line 2110
// "favouring the delays with the values in the lower range" rule for
// non-sub-multiple lags: when scores tie and no candidate has a near-
// integer-multiple relation, the lower-lag candidate wins.
func TestMergeThreeRanges_TieBreakLowerLag(t *testing.T) {
	r, e := scoreTriple(1500, 750)
	// 35, 73, 97 — none is a near sub-multiple of any other within the
	// ±1 OQ-1 tolerance: 73/35 ≈ 2.086 (|73-70|=3), 97/35 ≈ 2.77,
	// 97/73 ≈ 1.33.
	got := mergeThreeRanges(r, e, 35, r, e, 73, r, e, 97)
	if got != 35 {
		t.Fatalf("mergeThreeRanges(equal scores at 35/73/97) = %d, want 35", got)
	}
}

// TestMergeThreeRanges_LongRangeWinsOverNonMultiple verifies that when
// the longest-range candidate strictly dominates and the shorter-range
// lags are NOT sub-multiples, the longest-range lag is returned (no
// spurious lift activates).
func TestMergeThreeRanges_LongRangeWinsOverNonMultiple(t *testing.T) {
	rWeak, eWeak := scoreTriple(10, 1000)
	rStrong, eStrong := scoreTriple(500, 10)
	// 23 and 47 are not near sub-multiples of 100 (100/23 ≈ 4.35,
	// 100/47 ≈ 2.13, both outside the ±1 tolerance).
	got := mergeThreeRanges(rWeak, eWeak, 23, rWeak, eWeak, 47, rStrong, eStrong, 100)
	if got != 100 {
		t.Fatalf("mergeThreeRanges(non-multiple weak/weak/strong) = %d, want 100", got)
	}
}

// TestMergeThreeRanges_LiftInsufficient verifies the OQ-1 4/3 lift is
// bounded — a sufficiently strong higher-range candidate still wins
// over a sub-multiple lower-range candidate. With R'²(30) = 1 and
// R'²(90) = 4, the lifted lower score is 4/3 < 4, so 90 wins.
func TestMergeThreeRanges_LiftInsufficient(t *testing.T) {
	rLow, eLow := scoreTriple(10, 100)     // R²/E = 1
	rHigh, eHigh := scoreTriple(200, 1000) // R²/E = 40
	rMid, eMid := scoreTriple(1, 1000)     // negligible
	got := mergeThreeRanges(rLow, eLow, 30, rMid, eMid, 60, rHigh, eHigh, 90)
	if got != 90 {
		t.Fatalf("mergeThreeRanges(weak-30/0-60/strong-90) = %d, want 90", got)
	}
}

// TestMergeThreeRanges_NoAlloc enforces I4 on the merger hot path.
func TestMergeThreeRanges_NoAlloc(t *testing.T) {
	r, e := scoreTriple(2000, 500)
	allocs := testing.AllocsPerRun(1000, func() {
		_ = mergeThreeRanges(r, e, 30, r, e, 60, r, e, 90)
	})
	if allocs != 0 {
		t.Fatalf("mergeThreeRanges allocates %v/op, want 0", allocs)
	}
}
