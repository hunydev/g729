// Package g729 sentinel errors.
//
// All errors returned by the public API are exported sentinel values.
// Per design §6, the codec never panics, never logs, and never wraps
// internal errors — DSP overflow is absorbed by saturating fixed-point
// arithmetic, so the only failure modes are contract violations at the
// API boundary.
package g729

import "errors"

var (
	// ErrShortPCM is returned by EncodeFrame when len(pcm) != FrameSamples.
	ErrShortPCM = errors.New("g729: input PCM length not multiple of frame size (80)")

	// ErrShortOutput is returned by EncodeFrame when the output buffer
	// cannot hold FrameBytes.
	ErrShortOutput = errors.New("g729: output buffer too small")

	// ErrShortBitstream is returned by DecodeFrame and Decode when the
	// input length is not a multiple of FrameBytes.
	ErrShortBitstream = errors.New("g729: bitstream length not multiple of 10 bytes")
)

// Public frame-shape constants.
const (
	SampleRate   = 8000 // Hz, fixed by spec.
	FrameSamples = 80   // 10 ms at 8 kHz.
	FrameBytes   = 10   // 80 bits packed per frame.
)
