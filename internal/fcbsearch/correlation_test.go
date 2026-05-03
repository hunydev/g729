package fcbsearch_test

import (
	"testing"
	"testing/quick"

	"github.com/exedev/g729/internal/fcbsearch"
	"github.com/exedev/g729/internal/pitch/closedloop"
)

// CB-1 RED tests for §3.8.1 eq. 50 (x'(n) = x − gp·y) and eq. 52
// (d(n) = Σ_{i=n..39} x'(i)·h(i−n)).
//
// Q-format pin (OQ-Q-FORMAT-A10 default per Phase 2d sub-plan §3 line 142):
//   - x  : Q0 int16 (TG-1 target convention)
//   - y  : Q0 int16 (Phase 2c GP-1 output)
//   - gp : Q14 int16 ([0, GpUpperQ14] per Phase 2c GP-1 cap)
//   - h  : Q12 int16 (HI-1 impulse response)
//   - x' : Q0 int16; gp*y product is int32 Q14, arithmetically shifted
//          right by 14 to Q0, then subtracted from x and Word16-saturated.
//   - d  : int32 in Q12 (un-shifted accumulator of x'·h products), so the
//          downstream sign / φ′ / energy combinatorics keep full precision.

func TestCorrelationD_UnitImpulse(t *testing.T) {
	// h = δ (unit impulse in Q12: h[0] = 4096, rest = 0). Eq. 52 collapses
	// to d(n) = x'(n)·h(0), i.e. d(n) = x'(n) << 12.
	var xPrime [40]int16
	for n := range xPrime {
		xPrime[n] = int16(n - 20) // [-20..19]
	}
	var h [40]int16
	h[0] = 4096

	var d [40]int32
	fcbsearch.CorrelationD(&xPrime, &h, &d)

	for n := range d {
		want := int32(xPrime[n]) << 12
		if d[n] != want {
			t.Fatalf("d[%d]=%d want %d (unit-impulse collapse)", n, d[n], want)
		}
	}
}

func TestCorrelationD_TwoTapGolden(t *testing.T) {
	// h = [a, b, 0, ..., 0] in Q12. Eq. 52:
	//   d(n) = x'(n)·a + x'(n+1)·b   for n in [0, 38]
	//   d(39) = x'(39)·a
	var h [40]int16
	a, b := int16(3000), int16(-1500)
	h[0], h[1] = a, b

	var xPrime [40]int16
	for n := range xPrime {
		xPrime[n] = int16(100 + 7*n)
	}

	var got [40]int32
	fcbsearch.CorrelationD(&xPrime, &h, &got)

	for n := 0; n < 39; n++ {
		want := int32(xPrime[n])*int32(a) + int32(xPrime[n+1])*int32(b)
		if got[n] != want {
			t.Fatalf("d[%d]=%d want %d", n, got[n], want)
		}
	}
	want39 := int32(xPrime[39]) * int32(a)
	if got[39] != want39 {
		t.Fatalf("d[39]=%d want %d", got[39], want39)
	}
}

func TestAdjustedTarget_GpZero(t *testing.T) {
	// gp = 0 → x'(n) ≡ x(n) for all n (no pitch contribution to remove).
	var x, y [40]int16
	for n := range x {
		x[n] = int16(-15000 + 800*n)
		y[n] = int16(2000 - 100*n)
	}
	var xPrime [40]int16
	fcbsearch.AdjustedTarget(&x, &y, 0, &xPrime)
	for n := range x {
		if xPrime[n] != x[n] {
			t.Fatalf("xPrime[%d]=%d want %d (gp=0 must be identity)", n, xPrime[n], x[n])
		}
	}
}

func TestAdjustedTarget_HandDerivation(t *testing.T) {
	// Verify x'(n) = x(n) − ((gp·y(n)) >> 14) on a randomly seeded vector.
	const gp = int16(12000) // Q14 ≈ 0.732
	var x, y [40]int16
	for n := range x {
		x[n] = int16(500 - 30*n)
		y[n] = int16(-200 + 17*n)
	}

	var got [40]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &got)

	for n := range x {
		prod := int32(gp) * int32(y[n]) // Q14
		shifted := prod >> 14            // Q0
		raw := int32(x[n]) - shifted
		var want int16
		switch {
		case raw > 32767:
			want = 32767
		case raw < -32768:
			want = -32768
		default:
			want = int16(raw)
		}
		if got[n] != want {
			t.Fatalf("xPrime[%d]=%d want %d (raw=%d)", n, got[n], want, raw)
		}
	}
}

