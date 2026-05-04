// Package decoder implements the top-level G.729 Annex A decoder,
// wiring the per-stage packages (bitstream → lsp/pitch/fcb/gain → synth →
// postfilter → HP filter → ×2 scaling) into a single streaming state
// machine that consumes one packed 80-bit frame and produces 80 samples
// of 16-bit PCM per call.
//
// # Block diagram
//
//	             ┌────────────┐
//	10-byte ───▶ │ bitstream  │──▶ Frame (15 index fields)
//	frame        │  Unpack    │
//	             └────────────┘
//	                   │
//	                   ▼
//	             ┌────────────┐
//	             │   lsp      │──▶ sf1A, sf2A (Q12 LP coefs)
//	             │ Decoder    │
//	             └────────────┘
//	                   │                          per-subframe loop
//	                   ▼                          ┌──────────────────┐
//	        ┌──────────────────────────┐          │ pitch.AdaptCode  │
//	        │   decodeSubframe (×2)    │ ◀────────│ fcb.Decode       │
//	        │  writes 40 HP samples    │          │ gain.Decode      │
//	        └──────────────────────────┘          │ synth.Filter     │
//	                   │                          │ postfilter.Filter│
//	                   ▼                          │ hpFilter         │
//	        ┌──────────────────────────┐          └──────────────────┘
//	        │     pcm.ScaleUpSat       │
//	        │     (×2 amplitude)       │
//	        └──────────────────────────┘
//	                   │
//	                   ▼
//	             80 int16 PCM
//
// # State layout
//
// Decoder owns five sub-state blocks (LSP, gain, synthesizer, postfilter,
// HP filter memory) plus the excitation FIFO that feeds the adaptive
// codebook. All state is per-stream; one Decoder per active call.
//
// # First-frame semantics
//
// The zero value is a valid starting state. The LSP, gain, synthesizer,
// and postfilter sub-packages each handle lazy initialization on their
// first Decode/Filter call per ITU-T G.729 §4.3; the Decoder itself holds
// no extra first-frame flag.
//
// # Usage
//
// One Decoder per stream. Call Decode in strict frame order with a
// 10-byte packed frame and an 80-sample int16 output buffer. The
// Decoder is not safe for concurrent use; for a new stream either
// allocate a fresh value or call Reset.
//
// # Spec-conformance caveat
//
// A small number of frames in the published ITU-T Annex A test vectors
// intentionally produce LSB-level differences from the PST reference
// output. These are spec-conformant per the published G.729 / Annex A
// PDF and are pinned by `internal/decoder/itu_vector_pstdomain_test.go`,
// which records the seven affected vectors (TAME, SPEECH, FIXED, LSP,
// PITCH, TEST, OVERFLOW frame 0) along with the production output that
// the decoder is contracted to emit.
//
// # Spec
//
// ITU-T G.729 §4.1.6 (decoder architecture), §4.2 (post-processing),
// §4.3 (initialization). Annex A §A.4 (reduced-complexity variant).
package decoder
