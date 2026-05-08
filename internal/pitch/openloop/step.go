package openloop

// Step runs the §A.3.3 weighted-speech pipeline + §A.3.4 open-loop
// pitch search for one 80-sample frame and returns T_op ∈ [20, 143].
//
// Inputs / state:
//
//   - aHatQ12: order-10 LP polynomial in Q12 (leading 4096 at index 0)
//     for the current frame, as produced by the LP analysis chain.
//     Per §A.3.3 the pipeline consumes Â (the quantized coefficients);
//     callers that have not yet wired LSP→LP reconstruction may pass
//     the unquantized aQ12 as a documented Phase-2b stand-in (see
//     OQ-2 / Phase 2b plan §1 line 42).
//   - s: the current 80-sample pre-processed input frame s(0..79).
//   - residualMem: 10-sample s-history (s(-10..-1)). Updated on return
//     to hold s(70..79) for the next frame.
//   - swMem: 10-sample sw-history (sw(-10..-1)). Updated on return to
//     hold the current frame's sw(70..79) for the next frame.
//   - oldWspeech: 143-sample sw history. Slid in-place per
//     slideOldWspeech (I-2b-2): on return old[0:63] holds the previous
//     old[80:143] tail and old[63:143] holds the freshly computed sw.
//
// I3 / I4: writes only through the four state pointers; all per-call
// scratch (γ-weighted LP, A'(z), residual, fresh sw, the 223-sample
// wsp composite) lives on the stack. Zero allocations.
//
// Spec cite: §A.3.3 (G729E.txt lines 2058–2086) for the weighted-
// speech construction, §A.3.4 (lines 2089–2114) for the search.
func Step(
	aHatQ12 *[11]int16,
	s *[80]int16,
	residualMem *[10]int16,
	swMem *[10]int16,
	oldWspeech *[143]int16,
) int16 {
	var aw, aPrime [11]int16
	gammaWeightLP(aHatQ12, &aw)
	combineWith07(&aw, &aPrime)

	var residual, freshSw [80]int16
	lpResidual(s, aHatQ12, residualMem, &residual)
	lowpassWeightedSpeech(&residual, &aPrime, swMem, &freshSw)

	var wsp [223]int16
	copy(wsp[:143], oldWspeech[:])
	copy(wsp[143:], freshSw[:])
	top := Search(&wsp)

	// Advance §A.3.3 filter memories for the next frame.
	copy(residualMem[:], s[70:80])
	copy(swMem[:], freshSw[70:80])
	slideOldWspeech(oldWspeech, &freshSw)

	return top
}

// StepSplit is Step with separate quantized LP polynomials for the two
// 40-sample subframes. Annex A uses interpolated quantized parameters
// for subframe 1 and current quantized parameters for subframe 2; Step
// remains as the single-polynomial compatibility entry point.
func StepSplit(
	aHatSF1Q12, aHatSF2Q12 *[11]int16,
	s *[80]int16,
	residualMem *[10]int16,
	swMem *[10]int16,
	oldWspeech *[143]int16,
) int16 {
	var aw1, aw2, aPrime1, aPrime2 [11]int16
	gammaWeightLP(aHatSF1Q12, &aw1)
	combineWith07(&aw1, &aPrime1)
	gammaWeightLP(aHatSF2Q12, &aw2)
	combineWith07(&aw2, &aPrime2)

	var residual, freshSw [80]int16
	lpResidualSplit(s, aHatSF1Q12, aHatSF2Q12, residualMem, &residual)
	lowpassWeightedSpeechSplit(&residual, &aPrime1, &aPrime2, swMem, &freshSw)

	var wsp [223]int16
	copy(wsp[:143], oldWspeech[:])
	copy(wsp[143:], freshSw[:])
	top := Search(&wsp)

	copy(residualMem[:], s[70:80])
	copy(swMem[:], freshSw[70:80])
	slideOldWspeech(oldWspeech, &freshSw)

	return top
}

func lpResidualSplit(s *[80]int16, aHat1, aHat2 *[11]int16, mem *[10]int16, r *[80]int16) {
	var mem2 [10]int16
	var s1, s2, r1, r2 [40]int16
	copy(s1[:], s[:40])
	copy(s2[:], s[40:])
	lpResidualSubframe(&s1, aHat1, mem, &r1)
	copy(mem2[:], s1[30:40])
	lpResidualSubframe(&s2, aHat2, &mem2, &r2)
	copy(r[:40], r1[:])
	copy(r[40:], r2[:])
}

func lowpassWeightedSpeechSplit(r *[80]int16, aPrime1, aPrime2 *[11]int16, mem *[10]int16, sw *[80]int16) {
	var mem2 [10]int16
	var r1, r2, sw1, sw2 [40]int16
	copy(r1[:], r[:40])
	copy(r2[:], r[40:])
	lowpassWeightedSpeechSubframe(&r1, aPrime1, mem, &sw1)
	copy(mem2[:], sw1[30:40])
	lowpassWeightedSpeechSubframe(&r2, aPrime2, &mem2, &sw2)
	copy(sw[:40], sw1[:])
	copy(sw[40:], sw2[:])
}
