package lsp

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// applyPredictorWithMemory mirrors Decoder.applyPredictor's arithmetic
// for ITU-T G.729 §3.2.4 equation (20) but reads the past-residual
// memory by pointer and does NOT mutate it. This enables the encoder's
// L0 search loop to evaluate the MA predictor for both selectors (and
// trial residuals) without committing FIFO state until the
// minimum-distortion candidate is chosen.
//
// Q-format contract is identical to the decoder side:
//
//	residual                : Q13 Word16
//	predictor coefficients  : Q15 Word16
//	out                     : Q13 Word16
//	mem[k][i]               : Q13 Word16, mem[0] == r̂(n-1)
//
// I10 binding: encoder owns mem; this routine performs no allocation
// and no global state read/write.
func applyPredictorWithMemory(selector uint8, mem *[4][10]int16, residual, out *[10]int16) {
	preds := &tables.MAPredictorsLSP[selector]

	for i := 0; i < 10; i++ {
		var sumP int16
		for k := 0; k < 4; k++ {
			sumP = fixed.Add(sumP, preds[k][i])
		}
		comp := fixed.Sub(fixed.Max16, sumP)

		var acc fixed.Word32
		acc = fixed.LMac(acc, comp, residual[i])
		acc = fixed.LMac(acc, preds[0][i], mem[0][i])
		acc = fixed.LMac(acc, preds[1][i], mem[1][i])
		acc = fixed.LMac(acc, preds[2][i], mem[2][i])
		acc = fixed.LMac(acc, preds[3][i], mem[3][i])

		out[i] = fixed.Round(acc)
	}
}

// commitPredictorMemory advances the order-4 MA predictor FIFO by
// installing residual as the new r̂(n-1) and shifting the older
// frames back by one slot. This is the post-decision counterpart to
// applyPredictorWithMemory: call it once per encoded frame after the
// L0 selector and quantized residual have been chosen.
func commitPredictorMemory(mem *[4][10]int16, residual *[10]int16) {
	mem[3] = mem[2]
	mem[2] = mem[1]
	mem[1] = mem[0]
	mem[0] = *residual
}
