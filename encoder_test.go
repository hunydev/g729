package g729

import (
	"errors"
	"testing"
)

func TestEncoder_NewEncoder_NotNil(t *testing.T) {
	e := NewEncoder()
	if e == nil {
		t.Fatal("NewEncoder returned nil")
	}
}

func TestEncoder_EncodeFrame_RejectsShortPCM(t *testing.T) {
	e := NewEncoder()
	var out [FrameBytes]byte
	if err := e.EncodeFrame(make([]int16, FrameSamples-1), out[:]); !errors.Is(err, ErrShortPCM) {
		t.Fatalf("got %v want ErrShortPCM", err)
	}
}

func TestEncoder_EncodeFrame_RejectsShortOutput(t *testing.T) {
	e := NewEncoder()
	pcm := make([]int16, FrameSamples)
	if err := e.EncodeFrame(pcm, make([]byte, FrameBytes-1)); !errors.Is(err, ErrShortOutput) {
		t.Fatalf("got %v want ErrShortOutput", err)
	}
}

func TestEncoder_Reset_ZeroValueIsSafe(t *testing.T) {
	var e Encoder
	e.Reset()
}
