package gain

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// PredictedLogGain computes the MA-predicted log-gain Ê(m) per
// ITU-T G.729 §3.9.1 eq. (69):
//
//	Ê(m) = E̅ + Σ_{i=1..4} b_i · Û(m-i)        (Q10 dB)
//
// where b_i = tables.GainMAPredictor[i-1] (Q13), Û(m-i) =
// pastErrors[i-1] (Q10), and E̅ = tables.GainMeanEnergyQ10.
//
// Q-format walk: the products b_i · Û(m-i) are accumulated at Q24
// because the fixed-point L_mac form doubles the Q13×Q10 product. The
// accumulator is then shifted left by 2 and truncated to Q10. The
// long-term mean energy reference E̅ is added in int32 so high-energy
// frames are not collapsed by an intermediate Word16 saturation before
// g_c reconstruction.
//
// The pure free-function form is the receiver-side GQ-1 export consumed by
// the decoder and decoder-equivalence diagnostics. The encoder search/commit
// surface uses PredictedLogGainSat16 to preserve the legacy bounded VQ search
// state while receiver-side reconstruction keeps this wider value.
func PredictedLogGain(pastErrors *[4]int16) int32 {
	predicted := int32(PredictedEnergyQ24(pastErrors) >> 14)
	return int32(tables.GainMeanEnergyQ10) + predicted
}

// PredictedEnergyQ24 returns the MA-predicted energy delta term from eq. 69
// before the final Q10 truncation. Keeping this Q24 accumulator until it is
// combined with Ec-bar matches the decoder gain log-gain oracle.
func PredictedEnergyQ24(pastErrors *[4]int16) int64 {
	var acc int64
	for i := 0; i < 4; i++ {
		acc += int64(2) * int64(tables.GainMAPredictor[i]) * int64(pastErrors[i])
	}
	return acc
}

// PredictedLogGainSat16 returns the legacy Word16-saturated form of
// the MA predictor. It is kept for the encoder search/commit surface while
// decoder-side gain reconstruction uses the int32 PredictedLogGain value.
func PredictedLogGainSat16(pastErrors *[4]int16) int16 {
	var acc fixed.Word32
	for i := 0; i < 4; i++ {
		acc = fixed.LMac(acc, tables.GainMAPredictor[i], fixed.Word16(pastErrors[i]))
	}
	predicted := fixed.ExtractH(fixed.LShl(acc, 2))
	return int16(fixed.Add(tables.GainMeanEnergyQ10, predicted))
}

// predictedLogGain is the decoder-bound thin wrapper retained for the
// existing Decode pipeline. Delegates to PredictedLogGain.
func (d *Decoder) predictedLogGain() int32 {
	return PredictedLogGain(&d.pastErrors)
}