func TestAdjustedTarget_SaturatesOnLargeGpY(t *testing.T) {
	// Force the subtraction to overflow Word16 in both directions:
	//   x = -32768, gp·y >> 14 strongly positive → raw underflows to Min16.
	//   x = +32767, gp·y >> 14 strongly negative → raw overflows to Max16.
	var x, y [40]int16
	for n := range x {
		if n%2 == 0 {
			x[n] = -32768
			y[n] = 32767 // gp·y/16384 ≈ +y → x - (+y) very negative
		} else {
			x[n] = 32767
			y[n] = -32768
		}
	}
	const gp = int16(16384) // Q14 = 1.0

	var got [40]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &got)

	for n := range x {
		prod := int32(gp) * int32(y[n])
		shifted := prod >> 14
		raw := int32(x[n]) - shifted
		var want int16
		switch {
		case raw > 32767:
			want = 32767
		case raw < -32768:
			want = -32768
		default:
			want = int16(raw)
		}
		if got[n] != want {
			t.Fatalf("xPrime[%d]=%d want %d (raw=%d, x=%d, y=%d)",
				n, got[n], want, raw, x[n], y[n])
		}
	}
}

// TestCorrelationD_GpZeroMatchesBackwardFilter cross-checks against the
// Phase 2c precedent: with gp=0 we have x' = x, so d(n) >> 12 (with
// Word16 saturation) must equal closedloop.BackwardFilter's xb(n).
func TestCorrelationD_GpZeroMatchesBackwardFilter(t *testing.T) {
	var x, y, h [40]int16
	for n := range x {
		x[n] = int16(-4000 + 250*n)
		h[n] = int16(1500 - 35*n)
	}

	var xPrime [40]int16
	fcbsearch.AdjustedTarget(&x, &y, 0, &xPrime)

	var d [40]int32
	fcbsearch.CorrelationD(&xPrime, &h, &d)

	var xb [40]int16
	closedloop.BackwardFilter(&x, &h, &xb)

	for n := range d {
		shifted := d[n] >> 12
		// match BackwardFilter's saturation
		var want int32
		switch {
		case shifted > 32767:
			want = 32767
		case shifted < -32768:
			want = -32768
		default:
			want = shifted
		}
		if int32(xb[n]) != want {
			t.Fatalf("n=%d: d>>12=%d (sat %d), xb=%d", n, shifted, want, xb[n])
		}
	}
}

func TestAdjustedTarget_NoAlloc(t *testing.T) {
	var x, y, out [40]int16
	for n := range x {
		x[n] = int16(n * 31)
		y[n] = int16(n * -17)
	}
	if got := testing.AllocsPerRun(128, func() {
		fcbsearch.AdjustedTarget(&x, &y, 8000, &out)
	}); got != 0 {
		t.Fatalf("AdjustedTarget allocations/op = %v, want 0", got)
	}
}

func TestCorrelationD_NoAlloc(t *testing.T) {
	var xPrime, h [40]int16
	var d [40]int32
	for n := range xPrime {
		xPrime[n] = int16(n * 11)
		h[n] = int16(n * 7)
	}
	if got := testing.AllocsPerRun(128, func() {
		fcbsearch.CorrelationD(&xPrime, &h, &d)
	}); got != 0 {
		t.Fatalf("CorrelationD allocations/op = %v, want 0", got)
	}
}

// Property: AdjustedTarget output stays within [-32768, 32767] under any
// int16 inputs and any gp ∈ [0, GpUpperQ14] (Phase 2c GP-1 contract).
func TestAdjustedTarget_QuickCheck_Word16Range(t *testing.T) {
	f := func(seed uint64, gp uint16) bool {
		var x, y, out [40]int16
		s := seed
		for n := range x {
			s = s*6364136223846793005 + 1442695040888963407
			x[n] = int16(int64(s>>48) - 32768)
			s = s*6364136223846793005 + 1442695040888963407
			y[n] = int16(int64(s>>48) - 32768)
		}
		gpQ14 := int16(gp % uint16(closedloop.GpUpperQ14+1))
		fcbsearch.AdjustedTarget(&x, &y, gpQ14, &out)
		// Word16 range is implicit in the int16 type; this just exercises
		// the saturation path under randomized inputs.
		_ = out
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 64}); err != nil {
		t.Fatal(err)
	}
}
