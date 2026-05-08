package closedloop

import (
	"testing"

	"github.com/hunydev/g729/internal/fixed"
)

// refineExcLen mirrors fracTestExcLen / testExcLen used by the
// neighbouring FR-1 / CL-1 unit tests. ≥ PitchMaxInt(143) +
// SubframeLen(40) + Linter(10) slack.
const refineExcLen = 256

// computeRN reproduces the §A.3.7 numerator-only criterion
// RN(intLag, frac) = Σ_{n=0..39} 2·xb(n)·u_kt(n) (LMac scaling)
// using the production Interpolate3 primitive. It serves only as
// a verification scaffold inside tests; production code lives in
// refine.go (RefineFraction). Spec: G729E.txt eq. A.7 (line 2154),
// A.8 (line 2178).
func computeRN(xb *[SubframeLen]int16, exc []int16, intLag int16, frac int8) fixed.Word32 {
	var acc fixed.Word32
	for n := 0; n < SubframeLen; n++ {
		s := Interpolate3(exc, intLag-int16(n), frac)
		acc = fixed.LMac(acc, xb[n], s)
	}
	return acc
}

// TestRefineFraction_FracZeroForExactImpulse: with a unit impulse
// planted at u(−intLag) and xb concentrated at n = 0, the integer
// path (frac = 0) returns the impulse amplitude verbatim while the
// b30-interpolated paths (frac = ±1) attenuate it through the
// off-center taps. The integer winner therefore dominates and
// RefineFraction must report frac = 0.
//
// Spec: §A.3.7 eq. A.7/A.8 (G729E.txt lines 2154, 2178); §3.7.1
// b30 storage convention (PitchInterpFIR[0] = b30(0) = 29443 ≠ 1.0
// in Q15 — the integer path is a direct copy, not an FIR tap).
func TestRefineFraction_FracZeroForExactImpulse(t *testing.T) {
	const intLag int16 = 60
	var xb [SubframeLen]int16
	xb[0] = 1 << 14 // arbitrary positive concentrated target

	var exc [refineExcLen]int16
	exc[refineExcLen-SubframeLen-int(intLag)] = 1 << 14 // u(−intLag)

	got := RefineFraction(&xb, exc[:], intLag, true)
	if got != 0 {
		t.Fatalf("frac = %d, want 0 for exact-impulse alignment", got)
	}
}

// TestRefineFraction_FracPositiveWhenInputIsPlusOneThird matches
// the §A.3.7 criterion with a *matched filter*: build the three
// candidate u_kt(n) sample streams from a generic past-excitation
// buffer using the production Interpolate3 primitive, then drive
// xb := u_{intLag, +1}. By the Cauchy–Schwarz inequality
//
//	Σ s_+ · s_+ ≥ |Σ s_+ · s_x|
//
// for any other sequence s_x not proportional to s_+, so RN is
// strictly maximised at frac = +1 for non-degenerate exc.
//
// Spec: §A.3.7 eq. A.7 (G729E.txt line 2154) — RN(k,t) is the
// inner product of xb with u_kt; the matched filter argument
// guarantees the planted frac wins.
func TestRefineFraction_FracPositiveWhenInputIsPlusOneThird(t *testing.T) {
	const intLag int16 = 70
	var exc [refineExcLen]int16
	for i := range exc {
		// Bounded pseudo-random pattern; small magnitude keeps the
		// LMac accumulation well clear of Word32 saturation.
		exc[i] = int16(((i*131 + 17) & 0x7F) - 64)
	}
	var xb [SubframeLen]int16
	for n := 0; n < SubframeLen; n++ {
		xb[n] = Interpolate3(exc[:], intLag-int16(n), +1)
	}
	got := RefineFraction(&xb, exc[:], intLag, true)
	if got != +1 {
		// Sanity-print all three RN values to ease debugging on
		// regression.
		t.Fatalf("frac = %d, want +1; RN(-1)=%d RN(0)=%d RN(+1)=%d",
			got,
			computeRN(&xb, exc[:], intLag, -1),
			computeRN(&xb, exc[:], intLag, 0),
			computeRN(&xb, exc[:], intLag, +1))
	}
}

