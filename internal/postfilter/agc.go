package postfilter

// agcAlphaQ15 is the Annex A adaptive-gain smoothing factor α = 0.9 in
// Q15. The main G.729 postfilter uses a slower α, but Annex A §A.4.2.4
// tightens the smoother to track the lower-complexity decoder envelope.
const agcAlphaQ15 int64 = 29491

// computeAGCTargetGain returns the per-sample AGC target increment (Q14)
// derived from √(E(s) / E(sTilt)) and the Annex A smoothing complement.
//
// Implementation: accumulate E_s and E_t as sum-of-squares in int64,
// compute (E_s / E_t) at Q28, then take an integer square root (Newton-
// Raphson) yielding the result at Q14. Phase 1g may swap in fixed.Sqrt
// or a spec-specified table for ITU bit-exactness.
func (pf *Postfilter) computeAGCTargetGain(s, sTilt *[subframeLen]int16) int16 {
	var eS, eT int64
	for i := 0; i < subframeLen; i++ {
		eS += int64(s[i]) * int64(s[i])
		eT += int64(sTilt[i]) * int64(sTilt[i])
	}
	eS /= subframeLen
	eT /= subframeLen
	if eT == 0 {
		return 0
	}
	ratioQ28 := (eS << 28) / eT
	if ratioQ28 < 0 {
		return 0
	}
	targetQ14 := isqrtQ14(ratioQ28)
	return int16((int64(targetQ14) * (32768 - agcAlphaQ15)) >> 15)
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
