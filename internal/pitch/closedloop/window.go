package closedloop

// Subframe1Window returns the integer-lag search window [tmin, tmax]
// used for the first subframe's closed-loop pitch search around the
// open-loop pitch Top per ITU-T G.729 §3.7 / Annex A §A.3.7:
//
//	tmin = Top − 3
//	if tmin < 20 then tmin = 20
//	tmax = tmin + 6
//	if tmax > 143 then
//	    tmax = 143
//	    tmin = tmax − 6
//	end
//
// The returned width tmax − tmin is always exactly 6 (7 lags).
func Subframe1Window(top int16) (tmin, tmax int16) {
	tmin = top - 3
	if tmin < PitchMinInt {
		tmin = PitchMinInt
	}
	tmax = tmin + 6
	if tmax > PitchMaxInt {
		tmax = PitchMaxInt
		tmin = tmax - 6
	}
	return tmin, tmax
}

// Subframe2Window returns the integer-lag search window [tmin, tmax]
// used for the second subframe's closed-loop pitch search, sliding
// around the integer part of the first-subframe lag T1 per ITU-T
// G.729 §4.1.3 (G729E.txt lines 1512–1523):
//
//	tmin = int(T1) − 5
//	if tmin < 20 then tmin = 20
//	tmax = tmin + 9
//	if tmax > 143 then
//	    tmax = 143
//	    tmin = tmax − 9
//	end
//
// The decoder section §4.1.3 is the canonical specification of the
// 5-bit relative P2 encoding. The integer search itself uses this 10-lag
// window; the P2 bit field also contains the two fractional boundary
// codepoints (tmin-1,+1) and (tmax+1,-1), corresponding to the full
// [tmin-2/3, tmax+2/3] P2 fractional span.
//
// The returned width tmax − tmin is always exactly 9 (10 lags), the
// invariant exploited by the 5-bit P2 field.
//
// I3 / I4: pure, zero allocation, no state.
func Subframe2Window(intT1 int16) (tmin, tmax int16) {
	tmin = intT1 - 5
	if tmin < PitchMinInt {
		tmin = PitchMinInt
	}
	tmax = tmin + 9
	if tmax > PitchMaxInt {
		tmax = PitchMaxInt
		tmin = tmax - 9
	}
	return tmin, tmax
}
