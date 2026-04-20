package bitstream

// Parity returns the XOR of the 6 most significant bits of p1. The
// result is 0 or 1. This is the value encoders store in Frame.P0 and
// decoders compare against the transmitted P0 to detect errors in the
// pitch-delay field.
func Parity(p1 uint16) uint16 {
	x := (p1 >> 2) & 0x3F
	var p uint16
	for i := 0; i < 6; i++ {
		p ^= (x >> uint(i)) & 1
	}
	return p
}
