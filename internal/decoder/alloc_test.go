package decoder

import "testing"

func TestDecodeZeroAllocations(t *testing.T) {
	var d Decoder
	var packed [10]byte
	var out [80]int16

	allocs := testing.AllocsPerRun(100, func() {
		_ = d.Decode(packed[:], false, out[:])
	})
	if allocs != 0 {
		t.Fatalf("Decode allocates %.0f times per call, want 0", allocs)
	}
}
