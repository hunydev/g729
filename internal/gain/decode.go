package gain

import (
	"github.com/exedev/g729/internal/fixed"
)

// pastErrorsDefault is the spec's initial value for each entry of the
// MA-predictor tap line (§3.9 / §4.1.6): −14 dB Q10 = −14336.
const pastErrorsDefault int16 = -14336

// Numerical constants derived from physical identities (NOT from any
// existing G.729 implementation):
//
//	dbPerLog2Q13 = 10·log10(2) · 2¹³  ≈ 24660  // dB per unit log2
//	tenLog10_40Q10 = 10·log10(40) · 2¹⁰ ≈ 16402  // 10·log10(40) Q10 dB
//	invDbScaleQ15  = 1 / (20·log10(2)) · 2¹⁵ ≈ 5443  // log2 per dB
//	dbPerLog2Q10   = 20·log10(2) · 2¹⁰  ≈ 6165   // dB per unit log2
const (
	dbPerLog2Q13   = 24660
	tenLog10_40Q10 = 16402
	invDbScaleQ15  = 5443
	dbPerLog2Q10   = 6165
)

// Decode decodes one subframe's gains from idx and the fixed codebook
// vector c per ITU-T G.729 §3.9 / §4.1.6.
//
// Returns (gpQ14, gcQ12):
//
//   - gpQ14: pitch gain g_p in Q14 (range [0, ~1.2]).
//   - gcQ12: fixed-codebook gain g_c in Q12 (chosen here so that typical
//     gain magnitudes in (0, 8) map to a non-zero int16; Phase 1g will
//     rescale at the excitation handoff).
//
// Side effect: the 4-tap MA predictor FIFO is shifted and the new entry
// (current log-correction error U(m) = 20·log10(γ̂_c) Q10) is inserted
// at index 0.
func (d *Decoder) Decode(idx Indices, c *[40]int16) (gpQ14, gcQ12 int16) {
	if !d.initialized {
		for i := range d.pastErrors {
			d.pastErrors[i] = pastErrorsDefault
		}
		d.initialized = true
	}

	// 1. Predict log-gain Ê(m) from past errors (Q10 dB).
	predicted := d.predictedLogGain()

	// 2. E̅_c = 10·log10(E_c / 40) Q10 dB.
	ecEnergy := fixedCodebookEnergy(c)
	ecLog2Q10 := log2Fixed(ecEnergy)
	ecDbQ10 := int16((int32(ecLog2Q10)*dbPerLog2Q13 + (1 << 12)) >> 13)
	ecBarDbQ10 := fixed.Sub(ecDbQ10, tenLog10_40Q10)

	// 3. Effective log gain in dB → log2.
	logGainDbQ10 := fixed.Sub(predicted, ecBarDbQ10)
	log2GcQ10 := (int32(logGainDbQ10)*invDbScaleQ15 + (1 << 14)) >> 15

	// 4. g_c0 = 2^log2GcQ10 in a Q14 scaling (offset input by +14·1024 so
	//    pow2Fixed's Q0 output absorbs the 2^14 factor).
	gc0Q14 := pow2Fixed(fixed.Word32(log2GcQ10) + 14*1024)
	gp, gammaC := decodeVQ(idx)
	gpQ14 = gp

	// 5. g_c = γ̂_c · g_c0. γ̂_c is Q13, gc0 is Q14 → product Q27. Shift
	//    to Q12 (>> 15) and saturate to int16.
	prod := int32(gammaC) * int32(gc0Q14) >> 15
	if prod > 32767 {
		prod = 32767
	} else if prod < -32768 {
		prod = -32768
	}
	gcQ12 = int16(prod)

	// 6. Update MA predictor FIFO with U(m) = 20·log10(γ̂_c) Q10.
	//    γ̂_c is Q13 so log2(γ̂_c) - 13 gives the value's log2. Multiply
	//    by 20·log10(2) to convert to dB.
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
