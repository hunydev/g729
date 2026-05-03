package g729

import (
	"github.com/exedev/g729/internal/acelp"
	"github.com/exedev/g729/internal/filter"
	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
)

// Encoder holds G.729 Annex A encoder state for one logical stream.
//
// All buffers are preallocated; EncodeFrame allocates 0 in steady state.
// Concurrent calls on the same Encoder are a data race; callers needing
// parallel encoding must own one Encoder per channel.
type Encoder struct {
	pre pcm.PreProcessor

	// §5.3 preallocated histories.
	oldSpeech  [240]int16
	oldWspeech [143]int16
	oldExc     [154]int16
	synMem     [10]int16
	wMem       [10]int16
	errMem     [10]int16
	lspOld     [10]int16
	lspOldQ    [10]int16
	pastQuaEn  [4]int16
	freqPrev   [4][10]int16

	// Per-block state owners.
	lpc    lpc.Analyzer
	acelp  acelp.Searcher
	weight filter.Weighting
}

// NewEncoder returns an Encoder in initial state.
func NewEncoder() *Encoder {
	return &Encoder{}
}

// Reset returns the Encoder to initial state. Equivalent to using a fresh
// NewEncoder, but reuses the existing memory.
func (e *Encoder) Reset() {
	*e = Encoder{}
}

// EncodeFrame consumes exactly FrameSamples samples and writes exactly
// FrameBytes bytes to out. Internal state is retained across calls.
//
// Phase 2-0 stub: validates lengths and returns ErrNotImplemented. Real
// encoding is wired in Phase 2a..2f.
func (e *Encoder) EncodeFrame(pcm []int16, out []byte) error {
	if len(pcm) != FrameSamples {
		return ErrShortPCM
	}
	if len(out) < FrameBytes {
		return ErrShortOutput
	}
	return ErrNotImplemented
}
