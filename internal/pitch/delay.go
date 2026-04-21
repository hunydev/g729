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

// DecodeDelaySubframe2 reconstructs the subframe-2 fractional pitch
// delay from the 5-bit P2 index (relative to the integer part of T1),
// per ITU-T G.729 §3.7 (search-range derivation) and §3.7.1
// equation (42).
//
// t1Int is the integer part of T1, i.e. the T_int returned by
// DecodeDelaySubframe1 (the spec uses int(T1), not a rounded value).
//
// Search range derivation (§3.7):
//
//	t_min = int(T1) − 5
//	if t_min < 20         then t_min = 20
//	if t_min > 134        then t_min = 134        (equivalent to t_max ≤ 143)
//
// Encoding (eq. 42): P2 = 3*(T2_int − t_min) + frac + 2,
// frac ∈ {-1, 0, 1}.  Decoded inverse:
//
//	y     = P2 + 2
//	d     = y/3 − 1   (Euclidean division; d ∈ [-1, 10])
//	T_int = t_min + d
//	T_frac = y%3 − 1
//
// T_int may legitimately equal t_max+1 = 144 (with T_frac = -1) at
// the upper bound of the search window — that encoded delay
// (t_max + 2/3) is valid, and downstream FIR interpolation will
// access past_exc[n − T_int − i] within the buffer the caller
// provides.  No output clamping is applied.
func DecodeDelaySubframe2(p2 uint8, t1Int int) (tInt, tFrac int) {
	tMin := t1Int - 5
	if tMin < 20 {
		tMin = 20
	} else if tMin > 134 {
		tMin = 134
	}
	y := int(p2) + 2
	tInt = tMin + y/3 - 1
	tFrac = y%3 - 1
	return
}
