package gain

import "github.com/exedev/g729/internal/fixed"

// fixedCodebookEnergy returns the inner sum Σ c[n]² as a non-negative
// Word32.
//
// Q-format: the caller's c is Q13 pulse amplitudes (±8192 for ITU Annex A
// ACELP). Each squared term is therefore Q26, and the 40-term sum is at
// Q26 as well. The sum is always non-negative and bounded above by
// 40·8192² = 2^28·40/1 < 2^31, so no saturation can occur.
//
// Returning the raw sum at Q26 (rather than pre-applying the /40 of
// spec eq (66)) preserves precision for the downstream log2 conversion.
// The 10·log10(1/40) correction is absorbed into the compile-time
// constant tenLog10_40Q10 applied in decode.go.
//
// Callers must ALSO apply a Q-format correction to account for the
// Q26-vs-Q0 mismatch against the spec's log2 of a Q0 sum: see the
// comment in decode.go at the `ecLog2Q10 = ... - 26*1024` line.
func fixedCodebookEnergy(c *[40]int16) fixed.Word32 {
	var acc fixed.Word32
	for n := 0; n < 40; n++ {
		// LMult returns 2·a·b in Q1; LShr by 1 recovers a·b in Q0.
		acc = fixed.LAdd(acc, fixed.LShr(fixed.LMult(fixed.Word16(c[n]), fixed.Word16(c[n])), 1))
	}
	return acc
}

// FixedCodebookEnergy is the exported form of fixedCodebookEnergy used by
// the encoder-side gain predictor (internal/gainquant). Returns Σ c[n]²
// at Q26 for c at Q13. See the unexported form's contract for the full
// Q-format walk and downstream correction requirements.
func FixedCodebookEnergy(c *[40]int16) fixed.Word32 {
	return fixedCodebookEnergy(c)
}
