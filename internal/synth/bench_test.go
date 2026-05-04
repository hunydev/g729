package synth

import "testing"

var (
	benchA = [11]int16{4096, 1500, -800, 300, -100, 50, 20, -10, 5, -2, 1}
	benchV [40]int16
	benchC [40]int16
	benchS [40]int16
	benchU [40]int16
)

func init() {
	for i := range benchV {
		benchV[i] = int16(i * 17)
		benchC[i] = int16((i - 20) * 200)
	}
}

func BenchmarkBuildExcitation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		BuildExcitation(12000, 6000, 0, &benchV, &benchC, &benchU)
	}
}

func BenchmarkSynthesize(b *testing.B) {
	var synth Synthesizer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		synth.Synthesize(&benchA, &benchV, &benchC, 12000, 6000, 0, &benchS)
	}
}

func BenchmarkFilterSubframe(b *testing.B) {
	var synth Synthesizer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		synth.filterSubframe(&benchA, &benchU, &benchS)
	}
}
