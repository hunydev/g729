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

// TestINT1D5AWorkValidation — Phase 2a-INT-1-d5 (refined hypothesis
// H-L1′: aWork Q12 precision loss drives the Levinson cascade).
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-04-phase2a-int1-d4-pinpoint-plan.md
//	(§11 records the FIX-1A FAILED-REVERT and refined H-L1′; §12 / §13
//	will be appended by this dispatch with the validation outcome.)
//
// d4-FIX-1A revert demonstrated that Norm_l renormalization of `e`
// is a mathematical no-op on frame 29: both
// (num << eShift) / e_renorm = -53229 and num / e = -53261 saturate
// to MinInt16. The true root cause must lie UPSTREAM of the divide,
// in the per-iteration aWork inner-update precision (Q12 truncation
// of `(kQ15 · aPrev[i-j]) >> 15`).
//
// d5 is MEASUREMENT-ONLY (I6 freeze BINDING; I5 budget remains 1/5
// consumed, no further consumption in this dispatch). It runs three
// Levinson variants side-by-side on the same r'[0..10]:
//
//   - production-mirrored fixed-point (aWork int32 Q12)
//   - float64 oracle (real arithmetic, stdlib math only)
//   - wide-precision mirror (aWork int64 Q24; final quantization to
//     Q12 only at write-out)
//
// The wide variant is the proposed FIX-1B candidate from §11.5.
// If wide-aWork tracks the float oracle within ≤ 1 Q12 LSB through
// all 10 iterations on frame 29, hypothesis H-L1′ is CONFIRMED and
// the next dispatch can implement the wide-precision aWork in
// internal/lpc/levinson.go.
//
// ABSOLUTE CONSTRAINTS (parent plan §0 + d4 §0):
//   - Clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 source.
//     Spec source = G729E.{pdf,txt} only. Mirror routines are
//     transcribed from spec (and asserted bit-exact vs production
//     in d4 S0).
//   - I6 binding: zero production-file changes.
//   - I5 budget: NOT consumed by this dispatch (measurement only).
func TestINT1D5AWorkValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("d5 aWork-precision validation battery; -short")
	}

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

	// Replay HPF + sliding window so we have the same `oldSpeech[]`
	// production sees on entry to LP analysis for every captured frame.
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

	// ===== Step 1 + Step 2: side-by-side per-iteration aWork trace =====
	t.Run("S1_Frame29_AWorkSideBySide", func(t *testing.T) {
		s := snaps[29]

		// Recover r'[0..10] (post lag-window) the way production does.
		var windowed [240]int16
		mirrorWindowSpeech(&s.oldSpeech, &windowed)
		var r [11]int32
		scale := mirrorAutocorrelate(&windowed, &r, 0)
		mirrorApplyLagWindow(&r)

		t.Logf("=== Frame 29: production vs float-oracle vs wide-Q24 aWork ===")
		t.Logf("AC scale = %d (each r[k] divided by 4^scale vs raw)", scale)
		t.Logf("r'[0..10] (post lag-window) = %v", r)

		// Production-mirror trace (Q12 int32 aWork, identical to
		// internal/lpc/levinson.go).
		var traceProd levinsonTrace
		var aFix [11]int16
		mirrorLevinsonTraced(&r, &aFix, &traceProd)

		// Float64 oracle on the same r'[].
		var oracle floatLevinsonTrace
		floatLevinsonOnR(&r, &oracle)

		// Wide-Q24 mirror on the same r'[].
		var traceWide wideLevinsonTrace
		var aWide [11]int16
		wideLevinsonTraced(&r, &aWide, &traceWide)

		t.Logf("--- per-iteration table (i, e, k, aWork[1..i]) ---")
		var firstProdDiverge, firstWideDiverge int = -1, -1
		const oneQ12 float64 = 4096.0
		for i := 1; i <= 10; i++ {
			itP := traceProd.iter[i]
			itW := traceWide.iter[i]
			t.Logf("  i=%2d", i)
			t.Logf("     prod : e=%-14d k_Q15=%-7d num=%d", itP.eAfter, itP.kQ15, itP.qDivResult)
			t.Logf("     float: E=%-+.6e k=%+.7f", oracle.E[i], oracle.k[i])
			t.Logf("     wide : e=%-14d k_Q15=%-7d num=%d", itW.eAfter, itW.kQ15, itW.numAtDivide)

			// aWork[1..i] for all three (display in real units +
			// integer/Q12 form for prod and wide).
			t.Logf("     prod  aWork[1..%d] (Q12 int32) = %v", i, itP.aWork[1:i+1])
			var floatA [11]float64
			floatAWorkAt(i, &oracle.k, &floatA)
			t.Logf("     float aWork[1..%d] (real)     = %v", i, floatA[1:i+1])
			var wideQ12 [11]int32
			for j := 1; j <= i; j++ {
				wideQ12[j] = int32(itW.aWork[j] >> 12)
			}
			t.Logf("     wide  aWork[1..%d] (Q24>>12)  = %v", i, wideQ12[1:i+1])

			// Per-element divergence in Q12 LSBs.
			var maxProdErr, maxWideErr int32
			for j := 1; j <= i; j++ {
				expectedQ12 := int32(math.Round(floatA[j] * oneQ12))
				dProd := abs32(itP.aWork[j] - expectedQ12)
				dWide := abs32(wideQ12[j] - expectedQ12)
				if dProd > maxProdErr {
					maxProdErr = dProd
				}
				if dWide > maxWideErr {
					maxWideErr = dWide
				}
			}
			t.Logf("     |Δ vs float| prod max = %d Q12 LSB ; wide max = %d Q12 LSB",
				maxProdErr, maxWideErr)
			if firstProdDiverge < 0 && maxProdErr >= 1 {
				firstProdDiverge = i
			}
			if firstWideDiverge < 0 && maxWideErr >= 1 {
				firstWideDiverge = i
			}
		}

		t.Logf("=== Step 2: divergence onset ===")
		if firstProdDiverge >= 0 {
			t.Logf("aWork_prod first diverges from float oracle by ≥1 Q12 at i=%d", firstProdDiverge)
		} else {
			t.Logf("aWork_prod tracks float oracle within <1 Q12 through i=10 (no divergence)")
		}
		if firstWideDiverge >= 0 {
			t.Logf("aWork_wide (Q24→Q12) first diverges from float oracle by ≥1 Q12 at i=%d", firstWideDiverge)
		} else {
			t.Logf("aWork_wide (Q24→Q12) tracks float oracle within <1 Q12 through i=10 (no divergence)")
		}

		// ===== Step 3: wide → LSP root sanity =====
		t.Logf("=== Step 3: wide Levinson final a[] + LSP root sanity ===")
		t.Logf("wide a[0..10] Q12 (rounded) = %v", aWide)
		t.Logf("prod a[0..10] Q12           = %v", aFix)
		var qWide [10]int16
		errLSP := LPToLSP(&aWide, &qWide)
		if errLSP != nil {
			t.Logf("LPToLSP(wide a) returned ErrLPCNonStable — wide fix does NOT clear frame 29 LP-instability")
		} else {
			t.Logf("LPToLSP(wide a) succeeded: q[0..9] Q15 = %v", qWide)
			// Distinctness + ω gap sanity.
			var omega [10]float64
			for i := 0; i < 10; i++ {
				x := float64(qWide[i]) / 32768.0
				if x > 1.0 {
					x = 1.0
				} else if x < -1.0 {
					x = -1.0
				}
				omega[i] = math.Acos(x)
			}
			minGap := math.Inf(1)
			for i := 1; i < 10; i++ {
				g := omega[i] - omega[i-1]
				if g < minGap {
					minGap = g
				}
			}
			distinct := true
			for i := 0; i < 10; i++ {
				for j := i + 1; j < 10; j++ {
					if qWide[i] == qWide[j] {
						distinct = false
					}
				}
			}
			inRange := true
			for i := 0; i < 10; i++ {
				if qWide[i] >= 32767 || qWide[i] <= -32768 {
					inRange = false
				}
			}
			t.Logf("ω (rad) = %v", omega)
			t.Logf("min gap = %.6f rad (need ≥ ~0.01); distinct=%v ; in-range=%v",
				minGap, distinct, inRange)
			if distinct && inRange && minGap >= 0.01 {
				t.Logf("→ frame-29 LP-instability fatal would be CLEARED by wide-aWork fix")
			} else {
				t.Logf("→ wide-aWork passes Cheb sign-change scan but root-quality is marginal")
			}
		}
	})

	// ===== Step 4: Frame 5 wide-aWork → fixed-point VQ projection =====
	t.Run("S2_Frame5_WidePipelineProjection", func(t *testing.T) {
		const wantFrame = 5
		l0w, l1w, l2w, l3w := extractLSPFieldsD3(
			bitData[wantFrame*bytesPerBitFrame : (wantFrame+1)*bytesPerBitFrame])
		t.Logf("=== Frame 5 wide-aWork → fixed-point downstream VQ projection ===")
		t.Logf("WANT indices: L0=%d L1=%d L2=%d L3=%d", l0w, l1w, l2w, l3w)

		var freqPrev [4][10]int16
		InitFreqPrev(&freqPrev)
		var prevIndices Indices
		for f := 0; f <= wantFrame; f++ {
			var windowed [240]int16
			mirrorWindowSpeech(&snaps[f].oldSpeech, &windowed)
			var r [11]int32
			mirrorAutocorrelate(&windowed, &r, 0)
			mirrorApplyLagWindow(&r)
			var aWide [11]int16
			var trace wideLevinsonTrace
			wideLevinsonTraced(&r, &aWide, &trace)

			// Production fixed-point Chebyshev + LSPToLSF + Quantize.
			var qQ15 [10]int16
			if err := LPToLSP(&aWide, &qQ15); err != nil {
				t.Logf("frame %d: LPToLSP(wide a) FAILED — %v", f, err)
				prevIndices = Indices{}
				continue
			}
			var omega [10]int16
			LSPToLSF(&qQ15, &omega)
			idx := Quantize(&omega, &freqPrev)
			prevIndices = idx
			t.Logf("frame %d: wide a[]Q12=%v  q[]Q15=%v → indices L0=%d L1=%d L2=%d L3=%d",
				f, aWide, qQ15, idx.L0, idx.L1, idx.L2, idx.L3)
		}
		match := prevIndices.L0 == l0w && prevIndices.L1 == l1w &&
			prevIndices.L2 == l2w && prevIndices.L3 == l3w
		t.Logf("frame %d FINAL: produced=(L0=%d L1=%d L2=%d L3=%d) want=(L0=%d L1=%d L2=%d L3=%d) → MATCH=%v",
			wantFrame, prevIndices.L0, prevIndices.L1,
			prevIndices.L2, prevIndices.L3, l0w, l1w, l2w, l3w, match)

		switch {
		case match:
			t.Logf("VERDICT: H-L1′ wide-aWork fix RECOVERS frame-5 (L0,L1,L2,L3) WANT.")
			t.Logf("  → FIX-1B is a complete repair for both frame 29 (LP stability) and frame 5 (VQ).")
		case prevIndices.L1 == l1w:
			t.Logf("VERDICT: wide-aWork recovers L1 (the unweighted MSE first stage)")
			t.Logf("  but at least one of L0/L2/L3 still misses → PARTIAL fix; residual")
			t.Logf("  bug is downstream of LP analysis (predictor / weights / VQ).")
		default:
			t.Logf("VERDICT: wide-aWork does not recover frame-5 indices.")
			t.Logf("  The L3 / L2 misses on frame 5 likely originate from a SEPARATE")
			t.Logf("  hypothesis (H-L4 cold-start drift, or H-L2 Chebyshev bias).")
			t.Logf("  Apply FIX-1B for the frame-29 robustness gain; open d6 for residual.")
		}
	})

	// ===== Step 5: Bit-budget analysis =====
	t.Run("S3_WideBitBudget", func(t *testing.T) {
		t.Logf("=== Step 5: wide-aWork bit-budget analysis ===")
		t.Logf("Spec §3.2.2 lines 717–736 specify the recursion in REAL")
		t.Logf("  arithmetic; internal Q-format choices are implementation-")
		t.Logf("  defined under §3.2.1 line 691's \"to avoid arithmetic")
		t.Logf("  problems\" license.")
		t.Logf("")
		t.Logf("Q24 in int32: |aWork_Q24| < 2^31 ⇒ |a_j_real| < 128.")
		t.Logf("  For spec-stable LP filters |a_j| typically < 16; on transient")
		t.Logf("  frames such as frame 29 the production (saturated) a[] hits")
		t.Logf("  ±|7407|/4096 ≈ 1.81. Float-oracle |a_j| at i=6 stays under 2.0.")
		t.Logf("  → Q24 in int32 is SAFE for spec-stable inputs but offers ≤ 6")
		t.Logf("    bits headroom on transients; an out-of-spec input could overflow.")
		t.Logf("")
		t.Logf("Sum width: each term aWork_Q24 (≤2^31) · r[i-j] (≤2^31) ≤ 2^62;")
		t.Logf("  Σ over 11 terms ≤ ~2^65.5 — DOES NOT FIT in int64 in the worst")
		t.Logf("  case. In practice r[] is normalized into ≈ 2^29 by the AC scale")
		t.Logf("  heuristic, so per-term ≤ 2^60 and 11-sum ≤ 2^63.5 — also tight.")
		t.Logf("  → MUST shift aWork right before multiply (e.g. >> 12 to recover")
		t.Logf("    the production Q12 product) or use math/big / two-int64 split.")
		t.Logf("")
		t.Logf("Q30 in int64: |aWork_Q30| < 2^63 ⇒ |a_j_real| < 2^33; comfortable")
		t.Logf("  margin. Sum: 2^63 · 2^31 = 2^94 — REQUIRES shifting before sum.")
		t.Logf("")
		t.Logf("Recommendation:")
		t.Logf("  - Carry aWork as int64 in Q24 (12 extra fractional bits).")
		t.Logf("  - For the sum: shift aWork down to Q12 (>> 12 with rounding)")
		t.Logf("    just before the multiply, preserving the production sum")
		t.Logf("    width exactly (sum stays int64 as today).")
		t.Logf("  - The precision win comes from the INNER UPDATE")
		t.Logf("      aWork[j] = aPrev[j] + (kQ15 · aPrev[i-j]) >> 15")
		t.Logf("    which is now Q24 + (Q15 · Q24) >> 15 = Q24 — keeping 12")
		t.Logf("    extra fractional bits across the recursion.")
		t.Logf("  - Final write-out: a[j] = saturateInt16((aWork[j] + (1<<11)) >> 12)")
		t.Logf("    with rounding.")
	})
}

