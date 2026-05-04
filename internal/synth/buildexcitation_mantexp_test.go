package synth

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
)

// TestBuildExcitation_MantExp_Cases pins the new (mantissa Q14, exponent
// int8) g_c contract for synth.BuildExcitation per Phase 3a REF-1 §2.
//
// Reference algorithm per sample (verified by derivation in
// excitation.go doc block):
//
//	lPitch  = LMult(gpQ14, v[n])               // Q15
//	prod32  = LMult(gcMantQ14, c[n])            // Q28 (Q14·Q13·2)
//	shift_r = 13 - int(gcExp)                   // align to Q15
//	if shift_r >= 0: lCode = LShr(prod32, shift_r)
//	else:            lCode = LShl(prod32, -shift_r) (saturating)
//	lSum    = LAdd(lPitch, lCode)               // Q15
//	u[n]    = Round(LShl(lSum, 1))              // Q0 saturated
//
// Cases below independently compute the expected u[] from this reference.
func TestBuildExcitation_MantExp_Cases(t *testing.T) {
	type tc struct {
		name string
		gp   int16
		mant int16
		exp  int8
		v    [40]int16
		c    [40]int16
	}

	mkPulse := func(idx int, val int16) [40]int16 {
		var a [40]int16
		a[idx] = val
		return a
	}

	cases := []tc{
		// Equivalence to legacy: mant=16384, exp=0 → g_c = 1.0 (== gcQ12 4096).
		// Old reference behavior: u[5] = round(g_c * c[5]) with c[5]=8192 (Q13 = 1.0) → 1.
		{name: "legacy-equiv g_c=1.0 single pulse", gp: 0, mant: 16384, exp: 0, c: mkPulse(5, 8192)},
		{name: "legacy-equiv g_c=1.0 negative pulse", gp: 0, mant: 16384, exp: 0, c: mkPulse(5, -8192)},
		// Large g_c: gcExp=5, mant=20000 → g_c = 20000 * 2^-9 ≈ 39.0625.
		// With c[5]=8192 (1.0): contribution ≈ 39.
		{name: "large g_c gcExp=5", gp: 0, mant: 20000, exp: 5, c: mkPulse(5, 8192)},
		// Tiny g_c: gcExp=-5, mant=20000 → g_c = 20000 * 2^-19 ≈ 0.0381.
		// With c[5]=8192 (1.0): ≈ 0–1 magnitude.
		{name: "tiny g_c gcExp=-5", gp: 0, mant: 20000, exp: -5, c: mkPulse(5, 8192)},
		// Zero mantissa: lCode contribution must be exactly 0.
		{name: "zero mantissa with nonzero exp", gp: 16384, mant: 0, exp: 7, v: mkPulse(3, 1234), c: mkPulse(5, 8192)},
		// Saturating left-shift: gcExp=30 → shift_r = 13-30 = -17 → left-shift 17, saturates.
		{name: "saturating left-shift gcExp=30", gp: 0, mant: 16384, exp: 30, c: mkPulse(5, 8192)},
		// Mixed pitch + code: g_p=0.5 (Q14=8192), g_c=0.5 (mant=8192, exp=0); v[i]=200 c[i]=8192.
		{name: "mixed pitch+code", gp: 8192, mant: 8192, exp: 0,
			v: func() [40]int16 {
				var a [40]int16
				for i := range a {
					a[i] = 200
				}
				return a
			}(),
			c: func() [40]int16 {
				var a [40]int16
				for i := range a {
					a[i] = 8192
				}
				return a
			}(),
		},
	}

	for _, ca := range cases {
		ca := ca
		t.Run(ca.name, func(t *testing.T) {
			var u [40]int16
			BuildExcitation(ca.gp, ca.mant, ca.exp, &ca.v, &ca.c, &u)

			// Compute reference independently.
			var want [40]int16
			for n := 0; n < 40; n++ {
				lPitch := fixed.LMult(fixed.Word16(ca.gp), fixed.Word16(ca.v[n]))
				var lCode fixed.Word32
				if ca.mant != 0 {
					prod32 := fixed.LMult(fixed.Word16(ca.mant), fixed.Word16(ca.c[n]))
					shiftR := 13 - int(ca.exp)
					if shiftR >= 0 {
						lCode = fixed.LShr(prod32, fixed.Word16(shiftR))
					} else {
						lCode = fixed.LShl(prod32, fixed.Word16(-shiftR))
					}
				}
				lSum := fixed.LAdd(lPitch, lCode)
				want[n] = int16(fixed.Round(fixed.LShl(lSum, 1)))
			}

			for i := range u {
				if u[i] != want[i] {
					t.Errorf("u[%d] = %d, want %d", i, u[i], want[i])
				}
			}

			// Spot-check magnitudes for clarity (regression locks).
			switch ca.name {
			case "legacy-equiv g_c=1.0 single pulse":
				if u[5] != 1 {
					t.Errorf("u[5] = %d, want 1 (g_c=1.0 · 1.0)", u[5])
				}
			case "legacy-equiv g_c=1.0 negative pulse":
				if u[5] != -1 {
					t.Errorf("u[5] = %d, want -1", u[5])
				}
			case "large g_c gcExp=5":
				if u[5] < 38 || u[5] > 40 {
					t.Errorf("u[5] = %d, want ≈ 39", u[5])
				}
			case "tiny g_c gcExp=-5":
				if u[5] < -1 || u[5] > 1 {
					t.Errorf("u[5] = %d, want in [-1,1]", u[5])
				}
			case "zero mantissa with nonzero exp":
				// pitch-only contribution at index 3: gp=1.0, v=1234 → ~1234.
				if u[3] < 1230 || u[3] > 1240 {
					t.Errorf("u[3] = %d, want ≈1234 (pitch-only)", u[3])
				}
				// no code contribution at index 5 (would otherwise saturate).
				// pitch contribution at idx 5 is 0 since v[5]=0.
				if u[5] != 0 {
					t.Errorf("u[5] = %d, want 0 (mant=0 ⇒ lCode=0)", u[5])
				}
			case "saturating left-shift gcExp=30":
				if u[5] != 32767 {
					t.Errorf("u[5] = %d, want INT16_MAX (saturated)", u[5])
				}
			}
		})
	}
}
