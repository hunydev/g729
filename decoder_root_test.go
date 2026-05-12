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

func TestDecoder_DecodeFrameEnhanced_AcceptsValidShape(t *testing.T) {
	d := NewDecoder()
	var (
		bits [FrameBytes]byte
		out  [FrameSamples]int16
	)
	if err := d.DecodeFrameEnhanced(bits[:], out[:]); err != nil {
		t.Fatalf("unexpected error on zero frame: %v", err)
	}
}

func TestDecoder_DecodeFramePostfilterBlend_AcceptsValidShape(t *testing.T) {
	d := NewDecoder()
	var (
		bits [FrameBytes]byte
		out  [FrameSamples]int16
	)
	if err := d.DecodeFramePostfilterBlend(bits[:], out[:], 1, 2); err != nil {
		t.Fatalf("unexpected error on zero frame: %v", err)
	}
}

func TestDecoder_ResetRestoresFreshFrameOutput(t *testing.T) {
	var bits [FrameBytes]byte
	for i := range bits {
		bits[i] = byte(i*17 + 3)
	}

	d := NewDecoder()
	var got, want, scratch [FrameSamples]int16
	if err := d.DecodeFrame(bits[:], scratch[:]); err != nil {
		t.Fatalf("warmup DecodeFrame: %v", err)
	}
	d.Reset()
	if err := d.DecodeFrame(bits[:], got[:]); err != nil {
		t.Fatalf("DecodeFrame after Reset: %v", err)
	}

	fresh := NewDecoder()
	if err := fresh.DecodeFrame(bits[:], want[:]); err != nil {
		t.Fatalf("fresh DecodeFrame: %v", err)
	}
	if got != want {
		t.Fatal("Reset output differs from fresh decoder")
	}
}
