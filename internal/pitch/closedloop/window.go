package closedloop

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
// 5-bit relative P2 encoding (Table 8: 32 P2 codepoints map to 10
// integer lags × 3 fractions ≈ 30 admissible (intT2, frac) pairs;
// the surplus codepoints are absorbed by the encoding scheme). The
// encoder MUST produce P2 such that the decoder recovers (intT2,
// frac) inside this window — i.e. the closed-loop search itself must
// be confined to the same window. The function is therefore shared
// between CL-2 (search-window dispatch in SearchInteger) and the
// future ENC-1 P2 packing.
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
