package synth

import (
	"github.com/exedev/g729/internal/fixed"
)

// BuildExcitation composes the per-subframe excitation
//
// u(n) = g_p · v(n) + g_c · c(n)
//
// per ITU-T G.729 §4.1.6 eq. (75), using ITU saturation arithmetic.
//
// Q-formats:
//
// gpQ14      Q14 Word16 (adaptive codebook gain)
// gcMantQ14  Q14 Word16 mantissa of fixed codebook gain
// gcExp      int8        binary exponent of fixed codebook gain
// v          Q0  Word16 × 40 (adaptive codebook vector)
// c          Q13 Word16 × 40 (fixed codebook vector)
// u          Q0  Word16 × 40 (output excitation, saturated)
//
// Linear g_c = gcMantQ14 · 2^(gcExp - 14). gcMantQ14 == 0 ⇒ g_c = 0
// (zero-energy short-circuit; gcExp is then ignored).
//
// # Per-sample shift derivation
//
// Let M_lin = gcMantQ14 / 2^14 (Q14 fractional value of mantissa) and
// C_lin = c[n] / 2^13 (Q13 fractional value of code sample). Then
//
// prod32 = LMult(gcMantQ14, c[n]) = 2 · gcMantQ14 · c[n]
//
//	= M_lin · C_lin · 2^28              (Q28 representation)
//
// The full code-half contribution is g_c · c[n] = M_lin · 2^gcExp · C_lin
// because g_c = (gcMantQ14 / 2^14) · 2^gcExp. Writing this as a Q15
// integer (matching lPitch's Q15 from LMult(gpQ14 [Q14], v[n] [Q0])):
//
// target = g_c · c[n] · 2^15
//
//	= M_lin · C_lin · 2^(gcExp + 15)
//	= prod32 · 2^(gcExp + 15 - 28)
//	= prod32 · 2^(gcExp - 13)
//	= prod32 >> (13 - gcExp)
//
// Therefore shift_r = 13 - int(gcExp).
//
//   - gcExp ≤ 13 ⇒ shift_r ≥ 0 ⇒ arithmetic right shift (LShr).
//     For shift_r ≥ 31 this naturally collapses to 0 (LShr semantics).
//   - gcExp > 13 ⇒ shift_r < 0 ⇒ saturating left shift (LShl, which
//     saturates per ITU). The saturation envelope (gcExp ≥ 28 fully
//     saturates a max prod32) bounds the rare extreme-gain case.
//
// Sanity check at gcExp=0, gcMantQ14=16384 (g_c=1.0), c[n]=8192 (1.0):
//
// prod32 = 2·16384·8192 = 2^28
// shift_r = 13 ⇒ lCode = 2^28 >> 13 = 2^15 = 32768 (Q15 of 1.0). ✓
// u[n]   = Round(LShl(32768, 1)) = Round(2^16) = 1.            ✓
//
// Cross-check vs the prior single-Q12 path: old code did
// LShr(LMult(gcQ12, c[n]), 11) (Q26 → Q15). That equals the new code
// when gcQ12 = gcMantQ14 / 4 with gcExp = 0 (and equivalent reparam-
// etrizations), as expected.
//
// Per sample:
//
// lPitch = LMult(gpQ14, v[n])               // Q15
// prod32 = LMult(gcMantQ14, c[n])            // Q28 (skipped if mant=0)
// lCode  = LShr(prod32, 13-gcExp)            // gcExp ≤ 13
//
//	LShl(prod32, gcExp-13) (sat)      // gcExp > 13
//
// lSum   = LAdd(lPitch, lCode)               // Q15
// u[n]   = Round(LShl(lSum, 1))              // Q15 → Q16 → Q0 sat
func BuildExcitation(gpQ14, gcMantQ14 int16, gcExp int8, v, c *[40]int16, u *[40]int16) {
	shiftR := 13 - int(gcExp)
	for n := 0; n < 40; n++ {
		lPitch := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[n]))
		var lCode fixed.Word32
		if gcMantQ14 != 0 {
			prod32 := fixed.LMult(fixed.Word16(gcMantQ14), fixed.Word16(c[n]))
			if shiftR >= 0 {
				lCode = fixed.LShr(prod32, fixed.Word16(shiftR))
			} else {
				lCode = fixed.LShl(prod32, fixed.Word16(-shiftR))
			}
		}
		lSum := fixed.LAdd(lPitch, lCode)
		u[n] = int16(fixed.Round(fixed.LShl(lSum, 1)))
	}
}
