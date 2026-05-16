package synth

// Synthesizer holds the per-channel LP synthesis filter state.
//
// The zero value is ready for use: pastSynth is all zero, matching the
// G.729 §4.3 first-frame initial condition.
//
// Not safe for concurrent use. Caller owns one Synthesizer per decoder stream.
type Synthesizer struct {
	// pastSynth stores the 10 most recent output samples of Synthesize in Q0.
	// Indexing: pastSynth[0] = s(n-10) (oldest), pastSynth[9] = s(n-1) (most recent).
	pastSynth                [10]int16
	lastExcitationScaleShift uint
}

// Reset clears the synthesis-filter state to the all-zero initial condition
// specified by ITU-T G.729 §4.3.
func (synth *Synthesizer) Reset() {
	*synth = Synthesizer{}
}

// PastSynth returns the current 10-sample synthesis memory without mutating
// the filter state.
func (synth *Synthesizer) PastSynth() [10]int16 {
	return synth.pastSynth
}

// LastExcitationScaleShift reports whether the most recent Filter call used
// the overflow-recovery excitation scale-down path. A value of 2 means the
// caller must commit u>>2 into the adaptive-codebook history.
func (synth *Synthesizer) LastExcitationScaleShift() uint {
	return synth.lastExcitationScaleShift
}
