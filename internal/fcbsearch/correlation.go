package fcbsearch

import "github.com/hunydev/g729/internal/fixed"

// AdjustedTarget computes the second target signal x'(n) of
// ITU-T G.729 §3.8.1 eq. 50 (G729E.txt lines 1245–1248):
//
//	x'(n) = x(n) − ĝp · y(n),    n = 0,...,39
//
// where x(n) is the closed-loop pitch target (Phase 2c TG-1, Q0),
// y(n) is the filtered adaptive-codebook vector (Phase 2c GP-1, Q0),
// and ĝp is the unquantized adaptive-codebook gain (Phase 2c GP-1
// output, Q14, range [0, GpUpperQ14] = [0, ~1.2)).
//
// Q-format. The product gp·y is int32 in Q14; roundShift(..., 14)
// converts it to Q0 before subtracting from x and Word16-saturating.
// This documents the current fixed-point contract explicitly: the
// conversion is rounded, not a raw arithmetic right shift.
//
// I3 / I4: pure (writes only through xPrime), zero allocation.
func AdjustedTarget(x, y *[SubframeLen]int16, gp int16, xPrime *[SubframeLen]int16) {
	for n := 0; n < SubframeLen; n++ {
		prod := int32(gp) * int32(y[n]) // Q14
		xPrime[n] = fixed.Saturate(int32(x[n]) - roundShift(prod, 14))
	}
}

// CorrelationD computes the backward-filtered correlation d(n) of
// ITU-T G.729 §3.8.1 eq. 52 (G729E.txt lines 1256–1259):
//
//	d(n) = Σ_{i=n..39} x'(i) · h(i − n),    n = 0,...,39
//
// i.e. the time-reversed correlation of the adjusted target x'(n) with
// the truncated impulse response h(n) of the weighted synthesis filter.
//
// Q-format. x' is Q0 (AdjustedTarget output) and h is Q12 (HI-1).
// Each product is int32 Q12; the accumulator is left in Q12 (no
// post-shift) per the Phase 2d sub-plan §3 line 142 contract
// (`d *[40]int32`). Downstream sign extraction (CB-3) and the φ′
// search (CB-2) consume d in Q12 so no precision is lost prior to the
// algebraic search numerator C = Σᵢ sᵢ d(mᵢ) (eq. 54, line 1278).
//
// I3 / I4: pure (writes only through d), zero allocation.
func CorrelationD(xPrime, h *[SubframeLen]int16, d *[SubframeLen]int32) {
	for n := 0; n < SubframeLen; n++ {
		var acc int64
		for i := n; i < SubframeLen; i++ {
			acc += int64(xPrime[i]) * int64(h[i-n])
		}
		d[n] = saturateInt64ToInt32(acc)
	}
}

func saturateInt64ToInt32(v int64) int32 {
	if v > 0x7fffffff {
		return 0x7fffffff
	}
	if v < -0x80000000 {
		return -0x80000000
	}
	return int32(v)
}
