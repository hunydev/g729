package openloop

import "testing"

// BenchmarkLPResidual exercises eq. A.3 (G729E.txt §A.3.3): the
// 80-sample LP-residual filter driven by aHat (Q12).
func BenchmarkLPResidual(b *testing.B) {
	var s [80]int16
	for i := range s {
		s[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}
	aHat := [11]int16{4096, -2048, 1024, -512, 256, -128, 64, -32, 16, -8, 4}
	var mem [10]int16
	var r [80]int16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lpResidual(&s, &aHat, &mem, &r)
	}
}

// BenchmarkLowpassWeightedSpeech exercises eq. A.2 (G729E.txt §A.3.3):
// the A'(z)-driven low-pass weighting of the residual into sw.
func BenchmarkLowpassWeightedSpeech(b *testing.B) {
	var r [80]int16
	for i := range r {
		r[i] = int16(((i * 211) & 0x3FFF) - 0x2000)
	}
	aPrime := [11]int16{4096, -1800, 900, -450, 225, -112, 56, -28, 14, -7, 3}
	var mem [10]int16
	var sw [80]int16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lowpassWeightedSpeech(&r, &aPrime, &mem, &sw)
	}
}

// BenchmarkSlideOldWspeech exercises I-2b-2: the in-place 143-sample
// sw-history slide that prepends the previous 80-tail and appends the
// freshly computed 80-sample sw frame.
func BenchmarkSlideOldWspeech(b *testing.B) {
	var old [143]int16
	var fresh [80]int16
	for i := range old {
		old[i] = int16(i)
	}
	for i := range fresh {
		fresh[i] = int16(i + 200)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slideOldWspeech(&old, &fresh)
	}
}
