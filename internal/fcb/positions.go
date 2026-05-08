package fcb

// decodePositions extracts the 4 pulse positions from a 13-bit
// pulse-position code per ITU-T G.729 §3.8.2 eq. (62):
//
//	C = i0 + 8*i1 + 64*i2 + 512*(2*i3 + jx)
//
// where i0=m0/5, i1=m1/5, i2=m2/5, i3=m3/5 and jx selects the
// fourth-track half:
//
//	bits  2..0  : i0 ∈ [0, 7]   → pos[0] = 5*i0
//	bits  5..3  : i1 ∈ [0, 7]   → pos[1] = 5*i1 + 1
//	bits  8..6  : i2 ∈ [0, 7]   → pos[2] = 5*i2 + 2
//	bit      9  : jx ∈ {0, 1}   → track 3 half select
//	bits 12..10 : i3 ∈ [0, 7]   → pos[3] = 5*i3 + 3 + jx
//
// The four returned positions are guaranteed distinct (residues
// 0, 1, 2, and 3-or-4 modulo 5 on the 40-sample grid).
func decodePositions(code uint16) [4]int {
	i0 := int(code & 0x07)
	i1 := int((code >> 3) & 0x07)
	i2 := int((code >> 6) & 0x07)
	jx := int((code >> 9) & 0x01)
	i3 := int((code >> 10) & 0x07)
	return [4]int{
		5 * i0,
		5*i1 + 1,
		5*i2 + 2,
		5*i3 + 3 + jx,
	}
}
