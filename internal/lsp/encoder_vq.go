package lsp

import (
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
