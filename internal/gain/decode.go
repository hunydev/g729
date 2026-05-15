package gain

import (
	"github.com/hunydev/g729/internal/fixed"
)

// pastErrorsDefault is the spec's initial value for each entry of the
// MA-predictor tap line (§3.9 / §4.1.6): −14 dB Q10 = −14336.
const pastErrorsDefault int16 = -14336

// PastErrorsDefault is the exported alias of pastErrorsDefault, used by
// encoder-side predictor state initialization (internal/gainquant) so
// the cold-start MA tap line matches the decoder convention.
const PastErrorsDefault int16 = pastErrorsDefault

// Numerical constants derived from physical identities (NOT from any
// existing G.729 implementation):
//
//	dbPerLog2Q13 = 10·log10(2) · 2¹³  ≈ 24660  // dB per unit log2
//	tenLog10_40Q10 = 10·log10(40) · 2¹⁰ ≈ 16405  // 10·log10(40) Q10 dB
//	tenLog10_40ReferenceQ10 = 16404, the reference fixed-point bias used
//		for decoder gain reconstruction after Q15 log2 conversion.
//	invDbScaleQ15  ≈ 1 / (20·log10(2)) · 2¹⁵, fixed decoder constant
//	dbPerLog2Q10   = 20·log10(2) · 2¹⁰  ≈ 6165   // dB per unit log2
const (
	dbPerLog2Q13            = 24660
	tenLog10_40Q10          = 16405
	tenLog10_40ReferenceQ10 = 16404
	invDbScaleQ15           = 5439
	dbPerLog2Q10            = 6165

	// gainLogEcBarBiasQ14 is the fixed Q14 accumulator seed used by the
	// receiver log-gain path before subtracting the uncorrected Ec Log2 dB
	// product. It is pinned by decoder_tame_gain_log_gain_micro_expected.csv.
	gainLogEcBarBiasQ14 = 2085632
)

// Decode decodes one subframe's gains from idx and the fixed codebook
// vector c per ITU-T G.729 §3.9 / §4.1.6.
//
// Returns (gpQ14, gcMantQ14, gcExp) per REF-1
// (docs/superpowers/plans/2026-05-04-phase3a-gcrep-design.md §2):
//
//   - gpQ14:     pitch gain g_p in Q14 (range [0, ~1.2]).
//   - gcMantQ14: g_c mantissa in Q14, value in [16384, 32767]
//     representing [1.0, 2.0). 0 only on the zero-energy guard path.
//   - gcExp:     binary exponent (int8). Linear g_c = gcMantQ14 ·
//     2^(gcExp - 14). Typical corpus range [-15, +9].
//
// Side effect: the 4-tap MA predictor FIFO is shifted and the new entry
// (current log-correction error U(m) = 20·log10(γ̂_c) Q10) is inserted
// at index 0. U(m) is computed from γ̂_c alone per the spec and is
// independent of the (mantissa, exp) representation choice.
func (d *Decoder) Decode(idx Indices, c *[40]int16) (gpQ14, gcMantQ14 int16, gcExp int8) {
	return d.decode(idx, c, 26, 13)
}

// DecodeWithGammaLogCorrection decodes one subframe's gains while allowing
// the caller to choose the Q correction used when folding gamma_c into the
// reconstructed fixed-codebook gain. The default Decode path uses 13 because
// gamma_c is represented as Q13. The local decoder's experimental enhanced
// listening path may pass a different correction as a bounded black-box
// quality aid; callers that need strict decoder behavior should use Decode.
func (d *Decoder) DecodeWithGammaLogCorrection(idx Indices, c *[40]int16, gammaQCorrection int) (gpQ14, gcMantQ14 int16, gcExp int8) {
	if gammaQCorrection <= 0 {
		gammaQCorrection = 13
	}
	return d.decode(idx, c, 26, gammaQCorrection)
}

// DecodeWithLogCorrections decodes one subframe's gains while allowing both
// the fixed-codebook energy Q correction and gamma_c Q correction to be
// selected by an opt-in caller. The strict decoder path uses Decode.
func (d *Decoder) DecodeWithLogCorrections(idx Indices, c *[40]int16, ecQCorrection, gammaQCorrection int) (gpQ14, gcMantQ14 int16, gcExp int8) {
	if ecQCorrection <= 0 {
		ecQCorrection = 26
	}
	if gammaQCorrection <= 0 {
		gammaQCorrection = 13
	}
	return d.decode(idx, c, ecQCorrection, gammaQCorrection)
}

