package openloop

import "testing"

// BenchmarkEnergy exercises eq. A.5 denominator (G729E.txt §A.3.4
// lines 2102-2107) at a single representative lag in the [80,143]
// region.
func BenchmarkEnergy(b *testing.B) {
	wsp := makeBenchWsp()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = energy(&wsp, 80)
	}
}
