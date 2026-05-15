package pitch

import (
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// Linter is the one-sided length of the 1/3-sample pitch
// interpolation FIR per ITU-T G.729 §3.7.1 (Linter = 10).
const Linter = 10

// AdaptiveCodebook fills v with the 40-sample adaptive codebook
// vector for one subframe, reading from the past-excitation slice
// at integer delay tInt plus a fractional offset tFrac ∈ {-1, 0, 1}
// representing {-1/3, 0, +1/3}, per ITU-T G.729 §3.7.1 equation (40):
//
//	v(n) = Σ_{i=0..9} u(n − k − i)·b30(t + 3i)
//	     + Σ_{i=0..9} u(n − k + 1 + i)·b30(3 − t + 3i)
//
// where t ∈ {0,1,2} interpolates the source sample u(n-k+t/3).  The
// decoded pitch delay is T = tInt + tFrac/3, and v(n)=u(n-T), so
// (k,t) is derived as:
//
//	tFrac =  0:  k = tInt,     t = 0
//	tFrac = -1:  k = tInt,     t = 1
//	tFrac = +1:  k = tInt + 1, t = 2
//
// pastExc convention:
//
//	pastExc[len(pastExc) − 1] is u(-1), the most recent past sample.
//	For integer delay T, v[n] = pastExc[len − T + n]. The caller
//	must supply enough history for the FIR taps (the simplest
//	sufficient condition for the fractional case is
//	tInt ≥ 40 + Linter).
//
// When the interpolation window reaches into the current subframe, equation
// (40)'s u(0..n-1) references are evaluated from the adaptive vector samples
// already generated in this call; future current-subframe references remain
// zero.
//
// Allocates nothing.
func AdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[40]int16) {
	if tInt <= 40+Linter-1 {
		firInterpolateRecursiveCurrent(tInt, tFrac, pastExc, v)
		return
	}

	if tInt >= 40 {
		firInterpolate(tInt, tFrac, pastExc, v, 0, 40)
		return
	}

	if tFrac == 0 {
		firInterpolateRecursiveCurrent(tInt, tFrac, pastExc, v)
	} else {
		firInterpolateRecursiveCurrent(tInt, tFrac, pastExc, v)
	}
}

// firInterpolate fills v[start:end] from pastExc using the §3.7.1
// 1/3-sample interpolation FIR.
func firInterpolate(tInt, tFrac int, pastExc []int16, v *[40]int16, start, end int) {
	var k, posPhase, negPhase int
	if tFrac == 0 {
		k = tInt
		posPhase, negPhase = 0, 3
	} else if tFrac < 0 {
		k = tInt
		posPhase, negPhase = 1, 2
	} else {
		k = tInt + 1
		posPhase, negPhase = 2, 1
	}

	base := len(pastExc) - k
	fir := tables.PitchInterpFIR
	N := len(pastExc)
	for n := start; n < end; n++ {
		var acc fixed.Word32
		for i := 0; i < Linter; i++ {
			// Per §3.7.1: at the time the adaptive codebook is
			// constructed for the current subframe, samples u(0+)
			// have not yet been computed and are treated as 0.
			// Symmetrically, samples older than the buffer are 0.
			backIdx := base + n - i
			fwdIdx := base + n + 1 + i
			var back, fwd int16
			if backIdx >= 0 && backIdx < N {
				back = pastExc[backIdx]
			}
			if fwdIdx >= 0 && fwdIdx < N {
				fwd = pastExc[fwdIdx]
			}
			acc = fixed.LMac(acc, fir[posPhase+3*i], back)
			acc = fixed.LMac(acc, fir[negPhase+3*i], fwd)
		}
		v[n] = fixed.Round(acc)
	}
}

// firInterpolateRecursiveCurrent handles the short-pitch case. Eq. (40) can
// reference u(n-k+1+i) inside the current subframe; those samples are available
// only when they have already been generated.
func firInterpolateRecursiveCurrent(tInt, tFrac int, pastExc []int16, v *[40]int16) {
	var k, posPhase, negPhase int
	if tFrac == 0 {
		k = tInt
		posPhase, negPhase = 0, 3
	} else if tFrac < 0 {
		k = tInt
		posPhase, negPhase = 1, 2
	} else {
		k = tInt + 1
		posPhase, negPhase = 2, 1
	}

	fir := tables.PitchInterpFIR
	for n := 0; n < 40; n++ {
		var acc fixed.Word32
		for i := 0; i < Linter; i++ {
			back := adaptiveSource(n-k-i, pastExc, v)
			fwd := adaptiveSource(n-k+1+i, pastExc, v)
			acc = fixed.LMac(acc, fir[posPhase+3*i], back)
			acc = fixed.LMac(acc, fir[negPhase+3*i], fwd)
		}
		v[n] = fixed.Round(acc)
	}
}

func adaptiveSource(relative int, pastExc []int16, v *[40]int16) int16 {
	if relative < 0 {
		idx := len(pastExc) + relative
		if idx >= 0 && idx < len(pastExc) {
			return pastExc[idx]
		}
		return 0
	}
	if relative < 40 {
		return v[relative]
	}
	return 0
}
