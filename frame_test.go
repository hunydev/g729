package g729

import (
	"testing"
)

func TestEncodeFrame_TopLevelDelegates(t *testing.T) {
	e := NewEncoder()
	pcm := make([]int16, FrameSamples)
	var out [FrameBytes]byte
	if err := EncodeFrame(e, pcm, out[:]); err != nil {
		t.Fatalf("EncodeFrame returned %v; want nil (post API-1 wiring)", err)
	}
}

func TestDecodeFrame_TopLevelDelegates(t *testing.T) {
	d := NewDecoder()
	var (
		bits [FrameBytes]byte
		out  [FrameSamples]int16
	)
	if err := DecodeFrame(d, bits[:], out[:]); err != nil {
		t.Fatalf("unexpected error on zero frame: %v", err)
	}
}
