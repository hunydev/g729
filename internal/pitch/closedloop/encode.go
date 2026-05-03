package closedloop

import "github.com/exedev/g729/internal/pitch"

// EncodeP1 packs the subframe-1 fractional pitch lag (intLag, frac)
// into the 8-bit codeword P1 per ITU-T G.729 §3.7.2 eq. (41)
// (G729E.txt lines 1170–1175):
//
//	P1 = 3·(int(T1) − 19) + frac − 1   if T1 ∈ [19,…,85], frac ∈ {-1,0,1}
//	P1 = (int(T1) − 85) + 197          if T1 ∈ [86,…,143], frac = 0
//
// The fractional branch is taken for intLag ≤ 85 (any frac); the
// integer branch for intLag ≥ 86. The boundary codepoint P1=198 is
// owned by the fractional branch (intLag=85, frac=1) per the
// decoder inverse §4.1.3 (G729E.txt lines 1505–1510), which routes
// every P1 < 199 through the fractional reconstruction.
func EncodeP1(intLag int16, frac int8) uint8 {
	if intLag <= 85 {
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
