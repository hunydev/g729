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
//	tenLog10_40ReferenceQ10 = receiver fixed-point reconstruction bias.
//	invDbScaleQ15  ≈ 1 / (20·log10(2)) · 2¹⁵, fixed decoder constant
//
// Identical to the decoder-side constants in internal/gain/decode.go;
// re-stated here so the encoder predictor stays self-contained.
const (
	dbPerLog2Q13            = 24660
	tenLog10_40Q10          = 16405
	tenLog10_40ReferenceQ10 = 16404
	invDbScaleQ15           = 5439
	dbPerLog2Q10            = 6165
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
// to avoid decoder amplitude collapse. Production callers that need the
// clean-room Core/default Quality reconstruction surface use
// PredictedGcQ12Wide; this bounded helper is retained for diagnostics and
// explicit tuned-profile experiments.
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
	log2GcQ15, ok := predictedLog2GcQ15Search(pastQuaEn, c)
	if !ok {
		return 0
	}
	return predictedGcQ12FromLog2Q15(log2GcQ15)
}

// PredictedGcQ12Wide is the int32-predictor variant of PredictedGcQ12. It
// follows the same §3.9.1 equations but avoids the intermediate Word16
// saturation used by the legacy encoder search surface.
func PredictedGcQ12Wide(pastQuaEn *[4]int16, c *[40]int16) int32 {
	log2GcQ15, ok := predictedLog2GcQ15Wide(pastQuaEn, c)
	if !ok {
		return 0
	}
	return predictedGcQ12FromLog2Q15(log2GcQ15)
}

// predictedLog2GcQ10Search returns the predicted log2(g'c) at Q10 for
// the encoder's §3.9.2 VQ candidate search. It intentionally uses the
// bounded MA prediction described on PredictedGcQ12.
func predictedLog2GcQ15Search(pastQuaEn *[4]int16, c *[40]int16) (int32, bool) {
	ecEnergy := gain.FixedCodebookEnergy(c)
	if ecEnergy <= 0 {
		return 0, false
	}

	predicted := gain.PredictedLogGainSat16(pastQuaEn)

	ecDbQ10 := fixedCodebookEnergyDbQ10(ecEnergy)
	logGainDbQ10 := int32(predicted) + int32(tenLog10_40ReferenceQ10) - ecDbQ10
	return gain.LogGainToLog2Q15(logGainDbQ10), true
}

func predictedLog2GcQ15Wide(pastQuaEn *[4]int16, c *[40]int16) (int32, bool) {
	logGainDbQ10, ok := gain.LogGainDbQ10FromCodebook(pastQuaEn, c)
	if !ok {
		return 0, false
	}
	return gain.LogGainToLog2Q15(logGainDbQ10), true
}

func fixedCodebookEnergyDbQ10(ecEnergy fixed.Word32) int32 {
	logInput := fixed.LShl(ecEnergy, 1)
	log2Q15 := int32(gain.Log2FixedQ15(logInput)) - 27*(1<<15)
	return int32((int64(log2Q15) * int64(dbPerLog2Q13)) >> 18)
}

func predictedGcQ12FromLog2Q15(log2GcQ15 int32) int32 {
	return int32(gain.FixedGainQ14FromLog2Gamma(log2GcQ15, 8192) >> 2)
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
// encoder's caller may choose the bounded legacy search predictor or the wider
// receiver-aligned predictor. TestApply_MantissaExponent pins the
// representation contract and documented split cases.
func DequantGc(log2GcPredQ15 int32, ok bool, gammaCQ13 int32) (gcMantQ14 int16, gcExp int8) {
	if !ok || gammaCQ13 <= 0 {
		return 0, 0
	}
	gainQ14 := gain.QuantizeFixedGainQ1(gain.FixedGainQ14FromLog2Gamma(log2GcPredQ15, gammaCQ13))
	return gain.SplitGainQ14(gainQ14)
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
// ENC-GAIN-SPLIT: this intentionally shares the bounded legacy search
// predictor rather than the wider receiver-side decoder predictor. Core and
// default Quality use ReconstructWide together with PredictedGcQ12Wide;
// Reconstruct remains available for diagnostics and explicit tuned-profile
// experiments.
func Reconstruct(pastQuaEn *[4]int16, c *[40]int16, ga, gb uint8) (gpQ14, gcMantQ14 int16, gcExp int8) {
	gpQ14 = fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0])))
	gammaCQ13 := int32(tables.GainGBK1[ga][1]) + int32(tables.GainGBK2[gb][1])
	log2GcPredQ15, ok := predictedLog2GcQ15Search(pastQuaEn, c)
	gcMantQ14, gcExp = DequantGc(log2GcPredQ15, ok, gammaCQ13)
	return
}

// ReconstructWide is the int32-predictor variant of Reconstruct.
func ReconstructWide(pastQuaEn *[4]int16, c *[40]int16, ga, gb uint8) (gpQ14, gcMantQ14 int16, gcExp int8) {
	gpQ14 = fixed.Saturate(fixed.Word32(int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0])))
	gammaCQ13 := int32(tables.GainGBK1[ga][1]) + int32(tables.GainGBK2[gb][1])
	log2GcPredQ15, ok := predictedLog2GcQ15Wide(pastQuaEn, c)
	gcMantQ14, gcExp = DequantGc(log2GcPredQ15, ok, gammaCQ13)
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
//   - gain.QuantizedPredictionErrorQ10 applies the receiver-aligned fixed-
//     point Log2 and dB conversion path used by the decoder for U(m).
//
// Protective branch: γ̂ ≤ 0 is mathematically out-of-domain for log10;
// re-seed pastQuaEn[0] with PastErrorsDefault (-14 dB Q10), matching
// the decoder's gain.Decode zero-energy guard.
func UpdatePastQuaEn(pastQuaEn *[4]int16, gammaCQ13 int16) {
	var uCurrent int16
	if gammaCQ13 > 0 {
		uCurrent = gain.QuantizedPredictionErrorQ10(int32(gammaCQ13))
	} else {
		uCurrent = gain.PastErrorsDefault
	}
	pastQuaEn[3] = pastQuaEn[2]
	pastQuaEn[2] = pastQuaEn[1]
	pastQuaEn[1] = pastQuaEn[0]
	pastQuaEn[0] = uCurrent
}
