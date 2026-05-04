package lsp

// PrevLSPSnapshot returns a defensive copy of the inter-frame LSP
// state q̂_i^(previous) (Q15, cosine domain) carried by the LSP
// decoder per ITU-T G.729 §3.2.5 / §4.1.5 (subframe-1 interpolation
// input). After Decode returns, this snapshot equals q̂_i^(current)
// of the just-decoded frame (the `lsp` vector saved into d.prevLSP at
// step 9 of decoder.go).
//
// Phase 3b DIAG-2 helper. Read-only — does not advance the predictor
// nor mark the decoder initialized. Test-only consumer:
// internal/decoder/phase3b_diag2_lpinterp_test.go.
//
// Returned ordering matches d.prevLSP: index 0 = q̂_1, index 9 = q̂_10
// (cosine domain, monotonically decreasing for a well-formed LSP).
func (d *Decoder) PrevLSPSnapshot() [10]int16 {
	return d.prevLSP
}

// InitializedForDiag reports whether d.prevLSP holds a Decode-derived
// LSP vector (true) or the zero / cold-start state (false). Used by
// Phase 3b DIAG-2 to distinguish the pre-first-Decode snapshot from
// steady state.
func (d *Decoder) InitializedForDiag() bool {
	return d.initialized
}
