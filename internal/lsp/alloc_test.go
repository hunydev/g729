package lsp

import "testing"

func TestNoAllocationInDecode(t *testing.T) {
	var d Decoder
	idx := Indices{L0: 0, L1: 5, L2: 10, L3: 15}
	allocs := testing.AllocsPerRun(128, func() {
		d.Decode(idx)
	})
	if allocs != 0 {
		t.Fatalf("Decoder.Decode allocated %.2f times per call; want 0", allocs)
	}
}
