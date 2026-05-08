package openloop

import (
	"math/bits"

	"github.com/hunydev/g729/internal/fixed"
)

// energy computes the eq. A.5 denominator (G729E.txt §A.3.4 lines
// 2102-2107):
//
//	E(k) = Σ_{n=0..39} sw²(2n − k)
//
// over the 223-sample wsp window with the same indexing convention as
// OL-1 (sw(2n − k) = wsp[143 + 2n − k] for k ∈ [20,143]). Reads only
// wsp; pure, zero-allocation.
//
// Q-format. wsp is int16 Q0; each squared tap is at most 32767² ≈
// 1.07·10⁹ which fits Word32. The 40-tap sum can reach 4.29·10¹⁰
// which exceeds Max32, so accumulation uses fixed.LAdd to saturate at
// Max32 rather than wrap. This raw-square form (NOT LMac, which would
// add an implicit ×2) matches the OL-1 plan §6 OL-2 acceptance test
// "wsp[i] = 1024 → energy(k) = 40·1024² = 41943040".
//
// Note that OL-1's correlate uses LMac which carries an implicit ×2
// scaling so its returned rsq is actually 2·R(k). compareNormalized
// below squares rsq before comparison, and since the same ×2 factor
// appears on every candidate within a comparison, the R²/E ordering
// is preserved (the spurious ×4 in numerators cancels in the cross-
// multiplicative test).
func energy(wsp *[223]int16, k int) fixed.Word32 {
	var acc fixed.Word32
	for n := 0; n < 40; n++ {
		s := fixed.Word32(wsp[143+2*n-k])
		acc = fixed.LAdd(acc, s*s)
	}
	return acc
}

// compareNormalized returns true iff candidate-1's normalized open-loop
// pitch score (eq. A.5: R'(t) = R(t)/√E(t), §A.3.4 line 2104) is
// greater than or equal to candidate-2's. For non-negative R the
// monotone-equivalent ordering is on R²/E, computed here via the
// cross-multiplicative form
//
//	cand1 ≥ cand2  ⇔  R₁² · E₂ ≥ R₂² · E₁
//
// thereby avoiding the divide and the sqrt of eq. A.5.
//
// rsq parameters carry the OL-1 correlate output semantics — the field
// name is historical: it actually stores R(k) (Word32, signed-clipped
// to ≥ 0 by correlate; here re-clipped defensively), NOT R²(k). The
// squaring required by eq. A.5 is performed inside this function. See
// §A.3.4 line 2104 for the eq. A.5 normalization and the OL-1 doc
// comment in correlate.go for the rsq Q-format pin.
//
// Overflow safety. R and E are each bounded by Max32 ≈ 2³¹. Naïve
// int64(R)·int64(R)·int64(E) would reach 2⁹³ — wraps catastrophically
// in int64. We normalize both R values by a common right-shift so the
// larger fits in 15 bits; r² then fits in 30 bits and r²·E fits in
// 30+31 = 61 bits, well within signed int64. The shift is identical
// for both candidates so the ratio is preserved. Worst-case shift is
// 31 − 15 = 16 bits.
//
// E = 0 edge case: by Cauchy-Schwarz, R = 0 too, so the eq. A.5 ratio
// is 0/0 — treated here as score 0 (worst). A candidate with E = 0 or
// R = 0 thus loses to any positive-score candidate; ties (both zero)
// return true.
//
// Pure, zero-allocation.
func compareNormalized(rsq1, e1, rsq2, e2 fixed.Word32) bool {
	score1Zero := e1 <= 0 || rsq1 <= 0
	score2Zero := e2 <= 0 || rsq2 <= 0
	if score1Zero && score2Zero {
		return true
	}
	if score1Zero {
		return false
	}
	if score2Zero {
		return true
	}
	r1 := int64(rsq1)
	r2 := int64(rsq2)
	maxR := r1
	if r2 > maxR {
		maxR = r2
	}
	var s uint
	if l := bits.Len64(uint64(maxR)); l > 15 {
		s = uint(l - 15)
	}
	r1 >>= s
	r2 >>= s
	return r1*r1*int64(e2) >= r2*r2*int64(e1)
}
