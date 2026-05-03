package lpc

import "testing"

// BenchmarkLevinsonDurbin pins zero-allocation on the §3.2.2
// Levinson-Durbin recursion (Phase 2a INT-2-b). Uses a synthetic
// positive-definite Toeplitz r' that exercises the full 10-stage
// recursion (no early-abort silence branch).
func BenchmarkLevinsonDurbin(b *testing.B) {
	r := [11]int32{
		1 << 28,
		1 << 26, 1 << 24, 1 << 22, 1 << 20, 1 << 18,
		1 << 16, 1 << 14, 1 << 12, 1 << 10, 1 << 8,
	}
	var a [11]int16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		levinsonDurbin(&r, &a)
	}
}
