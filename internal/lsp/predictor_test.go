package lsp

import "testing"

func TestPredictorImpulseResponseFirstSlot(t *testing.T) {
	var d Decoder
	var residual, out [10]int16
	residual[0] = 1 << 13

	d.applyPredictor(0, &residual, &out)

	if out[0] <= 0 {
		t.Fatalf("predictor output for impulse at slot 0, selector 0: got %d, want >0", out[0])
	}
	if out[0] >= residual[0] {
		t.Fatalf("predictor output %d >= input %d — complementary coefficient implausibly >=1",
			out[0], residual[0])
	}
}

func TestPredictorStateAdvances(t *testing.T) {
	var d Decoder
	var r1, r2, out [10]int16
	for i := range r1 {
		r1[i] = 100
		r2[i] = 100
	}
	d.applyPredictor(0, &r1, &out)
	var secondOut [10]int16
	d.applyPredictor(0, &r2, &secondOut)
	if out == secondOut {
		t.Fatalf("predictor state did not advance: two calls with identical input produced identical output")
	}
}

func TestPredictorSelectorMatters(t *testing.T) {
	var d0, d1 Decoder
	var r, out0, out1 [10]int16
	for i := range r {
		r[i] = 200
	}
	d0.applyPredictor(0, &r, &out0)
	d1.applyPredictor(1, &r, &out1)
	if out0 == out1 {
		t.Fatalf("predictor output identical for L0=0 and L0=1: selector ignored")
	}
}
