package decoder

import "github.com/hunydev/g729/internal/fixed"

// Output HP filter coefficients per ITU-T G.729 §4.2.2. A 2-pole 2-zero
// IIR at 100 Hz cutoff.
//
// Real-valued (from spec):
//
//	H(z) = (b0 + b1·z⁻¹ + b2·z⁻²) / (1 + a1·z⁻¹ + a2·z⁻²)
//	b0 = +0.93980581, b1 = -1.8795834, b2 = +0.93980581
//	a1 = -1.9330735,  a2 = +0.93589199
//
// Fixed-point: b0/b1/b2 at Q13. Feedback uses the G.729 32x16 multiply
// form: y-state is stored as the native Q16 accumulator and multiplied by
// a1/a2 Q13 coefficients via high/low decomposition.
const (
	hpB0Q13 = 7699
	hpB1Q13 = -15398
	hpB2Q13 = 7699

	hpFeedbackA1Q13 = 15836
	hpFeedbackA2Q13 = -7667

	// Legacy diagnostic constants retained for older phase tests that inline
	// the previous Q12 feedback approximation.
	hpNegA1Q12 = 7918
	hpA2Q13    = 7667
)

// hpFilter applies the §4.2.2 output HP filter to in (40 samples) and
// writes the 40 HP-filtered samples to out. Advances d.hpX, d.hpY.
//
// out may alias in.
func (d *Decoder) hpFilter(in *[subframeLen]int16, out []int16) {
	d.hpFilterCore(in, out, nil, nil)
}

// hpFilterFinal applies the output HP filter and emits the final decoder PCM
// domain directly from the HP accumulator. Rounding after the final ×2 shift
// preserves the half-LSB that would be lost by rounding HP output first and
// doubling the rounded sample.
func (d *Decoder) hpFilterFinal(in *[subframeLen]int16, out []int16) {
	d.hpFilterCore(in, nil, out, nil)
}

func (d *Decoder) hpFilterWithPreAndFinal(in *[subframeLen]int16, pre, final []int16) {
	d.hpFilterCore(in, pre, final, nil)
}

type hpFilterTaps struct {
	Sample [subframeLen]hpFilterSampleTaps
}

type hpFilterSampleTaps struct {
	InputPostfilter int16

	X1Before int16
	X2Before int16
	Y1Before int32
	Y2Before int32

	Y1HiBefore int16
	Y1LoBefore int16
	Y2HiBefore int16
	Y2LoBefore int16

	Feedforward int32
	Feedback    int32
	Total       int32

	OutputPreScale int16
	OutputPCM      int16

	X1After int16
	X2After int16
	Y1After int32
	Y2After int32

	Y1HiAfter int16
	Y1LoAfter int16
	Y2HiAfter int16
	Y2LoAfter int16
}

func (d *Decoder) hpFilterCore(in *[subframeLen]int16, pre, final []int16, taps *hpFilterTaps) {
	x1 := d.hpX[0]
	x2 := d.hpX[1]
	y1 := d.hpY[0]
	y2 := d.hpY[1]

	for n := 0; n < subframeLen; n++ {
		xn := in[n]
		x1Before := x1
		x2Before := x2
		y1Before := y1
		y2Before := y2
		y1HiBefore, y1LoBefore := hpLExtract(y1Before)
		y2HiBefore, y2LoBefore := hpLExtract(y2Before)

		ff := fixed.LMult(xn, hpB0Q13)
		ff = fixed.LMac(ff, x1, hpB1Q13)
		ff = fixed.LMac(ff, x2, hpB2Q13)

		fb := fixed.LAdd(
			hpMpy32_16(y1, hpFeedbackA1Q13),
			hpMpy32_16(y2, hpFeedbackA2Q13),
		)
		acc := fixed.LShl(fixed.LAdd(ff, fb), 2)
		preScale := fixed.Round(acc)
		pcm := hpFinalFromAccNative(acc)

		if pre != nil {
			pre[n] = preScale
		}
		if final != nil {
			final[n] = pcm
		}

		x2 = x1
		x1 = xn
		y2 = y1
		y1 = int32(acc)
		y1HiAfter, y1LoAfter := hpLExtract(y1)
		y2HiAfter, y2LoAfter := hpLExtract(y2)
		if taps != nil {
			taps.Sample[n] = hpFilterSampleTaps{
				InputPostfilter: xn,
				X1Before:        x1Before,
				X2Before:        x2Before,
				Y1Before:        y1Before,
				Y2Before:        y2Before,
				Y1HiBefore:      y1HiBefore,
				Y1LoBefore:      y1LoBefore,
				Y2HiBefore:      y2HiBefore,
				Y2LoBefore:      y2LoBefore,
				Feedforward:     ff,
				Feedback:        fb,
				Total:           acc,
				OutputPreScale:  preScale,
				OutputPCM:       pcm,
				X1After:         x1,
				X2After:         x2,
				Y1After:         y1,
				Y2After:         y2,
				Y1HiAfter:       y1HiAfter,
				Y1LoAfter:       y1LoAfter,
				Y2HiAfter:       y2HiAfter,
				Y2LoAfter:       y2LoAfter,
			}
		}
	}

	d.hpX[0] = x1
	d.hpX[1] = x2
	d.hpY[0] = y1
	d.hpY[1] = y2
}

func hpFinalFromAccNative(acc int32) int16 {
	return fixed.Round(fixed.LShl(acc, 1))
}

func hpMpy32_16(x int32, n int16) int32 {
	hi, lo := hpLExtract(x)
	return fixed.LMac(fixed.LMult(hi, n), fixed.Mult(lo, n), 1)
}

func hpLExtract(x int32) (hi, lo int16) {
	hi = fixed.ExtractH(x)
	lo = fixed.ExtractL(fixed.LMsu(fixed.LShr(x, 1), hi, 16384))
	return hi, lo
}
