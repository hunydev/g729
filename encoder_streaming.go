package g729

import (
	"errors"
	"io"
)

// ErrNoStreamSink is returned by (*Encoder).Write when called on an
// Encoder that was not constructed via NewStreamingEncoder (i.e. the
// streaming sink is nil).
var ErrNoStreamSink = errors.New("g729: encoder has no streaming sink (use NewStreamingEncoder)")

// NewStreamingEncoder returns an Encoder whose (*Encoder).Write and
// (*Encoder).Flush methods emit packed 10-byte G.729 frames to w, using
// EncoderProfileQuality.
//
// Plan §6 Task API-2 + design spec §4.3. Buffers up to FrameSamples-1
// samples between calls; emits one frame per FrameSamples-sample
// boundary. Flush zero-pads any trailing partial frame per
// OQ-FLUSH-PAD (plan §10 = zero-pad with 0x0000 silence). Reset clears
// codec state and any buffered tail but keeps this sink attached.
func NewStreamingEncoder(w io.Writer) *Encoder {
	e := NewEncoder()
	e.streamSink = w
	return e
}

// NewStreamingEncoderWithProfile is NewStreamingEncoder with an explicit
// encoder search profile.
func NewStreamingEncoderWithProfile(w io.Writer, profile EncoderProfile) *Encoder {
	e := NewEncoderWithProfile(profile)
	e.streamSink = w
	return e
}

// Write consumes int16 PCM samples and emits one packed 10-byte G.729
// frame to the encoder's streaming sink for every FrameSamples-sample
// boundary crossed. Up to FrameSamples-1 trailing samples are buffered
// for the next call. Returns the number of int16 samples consumed,
// which is always len(samples) on success.
//
// Per plan §1 I-2f-3 + I-2f Streaming framing semantics. Steady-state
// (full-frame) calls are zero-allocation: the per-frame packed-byte
// scratch lives on the Encoder receiver (e.streamPacked) so the
// streamSink.Write interface call cannot escape its slice header to
// the heap.
func (e *Encoder) Write(samples []int16) (int, error) {
	if e.streamSink == nil {
		return 0, ErrNoStreamSink
	}
	consumed := 0

	// Drain any pre-existing partial buffer first.
	if e.streamBufLen > 0 {
		need := FrameSamples - e.streamBufLen
		if len(samples) < need {
			copy(e.streamBuf[e.streamBufLen:], samples)
			e.streamBufLen += len(samples)
			return len(samples), nil
		}
		copy(e.streamBuf[e.streamBufLen:], samples[:need])
		if err := e.EncodeFrame(e.streamBuf[:], e.streamPacked[:]); err != nil {
			return consumed, err
		}
		if _, err := e.streamSink.Write(e.streamPacked[:]); err != nil {
			return consumed, err
		}
		e.streamBufLen = 0
		samples = samples[need:]
		consumed += need
	}

	// Fast path: encode aligned frames directly from the input slice.
	for len(samples) >= FrameSamples {
		if err := e.EncodeFrame(samples[:FrameSamples], e.streamPacked[:]); err != nil {
			return consumed, err
		}
		if _, err := e.streamSink.Write(e.streamPacked[:]); err != nil {
			return consumed, err
		}
		samples = samples[FrameSamples:]
		consumed += FrameSamples
	}

	// Buffer any trailing partial frame for the next Write/Flush.
	if len(samples) > 0 {
		copy(e.streamBuf[:], samples)
		e.streamBufLen = len(samples)
		consumed += len(samples)
	}
	return consumed, nil
}

// Flush drains any partial trailing frame in the stream buffer by
// zero-padding it to FrameSamples and emitting one final packed frame
// to the streaming sink. No-op when the buffer is empty.
//
// OQ-FLUSH-PAD pin (plan §10): zero-pad with 0x0000 (linear-PCM
// silence). Callers MUST eventually call Flush to drain any tail.
func (e *Encoder) Flush() error {
	if e.streamSink == nil {
		return ErrNoStreamSink
	}
	if e.streamBufLen == 0 {
		return nil
	}
	for i := e.streamBufLen; i < FrameSamples; i++ {
		e.streamBuf[i] = 0
	}
	if err := e.EncodeFrame(e.streamBuf[:], e.streamPacked[:]); err != nil {
		return err
	}
	if _, err := e.streamSink.Write(e.streamPacked[:]); err != nil {
		return err
	}
	e.streamBufLen = 0
	return nil
}
