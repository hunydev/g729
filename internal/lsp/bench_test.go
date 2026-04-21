package lsp

import "testing"

func BenchmarkDecode(b *testing.B) {
	var d Decoder
	idx := Indices{L0: 0, L1: 42, L2: 11, L3: 3}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Decode(idx)
	}
}
