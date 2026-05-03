package g729

import (
	"errors"
	"testing"
)

// Phase 2f API-1 — Public Encoder.EncodeFrame end-to-end wiring.
//
// These tests pin the public API contract per plan §6 Task API-1:
//
//   - len(pcm) != FrameSamples → ErrShortPCM (length-validation gate);
//   - basic call returns nil and populates a 10-byte canonical G.729
//     frame via internal/bitstream.Pack (PACK-1);
//   - non-silent input produces a non-zero packed frame;
//   - same input + fresh encoder → identical packed frame
//     (deterministic);
//   - steady-state allocs == 0 per testing.AllocsPerRun (I4).
//
// The signature mirrors the existing skeleton (out []byte) per plan
// §6 line 241. ErrNotImplemented is removed by API-1 per plan
// §6 line 142 + master plan §7 line 1050.

// pcmDeterministic produces a stable 80-sample pseudo-PCM frame;
// matches the Phase 2d INT-2 corpus shape so the API-1 gate exercises
// production-realistic encoder state.
func pcmDeterministic() [FrameSamples]int16 {
	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}
	return pcm
}

func TestEncoderEncodeFrame_LengthValidation(t *testing.T) {
	e := NewEncoder()
	var out [FrameBytes]byte
	if err := e.EncodeFrame(make([]int16, FrameSamples-1), out[:]); !errors.Is(err, ErrShortPCM) {
		t.Fatalf("short pcm: got %v want ErrShortPCM", err)
	}
	if err := e.EncodeFrame(make([]int16, FrameSamples+1), out[:]); !errors.Is(err, ErrShortPCM) {
		t.Fatalf("long pcm: got %v want ErrShortPCM", err)
	}
	pcm := pcmDeterministic()
	if err := e.EncodeFrame(pcm[:], make([]byte, FrameBytes-1)); !errors.Is(err, ErrShortOutput) {
		t.Fatalf("short out: got %v want ErrShortOutput", err)
	}
}

func TestEncoderEncodeFrame_ProducesTenBytes(t *testing.T) {
	e := NewEncoder()
	pcm := pcmDeterministic()
	out := make([]byte, FrameBytes)
	if err := e.EncodeFrame(pcm[:], out); err != nil {
		t.Fatalf("EncodeFrame returned %v; want nil", err)
	}
	if len(out) != FrameBytes {
		t.Fatalf("len(out) = %d; want %d", len(out), FrameBytes)
	}
}

func TestEncoderEncodeFrame_PCMShape(t *testing.T) {
	e := NewEncoder()
	pcm := pcmDeterministic()
	var out [FrameBytes]byte
	if err := e.EncodeFrame(pcm[:], out[:]); err != nil {
		t.Fatalf("EncodeFrame returned %v; want nil", err)
	}
	allZero := true
	for _, b := range out {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatalf("non-silence input produced all-zero packed frame: %v", out)
	}
}

func TestEncoderEncodeFrame_DeterministicOnRepeat(t *testing.T) {
	pcm := pcmDeterministic()

	encA := NewEncoder()
	encB := NewEncoder()

	var outA, outB [FrameBytes]byte
	if err := encA.EncodeFrame(pcm[:], outA[:]); err != nil {
		t.Fatalf("encA.EncodeFrame: %v", err)
	}
	if err := encB.EncodeFrame(pcm[:], outB[:]); err != nil {
		t.Fatalf("encB.EncodeFrame: %v", err)
	}
	if outA != outB {
		t.Fatalf("non-deterministic: A=%v B=%v", outA, outB)
	}
}

func TestEncoderEncodeFrame_ZeroAlloc(t *testing.T) {
	enc := NewEncoder()
	pcm := pcmDeterministic()
	var out [FrameBytes]byte

	// Warm up so encoder state is production-shaped.
	if err := enc.EncodeFrame(pcm[:], out[:]); err != nil {
		t.Fatalf("warmup: %v", err)
	}

	allocs := testing.AllocsPerRun(128, func() {
		if err := enc.EncodeFrame(pcm[:], out[:]); err != nil {
			t.Fatalf("EncodeFrame: %v", err)
		}
	})
	if allocs != 0 {
		t.Fatalf("Encoder.EncodeFrame allocated %.2f times per call; want 0 (I4)", allocs)
	}
}
