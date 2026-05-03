package openloop

import "testing"

// BenchmarkPickBestInRange_HighDelay exercises §A.3.4 lines 2094-2097
// + 2113-2114 over the widest high-delay region [80,143] with the
// even-stride scan + ±1 refinement code path.
func BenchmarkPickBestInRange_HighDelay(b *testing.B) {
	wsp := makeBenchWsp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = pickBestInRange(&wsp, 80, 143)
	}
}

// BenchmarkPickBestInRange_MidDelay exercises §A.3.4 over the
// mid-delay range [40,79] (full-stride scan path).
func BenchmarkPickBestInRange_MidDelay(b *testing.B) {
	wsp := makeBenchWsp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = pickBestInRange(&wsp, 40, 79)
	}
}
