package gainquant

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/gain"
)

// Numerical constants derived from physical identities (clean-room from
// the spec, not from any existing G.729 implementation):
//
//	dbPerLog2Q13   = 10·log10(2) · 2¹³  ≈ 24660  // dB per unit log2
//	tenLog10_40Q10 = 10·log10(40) · 2¹⁰ ≈ 16405  // 10·log10(40) Q10 dB
//	invDbScaleQ15  = 1 / (20·log10(2)) · 2¹⁵ ≈ 5443  // log2 per dB
//
// Identical to the decoder-side constants in internal/gain/decode.go;
// re-stated here so the encoder predictor stays self-contained.
const (
	dbPerLog2Q13   = 24660
	tenLog10_40Q10 = 16405
	invDbScaleQ15  = 5443
	dbPerLog2Q10   = 6165
)

// PredictedGcQ12 returns the predicted fixed-codebook gain g'c (Q12)
// per ITU-T G.729 §3.9.1 eq. (71):
//
//	g'c = 10^[(Ê(m) − Ē(m)) / 20]
//
// where Ê(m) is the 4th-order MA prediction of the log-energy (eq. 69)
// formed from the FIFO of past quantized prediction errors `pastQuaEn`
// (Q10 dB; cold-start = gain.PastErrorsDefault = −14336), and Ē(m) is
// the mean-removed log-energy of the current fixed-codebook vector c
// (eq. 66, 10·log10(Σc²/40)).
//
// Composition: gain.PredictedLogGain (eq. 69) + gain.FixedCodebookEnergy
// + gain.Log2Fixed/gain.Pow2Fixed (eq. 66 / eq. 71).
//
// Q-format walk:
//
//   - gain.FixedCodebookEnergy returns Σc² at Q26 (c is Q13).
//   - gain.Log2Fixed treats the input as Q0; subtract 26·1024 to recover
//     log2(physical).
//   - Multiply by 10·log10(2) (Q13 constant) → 10·log10(Σc²) at Q10.
//   - Subtract 10·log10(40) (Q10) → Ē(m) Q10.
//   - logGainDb = Ê(m) − Ē(m); divide by 20·log10(2) (× invDbScaleQ15
//     >> 15) → log2(g'c) at Q10.
//   - gain.Pow2Fixed(log2GcQ10 + 12·1024) returns g'c·2^12 at Q0 = g'c
//     at Q12 (consumed as int16 by GQ-2).
//
// Zero-energy guard: when Σc² = 0, log10(0) is mathematically −∞;
// returning 0 (rather than saturating to int16 extrema) matches the
// decoder's protective branch in gain.Decode.
func PredictedGcQ12(pastQuaEn *[4]int16, c *[40]int16) int16 {
	ecEnergy := gain.FixedCodebookEnergy(c)
	if ecEnergy <= 0 {
		return 0
	}

	predicted := gain.PredictedLogGain(pastQuaEn)

	ecLog2Q10 := int32(gain.Log2Fixed(ecEnergy)) - 26*1024
	ecDbQ10 := (ecLog2Q10*dbPerLog2Q13 + (1 << 12)) >> 13
	ecBarDbQ10 := fixed.Saturate(fixed.Word32(ecDbQ10 - int32(tenLog10_40Q10)))

	logGainDbQ10 := fixed.Sub(predicted, ecBarDbQ10)
	log2GcQ10 := (int32(logGainDbQ10)*invDbScaleQ15 + (1 << 14)) >> 15

	gcQ12 := gain.Pow2Fixed(fixed.Word32(log2GcQ10) + 12*1024)
	if gcQ12 > 32767 {
		return 32767
	}
	if gcQ12 < -32768 {
		return -32768
	}
	return int16(gcQ12)
}

// UpdatePastQuaEn applies the §3.9.1 eq. (72) past-energy FIFO update:
//
//pastQuaEn[3] ← pastQuaEn[2]
//pastQuaEn[2] ← pastQuaEn[1]
//pastQuaEn[1] ← pastQuaEn[0]
//pastQuaEn[0] ← U(m) = 20·log10(γ̂_c)   (Q10 dB)
//
// The new entry U(m) is the current quantized prediction error in dB
// (eq. 72 line 1379), feeding the next subframe's MA prediction (eq. 69).
//
// Q-format walk:
//
//   - gammaCQ13 is the quantized fixed-codebook correction factor γ̂_c
//     (Q13; from §3.9.2 eq. 74 sum GBK1[ga][1] + GBK2[gb][1]).
//   - gain.Log2Fixed treats the input as Q0 → log2(γ̂·2^13) = log2(γ̂)+13;
//     subtract 13·1024 in Q10 to recover log2(γ̂) Q10.
//   - Multiply by 20·log10(2) (Q10 constant, dbPerLog2Q10 = 6165) and
//     >>10 → 20·log10(γ̂) at Q10 dB.
//
// Protective branch: γ̂ ≤ 0 is mathematically out-of-domain for log10;
// re-seed pastQuaEn[0] with PastErrorsDefault (-14 dB Q10), matching
// the decoder's gain.Decode zero-energy guard.
func UpdatePastQuaEn(pastQuaEn *[4]int16, gammaCQ13 int16) {
var uCurrent int16
if gammaCQ13 > 0 {
gammaLog2Q10 := int32(gain.Log2Fixed(fixed.Word32(gammaCQ13))) - 13*1024
val := (gammaLog2Q10*dbPerLog2Q10 + (1 << 9)) >> 10
if val > 32767 {
val = 32767
} else if val < -32768 {
val = -32768
}
uCurrent = int16(val)
} else {
uCurrent = gain.PastErrorsDefault
}
pastQuaEn[3] = pastQuaEn[2]
pastQuaEn[2] = pastQuaEn[1]
pastQuaEn[1] = pastQuaEn[0]
pastQuaEn[0] = uCurrent
}
