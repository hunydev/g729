package closedloop

import "testing"

// BenchmarkImpulseResponse exercises the §A.3.5 filter on a
// representative aHat vector and is used by the alloc gate to assert
// zero allocations per call (I4).
func BenchmarkImpulseResponse(b *testing.B) {
	a := [11]int16{4096, -3500, 2800, -2100, 1500, -1000, 700, -400, 200, -100, 50}
	var h [SubframeLen]int16
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ImpulseResponse(&a, &h)
	}
}

// TestImpulseResponse_ZeroAlloc enforces I4: ImpulseResponse must not
// allocate on the heap.
func TestImpulseResponse_ZeroAlloc(t *testing.T) {
	a := [11]int16{4096, -3500, 2800, -2100, 1500, -1000, 700, -400, 200, -100, 50}
	var h [SubframeLen]int16
	allocs := testing.AllocsPerRun(64, func() {
		ImpulseResponse(&a, &h)
	})
	if allocs != 0 {
		t.Fatalf("ImpulseResponse allocs/op = %v, want 0", allocs)
	}
}
