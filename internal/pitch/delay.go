package pitch

// DecodeDelaySubframe1 reconstructs the subframe-1 fractional pitch
// delay from the 8-bit P1 index, per ITU-T G.729 §3.7.1 equation (41).
//
// Returns (T_int, T_frac) with:
//
//	T_int  ∈ [19, 143]
//	T_frac ∈ {-1, 0, 1}, representing sub-sample offsets {-1/3, 0, +1/3}
//
// Encoding (eq. 41) is:
//
//	P1 = 3*(T1 − 19) + frac − 1   if T1 ∈ [19, 85], frac ∈ {-1, 0, 1}
//	P1 = (T1 − 85)   + 197         if T1 ∈ [86, 143], frac = 0
//
// Inverting: for P1 < 198, T_int = 19 + (P1+2)/3, T_frac = (P1+2)%3 − 1;
// for P1 ≥ 198, T_int = P1 − 112, T_frac = 0.
func DecodeDelaySubframe1(p1 uint8) (tInt, tFrac int) {
	if p1 < 198 {
		x := int(p1) + 2
		tInt = 19 + x/3
		tFrac = x%3 - 1
		return
	}
	tInt = int(p1) - 112
	tFrac = 0
	return
}
