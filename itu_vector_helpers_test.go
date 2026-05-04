package g729

import "encoding/binary"

// extractLSPFieldsFromG192 reads (L0, L1, L2, L3) from one G.192
// frame per §A.4 Table A.4 transmission order. Used by ITU-vector
// integration tests across both the default and conformance suites.
func extractLSPFieldsFromG192(g192Frame []byte) (l0, l1, l2, l3 uint8) {
	const g192Bit1 uint16 = 0x0081
	bit := func(i int) uint8 {
		off := 4 + 2*i
		if binary.LittleEndian.Uint16(g192Frame[off:off+2]) == g192Bit1 {
			return 1
		}
		return 0
	}
	pack := func(start, n int) uint8 {
		var v uint8
		for i := 0; i < n; i++ {
			v = (v << 1) | bit(start+i)
		}
		return v
	}
	l0 = pack(0, 1)
	l1 = pack(1, 7)
	l2 = pack(8, 5)
	l3 = pack(13, 5)
	return
}
