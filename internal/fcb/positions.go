package fcb

// decodePositions extracts the 4 pulse positions from a 13-bit
// pulse-position code per ITU-T G.729 §3.8 Table 7. Bit layout
// (MSB first):
//
//	bits 12..10: i0 ∈ [0, 7]   → pos[0] = 5·i0         (track 0)
//	bits  9.. 7: i1 ∈ [0, 7]   → pos[1] = 5·i1 + 1     (track 1)
//	bits  6.. 4: i2 ∈ [0, 7]   → pos[2] = 5·i2 + 2     (track 2)
//	bit       3: jx ∈ {0, 1}   → track 3 half select
//	bits  2.. 0: i3 ∈ [0, 7]   → pos[3] = 5·i3 + 3 + jx
//
// The four returned positions are guaranteed distinct (residues
// 0, 1, 2, and 3-or-4 modulo 5 on the 40-sample grid).
func decodePositions(code uint16) [4]int {
	i0 := int((code >> 10) & 0x07)
	i1 := int((code >> 7) & 0x07)
	i2 := int((code >> 4) & 0x07)
	jx := int((code >> 3) & 0x01)
	i3 := int(code & 0x07)
	return [4]int{
		5 * i0,
		5*i1 + 1,
		5*i2 + 2,
		5*i3 + 3 + jx,
	}
}
