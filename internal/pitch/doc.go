// Package pitch implements ITU-T G.729 + Annex A §3.7 / §4.1.3 /
// §4.1.4 adaptive-codebook decoding: pitch delay reconstruction
// from the transmitted P1, P0, P2 bit fields and construction of
// the 40-sample adaptive codebook vector for one subframe via
// 1/3-sample fractional interpolation of past excitation.
//
// All public functions are stateless. The past-excitation signal is
// owned by the caller (Phase 1g's top-level decoder); each
// AdaptiveCodebook call writes into a caller-owned output array.
package pitch
