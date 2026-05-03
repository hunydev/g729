package lpc

import "testing"

func TestAnalyzer_Reset_ZeroValueIsSafe(t *testing.T) {
	var a Analyzer
	a.Reset() // must not panic on zero value
}

func TestAnalyzer_Analyze_StubReturnsNotImplemented(t *testing.T) {
	var a Analyzer
	var (
		speech [LPCWindowSamples]int16
		out    [LPCOrder]int16
	)
	if err := a.Analyze(speech[:], out[:]); err == nil {
		t.Fatal("Analyze returned nil; expected stub error")
	}
}
