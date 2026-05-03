package closedloop

import "testing"

// BenchmarkTargetSignal exercises the §A.3.6 filter on a
// representative aHat / residual / memory tuple and feeds the alloc
// gate (TestTargetSignal_ZeroAlloc) for I4.
func BenchmarkTargetSignal(b *testing.B) {
	a := [11]int16{4096, -3500, 2800, -2100, 1500, -1000, 700, -400, 200, -100, 50}
	var r [SubframeLen]int16
	for n := range r {
		r[n] = int16((n*13 + 7) % 97)
	}
	mem := [10]int16{1, -2, 3, -4, 5, -6, 7, -8, 9, -10}
	var x [SubframeLen]int16
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		TargetSignal(&a, &r, &mem, &x)
	}
}

// TestTargetSignal_ZeroAlloc enforces I4: TargetSignal must not
// allocate on the heap.
func TestTargetSignal_ZeroAlloc(t *testing.T) {
	a := [11]int16{4096, -3500, 2800, -2100, 1500, -1000, 700, -400, 200, -100, 50}
	var r [SubframeLen]int16
	for n := range r {
		r[n] = int16((n*13 + 7) % 97)
	}
	mem := [10]int16{1, -2, 3, -4, 5, -6, 7, -8, 9, -10}
	var x [SubframeLen]int16
	allocs := testing.AllocsPerRun(64, func() {
		TargetSignal(&a, &r, &mem, &x)
	})
	if allocs != 0 {
		t.Fatalf("TargetSignal allocs/op = %v, want 0", allocs)
	}
}
