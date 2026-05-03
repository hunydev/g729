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
