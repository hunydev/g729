package closedloop

import "testing"

// TestTargetSignal_ZeroResidualZeroMemory exercises the trivial case
// of an all-zero residual fed through any stable Â with zero filter
// memory: the all-pole recurrence x(n) = -Σ aw[i]·x(n-i) (driven by
// r(n) = 0) collapses to all-zero output.
//
// Spec: ITU-T G.729 Annex A §A.3.6 (G729E.txt lines 2119–2125):
// "The target signal x(n) ... is computed by filtering of the LP
// residual signal r(n) through the weighted synthesis filter
// 1/Â(z/γ)."
func TestTargetSignal_ZeroResidualZeroMemory(t *testing.T) {
	a := [11]int16{4096, -3500, 2800, -2100, 1500, -1000, 700, -400, 200, -100, 50}
	var r [SubframeLen]int16
	var mem [10]int16
	var x [SubframeLen]int16
	TargetSignal(&a, &r, &mem, &x)
	for n := 0; n < SubframeLen; n++ {
		if x[n] != 0 {
			t.Fatalf("x[%d] = %d, want 0", n, x[n])
		}
	}
}

// TestTargetSignal_IdentityFilterImpulseResidual pins the closed-form
// case Â(z) = 1 (and hence Â(z/γ) = 1, 1/Â(z/γ) = identity). The
// target signal degenerates to x(n) = r(n) for every sample.
//
// Spec: §A.3.6 (filter form); §A.3.3 line 2063 (γ = 0.75 — irrelevant
// here since Â has no taps beyond a[0]).
func TestTargetSignal_IdentityFilterImpulseResidual(t *testing.T) {
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var r [SubframeLen]int16
	for n := range r {
		r[n] = int16(n - 20) // mix of negative and positive
	}
	var mem [10]int16
	var x [SubframeLen]int16
	TargetSignal(&a, &r, &mem, &x)
	for n := 0; n < SubframeLen; n++ {
		if x[n] != r[n] {
			t.Fatalf("identity filter: x[%d]=%d, want r[%d]=%d", n, x[n], n, r[n])
		}
	}
}

// TestTargetSignal_FirstOrderHandTrace pins a closed-form trace for
// the order-1 weighted filter
//
//	Â(z) = 1 - 0.5 z⁻¹  →  Â(z/γ) = 1 - 0.375 z⁻¹  (γ = 0.75)
//
// driven by an impulse residual r = [100, 0, 0, ...] with zero memory.
// The all-pole recurrence x(n) = r(n) - aw[1]·x(n-1) with
// aw[1] = -1536 (Q12) yields:
//
//	x[0] = r[0] = 100
//	x[n] = -aw[1]·x[n-1] / 2^12  ≈ 0.375·x[n-1]    (for n ≥ 1)
//
// In Q0 with the same Q12 scaling used by the synthesis filter:
//
//	x[1] ≈ round(0.375·100) = 38
//	x[2] ≈ round(0.375·38)  = 14
//
// We assert the leading samples to pin the recurrence direction and
// the Q12 coefficient scale.
//
// Spec: §A.3.6 filter form; gammaPow[1] = 24576 (γ = 0.75 in Q15).
func TestTargetSignal_FirstOrderHandTrace(t *testing.T) {
	a := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var r [SubframeLen]int16
	r[0] = 100
	var mem [10]int16
	var x [SubframeLen]int16
	TargetSignal(&a, &r, &mem, &x)

	if x[0] != 100 {
		t.Fatalf("x[0] = %d, want 100", x[0])
	}
	if x[1] != 38 {
		t.Fatalf("x[1] = %d, want 38 (Q12 all-pole product)", x[1])
	}
	if x[2] != 14 {
		t.Fatalf("x[2] = %d, want 14", x[2])
	}
	if x[3] != 5 {
		t.Fatalf("x[3] = %d, want 5", x[3])
	}
	for n := 6; n < SubframeLen; n++ {
		if x[n] != 0 {
			t.Fatalf("x[%d] = %d, want 0 (decayed tail)", n, x[n])
		}
	}
}

