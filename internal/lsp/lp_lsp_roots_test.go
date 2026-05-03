package lsp

import (
	"errors"
	"math"
	"testing"
)

// TestFindLSPRoots_RecoversChosenLSPs picks 10 LSPs uniformly spaced
// in ω ∈ (0, π), runs them through lspToLP → computeF1F2 →
// findLSPRoots, and asserts the recovered cosines are within the
// algorithm's intrinsic tolerance.
//
// Tolerance derivation: the 60-point grid spans ω ∈ [0, π] in 59
// equal sub-intervals of width π/59 ≈ 0.0532 rad. Four binary
// subdivisions (§3.2.3 line 784) shrink that to π/(59·16) ≈ 0.00333
// rad. The induced error in q = cos(ω) is bounded by sin(ω)·Δω,
// which in Q15 peaks at sin(π/2)·0.00333·32768 ≈ 109 LSBs. We allow
// 256 LSBs to also absorb chebyshevC Q24 truncation and the lspToLP
// → computeF1F2 forward error. The plan's "±1 LSB Q15" target is
// not algorithmically reachable with a 60-point grid + 4 bisections;
// the binding constraint per I11 is the (60, 4) pair, not the
// numeric tolerance.
func TestFindLSPRoots_RecoversChosenLSPs(t *testing.T) {
	var lsp [10]int16
	for i := 0; i < 10; i++ {
		omega := float64(i+1) * math.Pi / 11.0
		v := math.Round(math.Cos(omega) * 32768.0)
		if v > 32767 {
			v = 32767
		}
		if v < -32768 {
			v = -32768
		}
		lsp[i] = int16(v)
	}

	var a [11]int16
	lspToLP(&lsp, &a)

	var f1, f2 [6]int32
	computeF1F2(&a, &f1, &f2)

	var q [10]int16
	if err := findLSPRoots(&f1, &f2, &q); err != nil {
		t.Fatalf("findLSPRoots returned error: %v", err)
	}

	const tol int32 = 256
	for i := 0; i < 10; i++ {
		d := int32(q[i]) - int32(lsp[i])
		if d < 0 {
			d = -d
		}
		if d > tol {
			t.Errorf("q[%d]=%d want≈%d (Δ=%d > tol=%d)", i, q[i], lsp[i], d, tol)
		}
	}

	// LSPs are in cosine domain, ω strictly increasing ⇒ q strictly
	// decreasing.
	for i := 1; i < 10; i++ {
		if q[i] >= q[i-1] {
			t.Errorf("q not strictly decreasing at i=%d: q[%d]=%d, q[%d]=%d",
				i, i-1, q[i-1], i, q[i])
		}
	}

	// Endpoint sanity: 0 < ω_1 ⇒ q[0] < +1.0 (Q15 max), and ω_10 < π
	// ⇒ q[9] > -1.0 (Q15 min).
	if q[0] >= 32767 {
		t.Errorf("q[0]=%d should be < 32767 (ω_1 > 0)", q[0])
	}
	if q[9] <= -32768 {
		t.Errorf("q[9]=%d should be > -32768 (ω_10 < π)", q[9])
	}
}

// TestFindLSPRoots_NonStableReturnsErr feeds polynomials whose
// Chebyshev sums are dominated by a large constant term, so no sign
// changes occur on the grid. The function must surface
// ErrLPCNonStable for E8 routing.
func TestFindLSPRoots_NonStableReturnsErr(t *testing.T) {
	const oneQ24 int32 = 1 << 24

	// C(x) = T_5(x) + 0·T_4 + ... + 0·T_1 + (huge)/2 ⇒ strictly > 0
	// on [-1, +1] (|T_5| ≤ 1 ≪ huge/2).
	huge := int32(1 << 28)
	f1 := [6]int32{oneQ24, 0, 0, 0, 0, huge}
	f2 := [6]int32{oneQ24, 0, 0, 0, 0, huge}

	var q [10]int16
	err := findLSPRoots(&f1, &f2, &q)
	if err == nil {
		t.Fatal("expected ErrLPCNonStable, got nil")
	}
	if !errors.Is(err, ErrLPCNonStable) {
		t.Fatalf("expected ErrLPCNonStable, got %v", err)
	}
}
