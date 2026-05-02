package decoder

import "testing"

// Shared helpers for the remaining stage-F diagnostic tests in this package
// (e.g. stagef_quart_diagnostic_test.go). Originally defined in
// stagef_bis_diagnostic_test.go; relocated here when that file was disposed
// of in Phase 1o D-3.bis (Option B). No assertions live here — callers own
// any pass/fail logic.

func dumpInt16(t *testing.T, v []int16) {
	t.Helper()
	for r := 0; r < 5; r++ {
		base := r * 8
		t.Logf("  [%2d..%2d] %5d %5d %5d %5d %5d %5d %5d %5d",
			base, base+7,
			v[base+0], v[base+1], v[base+2], v[base+3],
			v[base+4], v[base+5], v[base+6], v[base+7],
		)
	}
}

func matchCount(a, b []int16, tol int32) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	c := 0
	for i := 0; i < n; i++ {
		d := int32(a[i]) - int32(b[i])
		if d < 0 {
			d = -d
		}
		if d <= tol {
			c++
		}
	}
	return c
}
