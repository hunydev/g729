package closedloop

import (
	"testing"
)

// TestImpulseResponse_IdentityFilter exercises the trivial Â(z) = 1
// case: aHatQ12 = [4096, 0, 0, ...] makes 1/Â(z/γ) = 1, so the
// impulse response collapses to the input — a single 1.0 followed by
// 39 zeros, all in Q12.
//
// Spec: §A.3.5 lines 2114–2117 (G729E.txt).
func TestImpulseResponse_IdentityFilter(t *testing.T) {
	var a = [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var h [SubframeLen]int16
	ImpulseResponse(&a, &h)

	if h[0] != 4096 {
		t.Fatalf("h[0] = %d, want 4096 (= 1.0 in Q12)", h[0])
	}
	for n := 1; n < SubframeLen; n++ {
		if h[n] != 0 {
			t.Fatalf("h[%d] = %d, want 0 (identity filter)", n, h[n])
		}
	}
}

// TestImpulseResponse_FirstTapAlwaysQ12One asserts the Q-format
// invariant pinned in package doc: for any normalized Â (a[0] = 4096
// per ITU LP convention) the leading impulse-response sample is
// exactly 1.0 (= 4096 in Q12). The Annex A weighting γ = 0.75 leaves
// aw[0] = 4096 unchanged (γ⁰ = 1.0) so the recurrence yields
// h[0] = δ[0] / aw[0] = 1.0.
//
// Spec: §A.3.5 lines 2114–2117; §A.3.3 line 2063 (γ = 0.75).
func TestImpulseResponse_FirstTapAlwaysQ12One(t *testing.T) {
	cases := [][11]int16{
		{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{4096, 1024, -512, 256, -128, 64, -32, 16, -8, 4, -2},
		{4096, -3500, 2800, -2100, 1500, -1000, 700, -400, 200, -100, 50},
	}
	for idx, a := range cases {
		var h [SubframeLen]int16
		aa := a
		ImpulseResponse(&aa, &h)
		if h[0] != 4096 {
			t.Fatalf("case %d: h[0] = %d, want 4096", idx, h[0])
		}
	}
}

// TestImpulseResponse_HandTracedFirstOrder pins a closed-form
// reference for the order-1 weighted filter
//
//	Â(z) = 1 - 0.5 z⁻¹    →    Â(z/γ) = 1 - (γ·0.5) z⁻¹  with γ = 0.75
//	                          = 1 - 0.375 z⁻¹
//
// The all-pole impulse response is h[n] = 0.375ⁿ. In Q12 the first
// few samples round to:
//
//	h[0] = 4096 (1.0)
//	h[1] = 1536 (0.375)
//	h[2] =  576 (0.140625)
//	h[3] =  216 (0.052734375)
//	h[4] =   81 (0.019775...)
//
// The Q15 representation of 0.5 is 16384 → in Q12 = 2048 (negated
// sign carried in the LP convention Â(z) = 1 + Σ a[i]z⁻ⁱ stores the
// polynomial coefficient directly). The fixed.Mult of -2048 by
// gammaPow[1]=24576 yields aw[1] = -1536, so the recurrence gives
// h[n] = -aw[1]·h[n-1] / aw[0] = 0.375·h[n-1].
//
// Spec: §A.3.5 (filter form); §A.3.3 line 2063 (γ = 0.75 LUT pinned
// in gammaPow[1] = 24576).
func TestImpulseResponse_HandTracedFirstOrder(t *testing.T) {
	a := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var h [SubframeLen]int16
	ImpulseResponse(&a, &h)

	want := [5]int16{4096, 1536, 576, 216, 81}
	for n, w := range want {
		if h[n] != w {
			t.Fatalf("h[%d] = %d, want %d", n, h[n], w)
		}
	}
	// Geometric decay is non-increasing (samples reach the Q12 floor of 0
	// after a few taps and then stay there).
	for n := 1; n < SubframeLen; n++ {
		if abs16(h[n]) > abs16(h[n-1]) {
			t.Fatalf("non-monotonic decay at n=%d: |h|=%d vs prior %d",
				n, abs16(h[n]), abs16(h[n-1]))
		}
	}
}

// TestImpulseResponse_Decay enforces the algorithmic property
// |h[39]| < |h[0]| for a representative stable LP filter (the same
// hand-traced order-1 filter above). The wider-coverage spectral-
// distance gates on PITCH.IN frames are deferred to INT-1.
//
// Spec: §A.3.5 (the filter is by design stable when Â is a stable
// LP-analysis polynomial and γ < 1 contracts the radii further).
func TestImpulseResponse_Decay(t *testing.T) {
	a := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var h [SubframeLen]int16
	ImpulseResponse(&a, &h)

	if abs16(h[SubframeLen-1]) >= abs16(h[0]) {
		t.Fatalf("expected decay: |h[%d]|=%d < |h[0]|=%d",
			SubframeLen-1, abs16(h[SubframeLen-1]), abs16(h[0]))
	}
}

func abs16(x int16) int32 {
	if x < 0 {
		return -int32(x)
	}
	return int32(x)
}
