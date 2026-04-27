package decoder

import (
	"math"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/postfilter"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_ALGTHMFrame0SF0Replay parses the actual ALGTHM.BIT
// frame 0 and walks the sf0 pipeline stage-by-stage with t.Logf at each
// boundary. This is the most decisive Stage D-bis stimulus: it uses
// the same indices that produce the observed 14 dB sf2 saturation in
// the production decoder.
//
// The replay does NOT use Decoder.decodeSubframe (which mutates state);
// each module is called directly with fresh state so observations are
// pure functions of the parsed indices.
//
// Cross-check: ALGTHM.PST sample n / 2 ≈ s[n] (Q0 pre-ScaleUpSat). The
// /2 is because pcm.ScaleUpSat applies left-shift-by-1 with saturation.
// Sample 0 is locked by Phase 1i — must remain 2.
func TestDiagnostic_ALGTHMFrame0SF0Replay(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}
	t.Logf("=== Parsed ALGTHM frame 0 indices ===")
	t.Logf("LSP: L0=%d L1=%d L2=%d L3=%d", f.L0, f.L1, f.L2, f.L3)
	t.Logf("sf0: P1=%d (parity P0=%d) C1=%d S1=%d GA1=%d GB1=%d",
		f.P1, f.P0, f.C1, f.S1, f.GA1, f.GB1)
	t.Logf("sf1: P2=%d C2=%d S2=%d GA2=%d GB2=%d",
		f.P2, f.C2, f.S2, f.GA2, f.GB2)

	// === LSP decode (both subframes' a[]) ===
	var lspDec lsp.Decoder
	sf0A, _ := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1),
		L2: uint8(f.L2), L3: uint8(f.L3),
	})
	t.Logf("=== sf0 LP coefficients a[0..10] (Q12) ===")
	for i := 0; i < 11; i++ {
		t.Logf("  a[%2d] = %6d (= %.6f)", i, sf0A[i], float64(sf0A[i])/4096.0)
	}

	// === Pitch delay sf0 ===
	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	t.Logf("=== sf0 pitch delay: tInt=%d tFrac=%+d ===", tInt1, tFrac1)

	// === Adaptive codebook v ===
	// Empty pastExc: v will be the "no past" case. This isolates
	// fcb/gain/excitation effects from pastExc state.
	var pastExc [pastExcLen]int16
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt1, tFrac1, pastExc[:], &v)
	t.Logf("=== sf0 v[*] (with empty pastExc) ===")
	t.Logf("  v[0..7]   = %v", v[:8])
	t.Logf("  v[20..27] = %v", v[20:28])

	// === Fixed codebook ===
	betaQ14 := fcb.ClampPitchGainForEnhancement(0) // first subframe, no prevGp
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt1, betaQ14, &c)
	t.Logf("=== sf0 c[*] non-zero entries ===")
	for n := 0; n < 40; n++ {
		if c[n] != 0 {
			t.Logf("  c[%2d] = %+d (= %+.4f Q13)", n, c[n], float64(c[n])/8192.0)
		}
	}
	var sumSqQ26 int64
	for n := 0; n < 40; n++ {
		sumSqQ26 += int64(c[n]) * int64(c[n])
	}
	t.Logf("  Σc² (raw Q26) = %d → true = %.4f",
		sumSqQ26, float64(sumSqQ26)/float64(int64(1)<<26))

	// === Gain decode ===
	var gn gain.Decoder
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	gcTrue := float64(gcQ12) / 4096.0
	gpTrue := float64(gpQ14) / 16384.0
	t.Logf("=== sf0 gain ===")
	t.Logf("  gpQ14=%d (= %.4f) gcQ12=%d (= %.4f)", gpQ14, gpTrue, gcQ12, gcTrue)
	t.Logf("  gcQ12 saturated? %v", gcQ12 == 32767 || gcQ12 == -32768)

	// Spec-derived expected g'_c for cross-check.
	cTrueSumSq := float64(sumSqQ26) / float64(int64(1)<<26)
	if cTrueSumSq > 0 {
		expectedEcBarDb := 10.0 * math.Log10(cTrueSumSq/40.0)
		expectedPredictedDb := 30.0 + 1.79*(-14.0)
		expectedGcPrime := math.Pow(10, (expectedPredictedDb-expectedEcBarDb)/20)
		t.Logf("  spec g'_c (default pastErrors) = %.4f → max gc bound ≈ %.4f",
			expectedGcPrime, expectedGcPrime*2)
	}

	// === Excitation u ===
	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)
	t.Logf("=== sf0 u[*] ===")
	t.Logf("  u[0..7]   = %v", u[:8])
	t.Logf("  u[20..27] = %v", u[20:28])
	t.Logf("  u[32..39] = %v", u[32:40])

	// === Synthesis filter ===
	var sy synth.Synthesizer
	var s [subframeLen]int16
	sy.Filter(&sf0A, &u, &s)
	t.Logf("=== sf0 s[*] (post-LP synth) ===")
	t.Logf("  s[0..7]   = %v", s[:8])
	t.Logf("  s[20..27] = %v", s[20:28])
	t.Logf("  s[32..39] = %v", s[32:40])

	// === Postfilter ===
	var pst postfilter.Postfilter
	var sPf [subframeLen]int16
	pst.Filter(&sf0A, tInt1, &s, &sPf)
	t.Logf("=== sf0 sPf[*] (post-postfilter) ===")
	t.Logf("  sPf[0..7]   = %v", sPf[:8])
	t.Logf("  sPf[20..27] = %v", sPf[20:28])
	t.Logf("  sPf[32..39] = %v", sPf[32:40])

	// === Cross-check vs ALGTHM.PST ===
	// Production decoder also runs hpFilter and pcm.ScaleUpSat. Here
	// we compare s[*] · 2 (no HP, no scale saturation) to want[*]
	// purely as an order-of-magnitude diagnostic.
	t.Logf("=== sf0 cross-check (s[n]·2 vs ALGTHM.PST[0][n]) ===")
	for _, n := range []int{0, 1, 2, 5, 10, 20, 30, 35, 39} {
		got2x := int32(s[n]) * 2
		want := int32(wantFrames[0][n])
		var deltaDb float64
		if want != 0 {
			ratio := math.Abs(float64(got2x-want)) / math.Abs(float64(want))
			if ratio > 0 {
				deltaDb = 20.0 * math.Log10(ratio+1e-9)
			}
		}
		t.Logf("  n=%2d: s·2=%6d  PST=%6d  Δ=%+d  Δ_dB=%.2f",
			n, got2x, want, got2x-want, deltaDb)
	}

	// === Full-subframe sample-by-sample sweep (s·2 vs PST) ===
	// For Stage D-bis report cross-stimulus boundary table.
	t.Logf("=== sf0 full sample sweep (s[n]·2 vs PST[0][n]) ===")
	for n := 0; n < 40; n++ {
		got2x := int32(s[n]) * 2
		want := int32(wantFrames[0][n])
		t.Logf("  n=%2d: s·2=%6d  PST=%6d  Δ=%+d", n, got2x, want, got2x-want)
	}

	// === sPf·2 vs PST sample sweep (production path closer) ===
	t.Logf("=== sf0 full sample sweep (sPf[n]·2 vs PST[0][n]) ===")
	for n := 0; n < 40; n++ {
		got2x := int32(sPf[n]) * 2
		want := int32(wantFrames[0][n])
		t.Logf("  n=%2d: sPf·2=%6d  PST=%6d  Δ=%+d", n, got2x, want, got2x-want)
	}
}
