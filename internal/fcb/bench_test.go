package fcb

import "testing"

func BenchmarkDecode_NoEnhancement(b *testing.B) {
	idx := Indices{Positions: 0x1234, Signs: 0b1010}
	var c [40]int16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decode(idx, 40, 0, &c)
	}
}

func BenchmarkDecode_WithEnhancement(b *testing.B) {
	idx := Indices{Positions: 0x1234, Signs: 0b1010}
	var c [40]int16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decode(idx, 30, 10000, &c)
	}
}

func BenchmarkDecode_ShortLagEnhancement(b *testing.B) {
	idx := Indices{Positions: 0x1234, Signs: 0b1010}
	var c [40]int16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decode(idx, 20, 13107, &c)
	}
}
