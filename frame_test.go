package g729

import (
	"errors"
	"testing"
)

func TestEncodeFrame_TopLevelDelegates(t *testing.T) {
	e := NewEncoder()
	pcm := make([]int16, FrameSamples)
	var out [FrameBytes]byte
	if err := EncodeFrame(e, pcm, out[:]); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("got %v want ErrNotImplemented (stub)", err)
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
