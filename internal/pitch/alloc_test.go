package pitch

import "testing"

func TestNoAllocationInCheckParity(t *testing.T) {
	allocs := testing.AllocsPerRun(128, func() {
		_ = CheckParity(123, 0)
	})
	if allocs != 0 {
		t.Fatalf("CheckParity allocated %.2f times per call; want 0", allocs)
	}
}

func TestNoAllocationInDecodeDelaySubframe1(t *testing.T) {
	allocs := testing.AllocsPerRun(128, func() {
		_, _ = DecodeDelaySubframe1(57)
	})
	if allocs != 0 {
		t.Fatalf("DecodeDelaySubframe1 allocated %.2f times per call; want 0", allocs)
	}
}

func TestNoAllocationInDecodeDelaySubframe2(t *testing.T) {
	allocs := testing.AllocsPerRun(128, func() {
		_, _ = DecodeDelaySubframe2(15, 60)
	})
	if allocs != 0 {
		t.Fatalf("DecodeDelaySubframe2 allocated %.2f times per call; want 0", allocs)
	}
}

func TestNoAllocationInAdaptiveCodebook(t *testing.T) {
	var pastExc [250]int16
	for i := range pastExc {
		pastExc[i] = int16(i)
	}
	var v [40]int16
	slice := pastExc[:]
	allocs := testing.AllocsPerRun(128, func() {
		AdaptiveCodebook(60, 1, slice, &v)
	})
	if allocs != 0 {
		t.Fatalf("AdaptiveCodebook allocated %.2f times per call; want 0", allocs)
	}
}
