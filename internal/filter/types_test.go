package filter

import "testing"

func TestWeighting_Reset_ZeroValueIsSafe(t *testing.T) {
	var w Weighting
	w.Reset()
}

func TestWeighting_Apply_StubReturnsNotImplemented(t *testing.T) {
	var w Weighting
	var (
		in  [40]int16
		out [40]int16
		a   [11]int16
	)
	if err := w.Apply(a[:], in[:], out[:]); err == nil {
		t.Fatal("Apply returned nil; expected stub error")
	}
}
