package fcb

// Indices are the fixed codebook bit-field values delivered per
// subframe by the bitstream unpacker. Positions carries the 13-bit
// pulse-position code (C1 or C2 from bitstream.Frame). Signs carries
// the 4-bit sign code (S1 or S2 from bitstream.Frame).
type Indices struct {
	Positions uint16 // 13 bits — eq. (62): C=i0+8*i1+64*i2+512*(2*i3+jx)
	Signs     uint8  //  4 bits — eq. (61): S=s0+2*s1+4*s2+8*s3
}

// PulseAmplitude is the unit-pulse magnitude used for the ACELP
// algebraic codebook vector c[], expressed in Q13 (+1.0 exactly).
const PulseAmplitude = 8192
