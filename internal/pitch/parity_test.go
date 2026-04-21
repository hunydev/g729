package pitch

import "testing"

// expectedParity computes the parity P0 expected for a given P1
// under the odd-parity convention over the upper 6 bits of P1.
// ITU-T G.729 §3.7.2 specifies "an XOR operation on the six most
// significant bits"; the trailing ^ 1 implements the odd-parity
// reading widely used in G.729 literature. Phase 1g's ITU-vector
// pass is the final arbiter between odd and even.
func expectedParity(p1 uint8) uint8 {
	bits := (p1 >> 2) & 0x3F
	x := bits ^ (bits >> 4)
	x ^= x >> 2
	x ^= x >> 1
	return (x & 1) ^ 1
}

func TestCheckParityExhaustive(t *testing.T) {
	matchCount := 0
	for p1 := 0; p1 < 256; p1++ {
		expected := expectedParity(uint8(p1))
		for p0 := uint8(0); p0 <= 1; p0++ {
			got := CheckParity(uint8(p1), p0)
			want := p0 == expected
			if got != want {
				t.Errorf("CheckParity(p1=%d, p0=%d) = %v, want %v (expected parity %d)",
					p1, p0, got, want, expected)
			}
			if got {
				matchCount++
			}
		}
	}
	if matchCount != 256 {
		t.Errorf("parity matches = %d, want 256 (half of 512 combinations)", matchCount)
	}
}
