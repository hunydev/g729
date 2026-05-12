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
// # Conformance caveat
//
// The decoder is not currently claimed to be ITU-vector byte-exact. The
// opt-in TestDecoderITUVectorValidation gate compares local output against the
// Annex A .BIT/.PST vectors at sample level and records the current matrix.
// Bad-frame concealment and parity handling are separate robustness surfaces
// and are not yet release-gated.
//
// # Spec
//
// ITU-T G.729 §4.1.6 (decoder architecture), §4.2 (post-processing),
// §4.3 (initialization). Annex A §A.4 (reduced-complexity variant).
package decoder
