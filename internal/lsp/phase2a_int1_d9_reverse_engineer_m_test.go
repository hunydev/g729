package lsp

import (
	"math"
	"testing"

	"github.com/exedev/g729/internal/tables"
)

// TestINT1D9ReverseEngineerM — d9 §S1 reverse-engineering of the
// ITU cold-start MA-predictor memory M_{k,i} from the LSP.BIT
// frame-0 ground-truth indices (L0=0, L1=120, L2=10, L3=10).
//
// I6 BINDING: read-only on production; t.Logf only.
//
// Background (from d8 §S6):
//   - Encoder ω after FIX-2D is within 7 Q13 LSB of analytical
//     i·π/11 for all-zero PCM (LP-analysis on zero buffer is forced
//     to a[]=[1,0,...,0] → q=cos(i·π/11) → ω=i·π/11).
//   - Decoder roundtrip on WANT indices, using current cold-start
//     memory M = initialPastResidual = i·π/11 Q13, produces ω̂_WANT
//     = [2415, 4765, 6875, 9512, 11713, 14089, 16412, 18483, 20849,
//     23487] Q13. |Δ| vs analytical: max=234 LSB, avg=109 LSB.
//
// Hypothesis H-M: ITU's cold-start past-residual memory M differs
// from i·π/11. If we assume the encoder's frame-0 ω_target IS
// analytical (forced by zero PCM), then ITU's WANT must reconstruct
// to ≈ ω_target via the decoder predictor. Solving for an "implied"
// per-coordinate predictor sum S_i = Σ_k p_{0,k,i} · M_{k,i}
// yields the M ITU is using.
//
// Math (decoder eq. 20, sel=0):
//
//	ω̂_i = comp_i · r̂_i + S_i,    comp_i = 1 − Σ_k p_{0,k,i}    (Q15)
//
// Where r̂_i is the post-rearrange residual for WANT.
//
// d8's omegaWant uses M = analytical, so:
//
//	omegaWant_i = comp_i · r̂_i + sumP_i · analytical_i (uniform M)
//	⇒ comp_i · r̂_i = omegaWant_i − S_analytical_i
//
// For ITU choosing WANT optimally with target ω = analytical:
//
//	analytical_i ≈ comp_i · r̂_i + S_i^ITU
//	⇒ S_i^ITU = analytical_i − comp_i · r̂_i
//	          = analytical_i − omegaWant_i + S_analytical_i
//
// Under the simplifying assumption that all 4 lags share the same
// scalar M_i (a common cold-start convention), M_i^ITU = S_i^ITU /
// sumP_i (Q13). Diagnostic-only; the true M may be lag-dependent.
func TestINT1D9ReverseEngineerM(t *testing.T) {
	// WANT indices for LSP.BIT frame 0.
	want := Indices{L0: 0, L1: 120, L2: 10, L3: 10}

	// Reproduce d8: residual post-rearrange.
	var residual [10]int16
	combineResidual(want.L1, want.L2, want.L3, &residual)
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)

	// d8 omegaWant under current cold-start (M = analytical).
	var fp [4][10]int16
	InitFreqPrev(&fp)
	var omegaWant [10]int16
	applyPredictorWithMemory(want.L0, &fp, &residual, &omegaWant)

	preds := &tables.MAPredictorsLSP[want.L0]

	// Per-coord sumP_i = Σ_k p_{0,k,i} (Q15) and comp_i = 32768−sumP.
	var sumP [10]int32
	var compQ15 [10]int32
	for i := 0; i < 10; i++ {
		s := int32(0)
		for k := 0; k < 4; k++ {
			s += int32(preds[k][i])
		}
		sumP[i] = s
		compQ15[i] = 32768 - s
	}

	// S_analytical_i: predictor contribution if memory is analytical
	// across all 4 lags. Compute exactly as MAC sum then ÷ 32768.
	// Using uniform M_i: S_analytical_i ≈ sumP_i · analytical_i / 32768
	// (rounded). For diagnostic precision, use float64 then round.
	t.Logf("=== d9: reverse-engineering ITU cold-start M ===")
	t.Logf("WANT (L0=%d, L1=%d, L2=%d, L3=%d)", want.L0, want.L1, want.L2, want.L3)
	t.Logf("residual r̂ post-rearrange Q13: %v", residual)
	t.Logf("ω_decoder(WANT, M=analytical) Q13: %v", omegaWant)
	t.Logf("ω_analytical (i·π/11) Q13:        %v", initialPastResidual)
	t.Logf("sumP_i (Q15): %v", sumP)
	t.Logf("comp_i (Q15): %v", compQ15)

	// ω_target candidate(s). Primary: analytical (encoder's actual
	// frame-0 ω with zero PCM is analytical to within ≤7 LSB).
	type tgt struct {
		name string
		w    [10]int16
	}
	candidates := []tgt{{"analytical", initialPastResidual}}

	for _, c := range candidates {
		t.Logf("--- ω_target = %s ---", c.name)

		// S_i^ITU = ω_target_i − comp_i · r̂_i (in Q13)
		// comp_i · r̂_i is Q15·Q13 = Q28 → ÷32768 → Q13.
		var S_ITU [10]int32
		var compR [10]int32
		for i := 0; i < 10; i++ {
			cr := (compQ15[i]*int32(residual[i]) + 16384) >> 15
			compR[i] = cr
			S_ITU[i] = int32(c.w[i]) - cr
		}
		t.Logf("comp·r̂ (Q13):          %v", compR)
		t.Logf("S_i^ITU = ω−comp·r̂:    %v", S_ITU)

		// Cross-check S_analytical via direct MAC formula.
		var S_analytical [10]int32
		for i := 0; i < 10; i++ {
			s := int64(0)
			for k := 0; k < 4; k++ {
				s += int64(preds[k][i]) * int64(initialPastResidual[i])
			}
			// Q15·Q13 = Q28 → round shift to Q13 (>>15).
			S_analytical[i] = int32((s + (1 << 14)) >> 15)
		}
		t.Logf("S_i^analytical (cross): %v", S_analytical)

		// Sanity: omegaWant_i ≈ comp·r̂_i + S_analytical_i (Q13).
		var checkOW [10]int32
		for i := 0; i < 10; i++ {
			checkOW[i] = compR[i] + S_analytical[i]
		}
		t.Logf("check ω̂ recompute:      %v  (should match omegaWant)", checkOW)

		// Implied uniform M_i^ITU = S_i^ITU / (sumP_i / 32768) Q13.
		var M_uniform [10]int32
		for i := 0; i < 10; i++ {
			M_uniform[i] = int32(math.Round(float64(S_ITU[i]) * 32768.0 / float64(sumP[i])))
		}
		t.Logf("Implied uniform M_i^ITU (Q13, all 4 lags equal): %v", M_uniform)

		// Δ vs analytical and vs zero, percent of analytical.
		var dAnalyt [10]int32
		var pct [10]float64
		for i := 0; i < 10; i++ {
			dAnalyt[i] = M_uniform[i] - int32(initialPastResidual[i])
			pct[i] = 100.0 * float64(dAnalyt[i]) / float64(initialPastResidual[i])
		}
		t.Logf("M_uniform − analytical (Q13 LSB): %v", dAnalyt)
		t.Logf("M_uniform / analytical (%%):       %.2f", pct)

		// Compare to candidate cold-start patterns.
		// (a) Zero memory.
		zeroMatch := true
		for i := 0; i < 10; i++ {
			if absI32(M_uniform[i]) > 200 {
				zeroMatch = false
				break
			}
		}
		// (b) Analytical (current production). Accept if max |Δ| ≤ 12 LSB.
		analytMaxAbs := int32(0)
		for i := 0; i < 10; i++ {
			if a := absI32(dAnalyt[i]); a > analytMaxAbs {
				analytMaxAbs = a
			}
		}
		// (c) initialPrevLSP-derived: q_i = cos(i·π/11) Q15 → if
		//     misinterpreted as Q13 it'd be way different scale, but
		//     also could be ω̂ from a centroid LSP. Just log L1[120].
		// (d) L1[120] alone (codebook centroid).
		l1_120 := tables.LSPCodebookL1[120]

		t.Logf("Disposition checks:")
		t.Logf("  zero memory     ? %v   (max |M|=%d, threshold 200)", zeroMatch, maxAbsI32(M_uniform))
		t.Logf("  analytical (current production)? max |Δ|=%d (≤12 = match)", analytMaxAbs)
		t.Logf("  L1[120] for reference: %v", l1_120)

		// Per-coord scalar: how does M_uniform compare to analytical
		// times a scalar β? Estimate β = mean(M/analytical).
		var beta float64
		for i := 0; i < 10; i++ {
			beta += float64(M_uniform[i]) / float64(initialPastResidual[i])
		}
		beta /= 10
		var residRMS float64
		for i := 0; i < 10; i++ {
			pred := beta * float64(initialPastResidual[i])
			d := float64(M_uniform[i]) - pred
			residRMS += d * d
		}
		residRMS = math.Sqrt(residRMS / 10)
		t.Logf("  best-fit scalar β = %.4f, residual RMS = %.1f Q13 LSB", beta, residRMS)
	}

	// Conclusion logging.
	t.Logf("")
	t.Logf("=== d9 conclusion (computed above) ===")
	t.Logf("If M_uniform ≈ analytical with max |Δ| ≤ ~20 LSB → H-M REFUTED")
	t.Logf("If M_uniform ≈ 0 → H-M CONFIRMED (zero cold-start)")
	t.Logf("If M_uniform ≈ β·analytical with β ≠ 1 and small RMS → scaled-analytical")
	t.Logf("Else → AMBIGUOUS / lag-dependent (uniform-M assumption fails); pivot to H-N")
}

func absI32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func maxAbsI32(a [10]int32) int32 {
	m := int32(0)
	for _, v := range a {
		if x := absI32(v); x > m {
			m = x
		}
	}
	return m
}
