package decoder

// Output HP filter coefficients per ITU-T G.729 §4.2.2. A 2-pole 2-zero
// IIR at 100 Hz cutoff.
//
// Real-valued (from spec):
//
//	H(z) = (b0 + b1·z⁻¹ + b2·z⁻²) / (1 + a1·z⁻¹ + a2·z⁻²)
//	b0 = +0.93980581, b1 = -1.8795834, b2 = +0.93980581
//	a1 = -1.9330735,  a2 = +0.93589199
//
// Fixed-point: b0/b1/b2 at Q13; |a1| at Q12 (because |a1|>1); a2 at Q13.
// x-state int16 Q0; y-state int32 Q12 (rounding-dead-zone avoidance).
const (
	hpB0Q13    = 7699
	hpB1Q13    = -15399
	hpB2Q13    = 7699
	hpNegA1Q12 = 7918
	hpA2Q13    = 7667
)

// hpFilter applies the §4.2.2 output HP filter to in (40 samples) and
// writes the 40 HP-filtered samples to out. Advances d.hpX, d.hpY.
//
// out may alias in.
func (d *Decoder) hpFilter(in *[subframeLen]int16, out []int16) {
	x1 := d.hpX[0]
	x2 := d.hpX[1]
	y1 := d.hpY[0]
	y2 := d.hpY[1]

	for n := 0; n < subframeLen; n++ {
		xn := in[n]

		ff := int32(hpB0Q13)*int32(xn) +
			int32(hpB1Q13)*int32(x1) +
			int32(hpB2Q13)*int32(x2) // Q13
		ff >>= 1 // Q12

		fb := int64(hpNegA1Q12) * int64(y1) // Q24
		fb >>= 12
		fb -= (int64(hpA2Q13) * int64(y2)) >> 13

		acc := int64(ff) + fb // Q12

		yn := (acc + (1 << 11)) >> 12
		if yn > 32767 {
			yn = 32767
		} else if yn < -32768 {
			yn = -32768
		}

		out[n] = int16(yn)

		x2 = x1
		x1 = xn
		y2 = y1
		y1 = int32(acc)
	}

	d.hpX[0] = x1
	d.hpX[1] = x2
	d.hpY[0] = y1
	d.hpY[1] = y2
}
