package pcm

import "testing"

func TestPreProcessor_ZeroValueIsUsable(t *testing.T) {
	// A zero-value PreProcessor must behave like a freshly Reset one:
	// its filter state should all be zero, so feeding zeros produces zeros.
	var p PreProcessor
	in := make([]int16, FrameLength)
	out := make([]int16, FrameLength)
	p.Process(in, out)
	for i, v := range out {
		if v != 0 {
			t.Errorf("out[%d] = %d, want 0 on zero-state zero-input", i, v)
		}
	}
}

func TestFrameLength(t *testing.T) {
	if FrameLength != 80 {
		t.Fatalf("FrameLength = %d, want 80 (10 ms at 8 kHz)", FrameLength)
	}
}
