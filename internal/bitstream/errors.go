package bitstream

import "errors"

// ErrShortOutput is returned when an output buffer is smaller than the
// required fixed size (FrameBytes for Pack, 80 samples for Decode).
var ErrShortOutput = errors.New("bitstream: output buffer too small")

// ErrShortInput is returned when an input buffer is shorter than the
// required fixed size (FrameBytes for Unpack).
var ErrShortInput = errors.New("bitstream: input buffer too short")

// ErrBadG192Sync is returned when a G.192 frame's sync word is neither
// the good-frame nor bad-frame marker.
var ErrBadG192Sync = errors.New("bitstream: invalid G.192 sync word")

// ErrBadG192Length is returned when a G.192 frame's length word does
// not equal FrameBits.
var ErrBadG192Length = errors.New("bitstream: invalid G.192 length word")

// ErrBadG192Bit is returned when a G.192 data word is neither the
// 0-bit nor the 1-bit marker.
var ErrBadG192Bit = errors.New("bitstream: invalid G.192 data word")
