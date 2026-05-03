package openloop

import "testing"

// BenchmarkGammaWeightLP exercises §A.3.3 line 2063 (a_w[i] = a[i] · γ^i)
// in isolation. Reports ns/op with B/op = allocs/op = 0 per I4.
func BenchmarkGammaWeightLP(b *testing.B) {
	a := [11]int16{4096, -2048, 1024, -512, 256, -128, 64, -32, 16, -8, 4}
	var out [11]int16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gammaWeightLP(&a, &out)
	}
}

// BenchmarkCombineWith07 exercises §A.3.3 line 2071 (A'(z) coefficients
// = a_w[i] − 0.7·a_w[i−1]) in isolation.
func BenchmarkCombineWith07(b *testing.B) {
	aw := [11]int16{4096, -2048, 1024, -512, 256, -128, 64, -32, 16, -8, 4}
	var out [11]int16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		combineWith07(&aw, &out)
	}
}
