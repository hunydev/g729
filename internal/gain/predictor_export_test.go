package gain

import "testing"

// TestPredictedLogGain_ExportMatchesMethod is the GQ-1 refactor gate:
// the exported pure free function gain.PredictedLogGain must produce
// identical Q10 output to the existing Decoder.predictedLogGain method
// across representative MA-predictor states (cold start, all-zero,
// asymmetric, and saturated).
func TestPredictedLogGain_ExportMatchesMethod(t *testing.T) {
	cases := []struct {
		name string
		past [4]int16
	}{
		{"cold start (-14 dB Q10 ×4)", [4]int16{PastErrorsDefault, PastErrorsDefault, PastErrorsDefault, PastErrorsDefault}},
		{"all zero", [4]int16{0, 0, 0, 0}},
		{"only first tap", [4]int16{1024, 0, 0, 0}},
		{"uniform +1 dB Q10", [4]int16{1024, 1024, 1024, 1024}},
		{"asymmetric mixed signs", [4]int16{2048, -1024, 512, -2048}},
		{"large positive (saturating contrib)", [4]int16{16384, 16384, 16384, 16384}},
		{"large negative", [4]int16{-16384, -16384, -16384, -16384}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := Decoder{pastErrors: c.past}
			methodVal := d.predictedLogGain()
			past := c.past
			freeVal := PredictedLogGain(&past)
			if methodVal != freeVal {
				t.Fatalf("PredictedLogGain free=%d method=%d (must match)", freeVal, methodVal)
			}
		})
	}
}

// TestPredictedLogGain_FreeFunctionDoesNotMutateInput pins the contract
// that the exported predictor reads but never writes its tap-line
// argument; the FIFO update is the encoder/decoder's responsibility.
func TestPredictedLogGain_FreeFunctionDoesNotMutateInput(t *testing.T) {
	past := [4]int16{2048, -1024, 512, -2048}
	orig := past
	_ = PredictedLogGain(&past)
	if past != orig {
		t.Fatalf("PredictedLogGain mutated input: got %v, want %v", past, orig)
	}
}

// TestPredictedLogGain_ZeroAlloc pins the encoder hot-path budget: the
// exported predictor must not allocate.
func TestPredictedLogGain_ZeroAlloc(t *testing.T) {
	past := [4]int16{PastErrorsDefault, PastErrorsDefault, PastErrorsDefault, PastErrorsDefault}
	allocs := testing.AllocsPerRun(128, func() {
		_ = PredictedLogGain(&past)
	})
	if allocs != 0 {
		t.Fatalf("PredictedLogGain allocs/op = %.2f, want 0", allocs)
	}
}
