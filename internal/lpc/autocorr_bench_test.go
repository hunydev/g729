package lpc

import "testing"

// BenchmarkAutocorr pins zero-allocation on the §3.2.1 eq. 5
// autocorrelation (Phase 2a INT-2-b). Exercises the scale>0 path by
// using a high-amplitude windowed buffer.
func BenchmarkAutocorr(b *testing.B) {
	var (
		windowed [240]int16
		r        [11]int32
	)
	for i := range windowed {
		windowed[i] = int16(((i * 211) & 0x7FFF) - 0x4000)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = autocorrelate(&windowed, &r)
	}
}
