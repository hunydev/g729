package gain

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// GainDecodeFullTaps captures the unsaturated 32-bit intermediates of
// the gain decoder for Phase 3a DIAG-1 corpus localization. Test-only
// — exists in a _test.go file so no production API surface is added.
//
// Field Q-formats and references (ITU-T G.729 §3.9 / §4.1.6):
//
//   - Predicted        Q10 dB — Ê(m) per eq. (69), int32 to avoid
//     intermediate Word16 saturation before g_c reconstruction.
//   - EcBarDbQ10       Q10 dB — E̅_c = 10·log10(Σc²/40) per eq. (66)
//   - LogGainDbQ10     Q10 dB — Ê(m) − E̅_c, the log-gain target
//   - Log2GcQ10        Q10 in log2 units — LogGainDbQ10 / (20·log10 2)
//   - Gc0Q14Unsat      g_c0 = 2^Log2Gc, expressed at Q14 as int32
//     WITHOUT clamping to int16. This is the "natural" predicted-gain
//     magnitude before the codebook-correction multiply.
//   - Gc0MantQ14       fractional Pow2 mantissa of g_c0 at Q14, before
//     applying the binary exponent.
//   - GammaCQ13        γ̂_c at Q13 from the conjugate-structure VQ
//   - ProdUnsat        γ̂_c · g_c0 ≫ 15  at Q12, int32 WITHOUT clamp
//   - GpQ14Final       g_p Q14 (post-VQ; matches Decode return)
//   - GcQ12Final       g_c Q12 saturated to int16, retained only for
//     back-compat with phase3a_diag1_gc_taps numbers.
//   - GcMantQ14        g_c mantissa Q14 ∈ [16384, 32767] (or 0 on
//     zero-energy guard). Matches Decode's new return per REF-1 §2.
//   - GcExp            g_c binary exponent (int8). Linear g_c =
//     GcMantQ14 · 2^(GcExp - 14).
//   - PastErrorsBefore gain MA predictor FIFO before the subframe update.
//   - PastErrorsAfter  gain MA predictor FIFO after the subframe update.
//   - UCurrent         current predictor update U(m), Q10 dB.
//   - ZeroEnergyGuard  true when the Σc²==0 short-circuit fired
type GainDecodeFullTaps struct {
	Predicted        int32
	EcBarDbQ10       int32
	LogGainDbQ10     int32
	Log2GcQ10        int32
	Gc0Q14Unsat      int32
	Gc0MantQ14       int16
	GammaCQ13        int32
	ProdUnsat        int32
	GpQ14Final       int16
	GcQ12Final       int16
	GcMantQ14        int16
	GcExp            int8
	PastErrorsBefore [4]int16
	PastErrorsAfter  [4]int16
	UCurrent         int16
	ZeroEnergyGuard  bool
}

// pow2FixedAsInt32 mirrors pow2Fixed (pow2.go) but returns the result
// as a non-saturating int32 computed via int64 intermediates. Same
// table-lookup interpolation as the production form; only the upper
// "shift > 16" early-clamp is removed so callers that want to observe
// natural pre-clamp magnitude (DIAG-1) can do so.
//
// Q-format CONTRACT mirrors pow2Fixed: input `x` is Q10, output is
// 2^(x/1024) at Q0; pre-add k·1024 to obtain a Qk-scaled result.
func pow2FixedAsInt32(x fixed.Word32) int32 {
	intPart := int32(x) >> 10
	frac := int32(x) - (intPart << 10)

	idx := frac >> 5
	a := frac & 0x1F
	t0 := int32(tables.Pow2Table[idx])
	t1 := int32(tables.Pow2Table[idx+1])
	fracQ14 := t0 + ((t1-t0)*a)>>5

	shift := intPart - 14
	switch {
	case shift >= 0:
		v := int64(fracQ14) << uint(shift)
		if v > int64(int32(0x7FFFFFFF)) {
			return int32(0x7FFFFFFF)
		}
		if v < int64(int32(-0x80000000)) {
			return int32(-0x80000000)
		}
		return int32(v)
	default:
		s := -shift
		if s >= 31 {
			return 0
		}
		return fracQ14 >> uint(s)
	}
}

