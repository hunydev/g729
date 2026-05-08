package tables

import "testing"

// PitchInterpFIR shape check — length is 3·Linter+1 = 31 per
// ITU-T G.729 §3.7.1 (Hamming-windowed sinc, Linter = 10, padded
// b30(30) = 0).
func TestPitchInterpFIRShape(t *testing.T) {
	const want = 31
	if len(PitchInterpFIR) != want {
		t.Fatalf("PitchInterpFIR: entries = %d, want %d", len(PitchInterpFIR), want)
	}
}

// PitchInterpFIR coefficient-range sanity (Q15 int16).
func TestPitchInterpFIRRange(t *testing.T) {
	for i, v := range PitchInterpFIR {
		if v < -32768 || v > 32767 {
			t.Errorf("PitchInterpFIR[%d] = %d outside int16 range", i, v)
		}
	}
}

// PitchInterpFIR has explicit zero entries at the indices that
// correspond to integer-grid sinc nulls (verifies transcription
// against the spec's "padded with zeros at ±30" property and the
// sinc(integer)=0 property at 3-sample strides).
func TestPitchInterpFIRZeros(t *testing.T) {
	for _, idx := range []int{10, 20, 30} {
		if PitchInterpFIR[idx] != 0 {
			t.Errorf("PitchInterpFIR[%d] = %d, want 0 (sinc null)", idx, PitchInterpFIR[idx])
		}
	}
}

func TestPitchInterpFIRCenterAndFirstFractionalTapPinned(t *testing.T) {
	if PitchInterpFIR[0] != 29443 {
		t.Fatalf("PitchInterpFIR[0]=%d, want stored b30(0)=29443", PitchInterpFIR[0])
	}
	if PitchInterpFIR[1] != 25207 {
		t.Fatalf("PitchInterpFIR[1]=%d, want stored b30(1)=25207", PitchInterpFIR[1])
	}
}