// TestTargetSignal_MemoryContinuity asserts that splitting an
// 80-sample filter run into two 40-sample subframes — with the
// caller carrying the trailing 10 output samples of subframe-1 into
// swMem before subframe-2 — produces the same output as a single
// contiguous 80-sample run. This pins the per-subframe memory
// contract that the encoder driver relies on (§A.3.10 / I3).
//
// TargetSignal itself is read-only on swMem (mutation deferred to
// the encoder driver per plan §TG-1 step 3); this test simulates the
// driver by manually copying x[30:40] of subframe-1 into swMem
// before the second call.
//
// Spec: §A.3.10 lines 2204–2215 (memory update of weighted synthesis
// filter for the next subframe).
func TestTargetSignal_MemoryContinuity(t *testing.T) {
	a := [11]int16{4096, -3500, 2800, -2100, 1500, -1000, 700, -400, 200, -100, 50}

	var r80 [80]int16
	for n := range r80 {
		r80[n] = int16((n*7 + 3) % 41) // varied non-trivial input
	}

	// Reference: single 80-sample run via two TargetSignal calls
	// over a synthetic 80-tap version is awkward (filter is fixed
	// at 40). Instead, run the same recurrence inline as ground
	// truth using the same Q-format conventions the impl will use,
	// then compare two-subframe TargetSignal output to it.
	//
	// We emulate using TargetSignal twice (on two halves) and
	// verify the second half lines up by building the ground truth
	// via a single in-place run of TargetSignal across an 80-long
	// view: do this by chaining two calls using the canonical
	// memory-carry sequence the encoder will use.
	var memChain [10]int16
	var sub1 [SubframeLen]int16
	var r1 [SubframeLen]int16
	copy(r1[:], r80[0:40])
	TargetSignal(&a, &r1, &memChain, &sub1)
	// Carry: tail of sub1 becomes the synthesis-filter memory.
	var memCarry [10]int16
	copy(memCarry[:], sub1[30:40])
	var sub2 [SubframeLen]int16
	var r2 [SubframeLen]int16
	copy(r2[:], r80[40:80])
	TargetSignal(&a, &r2, &memCarry, &sub2)

	// Independent recompute as ground truth: run the all-pole
	// recurrence directly with extended history.
	var hist [80]int16
	var aw [11]int16
	aw[0] = a[0]
	// Mirror the package's gamma weighting (cannot import its
	// internal var here — reconstruct from the published γ Q15 LUT).
	gamma := [11]int16{32767, 24576, 18432, 13824, 10368, 7776, 5832, 4374, 3281, 2460, 1845}
	for i := 1; i <= 10; i++ {
		aw[i] = int16((int32(a[i]) * int32(gamma[i])) >> 15)
	}
	for n := 0; n < 80; n++ {
		acc := int64(r80[n]) * int64(aw[0]) * 2
		for i := 1; i <= 10; i++ {
			if n-i >= 0 {
				acc -= int64(aw[i]) * int64(hist[n-i]) * 2
			}
		}
		acc <<= 3
		v := int32((acc + 0x8000) >> 16)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		hist[n] = int16(v)
	}

	for n := 0; n < SubframeLen; n++ {
		if sub1[n] != hist[n] {
			t.Fatalf("sub1[%d] = %d, want %d", n, sub1[n], hist[n])
		}
	}
	for n := 0; n < SubframeLen; n++ {
		if sub2[n] != hist[40+n] {
			t.Fatalf("sub2[%d] = %d, want %d (memory continuity broken)",
				n, sub2[n], hist[40+n])
		}
	}
}

// TestTargetSignal_DoesNotMutateMemory pins the read-only contract on
// swMem (plan §TG-1 step 3 — mutation deferred to the encoder driver
// per §A.3.10 + I3).
func TestTargetSignal_DoesNotMutateMemory(t *testing.T) {
	a := [11]int16{4096, -3500, 2800, -2100, 1500, -1000, 700, -400, 200, -100, 50}
	var r [SubframeLen]int16
	for n := range r {
		r[n] = int16(n - 20)
	}
	mem := [10]int16{1, -2, 3, -4, 5, -6, 7, -8, 9, -10}
	memCopy := mem
	var x [SubframeLen]int16
	TargetSignal(&a, &r, &mem, &x)
	if mem != memCopy {
		t.Fatalf("TargetSignal mutated swMem: got %v, want %v", mem, memCopy)
	}
}
