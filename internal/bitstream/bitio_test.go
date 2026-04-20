package bitstream

import "testing"

func TestBitWriter_SingleBit(t *testing.T) {
	var buf [1]byte
	var w BitWriter
	w.Init(buf[:])
	w.Write(1, 1)
	if buf[0] != 0b10000000 {
		t.Errorf("buf[0] = %#08b, want 10000000", buf[0])
	}
	if w.BitPos() != 1 {
		t.Errorf("BitPos = %d, want 1", w.BitPos())
	}
}

func TestBitWriter_EightOnes(t *testing.T) {
	var buf [1]byte
	var w BitWriter
	w.Init(buf[:])
	for i := 0; i < 8; i++ {
		w.Write(1, 1)
	}
	if buf[0] != 0xFF {
		t.Errorf("buf[0] = %#08b, want 11111111", buf[0])
	}
}

func TestBitWriter_MultiBitField(t *testing.T) {
	var buf [2]byte
	var w BitWriter
	w.Init(buf[:])
	// Write a 7-bit value 0b1010101 at bit 0 of byte 0. That value's MSB
	// lands in buf[0] bit 7 (=1), next in bit 6 (=0), ... so the layout
	// is: 1 0 1 0 1 0 1 _ — buf[0] = 0b10101010 = 0xAA.
	w.Write(0b1010101, 7)
	if buf[0] != 0b10101010 {
		t.Errorf("buf[0] = %#08b, want 10101010", buf[0])
	}
	if w.BitPos() != 7 {
		t.Errorf("BitPos = %d, want 7", w.BitPos())
	}
}

func TestBitWriter_CrossesByteBoundary(t *testing.T) {
	var buf [2]byte
	var w BitWriter
	w.Init(buf[:])
	w.Write(0x3, 2)  // 11 at bit pos 0..1 of buf[0]
	w.Write(0x3F, 8) // 00111111 at bit pos 2..9
	// buf[0] bits: 1 1 0 0 1 1 1 1 = 0xCF
	// buf[1] bits: 1 1 _ _ _ _ _ _ = 0xC0
	if buf[0] != 0xCF || buf[1] != 0xC0 {
		t.Errorf("buf = [%#02x, %#02x], want [0xCF, 0xC0]", buf[0], buf[1])
	}
	if w.BitPos() != 10 {
		t.Errorf("BitPos = %d, want 10", w.BitPos())
	}
}
