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

func TestBitReader_SingleBit(t *testing.T) {
buf := []byte{0b10000000}
var r BitReader
r.Init(buf)
if got := r.Read(1); got != 1 {
t.Errorf("Read(1) = %d, want 1", got)
}
if r.BitPos() != 1 {
t.Errorf("BitPos = %d, want 1", r.BitPos())
}
}

func TestBitReader_Byte(t *testing.T) {
buf := []byte{0xAB}
var r BitReader
r.Init(buf)
if got := r.Read(8); got != 0xAB {
t.Errorf("Read(8) = %#x, want 0xAB", got)
}
}

func TestBitReader_MultiBitField(t *testing.T) {
buf := []byte{0b10101010} // first 7 bits as a field = 0b1010101 = 85
var r BitReader
r.Init(buf)
if got := r.Read(7); got != 0b1010101 {
t.Errorf("Read(7) = %#b, want 1010101", got)
}
if r.BitPos() != 7 {
t.Errorf("BitPos = %d, want 7", r.BitPos())
}
}

func TestBitReader_CrossesByteBoundary(t *testing.T) {
// Inverse of the TestBitWriter_CrossesByteBoundary layout.
buf := []byte{0xCF, 0xC0}
var r BitReader
r.Init(buf)
if got := r.Read(2); got != 0x3 {
t.Errorf("Read(2) #1 = %#x, want 3", got)
}
if got := r.Read(8); got != 0x3F {
t.Errorf("Read(8) = %#x, want 0x3F", got)
}
}

func TestBitWriter_ReadRoundTrip(t *testing.T) {
// Sanity: BitWriter and BitReader agree bit-for-bit.
var buf [2]byte
var w BitWriter
w.Init(buf[:])
fields := []struct {
value uint16
bits  int
}{
{0x1, 1},
{0x55, 7},
{0xA, 4},
{0x3, 2},
{0xF, 2},
}
for _, f := range fields {
w.Write(f.value, f.bits)
}

var r BitReader
r.Init(buf[:])
for i, f := range fields {
got := r.Read(f.bits)
want := f.value & ((1 << uint(f.bits)) - 1)
if got != want {
t.Errorf("field %d: got %#x, want %#x", i, got, want)
}
}
}
