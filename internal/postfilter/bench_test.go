package postfilter

import "testing"

var (
	benchA   = [11]int16{4096, 1500, -800, 300, -100, 50, 20, -10, 5, -2, 1}
	benchS   [subframeLen]int16
	benchSPf [subframeLen]int16
)

func init() {
	for i := range benchS {
		benchS[i] = int16(i * 17)
	}
}

func BenchmarkFilter(b *testing.B) {
	var pf Postfilter
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pf.Filter(&benchA, 40, &benchS, &benchSPf)
	}
}
