package decoder

import "errors"

// errNotImplemented is a placeholder used until Task 6 wires Decode for
// real. No test asserts on its identity.
var errNotImplemented = errors.New("decoder: Decode not yet implemented")

// Decode consumes one packed G.729 frame (10 bytes) and writes 80
// samples of 16-bit PCM to out. bad signals a frame-erasure marker
// from the transport layer; Phase 1g treats it as a no-op (erasure
// concealment arrives in Phase 1h).
//
// Returns ErrShortInput / ErrShortOutput for undersized slices.
func (d *Decoder) Decode(packed []byte, bad bool, out []int16) error {
	if len(packed) < 10 {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}
	_ = bad
	return errNotImplemented
}