func (d *Decoder) decode(idx Indices, c *[40]int16, ecQCorrection, gammaQCorrection int) (gpQ14, gcMantQ14 int16, gcExp int8) {
	if !d.initialized {
		for i := range d.pastErrors {
			d.pastErrors[i] = pastErrorsDefault
		}
		d.initialized = true
	}

	// 1. E̅_c = 10·log10(E_c / 40) Q10 dB.
	ecEnergy := fixedCodebookEnergy(c)

	// Zero-energy guard: log10(0) is mathematically -∞; flooring it
	// inside log2Fixed produces an artificially boosted gc that
	// saturates to int16 extrema. The decoder should be robust to a
	// zero (or near-zero) fixed codebook by suppressing the fixed-
	// codebook contribution entirely for the current subframe and
	// re-seeding the MA predictor's history with the long-term default
	// (−14 dB Q10) so the next subframe's prediction is well-defined.
	if ecEnergy <= 0 {
		gp, _ := decodeVQ(idx)
		gpQ14 = gp
		gcMantQ14 = 0
		gcExp = 0
		d.pastErrors[3] = d.pastErrors[2]
		d.pastErrors[2] = d.pastErrors[1]
		d.pastErrors[1] = d.pastErrors[0]
		d.pastErrors[0] = pastErrorsDefault
		return
	}

	// 2. Predict log-gain Ê(m) from past errors (Q10 dB).
	predicted := d.predictedLogGain()

	ecLog2Q15 := fixedCodebookEnergyLog2Q15(ecEnergy, ecQCorrection)
	ecDbQ10 := energyDbFromLog2Q15Trunc(ecLog2Q15)

	// 3. Effective log gain in dB -> log2.
	//
	// The target is a Q10 dB quantity, but the predictor and energy terms can
	// differ by more than int16 can represent. Keep this subtraction in int32;
	// saturating here collapses high-dynamic-range fixed-codebook gains before
	// the pow2 reconstruction has a chance to express them.
	logGainDbQ10 := predicted + int32(tenLog10_40ReferenceQ10) - ecDbQ10
	if ecQCorrection == 26 {
		logGainDbQ10 = logGainDbQ10FromEnergyQ24(&d.pastErrors, ecEnergy)
	}
	log2GcQ15 := logGainToLog2Q15(logGainDbQ10)
	log2GcQ10 := log2GcQ15 >> 5

	// 4. Decode γ̂_c (Q13) and fold its log2 contribution INTO the
	//    log2 exponent BEFORE the pow2 split. This preserves the spec's
	//    natural mantissa+exponent decomposition and lets us return the
	//    full dynamic range without an int16 collapse on g_c.
	//
	//    log2(γ̂_c) at Q10  =  log2Fixed(γ̂_c) − 13·1024   (γ̂_c is Q13)
	//    γ̂_c == 0 short-circuits to mant=0, exp=0; predictor U(m) below
	//    falls back to the −14 dB default, mirroring the historical
	//    behavior of the legacy code path.
	gp, gammaC := decodeVQ(idx)
	gpQ14 = gp

	if gammaC <= 0 {
		gcMantQ14 = 0
		gcExp = 0
	} else if gammaQCorrection == 13 {
		gainQ14 := quantizeFixedGainQ1(fixedGainQ14FromLog2Gamma(log2GcQ15, gammaC))
		gcMantQ14, gcExp = splitGainQ14(gainQ14)
	} else {
		gammaLog2Q10 := int32(log2Fixed(fixed.Word32(gammaC))) - int32(gammaQCorrection)*1024
		log2GcWithGammaQ10 := log2GcQ10 + gammaLog2Q10
		intPart := log2GcWithGammaQ10 >> 10
		frac := log2GcWithGammaQ10 - (intPart << 10)
		gcMantQ14 = pow2FracQ14(frac)
		switch {
		case intPart > 127:
			gcExp = 127
		case intPart < -128:
			gcExp = -128
		default:
			gcExp = int8(intPart)
		}
	}

	// 5. Update MA predictor FIFO with U(m) = 20·log10(γ̂_c) Q10.
	//    γ̂_c is Q13 so log2(γ̂_c) - 13 gives the value's log2. Multiply
	//    by 20·log10(2) to convert to dB. (Unchanged from the pre-REF-1
	//    flow; computed from γ̂_c alone per spec.)
	var uCurrent int16
	if gammaC > 0 {
		uCurrent = quantizedPredictionErrorQ10(gammaC)
	} else {
		uCurrent = pastErrorsDefault
	}
	d.pastErrors[3] = d.pastErrors[2]
	d.pastErrors[2] = d.pastErrors[1]
	d.pastErrors[1] = d.pastErrors[0]
	d.pastErrors[0] = uCurrent

	return
}

