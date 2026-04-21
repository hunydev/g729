package pitch

// Indices are the pitch-related bit-field values delivered per frame
// by the bitstream unpacker. Values are raw integer indices, not
// bit-slices.
type Indices struct {
	P1 uint8 // 8 bits — subframe-1 pitch delay index (0..255)
	P0 uint8 // 1 bit  — parity check bit for P1
	P2 uint8 // 5 bits — subframe-2 pitch delay index (0..31)
}
