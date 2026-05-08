package decoder

import (
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/postfilter"
	"github.com/hunydev/g729/internal/synth"
)

const (
	frameSamples = 80
	subframeLen  = 40
	lpcOrder     = 10
	pitchMax     = 143
	pastExcLen   = pitchMax + 10 // 153 — see AdaptiveCodebook doc
)

// Decoder is the per-stream G.729 / Annex A decoder.
//
// The zero value is a valid initial state per ITU-T G.729 §4.3; callers
// may use `var d decoder.Decoder` directly without an explicit
// constructor. All sub-state (LSP MA predictor, gain MA predictor,
// synthesizer memory, postfilter memory, HP-filter memory, past-excitation
// FIFO) is owned by the Decoder value and is reset by Reset.
//
// A Decoder is bound to a single audio stream and must be driven in
// frame order by sequential calls to Decode. It is not safe for
// concurrent use; allocate one Decoder per active call / stream.
type Decoder struct {
	lsp lsp.Decoder
	gn  gain.Decoder
	syn synth.Synthesizer
	pst postfilter.Postfilter

	pastExc [pastExcLen]int16

	prevGpQ14 int16

	hpX [2]int16
	hpY [2]int32

	initialized bool
}

// Reset returns the decoder to its zero initial state, discarding all
// per-stream history (LSP / gain MA predictors, synthesizer and postfilter
// memories, HP-filter taps, past-excitation FIFO). Call Reset before
// reusing a Decoder for a new stream; it is equivalent to `*d = Decoder{}`.
func (d *Decoder) Reset() {
	*d = Decoder{}
}
