package gain

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// PredictedLogGain computes the MA-predicted log-gain Ê(m) per
// ITU-T G.729 §3.9.1 eq. (69):
//
//	Ê(m) = E̅ + Σ_{i=1..4} b_i · Û(m-i)        (Q10 dB)
//
// where b_i = tables.GainMAPredictor[i-1] (Q13), Û(m-i) =
// pastErrors[i-1] (Q10), and E̅ = tables.GainMeanEnergyQ10.
//
// Q-format walk: LMac accumulates b_i · Û(m-i) as Q(13+10+1) = Q24.
// Round(LShl(acc, 2)) shifts to Q26 then takes the high half with
// rounding, yielding the contribution in Q10. fixed.Add then folds
// in E̅, the long-term mean energy reference.
//
// The pure free-function form is the GQ-1 export consumed by the
// encoder-side predictor (internal/gainquant). The decoder method
// delegates to this implementation.
func PredictedLogGain(pastErrors *[4]int16) int16 {
	var acc fixed.Word32
	for i := 0; i < 4; i++ {
		acc = fixed.LMac(acc, tables.GainMAPredictor[i], fixed.Word16(pastErrors[i]))
	}
	predicted := fixed.Round(fixed.LShl(acc, 2))
	return int16(fixed.Add(tables.GainMeanEnergyQ10, predicted))
}

// predictedLogGain is the decoder-bound thin wrapper retained for the
// existing Decode pipeline. Delegates to PredictedLogGain.
func (d *Decoder) predictedLogGain() int16 {
	return PredictedLogGain(&d.pastErrors)
}
