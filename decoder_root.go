package g729

import (
	"github.com/hunydev/g729/internal/decoder"
)

// Decoder holds G.729 Annex A decoder state for one logical stream.
//
// All buffers are preallocated; DecodeFrame allocates 0 in steady state.
// Concurrent calls on the same Decoder are a data race; callers needing
// parallel decoding must own one Decoder per channel.
type Decoder struct {
	inner decoder.Decoder
}

// NewDecoder returns a Decoder in initial state.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Reset returns the Decoder to initial state. Equivalent to using a
// fresh NewDecoder, but reuses the existing memory.
func (d *Decoder) Reset() {
	d.inner.Reset()
}

// DecodeFrame consumes exactly FrameBytes (10) bytes from bits and
// writes exactly FrameSamples (80) int16 samples (8 kHz mono) to out.
//
// Returns ErrShortBitstream if len(bits) != FrameBytes, or
// ErrShortOutput if len(out) < FrameSamples. Internal state (LSP
// history, adaptive codebook, postfilter memories) is retained across
// calls. The decoder treats every frame as good; bad-frame /
// erasure-concealment handling is out of scope for v0.1.0.
func (d *Decoder) DecodeFrame(bits []byte, out []int16) error {
	if len(bits) != FrameBytes {
		return ErrShortBitstream
	}
	if len(out) < FrameSamples {
		return ErrShortOutput
	}
	return d.inner.Decode(bits, false, out)
}

// DecodeFrameEnhanced is an opt-in, non-strict decoder path for local
// listening diagnostics. Use DecodeFrame when strict decoder behavior is
// required; use this only as an audible fallback while the decoder core is
// still under black-box verification.
func (d *Decoder) DecodeFrameEnhanced(bits []byte, out []int16) error {
	if len(bits) != FrameBytes {
		return ErrShortBitstream
	}
	if len(out) < FrameSamples {
		return ErrShortOutput
	}
	return d.inner.DecodeEnvelopeRecovered(bits, false, out)
}
