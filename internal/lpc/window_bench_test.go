package lpc

import "testing"

// BenchmarkApplyWindow pins zero-allocation on the §3.2.1 eq. 4
// windowed-input multiply (Phase 2a INT-2-b).
func BenchmarkApplyWindow(b *testing.B) {
	var (
		speech   [240]int16
		windowed [240]int16
	)
	for i := range speech {
		speech[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		windowSpeech(&speech, &windowed)
	}
}
