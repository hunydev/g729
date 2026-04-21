package pitch

import "testing"

func BenchmarkAdaptiveCodebookIntegerDelay(b *testing.B) {
	var pastExc [250]int16
	for i := range pastExc {
		pastExc[i] = int16(i - 100)
	}
	var v [40]int16
	slice := pastExc[:]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AdaptiveCodebook(60, 0, slice, &v)
	}
}

func BenchmarkAdaptiveCodebookFractional(b *testing.B) {
	var pastExc [250]int16
	for i := range pastExc {
		pastExc[i] = int16(i - 100)
	}
	var v [40]int16
	slice := pastExc[:]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AdaptiveCodebook(60, 1, slice, &v)
	}
}

func BenchmarkAdaptiveCodebookShortPitch(b *testing.B) {
	var pastExc [250]int16
	for i := range pastExc {
		pastExc[i] = int16(i - 100)
	}
	var v [40]int16
	slice := pastExc[:]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AdaptiveCodebook(20, 0, slice, &v)
	}
}
