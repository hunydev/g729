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

func TestEncoder_ResetRestoresFreshFrameOutput(t *testing.T) {
	pcm := makeRamp(FrameSamples)
	var got, want [FrameBytes]byte

	e := NewEncoder()
	if err := e.EncodeFrame(makeRamp(FrameSamples * 2)[:FrameSamples], got[:]); err != nil {
		t.Fatalf("warmup EncodeFrame: %v", err)
	}
	e.Reset()
	if err := e.EncodeFrame(pcm, got[:]); err != nil {
		t.Fatalf("EncodeFrame after Reset: %v", err)
	}

	fresh := NewEncoder()
	if err := fresh.EncodeFrame(pcm, want[:]); err != nil {
		t.Fatalf("fresh EncodeFrame: %v", err)
	}
	if got != want {
		t.Fatalf("Reset output differs from fresh encoder\n got=% x\nwant=% x", got, want)
	}
}

func TestEncoderGainCommitScaleHelpers(t *testing.T) {
	if got := applyGainQ14ToQ0(16384, 1000); got != 1000 {
		t.Fatalf("applyGainQ14ToQ0 unity = %d, want 1000", got)
	}
	if got := applyGcToQ12(16384, 0, 4096); got != 1 {
		t.Fatalf("applyGcToQ12 gc=1 z=1 = %d, want 1", got)
	}
	if got := applyGcToQ12(16384, 12, 4096); got != 4096 {
		t.Fatalf("applyGcToQ12 gc=4096 z=1 = %d, want 4096", got)
	}
}

func TestGainSearchSurfaceScaleHelpers(t *testing.T) {
	target := [FrameSamples / 2]int16{-3, -2, -1, 0, 1, 2, 3, 32767, -32768}
	scaleGainSearchTargetHalf(&target)
	wantTarget := [...]int16{-2, -1, -1, 0, 1, 1, 2, 16384, -16384}
	for i, want := range wantTarget {
		if target[i] != want {
			t.Fatalf("scaleGainSearchTargetHalf[%d] = %d, want %d", i, target[i], want)
		}
	}

	adaptive := [FrameSamples / 2]int16{-20000, -2, -1, 0, 1, 2, 20000}
	scaleGainSearchAdaptiveSevenHalves(&adaptive)
	wantAdaptive := [...]int16{-32768, -7, -4, 0, 4, 7, 32767}
	for i, want := range wantAdaptive {
		if adaptive[i] != want {
			t.Fatalf("scaleGainSearchAdaptiveSevenHalves[%d] = %d, want %d", i, adaptive[i], want)
		}
	}
}
