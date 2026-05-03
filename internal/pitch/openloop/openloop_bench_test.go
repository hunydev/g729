package openloop

import "testing"

// BenchmarkSearch exercises the full §A.3.4 three-range open-loop
// search end-to-end.
func BenchmarkSearch(b *testing.B) {
	wsp := makeBenchWsp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Search(&wsp)
	}
}

// BenchmarkStep exercises the §A.3.3 weighted-speech pipeline + §A.3.4
// open-loop pitch search end-to-end (the openloop.Step entry that the
// root encoder.openloopStep wraps).
func BenchmarkStep(b *testing.B) {
	aHat := [11]int16{4096, -2048, 1024, -512, 256, -128, 64, -32, 16, -8, 4}
	var s [80]int16
	for i := range s {
		s[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}
	var residualMem, swMem [10]int16
	var oldWspeech [143]int16
	for i := range oldWspeech {
		oldWspeech[i] = int16(((i * 211) & 0x1FFF) - 0x1000)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Step(&aHat, &s, &residualMem, &swMem, &oldWspeech)
	}
}
