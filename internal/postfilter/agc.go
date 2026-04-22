package postfilter

// computeAGCTargetGain returns the AGC target gain g_target (Q14) per
// ITU-T G.729 §A.4.2.4 as √(E(s) / E(sTilt)).
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
	if eT == 0 {
		return 0
	}
	ratioQ28 := (eS << 28) / eT
	if ratioQ28 < 0 {
		return 0
	}
	return isqrtQ14(ratioQ28)
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

// applyAGC smooths g_target into agcGainPrev (one-pole lowpass, α ≈ 0.99)
// and scales sTilt to produce sPf per ITU-T G.729 §A.4.2.4.
func (pf *Postfilter) applyAGC(sTilt *[subframeLen]int16, gTargetQ14 int16, sPf *[subframeLen]int16) {
	const alphaQ15 int64 = 32440 // ≈ 0.99; ITU-T G.729 §A.4.2.4

	gTargetQ24 := int64(gTargetQ14) << 10
	if !pf.initialized {
		pf.agcGainPrev = int32(gTargetQ24)
		pf.initialized = true
	}

	g := int64(pf.agcGainPrev) // Q24
	for n := 0; n < subframeLen; n++ {
		g = (alphaQ15*g + (32768-alphaQ15)*gTargetQ24 + (1 << 14)) >> 15
		// g is Q24, sTilt is Q0 → product Q24; round to Q0.
		prod := g * int64(sTilt[n])
		v := (prod + (1 << 23)) >> 24
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
