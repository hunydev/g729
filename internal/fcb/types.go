package fcb

// Indices are the fixed codebook bit-field values delivered per
// subframe by the bitstream unpacker. Positions carries the 13-bit
// pulse-position code (C1 or C2 from bitstream.Frame). Signs carries
// the 4-bit sign code (S1 or S2 from bitstream.Frame).
type Indices struct {
	Positions uint16 // 13 bits — packed as i0|i1|i2|jx|i3 MSB-first
	Signs     uint8  //  4 bits — packed as s0|s1|s2|s3 MSB-first
}

// PulseAmplitude is the unit-pulse magnitude used for the ACELP
// algebraic codebook vector c[], expressed in Q13 (+1.0 exactly).
const PulseAmplitude = 8192
