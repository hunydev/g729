package lsp

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
)

// TestINT1D4Pinpoint — Phase 2a-INT-1-d4 (Levinson saturation +
// Chebyshev band-centre bias pinpoint).
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-04-phase2a-int1-d4-pinpoint-plan.md
//
// d3 promoted H-L1 (Levinson saturation cascade on transient frame
// 29) to CONFIRMED and H-L2 (Chebyshev band-centre bias on frame 5
// q[5]) to PLAUSIBLE. d4 pinpoints the exact §-citable fault site
// for each via four targeted measurements:
//
//	S5_Frame29_LevinsonTrace        — per-iteration Levinson trace
//	S6_Frame29_AutocorrShiftSweep   — re-run Levinson with extra AC headroom
//	S7_Frame5_Chebyshev6Bisection   — 6-bisection re-run on frame-5 a[]
//	S8_Frame5_FloatUpstreamProjection — float Levinson+Chebyshev → fixed VQ
//
// ABSOLUTE CONSTRAINTS (parent plan §0.4 + d1/d2/d3 §0):
//   - Clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 source.
//     Spec source = G729E.{pdf,txt} only. The mirrored
//     autocorrelate / lag-window / Levinson / Chebyshev routines
//     below are byte-for-byte transcriptions of the production
//     bodies in internal/lpc/{autocorr,lagwindow,levinson}.go and
//     internal/lsp/lp_lsp.go (all written from spec without
//     consulting any external G.729 source). Their bit-exactness vs
//     production is asserted via final-a[] comparison in
//     verifyMirrorMatchesProduction().
//   - I6 binding: zero production-file changes.
//   - Measurement-only: t.Logf for every numeric value; t.Errorf
//     only where it provides a clear binary-pass/fail signal
//     documented in the d4 plan §5–§7.
func TestINT1D4Pinpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("d4 Levinson+Chebyshev pinpoint battery; -short")
	}
	// FIX-1B (Phase 2a-INT-1) widened production levinsonDurbin's
	// internal aWork/aPrev to Q24 int64 (see d4 plan §14). The d4
	// mirror in this file is a verbatim transcription of the prior
	// Q12 int32 production code — by design it now diverges from the
	// Q24 production. The d4 trace data has served its diagnostic
	// purpose (refined H-L1 → H-L1′; superseded by d5/FIX-1B), so
	// the entire battery is retired from CI rather than re-aligning
	// the mirror to the new production format.
	t.Skip("retired post-FIX-1B: d4 mirror tracks pre-FIX Q12 internals; superseded by d5 + FIX-1B")

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		framesToDrive    = 30
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read LSP.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read LSP.BIT: %v", err)
	}
	if len(inData) < framesToDrive*bytesPerInFrame {
		t.Fatalf("LSP.IN too short: %d", len(inData))
	}
	if len(bitData) < framesToDrive*bytesPerBitFrame {
		t.Fatalf("LSP.BIT too short: %d", len(bitData))
	}

	// Replay HPF + 240-sample sliding window for each frame so we
	// have the same `oldSpeech[]` production sees on entry to LP
	// analysis.
	type frameSnap struct {
		oldSpeech [240]int16
		aProd     [11]int16
	}
	snaps := make([]frameSnap, framesToDrive)
	{
		var pp pcm.PreProcessor
		var an lpc.Analyzer
		var oldSpeech [240]int16
		for f := 0; f < framesToDrive; f++ {
			var pcmFrame [samplesPerFrame]int16
			off := f * bytesPerInFrame
			for i := 0; i < samplesPerFrame; i++ {
				pcmFrame[i] = int16(binary.LittleEndian.Uint16(
					inData[off+2*i : off+2*i+2]))
			}
			var processed [samplesPerFrame]int16
			pp.Process(pcmFrame[:], processed[:])
			copy(oldSpeech[0:160], oldSpeech[80:240])
			copy(oldSpeech[160:240], processed[:])
			snaps[f].oldSpeech = oldSpeech
			_ = an.Analyze(&oldSpeech, &snaps[f].aProd)
		}
	}

	// Sanity: mirror pipeline reproduces production a[] bit-exactly
	// on every captured frame. If this drifts, every per-iteration
	// trace below is suspect — fail loudly.
	t.Run("S0_MirrorMatchesProduction", func(t *testing.T) {
		var maxDrift int32
		var driftFrame int = -1
		for f := 0; f < framesToDrive; f++ {
			var aMirror [11]int16
			runMirrorPipeline(&snaps[f].oldSpeech, &aMirror, 0)
			for i := 0; i <= 10; i++ {
				d := int32(aMirror[i]) - int32(snaps[f].aProd[i])
				if d < 0 {
					d = -d
				}
				if d > maxDrift {
					maxDrift = d
					driftFrame = f
				}
			}
		}
		t.Logf("mirror vs production a[] max drift across all 30 frames = %d Q12 (frame %d)",
			maxDrift, driftFrame)
		if maxDrift != 0 {
			t.Errorf("mirror pipeline diverges from production by %d Q12 at frame %d — d4 trace results are NOT trustworthy",
				maxDrift, driftFrame)
		}
	})

	// ===== S5: Frame-29 per-iteration Levinson trace =====
	t.Run("S5_Frame29_LevinsonTrace", func(t *testing.T) {
		s := snaps[29]

		// Mirror autocorrelate + applyLagWindow to recover the same
		// r'[0..10] production hands to levinsonDurbin.
		var windowed [240]int16
		mirrorWindowSpeech(&s.oldSpeech, &windowed)
		var r [11]int32
		scale := mirrorAutocorrelate(&windowed, &r, 0)
		mirrorApplyLagWindow(&r)
		t.Logf("=== Frame 29 Levinson per-iteration trace ===")
		t.Logf("AC scale chosen by production heuristic = %d (each r[k] divided by 4^scale vs raw)", scale)
		t.Logf("r'[0..10] (post lag-window, AC-shared scale) = %v", r)

		var trace levinsonTrace
		var aFix [11]int16
		mirrorLevinsonTraced(&r, &aFix, &trace)
		t.Logf("mirrored fixed-point a[] Q12 = %v", aFix)
		t.Logf("production       a[] Q12 = %v", s.aProd)

		// Float oracle on the same 240-sample input (re-derived from
		// spec; identical to d3 oracle).
		var aOracle [11]float64
		var rRaw, rLag, k, ePred [11]float64
		oracleLPAnalysisD3(&s.oldSpeech, &aOracle, &rRaw, &rLag, &k, &ePred)

		t.Logf("--- per-iteration table (i, fixed: e_after, kQ15, |aWork[1..i]|max ; float: E[i], k_i, |a[1..i]|max) ---")
		var firstSat int = -1
		for i := 1; i <= 10; i++ {
			it := trace.iter[i]
			var maxAbsFix int32
			for j := 1; j <= i; j++ {
				v := it.aWork[j]
				if v < 0 {
					v = -v
				}
				if v > maxAbsFix {
					maxAbsFix = v
				}
			}
			var maxAbsFloat float64
			for j := 1; j <= i; j++ {
				if av := math.Abs(aOracleAt(j, i, &k)); av > maxAbsFloat {
					maxAbsFloat = av
				}
			}
			saturated := it.kQ15 == math.MaxInt16 || it.kQ15 == math.MinInt16
			anyAWorkSat := false
			for j := 1; j < i; j++ {
				if it.aWork[j] == math.MaxInt32 || it.aWork[j] == math.MinInt32 {
					anyAWorkSat = true
				}
			}
			satMark := ""
			if saturated || anyAWorkSat {
				satMark = " *SAT*"
				if firstSat < 0 {
					firstSat = i
				}
			}
			t.Logf("  i=%2d  fix: e=%d kQ15=%d a[1..i]max=%d  ; float: E=%.4e k=%+.6f a[1..i]max=%.4f%s",
				i, it.eAfter, it.kQ15, maxAbsFix,
				ePred[i], k[i], maxAbsFloat, satMark)
			t.Logf("        fix sum (pre-shift) = %d   q = num/e = %d", it.sum, it.qDivResult)
			t.Logf("        fix aWork[0..%d] = %v", i, it.aWork[:i+1])
		}
		if firstSat >= 0 {
			t.Logf("CASCADE-START: first saturation event at iteration i=%d", firstSat)
			t.Logf("  Diagnosis. levinsonDurbin computes")
			t.Logf("    num = -(sum << 3)   // sum is int64 of (Q12 · r'-scale) products")
			t.Logf("    q   = num / e        // e starts at r'(0), updated by (1-k²)>>30")
			t.Logf("  When e shrinks faster than sum (i.e. as the LP-analysis")
			t.Logf("  recursion approaches Nyquist energy on transient frames),")
			t.Logf("  the quotient q overflows int16 and kQ15 saturates at ±MaxInt16")
			t.Logf("  (= ±0.99997 instead of the true k_i = %+.4f).", k[firstSat])
			t.Logf("  The subsequent inner-loop update")
			t.Logf("    aWork[j] = aPrev[j] + (kQ15 · aPrev[i-j])>>15")
			t.Logf("  then propagates the saturated kQ15 into every aWork[j],")
			t.Logf("  producing the mirror-symmetric ±sat clones with trailing")
			t.Logf("  zeros documented in d3 §3.")
			t.Logf("  Spec §3.2.2 lines 717–736 specify the recursion in")
			t.Logf("  REAL arithmetic with no Q-format clipping — the spec's")
			t.Logf("  pre-condition is that r' be positive-definite (which the")
			t.Logf("  float oracle confirms IS satisfied at frame 29) so that")
			t.Logf("  |k_i|<1 ∀i. The Q-format implementation must therefore")
			t.Logf("  carry enough headroom on `e` and `sum` that the QUOTIENT")
			t.Logf("  never overflows when the true k_i is in (-1,+1).")
		} else {
			t.Logf("CASCADE-START: no saturation observed in mirror — the")
			t.Logf("  divergence between mirrored a[] and production a[] (if")
			t.Logf("  any) lies elsewhere; investigate window LUT or lag table.")
		}
	})

	// ===== S6: Autocorrelation shift sweep =====
	t.Run("S6_Frame29_AutocorrShiftSweep", func(t *testing.T) {
		s := snaps[29]
		var windowed [240]int16
		mirrorWindowSpeech(&s.oldSpeech, &windowed)

		// Force three different scales: the heuristic pick, +1, +2.
		var rBase [11]int32
		baseScale := mirrorAutocorrelate(&windowed, &rBase, 0)
		t.Logf("=== Frame 29 AC-shift sweep ===")
		t.Logf("baseline AC scale (production heuristic) = %d", baseScale)

		// Float oracle for reference k[].
		var aOracle [11]float64
		var rRaw, rLag, kFlt, ePred [11]float64
		oracleLPAnalysisD3(&s.oldSpeech, &aOracle, &rRaw, &rLag, &kFlt, &ePred)

		for extra := 0; extra <= 4; extra++ {
			var r [11]int32
			scale := mirrorAutocorrelate(&windowed, &r, baseScale+extra)
			mirrorApplyLagWindow(&r)
			var a [11]int16
			var trace levinsonTrace
			mirrorLevinsonTraced(&r, &a, &trace)

			// Recover real-valued k_fix per iteration (kQ15/32768) so
			// we can compare scale-by-scale against the float oracle.
			var maxKErr float64
			for i := 1; i <= 10; i++ {
				kFix := float64(trace.iter[i].kQ15) / 32768.0
				e := math.Abs(kFix - kFlt[i])
				if e > maxKErr {
					maxKErr = e
				}
			}

			// Stability + sign-change probe via production grid60.
			var f1, f2 [6]int32
			computeF1F2(&a, &f1, &f2)
			nF1 := countSignChangesGrid60(&f1)
			nF2 := countSignChangesGrid60(&f2)

			anySat := false
			for i := 1; i <= 10; i++ {
				if trace.iter[i].kQ15 == math.MaxInt16 || trace.iter[i].kQ15 == math.MinInt16 {
					anySat = true
				}
			}
			t.Logf("  scale=%d  a[]=%v  kQ15-saturation=%v  max|k_fix-k_flt|=%.4f  F1=%d F2=%d (need 5 each)",
				scale, a, anySat, maxKErr, nF1, nF2)
		}

		t.Logf("Interpretation:")
		t.Logf("  - If kQ15-saturation flips false at scale+1 or scale+2 AND")
		t.Logf("    F1=F2=5, then the Levinson cascade is RECOVERED purely by")
		t.Logf("    one extra AC-shift bit → root cause is autocorr.go's")
		t.Logf("    minimal-shift heuristic (lines 50–57); spec §3.2.1 line 691")
		t.Logf("    leaves AC-norm under-specified (\"to avoid arithmetic")
		t.Logf("    problems\"); fix = require ≥1 extra bit of headroom.")
		t.Logf("  - If saturation persists at scale+2, the bug is intrinsic to")
		t.Logf("    levinsonDurbin's Q-format choices (e/sum/num shift")
		t.Logf("    arrangement); fix lives in internal/lpc/levinson.go.")
	})

	// ===== S7: 6-bisection Chebyshev re-run on frame 5 =====
	t.Run("S7_Frame5_Chebyshev6Bisection", func(t *testing.T) {
		// Decoder oracle q[] for frame 5 (already established in d3
		// §5.2 = [31577 30044 27987 22264 12339 -3048 -14910 -21293
		// -27146 -29427]). Recompute here to keep this test
		// self-contained.
		var dec Decoder
		var rawLSP [10]int16
		for f := 0; f <= 5; f++ {
			l0, l1, l2, l3 := extractLSPFieldsD3(
				bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame])
			dec.Decode(Indices{L0: l0, L1: l1, L2: l2, L3: l3})
			rawLSP = dec.prevLSP
		}
		t.Logf("=== Frame 5 Chebyshev 6-bisection re-run ===")
		t.Logf("decoder oracle q[] Q15  = %v", rawLSP)

		// Production (4-bisection) baseline from same a[].
		aProd5 := snaps[5].aProd
		var qProd4 [10]int16
		if err := LPToLSP(&aProd5, &qProd4); err != nil {
			t.Fatalf("production LPToLSP frame 5: %v", err)
		}
		t.Logf("production 4-bisection q[]  = %v", qProd4)

		// 6- and 8-bisection variants on identical a[].
		var q6, q8 [10]int16
		if err := mirrorFindLSPRootsNBisect(&aProd5, &q6, 6); err != nil {
			t.Fatalf("mirror 6-bisect: %v", err)
		}
		if err := mirrorFindLSPRootsNBisect(&aProd5, &q8, 8); err != nil {
			t.Fatalf("mirror 8-bisect: %v", err)
		}
		t.Logf("mirror 6-bisection q[]  = %v", q6)
		t.Logf("mirror 8-bisection q[]  = %v", q8)

		report := func(label string, q [10]int16) {
			var maxAbs, sumAbs int32
			var argmax = -1
			for i := 0; i < 10; i++ {
				d := int32(q[i]) - int32(rawLSP[i])
				if d < 0 {
					d = -d
				}
				if d > maxAbs {
					maxAbs = d
					argmax = i
				}
				sumAbs += d
			}
			t.Logf("  %s vs decoder: max|Δ|=%d Q15 (at i=%d) sum|Δ|=%d Q15",
				label, maxAbs, argmax, sumAbs)
			for i := 0; i < 10; i++ {
				d := int32(q[i]) - int32(rawLSP[i])
				t.Logf("    Δq[%d] = %d", i, d)
			}
		}
		report("4-bisect (production)", qProd4)
		report("6-bisect (mirror)    ", q6)
		report("8-bisect (mirror)    ", q8)

		// Spec text reminder.
		t.Logf("Spec §3.2.3 lines 783–784 (verbatim): \"the polynomials F1(z) and")
		t.Logf("  F2(z) [are evaluated] at 60 points equally spaced between 0 and")
		t.Logf("  π and checking for sign changes. A sign change signifies the")
		t.Logf("  existence of a root and the sign change interval is then")
		t.Logf("  divided four times to allow better tracking of the root.\"")
		t.Logf("Reading. The spec literally specifies FOUR bisections. Production")
		t.Logf("  (bisectRoot, lp_lsp.go ~line 154: `for i := 0; i < 4; i++`) is")
		t.Logf("  spec-conformant. If the 6-bisection variant materially closes")
		t.Logf("  the q[5] gap, then the §3.2.3 4-bisection precision floor")
		t.Logf("  (~28 LSB Q15 worst case for a smooth-slope root) is being")
		t.Logf("  systematically biased toward one endpoint of the final 2×LSB")
		t.Logf("  bracket. The §-conformant fix is then NOT to bump the")
		t.Logf("  bisection count, but to LINEARLY INTERPOLATE within the final")
		t.Logf("  surviving interval (cMid-weighted midpoint instead of the")
		t.Logf("  arithmetic midpoint) — see d4 plan §6 candidate FIX-2.")
	})

	// ===== S8: Float-oracle upstream + fixed-point VQ projection =====
	t.Run("S8_Frame5_FloatUpstreamProjection", func(t *testing.T) {
		// Drive frames 0..5: for each frame, compute float-oracle a[]
		// and float-oracle LSP q[] from the same oldSpeech captured
		// above, quantize q[] to Q15 int16, run LSPToLSF + Quantize
		// using PRODUCTION fixed-point VQ. Compare frame-5 indices
		// against want.
		const wantFrame = 5
		l0w, l1w, l2w, l3w := extractLSPFieldsD3(
			bitData[wantFrame*bytesPerBitFrame : (wantFrame+1)*bytesPerBitFrame])
		t.Logf("=== Frame 5 float-upstream → fixed-VQ projection ===")
		t.Logf("WANT indices: L0=%d L1=%d L2=%d L3=%d",
			l0w, l1w, l2w, l3w)

		var freqPrev [4][10]int16
		InitFreqPrev(&freqPrev)

		var prevIndices Indices
		var prevQQ15 [10]int16
		for f := 0; f <= wantFrame; f++ {
			// Float-oracle LP from snaps[f].oldSpeech (already HPF'd
			// + windowed in the sliding window above).
			var aFloat [11]float64
			var rRaw, rLag, kFlt, ePred [11]float64
			oracleLPAnalysisD3(&snaps[f].oldSpeech, &aFloat,
				&rRaw, &rLag, &kFlt, &ePred)

			// Float-oracle Chebyshev: 10 cosine roots of F1·F2 in
			// real arithmetic, interleaved per §3.2.3 convention.
			var qFloatReal [10]float64
			oracleLSPRootsFloat(&aFloat, &qFloatReal)

			// Quantize q to Q15.
			var qQ15 [10]int16
			for i := 0; i < 10; i++ {
				v := math.Round(qFloatReal[i] * 32768.0)
				if v > 32767 {
					v = 32767
				} else if v < -32768 {
					v = -32768
				}
				qQ15[i] = int16(v)
			}

			// Run production LSPToLSF + Quantize on the float-derived
			// q (i.e. ALL upstream is float-oracle; everything
			// downstream is production fixed-point).
			var omega [10]int16
			LSPToLSF(&qQ15, &omega)
			idx := Quantize(&omega, &freqPrev)
			prevIndices = idx
			prevQQ15 = qQ15
			t.Logf("frame %d: float a[]Q12≈%v  q[]Q15=%v  → indices L0=%d L1=%d L2=%d L3=%d",
				f, aFloatToQ12(&aFloat), qQ15,
				idx.L0, idx.L1, idx.L2, idx.L3)
		}

		match := prevIndices.L0 == l0w && prevIndices.L1 == l1w &&
			prevIndices.L2 == l2w && prevIndices.L3 == l3w
		t.Logf("frame %d FINAL: produced=(L0=%d L1=%d L2=%d L3=%d) want=(L0=%d L1=%d L2=%d L3=%d) → MATCH=%v",
			wantFrame, prevIndices.L0, prevIndices.L1,
			prevIndices.L2, prevIndices.L3, l0w, l1w, l2w, l3w, match)
		_ = prevQQ15
		switch {
		case match:
			t.Logf("CRITICAL VERDICT: with FLOAT upstream (Levinson + Chebyshev")
			t.Logf("  in real arithmetic) the FIXED-POINT VQ converges to the")
			t.Logf("  WANT indices for frame 5. → bug is ENTIRELY UPSTREAM of")
			t.Logf("  the VQ (in lpc.Analyzer + lsp.LPToLSP). H-L is upgraded")
			t.Logf("  to FULLY CONFIRMED; H-L1 (Levinson) and H-L2 (Chebyshev)")
			t.Logf("  remain the only live root causes. No VQ-side regressions.")
		case prevIndices.L1 == l1w:
			t.Logf("CRITICAL VERDICT: float upstream recovers L1 (the")
			t.Logf("  unweighted-MSE first stage) but at least one of L0/L2/L3")
			t.Logf("  diverges. The L1 match implies upstream divergence is")
			t.Logf("  the dominant contributor; residual L2/L3 misses are")
			t.Logf("  either VQ-side (small, REOPEN-VQ-RESIDUAL needed) or")
			t.Logf("  predictor-memory drift carried in from frames 0..4.")
		default:
			t.Logf("CRITICAL VERDICT: float upstream STILL misses L1 (the")
			t.Logf("  unweighted first stage). There is a residual VQ-side or")
			t.Logf("  weights/predictor bug independent of LP analysis.")
			t.Logf("  → REOPEN-VQ branch: return to LSP encoding chain.")
		}
	})
}

