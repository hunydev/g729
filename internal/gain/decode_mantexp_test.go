package gain

import (
	"math"
	"testing"
)

// TestDecode_MantissaExponent exercises the new (mantissa Q14,
// exponent int8) g_c return per REF-1
// (docs/superpowers/plans/2026-05-04-phase3a-gcrep-design.md §2):
//
//	g_c (linear) = gcMantQ14 · 2^(gcExp - 14)
//	gcMantQ14 ∈ [16384, 32767]  (or 0 for zero-energy guard)
//
// Each case asserts: invariants on (mant, exp), an order-of-magnitude
// bracket on linear g_c, and that g_p stays in plausible Q14 range.
func TestDecode_MantissaExponent(t *testing.T) {
	type wantBracket struct {
		minLinear float64 // inclusive
		maxLinear float64 // exclusive
		minExp    int     // inclusive
		maxExp    int     // inclusive
	}
	cases := []struct {
		name    string
		idx     Indices
		setupC  func(c *[40]int16)
		zero    bool
		bracket wantBracket
	}{
		{
			// Cold-start moderate: single high-amplitude pulse, mid-table γ̂_c.
			// Predicted log-gain on cold start is the all-default tap line
			// (-14 dB Q10 ×4) producing a moderate g_c ≈ O(1).
			name:   "cold-start moderate",
			idx:    Indices{GA: 3, GB: 7},
			setupC: func(c *[40]int16) { c[5] = 8192 },
			bracket: wantBracket{
				minLinear: 0.05, maxLinear: 16.0,
				minExp: -6, maxExp: 4,
			},
		},
		{
			// Voiced large g_c: high-energy 4-pulse codebook + an idx whose
			// γ̂_c is at the high end of the conjugate-structure VQ table.
			// (GA=7, GB=15) maps to a near-maximum γ̂_c (Q13 close to 2.0).
			name: "voiced large gc",
			idx:  Indices{GA: 7, GB: 15},
			setupC: func(c *[40]int16) {
				c[5], c[11], c[22], c[33] = 8192, 8192, 8192, 8192
			},
			bracket: wantBracket{
				minLinear: 4.0, maxLinear: 256.0,
				minExp: 1, maxExp: 9,
			},
		},
		{
			// Low γ̂_c floor: high-energy 4-pulse codebook with the
			// smallest γ̂_c VQ entry (GA=0, GB=0). With the cold-start
			// predictor (-14 dB Q10 tap line) the codebook-energy
			// correction limits how low g_c can go; this case pins the
			// low-γ̂_c floor of the conjugate-structure VQ rather than a
			// pathological tiny gain.
			name: "low gamma_c floor",
			idx:  Indices{GA: 0, GB: 0},
			setupC: func(c *[40]int16) {
				c[5], c[11], c[22], c[33] = 8192, 8192, 8192, 8192
			},
			bracket: wantBracket{
				minLinear: 0.0, maxLinear: 64.0,
				minExp: -15, maxExp: 6,
			},
		},
		{
			// Zero-energy guard: c is all-zero, mant=0 and exp=0 by
			// contract (REF-1 §2 invariant); g_p still decoded.
			name:   "zero-energy guard",
			idx:    Indices{GA: 3, GB: 7},
			setupC: func(c *[40]int16) {},
			zero:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d Decoder
			var c [40]int16
			tc.setupC(&c)

			gpQ14, mant, exp := d.Decode(tc.idx, &c)

			if gpQ14 < 0 || gpQ14 > 32767 {
				t.Errorf("gpQ14=%d not in [0, 32767]", gpQ14)
			}

			if tc.zero {
				if mant != 0 || exp != 0 {
					t.Errorf("zero-energy guard: got (mant=%d, exp=%d), want (0, 0)", mant, exp)
				}
				return
			}

			if mant < 16384 || mant > 32767 {
				t.Errorf("mant=%d not in [16384, 32767] (Q14 [1.0, 2.0))", mant)
			}
			if int(exp) < tc.bracket.minExp || int(exp) > tc.bracket.maxExp {
				t.Errorf("exp=%d not in expected bracket [%d, %d]",
					exp, tc.bracket.minExp, tc.bracket.maxExp)
			}
			linear := float64(mant) * math.Exp2(float64(exp)-14.0)
			if linear < tc.bracket.minLinear || linear >= tc.bracket.maxLinear {
				t.Errorf("g_c linear=%g not in [%g, %g)",
					linear, tc.bracket.minLinear, tc.bracket.maxLinear)
			}
		})
	}
}

// TestDecode_FullTapsMatchesDecode pins that DecodeWithFullTaps and
// Decode produce numerically equivalent g_c reconstructions on the same
// input and predictor state. The full-taps shim must expose the new
// (mantissa, exponent) representation in addition to the legacy Q12
// scalar for back-compat with the phase3a_diag1_gc_taps test.
func TestDecode_FullTapsMatchesDecode(t *testing.T) {
	type sample struct {
		name   string
		idx    Indices
		setupC func(c *[40]int16)
	}
	samples := []sample{
		{"single pulse", Indices{GA: 3, GB: 7}, func(c *[40]int16) { c[5] = 8192 }},
		{"four pulses", Indices{GA: 5, GB: 9}, func(c *[40]int16) {
			c[5], c[11], c[22], c[33] = 8192, 8192, 8192, 8192
		}},
		{"low energy", Indices{GA: 0, GB: 0}, func(c *[40]int16) { c[0] = 1 }},
		{"zero energy", Indices{GA: 3, GB: 7}, func(c *[40]int16) {}},
	}
	for _, s := range samples {
		t.Run(s.name, func(t *testing.T) {
			var d1, d2 Decoder
			var c1, c2 [40]int16
			s.setupC(&c1)
			s.setupC(&c2)

			gp1, mant, exp := d1.Decode(s.idx, &c1)
			taps := d2.DecodeWithFullTaps(s.idx, &c2)

			if gp1 != taps.GpQ14Final {
				t.Errorf("gpQ14: Decode=%d FullTaps=%d", gp1, taps.GpQ14Final)
			}
			if mant != taps.GcMantQ14 || exp != taps.GcExp {
				t.Errorf("(mant, exp): Decode=(%d, %d) FullTaps=(%d, %d)",
					mant, exp, taps.GcMantQ14, taps.GcExp)
			}
		})
	}
}
