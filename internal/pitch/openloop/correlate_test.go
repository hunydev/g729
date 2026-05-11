package openloop

import "testing"

// TestCorrelate_Zero pins eq. A.4 trivial case: an all-zero wsp window
// must yield R(k) = 0 for every k ∈ [20,143] so the kernel returns
// rsq = 0. The lag value is unconstrained (no positive R(k) candidate
// to choose between); we only assert rsq == 0.
func TestCorrelate_Zero(t *testing.T) {
	var wsp [223]int16
	_, rsq := correlate(&wsp, 20, 143)
	if rsq != 0 {
		t.Fatalf("correlate(zero) rsq = %d, want 0", rsq)
	}
}

// TestCorrelate_ImpulseAtBoundary pins the read pattern of eq. A.4 at
// the history/current boundary. Setting only wsp[143] = 4096 (the
// first sample of the current frame) means the left factor sw(2n) is
// non-zero only at n = 0, and the right factor sw(2n − k) for k ≥ 20
// indexes wsp[143 + 2n − k] which for n = 0..39, k ∈ [20,143] never
// returns to wsp[143] (it lands in wsp[0..142] = history zeros, or
// in even-indexed wsp[145..221] = 0). Therefore R(k) must be 0 for
// all k ∈ [20,143]. This catches off-by-one read errors at the
// boundary index 143 (max-lag pinning per §A.3.4 line 2098).
func TestCorrelate_ImpulseAtBoundary(t *testing.T) {
	var wsp [223]int16
	wsp[143] = 4096
	_, rsq := correlate(&wsp, 20, 143)
	if rsq != 0 {
		t.Fatalf("correlate(impulse@143) rsq = %d, want 0 for k∈[20,143]", rsq)
	}
}

// TestCorrelate_Period80 pins eq. A.4 against a period-80 input: a
// square wave of period 80 (40 positive, 40 negative samples) over
// the whole wsp window. R(k = 80) = Σ wsp(2n)·wsp(2n) = max positive;
// R(k = 40) = -Σ wsp²(2n) is negative; other lags in [20,143] yield
// strictly smaller R. The kernel must therefore return lag == 80.
func TestCorrelate_Period80(t *testing.T) {
	var wsp [223]int16
	for i := 0; i < 223; i++ {
		if (i % 80) < 40 {
			wsp[i] = 1000
		} else {
			wsp[i] = -1000
		}
	}
	lag, rsq := correlate(&wsp, 20, 143)
	if lag != 80 {
		t.Fatalf("correlate(period-80) lag = %d, want 80", lag)
	}
	if rsq <= 0 {
		t.Fatalf("correlate(period-80) rsq = %d, want > 0", rsq)
	}
}

// TestCorrelate_AllNegativeCorrelationsStillMaximizesR pins the raw
// §A.3.4 per-range maximum. Even if every R(k) is negative, correlate
// must select the least-negative candidate rather than falling back to
// the lower bound as if all negative values were zero.
func TestCorrelate_AllNegativeCorrelationsStillMaximizesR(t *testing.T) {
	var wsp [223]int16
	wsp[143] = 100
	wsp[143-20] = -5
	wsp[143-21] = -2
	wsp[143-22] = -9
	lag, rsq := correlate(&wsp, 20, 22)
	if lag != 21 {
		t.Fatalf("correlate(all-negative) lag = %d, want 21 (least-negative R)", lag)
	}
	if rsq != -400 {
		t.Fatalf("correlate(all-negative) R = %d, want -400 (= 2*100*-2)", rsq)
	}
}

// TestCorrelate_NoAlloc enforces I4 on the hot path.
func TestCorrelate_NoAlloc(t *testing.T) {
	var wsp [223]int16
	for i := 0; i < 223; i++ {
		wsp[i] = int16(i & 0x3FF)
	}
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = correlate(&wsp, 20, 143)
	})
	if allocs != 0 {
		t.Fatalf("correlate allocates %v/op, want 0", allocs)
	}
}