// ---------------------------------------------------------------------
// Mirrored production internals (transcribed from spec; bit-exactness
// asserted vs production via S0 above).
// ---------------------------------------------------------------------

// d4MirrorWindowQ15 is the §3.2.1 eq. 3 30 ms asymmetric LP analysis
// window in Q15, computed from the closed form (matches production's
// internal/lpc/window.go lpAnalysisWindow LUT to within Q15 round-off).
var d4MirrorWindowQ15 = func() [240]int16 {
	var w [240]int16
	twoPi := 2.0 * math.Pi
	for n := 0; n < 200; n++ {
		v := 0.54 - 0.46*math.Cos(twoPi*float64(n)/399.0)
		w[n] = roundQ15(v)
	}
	for n := 200; n < 240; n++ {
		v := math.Cos(twoPi * float64(n-200) / 159.0)
		w[n] = roundQ15(v)
	}
	return w
}()

func roundQ15(v float64) int16 {
	r := math.Round(v * 32768.0)
	if r > 32767 {
		return 32767
	}
	if r < -32768 {
		return -32768
	}
	return int16(r)
}

// d4MirrorLagWindowQ15 — §3.2.1 eq. 6 60 Hz BW expansion table
// (matches production internal/lpc/lagwindow.go lagWindow).
var d4MirrorLagWindowQ15 = [10]int16{
	32732, 32623, 32442, 32191, 31871,
	31484, 31033, 30520, 29950, 29324,
}

