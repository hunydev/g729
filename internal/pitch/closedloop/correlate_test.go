package closedloop

import "testing"

// pitchMinInt and pitchMaxInt mirror the §A.3.7 integer-lag search
// boundary [20, 143] used by CL-1 (G729E.txt lines 2129–2131,
// 2167–2168). They are duplicated here in the test only so the
// production constants can move without coupling unrelated tests.
const (
	testPitchMin = 20
	testPitchMax = 143
	testExcLen   = 256 // ≥ pitchMaxInt + SubframeLen = 183, padded.
)

// TestSearchInteger_ZeroExcitationGivesZeroRN exercises eq. A.7 with
// u(n) ≡ 0: every term xb(n)·u(n−k) vanishes so RN(k) = 0 for all k
// in the search window. The function must report RNbest = 0 and
// fall back to the lower window bound (the standard "favour shorter
// delays" tie-break inherited from openloop §A.3.4 line 2110).
//
// Spec: ITU-T G.729 Annex A §A.3.7 eq. A.7 (G729E.txt line 2154).
func TestSearchInteger_ZeroExcitationGivesZeroRN(t *testing.T) {
	var xb [SubframeLen]int16
	for n := range xb {
		xb[n] = int16(100 - n) // arbitrary non-zero target; should not matter.
	}
	exc := make([]int16, testExcLen) // all zero
	intLag, RNbest := SearchInteger(&xb, exc, 60, 0)
	if RNbest != 0 {
		t.Fatalf("RNbest = %d, want 0 for zero excitation", RNbest)
	}
	// Window for center=60 is [57, 63]; tie-break favours kMin=57.
	if intLag != 57 {
		t.Fatalf("intLag = %d, want 57 (window kMin tie-break)", intLag)
	}
}

// TestSearchInteger_ImpulseExcitationLocksLag plants a unit impulse
// in the past-excitation buffer at offset −k₀ (i.e. u(n) = δ(n+k₀))
// and a strictly decreasing xb. Eq. A.7 then reduces to
// RN(k) = xb[k − k₀] for k − k₀ ∈ [0, 39], so the maximum sits at
// k = k₀ where xb is largest. Verifies (a) k₀ is selected and
// (b) the returned RN value carries the expected ×2 LMac scaling.
//
// Spec: §A.3.7 eq. A.7 (G729E.txt line 2154); search range pin per
// §A.3.7 line 2167 ("around a preselected value").
func TestSearchInteger_ImpulseExcitationLocksLag(t *testing.T) {
	const k0 = 50
	var xb [SubframeLen]int16
	for n := 0; n < SubframeLen; n++ {
		xb[n] = int16(400 - 10*n) // strictly decreasing, all positive.
	}
	exc := make([]int16, testExcLen)
	// u(n − k₀) = δ(n) ⇔ u(−k₀) = 1 ⇔ exc[len − SubframeLen − k₀] = 1.
	exc[testExcLen-SubframeLen-k0] = 1

	intLag, RNbest := SearchInteger(&xb, exc, k0, 0)
	if intLag != k0 {
		t.Fatalf("intLag = %d, want %d (impulse lag)", intLag, k0)
	}
	// LMac semantics: RNbest = 2 · xb[0] · 1 = 800.
	if RNbest != 800 {
		t.Fatalf("RNbest = %d, want 800 (= 2·xb[0])", RNbest)
	}
}

