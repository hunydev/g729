package pitch

// CheckParity recomputes the parity bit over the upper 6 bits of P1
// (ITU-T G.729 §3.7.2) and returns true iff it matches the
// transmitted P0. The parity convention follows the standard G.729
// odd-parity reading: P0 = NOT(b7 ⊕ b6 ⊕ b5 ⊕ b4 ⊕ b3 ⊕ b2).
func CheckParity(p1, p0 uint8) bool {
	bits := (p1 >> 2) & 0x3F
	x := bits ^ (bits >> 4)
	x ^= x >> 2
	x ^= x >> 1
	expected := (x & 1) ^ 1
	return (p0 & 1) == expected
}
