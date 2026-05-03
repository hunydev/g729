package lsp

// Decoder reconstructs the quantized LP filter from the 18-bit LSP
// parameters of one G.729 frame. It carries the state required across
// frames: past quantized residuals for the MA predictor, and the
// previous frame's LSP vector for inter-frame interpolation.
//
// The zero value is a valid Reset state.
type Decoder struct {
	// pastResiduals holds the 4 previous frames' quantized residual
	// vectors r̂(n-1)..r̂(n-4), all Q13. pastResiduals[0] is the most
	// recent (r̂(n-1)). Per ITU-T G.729 §3.2.4, the initial values for
	// k < 0 are l̂_i = i·π/11 (Q13). These are populated lazily on the
	// first call to Decode.
	pastResiduals [4][10]int16
	// prevLSP is the previous frame's post-stability LSP vector (Q15)
	// used by interpolateLSP. Per ITU-T G.729 §3.2.4 / §4.1.5, the
	// codec-start initial values are q_i = cos(i·π/11) Q15 (the
	// uniformly-spaced LSF init projected through cosine), populated
	// lazily on the first Decode call.
	prevLSP [10]int16

	initialized bool
}

// initialPrevLSP is q_i = cos(i·π/11) Q15 for i = 1..10, the codec-
// start initial value of the previous-frame LSP vector (ITU-T G.729
// §3.2.4 / §4.1.5).
var initialPrevLSP = [10]int16{
	31441, 27566, 21458, 13612, 4663,
	-4663, -13612, -21458, -27566, -31441,
}

// initialPastResidual is l̂_i = i·π/11 in Q13 for i = 1..10, per
// ITU-T G.729 §3.2.4. π in Q13 is 25736, so the i-th entry is
// round(i · 25736 / 11).
var initialPastResidual = [10]int16{
	2340,  // 1·π/11
	4679,  // 2·π/11
	7019,  // 3·π/11
	9359,  // 4·π/11
	11698, // 5·π/11
	14038, // 6·π/11
	16377, // 7·π/11
	18717, // 8·π/11
	21057, // 9·π/11
	23396, // 10·π/11
}

// Reset returns the decoder to its initial state.
func (d *Decoder) Reset() {
	*d = Decoder{}
}

// Decode reconstructs the per-subframe LP filter coefficients for one
// frame. sf1 is the interpolated LP for the first subframe, sf2 is the
// current-frame LP for the second subframe. Both are Q12 with
// sf1[0] = sf2[0] = 4096 (i.e. 1.0).
//
// Decode allocates nothing.
func (d *Decoder) Decode(idx Indices) (sf1, sf2 [11]int16) {
	if !d.initialized {
		for k := 0; k < 4; k++ {
			d.pastResiduals[k] = initialPastResidual
		}
	}

	// 1. Split-VQ combine: l̂_i = L1_i + L2_i (or L3_{i-5}).
	var residual [10]int16
	combineResidual(idx.L1, idx.L2, idx.L3, &residual)

	// 2. Pre-predictor pair-rearrangement on the residual: J=0.0012
	//    then J=0.0006 (ITU-T G.729 §3.2.4).
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)

	// 3. MA predictor reconstruction: ω̂ from current residual + past
	//    residuals (FIFO advance happens inside applyPredictor).
	var lsf [10]int16
	d.applyPredictor(idx.L0, &residual, &lsf)

	// 4. Post-predictor stability enforcement.
	enforceLSFStability(&lsf)

	// 5. LSF → LSP per coordinate.
	var lsp [10]int16
	for i := 0; i < 10; i++ {
		lsp[i] = lsfToLSP(lsf[i])
	}

	// 6. First-frame handling: with no prior frame, use the spec-
	//    defined uniform LSP init (cos(i·π/11) Q15) so the sf1
	//    interpolation produces a stable LP filter even at codec start.
	if !d.initialized {
		d.prevLSP = initialPrevLSP
		d.initialized = true
	}

	// 7. Per-subframe LSP interpolation.
	var lspSF1, lspSF2 [10]int16
	interpolateLSP(&d.prevLSP, &lsp, &lspSF1, &lspSF2)

	// 8. LSP → LP per subframe.
	LSPToLP(&lspSF1, &sf1)
	LSPToLP(&lspSF2, &sf2)

	// 9. Save for next frame's interpolation.
	d.prevLSP = lsp

	return sf1, sf2
}
