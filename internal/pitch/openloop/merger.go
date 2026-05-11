package openloop

import (
	"math/bits"

	"github.com/hunydev/g729/internal/fixed"
)

// §A.3.4 lines 2109-2111 prescribe "augmenting the normalized
// correlations corresponding to the lower delay range if their delays
// are submultiples of the delays in the higher delay range" without
// giving the augmentation factor. Core's global merge uses a modest 11/10 lift:
// enough to reject near-equal pitch multiples, but not so strong that it pins
// low-energy submultiples over clearly better mid-range candidates. The
// normalized quality heuristic keeps the older pairwise 2/1 lift because that
// path is product-tuned separately.
const (
	oq1GlobalSubMultipleLiftNumerator     = 11
	oq1GlobalSubMultipleLiftDenominator   = 10
	oq1PairwiseSubMultipleLiftNumerator   = 2
	oq1PairwiseSubMultipleLiftDenominator = 1
	// oq1SubMultipleTolerance is the ±sample slack used by
	// isNearSubmultiple when checking whether a higher-range lag is an
	// integer multiple of a lower-range lag. The ±2 sample window is looser
	// than the §A.3.4 third-region ±1 refinement granularity but absorbs more
	// of the open-loop estimator's natural rounding error around true
	// submultiples.
	oq1SubMultipleTolerance = 2
)

// mergeThreeRanges combines the §A.3.4 per-range OL-3 winners
// (rsq_i, e_i, lag_i) for i = 1, 2, 3 covering [20,39], [40,79],
// [80,143] respectively, into the final open-loop pitch lag T_op per
// G729E.txt §A.3.4 lines 2109-2111:
//
//	"The winner among the three normalized correlations is selected by
//	 favouring the delays with the values in the lower range. This is
//	 done by augmenting the normalized correlations corresponding to
//	 the lower delay range if their delays are submultiples of the
//	 delays in the higher delay range."
//
// Algorithm shape (per plan §6 OL-4 step 3, OQ-1 closure path):
//
//  1. Start T_op = lag1 (the shortest-range candidate is favored by
//     default per the "favouring the lower range" rule).
//
//  2. Range 2 may override range 1 iff its normalized eq. A.5 score
//     strictly beats range 1 after any range-1 sub-multiple lift.
//
//  3. Range 3 may override the current winner only if it beats every
//     lower-range candidate that is its near sub-multiple after applying
//     that lower candidate's lift. This keeps an intermediate range-2
//     winner from hiding a valid range-1 sub-multiple relation to range 3.
//     If no lower sub-multiple applies, range 3 uses the same strict
//     normalized-score comparison against the current winner.
//
// Returns T_op as int16 in [20, 143]. Pure, zero-allocation.
func mergeThreeRanges(rsq1, e1 fixed.Word32, lag1 int16, rsq2, e2 fixed.Word32, lag2 int16, rsq3, e3 fixed.Word32, lag3 int16) int16 {
	bestLag, bestR, bestE := lag1, rsq1, e1
	if shouldOverride(rsq2, e2, lag2, bestR, bestE, bestLag, oq1GlobalSubMultipleLiftNumerator, oq1GlobalSubMultipleLiftDenominator) {
		bestLag, bestR, bestE = lag2, rsq2, e2
	}
	if shouldOverrideRange3(rsq3, e3, lag3, bestR, bestE, bestLag, rsq1, e1, lag1, rsq2, e2, lag2) {
		bestLag = lag3
	}
	return bestLag
}

// mergeThreeRangesPairwise preserves the earlier pairwise merge used by the
// normalized-range quality heuristic. Core uses mergeThreeRanges so range 3 is
// checked against all lower sub-multiples; the heuristic path keeps its
// product-tuned behaviour separate from that spec-aligned correction.
func mergeThreeRangesPairwise(rsq1, e1 fixed.Word32, lag1 int16, rsq2, e2 fixed.Word32, lag2 int16, rsq3, e3 fixed.Word32, lag3 int16) int16 {
	bestLag, bestR, bestE := lag1, rsq1, e1
	if shouldOverride(rsq2, e2, lag2, bestR, bestE, bestLag, oq1PairwiseSubMultipleLiftNumerator, oq1PairwiseSubMultipleLiftDenominator) {
		bestLag, bestR, bestE = lag2, rsq2, e2
	}
	if shouldOverride(rsq3, e3, lag3, bestR, bestE, bestLag, oq1PairwiseSubMultipleLiftNumerator, oq1PairwiseSubMultipleLiftDenominator) {
		bestLag = lag3
	}
	return bestLag
}

