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
	// State fields are filled in by a later task. They are intentionally
	// unnamed here so the skeleton compiles without prejudging the
	// Q-format of the state.
	_unused byte
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
