package lsp

import "testing"

func TestLSFToLSPBoundaryCases(t *testing.T) {
	if got := lsfToLSP(0); got < 32700 {
		t.Errorf("lsfToLSP(0) = %d, want ≈ 32767", got)
	}
	if got := lsfToLSP(12868); got < -100 || got > 100 {
		t.Errorf("lsfToLSP(π/2) = %d, want ≈ 0", got)
	}
	if got := lsfToLSP(25700); got > -32000 {
		t.Errorf("lsfToLSP(near π) = %d, want ≈ -32767", got)
	}
}

func TestLSFToLSPMonotonic(t *testing.T) {
	lsfs := []int16{1000, 4000, 8000, 12000, 16000, 20000, 24000}
	var prev int16 = 32767
	for _, w := range lsfs {
		q := lsfToLSP(w)
		if q > prev {
			t.Errorf("lsfToLSP(%d) = %d > prev %d, not monotone decreasing", w, q, prev)
		}
		prev = q
	}
}
