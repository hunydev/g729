// Package synth implements the G.729 + Annex A decoder's excitation
// assembly and LP synthesis filter. It consumes per-subframe outputs from
// internal/pitch (adaptive codebook v[40] in Q0), internal/fcb (fixed
// codebook c[40] in Q13), internal/gain (g_p in Q14, g_c as
// (mantissa Q14, exponent int8) per Phase 3a REF-1 §2), and internal/lsp
// (LP filter coefficients a[11] in Q12) and produces 40 synthesized
// speech samples s[40] in Q0.
//
// # Pipeline
//
// Per ITU-T G.729 §4.1.2, §4.1.6, §3.10:
//
//  1. BuildExcitation:  u(n) = g_p · v(n) + g_c · c(n), saturated to Q0.
//  2. Synthesize:       s(n) = u(n) − Σ_{i=1..10} a[i] · s(n−i),
//     with s(n−i) for n−i < 0 drawn from pastSynth.
//
// Step 2 carries state across subframes (the 10 most recent synthesized
// samples). A Synthesizer's zero value is a valid Reset state per §4.3.
//
// # Numerical contract
//
// gpQ14:      Q14 Word16; range [0, ~19661] (≈ 1.2)
// gcMantQ14:  Q14 Word16 mantissa of g_c, gcMantQ14 ∈ [16384, 32767] (or 0)
// gcExp:      int8 binary exponent of g_c; g_c = gcMantQ14 · 2^(gcExp-14)
// v[n]:       Q0  Word16 (adaptive codebook)
// c[n]:       Q13 Word16 (fixed codebook, pulses ±8192)
// a[i]:       Q12 Word16; a[0] = 4096 (present for layout only)
// u[n]:       Q0  Word16 (excitation, saturated)
// s[n]:       Q0  Word16 (synthesis, saturated)
//
// # Q-format alignment in BuildExcitation
//
// The pitch half lands at Q15 in Word32 via LMult(gpQ14 [Q14], v[n] [Q0]).
// The code half multiplies the Q14 mantissa by c[n] (Q13) yielding prod32
// at Q28 (LMult doubles, so Q14·Q13 → Q28). The full g_c contribution
// equals prod32 · 2^(gcExp - 13), so we right-shift prod32 by
// shift_r = 13 - gcExp to land at Q15. When gcExp > 13 we left-shift
// (saturating) instead. See excitation.go for the full derivation.
// Phase 3a REF-1 amended the legacy single-Q12 g_c (which truncated the
// dynamic range to ≤ 7.999) to this mantissa+exponent representation so
// the spec's full ~159 peak g_c survives into the excitation.
//
// # Scratch-from-spec
//
// All arithmetic derives from ITU-T G.729 §3.10 / §4.1.2 / §4.1.6 directly.
// No ITU reference C source, bcg729, Sipro Lab, or any other existing
// G.729 implementation was consulted for algorithmic code. The synthesis
// filter uses direct-form fixed-point saturation arithmetic with the §3.10
// two-pass overflow recovery path implemented in filter.go.
//
// # Concurrency
//
// Synthesizer is not safe for concurrent use. One instance per decoder channel.
package synth