// TestSearchInteger_BackwardFilteredAlignment pins the algebraic
// identity Σ x(n)·yk(n) = Σ xb(n)·u(n−k) (eq. A.7) for a hand-built
// case: with h = δ (only the leading tap nonzero in Q12), the
// backward-filtered target equals x itself, so the closed-loop
// correlation degenerates to Σ x(n)·u(n−k). We feed
// x(n) = h(n) = identity tap so xb collapses to x = δ as well, and
// place the excitation impulse at k₀ to verify the alignment
// matches the impulse-response convention.
//
// Spec: §A.3.7 eq. A.7 second equality (G729E.txt line 2154);
// h-of-Q12 convention from HI-1 (impulse.go).
func TestSearchInteger_BackwardFilteredAlignment(t *testing.T) {
	const k0 = 80
	var x, h [SubframeLen]int16
	h[0] = 4096           // 1.0 in Q12 — identity impulse response.
	x[0] = 1234           // arbitrary non-trivial target sample.
	x[5] = -7             // ensure a second nonzero target sample.
	x[39] = 42            // ensure last-sample alignment is exercised.

	var xb [SubframeLen]int16
	BackwardFilter(&x, &h, &xb)
	// Identity h means xb(n) = x(n)·h(0)>>12 = x(n) for all n.
	for n := 0; n < SubframeLen; n++ {
		if xb[n] != x[n] {
			t.Fatalf("BackwardFilter[h=δ]: xb[%d] = %d, want x[%d] = %d",
				n, xb[n], n, x[n])
		}
	}

	exc := make([]int16, testExcLen)
	exc[testExcLen-SubframeLen-k0] = 1
	intLag, RNbest := SearchInteger(&xb, exc, k0, 0)
	if intLag != k0 {
		t.Fatalf("intLag = %d, want %d", intLag, k0)
	}
	// Only n=0 contributes (impulse at u(0−k₀)); RN = 2·xb[0]·1 = 2·1234.
	if RNbest != 2*1234 {
		t.Fatalf("RNbest = %d, want %d", RNbest, 2*1234)
	}
}

// TestBackwardFilter_NonTrivialKernel pins the convolution direction
// for a two-tap h. Per the derivation in package doc:
//
//	xb(n) = Σ_{m=n..39} x(m) · h(m − n)
//
// With h = [4096, 4096, 0, ...] (1.0 + 1.0·z⁻¹ in Q12) and
// x(m) = m + 1 (m = 0..39), xb(n) = x(n) + x(n+1) for n < 39 and
// xb(39) = x(39). The Q12 product is normalised back to Q0 by an
// arithmetic shift right by 12, matching h's Q-format.
//
// Spec: §A.3.7 eq. A.7 — "xb(n) is the backward filtered target
// signal (correlation between x(n) and the impulse response h(n))"
// (G729E.txt lines 2155–2156).
func TestBackwardFilter_NonTrivialKernel(t *testing.T) {
	var x, h [SubframeLen]int16
	h[0] = 4096
	h[1] = 4096
	for m := 0; m < SubframeLen; m++ {
		x[m] = int16(m + 1)
	}
	var xb [SubframeLen]int16
	BackwardFilter(&x, &h, &xb)
	for n := 0; n < SubframeLen-1; n++ {
		want := int16(int(x[n]) + int(x[n+1]))
		if xb[n] != want {
			t.Fatalf("xb[%d] = %d, want %d (= x[%d]+x[%d])",
				n, xb[n], want, n, n+1)
		}
	}
	if xb[SubframeLen-1] != x[SubframeLen-1] {
		t.Fatalf("xb[39] = %d, want %d (tail = x[39])",
			xb[SubframeLen-1], x[SubframeLen-1])
	}
}

// TestSearchInteger_ZeroAlloc enforces I4 for the integer-lag search.
func TestSearchInteger_ZeroAlloc(t *testing.T) {
	var xb [SubframeLen]int16
	for n := range xb {
		xb[n] = int16(n - 20)
	}
	exc := make([]int16, testExcLen)
	for i := range exc {
		exc[i] = int16((i*7 + 3) % 41)
	}
	allocs := testing.AllocsPerRun(64, func() {
		_, _ = SearchInteger(&xb, exc, 60, 0)
	})
	if allocs != 0 {
		t.Fatalf("SearchInteger allocs/op = %v, want 0", allocs)
	}
}

// TestBackwardFilter_ZeroAlloc enforces I4 for the back-filter.
func TestBackwardFilter_ZeroAlloc(t *testing.T) {
	var x, h, xb [SubframeLen]int16
	for n := range x {
		x[n] = int16(n - 20)
		h[n] = int16(4096 - n*40)
	}
	allocs := testing.AllocsPerRun(64, func() {
		BackwardFilter(&x, &h, &xb)
	})
	if allocs != 0 {
		t.Fatalf("BackwardFilter allocs/op = %v, want 0", allocs)
	}
}

// Compile-time guard: testPitchMin/testPitchMax mirror production.
var _ = [1]struct{}{}[testPitchMax-PitchMaxInt]
var _ = [1]struct{}{}[testPitchMin-PitchMinInt]
