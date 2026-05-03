package closedloop

import "testing"

// TestGpAndY_UnitImpulseHIsIdentity: with h(0) = 1.0 in Q12 (= 4096)
// and h(n>0) = 0, the convolution y(n) = Σ_{i=0..n} v(i)·h(n−i)
// degenerates to y(n) = v(n) (the v·h product is Q12 → shifted right
// by 12 to land y in Q0, mirroring BackwardFilter's xb scaling).
// Picking x ≡ y gives Σ x·y = Σ y² so eq. 43 yields Gp = 1.0 →
// Q14 = 16384.
//
// Spec: ITU-T G.729 §3.7.3 eq. 43/44 (G729E.txt lines 1186–1199).
func TestGpAndY_UnitImpulseHIsIdentity(t *testing.T) {
	var x, v, h, y [SubframeLen]int16
	h[0] = 4096 // 1.0 in Q12
	for n := 0; n < SubframeLen; n++ {
		v[n] = int16(n + 1)
		x[n] = int16(n + 1)
	}
	gp := GpAndY(&x, &v, &h, &y)
	for n := 0; n < SubframeLen; n++ {
		if y[n] != int16(n+1) {
			t.Fatalf("y[%d] = %d, want %d (identity convolution)", n, y[n], n+1)
		}
	}
	if gp != 16384 {
		t.Fatalf("Gp = %d, want 16384 (1.0 in Q14)", gp)
	}
}

// TestGpAndY_TwoTapConvolution: spot-check eq. 44 with a length-2
// impulse response h = [1.0, 1.0] in Q12 = [4096, 4096]. For
// v = [1,2,3,4,5,0,...] the convolution gives
// y = [1, 3, 5, 7, 9, 5, 0, ...]. Numerator and denominator follow
// directly so the closed-form Gp can be hand-computed and compared
// to the Q14 result.
func TestGpAndY_TwoTapConvolution(t *testing.T) {
	var x, v, h, y [SubframeLen]int16
	h[0], h[1] = 4096, 4096
	v[0], v[1], v[2], v[3], v[4] = 1, 2, 3, 4, 5
	want := [SubframeLen]int16{}
	want[0], want[1], want[2], want[3], want[4], want[5] = 1, 3, 5, 7, 9, 5
	// Choose x = y so Gp = 1.0 exactly without round-off.
	for n := 0; n < SubframeLen; n++ {
		x[n] = want[n]
	}
	gp := GpAndY(&x, &v, &h, &y)
	if y != want {
		t.Fatalf("y = %v, want %v", y, want)
	}
	if gp != 16384 {
		t.Fatalf("Gp = %d, want 16384 (1.0 in Q14)", gp)
	}
}

// TestGpAndY_HalfGain: with y[0]=2 (h identity, v[0]=2) and x[0]=1,
// num = 2 and den = 4, so Gp = 0.5 → Q14 = 8192. Verifies the
// generic non-degenerate division path of eq. 43.
func TestGpAndY_HalfGain(t *testing.T) {
	var x, v, h, y [SubframeLen]int16
	h[0] = 4096
	v[0] = 2
	x[0] = 1
	gp := GpAndY(&x, &v, &h, &y)
	if y[0] != 2 {
		t.Fatalf("y[0] = %d, want 2", y[0])
	}
	if gp != 8192 {
		t.Fatalf("Gp = %d, want 8192 (0.5 in Q14)", gp)
	}
}

// TestGpAndY_ClampUpperBound: with y[0]=1 and x[0]=2 the raw ratio
// is 2.0, well above the §3.7.3 eq. 43 cap of 1.2. The result must
// be clamped to GpUpperQ14 = round(1.2·2^14) = 19661.
//
// OQ-GBOUND (resolved): the spec writes "bounded by 0 ≤ gp ≤ 1.2"
// using the inclusive ≤ operator on both ends, so 1.2 is reachable
// (inclusive upper bound). Q14 representation pinned at 19661.
func TestGpAndY_ClampUpperBound(t *testing.T) {
	var x, v, h, y [SubframeLen]int16
	h[0] = 4096
	v[0] = 1
	x[0] = 2
	gp := GpAndY(&x, &v, &h, &y)
	if gp != GpUpperQ14 {
		t.Fatalf("Gp = %d, want %d (1.2 in Q14)", gp, GpUpperQ14)
	}
}

// TestGpAndY_ClampUpperBoundExact: ratio exactly 1.2 (num=6, den=5)
// must hit the inclusive cap GpUpperQ14, not pass through the
// division branch. Pins the OQ-GBOUND inclusivity decision at the
// boundary itself.
func TestGpAndY_ClampUpperBoundExact(t *testing.T) {
	var x, v, h, y [SubframeLen]int16
	h[0] = 4096
	v[0] = 5
	x[0] = 6
	gp := GpAndY(&x, &v, &h, &y)
	if gp != GpUpperQ14 {
		t.Fatalf("Gp = %d, want %d (1.2 in Q14, inclusive)", gp, GpUpperQ14)
	}
}

// TestGpAndY_ZeroEnergyY: when v = 0 (and so y = 0), Σ y² = 0.
// The implementation must return Gp = 0 instead of dividing by zero.
func TestGpAndY_ZeroEnergyY(t *testing.T) {
	var x, v, h, y [SubframeLen]int16
	h[0] = 4096
	for n := range x {
		x[n] = int16(n + 1)
	}
	gp := GpAndY(&x, &v, &h, &y)
	if gp != 0 {
		t.Fatalf("Gp = %d, want 0 (zero-energy y)", gp)
	}
	for n := 0; n < SubframeLen; n++ {
		if y[n] != 0 {
			t.Fatalf("y[%d] = %d, want 0", n, y[n])
		}
	}
}

// TestGpAndY_NegativeCorrelationClampsToZero: when x is anti-phase
// to y the numerator Σ x·y is negative; the eq. 43 lower bound
// "0 ≤ gp" forces Gp = 0 (no negative gain emitted).
func TestGpAndY_NegativeCorrelationClampsToZero(t *testing.T) {
	var x, v, h, y [SubframeLen]int16
	h[0] = 4096
	for n := 0; n < SubframeLen; n++ {
		v[n] = int16(n + 1)
		x[n] = -int16(n + 1)
	}
	gp := GpAndY(&x, &v, &h, &y)
	if gp != 0 {
		t.Fatalf("Gp = %d, want 0 (negative correlation clamped)", gp)
	}
}

// TestGpAndY_ZeroAlloc enforces I4 (zero-allocation) on the
// adaptive-codebook gain primitive.
func TestGpAndY_ZeroAlloc(t *testing.T) {
	var x, v, h, y [SubframeLen]int16
	h[0] = 4096
	for n := range x {
		x[n] = int16(n - 20)
		v[n] = int16(n + 1)
	}
	allocs := testing.AllocsPerRun(64, func() {
		_ = GpAndY(&x, &v, &h, &y)
	})
	if allocs != 0 {
		t.Fatalf("GpAndY allocs/op = %v, want 0", allocs)
	}
}
