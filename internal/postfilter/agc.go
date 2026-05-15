package postfilter

import "github.com/hunydev/g729/internal/fixed"

// agcAlphaQ15 is the Annex A adaptive-gain smoothing factor α = 0.9 in
// Q15. The main G.729 postfilter uses a slower α, but Annex A §A.4.2.4
// tightens the smoother to track the lower-complexity decoder envelope.
const agcAlphaQ15 int64 = 29491
const agcAlphaComplementQ15 int64 = 3276

// computeAGCTargetGain returns the per-sample AGC target increment (Q14)
// derived from √(E(s) / E(sTilt)) and the Annex A smoothing complement.
//
// Annex A's AGC energy estimator uses the shifted sample domain rather than
// full-resolution average energy. The target is kept at Q14 for historical
// call sites, but its low two bits are zero because the AGC recurrence consumes
// the corresponding Q12 increment.
func (pf *Postfilter) computeAGCTargetGain(s, sTilt *[subframeLen]int16) int16 {
	eS := agcTargetEnergy(s)
	eT := agcTargetEnergy(sTilt)
	if eS == 0 || eT == 0 {
		return 0
	}
	if eS == eT {
		targetQ12 := (int64(4096) * agcAlphaComplementQ15) >> 15
		return int16(targetQ12 << 2)
	}
	sqrtQ12 := agcTargetSqrtInputOverPostQ12(eS, eT)
	targetQ12 := (sqrtQ12 * agcAlphaComplementQ15) >> 15
	return int16(targetQ12 << 2)
}

func agcTargetEnergy(x *[subframeLen]int16) int64 {
	var energy int64
	for n := 0; n < subframeLen; n++ {
		v := int64(x[n] >> 2)
		energy += 2 * v * v
	}
	return energy
}

func agcTargetSqrtInputOverPostQ12(inputEnergy, postEnergy int64) int64 {
	postShift := agcTargetNormShift(postEnergy)
	inputShift := agcTargetNormShift(inputEnergy)
	postNorm := agcTargetRoundedNorm(postEnergy, postShift)
	inputNorm := agcTargetRoundedNorm(inputEnergy, inputShift)

	// DivS requires numerator <= denominator. The inverse-sqrt path also wants
	// an even exponent delta so the square-root denormalization is integral.
	if postNorm > inputNorm || ((postShift-inputShift)&1) != 0 {
		if postShift > 0 {
			postShift--
			postNorm = agcTargetRoundedNorm(postEnergy, postShift)
		}
	}

	if postNorm <= 0 || inputNorm <= 0 {
		return 0
	}
	ratioDivQ15 := fixed.DivS(int16(postNorm), int16(inputNorm))
	expDelta := postShift - inputShift
	shift := 7 - expDelta
	ratioQ28 := int64(ratioDivQ15)
	if shift >= 0 {
		ratioQ28 <<= shift
	} else {
		ratioQ28 >>= -shift
	}
	return agcInverseSqrtQ12(ratioQ28)
}

func agcTargetNormShift(v int64) int {
	if v <= 0 {
		return 0
	}
	var shift int
	for v < 0x40000000 {
		v <<= 1
		shift++
	}
	return shift
}

func agcTargetRoundedNorm(v int64, shift int) int64 {
	norm := ((v << shift) + 0x8000) >> 16
	if norm > 32767 {
		return 32767
	}
	return norm
}

func agcInverseSqrtQ12(x int64) int64 {
	if x <= 0 {
		return 0
	}
	normShift := agcTargetNormShift(x)
	normX := x << normShift
	adjustedX := normX
	if normShift%2 == 0 {
		adjustedX >>= 1
	}

	index := int((adjustedX >> 25) - 16)
	if index < 0 {
		index = 0
	} else if index > 30 {
		index = 30
	}
	frac := (adjustedX >> 10) & 0x7fff

	base := agcInverseSqrtTableValue(index)
	next := agcInverseSqrtTableValue(index + 1)
	acc := (base << 16) - 2*(base-next)*frac
	denormShift := 16 - ((normShift + 1) >> 1)
	out := acc >> denormShift
	return (out + 64) >> 7
}

