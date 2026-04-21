package lsp

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
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
// fractional multiply); a single fixed.Round at the end brings the
// accumulated sum back to Q13 with rounding.
func (d *Decoder) applyPredictor(selector uint8, residual, out *[10]int16) {
	preds := &tables.MAPredictorsLSP[selector]

	for i := 0; i < 10; i++ {
		var sumP int16
		for k := 0; k < 4; k++ {
			sumP = fixed.Add(sumP, preds[k][i])
		}
		comp := fixed.Sub(fixed.Max16, sumP)

		var acc fixed.Word32
		acc = fixed.LMac(acc, comp, residual[i])
		acc = fixed.LMac(acc, preds[0][i], d.pastResiduals[0][i])
		acc = fixed.LMac(acc, preds[1][i], d.pastResiduals[1][i])
		acc = fixed.LMac(acc, preds[2][i], d.pastResiduals[2][i])
		acc = fixed.LMac(acc, preds[3][i], d.pastResiduals[3][i])

		out[i] = fixed.Round(acc)
	}

	d.pastResiduals[3] = d.pastResiduals[2]
	d.pastResiduals[2] = d.pastResiduals[1]
	d.pastResiduals[1] = d.pastResiduals[0]
	d.pastResiduals[0] = *residual
}
