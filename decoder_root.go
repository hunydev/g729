package g729

import (
	"github.com/exedev/g729/internal/decoder"
)

// Decoder holds G.729 Annex A decoder state for one logical stream.
type Decoder struct {
	inner decoder.Decoder
}

// NewDecoder returns a Decoder in initial state.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Reset returns the Decoder to initial state.
func (d *Decoder) Reset() {
	d.inner.Reset()
}

// DecodeFrame consumes exactly FrameBytes bytes and writes exactly
// FrameSamples samples to out.
//
// Per Phase 2-0 plan note (Task 2-0-6), the inner decoder API is
// internal/decoder.Decoder.Decode(packed, bad, out); this shell forwards
// with bad=false and adds the public-API len validation.
func (d *Decoder) DecodeFrame(bits []byte, out []int16) error {
	if len(bits) != FrameBytes {
		return ErrShortBitstream
	}
	if len(out) < FrameSamples {
		return ErrShortOutput
	}
	return d.inner.Decode(bits, false, out)
}
