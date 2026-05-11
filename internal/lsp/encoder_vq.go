package lsp

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// QuantizeTopKMax is the largest first-stage candidate set accepted by
// QuantizeTopK. The bound keeps the expanded VQ diagnostic stack-only and
// predictable.
const QuantizeTopKMax = 16

// searchL1 returns the index L1 ∈ [0,128) of the row of
// LSPCodebookL1 that minimizes the unweighted Σ_{i=0..9} (target_i −
// row_i)² and the corresponding sum-of-squares cost in Word32.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 887–888 — "the entry L1
// that minimizes the (unweighted) mean-squared error" of the
// first-stage LSF quantizer is selected by exhaustive search of all
// 128 rows.
//
// Q-format: target and codebook rows are Q13; per-coefficient
// difference is Q13 fitting Word16; the square is Q26 fitting Word32;
// the 10-term sum stays in Word32 (max ≈ 10·(2¹³)² < 2³⁰).
func searchL1(target *[10]int16) (int, int32) {
	bestIdx := 0
	var bestMSE int32 = -1
	for row := 0; row < len(tables.LSPCodebookL1); row++ {
		var sum int32
		for i := 0; i < 10; i++ {
			diff := int32(target[i]) - int32(tables.LSPCodebookL1[row][i])
			sum += diff * diff
		}
		if bestMSE < 0 || sum < bestMSE {
			bestMSE = sum
			bestIdx = row
		}
	}
	return bestIdx, bestMSE
}

// searchL2 returns the index L2 ∈ [0, 32) of the row of
// LSPCodebookL2 that minimizes the partial weighted MSE
//
// //
// where ω̂[0..4] is the lower half of the LSF reconstructed from the
// candidate residual r[0..4] = L1[l1][0..4] + L2[row][0..4] via the
// MA predictor (selector ∈ {0,1}, predictor memory mem) followed by
// the J=0.0012 pair-rearrangement (lsfRearrJ1) restricted to the
// first five coefficients.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 889–892 — "Using the
// selected first stage vector L1, the entry L2 of the second stage
// lower 5-dimensional codebook is searched. … rearranged to
// guarantee a minimum distance of 0.0012" (line 890).
//
// Q-format: omega and weights are Q13 / Q11 respectively (matching
// computeTargetLSF / weightsLSF). Per-term squared error in Q26 is
// promoted to int64 before scaling by the Q11 weight to avoid
// overflow; the cumulative cost remains within int64 (5 terms ·
// 2^15 · 2^28 ≪ 2^63).
//
// Allocation contract (I4): all workspace lives in stack-allocated
// fixed-size arrays. The mem parameter is read-only (caller owns the
// FIFO; commitPredictorMemory advances it once the L0 winner is
// chosen).
func searchL2(l1, selector uint8, mem *[4][10]int16, omega, weights *[10]int16) (int, int64) {
	var (
		bestIdx  int
		bestMSE  int64 = -1
		residual [10]int16
		omegaHat [10]int16
	)

	for row := 0; row < len(tables.LSPCodebookL2); row++ {
		// Partial residual on i=0..4; upper half is irrelevant
		// because the partial sum below ranges only over i=0..4 and
		// applyPredictorWithMemory's per-coefficient output depends
		// solely on residual[i] and mem[k][i] at the same i.
		for i := 0; i < 5; i++ {
			residual[i] = fixed.Add(tables.LSPCodebookL1[l1][i], tables.LSPCodebookL2[row][i])
		}

		applyPredictorWithMemory(selector, mem, &residual, &omegaHat)

		// J=0.0012 pair-rearrangement restricted to the partial
		// 5-vector (indices 1..4): per spec line 890, applied before
		// the partial weighted MSE is evaluated. We inline the
		// rearrangeAdjacent body to avoid touching omegaHat[5..9],
		// which would otherwise contaminate omegaHat[4] when the
		// uninitialised upper half collapses below the J threshold.
		for i := 1; i < 5; i++ {
			if omegaHat[i]-omegaHat[i-1] < lsfRearrJ1 {
				sum := int32(omegaHat[i]) + int32(omegaHat[i-1])
				omegaHat[i-1] = int16((sum - int32(lsfRearrJ1)) / 2)
				omegaHat[i] = int16((sum + int32(lsfRearrJ1)) / 2)
			}
		}

		var mse int64
		for i := 0; i < 5; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			mse += int64(weights[i]) * d * d
		}

		if bestMSE < 0 || mse < bestMSE {
			bestMSE = mse
			bestIdx = row
		}
	}
	return bestIdx, bestMSE
}

