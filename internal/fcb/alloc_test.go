package fcb

import "testing"

func TestNoAllocationInClampPitchGainForEnhancement(t *testing.T) {
	allocs := testing.AllocsPerRun(128, func() {
		_ = ClampPitchGainForEnhancement(8000)
	})
	if allocs != 0 {
		t.Fatalf("ClampPitchGainForEnhancement allocated %.2f times per call; want 0", allocs)
	}
}

func TestNoAllocationInDecode_NoEnhancement(t *testing.T) {
	idx := Indices{Positions: 0x1234, Signs: 0b1010}
	var c [40]int16
	allocs := testing.AllocsPerRun(128, func() {
		Decode(idx, 40, 0, &c)
	})
	if allocs != 0 {
		t.Fatalf("Decode (β=0 path) allocated %.2f times per call; want 0", allocs)
	}
}

func TestNoAllocationInDecode_WithEnhancement(t *testing.T) {
	idx := Indices{Positions: 0x1234, Signs: 0b1010}
	var c [40]int16
	allocs := testing.AllocsPerRun(128, func() {
		Decode(idx, 30, 10000, &c)
	})
	if allocs != 0 {
		t.Fatalf("Decode (with enhancement) allocated %.2f times per call; want 0", allocs)
	}
}
