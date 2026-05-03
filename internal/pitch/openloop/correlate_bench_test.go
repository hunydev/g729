package openloop

import "testing"

func makeBenchWsp() [223]int16 {
	var wsp [223]int16
	for i := range wsp {
		wsp[i] = int16(((i * 53) & 0x1FFF) - 0x1000)
	}
	return wsp
}

// BenchmarkCorrelate exercises eq. A.4 over the [80,143] high-delay
// region (the widest of the three open-loop ranges).
func BenchmarkCorrelate(b *testing.B) {
	wsp := makeBenchWsp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = correlate(&wsp, 80, 143)
	}
}
