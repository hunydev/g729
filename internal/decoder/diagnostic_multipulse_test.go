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

	// === Spec-aligned assertions (Case A — gcQ12 unsaturated, gcTrue
	// well within g'_c·γ̂_max bound) ===
	if sumSqQ26 != 4*(int64(1)<<26) {
		t.Errorf("BOUNDARY ① fcb energy: Σc²=%d, want %d (= 4·2^26)",
			sumSqQ26, int64(4)<<26)
	}
	maxExpectedGc := expectedGcPrime * 2.0
	if gcTrue < 0 || gcTrue > maxExpectedGc+0.5 {
		t.Errorf("BOUNDARY ⑩ gain: gcTrue=%.4f exceeds spec bound [0, %.4f]; "+
			"this is the Stage F target (14 dB suspect at gain log-domain math)",
			gcTrue, maxExpectedGc)
	}
	if gcQ12 == 32767 || gcQ12 == -32768 {
		t.Errorf("BOUNDARY ⑩ gain: gcQ12 saturated (%d); 14 dB suspect at "+
			"gain log-domain math — review §3.9.1 ecBar/predicted/logGain chain",
			gcQ12)
	}
}

// TestDiagnostic_PitchActivePulseChain reproduces the IIR-accumulation
// hypothesis from Phase 1k Stage D report §5: with gpQ14 ≠ 0 the
// adaptive-codebook contribution v[*] activates the LP synthesis IIR
// path. Stimulus: single +Q13 pulse + synthetic non-zero v matching a
// canonical pitch contribution of 0.5 Q14.
//
// This does NOT call pitch.AdaptiveCodebook (which would require
// pastExc state); instead we inject a deterministic v that simulates
// the post-pitch contribution, isolating the LP synthesis IIR.
func TestDiagnostic_PitchActivePulseChain(t *testing.T) {
	var c [40]int16
	c[0] = 8192 // single +Q13 pulse, identical to single-pulse harness

	// Synthetic v: a smooth ramp simulating a +0.5-amplitude pitch
	// contribution. Q0 sample magnitudes deliberately small so any
	// observed downstream amplification is clearly LP-attributable.
	var v [40]int16
	for n := 0; n < 40; n++ {
		v[n] = int16(n + 1) // 1..40 in Q0
	}
	const gpQ14 int16 = 8192 // 0.5 in Q14

	var gd gain.Decoder
	_, gcQ12 := gd.Decode(gain.Indices{GA: 3, GB: 7}, &c)
	gcTrue := float64(gcQ12) / 4096.0

	t.Logf("=== Pitch-active stimulus (gpQ14=%d ≈ %.4f) ===",
		gpQ14, float64(gpQ14)/16384.0)
	t.Logf("[⑩ gain] gcQ12=%d (true gc=%.4f)", gcQ12, gcTrue)

	var u [40]int16
	synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)
	t.Logf("[⑪ u] u[0..7]=%v", u[:8])
	t.Logf("[⑪ u] u[20..27]=%v", u[20:28])
	t.Logf("[⑪ u] u[32..39]=%v", u[32:40])

	// Trivial passthrough filter to isolate non-IIR effects.
	var sy synth.Synthesizer
	aTrivial := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var sTrivial [40]int16
	sy.Filter(&aTrivial, &u, &sTrivial)
	t.Logf("[⑫ s trivial] s[0..7]=%v", sTrivial[:8])
	t.Logf("[⑫ s trivial] s[32..39]=%v", sTrivial[32:40])

	// Non-trivial IIR: spec example A(z)=1−0.9·z^-1 in Q12.
	// a[0]=4096 (1.0 Q12), a[1]=-3686 (-0.9 Q12), rest 0.
	// Synthesizer applies 1/A(z) i.e. y[n]=u[n]+0.9·y[n-1] → strong IIR
	// memory. If 14 dB amplification arises here, branch C is the
	// target.
	var syIIR synth.Synthesizer
	aIIR := [11]int16{4096, -3686, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var sIIR [40]int16
	syIIR.Filter(&aIIR, &u, &sIIR)
	t.Logf("[⑫ s IIR] s[0..7]=%v", sIIR[:8])
	t.Logf("[⑫ s IIR] s[20..27]=%v", sIIR[20:28])
	t.Logf("[⑫ s IIR] s[32..39]=%v", sIIR[32:40])

	// Empirical amplification ratio (observe only).
	var maxTrivial, maxIIR int32
	for n := 0; n < 40; n++ {
		if val := int32(sTrivial[n]); val < 0 {
			if -val > maxTrivial {
				maxTrivial = -val
			}
		} else if val > maxTrivial {
			maxTrivial = val
		}
		if val := int32(sIIR[n]); val < 0 {
			if -val > maxIIR {
				maxIIR = -val
			}
		} else if val > maxIIR {
			maxIIR = val
		}
	}
	t.Logf("[⑫ amplification] max|sTrivial|=%d max|sIIR|=%d", maxTrivial, maxIIR)
	if maxTrivial > 0 && maxIIR > 0 {
		ratioDb := 20.0 * math.Log10(float64(maxIIR)/float64(maxTrivial))
		t.Logf("[⑫ amplification] max|sIIR|/max|sTrivial| = %.4f dB",
			ratioDb)
	}
}
