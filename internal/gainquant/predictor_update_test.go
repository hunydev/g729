package gainquant

import (
	"testing"

	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/tables"
)

// TestUpdatePastQuaEn_FIFOShift pins the §3.9.1 eq. (72) FIFO discipline:
// pastQuaEn[3] ← pastQuaEn[2] ← pastQuaEn[1] ← pastQuaEn[0] ← U(m).
//
//	U(m) = 20·log10(γ̂_c)   (Q10 dB; γ̂_c is Q13 input).
//
// For γ̂_c = 1.0 (Q13 = 8192), U(m) = 0 dB → new pastQuaEn[0] = 0.
func TestUpdatePastQuaEn_FIFOShift(t *testing.T) {
	past := [4]int16{100, 200, 300, 400}
	UpdatePastQuaEn(&past, 8192) // γ̂_c = 1.0 (Q13)

	if past[0] != 0 {
		t.Errorf("pastQuaEn[0] = %d (Q10 dB), want 0 for γ̂=1.0", past[0])
	}
	if past[1] != 100 {
		t.Errorf("pastQuaEn[1] = %d, want 100 (FIFO shift from old [0])", past[1])
	}
	if past[2] != 200 {
		t.Errorf("pastQuaEn[2] = %d, want 200", past[2])
	}
	if past[3] != 300 {
		t.Errorf("pastQuaEn[3] = %d, want 300", past[3])
	}
}

// TestUpdatePastQuaEn_LogUnity pins the U(m) = 20·log10(1) = 0 identity
// at the canonical γ̂=1.0 fixpoint.
func TestUpdatePastQuaEn_LogUnity(t *testing.T) {
	past := [4]int16{0, 0, 0, 0}
	UpdatePastQuaEn(&past, 8192) // γ̂ = 1.0 Q13
	if past[0] != 0 {
		t.Fatalf("U(m) for γ̂=1.0 = %d (Q10 dB), want 0", past[0])
	}
}

// TestUpdatePastQuaEn_LogTwo pins U(m) = 20·log10(2) ≈ 6.0206 dB.
//
//	γ̂ = 2.0  (Q13 = 16384)
//	U(m) Q10 = round(6.0206 · 1024) = 6165
//
// Tolerance ±2 LSB (matches log2Fixed accuracy contract).
func TestUpdatePastQuaEn_LogTwo(t *testing.T) {
	past := [4]int16{0, 0, 0, 0}
	UpdatePastQuaEn(&past, 16384) // γ̂ = 2.0 Q13

	const want int16 = 6165
	const tol = 2
	diff := int32(past[0]) - int32(want)
	if diff < -tol || diff > tol {
		t.Fatalf("U(m) for γ̂=2.0 = %d (Q10 dB), want %d ±%d", past[0], want, tol)
	}
}

// TestUpdatePastQuaEn_NonPositiveSeedsDefault pins the protective branch:
// γ̂ ≤ 0 is mathematically out-of-domain for log10; re-seed pastQuaEn[0]
// with the long-term default (PastErrorsDefault = -14 dB Q10 = -14336),
// matching the decoder's gain.Decode zero-energy guard.
func TestUpdatePastQuaEn_NonPositiveSeedsDefault(t *testing.T) {
	past := [4]int16{1, 2, 3, 4}
	UpdatePastQuaEn(&past, 0)
	if past[0] != gain.PastErrorsDefault {
		t.Errorf("γ̂=0 → past[0] = %d, want %d (PastErrorsDefault)",
			past[0], gain.PastErrorsDefault)
	}
}

// TestUpdatePastQuaEn_RoundTripWithPredictor cross-checks GQ-2 → GQ-3 →
// GQ-1 closure: feed γ̂_c from a known (ga, gb) into UpdatePastQuaEn,
// then re-derive the receiver-aligned U(m) value consumed by the next
// predictor call.
func TestUpdatePastQuaEn_RoundTripWithPredictor(t *testing.T) {
	// Use codebook entry (ga=0, gb=0): γ̂_c Q13 = GBK1[0][1] + GBK2[0][1].
	gammaCQ13 := tables.GainGBK1[0][1] + tables.GainGBK2[0][1]

	past := [4]int16{0, 0, 0, 0}
	UpdatePastQuaEn(&past, gammaCQ13)

	// Recompute U(m) directly via the receiver-aligned helper and compare.
	// This is the "prediction next call should produce matching value" probe:
	// the inserted past[0] must be exactly what gain.PredictedLogGain would
	// consume next subframe.
	if gammaCQ13 <= 0 {
		t.Skip("codebook entry non-positive; cross-check requires γ̂>0")
	}
	wantQ10 := gain.QuantizedPredictionErrorQ10(int32(gammaCQ13))

	if past[0] != wantQ10 {
		t.Fatalf("UpdatePastQuaEn(γ̂=%d Q13) → past[0]=%d, want %d (re-derived)",
			gammaCQ13, past[0], wantQ10)
	}
}

func TestUpdatePastQuaEn_UCurrentOracleBoundaries(t *testing.T) {
	cases := []struct {
		gammaCQ13 int16
		wantQ10   int16
	}{
		{gammaCQ13: 8360, wantQ10: 179},
		{gammaCQ13: 7339, wantQ10: -980},
		{gammaCQ13: 32023, wantQ10: 12124},
	}
	for _, tc := range cases {
		past := [4]int16{}
		UpdatePastQuaEn(&past, tc.gammaCQ13)
		if past[0] != tc.wantQ10 {
			t.Fatalf("UpdatePastQuaEn(γ̂=%d Q13) = %d, want %d",
				tc.gammaCQ13, past[0], tc.wantQ10)
		}
	}
}

// TestUpdatePastQuaEn_ZeroAlloc pins the encoder hot-path budget: the
// past-energy update runs once per subframe and must not allocate.
func TestUpdatePastQuaEn_ZeroAlloc(t *testing.T) {
	past := [4]int16{0, 0, 0, 0}
	allocs := testing.AllocsPerRun(128, func() {
		UpdatePastQuaEn(&past, 8192)
	})
	if allocs != 0 {
		t.Fatalf("UpdatePastQuaEn allocs/op = %.2f, want 0", allocs)
	}
}
