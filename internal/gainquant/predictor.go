package gainquant

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/tables"
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
// ENC-GAIN-SPLIT: this search-surface predictor intentionally uses the
// legacy Word16-bounded predicted log gain before expanding to g'c Q12.
// The receiver-side gain reconstruction keeps the wider int32 predictor
// to avoid decoder amplitude collapse, but applying that wider predictor
// directly to the §3.9.2 encoder search regresses the FFmpeg black-box
// encoder gate. Keeping the split explicit prevents decoder robustness
// fixes from silently changing encoder candidate selection.
//
// Composition: gain.PredictedLogGainSat16 (eq. 69 bounded form) +
// gain.FixedCodebookEnergy + gain.Log2Fixed/gain.Pow2Fixed (eq. 66 /
// eq. 71).
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
//     at Q12 (consumed by SearchConjugate as int32).
//
// Zero-energy guard: when Σc² = 0, log10(0) is mathematically −∞;
// returning 0 (rather than saturating to int32 extrema) matches the
// decoder's protective branch in gain.Decode.
func PredictedGcQ12(pastQuaEn *[4]int16, c *[40]int16) int32 {
	log2GcQ10, ok := predictedLog2GcQ10Search(pastQuaEn, c)
	if !ok {
		return 0
	}
	return int32(gain.Pow2Fixed(fixed.Word32(log2GcQ10) + 12*1024))
}

// predictedLog2GcQ10Search returns the predicted log2(g'c) at Q10 for
// the encoder's §3.9.2 VQ candidate search. It intentionally uses the
// bounded MA prediction described on PredictedGcQ12.
func predictedLog2GcQ10Search(pastQuaEn *[4]int16, c *[40]int16) (int32, bool) {
	ecEnergy := gain.FixedCodebookEnergy(c)
	if ecEnergy <= 0 {
		return 0, false
	}

	predicted := gain.PredictedLogGainSat16(pastQuaEn)

	ecLog2Q10 := int32(gain.Log2Fixed(ecEnergy)) - 26*1024
	ecDbQ10 := (ecLog2Q10*dbPerLog2Q13 + (1 << 12)) >> 13
	ecBarDbQ10 := fixed.Saturate(fixed.Word32(ecDbQ10 - int32(tenLog10_40Q10)))

	logGainDbQ10 := fixed.Sub(predicted, ecBarDbQ10)
	log2GcQ10 := (int32(logGainDbQ10)*invDbScaleQ15 + (1 << 14)) >> 15
	return log2GcQ10, true
}

// DequantGc reconstructs the chosen quantized fixed-codebook gain g_c
// in the native (mantissa Q14, exponent int8) representation per
// REF-1 §2 (docs/superpowers/plans/2026-05-04-phase3a-gcrep-design.md).
//
// Inputs:
//
//   - log2GcPredQ10: predicted log2(g'c) at Q10 from predictedLog2GcQ10.
//   - ok:            false ⇔ Σc² == 0 zero-energy guard (mant=0, exp=0).
//   - gammaCQ13:     γ̂_c at Q13 = GBK1[ga][1] + GBK2[gb][1].
//
// Math (same mantissa/exponent decomposition used by gain.Decoder.Decode):
//
//	log2(γ̂_c) Q10  = log2Fixed(γ̂_c) − 13·1024     (γ̂_c is Q13)
//	log2(g_c)  Q10 = log2GcPredQ10 + log2(γ̂_c) Q10
//	intPart        = log2(g_c) >> 10
//	frac           = log2(g_c) − (intPart << 10)
//	gcMantQ14      = Pow2FracQ14(frac)
//	gcExp          = clamp(intPart, [-128, 127])
//
// Encoder-side split: the decomposition is shared with the decoder, but the
// encoder's caller may pass the bounded search predictor while the strict
// receiver decoder uses a wider predictor. TestApply_MantissaExponent pins the
// representation contract and documented split cases.
func DequantGc(log2GcPredQ10 int32, ok bool, gammaCQ13 int16) (gcMantQ14 int16, gcExp int8) {
	if !ok || gammaCQ13 <= 0 {
		return 0, 0
	}
	gammaLog2Q10 := int32(gain.Log2Fixed(fixed.Word32(gammaCQ13))) - 13*1024
	log2GcWithGammaQ10 := log2GcPredQ10 + gammaLog2Q10
	intPart := log2GcWithGammaQ10 >> 10
	frac := log2GcWithGammaQ10 - (intPart << 10)
	gcMantQ14 = gain.Pow2FracQ14(frac)
	switch {
	case intPart > 127:
		gcExp = 127
	case intPart < -128:
		gcExp = -128
	default:
		gcExp = int8(intPart)
	}
	return
}

// Reconstruct is the encoder-side "Apply" surface: given the predictor
// state (pastQuaEn), the current fixed-codebook vector c, and the
// codebook entry pair (ga, gb) chosen by SearchConjugate, returns the
// (gpQ14, gcMantQ14, gcExp) triple used to commit the encoder's local
// synthesis state.
//
// Pure / read-only on inputs; does NOT update pastQuaEn (the caller
// owns the FIFO advance via UpdatePastQuaEn after the §A.3.10 commit).
//
// ENC-GAIN-SPLIT: this intentionally shares the bounded search predictor
// rather than the wider receiver-side decoder predictor. The FFmpeg
// black-box encoder gate regresses when the encoder commits local synthesis
// with the wider predictor even though the strict decoder benefits from it.
func Reconstruct(pastQuaEn *[4]int16, c *[40]int16, ga, gb uint8) (gpQ14, gcMantQ14 int16, gcExp int8) {
	gpQ14 = fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0])))
	gammaCQ13 := fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[ga][1]) + int32(tables.GainGBK2[gb][1])))
	log2GcPredQ10, ok := predictedLog2GcQ10Search(pastQuaEn, c)
	gcMantQ14, gcExp = DequantGc(log2GcPredQ10, ok, gammaCQ13)
	return
}

// UpdatePastQuaEn applies the §3.9.1 eq. (72) past-energy FIFO update:
//
// pastQuaEn[3] ← pastQuaEn[2]
// pastQuaEn[2] ← pastQuaEn[1]
// pastQuaEn[1] ← pastQuaEn[0]
// pastQuaEn[0] ← U(m) = 20·log10(γ̂_c)   (Q10 dB)
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
