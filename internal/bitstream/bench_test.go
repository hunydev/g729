package bitstream

import (
	"bytes"
	"testing"
)

func BenchmarkPack(b *testing.B) {
	f := Frame{
		L0: 1, L1: 64, L2: 15, L3: 9,
		P1: 120, P0: 1,
		C1: 0x1ABC, S1: 5, GA1: 3, GB1: 11,
		P2: 15, C2: 0x0DEF, S2: 9, GA2: 2, GB2: 6,
	}
	var out [FrameBytes]byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Pack(&f, out[:])
	}
}

func BenchmarkUnpack(b *testing.B) {
	bits := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}
	var f Frame
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Unpack(bits, &f)
	}
}

func BenchmarkParity(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Parity(uint16(i))
	}
}

func BenchmarkWriteG192Frame(b *testing.B) {
	frame := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}
	var buf bytes.Buffer
	buf.Grow(G192FrameBytes)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = WriteG192Frame(&buf, frame, false)
	}
}

func BenchmarkReadG192Frame(b *testing.B) {
	frame := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}
	var encoded bytes.Buffer
	encoded.Grow(G192FrameBytes)
	if err := WriteG192Frame(&encoded, frame, false); err != nil {
		b.Fatalf("WriteG192Frame setup: %v", err)
	}
	encodedBytes := encoded.Bytes()

	var r bytes.Reader
	var out [FrameBytes]byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Reset(encodedBytes)
		_, _ = ReadG192Frame(&r, out[:])
	}
}
