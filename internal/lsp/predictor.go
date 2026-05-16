package lsp

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// applyPredictor reconstructs the quantized LSF vector ω̂(n) from the
// current residual r̂(n) and the decoder's past-residual state, per
// the order-4 MA predictor of ITU-T G.729 §3.2.4 equation (20):
//
//	ω̂(n)[i] = (1 - Σ_{k=1..4} p_k^{L0}[i]) · r̂(n)[i]
//	        + Σ_{k=1..4} p_k^{L0}[i] · r̂(n-k)[i]
//
// selector picks one of the two predictor sets (the L0 bit). After
// producing out, applyPredictor shifts the past-residual FIFO and
// installs r̂(n) as the new r̂(n-1).
//
// Q-format contract:
//
//	residual                : Q13 Word16
//	predictor coefficients  : Q15 Word16
//	out                     : Q13 Word16
//
// LMac products are Q29 Word32 (Q15·Q13 with the implicit ×2 of the
// fractional multiply); the accumulated sum is converted back to Q13
// by extracting the high word.
func (d *Decoder) applyPredictor(selector uint8, residual, out *[10]int16) {
	for i := 0; i < 10; i++ {
		out[i] = applyPredictorCoordinate(selector, i, residual[i], &d.pastResiduals)
	}

	d.pastResiduals[3] = d.pastResiduals[2]
	d.pastResiduals[2] = d.pastResiduals[1]
	d.pastResiduals[1] = d.pastResiduals[0]
	d.pastResiduals[0] = *residual
}

func applyPredictorCoordinate(selector uint8, i int, residual int16, past *[4][10]int16) int16 {
	preds := &tables.MAPredictorsLSP[selector]
	comp := tables.MAPredictorInvSumLSP[selector][i]

	var acc fixed.Word32
	acc = fixed.LMac(acc, comp, residual)
	acc = fixed.LMac(acc, preds[0][i], past[0][i])
	acc = fixed.LMac(acc, preds[1][i], past[1][i])
	acc = fixed.LMac(acc, preds[2][i], past[2][i])
	acc = fixed.LMac(acc, preds[3][i], past[3][i])
	return fixed.ExtractH(acc)
}

func inversePredictorResidual(selector uint8, target *[10]int16, past *[4][10]int16, out *[10]int16) {
	preds := &tables.MAPredictorsLSP[selector]
	inv := &maPredictorInvSumReciprocalLSP[selector]

	for i := 0; i < 10; i++ {
		acc := fixed.LDepositH(target[i])
		acc = fixed.LMsu(acc, preds[0][i], past[0][i])
		acc = fixed.LMsu(acc, preds[1][i], past[1][i])
		acc = fixed.LMsu(acc, preds[2][i], past[2][i])
		acc = fixed.LMsu(acc, preds[3][i], past[3][i])

		nQ13 := fixed.ExtractH(acc)
		out[i] = fixed.Saturate(fixed.Word32((int64(nQ13) * int64(inv[i])) >> 12))
	}
}

// maPredictorInvSumReciprocalLSP is the Q12 reciprocal scale for the
// LSP MA predictor complement term. The good-frame encoder target path
// uses an exact division because its result is quantized immediately;
// bad-frame decoder concealment advances the residual FIFO with this
// fixed-point reciprocal multiply so subsequent good frames keep the
// same predictor state as the reference decoder.
var maPredictorInvSumReciprocalLSP = [2][10]int16{
	{17210, 15888, 16357, 16183, 16516, 15833, 15888, 15421, 14840, 15597},
	{9202, 7320, 6788, 7738, 8170, 8154, 8856, 8818, 8366, 8544},
}
