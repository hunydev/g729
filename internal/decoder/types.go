package decoder

import (
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/postfilter"
	"github.com/exedev/g729/internal/synth"
)

const (
	frameSamples = 80
	subframeLen  = 40
	lpcOrder     = 10
	pitchMax     = 143
	pastExcLen   = pitchMax + 10 // 153 — see AdaptiveCodebook doc
)

// Decoder is the per-stream G.729 Annex A decoder. The zero value is a
// valid initial state (see §4.3). Not safe for concurrent use; one
// Decoder per active call.
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

// Reset returns the decoder to its zero initial state.
func (d *Decoder) Reset() {
	*d = Decoder{}
}
