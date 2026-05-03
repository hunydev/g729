package closedloop

import "testing"

// TestSubframe2Window_TypicalCenter pins the §4.1.3 sliding-window
// rule for a mid-range int(T1): tmin = max(20, T1−5), tmax = tmin+9
// (G729E.txt lines 1512–1523):
//
//	tmin = int(T1) − 5
//	if tmin < 20 then tmin = 20
//	tmax = tmin + 9
//	if tmax > 143 then tmax = 143; tmin = tmax − 9
//
// For T1 = 60 the window is the full 10-lag span [55, 64].
func TestSubframe2Window_TypicalCenter(t *testing.T) {
	tmin, tmax := Subframe2Window(60)
	if tmin != 55 || tmax != 64 {
		t.Fatalf("Subframe2Window(60) = (%d,%d), want (55,64)", tmin, tmax)
	}
}

// TestSubframe2Window_LowerClamp verifies the "if tmin < 20 then tmin
// = 20" branch (G729E.txt line 1517). For T1 = 20, raw tmin = 15 is
// clamped to 20 and tmax = 29.
func TestSubframe2Window_LowerClamp(t *testing.T) {
	tmin, tmax := Subframe2Window(20)
	if tmin != 20 || tmax != 29 {
		t.Fatalf("Subframe2Window(20) = (%d,%d), want (20,29)", tmin, tmax)
	}
}

// TestSubframe2Window_LowerClampInterior covers T1 just above the
// hard floor: T1 = 24 ⇒ raw tmin = 19 → clamped to 20, tmax = 29.
func TestSubframe2Window_LowerClampInterior(t *testing.T) {
	tmin, tmax := Subframe2Window(24)
	if tmin != 20 || tmax != 29 {
		t.Fatalf("Subframe2Window(24) = (%d,%d), want (20,29)", tmin, tmax)
	}
}

// TestSubframe2Window_LowerEdgeOff verifies T1 = 25 ⇒ tmin = 20 (no
// clamp triggered yet by the spec, since 25−5 = 20).
func TestSubframe2Window_LowerEdgeOff(t *testing.T) {
	tmin, tmax := Subframe2Window(25)
	if tmin != 20 || tmax != 29 {
		t.Fatalf("Subframe2Window(25) = (%d,%d), want (20,29)", tmin, tmax)
	}
}

// TestSubframe2Window_UpperClamp verifies the "if tmax > 143" branch
// (G729E.txt lines 1519–1522). For T1 = 143, raw (tmin,tmax) =
// (138,147); the upper clamp pulls tmax back to 143 and slides
// tmin to 134.
func TestSubframe2Window_UpperClamp(t *testing.T) {
	tmin, tmax := Subframe2Window(143)
	if tmin != 134 || tmax != 143 {
		t.Fatalf("Subframe2Window(143) = (%d,%d), want (134,143)", tmin, tmax)
	}
}

// TestSubframe2Window_UpperEdgeOff verifies T1 = 139 sits exactly at
// the boundary where no upper clamp is needed: tmin = 134, tmax = 143.
func TestSubframe2Window_UpperEdgeOff(t *testing.T) {
	tmin, tmax := Subframe2Window(139)
	if tmin != 134 || tmax != 143 {
		t.Fatalf("Subframe2Window(139) = (%d,%d), want (134,143)", tmin, tmax)
	}
}

// TestSubframe2Window_WindowAlwaysTenLags asserts the structural
// invariant: tmax − tmin == 9 across the entire valid integer range
// [PitchMinInt, PitchMaxInt]. This is the §4.1.3 contract that
// makes the 5-bit P2 field (32 codepoints / 3 fractions ≈ 10 integer
// lags) sufficient.
func TestSubframe2Window_WindowAlwaysTenLags(t *testing.T) {
	for T1 := int16(PitchMinInt); T1 <= int16(PitchMaxInt); T1++ {
		tmin, tmax := Subframe2Window(T1)
		if tmin < PitchMinInt || tmax > PitchMaxInt {
			t.Fatalf("Subframe2Window(%d) = (%d,%d) out of [%d,%d]",
				T1, tmin, tmax, PitchMinInt, PitchMaxInt)
		}
		if tmax-tmin != 9 {
			t.Fatalf("Subframe2Window(%d) width = %d, want 9", T1, tmax-tmin)
		}
	}
}

// TestSearchInteger_Sub0UsesCenterPlusMinus3 pins the CL-1 contract
// reaffirmed by CL-2: with sub = 0 (subframe-1 / open-loop centre),
// the search window is [centre−3, centre+3]. Plant a unit impulse at
// centre+3 and verify it wins.
func TestSearchInteger_Sub0UsesCenterPlusMinus3(t *testing.T) {
	const centre = 60
	const k0 = centre + 3
	var xb [SubframeLen]int16
	for n := 0; n < SubframeLen; n++ {
		xb[n] = int16(400 - 10*n)
	}
	exc := make([]int16, testExcLen)
	exc[testExcLen-SubframeLen-k0] = 1

	intLag, _ := SearchInteger(&xb, exc, centre, 0)
	if intLag != k0 {
		t.Fatalf("sub=0 SearchInteger intLag = %d, want %d", intLag, k0)
	}
}