func agcInverseSqrtTableValue(index int) int64 {
	const tableScale = int64(32768)
	numerator := int64(16) * tableScale * tableScale
	denominator := int64(16 + index)
	root := isqrt64(numerator / denominator)
	for (root+1)*(root+1)*denominator <= numerator {
		root++
	}
	loDiff := numerator - root*root*denominator
	hiDiff := (root+1)*(root+1)*denominator - numerator
	if hiDiff < loDiff {
		root++
	}
	return root
}

// isqrtRoundedQ12 returns round(√x) where x is interpreted at Q24.
func isqrtRoundedQ12(xQ24 int64) int64 {
	if xQ24 <= 0 {
		return 0
	}
	root := isqrt64(xQ24)
	loDiff := xQ24 - root*root
	hi := root + 1
	hiDiff := hi*hi - xQ24
	if hiDiff < loDiff {
		return hi
	}
	return root
}

func isqrt64(x int64) int64 {
	if x <= 0 {
		return 0
	}
	guess := x
	for {
		next := (guess + x/guess) >> 1
		if next >= guess {
			return guess
		}
		guess = next
	}
}

// isqrtQ14 returns ⌊√x⌋ where x is interpreted at Q28 (result is at Q14).
// Uses integer Newton-Raphson; replace with a table or fixed.Sqrt if
// Phase 1g demands bit-exact match.
func isqrtQ14(xQ28 int64) int16 {
	if xQ28 == 0 {
		return 0
	}
	var guess int64 = 1 << 14
	for i := 0; i < 10; i++ {
		next := (guess + xQ28/guess) >> 1
		if next == guess {
			break
		}
		guess = next
	}
	if guess > 32767 {
		return 32767
	}
	return int16(guess)
}

// applyAGC smooths g_target into agcGainPrev (one-pole lowpass, α = 0.9)
// and scales sTilt to produce sPf per ITU-T G.729 §A.4.2.4.
func (pf *Postfilter) applyAGC(sTilt *[subframeLen]int16, gTargetQ14 int16, sPf *[subframeLen]int16) {
	pf.applyAGCWithTaps(sTilt, gTargetQ14, sPf, nil)
}

func (pf *Postfilter) applyAGCWithTaps(sTilt *[subframeLen]int16, gTargetQ14 int16, sPf *[subframeLen]int16, taps *FilterTaps) {
	if !pf.initialized {
		pf.agcGainPrev = 1 << 24
		pf.initialized = true
	}
	if gTargetQ14 == 0 {
		copy(sPf[:], sTilt[:])
		if taps != nil {
			for n := 0; n < subframeLen; n++ {
				taps.AGCGainQ24[n] = 0
			}
		}
		pf.agcGainPrev = 0
		return
	}

	g := int64(pf.agcGainPrev) // Q24
	targetQ12 := int64(gTargetQ14) >> 2
	for n := 0; n < subframeLen; n++ {
		before := g
		mulPrev := (agcAlphaQ15 * g) >> 27
		acc := mulPrev + targetQ12
		g = acc << 12
		if taps != nil {
			taps.AGCGainBeforeUpdateQ24[n] = int32(before)
			taps.AGCUpdateMulPrevQ0[n] = int32(mulPrev)
			taps.AGCUpdateMulTargetQ0[n] = int32(targetQ12)
			taps.AGCUpdateAccQ0[n] = int32(acc)
			taps.AGCGainQ24[n] = int32(g)
		}
		// g is Q24, sTilt is Q0 → product Q24; truncate to Q0.
		prod := g * int64(sTilt[n])
		if taps != nil {
			taps.AGCOutputProductQ24[n] = prod
		}
		v := prod >> 24
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		sPf[n] = int16(v)
	}
	if g < 0 {
		g = 0
	}
	pf.agcGainPrev = int32(g)
}
