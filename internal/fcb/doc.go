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
//	PulseAmplitude = 8192
//	    Unit pulse magnitude in Q13 (= +1.0).
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
//	Positions: 13-bit uint16, packed MSB-first as i0|i1|i2|jx|i3
//	           (3+3+3+1+3 bits).
//	Signs:     4-bit uint8, packed MSB-first as s0|s1|s2|s3.
//	           Bit 1 = +1, bit 0 = −1.
//	c:         Q13 int16 on output. |c[n]| ≤ PulseAmplitude before
//	           enhancement; after enhancement |c[n]| can grow
//	           modestly (bounded by int16 saturation).
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
// that derives β is owned by the top-level decoder (Phase 1g),
// which also initializes it to 0.8 Q14 for the very first subframe
// (where no previous gain exists).
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
