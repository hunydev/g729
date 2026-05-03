package openloop

import "testing"

// fillSquareWave fills wsp with a square wave of the given period and
// amplitude ±A over the full 223-sample window. For an integer period
// P, R(k=P) attains the per-lag maximum of eq. A.4 over the decimated
// 40-tap inner product.
func fillSquareWave(wsp *[223]int16, period int, amp int16) {
	half := period / 2
	for i := range wsp {
		if (i % period) < half {
			wsp[i] = amp
		} else {
			wsp[i] = -amp
		}
	}
}

// TestPickBestInRange_Range20_Period25 pins the full-stride scan over
// the §A.3.4 third delay region [20,39]. A period-25 square wave makes
// R(25) the unique max of eq. A.4 in that range; constant magnitude
// makes E(k) constant so the eq. A.5 score order matches R(k) order.
func TestPickBestInRange_Range20_Period25(t *testing.T) {
	var wsp [223]int16
	fillSquareWave(&wsp, 25, 1024)
	lag, _, _ := pickBestInRange(&wsp, 20, 39)
	if lag != 25 {
		t.Fatalf("pickBestInRange[20,39] period-25 lag = %d, want 25", lag)
	}
}

// TestPickBestInRange_Range40_Period64 pins the full-stride scan over
// the §A.3.4 second delay region [40,79]. Period-64 → R(64) max.
func TestPickBestInRange_Range40_Period64(t *testing.T) {
	var wsp [223]int16
	fillSquareWave(&wsp, 64, 1024)
	lag, _, _ := pickBestInRange(&wsp, 40, 79)
	if lag != 64 {
		t.Fatalf("pickBestInRange[40,79] period-64 lag = %d, want 64", lag)
	}
}

// TestPickBestInRange_Range80_Period110 pins the §A.3.4 first delay
// region [80,143] even-first scan: a period-110 square wave makes
// R(110) the unique even-pass winner; the ±1 refinement around 110
// confirms 110 (R(109), R(111) both strictly less than R(110) due to
// the 1-sample phase shift).
func TestPickBestInRange_Range80_Period110(t *testing.T) {
	var wsp [223]int16
	fillSquareWave(&wsp, 110, 1024)
	lag, _, _ := pickBestInRange(&wsp, 80, 143)
	if lag != 110 {
		t.Fatalf("pickBestInRange[80,143] period-110 lag = %d, want 110", lag)
	}
}

// TestPickBestInRange_Range80_OddRefinement pins the §A.3.4 lines
// 2113-2114 ±1 refinement: when the best even lag in [80,143] is 110
// but R(109)·E(110) ties (or beats) R(110)·E(109) under the eq. A.5
// score, the function must visit 109 (not just stop at the even-pass
// winner) and return 109 by the lower-lag tie-break rule (§A.3.4 line
// 2110).
//
// Construction. With wsp constant 1024 at:
//   - current frame even-stride positions  {143,145,…,221}  (sw(2n)=1024)
//   - history odd indices                  {33,35,…,111}     (drives R(110))
//   - history even indices                 {34,36,…,112}     (drives R(109))
//
// the eq. A.4 sums give R(110) = R(109) = 40·1024² and the eq. A.5
// energies give E(110) = E(109) = 40·1024², so scores tie. Adjacent
// even lags 108 and 112 only see 39 of the required 40 history taps
// (one boundary tap missing) so R(108) = R(112) = 39·1024² < R(110),
// confirming 110 as the unique even-pass winner. Adjacent odd lag 111
// likewise sees only 39 even-history taps so R(111) = 39·1024² <
// R(109). Refinement therefore returns 109.
func TestPickBestInRange_Range80_OddRefinement(t *testing.T) {
	var wsp [223]int16
	for n := 0; n < 40; n++ {
		wsp[143+2*n] = 1024 // sw(2n) current frame
		wsp[33+2*n] = 1024  // history odd, fills reads for k=110
		wsp[34+2*n] = 1024  // history even, fills reads for k=109
	}
	lag, _, _ := pickBestInRange(&wsp, 80, 143)
	if lag != 109 {
		t.Fatalf("pickBestInRange[80,143] odd-refinement lag = %d, want 109", lag)
	}
}

// TestPickBestInRange_Range80_EvenOnlyScan pins that odd lags in
// [80,143] OUTSIDE the ±1 neighborhood of the even-pass winner are NOT
// visited (§A.3.4 line 2113 "only the correlations at the even delays
// are computed in the first pass").
//
// Construction. wsp is non-zero only at:
//   - current frame even-stride positions  {143,145,…,221}  = 1024
//   - history even indices                 {48,50,…,126}    = 1024
//
// With history odd indices all zero, every EVEN k in [80,143] reads
// odd-indexed history → R(k) = E(k) = 0 → score 0 for all evens. The
// even-pass tie-breaks to the lowest even lag k=80. Odd lag 95 has
// R(95) = E(95) = 40·1024² (perfect alignment) — the global max. But
// the ±1 refinement around 80 only visits {79→clamp 80, 80, 81}, never
// reaches 95. The returned lag must therefore differ from 95.
func TestPickBestInRange_Range80_EvenOnlyScan(t *testing.T) {
	var wsp [223]int16
	for n := 0; n < 40; n++ {
		wsp[143+2*n] = 1024
		wsp[48+2*n] = 1024
	}
	lag, _, _ := pickBestInRange(&wsp, 80, 143)
	if lag == 95 {
		t.Fatalf("pickBestInRange[80,143] visited odd lag 95 outside ±1 refinement window; even-only scan rule §A.3.4 line 2113 violated")
	}
	// Sanity: the algorithm must return one of the visited candidates
	// {80, 81} (best_even=80 by tie-break, refinement window {80,81}).
	if lag != 80 && lag != 81 {
		t.Fatalf("pickBestInRange[80,143] lag = %d, want 80 or 81 (refinement window around best_even=80)", lag)
	}
}

// TestPickBestInRange_NoAlloc enforces I4 zero-allocation on all three
// §A.3.4 ranges (the [80,143] path runs the longest scan + refinement
// branch).
func TestPickBestInRange_NoAlloc(t *testing.T) {
	var wsp [223]int16
	for i := range wsp {
		wsp[i] = int16(i & 0x3FF)
	}
	for _, rng := range [3][2]int{{20, 39}, {40, 79}, {80, 143}} {
		kMin, kMax := rng[0], rng[1]
		allocs := testing.AllocsPerRun(50, func() {
			_, _, _ = pickBestInRange(&wsp, kMin, kMax)
		})
		if allocs != 0 {
			t.Fatalf("pickBestInRange[%d,%d] allocates %v/op, want 0", kMin, kMax, allocs)
		}
	}
}
