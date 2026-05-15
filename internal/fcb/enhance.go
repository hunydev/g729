package fcb

import "github.com/hunydev/g729/internal/fixed"

// Pitch enhancement filter coefficient endpoints (Q14), per ITU-T
// G.729 §3.8 equation (47): β = ĝ_p^(m-1) bounded near 0.2..0.8.
// The upper endpoint is pinned by the decoder stage oracle; using
// round(0.8*2^14)=13107 over-sharpens full-scale pulses by 45 Q13 units.
const (
	betaLowerQ14 = 3277 // round(0.2 · 2^14)
	betaUpperQ14 = 13017

	// InitialPitchEnhancementQ14 is the stream-start pitch sharpening
	// coefficient used before any previous decoded pitch gain exists.
	InitialPitchEnhancementQ14 = betaLowerQ14
)

// ClampPitchGainForEnhancement returns the pitch enhancement filter
// coefficient β_Q14, derived from the previous subframe's decoded
// pitch gain gpPrevQ14 by clamping to the fixed-point bounds used by
// ITU-T G.729 §3.8 equation (47). Negative inputs clamp to the lower bound.
func ClampPitchGainForEnhancement(gpPrevQ14 int16) int16 {
	if gpPrevQ14 < betaLowerQ14 {
		return betaLowerQ14
	}
	if gpPrevQ14 > betaUpperQ14 {
		return betaUpperQ14
	}
	return gpPrevQ14
}

// ApplyPitchEnhancement runs the in-place IIR pitch enhancement
// filter c'(n) = c(n) + β·c'(n−T) for n = T..39, per ITU-T G.729
// §3.8 equation (46), applied only when the integer pitch lag T is
// less than the subframe size 40 (eq. 48).
//
// Q-format chain (β is Q14, c is Q13):
//
// prod32 = LMult(βQ14, c[n-t])     →  Q28 (LMult doubles)
// prod32 = LShl(prod32, 1)          →  Q29
// delta  = ExtractH(prod32)         →  Q13
// c[n]   = Add(c[n], delta)         →  Q13 (saturating)
//
// In-place update means c[n-t] for n in [t..39] is the post-filtered
// (cascaded) value, giving the correct IIR behaviour.
func ApplyPitchEnhancement(c *[40]int16, t int, betaQ14 int16) {
	if t < 1 || t >= 40 {
		return
	}
	if betaQ14 == 0 {
		return
	}
	bQ14 := fixed.Word16(betaQ14)
	for n := t; n < 40; n++ {
		prod := fixed.LMult(bQ14, fixed.Word16(c[n-t]))
		prod = fixed.LShl(prod, 1)
		delta := fixed.ExtractH(prod)
		c[n] = int16(fixed.Add(fixed.Word16(c[n]), delta))
	}
}