// mirrorWindowSpeech mirrors internal/lpc/window.go windowSpeech via
// fixed.Mult equivalent (Q0·Q15 → Q0).
func mirrorWindowSpeech(speech *[240]int16, windowed *[240]int16) {
	for n := 0; n < 240; n++ {
		// fixed.Mult: prod>>15. We never hit Min16/Min16 here.
		windowed[n] = int16((int32(speech[n]) * int32(d4MirrorWindowQ15[n])) >> 15)
	}
}

// mirrorAutocorrelate mirrors internal/lpc/autocorr.go autocorrelate
// with an OPTIONAL forced minimum scale (forceScale ≥ chosen scale →
// shift up by extra bits). forceScale==0 reproduces production
// behaviour exactly.
func mirrorAutocorrelate(windowed *[240]int16, r *[11]int32, forceScale int) (scale int) {
	const maxWord32 = int64(1<<31 - 1)

	var sumSq int64
	for n := 0; n < 240; n++ {
		v := int64(windowed[n])
		sumSq += v * v
	}
	for sumSq > maxWord32 {
		sumSq >>= 2
		scale++
	}
	if forceScale > scale {
		scale = forceScale
	}
	if scale == 0 {
		for k := 0; k <= 10; k++ {
			var acc int32
			for n := k; n < 240; n++ {
				acc += int32(windowed[n]) * int32(windowed[n-k])
			}
			r[k] = acc
		}
		return 0
	}
	var s [240]int32
	for n := 0; n < 240; n++ {
		s[n] = int32(windowed[n]) >> uint(scale)
	}
	for k := 0; k <= 10; k++ {
		var acc int32
		for n := k; n < 240; n++ {
			acc += s[n] * s[n-k]
		}
		r[k] = acc
	}
	return scale
}

