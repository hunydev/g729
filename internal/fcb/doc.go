// Package fcb implements ITU-T G.729 + Annex A §3.8 / §4.1.4 fixed
// (algebraic) codebook decoding: reconstruction of the 40-sample
// codebook vector c[] for one subframe from a 13-bit pulse-position
// code and a 4-bit sign code, followed by the pitch pre-emphasis
// filter c(n) += β·c(n−T) (applied only when T < 40, per eq. 48).
//
// All public functions are stateless. The decoder's previous-pitch-
// gain state is owned by the caller (Phase 1g's top-level decoder);
// each Decode call writes into a caller-owned output array.
package fcb