func fixedCodebookEnergyLog2Q15(ecEnergy fixed.Word32, ecQCorrection int) int32 {
	logInput := ecEnergy
	qCorrection := ecQCorrection
	if ecQCorrection == 26 {
		// Reference decoder arithmetic feeds the pre-L_shr L_mult energy into
		// Log2, then compensates the extra factor by subtracting 27 bits. This
		// preserves five more fractional log2 bits for the dB conversion while
		// representing the same physical Σc² energy.
		logInput = fixed.LShl(ecEnergy, 1)
		qCorrection = 27
	}
	return int32(log2FixedQ15(logInput)) - int32(qCorrection)*(1<<15)
}

// LogGainDbQ10FromCodebook returns the receiver-aligned fixed-codebook log
// gain target in Q10 dB. It keeps the MA predictor and Ec-bar terms in Q24
// until the final truncation, which avoids the 1-LSB drift caused by combining
// separately truncated Q10 terms.
func LogGainDbQ10FromCodebook(pastErrors *[4]int16, c *[40]int16) (int32, bool) {
	ecEnergy := fixedCodebookEnergy(c)
	if ecEnergy <= 0 {
		return 0, false
	}
	return logGainDbQ10FromEnergyQ24(pastErrors, ecEnergy), true
}

func logGainDbQ10FromEnergyQ24(pastErrors *[4]int16, ecEnergy fixed.Word32) int32 {
	rawLog2Q15 := int32(log2FixedQ15(fixed.LShl(ecEnergy, 1)))
	intPart := rawLog2Q15 >> 15
	fracQ15 := rawLog2Q15 - (intPart << 15)
	ecBarQ14 := int64(gainLogEcBarBiasQ14) + log2DBProductQ14(intPart, fracQ15, -dbPerLog2Q13)
	logGainQ24 := (ecBarQ14 << 10) + PredictedEnergyQ24(pastErrors)
	return int32(logGainQ24 >> 14)
}

func log2DBProductQ14(intPart, fracQ15, multiplierQ13 int32) int64 {
	// The reference path does not multiply the full Q15 log2 value in one
	// wide expression. It doubles the integer contribution and the Q15
	// fractional Mult result separately, which changes half-LSB ties by one.
	intTerm := int64(intPart) * int64(multiplierQ13) * 2
	fracTerm := int64(fixed.Mult(fixed.Word16(fracQ15), fixed.Word16(multiplierQ13))) * 2
	return intTerm + fracTerm
}

func energyDbFromLog2Q15Trunc(log2Q15 int32) int32 {
	return int32((int64(log2Q15) * int64(dbPerLog2Q13)) >> 18)
}

func energyDbFromLog2Q15Rounded(log2Q15 int32) int32 {
	return int32((int64(log2Q15)*int64(dbPerLog2Q13) + (1 << 17)) >> 18)
}

// QuantizedPredictionErrorQ10 returns U(m) = 20*log10(gamma_hat) in Q10 dB
// for a positive gamma_hat Q13 value. The fixed-point path mirrors the
// decoder-stage oracle: form the corrected Log2 output as Q16, truncate to Q13,
// multiply by the Q12 dB/log2 factor, then shift back to Q10.
func QuantizedPredictionErrorQ10(gammaCQ13 int32) int16 {
	gammaLog2Q15 := int32(log2FixedQ15(fixed.Word32(gammaCQ13))) - 13*(1<<15)
	if gammaLog2Q15 == 0 {
		return 0
	}
	log2Q13 := gammaLog2Q15 >> 2
	val := int32((int64(log2Q13) * int64(dbPerLog2Q13)) >> 15)
	if val > 32767 {
		return 32767
	}
	if val < -32768 {
		return -32768
	}
	return int16(val)
}

func quantizedPredictionErrorQ10(gammaCQ13 int32) int16 {
	return QuantizedPredictionErrorQ10(gammaCQ13)
}
