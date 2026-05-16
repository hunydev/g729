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
	// recent (r̂(n-1)). These are populated lazily on the first call to
	// Decode.
	pastResiduals [4][10]int16
	// prevLSP is the previous frame's post-stability LSP vector (Q15)
	// used by interpolateLSP. The codec-start initial value is
	// populated lazily on the first Decode call.
	prevLSP [10]int16
	// prevLSF is the same previous-frame post-stability vector in LSF
	// domain (Q13). Erasure concealment reuses this state directly;
	// recomputing it from prevLSP shifts the MA predictor FIFO by a few
	// LSBs due to inverse-cosine quantization.
	prevLSF [10]int16

	lastSelector          uint8
	lastLSFAfterPredictor [10]int16
	lastLSFAfterStability [10]int16
	lastCurrLSP           [10]int16

	initialized bool
}

// initialPrevLSP is the codec-start previous-frame LSP vector (Q15).
// This fixed startup vector is pinned by the decoder_tame_lsp_pipeline
// numeric oracle; using the cosine of the initial LSF residuals here
// shifts frame-0 LP coefficients away from the reference decoder.
var initialPrevLSP = [10]int16{
	30000, 26000, 21000, 15000, 8000,
	0, -8000, -15000, -21000, -26000,
}

// initialPastResidual is the codec-start LSF residual FIFO value (Q13).
// The values are fixed startup table entries; recomputing i·π/11 with
// generic rounding shifts the MA predictor output by 1-2 LSBs.
var initialPastResidual = [10]int16{
	2339, 4679, 7018, 9358, 11698,
	14037, 16377, 18717, 21056, 23396,
}

// Reset returns the decoder to its initial state.
func (d *Decoder) Reset() {
	*d = Decoder{}
}

// PrevLSP returns the current previous-frame LSP vector used for interpolation.
// A cold decoder reports the fixed startup vector without mutating the decoder.
func (d *Decoder) PrevLSP() [10]int16 {
	if d.initialized {
		return d.prevLSP
	}
	return initialPrevLSP
}

// PrevLSF returns the current previous-frame LSF vector used for bad-frame
// LSP predictor concealment. A cold decoder reports the fixed startup vector.
func (d *Decoder) PrevLSF() [10]int16 {
	if d.initialized {
		return d.prevLSF
	}
	return initialPastResidual
}

// PastResiduals returns the current 4-tap LSP MA predictor FIFO. A cold
// decoder reports the spec startup residual state without mutating it.
func (d *Decoder) PastResiduals() [4][10]int16 {
	if d.initialized {
		return d.pastResiduals
	}
	return [4][10]int16{
		initialPastResidual,
		initialPastResidual,
		initialPastResidual,
		initialPastResidual,
	}
}

func (d *Decoder) LastLSFAfterPredictor() [10]int16 {
	return d.lastLSFAfterPredictor
}

func (d *Decoder) LastLSFAfterStability() [10]int16 {
	return d.lastLSFAfterStability
}

func (d *Decoder) LastCurrLSP() [10]int16 {
	return d.lastCurrLSP
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
		d.prevLSF = initialPastResidual
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
	d.lastLSFAfterPredictor = lsf

	// 4. Post-predictor stability enforcement.
	enforceLSFStability(&lsf)
	d.lastLSFAfterStability = lsf

	// 5. LSF → LSP per coordinate.
	var lsp [10]int16
	for i := 0; i < 10; i++ {
		lsp[i] = lsfToLSP(lsf[i])
	}
	d.lastCurrLSP = lsp

	// 6. First-frame handling: with no prior frame, use the fixed
	//    codec-start LSP init so the sf1 interpolation has the same
	//    previous-frame state as the reference decoder.
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
	d.prevLSF = lsf
	d.lastSelector = idx.L0

	return sf1, sf2
}

// DecodeErasure returns the LP filters used when the current frame is
// concealed. The LP filters reuse the previous frame's stable LSP vector,
// while the LSP MA predictor FIFO still advances using the preserved
// previous-frame stable LSF vector.
func (d *Decoder) DecodeErasure() (sf1, sf2 [11]int16) {
	if !d.initialized {
		for k := 0; k < 4; k++ {
			d.pastResiduals[k] = initialPastResidual
		}
		d.prevLSP = initialPrevLSP
		d.prevLSF = initialPastResidual
		d.initialized = true
	}

	prevLSF := d.prevLSF
	d.lastLSFAfterPredictor = prevLSF
	d.lastLSFAfterStability = prevLSF
	d.lastCurrLSP = d.prevLSP
	d.advanceErasurePredictor(&prevLSF)

	var lspSF1, lspSF2 [10]int16
	interpolateLSP(&d.prevLSP, &d.prevLSP, &lspSF1, &lspSF2)
	LSPToLP(&lspSF1, &sf1)
	LSPToLP(&lspSF2, &sf2)
	return sf1, sf2
}

func (d *Decoder) advanceErasurePredictor(lsf *[10]int16) {
	var residual [10]int16
	inversePredictorResidual(d.lastSelector, lsf, &d.pastResiduals, &residual)
	d.pastResiduals[3] = d.pastResiduals[2]
	d.pastResiduals[2] = d.pastResiduals[1]
	d.pastResiduals[1] = d.pastResiduals[0]
	d.pastResiduals[0] = residual
}
