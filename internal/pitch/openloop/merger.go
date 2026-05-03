package openloop

import (
	"math/bits"

	"github.com/exedev/g729/internal/fixed"
)

// OQ-1 binding constant — pinned for INT-1 first-attempt; refine if
// plausibility <80%. See plan §9 OQ-1: §A.3.4 lines 2109-2111 prescribe
// "augmenting the normalized correlations corresponding to the lower
// delay range if their delays are submultiples of the delays in the
// higher delay range" without giving the augmentation factor. The
// first-attempt heuristic per plan §6 OL-4 is to require any higher-
// range candidate to beat the lower-range candidate by at least the
// inverse of this ratio (4/3 ≈ 1.333) on the eq. A.5 R'² score; the
// equivalent statement is "the lower-range R' must reach at least 3/4
// of the higher-range R' (in squared form, 3/4 of R'²) to win". A
// 4/3 lift is a textbook bias commonly applied to bias open-loop pitch
// estimators toward the fundamental period when sub-multiples are
// present (first-attempt heuristic per plan §6 OL-4; not lifted from
// any third-party G.729 implementation per I1).
const (
	oq1SubMultipleLiftNumerator   = 4
	oq1SubMultipleLiftDenominator = 3
	// oq1SubMultipleTolerance is the ±sample slack used by
	// isNearSubmultiple when checking whether a higher-range lag is an
	// integer multiple of a lower-range lag. A ±1 sample tolerance
	// matches the §A.3.4 third-region ±1 refinement granularity (lines
	// 2113-2114) and tolerates the open-loop estimator's natural one-
	// sample rounding error. OQ-1 binding constant — pinned for INT-1
	// first-attempt; refine if plausibility <80%.
	oq1SubMultipleTolerance = 1
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
//  2. For each higher range r ∈ {2, 3} in order, override T_op iff the
//     candidate's normalized eq. A.5 score strictly beats the current
//     T_op's score, with one of two thresholds:
//
//     - If lagR is a near sub-multiple of T_op's lag (i.e., lagR ≈
//     k·T_op for some integer k ≥ 2 within ±1 sample), T_op's score
//     is lifted by oq1SubMultipleLiftNumerator / oq1Sub-
//     MultipleLiftDenominator before comparison. The candidate must
//     beat the lifted score to win.
//     - Otherwise raw strict-greater compare on R'²: candidate must
//     have a strictly larger normalized correlation (ties keep the
//     lower-lag T_op, satisfying the §A.3.4 line 2110 lower-range
//     preference for non-sub-multiple cases).
//
// Returns T_op as int16 in [20, 143]. Pure, zero-allocation.
func mergeThreeRanges(rsq1, e1 fixed.Word32, lag1 int16, rsq2, e2 fixed.Word32, lag2 int16, rsq3, e3 fixed.Word32, lag3 int16) int16 {
	bestLag, bestR, bestE := lag1, rsq1, e1
	if shouldOverride(rsq2, e2, lag2, bestR, bestE, bestLag) {
		bestLag, bestR, bestE = lag2, rsq2, e2
	}
	if shouldOverride(rsq3, e3, lag3, bestR, bestE, bestLag) {
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
func shouldOverride(rH, eH fixed.Word32, lagH int16, rOp, eOp fixed.Word32, lagOp int16) bool {
	if isNearSubmultiple(int(lagH), int(lagOp)) {
		return liftedStrictGreater(rH, eH, rOp, eOp)
	}
	return strictGreater(rH, eH, rOp, eOp)
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
// Cross-multiplied integer form (with num = oq1SubMultipleLift-
// Numerator, den = oq1SubMultipleLiftDenominator):
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
func liftedStrictGreater(rH, eH, rOp, eOp fixed.Word32) bool {
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
	return rh*rh*int64(eOp)*int64(oq1SubMultipleLiftDenominator) >
		ro*ro*int64(eH)*int64(oq1SubMultipleLiftNumerator)
}
