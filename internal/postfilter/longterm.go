package postfilter

// refinePitch selects the best pitch lag T ∈ {tInt-1, tInt, tInt+1} (within
// [20, pitchMax]) by maximising the normalised cross-correlation with the
// residual per ITU-T G.729 §A.4.2.2.
//
// The caller MUST have written the current subframe's residual r(n) into
// pf.pastResidual[pitchMax + n] before invoking refinePitch. Past samples
// r(n-T) for n-T < 0 are read from the lower portion of pastResidual.
//
// Returns the selected lag T.
func (pf *Postfilter) refinePitch(r *[subframeLen]int16, tInt int) int {
	const minT = 20
	const maxT = pitchMax

	_ = r // r is also accessible via pf.pastResidual[pitchMax + n]; kept
	// in the signature so callers express the data dependency.

	bestT := tInt
	var bestRsq, bestE int64 = 0, 1

	for k := -1; k <= 1; k++ {
		T := tInt + k
		if T < minT || T > maxT {
			continue
		}
		var R, E int64
		for n := 0; n < subframeLen; n++ {
			rn := int64(pf.pastResidual[pitchMax+n])
			rnT := int64(pf.pastResidual[pitchMax+n-T])
			R += rn * rnT
			E += rnT * rnT
		}
		if R <= 0 || E == 0 {
			continue
		}
		// Compare R²/E vs bestRsq/bestE via cross-multiplication.
		if R*R*bestE > bestRsq*E {
			bestT = T
			bestRsq = R * R
			bestE = E
		}
	}

	return bestT
}

// computeLongTermGain returns pre-computed weights g0 = 1/(1+g_l) and
// g1 = g_l/(1+g_l) (Q14) for the long-term postfilter, where
//
// g_l = clamp(R(T)/E(T), 0, γ_l)
//
// per ITU-T G.729 §A.4.2.2 with γ_l = 0.5 (Annex A bound).
//
// Like refinePitch, this reads the residual via pf.pastResidual; the
// caller MUST have written current r(n) into pf.pastResidual[pitchMax+n]
// beforehand.
func (pf *Postfilter) computeLongTermGain(r *[subframeLen]int16, T int) (g0, g1 int16) {
	const gammaLQ14 int16 = 8192 // = 0.5; ITU-T G.729 §A.4.2.2

	_ = r

	var R, E int64
	for n := 0; n < subframeLen; n++ {
		rn := int64(pf.pastResidual[pitchMax+n])
		rnT := int64(pf.pastResidual[pitchMax+n-T])
		R += rn * rnT
		E += rnT * rnT
	}

	if R <= 0 || E == 0 {
		return 16384, 0
	}

	gRawQ14 := int16(clamp64(R*16384/E, 0, 32767))
	var gLQ14 int16
	if gRawQ14 > gammaLQ14 {
		gLQ14 = gammaLQ14
	} else {
		gLQ14 = gRawQ14
	}

	denom := int32(16384 + int32(gLQ14))
	g0 = int16(int32(16384) * 16384 / denom)
	g1 = int16(int32(gLQ14) * 16384 / denom)
	return g0, g1
}

func clamp64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// applyLongTerm filters the residual with the long-term postfilter
//
// r'(n) = g0 · r(n) + g1 · r(n-T)
//
// per ITU-T G.729 §A.4.2.2.
func (pf *Postfilter) applyLongTerm(r *[subframeLen]int16, T int, rOut *[subframeLen]int16) {
	g0, g1 := pf.computeLongTermGain(r, T)

	for n := 0; n < subframeLen; n++ {
		p0 := int32(g0) * int32(pf.pastResidual[pitchMax+n])
		p1 := int32(g1) * int32(pf.pastResidual[pitchMax+n-T])
		sum := p0 + p1
		rOut[n] = int16((sum + (1 << 13)) >> 14)
	}
}
