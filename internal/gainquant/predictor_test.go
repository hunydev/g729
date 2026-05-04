package gainquant

import (
	"testing"

	"github.com/exedev/g729/internal/gain"
)

// TestPredictedGcQ12_ColdStartFourPulses is the GQ-1 RED-part-B golden:
// for the cold-start MA-predictor tap line (pastQuaEn = 4× -14336 Q10)
// and a fixed-codebook vector of 4 unit pulses (c[i] = ±8192 Q13 at
// canonical positions), the predicted gain g'c per §3.9.1 eq. (71)
// must match the analytic float reference within ±1% (Q12 LSBs).
//
// Analytic walk (decimal):
//
//	Σ b_i = (5571 + 4751 + 2785 + 1556)/8192 ≈ 1.7900
//	Ê(m) = E̅ + Σ b_i · Û(m-i)
//	     = 30 dB + 1.7900 · (−14 dB)
//	     ≈ 30 − 25.06 ≈ 4.94 dB                    (eq. 69)
//	E_c  = (1/40) · Σ c² = 4/40 = 0.1
//	Ē(m) = 10·log10(E_c) = −10 dB                  (eq. 66)
//	g'c  = 10^((Ê − Ē)/20) = 10^((4.94+10)/20)
//	     ≈ 10^0.747 ≈ 5.585                         (eq. 71)
//	g'c·2^12 ≈ 22878 (Q12)
func TestPredictedGcQ12_ColdStartFourPulses(t *testing.T) {
	past := [4]int16{
		gain.PastErrorsDefault, gain.PastErrorsDefault,
		gain.PastErrorsDefault, gain.PastErrorsDefault,
	}
	var c [40]int16
	c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192

	got := PredictedGcQ12(&past, &c)

	const wantApprox int32 = 22878 // Q12 (5.585 × 2^12)
	const tol = 230                // ≈ 1%
	diff := got - wantApprox
	if diff < -tol || diff > tol {
		t.Fatalf("PredictedGcQ12 cold-start 4 pulses = %d (Q12), want %d ±%d",
			got, wantApprox, tol)
	}

	if got <= 0 {
		t.Fatalf("PredictedGcQ12 = %d, must be > 0 for non-zero codebook energy", got)
	}
}

// TestPredictedGcQ12_PureFunction asserts the predictor reads its
// inputs without mutation; the FIFO and codebook are owned by the
// caller (encoder).
func TestPredictedGcQ12_PureFunction(t *testing.T) {
	past := [4]int16{
		gain.PastErrorsDefault, gain.PastErrorsDefault,
		gain.PastErrorsDefault, gain.PastErrorsDefault,
	}
	var c [40]int16
	c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192
	pastOrig, cOrig := past, c

	_ = PredictedGcQ12(&past, &c)

	if past != pastOrig {
		t.Errorf("PredictedGcQ12 mutated past: got %v, want %v", past, pastOrig)
	}
	if c != cOrig {
		t.Errorf("PredictedGcQ12 mutated c (40-element diff)")
	}
}

// TestPredictedGcQ12_ZeroEnergyReturnsZero pins the zero-energy guard:
// when the codebook has no energy, g'c is not defined (log10(0) = −∞);
// the predictor must return 0 rather than saturating to int16 extrema.
// This mirrors the decoder's protective branch in gain.Decode.
func TestPredictedGcQ12_ZeroEnergyReturnsZero(t *testing.T) {
	past := [4]int16{
		gain.PastErrorsDefault, gain.PastErrorsDefault,
		gain.PastErrorsDefault, gain.PastErrorsDefault,
	}
	var c [40]int16
	got := PredictedGcQ12(&past, &c)
	if got != 0 {
		t.Fatalf("PredictedGcQ12(zero codebook) = %d, want 0", got)
	}
}

// TestPredictedGcQ12_ZeroAlloc pins the encoder hot-path budget: the
// gain predictor runs once per subframe inside the GQ-2 search loop
// and must not allocate.
func TestPredictedGcQ12_ZeroAlloc(t *testing.T) {
	past := [4]int16{
		gain.PastErrorsDefault, gain.PastErrorsDefault,
		gain.PastErrorsDefault, gain.PastErrorsDefault,
	}
	var c [40]int16
	c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192

	allocs := testing.AllocsPerRun(128, func() {
		_ = PredictedGcQ12(&past, &c)
	})
	if allocs != 0 {
		t.Fatalf("PredictedGcQ12 allocs/op = %.2f, want 0", allocs)
	}
}
