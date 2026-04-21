// Package gain implements ITU-T G.729 + Annex A §3.9 / §4.1.6 gain
// VQ decoding: from 7 bits per subframe (GA 3 bits + GB 4 bits)
// and the Phase 1c fixed codebook vector, produce the pitch gain
// g_p (Q14) and fixed codebook gain g_c (Q1) used by the Phase 1e
// excitation sum u(n) = g_p·v(n) + g_c·c(n).
//
// The Decoder holds a 4-tap MA predictor state across subframes.
// Reset() returns to the zero value.
package gain
