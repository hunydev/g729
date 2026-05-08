package decoder

import (
	"math"

	"github.com/hunydev/g729/internal/fixed"
)

func gainLinearFromMantExp(mantQ14 int16, exp int8) float64 {
	if mantQ14 == 0 {
		return 0
	}
	return float64(mantQ14) * math.Exp2(float64(exp)-14.0)
}

func excitationCodeQ15FromMantExp(mantQ14 int16, exp int8, cQ13 int16) fixed.Word32 {
	if mantQ14 == 0 {
		return 0
	}
	prod32 := fixed.LMult(fixed.Word16(mantQ14), fixed.Word16(cQ13))
	shiftR := 13 - int(exp)
	if shiftR >= 0 {
		return fixed.LShr(prod32, fixed.Word16(shiftR))
	}
	return fixed.LShl(prod32, fixed.Word16(-shiftR))
}

func excitationSampleFromMantExp(gpQ14, mantQ14 int16, exp int8, vQ0, cQ13 int16) int16 {
	lPitch := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(vQ0))
	lCode := excitationCodeQ15FromMantExp(mantQ14, exp, cQ13)
	lSum := fixed.LAdd(lPitch, lCode)
	return int16(fixed.Round(fixed.LShl(lSum, 1)))
}