// Quantize is the top-level LSF split-VQ encoder entry point. For
// each MA predictor selector L0 ∈ {0, 1} it runs the sequential-
// greedy search L1 → L2 → L3, reconstructs the full ω̂ via the
// predictor + J=0.0012 pair-rearrangement, and computes the total
// weighted MSE Σ_{i=0..9} w_i · (ω_i − ω̂_i)². The selector with
// the lower total cost wins; commitPredictorMemory is called exactly
// once on the winning residual to advance freqPrev.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 851 ("For each of the
// two MA predictors the best approximation … has to be found") and
// 896–897 ("the MA predictor L0 that produces the lowest weighted
// MSE is selected").
//
// Q-format: omega and freqPrev[k] in Q13; weights computed inline in
// Q11; total cost in int64 (10 terms · 2^15 · 2^28 ≪ 2^63).
//
// Allocation contract (I4): all workspace lives in fixed-size stack
// arrays. freqPrev is owned by the caller; this routine mutates it
// exactly once via commitPredictorMemory at the end.
//
// Evaluation budget (I12): 2 · (128 + 32 + 32) = 384 candidate
// codebook-row evaluations per frame, per the sequential-greedy
// reading of §3.2.4 lines 887–895.
func Quantize(omega *[10]int16, freqPrev *[4][10]int16) Indices {
	var weights [10]int16
	weightsLSF(omega, &weights)

	var (
		bestSel                uint8
		bestL1, bestL2, bestL3 uint8
		bestResidual           [10]int16
		bestCost               int64 = -1

		target   [10]int16
		residual [10]int16
		omegaHat [10]int16
	)

	for sel := uint8(0); sel < 2; sel++ {
		computeTargetLSF(sel, freqPrev, omega, &target)
		l1, _ := searchL1(&target)
		l2, _ := searchL2(uint8(l1), sel, freqPrev, omega, &weights)
		l3, _ := searchL3(uint8(l1), uint8(l2), sel, freqPrev, omega, &weights)

		for i := 0; i < 5; i++ {
			residual[i] = fixed.Add(tables.LSPCodebookL1[l1][i], tables.LSPCodebookL2[l2][i])
			residual[5+i] = fixed.Add(tables.LSPCodebookL1[l1][5+i], tables.LSPCodebookL3[l3][i])
		}
		// §3.2.4 lines 819–833: the final reconstruction (which the
		// decoder will reproduce verbatim) rearranges the residual
		// l̂_i twice — first with J=0.0012, then with J=0.0006 —
		// BEFORE applying the MA predictor. The encoder's final
		// L0-selection cost must reflect this exact pipeline so the
		// chosen ω̂ matches the decoder's. (The intra-search L2/L3
		// cost approximations rearrange ω̂ with J1 only per spec
		// lines 890/894 — that's a search heuristic, not the final
		// reconstruction.)
		rearrangeAdjacent(&residual, lsfRearrJ1)
		rearrangeAdjacent(&residual, lsfRearrJ2)
		applyPredictorWithMemory(sel, freqPrev, &residual, &omegaHat)
		// §3.2.4 lines 897–898 + decoder.go step 4: post-predictor
		// stability enforcement is part of the final ω̂ pipeline; the
		// L0 cost must measure ω̂ AFTER stability so the encoder's
		// pick aligns with the decoder's reproduction of the same
		// indices.
		enforceLSFStability(&omegaHat)

		var cost int64
		for i := 0; i < 10; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			cost += int64(weights[i]) * d * d
		}

		if bestCost < 0 || cost < bestCost {
			bestCost = cost
			bestSel = sel
			bestL1 = uint8(l1)
			bestL2 = uint8(l2)
			bestL3 = uint8(l3)
			bestResidual = residual
		}
	}

	commitPredictorMemory(freqPrev, &bestResidual)

	return Indices{L0: bestSel, L1: bestL1, L2: bestL2, L3: bestL3}
}

