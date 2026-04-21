package gain

import "testing"

func TestNoAllocationInDecode(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[5] = 8192
	idx := Indices{GA: 3, GB: 7}

	allocs := testing.AllocsPerRun(128, func() {
		_, _ = d.Decode(idx, &c)
	})
	if allocs != 0 {
		t.Fatalf("Decode allocated %.2f times per call; want 0", allocs)
	}
}

func TestNoAllocationInReset(t *testing.T) {
	var d Decoder
	allocs := testing.AllocsPerRun(128, func() {
		d.Reset()
	})
	if allocs != 0 {
		t.Fatalf("Reset allocated %.2f times per call; want 0", allocs)
	}
}
