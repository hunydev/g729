package gainquant

import (
	"testing"

	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/tables"
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

// TestPredictedGcQ12Wide_MatchesBoundedWhenNoSaturation pins the profile
// split: the wide core predictor must be a no-op for ordinary states where
// the §3.9.1 MA prediction does not hit the legacy Word16 bound.
func TestPredictedGcQ12Wide_MatchesBoundedWhenNoSaturation(t *testing.T) {
	past := [4]int16{
		gain.PastErrorsDefault, gain.PastErrorsDefault,
		gain.PastErrorsDefault, gain.PastErrorsDefault,
	}
	var c [40]int16
	c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192

	bounded := PredictedGcQ12(&past, &c)
	wide := PredictedGcQ12Wide(&past, &c)
	if wide != bounded {
		t.Fatalf("PredictedGcQ12Wide cold-start = %d, bounded = %d; want equal before saturation", wide, bounded)
	}
}

// TestPredictedGcQ12Wide_ExceedsBoundedAfterPredictionSaturation protects the
// core-profile fix that avoids collapsing high-energy subframes onto the
// legacy Word16-bounded search predictor.
func TestPredictedGcQ12Wide_ExceedsBoundedAfterPredictionSaturation(t *testing.T) {
	past := [4]int16{32767, 32767, 32767, 32767}
	var c [40]int16
	c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192

	bounded := PredictedGcQ12(&past, &c)
	wide := PredictedGcQ12Wide(&past, &c)
	if wide <= bounded {
		t.Fatalf("PredictedGcQ12Wide high-energy = %d, bounded = %d; want wide > bounded", wide, bounded)
	}
}

// TestReconstructWide_OnlyExpandsFixedGain verifies that ReconstructWide keeps
// the transmitted pitch-gain reconstruction unchanged while using the wider
// fixed-gain predictor for core-profile local state commits.
func TestReconstructWide_OnlyExpandsFixedGain(t *testing.T) {
	past := [4]int16{32767, 32767, 32767, 32767}
	var c [40]int16
	c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192

	const gaPhys, gbPhys = uint8(2), uint8(4)
	gpBounded, mantBounded, expBounded := Reconstruct(&past, &c, gaPhys, gbPhys)
	gpWide, mantWide, expWide := ReconstructWide(&past, &c, gaPhys, gbPhys)

	if gpWide != gpBounded {
		t.Fatalf("ReconstructWide gp = %d, bounded gp = %d; pitch gain must be unchanged", gpWide, gpBounded)
	}
	if !mantExpGreater(mantWide, expWide, mantBounded, expBounded) {
		t.Fatalf("ReconstructWide fixed gain mant/exp=(%d,%d), bounded=(%d,%d); want wider gain",
			mantWide, expWide, mantBounded, expBounded)
	}
}

func TestReconstructWide_MatchesDecoderOnLowEnergyCodebook(t *testing.T) {
	past := [4]int16{
		gain.PastErrorsDefault, gain.PastErrorsDefault,
		gain.PastErrorsDefault, gain.PastErrorsDefault,
	}
	var c [40]int16
	c[0] = 1

	const gaBits, gbBits = uint8(4), uint8(9)
	gaPhys := tables.GainImap1[gaBits]
	gbPhys := tables.GainImap2[gbBits]
	gpEnc, mantEnc, expEnc := ReconstructWide(&past, &c, gaPhys, gbPhys)

	var d gain.Decoder
	gain.SeedDecoder(&d, past)
	gpDec, mantDec, expDec := d.Decode(gain.Indices{GA: gaBits, GB: gbBits}, &c)

	if gpEnc != gpDec || mantEnc != mantDec || expEnc != expDec {
		t.Fatalf("ReconstructWide=(gp=%d, mant=%d, exp=%d), decoder=(gp=%d, mant=%d, exp=%d)",
			gpEnc, mantEnc, expEnc, gpDec, mantDec, expDec)
	}
}

func TestReconstructWide_MatchesDecoderDecodeGrid(t *testing.T) {
	codebooks := []struct {
		name string
		c    [40]int16
	}{
		{
			name: "four unit pulses",
			c: func() [40]int16 {
				var c [40]int16
				c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192
				return c
			}(),
		},
		{
			name: "low energy",
			c: func() [40]int16 {
				var c [40]int16
				c[0] = 1
				return c
			}(),
		},
		{
			name: "pitch enhanced shape",
			c: func() [40]int16 {
				var c [40]int16
				c[0], c[6], c[12], c[18], c[24], c[30] = 8192, -4096, 6144, -3072, 2048, -1024
				return c
			}(),
		},
	}
	pasts := []struct {
		name string
		past [4]int16
	}{
		{name: "cold", past: [4]int16{gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault}},
		{name: "high", past: [4]int16{32767, 32767, 32767, 32767}},
		{name: "mixed", past: [4]int16{12000, -8000, 3000, gain.PastErrorsDefault}},
		{name: "low", past: [4]int16{-32768, -32768, -32768, -32768}},
	}
	indices := []struct {
		gaBits uint8
		gbBits uint8
	}{
		{0, 0},
		{4, 9},
		{7, 15},
	}

	for _, cb := range codebooks {
		for _, ps := range pasts {
			for _, idx := range indices {
				gaPhys := tables.GainImap1[idx.gaBits]
				gbPhys := tables.GainImap2[idx.gbBits]
				gpEnc, mantEnc, expEnc := ReconstructWide(&ps.past, &cb.c, gaPhys, gbPhys)

				var dec gain.Decoder
				gain.SeedDecoder(&dec, ps.past)
				gpDec, mantDec, expDec := dec.Decode(gain.Indices{GA: idx.gaBits, GB: idx.gbBits}, &cb.c)

				if gpEnc != gpDec || mantEnc != mantDec || expEnc != expDec {
					t.Fatalf("%s/%s GA=%d GB=%d: ReconstructWide=(gp=%d, mant=%d, exp=%d), decoder=(gp=%d, mant=%d, exp=%d)",
						cb.name, ps.name, idx.gaBits, idx.gbBits,
						gpEnc, mantEnc, expEnc, gpDec, mantDec, expDec)
				}
			}
		}
	}
}

func mantExpGreater(mant1 int16, exp1 int8, mant2 int16, exp2 int8) bool {
	if exp1 != exp2 {
		return exp1 > exp2
	}
	return mant1 > mant2
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
