package pitch

import "testing"

func TestIndicesZeroValue(t *testing.T) {
	var idx Indices
	if idx.P1 != 0 || idx.P0 != 0 || idx.P2 != 0 {
		t.Fatalf("zero-value Indices = %+v, want all zero", idx)
	}
}
