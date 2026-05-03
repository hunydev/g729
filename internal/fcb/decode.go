package fcb

// Decode fills c[0..39] with the ACELP algebraic codebook vector for
// one subframe, per ITU-T G.729 §3.8 / §4.1.5:
//
//  1. Decode 4 pulse positions from idx.Positions (13 bits).
//  2. Apply 4 signs from idx.Signs (4 bits) and place pulses at
//     ±PulseAmplitude (Q13).
//  3. Apply the pitch pre-emphasis filter
//     c(n) ← c(n) + β·c(n − t), for n = t..39,
//     using the already-clamped betaQ14 (see
//     ClampPitchGainForEnhancement).
//
// t is the integer pitch lag of the current subframe (from
// internal/pitch.DecodeDelaySubframe*). For pitch delays with
// fractional offset ±1/3 the rounded lag equals the integer part,
// so callers pass t = tInt directly.
//
// Output c is Q13. Allocates nothing.
func Decode(idx Indices, t int, betaQ14 int16, c *[40]int16) {
	positions := decodePositions(idx.Positions)
	placePulses(positions, idx.Signs, c)
	ApplyPitchEnhancement(c, t, betaQ14)
}
