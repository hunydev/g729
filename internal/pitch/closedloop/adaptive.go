package closedloop

// AdaptiveVector fills v with the 40-sample adaptive-codebook
// vector v(n) for one subframe at fractional pitch delay
// intLag + frac/3, per ITU-T G.729 §A.3.7 eq. A.8 binding to
// §3.7.1 eq. (40) (G729E.txt lines 1162, 2178):
//
//	v(n) = u(n − intLag + frac/3),  n = 0..39
//
// where u is the past-excitation buffer rooted at exc[len(exc) − 1]
// = u(−1). For frac = 0 the equation degenerates to a direct
// integer copy v(n) = exc[len(exc) − intLag + n] (b30(0) = 1.0 is
// the implicit centre tap of the FIR, see frac.go). For frac = ±1
// the 1/3-sample b30 FIR is applied via the shared Interpolate3
// primitive using the same algebraic mapping documented in
// RefineFraction (refine.go):
//
//	v(n) = Interpolate3(exc, intLag − n, frac)
//
// Buffer layout. exc holds past excitation history; the caller
// optionally pre-fills exc[len(exc) − SubframeLen : len(exc)] with
// the current-subframe LP-residual extension r(0..39) so that
// short integer lags (intLag < SubframeLen) are still resolvable
// without special-casing here. AdaptiveVector itself performs no
// short-pitch periodicity replication — that would belong to a
// caller-side helper if/when needed; the encoder closed-loop search
// already uses the LP-residual extension trick.
//
// Q-format. exc and v share Q0 (Word16 excitation samples). The
// FIR is unity-gain Q15 and Round drops the low 16 bits, so the
// in/out scaling is preserved (cf. Interpolate3 godoc).
//
// I3 / I4: pure (reads exc only, writes through v), zero allocation,
// no internal state.
//
// Spec anchors: §A.3.7 eq. A.8 (G729E.txt line 2178); §3.7.1 eq.
// (40) (G729E.txt line 1162); decoder-side mirror in
// internal/pitch/adaptive.go (AdaptiveCodebook).
func AdaptiveVector(exc []int16, intLag int16, frac int8, v *[SubframeLen]int16) {
	if frac == 0 {
		base := len(exc) - int(intLag)
		for n := 0; n < SubframeLen; n++ {
			v[n] = exc[base+n]
		}
		return
	}
	for n := 0; n < SubframeLen; n++ {
		v[n] = Interpolate3(exc, intLag-int16(n), frac)
	}
}
