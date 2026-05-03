package fcbsearch_test

import (
	"testing"

	"github.com/exedev/g729/internal/fcbsearch"
)

// CB-3 RED tests for §3.8.1 sign decomposition (G729E.txt lines 1296–1300):
// "the signal d(n) is decomposed into two parts: its absolute value |d(n)|
// and its sign sign[d(n)]". The signs are extracted *once* from d(n) and
// applied during the depth-first ACELP search; CB-4 reapplies them when
// the final excitation c(n) is built.
//
// Sign convention pinned by the Phase 2d sub-plan (OQ-A38-SIGNTIE,
// docs/.../2026-05-11-phase2d-fixed-codebook-acelp-plan.md §9):
//   - signs[n] = +1 if d[n] >= 0
//   - signs[n] = -1 if d[n] <  0
//   - signs[n] = +1 if d[n] == 0  (spec is silent; default = +1)

func TestSignsFromD_AllPositive(t *testing.T) {
	var d [40]int32
	for n := range d {
		d[n] = int32(1 + n*37) // strictly positive
	}
	var signs [40]int16
	var dAbs [40]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)
	for n := range d {
		if signs[n] != +1 {
			t.Fatalf("signs[%d]=%d want +1 (d=%d)", n, signs[n], d[n])
		}
		if dAbs[n] != d[n] {
			t.Fatalf("dAbs[%d]=%d want %d", n, dAbs[n], d[n])
		}
	}
}

func TestSignsFromD_AllNegative(t *testing.T) {
	var d [40]int32
	for n := range d {
		d[n] = -int32(1 + n*53) // strictly negative
	}
	var signs [40]int16
	var dAbs [40]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)
	for n := range d {
		if signs[n] != -1 {
			t.Fatalf("signs[%d]=%d want -1 (d=%d)", n, signs[n], d[n])
		}
		if dAbs[n] != -d[n] {
			t.Fatalf("dAbs[%d]=%d want %d (|d|)", n, dAbs[n], -d[n])
		}
	}
}

func TestSignsFromD_MixedHandGolden(t *testing.T) {
	// Hand-crafted vector covering positive, negative, and zero entries.
	var d [40]int32
	values := []int32{
		+5, -3, 0, +1234567, -7654321, 0, +1, -1,
		+100, -100, +0, -0, +99999, -99999, +2, -2,
		+1000000, -1000000, +1, -1, 0, +50, -50, +7,
		-7, +13, -13, +21, -21, +0, -0, +99,
		-99, +123, -123, +456, -456, +789, -789, +1,
	}
	for n := range d {
		d[n] = values[n]
	}

	var signs [40]int16
	var dAbs [40]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	for n, v := range values {
		var wantSign int16
		var wantAbs int32
		if v >= 0 {
			wantSign = +1
			wantAbs = v
		} else {
			wantSign = -1
			wantAbs = -v
		}
		if signs[n] != wantSign {
			t.Fatalf("signs[%d]=%d want %d (d=%d)", n, signs[n], wantSign, v)
		}
		if dAbs[n] != wantAbs {
			t.Fatalf("dAbs[%d]=%d want %d (d=%d)", n, dAbs[n], wantAbs, v)
		}
	}
}

// TestSignsFromD_ZeroPinPlusOne verifies the OQ-A38-SIGNTIE pin: when
// d(n) == 0, signs[n] defaults to +1 (Phase 2d sub-plan §9 line 458).
func TestSignsFromD_ZeroPinPlusOne(t *testing.T) {
	var d [40]int32 // all zeros
	var signs [40]int16
	var dAbs [40]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)
	for n := range d {
		if signs[n] != +1 {
			t.Fatalf("signs[%d]=%d want +1 (OQ-A38-SIGNTIE pin)", n, signs[n])
		}
		if dAbs[n] != 0 {
			t.Fatalf("dAbs[%d]=%d want 0", n, dAbs[n])
		}
	}
}

// TestSignsFromD_FromCB1Trace cross-checks sign extraction against a real
// d[40] produced by CB-1 (AdjustedTarget + CorrelationD) from a hand-
// chosen x, y, gp, h. This pins CB-3 to the actual CB-1 Q12 output.
func TestSignsFromD_FromCB1Trace(t *testing.T) {
	var x, y, h [40]int16
	for n := range x {
		x[n] = int16(-2000 + 137*n) // crosses zero around n≈14
		y[n] = int16(500 - 23*n)
		h[n] = int16(2048 - 41*n) // Q12, decreasing
	}
	const gp = int16(10000) // Q14 ≈ 0.61

	var xPrime [40]int16
	fcbsearch.AdjustedTarget(&x, &y, gp, &xPrime)

	var d [40]int32
	fcbsearch.CorrelationD(&xPrime, &h, &d)

	var signs [40]int16
	var dAbs [40]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	for n := range d {
		var wantSign int16
		var wantAbs int32
		if d[n] >= 0 {
			wantSign = +1
			wantAbs = d[n]
		} else {
			wantSign = -1
			wantAbs = -d[n]
		}
		if signs[n] != wantSign {
			t.Fatalf("signs[%d]=%d want %d (d=%d, CB-1 trace)", n, signs[n], wantSign, d[n])
		}
		if dAbs[n] != wantAbs {
			t.Fatalf("dAbs[%d]=%d want %d (d=%d)", n, dAbs[n], wantAbs, d[n])
		}
		if dAbs[n] < 0 {
			t.Fatalf("dAbs[%d]=%d must be non-negative", n, dAbs[n])
		}
	}
}

func TestSignsFromD_NoAlloc(t *testing.T) {
	var d [40]int32
	for n := range d {
		d[n] = int32((n - 20) * 12345)
	}
	var signs [40]int16
	var dAbs [40]int32
	if got := testing.AllocsPerRun(128, func() {
		fcbsearch.SignsFromD(&d, &signs, &dAbs)
	}); got != 0 {
		t.Fatalf("SignsFromD allocations/op = %v, want 0", got)
	}
}
