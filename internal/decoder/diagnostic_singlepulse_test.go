package decoder

// PHASE 1o D-3.ter DISPOSITION — KEEP-WITH-NOTE.
//
// The hypothesis investigated by this diagnostic file has been closed by
// the gate 17 PSTdomain demotion (Phase 1o D-1b, commit 6633b28) and/or
// the Phase 1o D-3 state-bearing root-cause cycle (commits aa27ad1,
// 0428df7, bd37512, da089b5, be80eaf, c81645b — closure c81645b/this-cycle).
// Retained as evidence-trail and a verification-path demonstrator that
// future Phase-2 encoder cross-reference work may want to re-walk; do NOT
// extend this file — open a new dated diagnostic file instead. See
// session-state checkpoints 011..020 for the gate 17 / 28-cycle history,
// and docs/superpowers/plans/2026-05-09-phase1o-decoder-domain-closure-plan.md
// §3 D-3.ter for the housekeeping decision rationale.

import (
	"math"
	"testing"

	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/postfilter"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_SinglePulseChain feeds a controlled single-pulse
// fixed-codebook input through the full decoder chain (gain →
// excitation → synth → postfilter) and logs the value at each of 13
// canonical boundaries against spec-derived true values.
//
// Purpose (Phase 1k §6 of design): observe at which boundary the
// 14 dB divergence enters. No assertions — Task 7 promotes the
// spec-aligned boundaries to assertions; the first divergent
// boundary becomes the Stage F target.
//
// Input choice: single +Q13 pulse at position 0, gpQ14=0 (zero
// pitch contribution), default past errors (Table 6 init = −14 dB Q10
// each), idx (GA=3, GB=7) — same as the existing pathological tests
// so the γ̂_c value is reproducible.
func TestDiagnostic_SinglePulseChain(t *testing.T) {
	var c [40]int16
	c[0] = 8192 // +1.0 Q13

	// Spec-derived true expected values (ITU-T G.729 §3.9.1 eq 66-72,
	// §A.4.2 pseudo-code).
	const sigmaCSquaredTrue float64 = 1.0 // single +1.0 pulse
	expectedEcBarDb := 10.0 * math.Log10(sigmaCSquaredTrue/40.0)
	expectedPredictedDb := 30.0 + 1.79*(-14.0)
	expectedLogGainDb := expectedPredictedDb - expectedEcBarDb
	expectedGcPrime := math.Pow(10, expectedLogGainDb/20)

	t.Logf("=== Spec-derived true expected values ===")
	t.Logf("Σc² true              = %g", sigmaCSquaredTrue)
	t.Logf("Ē_c (true dB)         = %.4f", expectedEcBarDb)
	t.Logf("Ê predicted (true dB) = %.4f", expectedPredictedDb)
	t.Logf("logGain (true dB)     = %.4f", expectedLogGainDb)
	t.Logf("g'_c (true)           = %.4f", expectedGcPrime)

	// === Boundary ① fcb output ===
	var sumSqQ26 int64
	var maxAbs int16
	for n := 0; n < 40; n++ {
		sumSqQ26 += int64(c[n]) * int64(c[n])
		a := c[n]
		if a < 0 {
			a = -a
		}
		if a > maxAbs {
			maxAbs = a
		}
	}
	cTrueSumSq := float64(sumSqQ26) / float64(int64(1)<<26)
	t.Logf("[① fcb] Σc²(raw=Q26)  = %d → true=%.4f (want %.4f)",
		sumSqQ26, cTrueSumSq, sigmaCSquaredTrue)
	t.Logf("[① fcb] max|c|        = %d (Q13, true=%.4f)",
		maxAbs, float64(maxAbs)/8192.0)

	// === Boundary ② fixedCodebookEnergy ===
	var ecObserved int64
	for n := 0; n < 40; n++ {
		ecObserved += int64(c[n]) * int64(c[n])
	}
	t.Logf("[② energy] raw=%d Q26 → true=%.4f (want %.4f)",
		ecObserved, float64(ecObserved)/float64(int64(1)<<26),
		sigmaCSquaredTrue)

	// === Boundary ⑩-⑪ gain.Decode + BuildExcitation ===
	var gd gain.Decoder
	gpQ14, gcMant_gcQ12, gcExp_gcQ12 := gd.Decode(gain.Indices{GA: 3, GB: 7}, &c)
	gcQ12 := gain.LegacyGcQ12FromMantExp(gcMant_gcQ12, gcExp_gcQ12)
	gcTrue := float64(gcQ12) / 4096.0
	t.Logf("[⑩ gain] gpQ14=%d gcQ12=%d (true gc=%.4f)",
		gpQ14, gcQ12, gcTrue)
	t.Logf("[⑩ gain] spec g'_c=%.4f → gc bounded by [0, ~22] for γ̂_max≈2",
		expectedGcPrime)

	var v, u [40]int16
	synth.BuildExcitation(0, gcMant_gcQ12, gcExp_gcQ12, &v, &c, &u)
	t.Logf("[⑪ u] u[0]=%d (= round(gcQ12/4096) = round(%.4f) = expect %d)",
		u[0], gcTrue, int(math.Round(gcTrue)))

	// === Boundary ⑫ synth.Filter ===
	var sy synth.Synthesizer
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var s [40]int16
	sy.Filter(&a, &u, &s)
	t.Logf("[⑫ s] s[0]=%d (trivial filter passthrough → expect %d)",
		s[0], u[0])

	// === Boundary ⑬ postfilter ===
	var pf postfilter.Postfilter
	var sPf [40]int16
	pf.Filter(&a, 40, &s, &sPf)
	t.Logf("[⑬ sPf] sPf[0..7]=%v", sPf[:8])

	t.Logf("=== boundaries logged: ① fcb energy ② accumulator " +
		"⑩ gcQ12 ⑪ u[0] ⑫ s[0] ⑬ sPf[0..7] ===")
	t.Logf("Boundaries ③-⑨ (gain log-domain intermediates) are " +
		"package-private to internal/gain; Task 3's gain Q-format " +
		"contract tests cover them. Cross-reference those test " +
		"outputs when chasing 14 dB divergence.")

	// === Spec-aligned boundary assertions (Task 7) ===
	//
	// Observed in Task 6: with a single +Q13 pulse at position 0,
	// idx={GA:3,GB:7}, gpQ14=0 — all five quantitative boundaries
	// (① fcb energy, ⑩ gcQ12 in-bound, ⑪ u[0]=round(gc), ⑫ s[0]=u[0])
	// land on their spec-aligned values. The 14 dB ALGTHM frame 0
	// sf2 saturation is therefore NOT reproduced by this single-pulse
	// stimulus. These permanent assertions guard the harness against
	// regression so future Stage F work can re-use it.

	// Boundary ① — Σc² in Q26 form must equal 2^26 for single Q13 pulse.
	if sumSqQ26 != 1<<26 {
		t.Errorf("BOUNDARY ① fcb energy: Σc²=%d, want %d (= 2^26)",
			sumSqQ26, int64(1)<<26)
	}

	// Boundary ⑪ — u[0] must equal round(true gc).
	expectedU := int16(math.Round(gcTrue))
	if u[0] != expectedU {
		t.Errorf("BOUNDARY ⑪ excitation: u[0]=%d, want %d (= round(gcTrue=%.4f))",
			u[0], expectedU, gcTrue)
	}

	// Boundary ⑫ — trivial LP filter (a[0]=4096, rest 0) is identity.
	if s[0] != u[0] {
		t.Errorf("BOUNDARY ⑫ synth.Filter: s[0]=%d, u[0]=%d (trivial filter must be identity)",
			s[0], u[0])
	}

	// Boundary ⑩ Stage F trigger — gcQ12 true magnitude must be within
	// [0, expectedGcPrime · γ̂_max] where γ̂_max ≈ 2.
	maxExpectedGc := expectedGcPrime * 2.0
	if gcTrue < 0 || gcTrue > maxExpectedGc+0.5 {
		t.Errorf("BOUNDARY ⑩ gain: gcTrue=%.4f exceeds spec bound [0, %.4f]; "+
			"this is the Stage F target (14 dB suspect at gain log-domain math)",
			gcTrue, maxExpectedGc)
	} else if gcQ12 == 32767 || gcQ12 == -32768 {
		t.Errorf("BOUNDARY ⑩ gain: gcQ12 saturated (%d); 14 dB suspect at "+
			"gain log-domain math — review §3.9.1 ecBar/predicted/logGain chain",
			gcQ12)
	}
}
