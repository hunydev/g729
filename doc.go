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
// ErrShortBitstream, ErrUnsupportedAnnexB, ErrNoStreamSink) on contract
// violation. The codec never panics and never wraps internal errors; DSP
// overflow is absorbed by saturating fixed-point arithmetic at every stage.
//
// # Public API
//
// Frame-at-a-time:
//
//   - Encoder, NewEncoder, NewEncoderWithProfile,
//     (*Encoder).EncodeFrame, (*Encoder).Reset
//   - Decoder, NewDecoder, (*Decoder).DecodeFrame, (*Decoder).Reset
//   - (*Decoder).DecodeFrameEnhanced and
//     (*Decoder).DecodeFramePostfilterBlend for listening diagnostics
//   - EncodeFrame, DecodeFrame (top-level convenience wrappers)
//
// Streaming (encoder side only; the decoder is naturally frame-driven):
//
//   - NewStreamingEncoder, NewStreamingEncoderWithProfile,
//     (*Encoder).Write, (*Encoder).Flush
//   - Write, Flush (top-level convenience wrappers)
//
// Frame-shape constants:
//
//   - SampleRate (8000), FrameSamples (80), FrameBytes (10)
//
// NewEncoder and NewStreamingEncoder use EncoderProfileCore, which keeps the
// emitted bitstream standard-compatible while avoiding repository-local
// quality heuristics that can sound processed in blind listening. It keeps the
// focused fixed-codebook threshold-search frame budget and sequential Annex A
// LSP VQ path, applies the open-loop submultiple lift across all lower ranges
// using the Core lift, and evaluates encodable closed-loop pitch boundary
// codepoints. Its gain path keeps Annex A's 4x8 GA/GB preselection breadth
// while using a higher-precision preselect-center solve before the final
// integer codebook cost ranking.
// It is not an ITU byte-exact conformance mode.
// EncoderProfileCoreClipRepair is a listening-diagnostic Core variant that
// keeps that Core search policy but permits decoder-in-loop gain clip repair
// with a lower pre-clip threshold.
// EncoderProfileQualityAnnexALSP is available for listening diagnostics that
// keep the quality profile while using the sequential Annex A LSP VQ path.
// EncoderProfileQualityClean is available for listening diagnostics that keep
// the quality gain/LSP path while using a smoother closed-loop pitch policy and
// stricter, high-residual-aware gain MSE repair.
// EncoderProfileQualityCleanSNR, EncoderProfileQualityCleanSmooth,
// EncoderProfileQualityCleanVoiced, EncoderProfileQualityCleanDegrit,
// EncoderProfileQualityCleanHarmonic,
// EncoderProfileQualityCleanHarmonicStrong,
// EncoderProfileQualityCleanHarmonicDeep, and
// EncoderProfileQualityCleanFCBRerank are listening-diagnostic variants of
// that clean pitch policy for clarity-vs-smoothness A/B tests.
// EncoderProfileQuality remains available as the older broad quality-heuristic
// profile for diagnostics. EncoderProfileQualityPESQ is a numeric diagnostic
// profile that enables native reconstructed-gain residual search, gain clip
// repair, and fixed-codebook residual reranking. EncoderProfileQualityPESQDegrit
// is a PESQ-candidate variant that also enables bounded gain MSE/noise repair
// for blind tests targeting high-residual grit.
// EncoderProfileCoreFast is an explicit opt-in throughput profile. It keeps
// the normal 10-byte payload shape but reduces selected encoder search
// precision/budget, so it is a performance trade-off rather than the default
// quality/conformance evidence path.
//
// EncodeFrame and DecodeFrame are zero-allocation in steady state; see the
// v0.1.0-rc1 release verification log for hot-path benchmarks.
//
// # Concurrency
//
// Each Encoder and each Decoder instance is single-threaded. Concurrent
// calls on the same instance are a data race; callers needing parallel
// streams must own one Encoder and one Decoder per channel. There is
// no shared mutable state across instances; multiple instances can run
// concurrently on different goroutines.
//
// Reset returns codec state to the initial stream state. On streaming
// encoders, Reset also clears any buffered PCM tail while preserving the
// sink passed to NewStreamingEncoder.
//
// # SDP / RTP packetization
//
// The codec produces and consumes RTP-suitable 10-byte payloads. RTP
// header construction belongs to the caller. SDP must advertise
// "annexb=no" because Annex B (SID / CNG / DTX) is not implemented.
// 2-byte RTP Annex B SID/CNG frames are rejected with ErrUnsupportedAnnexB
// rather than decoded as speech.
// See README.md, examples/, and cmd/g729rtpcheck for ptime=10 and
// ptime=20 SDP fragments, payload bundling, and black-box RTP capture
// validation.
//
// # Conformance scope
//
// The strict decoder has a private oracle gate that decodes fixed ITU Annex A
// bitstreams and compares final PCM sample-for-sample against companion
// reference PCM; the current run is exact across 740800/740800 final PCM
// samples. The outbound encoder/RTP send path is separately black-box gated
// against FFmpeg executable decode for G729/8000 annexb=no speech frames.
// This module does not claim ITU certification, ITU endorsement, or encoder
// byte-exact conformance. It does not implement Annex B, G.729.1, G.729D, or
// G.729E. See README.md "Known scope and limitations" for the canonical
// disclosure.
//
// # Clean-room declaration
//
// This project maintains a clean-room constraint. No ITU reference C,
// bcg729, FFmpeg, Sipro, or other G.729 implementation source was used.
// Public specifications, black-box executable behavior, private numeric
// oracle outputs, and independently written tests were used. v0.1.0 claims
// strict decoder final-PCM sample equality against the private oracle gate; it
// does not claim ITU certification or encoder byte-exact conformance.
package g729
