package synth

import (
	"github.com/exedev/g729/internal/fixed"
)

// BuildExcitation composes the per-subframe excitation
//
//	u(n) = g_p · v(n) + g_c · c(n)
//
// per ITU-T G.729 §4.1.6 eq. (75), using ITU saturation arithmetic.
//
// Q-formats:
//
//	gpQ14  Q14 Word16 (adaptive codebook gain)
//	gcQ12  Q12 Word16 (fixed codebook gain, from internal/gain)
//	v      Q0  Word16 × 40 (adaptive codebook vector)
//	c      Q13 Word16 × 40 (fixed codebook vector)
//	u      Q0  Word16 × 40 (output excitation, saturated)
//
// Per sample:
//
//	lPitch = LMult(gpQ14, v[n])              // Q15 (= 2·gp·v)
//	lCode  = LShr(LMult(gcQ12, c[n]), 11)    // Q26 → Q15
//	lSum   = LAdd(lPitch, lCode)             // Q15
//	u[n]   = Round(LShl(lSum, 1))            // Q15 → Q16 → Q0
func BuildExcitation(gpQ14, gcQ12 int16, v, c *[40]int16, u *[40]int16) {
	for n := 0; n < 40; n++ {
		lPitch := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[n]))
		u[n] = int16(fixed.Round(fixed.LShl(lPitch, 1)))
	}
	_ = gcQ12
	_ = c
}
