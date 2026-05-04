package decoder

// PastExcSnapshot returns a defensive copy of the adaptive-codebook
// past-excitation FIFO d.pastExc (Q0 int16, length pastExcLen = 153).
//
// Layout (per AdaptiveCodebook docstring): d.pastExc[pastExcLen-1] is
// u(-1), the most recent past sample. After Decode returns at end of
// frame m, the trailing 80 samples [pastExcLen-80 : pastExcLen] hold
// u_m(0..79) of the just-decoded frame in transmission order
// (sf-1 ⊕ sf-2 of frame m). Older samples occupy the head.
//
// Phase 3b DIAG-3 helper. Read-only — does not advance any state.
// Test-only consumer:
// internal/decoder/phase3b_diag3_acfifo_test.go.
func (d *Decoder) PastExcSnapshot() [pastExcLen]int16 {
	return d.pastExc
}

// PastExcLenForDiag exposes pastExcLen for sizing diagnostic-side
// buffers without re-deriving the constant. Test-only.
func PastExcLenForDiag() int { return pastExcLen }
