package pitch

// CheckParity recomputes the parity bit over the upper 6 bits of P1
// (ITU-T G.729 §3.7.2) and returns true iff it matches the
// transmitted P0. The parity convention follows the standard G.729
// odd-parity reading: P0 = NOT(b7 ⊕ b6 ⊕ b5 ⊕ b4 ⊕ b3 ⊕ b2).
func CheckParity(p1, p0 uint8) bool {
	return (p0 & 1) == Parity(p1)
}

// Parity returns the encoder-side P0 parity bit for the supplied P1
// codeword per ITU-T G.729 §3.7.2: an XOR over the six MSBs of P1
// folded into the standard odd-parity reading P0 = NOT(b7 ⊕ … ⊕ b2).
// Round-trip with CheckParity: CheckParity(p1, Parity(p1)) == true
// for every p1 ∈ [0, 255].
func Parity(p1 uint8) uint8 {
	bits := (p1 >> 2) & 0x3F
	x := bits ^ (bits >> 4)
	x ^= x >> 2
	x ^= x >> 1
	return (x & 1) ^ 1
}
