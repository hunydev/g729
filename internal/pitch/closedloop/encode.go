package closedloop

import "github.com/hunydev/g729/internal/pitch"

// EncodeP1 packs the subframe-1 fractional pitch lag (intLag, frac)
// into the 8-bit codeword P1 per ITU-T G.729 §3.7.2 eq. (41)
// (G729E.txt lines 1170–1175):
//
//	P1 = 3·(int(T1) − 19) + frac − 1   if P1 < 198 after packing
//	P1 = (int(T1) − 85) + 197          for the integer-only upper range
//
// The decoder inverse owns P1=198 as (86,0), so (85,+1) has no exact
// codepoint. Normalize that unrepresentable upper fractional edge to the
// nearest integer codepoint (85,0) instead of silently emitting P1=198 with
// a different decoded lag.
func EncodeP1(intLag int16, frac int8) uint8 {
	if intLag < 85 || (intLag == 85 && frac <= 0) {
		return uint8(3*(intLag-19) + int16(frac) - 1)
	}
	return uint8(intLag + 112)
}

// EncodeP2 packs the subframe-2 fractional pitch lag (intT2, frac)
// relative to tmin into the 5-bit codeword P2 per ITU-T G.729
// §3.7.2 eq. (42) (G729E.txt lines 1177–1180):
//
//	P2 = 3·(int(T2) − tmin) + frac + 2,   frac ∈ {-1, 0, 1}
//
// tmin is derived from the subframe-1 integer lag via §4.1.3 lines
// 1512–1518 (see Subframe2Window). The caller is responsible for
// ensuring (intT2, frac, tmin) lies inside the 10-lag P2 window so
// that the resulting P2 fits in 5 bits.
func EncodeP2(intT2 int16, frac int8, tmin int16) uint8 {
	return uint8(3*(intT2-tmin) + int16(frac) + 2)
}

// EncodeP0 returns the 1-bit parity P0 over the six MSBs of P1 per
// ITU-T G.729 §3.7.2 (G729E.txt lines 1182–1185). Delegates to the
// shared pitch.Parity helper so encoder and decoder agree on the
// odd-parity convention exercised by pitch.CheckParity.
func EncodeP0(p1 uint8) uint8 {
	return pitch.Parity(p1)
}
