package closedloop

import "github.com/hunydev/g729/internal/fixed"

// RefineFraction selects the 1/3-sample fractional pitch lag offset
// that maximises the §A.3.7 numerator-only criterion
//
//	RN(intLag, t) = Σ_{n=0..39} xb(n) · u_kt(n)
//
// around the integer winner intLag returned by SearchInteger. The
// candidate fractions are the three 1/3-sample positions adjacent
// to the integer lag, expressed in encoder-side convention as
// frac ∈ {−1, 0, +1} mapping to delays intLag + frac/3 (per
// §3.7.2 eq. 41 / G729E.txt lines 1170–1185).
//
// The interpolated past excitation u_kt(n) is built with the same
// b30 FIR primitive shared with the decoder: for each subframe
// position n ∈ [0, 39],
//
//	u_kt(n) = Interpolate3(exc, intLag − n, frac)
//
// per ITU-T G.729 §3.7.1 eq. (40) instantiated for §A.3.7 eq. A.8
// (G729E.txt lines 1162, 2178). Interpolate3 treats frac as part of
// the transmitted pitch delay, so Interpolate3(intLag − n, frac)
// evaluates u(n − (intLag + frac/3)), the eq. A.8 sample at subframe
// position n.
//
// allowFrac gates whether ±1/3 are evaluated. The encoder passes true
// for T2, whose P2 codebook is always fractional, and for T1 only when
// intLag < 85; otherwise T1 is integer-only in [85,143].
//
// Tie-break. When two or more RN(t) values coincide (notably the
// xb ≡ 0 degenerate case), the implementation favours the lowest
// composite delay intLag + frac/3, i.e. frac = −1, mirroring the
// open-loop §A.3.4 line 2110 "favouring the delays with the values
// in the lower range" convention. Operationally: candidates are
// evaluated in order (−1, 0, +1) with a strict ">" replacement test
// so the first (lowest-delay) candidate retains the win on ties.
//
// Buffer convention. exc shares the SearchInteger layout: it is the
// past-excitation buffer with the LP-residual extension covering
// u(0..39) appended at the tail, anchored so u(0) =
// exc[len(exc) − SubframeLen] and u(n − k) =
// exc[len(exc) − SubframeLen − k + n] for any (k, n) in the search
// domain (cf. SearchInteger godoc).
//
// Q-format. xb and exc are Word16 (Q0). The accumulator uses
// fixed.LMac so RN(t) carries the standard ITU implicit ×2 product
// scaling — but only relative ordering matters for the argmax, so
// the scaling factor is irrelevant to the returned frac.
//
// I3 / I4: pure (reads xb / exc), zero allocation.
//
// Spec anchors: §A.3.7 eq. A.7 (G729E.txt line 2154); eq. A.8
// (line 2178); fractional-search gating "if the optimum integer
// delay is less than 85" (lines 2169–2170); §3.7.2 eq. 41 frac
// transmission convention.
func RefineFraction(xb *[SubframeLen]int16, exc []int16, intLag int16, allowFrac bool) int8 {
	if !allowFrac {
		return 0
	}
	bestFrac := int8(-1)
	bestRN := correlateAtFrac(xb, exc, intLag, -1)
	for _, frac := range [2]int8{0, +1} {
		rn := correlateAtFrac(xb, exc, intLag, frac)
		if rn > bestRN {
			bestRN = rn
			bestFrac = frac
		}
	}
	return bestFrac
}

// correlateAtFrac evaluates RN(intLag, frac) per §A.3.7 eq. A.7
// using Interpolate3 to materialise u_kt(n) on the fly. No scratch
// array is required because the inner product is accumulated
// incrementally.
func correlateAtFrac(xb *[SubframeLen]int16, exc []int16, intLag int16, frac int8) fixed.Word32 {
	var acc fixed.Word32
	for n := 0; n < SubframeLen; n++ {
		s := Interpolate3(exc, intLag-int16(n), frac)
		acc = fixed.LMac(acc, xb[n], s)
	}
	return acc
}