// DecodeWithFullTaps mirrors (*Decoder).Decode (decode.go) line-for-line
// but additionally returns the unsaturated 32-bit intermediates required
// by Phase 3a DIAG-1. The MA-predictor FIFO is updated identically to
// Decode so that the per-subframe state evolves the same way and the
// returned (GpQ14Final, GcQ12Final) match what Decode would have
// produced on the same input.
//
// Test-only: this is a one-for-one substitute for Decode in the taps
// pathway. Do NOT call both Decode and DecodeWithFullTaps on the same
// subframe — the predictor would be double-advanced.
func (d *Decoder) DecodeWithFullTaps(idx Indices, c *[40]int16) GainDecodeFullTaps {
	var out GainDecodeFullTaps

	if !d.initialized {
		for i := range d.pastErrors {
			d.pastErrors[i] = pastErrorsDefault
		}
		d.initialized = true
	}
	out.PastErrorsBefore = d.pastErrors

	ecEnergy := fixedCodebookEnergy(c)

	if ecEnergy <= 0 {
		gp, gammaC := decodeVQ(idx)
		out.ZeroEnergyGuard = true
		out.Predicted = d.predictedLogGain()
		out.GammaCQ13 = gammaC
		out.GpQ14Final = gp
		out.GcQ12Final = 0
		out.GcMantQ14 = 0
		out.GcExp = 0
		out.UCurrent = pastErrorsDefault
		d.pastErrors[3] = d.pastErrors[2]
		d.pastErrors[2] = d.pastErrors[1]
		d.pastErrors[1] = d.pastErrors[0]
		d.pastErrors[0] = pastErrorsDefault
		out.PastErrorsAfter = d.pastErrors
		return out
	}

	predicted := d.predictedLogGain()
	out.Predicted = predicted

	ecLog2Q10 := int32(log2Fixed(ecEnergy)) - 26*1024
	ecDbQ10 := (ecLog2Q10*dbPerLog2Q13 + (1 << 12)) >> 13
	ecBarDbQ10 := ecDbQ10 - int32(tenLog10_40Q10)
	out.EcBarDbQ10 = ecBarDbQ10

	logGainDbQ10 := predicted - int32(ecBarDbQ10)
	out.LogGainDbQ10 = logGainDbQ10
	log2GcQ15 := logGainToLog2Q15(logGainDbQ10)
	log2GcQ10 := log2GcQ15 >> 5
	out.Log2GcQ10 = log2GcQ10

	intPart := log2GcQ15 >> 15
	fracQ15 := log2GcQ15 - (intPart << 15)
	gc0MantQ14 := pow2FracQ14FromQ15(fracQ15)
	out.Gc0MantQ14 = gc0MantQ14

	gc0Unsat64 := int64(gc0MantQ14)
	if intPart >= 0 {
		if intPart >= 31 {
			gc0Unsat64 = int64(int32(0x7FFFFFFF))
		} else {
			gc0Unsat64 <<= uint(intPart)
		}
	} else {
		shift := -intPart
		if shift >= 63 {
			gc0Unsat64 = 0
		} else {
			gc0Unsat64 >>= uint(shift)
		}
	}
	if gc0Unsat64 > int64(int32(0x7FFFFFFF)) {
		gc0Unsat64 = int64(int32(0x7FFFFFFF))
	}
	out.Gc0Q14Unsat = int32(gc0Unsat64)

	gp, gammaC := decodeVQ(idx)
	out.GpQ14Final = gp
	out.GammaCQ13 = gammaC

	gainQ14 := fixedGainQ14FromLog2Gamma(log2GcQ15, gammaC)
	prod64 := gainQ14 >> 2
	var prodUnsat int32
	switch {
	case prod64 > int64(int32(0x7FFFFFFF)):
		prodUnsat = int32(0x7FFFFFFF)
	case prod64 < int64(int32(-0x80000000)):
		prodUnsat = int32(-0x80000000)
	default:
		prodUnsat = int32(prod64)
	}
	out.ProdUnsat = prodUnsat

	prod := prodUnsat
	if prod > 32767 {
		prod = 32767
	} else if prod < -32768 {
		prod = -32768
	}
	out.GcQ12Final = int16(prod)

	if gammaC <= 0 {
		out.GcMantQ14 = 0
		out.GcExp = 0
	} else {
		out.GcMantQ14, out.GcExp = splitGainQ14(gainQ14)
	}

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
	out.UCurrent = uCurrent
	d.pastErrors[3] = d.pastErrors[2]
	d.pastErrors[2] = d.pastErrors[1]
	d.pastErrors[1] = d.pastErrors[0]
	d.pastErrors[0] = uCurrent
	out.PastErrorsAfter = d.pastErrors

	return out
}
