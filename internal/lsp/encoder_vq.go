package lsp

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

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
////
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
