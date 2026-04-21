// Package synth implements the G.729 + Annex A decoder's excitation
// assembly and LP synthesis filter. It consumes per-subframe outputs from
// internal/pitch (adaptive codebook v[40] in Q0), internal/fcb (fixed
// codebook c[40] in Q13), internal/gain (g_p in Q14, g_c in Q12), and
// internal/lsp (LP filter coefficients a[11] in Q12) and produces 40
// synthesized speech samples s[40] in Q0.
//
// # Pipeline
//
// Per ITU-T G.729 §4.1.2, §4.1.6, §3.10:
//
//  1. BuildExcitation:  u(n) = g_p · v(n) + g_c · c(n), saturated to Q0.
//  2. Synthesize:       s(n) = u(n) − Σ_{i=1..10} a[i] · s(n−i),
//                       with s(n−i) for n−i < 0 drawn from pastSynth.
//
// Step 2 carries state across subframes (the 10 most recent synthesized
// samples). A Synthesizer's zero value is a valid Reset state per §4.3.
//
// # Numerical contract
//
//gpQ14:  Q14 Word16; range [0, ~19661] (≈ 1.2)
//gcQ12:  Q12 Word16; range (0, ~32767)
//v[n]:   Q0  Word16 (adaptive codebook)
//c[n]:   Q13 Word16 (fixed codebook, pulses ±8192)
//a[i]:   Q12 Word16; a[0] = 4096 (present for layout only)
//u[n]:   Q0  Word16 (excitation, saturated)
//s[n]:   Q0  Word16 (synthesis, saturated)
//
// # Q-format alignment in BuildExcitation
//
// The two contributions have different natural Q-formats after multiply:
// the pitch half lands at Q15 in Word32 via LMult(gpQ14, v), while the
// code half lands at Q26 via LMult(gcQ12, c). The code half is down-shifted
// by 11 bits before summation to align at Q15, at the cost of 11 bits of
// gcQ12 fractional precision. Perceptually this is negligible because the
// code half's MSBs dominate the sum for audible signals; bit-exact ITU
// conformance will be verified in Phase 1g.
//
// # Scratch-from-spec
//
// All arithmetic derives from ITU-T G.729 §3.10 / §4.1.2 / §4.1.6 directly.
// No ITU reference C source, bcg729, Sipro Lab, or any other existing
// G.729 implementation was consulted for algorithmic code. The synthesis
// filter uses direct-form saturation arithmetic (no two-pass overflow
// guard); the spec does not require two-pass, and perceptual tests in
// Phase 1g will verify acceptability.
//
// # Concurrency
//
// Synthesizer is not safe for concurrent use. One instance per decoder channel.
package synth
