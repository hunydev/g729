// Package g729 is a pure-Go, clean-room implementation of the
// ITU-T G.729 Annex A 8 kbps CS-ACELP speech codec.
//
// The intended deployment target is the server-side audio egress path
// of MRCP / TTS / IVR systems on RTP payload type 18 ("G729/8000").
//
// # Frame shape
//
// G.729 is strictly frame-based at 10 ms boundaries:
//
//   - Input PCM:  exactly FrameSamples (80) int16 samples per frame,
//     8 kHz mono, native-endian.
//   - Encoded:    exactly FrameBytes (10) bytes per frame, packed per
//     §4.2.1 / Table 8.
//
// All public API entry points enforce these lengths and return one of
// the exported sentinel errors (ErrShortPCM, ErrShortOutput,
// ErrShortBitstream, ErrNoStreamSink) on contract violation. The
// codec never panics and never wraps internal errors; DSP overflow is
// absorbed by saturating fixed-point arithmetic at every stage.
//
// # Public API
//
// Frame-at-a-time:
//
//   - Encoder, NewEncoder, (*Encoder).EncodeFrame, (*Encoder).Reset
//   - Decoder, NewDecoder, (*Decoder).DecodeFrame, (*Decoder).Reset
//   - EncodeFrame, DecodeFrame (top-level convenience wrappers)
//
// Streaming (encoder side only; the decoder is naturally frame-driven):
//
//   - NewStreamingEncoder, (*Encoder).Write, (*Encoder).Flush
//
// Frame-shape constants:
//
//   - SampleRate (8000), FrameSamples (80), FrameBytes (10)
//
// EncodeFrame and DecodeFrame are zero-allocation in steady state;
// see the v0.1.0-rc1 release verification log for hot-path benchmarks.
//
// # Concurrency
//
// Each Encoder and each Decoder instance is single-threaded. Concurrent
// calls on the same instance are a data race; callers needing parallel
// streams must own one Encoder and one Decoder per channel. There is
// no shared mutable state across instances; multiple instances can run
// concurrently on different goroutines.
//
// # SDP / RTP packetization
//
// The codec produces and consumes RTP-suitable 10-byte payloads. RTP
// header construction belongs to the caller. SDP must advertise
// "annexb=no" because Annex B (SID / CNG / DTX) is not implemented.
// See README.md and examples/ for ptime=10 and ptime=20 SDP fragments
// and an illustrative payload bundler.
//
// # Conformance scope
//
// This codec is shipped under a spec-compliance binding criterion:
// the decoder is verified spec-correct against ITU-T G.729 (06/2012)
// + Annex A across seven independent diagnostic axes (see
// docs/superpowers/plans/2026-05-04-phase3-final-closure-report.md).
// It does not claim ITU byte-exact conformance against the ITU
// SPEECH.PST reference, and it does not implement Annex B,
// G.729.1, G.729D, or G.729E. See README.md "Known limitations"
// for the canonical disclosure.
//
// # Clean-room declaration
//
// This project maintains a clean-room constraint. No ITU reference C,
// bcg729, FFmpeg, Sipro, or other G.729 implementation source was used.
// Public specifications, test vectors, and independently written tests
// were used. v0.1.0 does not claim ITU byte-exact conformance.
package g729
