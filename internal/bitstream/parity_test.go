package bitstream

import "testing"

func TestParity(t *testing.T) {
	// Reference values computed manually from the definition
	// P0 = XOR of bits 7..2 of P1.
	tests := []struct {
		p1   uint16
		want uint16
	}{
		{0, 0},    // upper 6 bits = 0 -> XOR = 0
		{0x03, 0}, // lower 2 bits don't count
		{0x04, 1}, // bit 2 only -> XOR = 1
		{0x08, 1}, // bit 3 only
		{0x0C, 0}, // bits 2+3 -> XOR = 0
		{0xFF, 0}, // upper 6 = 0b111111 -> 6 ones -> XOR = 0
		{0xFC, 0}, // upper 6 = 0b111111 -> XOR = 0
		{0x7C, 1}, // upper 6 = 0b011111 -> 5 ones -> XOR = 1
		{0xA0, 0}, // upper 6 = 0b101000 -> 2 ones -> XOR = 0
		{0xA4, 1}, // upper 6 = 0b101001 -> 3 ones -> XOR = 1
	}
	for _, tc := range tests {
		if got := Parity(tc.p1); got != tc.want {
			t.Errorf("Parity(%#x) = %d, want %d", tc.p1, got, tc.want)
		}
	}
}
