package gainquant

import (
	"testing"

	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/tables"
)

// TestApply_MantissaExponent is the IMPL-3 RED-part-A pin: for any
// (past, c, ga, gb) tuple, the encoder-side reconstruction of the
// quantized fixed-codebook gain MUST match the decoder-side
// reconstruction (gain.Decoder.Decode) bit-for-bit in the new native
// (gcMantQ14, gcExp) representation per REF-1 §2.
//
// Spec equivalence: §3.9.2 eq. (74) and §4.1.6 — encoder applies the
// chosen quantized gains, decoder reconstructs the same gains from the
// transmitted (GA, GB) indices; both arrive at the same numeric
// (g_p, g_c). With the IMPL-1/IMPL-2 (mant, exp) representation now
// the canonical g_c form, the encoder-side gainquant MUST emit the
// same triple the decoder will recompute.
//
// Invariants (REF-1 §2):
//
//	g_c (linear) = gcMantQ14 · 2^(gcExp - 14)
//	gcMantQ14 ∈ [16384, 32767]   (Q14 [1.0, 2.0))
//	gcMantQ14 = 0 ⇒ g_c = 0      (zero-energy guard path)
func TestApply_MantissaExponent(t *testing.T) {
	cases := []struct {
		name   string
		past   [4]int16
		setupC func(c *[40]int16)
		ga, gb uint8
		zero   bool
	}{
		{
			name:   "cold-start moderate",
			past:   [4]int16{gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault},
			setupC: func(c *[40]int16) { c[5] = 8192 },
			ga:     3, gb: 7,
		},
		{
			name: "voiced large gc",
			past: [4]int16{gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault},
			setupC: func(c *[40]int16) {
				c[5], c[11], c[22], c[33] = 8192, 8192, 8192, 8192
			},
			ga: 7, gb: 15,
		},
		{
			name: "low gamma_c floor",
			past: [4]int16{gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault},
			setupC: func(c *[40]int16) {
				c[5], c[11], c[22], c[33] = 8192, 8192, 8192, 8192
			},
			ga: 0, gb: 0,
		},
		{
			name: "tiny single sample",
			past: [4]int16{gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault},
			setupC: func(c *[40]int16) {
				c[0] = 1
			},
			ga: 4, gb: 9,
		},
		{
			name:   "zero-energy guard",
			past:   [4]int16{gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault, gain.PastErrorsDefault},
			setupC: func(c *[40]int16) {},
			ga:     3, gb: 7,
			zero: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Encoder-side reconstruction. Reconstruct takes physical
			// codebook indices; the test fixture (tc.ga, tc.gb) is the
			// transmitted-bit form (matches gain.Indices on the decoder
			// side) so we apply the §3.9.3 inverse map (GainImap) here.
			past := tc.past
			var cEnc [40]int16
			tc.setupC(&cEnc)
			gaPhys := tables.GainImap1[tc.ga]
			gbPhys := tables.GainImap2[tc.gb]
			gpEnc, mantEnc, expEnc := Reconstruct(&past, &cEnc, gaPhys, gbPhys)

			// Decoder-side reconstruction, fresh decoder seeded to the
			// same predictor state.
			var d gain.Decoder
			gain.SeedDecoder(&d, tc.past)
			var cDec [40]int16
			tc.setupC(&cDec)
			gpDec, mantDec, expDec := d.Decode(gain.Indices{GA: tc.ga, GB: tc.gb}, &cDec)

			if gpEnc != gpDec {
				t.Errorf("gpQ14: encoder=%d decoder=%d (must be equal)", gpEnc, gpDec)
			}
			if mantEnc != mantDec || expEnc != expDec {
				t.Errorf("(mant, exp): encoder=(%d, %d) decoder=(%d, %d) (must be equal)",
					mantEnc, expEnc, mantDec, expDec)
			}

			if tc.zero {
				if mantEnc != 0 || expEnc != 0 {
					t.Errorf("zero-energy guard: encoder=(mant=%d, exp=%d), want (0, 0)", mantEnc, expEnc)
				}
				return
			}

			if mantEnc < 16384 || mantEnc > 32767 {
				t.Errorf("mantEnc=%d not in [16384, 32767] (Q14 [1.0, 2.0))", mantEnc)
			}
		})
	}
}
