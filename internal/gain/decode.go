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
//	invDbScaleQ15  ≈ 1 / (20·log10(2)) · 2¹⁵, fixed decoder constant
//	dbPerLog2Q10   = 20·log10(2) · 2¹⁰  ≈ 6165   // dB per unit log2
const (
	dbPerLog2Q13   = 24660
	tenLog10_40Q10 = 16405
	invDbScaleQ15  = 5439
	dbPerLog2Q10   = 6165
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

	// Q26→Q0 correction: fixedCodebookEnergy returns Σc² at Q26 (energy.go
	// §). log2(E_phys) = log2(E_Q26) − 26 ⇒ subtract 26·1024 in Q10. Keep
	// int32 throughout so high-dynamic-range gain reconstruction is not
	// collapsed by an intermediate Word16 saturation.
	ecLog2Q10 := int32(log2Fixed(ecEnergy)) - int32(ecQCorrection)*1024
	ecDbQ10 := (ecLog2Q10*dbPerLog2Q13 + (1 << 12)) >> 13
	ecBarDbQ10 := ecDbQ10 - int32(tenLog10_40Q10)

	// 3. Effective log gain in dB -> log2.
	//
	// The target is a Q10 dB quantity, but the predictor and energy terms can
	// differ by more than int16 can represent. Keep this subtraction in int32;
	// saturating here collapses high-dynamic-range fixed-codebook gains before
	// the pow2 reconstruction has a chance to express them.
	logGainDbQ10 := predicted - int32(ecBarDbQ10)
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
		gainQ14 := fixedGainQ14FromLog2Gamma(log2GcQ15, gammaC)
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
		gammaLog2Q10 := log2Fixed(fixed.Word32(gammaC)) - 13*1024
		val := (int32(gammaLog2Q10)*dbPerLog2Q10 + (1 << 9)) >> 10
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		uCurrent = int16(val)
	} else {
		uCurrent = pastErrorsDefault
	}
	d.pastErrors[3] = d.pastErrors[2]
	d.pastErrors[2] = d.pastErrors[1]
	d.pastErrors[1] = d.pastErrors[0]
	d.pastErrors[0] = uCurrent

	return
}