// QuantizeTopK is an expanded clean-room LSP VQ search. For each selector it
// keeps the top-K first-stage L1 rows by the same unweighted L1 distortion as
// Quantize, then searches every L2/L3 pair under the final weighted
// reconstruction cost.
//
// It intentionally shares the same target, predictor, rearrangement,
// stability, and predictor-memory commit pipeline as Quantize. The only
// difference is search breadth. The public encoder uses topK=1 as a small
// quality/complexity tradeoff while diagnostics can raise topK to measure the
// remaining L1-search ceiling. No external implementation behavior is encoded
// here.
func QuantizeTopK(omega *[10]int16, freqPrev *[4][10]int16, topK int) Indices {
	if topK < 1 {
		topK = 1
	}
	if topK > QuantizeTopKMax {
		topK = QuantizeTopKMax
	}

	var weights [10]int16
	weightsLSF(omega, &weights)

	var (
		bestSel                uint8
		bestL1, bestL2, bestL3 uint8
		bestResidual           [10]int16
		bestCost               int64 = -1

		target [10]int16
		topL1  [QuantizeTopKMax]uint8
	)

	for sel := uint8(0); sel < 2; sel++ {
		computeTargetLSF(sel, freqPrev, omega, &target)
		nL1 := searchL1TopK(&target, topK, &topL1)
		for li := 0; li < nL1; li++ {
			l1 := topL1[li]
			for l2 := 0; l2 < len(tables.LSPCodebookL2); l2++ {
				for l3 := 0; l3 < len(tables.LSPCodebookL3); l3++ {
					cost, residual := finalQuantizedLSFCost(sel, l1, uint8(l2), uint8(l3), freqPrev, omega, &weights)
					if bestCost < 0 || cost < bestCost {
						bestCost = cost
						bestSel = sel
						bestL1 = l1
						bestL2 = uint8(l2)
						bestL3 = uint8(l3)
						bestResidual = residual
					}
				}
			}
		}
	}

	commitPredictorMemory(freqPrev, &bestResidual)

	return Indices{L0: bestSel, L1: bestL1, L2: bestL2, L3: bestL3}
}

func searchL1TopK(target *[10]int16, topK int, rows *[QuantizeTopKMax]uint8) int {
	if topK > QuantizeTopKMax {
		topK = QuantizeTopKMax
	}
	var costs [QuantizeTopKMax]int32
	n := 0
	for row := 0; row < len(tables.LSPCodebookL1); row++ {
		var sum int32
		for i := 0; i < 10; i++ {
			diff := int32(target[i]) - int32(tables.LSPCodebookL1[row][i])
			sum += diff * diff
		}
		pos := n
		for pos > 0 && sum < costs[pos-1] {
			pos--
		}
		if pos >= topK {
			continue
		}
		if n < topK {
			n++
		}
		for i := n - 1; i > pos; i-- {
			costs[i] = costs[i-1]
			rows[i] = rows[i-1]
		}
		costs[pos] = sum
		rows[pos] = uint8(row)
	}
	return n
}

func finalQuantizedLSFCost(
	selector uint8,
	l1, l2, l3 uint8,
	mem *[4][10]int16,
	omega, weights *[10]int16,
) (int64, [10]int16) {
	var residual, omegaHat [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = fixed.Add(tables.LSPCodebookL1[l1][i], tables.LSPCodebookL2[l2][i])
		residual[5+i] = fixed.Add(tables.LSPCodebookL1[l1][5+i], tables.LSPCodebookL3[l3][i])
	}
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)
	applyPredictorWithMemory(selector, mem, &residual, &omegaHat)
	enforceLSFStability(&omegaHat)

	var cost int64
	for i := 0; i < 10; i++ {
		d := int64(omega[i]) - int64(omegaHat[i])
		cost += int64(weights[i]) * d * d
	}
	return cost, residual
}

