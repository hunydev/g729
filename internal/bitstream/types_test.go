package bitstream

import "testing"

func TestConstants(t *testing.T) {
	if FrameBits != 80 {
		t.Errorf("FrameBits = %d, want 80", FrameBits)
	}
	if FrameBytes != 10 {
		t.Errorf("FrameBytes = %d, want 10", FrameBytes)
	}
}

func TestFrameZeroValue(t *testing.T) {
	var f Frame
	// All fields must default-zero, and reading one via interface must compile.
	if f.L0 != 0 || f.GB2 != 0 {
		t.Errorf("Frame zero value should have all fields zero")
	}
}
