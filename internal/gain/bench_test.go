package gain

import "testing"

func BenchmarkDecode(b *testing.B) {
	var d Decoder
	var c [40]int16
	c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192
	idx := Indices{GA: 3, GB: 7}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Decode(idx, &c)
	}
}
