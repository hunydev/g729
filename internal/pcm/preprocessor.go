package pcm

import "github.com/hunydev/g729/internal/fixed"

// FrameLength is the number of PCM samples in one G.729 frame
// (10 ms at 8 kHz).
const FrameLength = 80

// PreProcessor applies the ITU-T G.729 §3.1.1 input pre-processing:
// a second-order pole-zero high-pass filter at ~140 Hz with the 1/2
// input scaling folded into the numerator coefficients.
//
// Filter memory is kept across calls so that a 10 ms frame fed one
// frame at a time produces the same output as feeding the whole signal
// in one call. The zero value is usable and equivalent to calling
// Reset on a fresh instance.
//
// PreProcessor is not safe for concurrent use. Each channel of a
// multi-channel encoder should own a dedicated PreProcessor.
type PreProcessor struct {
	// x1, x2 are the two previous input samples (Q0 int16 widened to
	// int32 only for uniform field typing; the low 16 bits are the
	// meaningful value).
	x1, x2 int32

	// y1, y2 are the two previous output values kept in accumulator
	// precision (Q(BQ), i.e. Q13 with the default coefficient
	// Q-format) so that rounding error is not fed back into the next
	// step. The Q15 representation of the rounded output is derived
	// from these on demand.
	y1, y2 int32
}

// Reset returns the filter to its initial state (all memory zero). A
// PreProcessor should be Reset before the first call when an instance
// is being reused across independent streams.
func (p *PreProcessor) Reset() {
	*p = PreProcessor{}
}

// Process applies the pre-processing filter to in, writing the result
// to out. len(out) must be >= len(in); any extra tail in out is
// untouched. in and out may alias. Process allocates nothing.
func (p *PreProcessor) Process(in, out []int16) {
	n := len(in)
	if len(out) < n {
		n = len(out)
	}

	// Recursion (real-valued, from spec):
	//   y[n] = a1·y[n-1] + a2·y[n-2] + b0·x[n] + b1·x[n-1] + b2·x[n-2]
	//
	// Q-format analysis:
	//   - Coefficients (A1, A2, B0, B1, B2) are Q13 Word16.
	//   - x-state (x1, x2) is Q0 Word16 (held in int32 for uniform
	//     field typing).
	//   - y-state (y1, y2) is the *unrounded* accumulator value
	//     itself, kept in Q14 Word32. This is required to avoid the
	//     rounding dead-zone that a Word16 (Q0) y-state would
	//     introduce: the feedback gain (A1+A2)/2^13 = 8148/8192 is
	//     within 0.54 % of unity, so any int16 rounding of
	//     intermediate y-values creates a stable fixed point well
	//     above the DC-rejection tolerance.
	//   - fixed.LMac(acc, a, b) computes Saturate(acc + 2·a·b), so
	//     one LMac of a Q13 coefficient and a Q0 value contributes a
	//     Q(13+0+1) = Q14 term. The B-coefficient sums are therefore
	//     directly Q14-compatible with the accumulator.
	//   - For the A-coefficient feedback the multiplier is a Q14
	//     Word32 (y1, y2), so we cannot use LMac. Instead we form
	//     A·y/2^13 directly with int64 widening (no int16/int32
	//     overflow risk because the worst-case product fits in
	//     int64) and add the saturated Word32 result to acc. This
	//     keeps the accumulator-precision feedback contract while
	//     still routing the final saturation through internal/fixed.
	//
	// To convert Q14 Word32 to Q0 Word16 with rounding and saturation
	// we need an arithmetic right shift by 14 with rounding:
	//
	//     Round(LShl(acc, 2))
	//
	// because LShl(acc, 2) multiplies by 4 (Q14 → Q16) with
	// saturation, and Round(x) = ExtractH(x + 0x8000) which is a
	// right shift by 16 with rounding. Net effect: right shift by
	// 14 with rounding, saturation preserved at both steps.
	for i := 0; i < n; i++ {
		x0 := in[i]

		var acc fixed.Word32
		acc = fixed.LMac(acc, B0, x0)
		acc = fixed.LMac(acc, B1, int16(p.x1))
		acc = fixed.LMac(acc, B2, int16(p.x2))

		acc = fixed.LAdd(acc, scaleFeedback(A1, p.y1))
		acc = fixed.LAdd(acc, scaleFeedback(A2, p.y2))

		y0 := fixed.Round(fixed.LShl(acc, 2))

		out[i] = y0

		p.x2 = p.x1
		p.x1 = int32(x0)
		p.y2 = p.y1
		p.y1 = acc
	}
}

// scaleFeedback computes round(a·y / 2^BQ) with Word32 saturation. a is a
// Q13 coefficient (BQ=13), y is the previous accumulator value in Q14
// Word32, and the result is in Q14 — the same scale as the LMac-based
// B-coefficient contributions, so it can be added directly to acc.
//
// int64 widening is safe here: |a| < 2^14, |y| < 2^31, so |a·y| < 2^45
// and the final Word32 cast is range-checked. This routine intentionally
// does NOT use Go's int16/int32 arithmetic (which could wrap); the only
// non-primitive operations are int64-domain mult/shift/range-check, all
// provably overflow-free for the inputs above.
func scaleFeedback(a int16, y fixed.Word32) fixed.Word32 {
	p := (int64(a) * int64(y)) >> BQ
	if p > int64(fixed.Max32) {
		return fixed.Max32
	}
	if p < int64(fixed.Min32) {
		return fixed.Min32
	}
	return fixed.Word32(p)
}
