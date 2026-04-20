package bitstream

import (
	"bytes"
	"errors"
	"testing"
)

func TestPack_AllZero(t *testing.T) {
	var f Frame
	var out [FrameBytes]byte
	if err := Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	var want [FrameBytes]byte
	if !bytes.Equal(out[:], want[:]) {
		t.Errorf("Pack(zero) = % x, want % x", out, want)
	}
}

func TestPack_AllOnesAtMaxValues(t *testing.T) {
	// Set every field to the max its bit width allows.
	f := Frame{
		L0: 1, L1: 0x7F, L2: 0x1F, L3: 0x1F,
		P1: 0xFF, P0: 1,
		C1: 0x1FFF, S1: 0xF, GA1: 7, GB1: 0xF,
		P2: 0x1F,
		C2: 0x1FFF, S2: 0xF, GA2: 7, GB2: 0xF,
	}
	var out [FrameBytes]byte
	if err := Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	// Every bit should be 1, so all bytes are 0xFF.
	for i, b := range out {
		if b != 0xFF {
			t.Errorf("out[%d] = %#x, want 0xFF", i, b)
		}
	}
}

func TestPack_OnlyL0_FirstBit(t *testing.T) {
	f := Frame{L0: 1}
	var out [FrameBytes]byte
	if err := Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	want := [FrameBytes]byte{0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(out[:], want[:]) {
		t.Errorf("Pack(L0=1) = % x, want % x", out, want)
	}
}

func TestPack_OnlyL1_Bits1Through7(t *testing.T) {
	// L1 is a 7-bit field starting at bit 1 of byte 0 (MSB-first).
	// L1 = 0b1010101 -> first byte bits: 0 1 0 1 0 1 0 1 = 0x55.
	f := Frame{L1: 0b1010101}
	var out [FrameBytes]byte
	if err := Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	want := [FrameBytes]byte{0x55, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(out[:], want[:]) {
		t.Errorf("Pack(L1=0x55) = % x, want % x", out, want)
	}
}

func TestPack_ShortOutput(t *testing.T) {
	var f Frame
	short := make([]byte, FrameBytes-1)
	if err := Pack(&f, short); !errors.Is(err, ErrShortOutput) {
		t.Errorf("Pack short = %v, want ErrShortOutput", err)
	}
}

func TestPack_ReusesBuffer(t *testing.T) {
	// Pack must clear the destination bytes before writing, so stale
	// bits don't leak through.
	out := make([]byte, FrameBytes)
	for i := range out {
		out[i] = 0xFF
	}
	var f Frame // all zero
	if err := Pack(&f, out); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	for i, b := range out {
		if b != 0 {
			t.Errorf("out[%d] = %#x after Pack(zero), want 0", i, b)
		}
	}
}
