package fixed

import "testing"

func BenchmarkAdd(b *testing.B) {
	var s16 Word16 = 12345
	for i := 0; i < b.N; i++ {
		_ = Add(s16, s16)
	}
}

func BenchmarkLMult(b *testing.B) {
	var s16 Word16 = 12345
	for i := 0; i < b.N; i++ {
		_ = LMult(s16, s16)
	}
}

func BenchmarkLMac(b *testing.B) {
	var s16 Word16 = 12345
	var s32 Word32 = 0
	for i := 0; i < b.N; i++ {
		s32 = LMac(s32, s16, s16)
	}
	_ = s32
}

func BenchmarkDivS(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DivS(12345, 23456)
	}
}

func BenchmarkNormL(b *testing.B) {
	var s32 Word32 = 0x0000FF00
	for i := 0; i < b.N; i++ {
		_ = NormL(s32)
	}
}
