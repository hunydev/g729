package decoder

import "errors"

// ErrShortInput is returned by Decoder.Decode when the packed-frame
// byte slice is shorter than one G.729 frame (bitstream.FrameBytes = 10).
// Callers should ensure their transport layer assembles full 80-bit
// frames before calling Decode.
var ErrShortInput = errors.New("decoder: packed frame shorter than 10 bytes")

// ErrShortOutput is returned by Decoder.Decode when the PCM output
// slice is shorter than one G.729 frame (80 int16 samples at 8 kHz).
// Callers should pre-size the output buffer to a multiple of 80 samples.
var ErrShortOutput = errors.New("decoder: PCM output shorter than 80 samples")