// TupleCostForDiagnostic evaluates one transmitted LSP tuple against omega
// using the same final weighted reconstruction cost as Quantize. It is
// intended for black-box oracle diagnostics; it does not mutate freqPrev.
func TupleCostForDiagnostic(omega *[10]int16, freqPrev *[4][10]int16, idx Indices) int64 {
	var weights [10]int16
	weightsLSF(omega, &weights)
	cost, _ := finalQuantizedLSFCost(idx.L0, idx.L1, idx.L2, idx.L3, freqPrev, omega, &weights)
	return cost
}

// CommitIndicesForDiagnostic advances freqPrev with the residual represented
// by idx. It mirrors the Quantize commit step for diagnostic memory tracking.
func CommitIndicesForDiagnostic(freqPrev *[4][10]int16, idx Indices) {
	var residual [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = fixed.Add(tables.LSPCodebookL1[idx.L1][i], tables.LSPCodebookL2[idx.L2][i])
		residual[5+i] = fixed.Add(tables.LSPCodebookL1[idx.L1][5+i], tables.LSPCodebookL3[idx.L3][i])
	}
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)
	commitPredictorMemory(freqPrev, &residual)
}

// searchL3 returns the index L3 ∈ [0, 32) of the row of
// LSPCodebookL3 that minimizes the partial weighted MSE
//
//	Σ_{i=5..9} w_i · (ω_i − ω̂_i)²
//
// where ω̂[0..9] is reconstructed from the candidate residual
// r[0..4] = L1[l1][0..4] + L2[l2][0..4],
// r[5..9] = L1[l1][5..9] + L3[row][0..4]
// via the MA predictor (selector ∈ {0,1}, predictor memory mem)
// followed by the J=0.0012 pair-rearrangement (lsfRearrJ1) applied
// across the full reconstructed [0..9] vector.
//
// Spec: ITU-T G.729 (06/2012) §3.2.4 lines 893–895 — "Then, using
// the selected first stage vector L1 and the selected lower part of
// the second stage L2, the upper part of the second stage L3 is
// searched. Again the rearrangement procedure is used to guarantee a
// minimum distance of 0.0012" (line 893).
//
// Q-format: identical to searchL2; per-term squared error in Q26 is
// promoted to int64 before scaling by the Q11 weight; the cumulative
// 5-term cost stays well within int64.
//
// Allocation contract (I4): all workspace lives in stack-allocated
// fixed-size arrays. mem is read-only.
func searchL3(l1, l2, selector uint8, mem *[4][10]int16, omega, weights *[10]int16) (int, int64) {
	var (
		bestIdx  int
		bestMSE  int64 = -1
		residual [10]int16
		omegaHat [10]int16
	)

	// Lower half of the residual is fixed across all 32 candidates:
	// it depends only on the L1 and L2 winners, which are bound
	// arguments. Hoist the combine out of the inner loop.
	for i := 0; i < 5; i++ {
		residual[i] = fixed.Add(tables.LSPCodebookL1[l1][i], tables.LSPCodebookL2[l2][i])
	}

	for row := 0; row < len(tables.LSPCodebookL3); row++ {
		for i := 0; i < 5; i++ {
			residual[5+i] = fixed.Add(tables.LSPCodebookL1[l1][5+i], tables.LSPCodebookL3[row][i])
		}

		applyPredictorWithMemory(selector, mem, &residual, &omegaHat)

		// J=0.0012 pair-rearrangement spans the full [0..9] vector
		// per spec line 893 ("Again the rearrangement procedure is
		// used"). This may shift omegaHat[5] via its dependence on
		// omegaHat[4]; the partial MSE on i=5..9 is computed after.
		rearrangeAdjacent(&omegaHat, lsfRearrJ1)

		var mse int64
		for i := 5; i < 10; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			mse += int64(weights[i]) * d * d
		}

		if bestMSE < 0 || mse < bestMSE {
			bestMSE = mse
			bestIdx = row
		}
	}
	return bestIdx, bestMSE
}