// mirrorApplyLagWindow mirrors internal/lpc/lagwindow.go applyLagWindow
// (eq. 7 white-noise correction + eq. 6 60 Hz BW expansion).
func mirrorApplyLagWindow(r *[11]int32) {
	// eq. 7: r(0) ← r(0) + r(0)>>13 with saturating add.
	add := int64(r[0]) + int64(r[0]>>13)
	switch {
	case add > math.MaxInt32:
		r[0] = math.MaxInt32
	case add < math.MinInt32:
		r[0] = math.MinInt32
	default:
		r[0] = int32(add)
	}
	for k := 1; k <= 10; k++ {
		r[k] = int32((int64(r[k]) * int64(d4MirrorLagWindowQ15[k-1])) >> 15)
	}
}

// levinsonTrace stores per-iteration measurements emitted by
// mirrorLevinsonTraced (S5 instrumentation).
type levinsonTrace struct {
	iter [11]struct {
		sum        int64
		qDivResult int64
		kQ15       int32
		eAfter     int64
		aWork      [11]int32
	}
}

// mirrorLevinsonTraced is byte-for-byte the production levinsonDurbin
// body (transcribed from internal/lpc/levinson.go) with per-iteration
// snapshots written into trace. Q-format identical to production.
func mirrorLevinsonTraced(r *[11]int32, a *[11]int16, trace *levinsonTrace) {
	const order = 10
	const q12one = int32(4096)

	var aWork [order + 1]int32
	var aPrev [order + 1]int32
	aWork[0] = q12one

	e := int64(r[0])

	for i := 1; i <= order; i++ {
		var sum int64
		sum = int64(aWork[0]) * int64(r[i])
		for j := 1; j < i; j++ {
			sum += int64(aWork[j]) * int64(r[i-j])
		}

		var kQ15 int32
		var qDiv int64
		if e > 0 {
			num := -(sum << 3)
			q := num / e
			qDiv = q
			switch {
			case q > math.MaxInt16:
				kQ15 = math.MaxInt16
			case q < math.MinInt16:
				kQ15 = math.MinInt16
			default:
				kQ15 = int32(q)
			}
		}

		copy(aPrev[:i], aWork[:i])
		for j := 1; j < i; j++ {
			upd := int64(aPrev[j]) + ((int64(kQ15) * int64(aPrev[i-j])) >> 15)
			aWork[j] = saturateInt32D4(upd)
		}
		aWork[i] = kQ15 >> 3

		kSq := int64(kQ15) * int64(kQ15)
		if kSq > (int64(1) << 30) {
			kSq = int64(1) << 30
		}
		oneMinusKSq := (int64(1) << 30) - kSq
		e = (e * oneMinusKSq) >> 30

		trace.iter[i].sum = sum
		trace.iter[i].qDivResult = qDiv
		trace.iter[i].kQ15 = kQ15
		trace.iter[i].eAfter = e
		trace.iter[i].aWork = aWork
	}

	a[0] = 4096
	for j := 1; j <= order; j++ {
		a[j] = saturateInt16D4(aWork[j])
	}
}

