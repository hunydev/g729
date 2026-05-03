package closedloop

import "github.com/exedev/g729/internal/fixed"

// GpUpperQ14 is the Q14 representation of the §3.7.3 eq. 43 upper
// bound 1.2 on the adaptive-codebook gain Gp:
//
//	round(1.2 · 2^14) = round(19660.8) = 19661.
//
// OQ-GBOUND resolution: §3.7.3 eq. 43 (G729E.txt line ~1192) writes
// "bounded by 0 ≤ gp ≤ 1.2" using inclusive ≤ on both ends, so the
// upper bound is reachable. Q14 was chosen as the canonical scale
// for adaptive-codebook gain in the encoder (matches the eventual
// quantiser interface in §3.9 / §A.3.9 — confirmed by the fact that
// Annex A's gain quantiser uses Q14 throughout).
const GpUpperQ14 int16 = 19661

// GpAndY computes the filtered adaptive-codebook vector y(n) and the
// adaptive-codebook gain Gp per ITU-T G.729 §3.7.3 (G729E.txt lines
// 1186–1199):
//
//	eq. 44:  y(n) = Σ_{i=0..n} v(i)·h(n−i),         n = 0,...,39
//	eq. 43:  Gp   = Σ x(n)·y(n) / Σ y(n)·y(n),       0 ≤ Gp ≤ 1.2
//
// The convolution in eq. 44 is the zero-state response of the
// weighted synthesis filter 1/Â(z/γ) (whose impulse response is h)
// to the adaptive-codebook vector v.
//
// Q-format. x is Q0 (TG-1 target convention) and v is Q0 (VP-1
// excitation convention); h is Q12 (HI-1 impulse-response
// convention). The eq. 44 product v(i)·h(n−i) accumulates in Q12
// and is arithmetically right-shifted by 12 with Word16 saturation
// to land y in Q0, mirroring BackwardFilter's xb scaling so the
// downstream eq. 43 numerator and denominator share a common scale.
// The returned Gp is in Q14, already clamped to [0, GpUpperQ14] =
// [0, 1.2].
//
// Numerical guards.
//   - Σ y² = 0 (zero-energy y, e.g. v ≡ 0): return Gp = 0 instead
//     of dividing by zero.
//   - Σ x·y ≤ 0 (anti-phase or orthogonal): return Gp = 0 to honour
//     the lower bound 0 ≤ gp.
//   - num/den ≥ 1.2: return GpUpperQ14 (inclusive cap).
//
// I3 / I4: pure (writes only through y), zero allocation.
func GpAndY(x, v, h, y *[SubframeLen]int16) (gp int16) {
	for n := 0; n < SubframeLen; n++ {
		var acc int32
		for i := 0; i <= n; i++ {
			acc += int32(v[i]) * int32(h[n-i])
		}
		y[n] = fixed.Saturate(acc >> 12)
	}

	var num, den int64
	for n := 0; n < SubframeLen; n++ {
		yn := int64(y[n])
		num += int64(x[n]) * yn
		den += yn * yn
	}

	if den <= 0 || num <= 0 {
		return 0
	}
	// Compare num/den to 1.2 = 6/5 without losing precision.
	if num*5 >= den*6 {
		return GpUpperQ14
	}
	return int16((num << 14) / den)
}
