package pcm

// FrameLength is the number of PCM samples in one G.729 frame
// (10 ms at 8 kHz).
const FrameLength = 80

// PreProcessor applies the ITU-T G.729 §3.1.1 input pre-processing:
// a second-order pole-zero high-pass filter at ~140 Hz with the 1/2
// input scaling folded into the numerator coefficients.
//
// Filter memory is kept across calls so that a 10 ms frame fed one
// frame at a time produces the same output as feeding the whole signal
// in one call. The zero value is usable and equivalent to calling
// Reset on a fresh instance.
//
// PreProcessor is not safe for concurrent use. Each channel of a
// multi-channel encoder should own a dedicated PreProcessor.
type PreProcessor struct {
	// x1, x2 are the two previous input samples (Q0 int16 widened to
	// int32 only for uniform field typing; the low 16 bits are the
	// meaningful value).
	x1, x2 int32

	// y1, y2 are the two previous output values kept in accumulator
	// precision (Q(BQ), i.e. Q13 with the default coefficient
	// Q-format) so that rounding error is not fed back into the next
	// step. The Q15 representation of the rounded output is derived
	// from these on demand.
	y1, y2 int32
}

// Reset returns the filter to its initial state (all memory zero). A
// PreProcessor should be Reset before the first call when an instance
// is being reused across independent streams.
func (p *PreProcessor) Reset() {
	*p = PreProcessor{}
}

// Process applies the pre-processing filter to in, writing the result
// to out. in and out must have the same length and may alias. Process
// allocates nothing.
//
// At the skeleton stage, Process just zeros out.
func (p *PreProcessor) Process(in, out []int16) {
	n := len(in)
	if len(out) < n {
		n = len(out)
	}
	for i := 0; i < n; i++ {
		out[i] = 0
	}
}
