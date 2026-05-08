package fcbsearch

// PackS encodes the 4-bit sign field S of ITU-T G.729 §3.8.2 eq. 61.
//
// The sign of pulse i (i ∈ {0,1,2,3}) is read from signs[positions[i]]
// in the §3.8.1 sign-decomposition convention (signs ∈ {-1, +1};
// produced by SignsFromD):
//
//	S = s0 + 2*s1 + 4*s2 + 8*s3
//
// Thus bit i carries pulse i: 1 for +pulse, 0 for -pulse.
//
// I3 / I4: pure (no global state, returns a value), zero allocation.
func PackS(positions *[4]int8, signs *[SubframeLen]int16) uint8 {
	var s uint8
	for i := 0; i < 4; i++ {
		if signs[positions[i]] > 0 {
			s |= 1 << uint(i)
		}
	}
	return s
}

// PackC encodes the 13-bit pulse-position field C of ITU-T G.729
// §3.8.2 eq. 62:
//
//	C = i0 + 8*i1 + 64*i2 + 512*(2*i3 + jx)
//
// which corresponds to:
//
//	bits  2..0  = i0 = pos[0] / 5
//	bits  5..3  = i1 = (pos[1] - 1) / 5
//	bits  8..6  = i2 = (pos[2] - 2) / 5
//	bit      9  = jx in {0, 1}
//	bits 12..10 = i3 = (pos[3] - 3 - jx) / 5
//
// jx convention (matches decoder):
//
//	pos[3] ∈ {3, 8, 13, 18, 23, 28, 33, 38} → jx = 0  (track 3a)
//	pos[3] ∈ {4, 9, 14, 19, 24, 29, 34, 39} → jx = 1  (track 3b)
//
// Determined from (pos[3] − 3) mod 5: residue 0 ⇒ jx=0; residue 1
// (i.e. pos[3] mod 5 == 4) ⇒ jx=1. Caller is responsible for passing
// positions on the canonical ACELP tracks (guaranteed by
// SearchDepthFirst).
//
// I3 / I4: pure, zero allocation.
func PackC(positions *[4]int8) uint16 {
	i0 := uint16(positions[0]) / 5
	i1 := (uint16(positions[1]) - 1) / 5
	i2 := (uint16(positions[2]) - 2) / 5
	var jx uint16
	p3 := uint16(positions[3])
	if p3%5 == 4 {
		jx = 1
	}
	i3 := (p3 - 3 - jx) / 5
	return i0 | (i1 << 3) | (i2 << 6) | (jx << 9) | (i3 << 10)
}
