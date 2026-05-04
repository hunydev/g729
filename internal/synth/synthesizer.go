package synth

// Synthesize produces one subframe of synthesized speech by composing
// BuildExcitation and the LP synthesis filter.
//
// Inputs:
//
//	a          — LP coefficients for this subframe (Q12, a[0] = 4096)
//	v          — adaptive codebook vector (Q0, from internal/pitch)
//	c          — fixed codebook vector (Q13, from internal/fcb)
//	gpQ14      — adaptive codebook gain (Q14, from internal/gain)
//	gcMantQ14  — fixed codebook gain mantissa (Q14, from internal/gain)
//	gcExp      — fixed codebook gain exponent (int8, from internal/gain)
//
// Output:
//
//	s        — synthesized speech samples (Q0, pre-postfilter)
//
// Spec: ITU-T G.729 §4.1.2 / §4.1.6. Updates synth.pastSynth to the last
// 10 samples of s, ready for the next subframe.
//
// Zero-allocation: the intermediate excitation lives on this function's stack.
func (synth *Synthesizer) Synthesize(a *[11]int16, v, c *[40]int16, gpQ14, gcMantQ14 int16, gcExp int8, s *[40]int16) {
	var u [40]int16
	BuildExcitation(gpQ14, gcMantQ14, gcExp, v, c, &u)
	synth.filterSubframe(a, &u, s)
}

// Filter runs the LP synthesis filter 1/A(z) on a pre-built excitation
// vector u and writes the synthesized speech samples (Q0, pre-postfilter)
// to out. This is the counterpart to Synthesize when the caller needs
// u separately — e.g. the top-level decoder appends u to the adaptive-
// codebook history FIFO.
//
// Spec: ITU-T G.729 §4.1.2 / §3.10. Updates synth.pastSynth to the last
// 10 samples of out. Zero-allocation.
func (synth *Synthesizer) Filter(a *[11]int16, u, out *[40]int16) {
synth.filterSubframe(a, u, out)
}