// TestSearchInteger_Sub0WindowExcludesFar verifies that sub=0 does
// NOT see lags more than 3 away from centre: an impulse at centre+5
// sits outside the window so the search must pick a different lag.
func TestSearchInteger_Sub0WindowExcludesFar(t *testing.T) {
	const centre = 60
	const k0 = centre + 5 // outside [57,63]
	var xb [SubframeLen]int16
	for n := 0; n < SubframeLen; n++ {
		xb[n] = int16(400 - 10*n)
	}
	exc := make([]int16, testExcLen)
	exc[testExcLen-SubframeLen-k0] = 1

	intLag, RNbest := SearchInteger(&xb, exc, centre, 0)
	if RNbest != 0 {
		t.Fatalf("sub=0 with impulse outside window: RNbest = %d, want 0", RNbest)
	}
	if intLag < 57 || intLag > 63 {
		t.Fatalf("sub=0 intLag = %d, want within [57,63]", intLag)
	}
}

// TestSearchInteger_Sub1UsesSubframe2Window pins the §4.1.3 routing:
// with sub = 1 the search window is exactly Subframe2Window(centre).
// For centre = int(T1) = 60 the window is [55, 64]; plant an impulse
// at 64 (5 away — outside the sub=0 ±3 window but inside the §4.1.3
// 10-lag window) and verify it wins.
func TestSearchInteger_Sub1UsesSubframe2Window(t *testing.T) {
	const centre = 60
	const k0 = 64 // tmax of Subframe2Window(60); outside ±3 of centre.
	var xb [SubframeLen]int16
	for n := 0; n < SubframeLen; n++ {
		xb[n] = int16(400 - 10*n)
	}
	exc := make([]int16, testExcLen)
	exc[testExcLen-SubframeLen-k0] = 1

	intLag, RNbest := SearchInteger(&xb, exc, centre, 1)
	if intLag != k0 {
		t.Fatalf("sub=1 SearchInteger intLag = %d, want %d (Subframe2Window upper)",
			intLag, k0)
	}
	if RNbest == 0 {
		t.Fatalf("sub=1 RNbest = 0, want non-zero (impulse inside window)")
	}
}

// TestSearchInteger_Sub1LowerEdgeIncluded plants an impulse at
// centre−5 (= tmin) for centre = 60 to confirm the lower bound of
// the §4.1.3 window is inclusive.
func TestSearchInteger_Sub1LowerEdgeIncluded(t *testing.T) {
	const centre = 60
	const k0 = 55 // tmin of Subframe2Window(60).
	var xb [SubframeLen]int16
	xb[0] = 1 // unique non-zero target sample at n=0.
	exc := make([]int16, testExcLen)
	exc[testExcLen-SubframeLen-k0] = 1

	intLag, _ := SearchInteger(&xb, exc, centre, 1)
	if intLag != k0 {
		t.Fatalf("sub=1 SearchInteger intLag = %d, want %d (lower edge)",
			intLag, k0)
	}
}

// TestSearchInteger_Sub1UpperClamp verifies that for centre = T1 =
// 143 (max), sub=1 honours the §4.1.3 upper-clamp window [134, 143],
// not centre ±3. Impulse at 134 should win against a flat window.
func TestSearchInteger_Sub1UpperClamp(t *testing.T) {
	const centre = 143
	const k0 = 134
	var xb [SubframeLen]int16
	xb[0] = 1
	exc := make([]int16, testExcLen)
	exc[testExcLen-SubframeLen-k0] = 1

	intLag, _ := SearchInteger(&xb, exc, centre, 1)
	if intLag != k0 {
		t.Fatalf("sub=1 centre=143 intLag = %d, want %d (slid window)",
			intLag, k0)
	}
}

// TestSubframe2Window_ZeroAlloc enforces I4 for the window helper.
func TestSubframe2Window_ZeroAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(64, func() {
		_, _ = Subframe2Window(60)
	})
	if allocs != 0 {
		t.Fatalf("Subframe2Window allocs/op = %v, want 0", allocs)
	}
}

// TestSearchInteger_Sub1ZeroAlloc enforces I4 for the sub=1 path
// which now routes through Subframe2Window.
func TestSearchInteger_Sub1ZeroAlloc(t *testing.T) {
	var xb [SubframeLen]int16
	for n := range xb {
		xb[n] = int16(n - 20)
	}
	exc := make([]int16, testExcLen)
	for i := range exc {
		exc[i] = int16((i*7 + 3) % 41)
	}
	allocs := testing.AllocsPerRun(64, func() {
		_, _ = SearchInteger(&xb, exc, 60, 1)
	})
	if allocs != 0 {
		t.Fatalf("SearchInteger sub=1 allocs/op = %v, want 0", allocs)
	}
}
