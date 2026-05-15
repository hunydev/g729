package postfilter

import "github.com/hunydev/g729/internal/fixed"

type longTermGainWeights struct {
	g0Q14                 int16
	g1Q14                 int16
	gammaScaledGainQ15    int16
	longTermFilterEnabled bool
}

// refinePitch selects the best integer pitch lag T in the Annex A search
// range [Tcl-3, Tcl+3] (within [20, pitchMax]) by maximising the
// cross-correlation with the residual per ITU-T G.729 §A.4.2.1.
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
	bestCorr := int64(-1)

	for T := lo; T <= hi; T++ {
		var R int64
		for n := 0; n < subframeLen; n++ {
			R += longTermScaledProduct(
				pf.pastResidual[pitchMax+n],
				pf.pastResidual[pitchMax+n-T],
			)
		}
		if R > bestCorr {
			bestT = T
			bestCorr = R
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
	weights := pf.computeLongTermGainWeights(r, T)
	return weights.g0Q14, weights.g1Q14
}

func (pf *Postfilter) computeLongTermGainWeights(r *[subframeLen]int16, T int) longTermGainWeights {
	const (
		identityG0Q14              int16 = 16383
		maxGammaScaledGainQ15      int16 = 10923
		longTermGateThresholdQ15Sq int64 = 16384
	)

	var R int64
	delayedE := int64(1)
	currentE := int64(1)
	for n := 0; n < subframeLen; n++ {
		rn := pf.pastResidual[pitchMax+n]
		rnT := pf.pastResidual[pitchMax+n-T]
		R += longTermScaledProduct(rn, rnT)
		delayedE += longTermScaledProduct(rnT, rnT)
		currentE += longTermScaledProduct(r[n], r[n])
	}

	if R <= 0 || delayedE == 0 || currentE == 0 {
		return longTermGainWeights{g0Q14: identityG0Q14}
	}

	normShift := longTermNormShift(R, delayedE, currentE)
	rNormQ15 := longTermRoundedNormQ15(R, normShift)
	delayedENormQ15 := longTermRoundedNormQ15(delayedE, normShift)
	currentENormQ15 := longTermRoundedNormQ15(currentE, normShift)

	if int64(rNormQ15)*int64(rNormQ15) < longTermGateThresholdQ15Sq*int64(delayedENormQ15)*int64(currentENormQ15)>>15 {
		return longTermGainWeights{g0Q14: identityG0Q14}
	}

	gammaScaledGainQ15 := maxGammaScaledGainQ15
	if rNormQ15 < delayedENormQ15 {
		gammaScaledRQ14 := rNormQ15 >> 2
		gainDenQ14 := (delayedENormQ15 >> 1) + gammaScaledRQ14
		gammaScaledGainQ15 = fixed.DivS(gammaScaledRQ14, gainDenQ14)
	}

	g1Q14 := gammaScaledGainQ15 >> 1
	return longTermGainWeights{
		g0Q14:                 identityG0Q14 - g1Q14,
		g1Q14:                 g1Q14,
		gammaScaledGainQ15:    gammaScaledGainQ15,
		longTermFilterEnabled: true,
	}
}

func longTermNormShift(values ...int64) int {
	var max int64
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		return 0
	}

	var shift int
	for max < 0x40000000 {
		max <<= 1
		shift++
	}
	return shift
}

func longTermRoundedNormQ15(v int64, shift int) int16 {
	norm := ((v << shift) + 0x8000) >> 16
	if norm > 32767 {
		return 32767
	}
	return int16(norm)
}

func longTermScaledProduct(a, b int16) int64 {
	return 2 * int64(a>>2) * int64(b>>2)
}

// applyLongTerm filters the residual with the long-term postfilter
//
// r'(n) = g0 · r(n) + g1 · r(n-T)
//
// per ITU-T G.729 §A.4.2.2.
func (pf *Postfilter) applyLongTerm(r *[subframeLen]int16, T int, rOut *[subframeLen]int16) {
	weights := pf.computeLongTermGainWeights(r, T)
	pf.applyLongTermWithGainQ15(T, weights, rOut)
}

func (pf *Postfilter) applyLongTermWithGainQ15(T int, weights longTermGainWeights, rOut *[subframeLen]int16) {
	if !weights.longTermFilterEnabled {
		copy(rOut[:], pf.pastResidual[pitchMax:pitchMax+subframeLen])
		return
	}
	g0Q15 := int64(fixed.Max16 - weights.gammaScaledGainQ15)
	g1Q15 := int64(weights.gammaScaledGainQ15)
	for n := 0; n < subframeLen; n++ {
		p0 := (g0Q15 * int64(pf.pastResidual[pitchMax+n])) >> 15
		p1 := (g1Q15 * int64(pf.pastResidual[pitchMax+n-T])) >> 15
		sum := p0 + p1
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		rOut[n] = int16(sum)
	}
}