// shouldOverride returns true iff the higher-range candidate
// (rH, eH, lagH) strictly beats the current T_op (rOp, eOp, lagOp).
// When lagH is a near integer multiple of lagOp the comparison applies
// the OQ-1 sub-multiple lift to T_op's score; otherwise a raw strict-
// greater compare on R'² is used. Strict-greater on the non-multiple
// path is what realises the "favour the lower range" tie rule of
// §A.3.4 line 2110 — ties keep the existing (lower) T_op.
func shouldOverride(rH, eH fixed.Word32, lagH int16, rOp, eOp fixed.Word32, lagOp int16, liftNum, liftDen int) bool {
	if isNearSubmultiple(int(lagH), int(lagOp)) {
		return liftedStrictGreater(rH, eH, rOp, eOp, liftNum, liftDen)
	}
	return strictGreater(rH, eH, rOp, eOp)
}

func shouldOverrideRange3(r3, e3 fixed.Word32, lag3 int16, rOp, eOp fixed.Word32, lagOp int16, r1, e1 fixed.Word32, lag1 int16, r2, e2 fixed.Word32, lag2 int16) bool {
	if isNearSubmultiple(int(lag3), int(lag1)) && !liftedStrictGreater(r3, e3, r1, e1, oq1GlobalSubMultipleLiftNumerator, oq1GlobalSubMultipleLiftDenominator) {
		return false
	}
	if isNearSubmultiple(int(lag3), int(lag2)) && !liftedStrictGreater(r3, e3, r2, e2, oq1GlobalSubMultipleLiftNumerator, oq1GlobalSubMultipleLiftDenominator) {
		return false
	}
	return shouldOverride(r3, e3, lag3, rOp, eOp, lagOp, oq1GlobalSubMultipleLiftNumerator, oq1GlobalSubMultipleLiftDenominator)
}

// isNearSubmultiple reports whether higher ≈ k · lower for some
// integer k ∈ [2, 7] within ±oq1SubMultipleTolerance samples. The
// upper bound k = 7 covers the [20,143] range pair extremes
// (143 / 20 ≈ 7.15); the lower bound k = 2 excludes the trivial
// k = 1 self-match and matches the spec wording "submultiples"
// (proper integer factors > 1).
func isNearSubmultiple(higher, lower int) bool {
	if lower <= 0 {
		return false
	}
	for k := 2; k <= 7; k++ {
		d := higher - k*lower
		if d < 0 {
			d = -d
		}
		if d <= oq1SubMultipleTolerance {
			return true
		}
		if k*lower > higher+oq1SubMultipleTolerance {
			return false
		}
	}
	return false
}

// strictGreater returns true iff (R'(H))² > (R'(Op))² — i.e., the
// higher-range candidate has a strictly larger eq. A.5 normalized
// correlation. Equivalent to !compareNormalized(rOp, eOp, rH, eH)
// because compareNormalized's ≥ relation is reflexive on ties.
func strictGreater(rH, eH, rOp, eOp fixed.Word32) bool {
	return !compareNormalized(rOp, eOp, rH, eH)
}

// liftedStrictGreater returns true iff R'²(H) > (num/den) · R'²(Op),
// i.e., the higher-range candidate beats the OQ-1-lifted T_op score.
// Cross-multiplied integer form:
//
//	R²(H) · E(Op) · den > R²(Op) · E(H) · num
//
// Same overflow strategy as compareNormalized: normalize both R values
// by a common right-shift so the larger fits in 13 bits; the squared
// product then fits in 26 bits, leaving 31 bits for E and 4 bits for
// the lift constant before saturating int64. Worst-case shift is
// 31 − 13 = 18 bits.
//
// Edge cases mirror compareNormalized: zero/negative scores lose to
// any positive score; if both sides are zero the lifted side does NOT
// strictly exceed, so this returns false (the existing T_op stays).
func liftedStrictGreater(rH, eH, rOp, eOp fixed.Word32, liftNum, liftDen int) bool {
	if eH <= 0 || rH <= 0 {
		return false
	}
	if eOp <= 0 || rOp <= 0 {
		return true
	}
	rh := int64(rH)
	ro := int64(rOp)
	maxR := rh
	if ro > maxR {
		maxR = ro
	}
	var s uint
	if l := bits.Len64(uint64(maxR)); l > 13 {
		s = uint(l - 13)
	}
	rh >>= s
	ro >>= s
	return rh*rh*int64(eOp)*int64(liftDen) >
		ro*ro*int64(eH)*int64(liftNum)
}
