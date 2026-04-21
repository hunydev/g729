package pitch

// AdaptiveCodebook fills v with the 40-sample adaptive codebook
// vector for one subframe, reading from the past-excitation slice
// at integer delay tInt plus a fractional offset tFrac ∈ {-1, 0, 1}
// representing {-1/3, 0, +1/3}, per ITU-T G.729 §3.7.1 equation (40).
//
// pastExc convention:
//
//	pastExc[len(pastExc) − 1] is u(-1), the most recent past
//	sample (immediately before v[0]). For integer delay T,
//	v[n] = u(n − T) = pastExc[len − T + n]. The caller must
//	supply enough history: for fractional interpolation, at least
//	tInt + Linter samples are required where Linter = 10.
//
// When tInt < 40, the function extends the adaptive codebook by
// periodicity (Task 8): v[n] = v[n − tInt] for n ≥ tInt.
//
// Allocates nothing.
func AdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[40]int16) {
	// Integer-delay fast path. Fractional offsets handled in Task 7;
	// short-pitch periodicity in Task 8.
	if tFrac == 0 && tInt >= 40 {
		base := len(pastExc) - tInt
		for n := 0; n < 40; n++ {
			v[n] = pastExc[base+n]
		}
		return
	}
	for n := 0; n < 40; n++ {
		v[n] = 0
	}
}
