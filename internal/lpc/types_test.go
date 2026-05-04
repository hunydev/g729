package lpc

import "testing"

func TestAnalyzer_Reset_ZeroValueIsSafe(t *testing.T) {
	var a Analyzer
	a.Reset() // must not panic on zero value
}

// TestAnalyzer_Analyze_AllZeroSpeechProducesTrivialFilter pins the
// silence-input behaviour: r'(0) = 0 trips the levinsonDurbin
// stability guard at every stage and yields a[0]=4096 (Q12 1.0)
// with a[1..10]=0, the all-pole filter A(z) = 1 documented in the
// levinson.go preamble.
func TestAnalyzer_Analyze_AllZeroSpeechProducesTrivialFilter(t *testing.T) {
	var (
		a      Analyzer
		speech [LPCWindowSamples]int16
		out    [LPCOrder + 1]int16
	)
	if err := a.Analyze(&speech, &out); err != nil {
		t.Fatalf("Analyze on zero speech: %v", err)
	}
	if out[0] != 4096 {
		t.Errorf("a[0] = %d, want 4096 (Q12 1.0)", out[0])
	}
	for i := 1; i <= LPCOrder; i++ {
		if out[i] != 0 {
			t.Errorf("a[%d] = %d, want 0 for silence input", i, out[i])
		}
	}
}
