package openloop

import "testing"

// BenchmarkMergeThreeRanges exercises §A.3.4 lines 2109-2111: the
// three-range (R²/E) merge with the OQ-1 sub-multiple lift. Inputs
// chosen so the high-delay range wins on raw R²/E but the mid-delay
// range is a near-sub-multiple, exercising the lifted-strict-greater
// override path.
func BenchmarkMergeThreeRanges(b *testing.B) {
	wsp := makeBenchWsp()
	lag1, rsq1, e1 := pickBestInRange(&wsp, 20, 39)
	lag2, rsq2, e2 := pickBestInRange(&wsp, 40, 79)
	lag3, rsq3, e3 := pickBestInRange(&wsp, 80, 143)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mergeThreeRanges(rsq1, e1, lag1, rsq2, e2, lag2, rsq3, e3, lag3)
	}
}