// ---------------------------------------------------------------------
// Wide-precision Levinson mirror (aWork in int64 Q24).
//
// This is a clean-room transcription of the §3.2.2 recursion using
// 12 extra fractional bits on aWork. The outer skeleton is identical
// to mirrorLevinsonTraced; only aWork's storage format changes.
// Sum is computed in Q12 units (matching production) by shifting
// aWork down with rounding just before the multiply, so the sum
// width and the divide arithmetic are bit-identical to production.
// All extra precision is delivered at the inner-update step
//
//	aWork[j] = aPrev[j] + (kQ15 · aPrev[i-j]) >> 15
//
// where aPrev is now Q24, retaining 12 fractional bits that the
// production Q12 would have truncated.
// ---------------------------------------------------------------------

type wideLevinsonTrace struct {
	iter [11]struct {
		sum         int64 // Q12·rscale, identical scaling to production
		numAtDivide int64 // = -(sum << 3), Q15·rscale
		kQ15        int32
		eAfter      int64
		aWork       [11]int64 // Q24
	}
}

func wideLevinsonTraced(r *[11]int32, a *[11]int16, trace *wideLevinsonTrace) {
	const order = 10
	const oneQ24 = int64(1) << 24

	var aWork [order + 1]int64
	var aPrev [order + 1]int64
	aWork[0] = oneQ24

	e := int64(r[0])

	for i := 1; i <= order; i++ {
		// Sum at production Q12 width. Round-shift aWork (Q24) → Q12.
		var sum int64
		sum = q24ToQ12(aWork[0]) * int64(r[i])
		for j := 1; j < i; j++ {
			sum += q24ToQ12(aWork[j]) * int64(r[i-j])
		}

		var kQ15 int32
		var num int64
		if e > 0 {
			num = -(sum << 3)
			q := num / e
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
		// Inner update at full Q24 precision.
		// aPrev[j] is Q24; (kQ15 (Q15) · aPrev[i-j] (Q24)) >> 15 = Q24.
		for j := 1; j < i; j++ {
			aWork[j] = aPrev[j] + (int64(kQ15)*aPrev[i-j])>>15
		}
		// New aWork[i] = k_i in Q24 (kQ15 << 9).
		aWork[i] = int64(kQ15) << 9

		kSq := int64(kQ15) * int64(kQ15)
		if kSq > (int64(1) << 30) {
			kSq = int64(1) << 30
		}
		oneMinusKSq := (int64(1) << 30) - kSq
		e = (e * oneMinusKSq) >> 30

		trace.iter[i].sum = sum
		trace.iter[i].numAtDivide = num
		trace.iter[i].kQ15 = kQ15
		trace.iter[i].eAfter = e
		trace.iter[i].aWork = aWork
	}

	a[0] = 4096
	for j := 1; j <= order; j++ {
		// Round-shift Q24 → Q12 with banker-free arithmetic rounding.
		v := aWork[j]
		if v >= 0 {
			v = (v + (1 << 11)) >> 12
		} else {
			v = -((-v + (1 << 11)) >> 12)
		}
		switch {
		case v > math.MaxInt16:
			a[j] = math.MaxInt16
		case v < math.MinInt16:
			a[j] = math.MinInt16
		default:
			a[j] = int16(v)
		}
	}
}

// q24ToQ12 round-shifts a Q24 int64 down to Q12 int64 (signed
// half-away-from-zero rounding to avoid systematic bias toward
// either rail in the sum accumulator).
func q24ToQ12(v int64) int64 {
	if v >= 0 {
		return (v + (1 << 11)) >> 12
	}
	return -((-v + (1 << 11)) >> 12)
}

// ---------------------------------------------------------------------
// Float64 Levinson oracle that consumes the SAME r'[] as production
// (i.e. post-fixed-point autocorrelate + lag-window). This is the
// truthful "what the production quotient would be with infinite
// precision arithmetic" reference, isolating the aWork-precision
// hypothesis from any windowing / autocorrelate / lag-window drift.
// ---------------------------------------------------------------------

type floatLevinsonTrace struct {
	a [11][11]float64 // a[i][j] = a_j after iteration i
	k [11]float64
	E [11]float64
}

func floatLevinsonOnR(r *[11]int32, out *floatLevinsonTrace) {
	var rf [11]float64
	for i := 0; i <= 10; i++ {
		rf[i] = float64(r[i])
	}
	var a [11]float64
	var aPrev [11]float64
	a[0] = 1.0
	out.E[0] = rf[0]
	out.a[0][0] = 1.0
	for i := 1; i <= 10; i++ {
		var sum float64
		for j := 0; j < i; j++ {
			sum += a[j] * rf[i-j]
		}
		var ki float64
		if out.E[i-1] != 0 {
			ki = -sum / out.E[i-1]
		}
		out.k[i] = ki
		copy(aPrev[:i], a[:i])
		for j := 1; j < i; j++ {
			a[j] = aPrev[j] + ki*aPrev[i-j]
		}
		a[i] = ki
		out.E[i] = (1.0 - ki*ki) * out.E[i-1]
		for j := 0; j <= i; j++ {
			out.a[i][j] = a[j]
		}
	}
}

// floatAWorkAt fills outA[1..i] with the float-oracle a^{(i)}_j from
// the per-iteration table. Index outA[0] is set to 1.
func floatAWorkAt(i int, k *[11]float64, outA *[11]float64) {
	var a [11]float64
	var aPrev [11]float64
	a[0] = 1.0
	for m := 1; m <= i; m++ {
		copy(aPrev[:m], a[:m])
		for j := 1; j < m; j++ {
			a[j] = aPrev[j] + k[m]*aPrev[m-j]
		}
		a[m] = k[m]
	}
	*outA = a
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
