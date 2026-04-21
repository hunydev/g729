// Package lsp implements the G.729 + Annex A decoder's line-spectral
// pair reconstruction. It consumes the 18 LSP bits of one frame
// (L0 / L1 / L2 / L3) and produces two 11-coefficient LP filter
// vectors — one per subframe — that the synthesis filter in
// internal/synth consumes directly.
//
// # Pipeline
//
// Per ITU-T G.729 §3.2.4, §3.2.5, §3.2.6 and §4.1.1–§4.1.2:
//
//  1. Split-VQ combine:   L1, L2, L3 indices → 10-element Q13 residual.
//  2. Pre-predictor pair-rearrangement (J = 0.0012 then 0.0006).
//  3. MA prediction:      r̂(n) + order-4 MA state → quantized Q13 LSF.
//  4. Stability:          sort + edge clamps + minimum spacing on ω̂.
//  5. LSF → LSP:          cosine lookup via tables.CosLSP, Q15 output.
//  6. Interpolation:      half-and-half with prev frame for subframe 1,
//                         current frame directly for subframe 2.
//  7. LSP → LP:           Chebyshev polynomial expansion per subframe,
//                         producing a[0..10] Q12 with a[0] = 4096.
//
// Steps 3 and 6 carry state across frames (past 4 residuals; one
// frame of LSP history). A Decoder's zero value is a valid Reset
// state; the first frame initializes the past-residual FIFO with
// l̂_i = i·π/11 per ITU-T G.729 §3.2.4.
//
// # Numerical contract
//
//	Indices: raw integers from the bitstream — L0 ∈ {0,1},
//	         L1 ∈ [0,127], L2, L3 ∈ [0,31].
//	LSF values: Q13 Word16, 0 ≤ ω < π (Q13 full-scale π = 25736).
//	LSP values: Q15 Word16, −1 < q ≤ 1.
//	LP output:  Q12 Word16, a[0] = 4096, a[i] bounded by LPC stability.
//
// # Scratch-from-spec
//
// Every codebook, predictor coefficient, and cosine table value comes
// from the ITU-T G.729 specification document. No reference C, no
// bcg729, no other implementation was consulted for algorithmic code.
// Every arithmetic step routes through internal/fixed so that ITU's
// saturation semantics are preserved.
//
// # Concurrency
//
// Decoder is not safe for concurrent use. One instance per decoder
// channel.
package lsp