func saturateInt32D4(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

func saturateInt16D4(v int32) int16 {
	if v > math.MaxInt16 {
		return math.MaxInt16
	}
	if v < math.MinInt16 {
		return math.MinInt16
	}
	return int16(v)
}

// runMirrorPipeline composes mirrorWindowSpeech → mirrorAutocorrelate
// → mirrorApplyLagWindow → mirrorLevinsonTraced (discarding the
// trace). With forceExtraShift==0 it MUST match production a[]
// bit-exactly (asserted by S0).
func runMirrorPipeline(speech *[240]int16, a *[11]int16, forceExtraShift int) {
	var windowed [240]int16
	mirrorWindowSpeech(speech, &windowed)
	var r [11]int32
	scale := mirrorAutocorrelate(&windowed, &r, 0)
	if forceExtraShift > 0 {
		// Re-run with scale + extra to simulate AC headroom sweep.
		mirrorAutocorrelate(&windowed, &r, scale+forceExtraShift)
	}
	mirrorApplyLagWindow(&r)
	var trace levinsonTrace
	mirrorLevinsonTraced(&r, a, &trace)
}

// mirrorFindLSPRootsNBisect runs production-equivalent findLSPRoots
// over the production grid60, but parameterizes the bisection
// iteration count (production = 4; S7 sweeps to 6 and 8).
func mirrorFindLSPRootsNBisect(a *[11]int16, q *[10]int16, nBisect int) error {
	var f1, f2 [6]int32
	computeF1F2(a, &f1, &f2)

	var rootsF1, rootsF2 [5]int16
	var nF1, nF2 int

	xPrev := grid60[0]
	cPrev1 := chebyshevC(xPrev, &f1)
	cPrev2 := chebyshevC(xPrev, &f2)

	for k := 1; k < 60; k++ {
		x := grid60[k]
		c1 := chebyshevC(x, &f1)
		c2 := chebyshevC(x, &f2)

		if nF1 < 5 && (cPrev1 < 0) != (c1 < 0) {
			rootsF1[nF1] = mirrorBisectRoot(xPrev, x, cPrev1, c1, &f1, nBisect)
			nF1++
		}
		if nF2 < 5 && (cPrev2 < 0) != (c2 < 0) {
			rootsF2[nF2] = mirrorBisectRoot(xPrev, x, cPrev2, c2, &f2, nBisect)
			nF2++
		}
		xPrev = x
		cPrev1 = c1
		cPrev2 = c2
	}
	if nF1 < 5 || nF2 < 5 {
		return ErrLPCNonStable
	}
	for i := 0; i < 5; i++ {
		q[2*i] = rootsF1[i]
		q[2*i+1] = rootsF2[i]
	}
	return nil
}

func mirrorBisectRoot(xLo, xHi int16, cLo, cHi int32, f *[6]int32, n int) int16 {
	for i := 0; i < n; i++ {
		mid := int16((int32(xLo) + int32(xHi)) >> 1)
		cMid := chebyshevC(mid, f)
		if (cLo < 0) != (cMid < 0) {
			xHi = mid
			cHi = cMid
		} else {
			xLo = mid
			cLo = cMid
		}
	}
	_ = cHi
	return int16((int32(xLo) + int32(xHi)) >> 1)
}

// ---------------------------------------------------------------------
// Float-oracle Chebyshev for S8 (clean-room §3.2.3 transcription).
// ---------------------------------------------------------------------

// oracleLSPRootsFloat finds the 10 LSP cosines q[0..9] of the order-10
// LP filter aReal[0..10] in real arithmetic per §3.2.3 eq. 9–17.
// Roots are interleaved F1/F2 in strictly increasing-ω order to
// match production's findLSPRoots convention.
func oracleLSPRootsFloat(aReal *[11]float64, q *[10]float64) {
	// f1, f2 from eq. 15.
	var f1, f2 [6]float64
	f1[0] = 1.0
	f2[0] = 1.0
	for i := 0; i < 5; i++ {
		f1[i+1] = aReal[i+1] + aReal[10-i] - f1[i]
		f2[i+1] = aReal[i+1] - aReal[10-i] + f2[i]
	}
	// C(x) per eq. 17 via the back-recursion of §3.2.3 lines 794–799.
	cheb := func(x float64, f *[6]float64) float64 {
		bk1 := 1.0 // b[5]
		bk2 := 0.0 // b[6]
		for k := 4; k >= 1; k-- {
			bk := 2.0*x*bk1 - bk2 + f[5-k]
			bk2 = bk1
			bk1 = bk
		}
		return x*bk1 - bk2 + f[5]/2.0
	}
	// Locate 5 roots for each polynomial via dense scan + bisection
	// to ~1e-9 precision.
	findRoots := func(f *[6]float64) [5]float64 {
		const N = 4096
		var prevX, prevC float64
		prevX = 1.0
		prevC = cheb(prevX, f)
		var roots [5]float64
		var n int
		for i := 1; i <= N; i++ {
			x := 1.0 - 2.0*float64(i)/float64(N) // sweeps +1 → −1 (ω: 0 → π)
			c := cheb(x, f)
			if (prevC < 0) != (c < 0) && n < 5 {
				lo, hi := prevX, x
				cLo, cHi := prevC, c
				for it := 0; it < 60; it++ {
					mid := 0.5 * (lo + hi)
					cMid := cheb(mid, f)
					if (cLo < 0) != (cMid < 0) {
						hi = mid
						cHi = cMid
					} else {
						lo = mid
						cLo = cMid
					}
				}
				_ = cHi
				roots[n] = 0.5 * (lo + hi)
				n++
			}
			prevX = x
			prevC = c
		}
		return roots
	}
	r1 := findRoots(&f1) // already in decreasing-x = increasing-ω order
	r2 := findRoots(&f2)
	for i := 0; i < 5; i++ {
		q[2*i] = r1[i]
		q[2*i+1] = r2[i]
	}
}

// aOracleAt rebuilds the float oracle's a^{(i)}_j without re-running
// the full Levinson — it re-derives via stored k[]. Used only for
// per-iteration "expected" magnitudes in S5's table.
func aOracleAt(j, i int, k *[11]float64) float64 {
	// Levinson recursion: a^{(0)} = [1]; a^{(m)}_m = k_m;
	// a^{(m)}_j = a^{(m-1)}_j + k_m * a^{(m-1)}_{m-j} for 1≤j<m.
	// Build a[0..10] up to order i.
	var a [11]float64
	a[0] = 1.0
	var aPrev [11]float64
	for m := 1; m <= i; m++ {
		copy(aPrev[:m], a[:m])
		for jj := 1; jj < m; jj++ {
			a[jj] = aPrev[jj] + k[m]*aPrev[m-jj]
		}
		a[m] = k[m]
	}
	if j < 0 || j > 10 {
		return 0
	}
	return a[j]
}

// aFloatToQ12 rounds float a[0..10] into Q12 int16 for logging
// alongside the float-derived q[].
func aFloatToQ12(a *[11]float64) [11]int16 {
	var out [11]int16
	for i := 0; i <= 10; i++ {
		v := math.Round(a[i] * 4096.0)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[i] = int16(v)
	}
	return out
}
