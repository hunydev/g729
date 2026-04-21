package tables

import "testing"

func TestLSPCodebookL1Shape(t *testing.T) {
	if len(LSPCodebookL1) != 128 {
		t.Fatalf("LSPCodebookL1: rows = %d, want 128", len(LSPCodebookL1))
	}
	for i, row := range LSPCodebookL1 {
		if len(row) != 10 {
			t.Fatalf("LSPCodebookL1[%d]: cols = %d, want 10", i, len(row))
		}
	}
}

func TestLSPCodebookL1Range(t *testing.T) {
	const cap = 2 * 25736
	for i, row := range LSPCodebookL1 {
		for j, v := range row {
			if int(v) > cap || int(v) < -cap {
				t.Errorf("LSPCodebookL1[%d][%d] = %d out of sane range ±%d",
					i, j, v, cap)
			}
		}
	}
}
