package closedloop

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
// Q-format. xb and exc are Word16 (Q0). The accumulator keeps the
// standard ITU implicit ×2 product scaling, but uses int64 internally
// so fractional selection follows the unsaturated RN(t) ordering.
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

// RefineFractionSubframe1 selects the best P1-encodable fractional delay for
// the first subframe around the integer lag returned by SearchInteger.
//
// The first-subframe P1 field spans the fractional region [19+1/3, 84+2/3]
// plus the integer-only region [85,143]. Most fractional candidates are covered
// by testing frac ∈ {-1,0,+1} around the integer winner, but the P1 codebook
// also exposes two fractional boundary codepoints outside that local triplet:
// (19,+1) at the lower edge and (85,-1) at the upper fractional edge. This
// helper adds those candidates when the integer winner is at the adjacent
// boundary while keeping RefineFraction's strict-greater/lower-delay tie-break.
func RefineFractionSubframe1(xb *[SubframeLen]int16, exc []int16, intLag int16) (int16, int8) {
	if intLag >= 85 {
		return intLag, 0
	}

	bestLag := intLag
	bestFrac := int8(-1)
	bestRNSet := false
	var bestRN int64
	consider := func(lag int16, frac int8) {
		rn := correlateAtFrac(xb, exc, lag, frac)
		if !bestRNSet || rn > bestRN {
			bestRNSet = true
			bestRN = rn
			bestLag = lag
			bestFrac = frac
		}
	}

	if intLag == PitchMinInt {
		consider(PitchMinInt-1, +1)
	}
	consider(intLag, -1)
	consider(intLag, 0)
	consider(intLag, +1)
	if intLag == 84 {
		consider(85, -1)
	}
	return bestLag, bestFrac
}

// RefineFractionSubframe2 selects the best P2-encodable fractional delay for
// the second subframe around the integer lag returned by SearchInteger.
//
// Clause 3.7 defines the second-subframe fractional search span as
// [tmin-2/3, tmax+2/3]. Most of that span is covered by testing frac ∈
// {-1,0,+1} around the integer winner, but if the integer winner is exactly
// tmin or tmax the P2 codebook also exposes one boundary codepoint outside the
// integer window: (tmin-1,+1) and (tmax+1,-1). This helper adds those two edge
// candidates while keeping RefineFraction's strict-greater/lower-delay
// tie-break.
func RefineFractionSubframe2(xb *[SubframeLen]int16, exc []int16, intLag int16, intT1 int16) (int16, int8) {
	tmin, tmax := Subframe2Window(intT1)
	bestLag := intLag
	bestFrac := int8(-1)
	bestRNSet := false
	var bestRN int64
	consider := func(lag int16, frac int8) {
		rn := correlateAtFrac(xb, exc, lag, frac)
		if !bestRNSet || rn > bestRN {
			bestRNSet = true
			bestRN = rn
			bestLag = lag
			bestFrac = frac
		}
	}

	if intLag == tmin {
		consider(tmin-1, +1)
	}
	consider(intLag, -1)
	consider(intLag, 0)
	consider(intLag, +1)
	if intLag == tmax {
		consider(tmax+1, -1)
	}
	return bestLag, bestFrac
}

// correlateAtFrac evaluates RN(intLag, frac) per §A.3.7 eq. A.7
// using Interpolate3 to materialise u_kt(n) on the fly. No scratch
// array is required because the inner product is accumulated
// incrementally.
func correlateAtFrac(xb *[SubframeLen]int16, exc []int16, intLag int16, frac int8) int64 {
	var acc int64
	for n := 0; n < SubframeLen; n++ {
		s := Interpolate3(exc, intLag-int16(n), frac)
		acc += 2 * int64(xb[n]) * int64(s)
	}
	return acc
}
