package decoder

import "errors"

// ErrShortInput is returned when the packed-frame byte slice is
// shorter than bitstream.FrameBytes (10).
var ErrShortInput = errors.New("decoder: packed frame shorter than 10 bytes")

// ErrShortOutput is returned when the PCM output slice is shorter
// than one G.729 frame (80 int16 samples).
var ErrShortOutput = errors.New("decoder: PCM output shorter than 80 samples")
