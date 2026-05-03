package g729

import (
	"errors"
	"testing"
)

func TestDecoder_NewDecoder_NotNil(t *testing.T) {
	d := NewDecoder()
	if d == nil {
		t.Fatal("NewDecoder returned nil")
	}
}

func TestDecoder_DecodeFrame_RejectsShortBitstream(t *testing.T) {
	d := NewDecoder()
	var out [FrameSamples]int16
	if err := d.DecodeFrame(make([]byte, FrameBytes-1), out[:]); !errors.Is(err, ErrShortBitstream) {
		t.Fatalf("got %v want ErrShortBitstream", err)
	}
}

func TestDecoder_DecodeFrame_RejectsShortOutput(t *testing.T) {
	d := NewDecoder()
	var bits [FrameBytes]byte
	if err := d.DecodeFrame(bits[:], make([]int16, FrameSamples-1)); !errors.Is(err, ErrShortOutput) {
		t.Fatalf("got %v want ErrShortOutput", err)
	}
}

func TestDecoder_DecodeFrame_AcceptsValidShape(t *testing.T) {
	d := NewDecoder()
	var (
		bits [FrameBytes]byte
		out  [FrameSamples]int16
	)
	if err := d.DecodeFrame(bits[:], out[:]); err != nil {
		t.Fatalf("unexpected error on zero frame: %v", err)
	}
}
