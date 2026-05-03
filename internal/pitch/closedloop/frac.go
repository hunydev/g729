package closedloop

import (
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// linter is the one-sided length of the 1/3-sample b30 interpolation
// FIR per ITU-T G.729 §3.7.1 line 1158 ("Linter = 10").
const linter = 10

// Interpolate3 returns one sample of the past excitation u
// interpolated at fractional delay (intLag + frac/3), per ITU-T
// G.729 §3.7.1 equation (40) (G729E.txt line 1162):
//
//	v(0) = Σ_{i=0..9} u(−k − i)·b30(t + 3i)
//	     + Σ_{i=0..9} u(−k + 1 + i)·b30(3 − t + 3i)
//
// where (k, t) is derived from (intLag, frac):
//
//	frac =  0 → integer copy: returns u(−intLag) directly.
//	frac = +1 → k = intLag,     t = 1.
//	frac = −1 → k = intLag − 1, t = 2.
//
// Buffer convention: u(0) is anchored at u[len(u) − SubframeLen],
// so u(−1) = u[len(u) − SubframeLen − 1] and u(−d) maps to
// u[len(u) − SubframeLen − d] for any d ≥ 1. The trailing
// SubframeLen samples u[len(u) − SubframeLen : len(u)] hold the
// LP-residual extension u(0..39) per §A.3.7 line 2161, supporting
// the short-pitch case where intLag − n < 0 (i.e. the fractional
// FIR reaches into u(0)+). Out-of-range reads (when the FIR window
// extends past either end of u) are treated as zero, matching the
// §3.7.1 "samples u(0+) ... are treated as 0" boundary handling
// for the still-shorter cases.
//
// The FIR coefficient table b30 is shared with the decoder side via
// internal/tables.PitchInterpFIR; only one transcription of the
// normative numerical values exists in the codebase. The indexing
// convention PitchInterpFIR[t + 3*i] = b30(t + 3i) for i ∈ [0, 9]
// and t ∈ {0, 1, 2} is the one documented in
// internal/tables/pitch_interp.go.
//
// Q-format: u and the returned sample share the caller's Q-format
// (the FIR is unity-gain Q15 and Round drops the low 16 bits, so
// in/out scaling is preserved).
//
// I3 / I4: pure, zero allocation, no state. The function is a single
// 20-tap MAC chain over fixed.LMac with bound-checked reads.
//
// Spec anchors: §3.7.1 eq. (40) (G729E.txt line 1162); b30 symmetry
// statement (G729E.txt lines 1165–1167); §A.3.7 eq. A.8 binding
// reference (G729E.txt line 2176, "see clause 3.7.1").
func Interpolate3(u []int16, intLag int16, frac int8) int16 {
	N := len(u)
	anchor := N - SubframeLen
	if frac == 0 {
		// Center tap b30(0) = 1.0 ⇒ direct copy. The b30 table omits
		// this implicit unity tap.
		return u[anchor-int(intLag)]
	}

	var k, posPhase, negPhase int
	if frac > 0 {
		k = int(intLag)
		posPhase, negPhase = 1, 2
	} else {
		k = int(intLag) - 1
		posPhase, negPhase = 2, 1
	}

	base := anchor - k
	fir := tables.PitchInterpFIR
	var acc fixed.Word32
	for i := 0; i < linter; i++ {
		backIdx := base - i
		fwdIdx := base + 1 + i
		var back, fwd int16
		if backIdx >= 0 && backIdx < N {
			back = u[backIdx]
		}
		if fwdIdx >= 0 && fwdIdx < N {
			fwd = u[fwdIdx]
		}
		acc = fixed.LMac(acc, fir[posPhase+3*i], back)
		acc = fixed.LMac(acc, fir[negPhase+3*i], fwd)
	}
	return fixed.Round(acc)
}
