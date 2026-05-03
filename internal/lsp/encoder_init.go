package lsp

// InitFreqPrev populates the encoder-side MA-predictor FIFO
// (freqPrev[0..3][0..9]) with the codec-start initial values per
// ITU-T G.729 §3.2.4. All four past frames are seeded with the
// uniformly-spaced LSF init l̂_i = i·π/11 in Q13 (the same constant
// the decoder uses on cold start; see decoder.initialPastResidual).
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
// codec-start initial LSP vector q_i = cos(i·π/11) Q15 per ITU-T G.729
// §3.2.6 / §4.1.5. Callers must invoke this exactly once after Encoder
// construction (and after Reset) so that the FIX-3-B anti-palindromic
// LP guard in lpcStep has a spec-aligned fallback if the very first
// frame triggers an F1/F2 sign-change deficit.
//
// I3 / I4: pure write-through, no allocation.
func InitLSPOld(lspOld *[10]int16) {
	*lspOld = initialPrevLSP
}