// TestRefineFraction_FracNegativeWhenInputIsMinusOneThird mirrors
// the +1/3 matched-filter test for the −1/3 fractional position.
func TestRefineFraction_FracNegativeWhenInputIsMinusOneThird(t *testing.T) {
	const intLag int16 = 70
	var exc [refineExcLen]int16
	for i := range exc {
		exc[i] = int16(((i*97 + 23) & 0x7F) - 64)
	}
	var xb [SubframeLen]int16
	for n := 0; n < SubframeLen; n++ {
		xb[n] = Interpolate3(exc[:], intLag-int16(n), -1)
	}
	got := RefineFraction(&xb, exc[:], intLag, true)
	if got != -1 {
		t.Fatalf("frac = %d, want -1; RN(-1)=%d RN(0)=%d RN(+1)=%d",
			got,
			computeRN(&xb, exc[:], intLag, -1),
			computeRN(&xb, exc[:], intLag, 0),
			computeRN(&xb, exc[:], intLag, +1))
	}
}

// TestRefineFraction_AllowFracFalseForcesInteger pins the spec
// boundary §A.3.7 G729E.txt lines 2169–2170: "For the determination
// of T2 and T1 if the optimum integer delay is less than 85, the
// fractions around the optimum integer delay have to be tested."
// The contrapositive: when intLag ∈ [85, 143] the search is integer
// only. The caller signals this via allowFrac = false; RefineFraction
// must short-circuit and return frac = 0 unconditionally, regardless
// of what RN(±1) would yield.
//
// We deliberately stage exc/xb such that the +1/3 evaluation would
// win if it were considered (matched filter for frac=+1), proving
// the gate is enforced by allowFrac and not by an accidental
// data-dependent path.
func TestRefineFraction_AllowFracFalseForcesInteger(t *testing.T) {
	const intLag int16 = 100 // ∈ [85, 143] — integer-only per §A.3.7.
	var exc [refineExcLen]int16
	for i := range exc {
		exc[i] = int16(((i*131 + 17) & 0x7F) - 64)
	}
	var xb [SubframeLen]int16
	for n := 0; n < SubframeLen; n++ {
		xb[n] = Interpolate3(exc[:], intLag-int16(n), +1)
	}
	got := RefineFraction(&xb, exc[:], intLag, false)
	if got != 0 {
		t.Fatalf("frac = %d, want 0 (allowFrac=false → integer only per §A.3.7)", got)
	}
}

// TestRefineFraction_TieBreakFavoursLowerFrac: when all three
// candidate RN values are equal (e.g. xb ≡ 0 ⇒ RN(frac) ≡ 0), the
// implementation must follow the openloop §A.3.4 line 2110
// "favouring the delays with the values in the lower range"
// convention. In encoder convention T1 = intLag + frac/3, so the
// lowest delay is frac = −1.
func TestRefineFraction_TieBreakFavoursLowerFrac(t *testing.T) {
	const intLag int16 = 50
	var xb [SubframeLen]int16 // all zero ⇒ RN ≡ 0
	var exc [refineExcLen]int16
	for i := range exc {
		exc[i] = int16(i - 128)
	}
	got := RefineFraction(&xb, exc[:], intLag, true)
	if got != -1 {
		t.Fatalf("frac = %d, want -1 (tie-break favours lower delay)", got)
	}
}

// TestRefineFraction_NoAlloc enforces I4: the per-subframe FR-2
// refinement runs three 40-sample inner products with on-stack
// scratch only.
func TestRefineFraction_NoAlloc(t *testing.T) {
	const intLag int16 = 70
	var exc [refineExcLen]int16
	for i := range exc {
		exc[i] = int16(((i * 31) & 0x3FF) - 0x200)
	}
	var xb [SubframeLen]int16
	for n := range xb {
		xb[n] = int16(n*7 - 100)
	}
	var sink int8
	avg := testing.AllocsPerRun(200, func() {
		sink ^= RefineFraction(&xb, exc[:], intLag, true)
		sink ^= RefineFraction(&xb, exc[:], intLag, false)
	})
	if avg != 0 {
		t.Fatalf("RefineFraction alloc/op = %v, want 0", avg)
	}
	_ = sink
}
