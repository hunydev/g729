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
