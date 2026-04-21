package fcb

// Pitch enhancement filter coefficient endpoints (Q14), per ITU-T
// G.729 §3.8 equation (47): β = ĝ_p^(m-1) bounded by 0.2 ≤ β ≤ 0.8.
const (
	betaLowerQ14 = 3277  // round(0.2 · 2^14)
	betaUpperQ14 = 13107 // round(0.8 · 2^14)
)

// ClampPitchGainForEnhancement returns the pitch enhancement filter
// coefficient β_Q14, derived from the previous subframe's decoded
// pitch gain gpPrevQ14 by clamping to [0.2, 0.8] per ITU-T G.729
// §3.8 equation (47). Negative inputs clamp to the lower bound.
func ClampPitchGainForEnhancement(gpPrevQ14 int16) int16 {
	if gpPrevQ14 < betaLowerQ14 {
		return betaLowerQ14
	}
	if gpPrevQ14 > betaUpperQ14 {
		return betaUpperQ14
	}
	return gpPrevQ14
}
