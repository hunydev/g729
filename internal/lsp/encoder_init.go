package lsp

// InitFreqPrev populates the encoder-side MA-predictor FIFO
// (freqPrev[0..3][0..9]) with the same codec-start residual vector the
// decoder uses on cold start.
//
// Callers must invoke this exactly once after Encoder construction
// (and after Reset) before the first lpcStep / Quantize call. A
// zero-initialized FIFO would bias the MA prediction toward q ≈ 1
// (cos(0)) and produce systematically wrong L0/L1 selections on the
// opening frames.
//
// I3 / I4: pure write-through, no allocation.
func InitFreqPrev(freqPrev *[4][10]int16) {
	for k := 0; k < 4; k++ {
		freqPrev[k] = initialPastResidual
	}
}

// InitLSPOld seeds the encoder-side previous-frame LSP cache with the
// same fixed codec-start vector used by the decoder. Callers must
// invoke this exactly once after Encoder construction (and after
// Reset) so that the FIX-3-B anti-palindromic LP guard in lpcStep has
// a stable fallback if the very first frame triggers an F1/F2
// sign-change deficit.
//
// I3 / I4: pure write-through, no allocation.
func InitLSPOld(lspOld *[10]int16) {
	*lspOld = initialPrevLSP
}
