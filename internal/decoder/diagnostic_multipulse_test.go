package decoder

import (
	"math"
	"testing"

	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/postfilter"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_FourPulseCanonicalChain feeds the canonical ITU 4-pulse
// fixed-codebook stimulus through the same 13-boundary harness as the
// single-pulse test (Phase 1k Task 6). Purpose: reproduce the
// Phase 1j-suspected gcQ12 saturation when Σc²=4 (vs Σc²=1 for the
// single-pulse case).
//
// Spec-derived expected values (ITU-T G.729 §3.9.1 eq 66-72):
//
//	Σc²        = 4
//	Ē_c (dB)   = -10.0
//	Ê (dB)     =  4.94
//	g'_c       =  5.6234
//
// Stimulus: 4 unit pulses at positions {5, 11, 22, 33} with alternating
// signs (matches one of the ITU ACELP track combinations). idx={GA:3,GB:7}
// matches the existing pathological tests so γ̂_c is reproducible.
func TestDiagnostic_FourPulseCanonicalChain(t *testing.T) {
	var c [40]int16
	c[5] = 8192
	c[11] = -8192
	c[22] = 8192
	c[33] = -8192

	const sigmaCSquaredTrue float64 = 4.0
	expectedEcBarDb := 10.0 * math.Log10(sigmaCSquaredTrue/40.0)
	expectedPredictedDb := 30.0 + 1.79*(-14.0)
	expectedLogGainDb := expectedPredictedDb - expectedEcBarDb
	expectedGcPrime := math.Pow(10, expectedLogGainDb/20)

	t.Logf("=== 4-pulse canonical spec-derived values ===")
	t.Logf("Σc² true              = %g", sigmaCSquaredTrue)
	t.Logf("Ē_c (true dB)         = %.4f", expectedEcBarDb)
	t.Logf("Ê predicted (true dB) = %.4f", expectedPredictedDb)
	t.Logf("logGain (true dB)     = %.4f", expectedLogGainDb)
	t.Logf("g'_c (true)           = %.4f", expectedGcPrime)

	// === Boundary ① fcb output ===
	var sumSqQ26 int64
	for n := 0; n < 40; n++ {
		sumSqQ26 += int64(c[n]) * int64(c[n])
	}
	cTrueSumSq := float64(sumSqQ26) / float64(int64(1)<<26)
	t.Logf("[① fcb] Σc²(raw=Q26)  = %d → true=%.4f (want %.4f)",
		sumSqQ26, cTrueSumSq, sigmaCSquaredTrue)

	// === Boundary ⑩-⑪ gain.Decode + BuildExcitation ===
	var gd gain.Decoder
	gpQ14, gcQ12 := gd.Decode(gain.Indices{GA: 3, GB: 7}, &c)
	gcTrue := float64(gcQ12) / 4096.0
	t.Logf("[⑩ gain] gpQ14=%d gcQ12=%d (true gc=%.4f)",
		gpQ14, gcQ12, gcTrue)
	t.Logf("[⑩ gain] spec g'_c=%.4f, max bound (γ̂_max≈2) = %.4f",
		expectedGcPrime, expectedGcPrime*2)
	t.Logf("[⑩ gain] saturation check: gcQ12 == ±32767/-32768 ? %v",
		gcQ12 == 32767 || gcQ12 == -32768)

	var v, u [40]int16
	synth.BuildExcitation(0, gcQ12, &v, &c, &u)
	t.Logf("[⑪ u] u[5]=%d u[11]=%d u[22]=%d u[33]=%d (other=0 expected)",
		u[5], u[11], u[22], u[33])

	// === Boundary ⑫ synth.Filter (trivial identity) ===
	var sy synth.Synthesizer
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s [40]int16
	sy.Filter(&a, &u, &s)
	t.Logf("[⑫ s] s[5]=%d s[11]=%d s[22]=%d s[33]=%d",
		s[5], s[11], s[22], s[33])

	// === Boundary ⑬ postfilter ===
	var pf postfilter.Postfilter
	var sPf [40]int16
	pf.Filter(&a, 40, &s, &sPf)
	t.Logf("[⑬ sPf] sPf[0..7]=%v", sPf[:8])
}
