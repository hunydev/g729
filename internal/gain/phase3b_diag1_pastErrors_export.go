package gain

// PastErrorsSnapshot returns a defensive copy of the 4-tap MA-predictor
// state Û(m-1..m-4) at Q10 dB (ITU-T G.729 §3.9.1 eq. (69), §4.3
// Table 9 cold-start value −14 dB).
//
// Phase 3b DIAG-1 helper. Read-only — does not advance the predictor
// nor mark the decoder initialized. Test-only consumer:
// internal/decoder/phase3b_diag1_pasterrors_test.go.
//
// Returned ordering matches d.pastErrors: index 0 = Û(m-1) (most
// recent), index 3 = Û(m-4) (oldest). Before the first Decode call
// the snapshot reflects the zero-value state; the spec seed (−14336)
// is written lazily on the first Decode invocation.
func (d *Decoder) PastErrorsSnapshot() [4]int16 {
	return d.pastErrors
}

// Initialized reports whether the decoder's MA-predictor FIFO has been
// seeded with the §4.3 Table 9 cold-start value (−14336 = −14 dB Q10).
// Phase 3b DIAG-1 helper for distinguishing pre-first-Decode zero state
// from steady-state state.
func (d *Decoder) Initialized() bool {
	return d.initialized
}
