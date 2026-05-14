// Package fcb implements the G.729 + Annex A decoder's ACELP
// (algebraic) fixed codebook: reconstruction of the 40-sample
// codebook vector c[] per subframe from a 13-bit pulse-position
// code and a 4-bit sign code, followed by the pitch pre-emphasis
// filter.
//
// # Public API
//
//	Indices{Positions uint16, Signs uint8}
//	    Bit-field indices delivered by the bitstream unpacker
//	    (C1/S1 or C2/S2 from bitstream.Frame).
//
//	PulseAmplitude = 8191
//	    Positive unit pulse endpoint in Q13. Negative pulses use -8192.
//
//	ClampPitchGainForEnhancement(gpPrevQ14 int16) int16
//	    Per §4.1.5. Clamps the previous subframe's decoded pitch
//	    gain to [0.2, 0.8] in Q14 → β_Q14 ∈ [3277, 13107].
//
//	Decode(idx Indices, t int, betaQ14 int16, c *[40]int16)
//	    Per §3.8 / §4.1.5. Zeros c, places 4 signed pulses, then
//	    applies c(n) += β·c(n−t) for n = t..39.
//
// # Numerical contract
//
//	Positions: 13-bit uint16, packed per §3.8.2 eq. (62):
//	           C = i0 + 8*i1 + 64*i2 + 512*(2*i3+jx).
//	Signs:     4-bit uint8, packed per §3.8.2 eq. (61):
//	           S = s0 + 2*s1 + 4*s2 + 8*s3.
//	           Bit 1 = +1, bit 0 = -1.
//	c:         Q13 int16 on output. Before enhancement, positive pulses
//	           are +8191 and negative pulses are -8192; after enhancement
//	           |c[n]| can grow modestly (bounded by int16 saturation).
//	t:         integer pitch lag of the current subframe
//	           (from internal/pitch). t < 1 or t ≥ 40 is a no-op
//	           enhancement.
//	betaQ14:   pitch enhancement coefficient in Q14. Callers
//	           should pass the output of
//	           ClampPitchGainForEnhancement.
//
// # State ownership
//
// This package holds no state. The previous-subframe pitch gain
// that derives β is owned by the top-level decoder. The decoder's
// stream-start state starts with no previous decoded pitch gain, so
// the decoder supplies InitialPitchEnhancementQ14 for the first subframe
// and ClampPitchGainForEnhancement for subsequent subframes.
//
// # Scratch-from-spec
//
// Algorithm from ITU-T G.729 §3.8 (fixed codebook structure) and
// §4.1.5 (fixed codebook decoder + pitch pre-emphasis), plus
// Annex A §A.3.8 (decoder unchanged from full G.729). No ITU
// reference C source has been consulted. No numerical tables —
// the fixed codebook positions are formulaic and the filter has
// no look-up tables. Every arithmetic step routes through
// internal/fixed.
//
// # Concurrency
//
// All functions are pure and safe for concurrent use. The caller
// owns all state.
package fcb
