package postfilter

// refinePitch selects the best integer pitch lag T in the Annex A search
// range [Tcl-3, Tcl+3] (within [20, pitchMax]) by maximising the
// normalised cross-correlation with the residual per ITU-T G.729 §A.4.2.1.
//
// The caller MUST have written the current subframe's residual r(n) into
// pf.pastResidual[pitchMax + n] before invoking refinePitch. Past samples
// r(n-T) for n-T < 0 are read from the lower portion of pastResidual.
//
// Returns the selected lag T.
func (pf *Postfilter) refinePitch(r *[subframeLen]int16, tInt int) int {
	const minT = 17
	const maxT = pitchMax

	_ = r // r is also accessible via pf.pastResidual[pitchMax + n]; kept
	// in the signature so callers express the data dependency.

	center := tInt
	if center > 140 {
		center = 140
	}

	lo := center - 3
	if lo < minT {
		lo = minT
	}
	hi := center + 3
	if hi > maxT {
		hi = maxT
	}

	bestT := lo
	bestScore := float64(-1)

	for T := lo; T <= hi; T++ {
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
		score := float64(R) * float64(R) / float64(E)
		if score > bestScore {
			bestT = T
			bestScore = score
		}
	}

	return bestT
}

// computeLongTermGain returns pre-computed weights g0 = 1/(1+γp·g_l) and
// g1 = γp·g_l/(1+γp·g_l) (Q14) for the long-term postfilter, where
//
// g_l = clamp(R(T)/E(T), 0, 1)
//
// per ITU-T G.729 §4.2.1 / §A.4.2.1 with γp = 0.5. The filter is
// disabled when the squared normalized correlation is less than 0.5
// (long-term prediction gain below the 3 dB gate).
//
// Like refinePitch, this reads the residual via pf.pastResidual; the
// caller MUST have written current r(n) into pf.pastResidual[pitchMax+n]
// beforehand.
func (pf *Postfilter) computeLongTermGain(r *[subframeLen]int16, T int) (g0, g1 int16) {
	const gammaPQ14 int64 = 8192 // = 0.5; ITU-T G.729 §4.2.1 / §A.4.2.1

	var R, delayedE, currentE int64
	for n := 0; n < subframeLen; n++ {
		rn := int64(pf.pastResidual[pitchMax+n])
		rnT := int64(pf.pastResidual[pitchMax+n-T])
		R += rn * rnT
		delayedE += rnT * rnT
		currentE += int64(r[n]) * int64(r[n])
	}

	if R <= 0 || delayedE == 0 || currentE == 0 {
		return 16383, 0
	}

	normCorrSq := float64(R) * float64(R) / (float64(delayedE) * float64(currentE))
	if normCorrSq < 0.5 {
		return 16383, 0
	}

	glQ14 := clamp64(R*16384/delayedE, 0, 16384)
	gammaGlQ14 := int32((glQ14*gammaPQ14 + (1 << 13)) >> 14)

	denom := int32(16384) + gammaGlQ14
	g0 = int16(int32(16384) * 16384 / denom)
	g1 = int16(gammaGlQ14 * 16384 / denom)
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
	pf.applyLongTermWithGains(T, g0, g1, rOut)
}

func (pf *Postfilter) applyLongTermWithGains(T int, g0, g1 int16, rOut *[subframeLen]int16) {
	if g1 == 0 {
		copy(rOut[:], pf.pastResidual[pitchMax:pitchMax+subframeLen])
		return
	}
	for n := 0; n < subframeLen; n++ {
		p0 := int64(g0) * int64(pf.pastResidual[pitchMax+n])
		p1 := int64(g1) * int64(pf.pastResidual[pitchMax+n-T])
		sum := (p0 >> 14) + (p1 >> 14)
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		rOut[n] = int16(sum)
	}
}
