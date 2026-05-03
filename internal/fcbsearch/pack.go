package fcbsearch

// PackS encodes the 4-bit sign field S of ITU-T G.729 §3.8.2 eq. 61.
//
// The sign of pulse i (i ∈ {0,1,2,3}) is read from signs[positions[i]]
// in the §3.8.1 sign-decomposition convention (signs ∈ {−1, +1};
// produced by SignsFromD). Convention pinned to the decoder side
// (fcb.placePulses, internal/fcb/signs.go):
//
//	bit (3 − i) = 1   ↔   signs[positions[i]] > 0   (+pulse)
//	bit (3 − i) = 0   ↔   signs[positions[i]] ≤ 0   (−pulse)
//
// The bit ordering (MSB = pulse 0) matches placePulses' decoder loop,
// so PackS is its exact inverse and the round-trip is bit-exact.
//
// I3 / I4: pure (no global state, returns a value), zero allocation.
func PackS(positions *[4]int8, signs *[SubframeLen]int16) uint8 {
	var s uint8
	for i := 0; i < 4; i++ {
		if signs[positions[i]] > 0 {
			s |= 1 << (3 - uint(i))
		}
	}
	return s
}

// PackC encodes the 13-bit pulse-position field C of ITU-T G.729
// §3.8.2 eq. 62. Bit layout (MSB first, exact inverse of
// fcb.decodePositions):
//
//	bits 12..10 = i0 = pos[0] / 5            (track 0)
//	bits  9.. 7 = i1 = (pos[1] − 1) / 5      (track 1)
//	bits  6.. 4 = i2 = (pos[2] − 2) / 5      (track 2)
//	bit       3 = jx ∈ {0, 1}                (track-3 half selector)
//	bits  2.. 0 = i3 = (pos[3] − 3 − jx) / 5 (track 3a/3b)
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
	return (i0 << 10) | (i1 << 7) | (i2 << 4) | (jx << 3) | i3
}
