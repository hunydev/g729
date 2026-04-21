package pitch

import "testing"

// Boundary-case table derived from ITU-T G.729 §3.7.1 equation (41):
//
//   P1 = 3*(T1 − 19) + frac − 1   for T1 ∈ [19, 85], frac ∈ {-1, 0, 1}
//   P1 = (T1 − 85) + 197          for T1 ∈ [86, 143], frac = 0
//
// Inverting the first branch: T_int = 19 + (P1+2)/3, T_frac = (P1+2)%3 − 1.
// The first valid (T1, frac) pair is (19, +1) at P1=0 because the smallest
// usable delay is 19+1/3 (negative-frac slots at T1=19 are unreachable).
var subframe1Cases = []struct {
	p1       uint8
	wantInt  int
	wantFrac int
}{
	{0, 19, 1},
	{1, 20, -1},
	{2, 20, 0},
	{3, 20, 1},
	{4, 21, -1},
	{197, 85, 0},
	{198, 86, 0},
	{199, 87, 0},
	{255, 143, 0},
}

func TestDecodeDelaySubframe1Boundaries(t *testing.T) {
	for _, tc := range subframe1Cases {
		gotInt, gotFrac := DecodeDelaySubframe1(tc.p1)
		if gotInt != tc.wantInt || gotFrac != tc.wantFrac {
			t.Errorf("DecodeDelaySubframe1(%d) = (%d, %d), want (%d, %d)",
				tc.p1, gotInt, gotFrac, tc.wantInt, tc.wantFrac)
		}
	}
}

func TestDecodeDelaySubframe1RangeInvariants(t *testing.T) {
	for p1 := 0; p1 < 256; p1++ {
		tInt, tFrac := DecodeDelaySubframe1(uint8(p1))
		if tInt < 19 || tInt > 143 {
			t.Errorf("p1=%d → T_int=%d, out of [19, 143]", p1, tInt)
		}
		if tFrac < -1 || tFrac > 1 {
			t.Errorf("p1=%d → T_frac=%d, out of {-1, 0, 1}", p1, tFrac)
		}
	}
}
