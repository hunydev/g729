package pitch

import "testing"

// Boundary-case table derived from ITU-T G.729 §3.7.1 equation (41):
//
//	P1 = 3*(T1 − 19) + frac − 1   for T1 ∈ [19, 85], frac ∈ {-1, 0, 1}
//	P1 = (T1 − 85) + 197          for T1 ∈ [86, 143], frac = 0
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

// Subframe-2 cases derived from ITU-T G.729 §3.7 (search-range
// derivation) and §3.7.1 equation (42):
//
//	t_min = max(20, min(int(T1) − 5, 134))   (clamped per §3.7)
//	P2    = 3*(T2_int − t_min) + frac + 2,   frac ∈ {-1, 0, 1}
//
// Inverting: y = P2 + 2; d = y/3 − 1; T2_int = t_min + d;
// T_frac = y%3 − 1. Range: d ∈ [-1, 10], T_frac ∈ {-1, 0, 1}.
//
// Note: the spec uses int(T1) (the integer part of T1, i.e.
// T1_int from DecodeDelaySubframe1) — *not* a rounded value.
func TestDecodeDelaySubframe2Center(t *testing.T) {
	// t1Int=50 → t_min=45.  P2=16 ⇒ y=18, d=5, frac=-1
	//   ⇒ T2_int=50, T2_frac=-1 (delay 49 + 2/3).
	gotInt, gotFrac := DecodeDelaySubframe2(16, 50)
	if gotInt != 50 || gotFrac != -1 {
		t.Errorf("DecodeDelaySubframe2(16, 50) = (%d, %d), want (50, -1)",
			gotInt, gotFrac)
	}
}

func TestDecodeDelaySubframe2BoundaryIndices(t *testing.T) {
	// t1Int=60 → t_min=55.
	// P2=0  ⇒ y=2, d=-1, frac=1 ⇒ (54, 1).
	gotInt, gotFrac := DecodeDelaySubframe2(0, 60)
	if gotInt != 54 || gotFrac != 1 {
		t.Errorf("DecodeDelaySubframe2(0, 60) = (%d, %d), want (54, 1)",
			gotInt, gotFrac)
	}
	// P2=31 ⇒ y=33, d=10, frac=-1 ⇒ (65, -1) (delay 64+2/3).
	gotInt, gotFrac = DecodeDelaySubframe2(31, 60)
	if gotInt != 65 || gotFrac != -1 {
		t.Errorf("DecodeDelaySubframe2(31, 60) = (%d, %d), want (65, -1)",
			gotInt, gotFrac)
	}
}

func TestDecodeDelaySubframe2LowerClamp(t *testing.T) {
	// t1Int=20 → t_min raw = 15, clamped up to 20.
	// P2=0 ⇒ d=-1, frac=1 ⇒ T_int=19. Lower bound 19 satisfied
	// implicitly by the t_min clamp (no separate output clamp).
	gotInt, gotFrac := DecodeDelaySubframe2(0, 20)
	if gotInt != 19 || gotFrac != 1 {
		t.Errorf("DecodeDelaySubframe2(0, 20) = (%d, %d), want (19, 1)",
			gotInt, gotFrac)
	}
}

func TestDecodeDelaySubframe2UpperClamp(t *testing.T) {
	// t1Int=140 → raw 135, clamped down to 134 (so t_max=143).
	// P2=31 ⇒ d=10, frac=-1 ⇒ T_int=144, frac=-1.  Note: T_int=144
	// is intentional; the encoded delay is 143+2/3, valid because
	// the FIR interpolation reaches one sample past T_int.
	gotInt, gotFrac := DecodeDelaySubframe2(31, 140)
	if gotInt != 144 || gotFrac != -1 {
		t.Errorf("DecodeDelaySubframe2(31, 140) = (%d, %d), want (144, -1)",
			gotInt, gotFrac)
	}
}

func TestDecodeDelaySubframe2RangeInvariants(t *testing.T) {
	for t1 := 19; t1 <= 143; t1++ {
		for p2 := 0; p2 < 32; p2++ {
			tInt, tFrac := DecodeDelaySubframe2(uint8(p2), t1)
			if tInt < 19 || tInt > 144 {
				t.Errorf("t1=%d, p2=%d → T_int=%d out of [19, 144]", t1, p2, tInt)
			}
			if tFrac < -1 || tFrac > 1 {
				t.Errorf("t1=%d, p2=%d → T_frac=%d out of {-1,0,1}", t1, p2, tFrac)
			}
		}
	}
}
