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

func TestLSPCodebookL2Shape(t *testing.T) {
if len(LSPCodebookL2) != 32 {
t.Fatalf("LSPCodebookL2: rows = %d, want 32", len(LSPCodebookL2))
}
for i, row := range LSPCodebookL2 {
if len(row) != 5 {
t.Fatalf("LSPCodebookL2[%d]: cols = %d, want 5", i, len(row))
}
}
}

func TestLSPCodebookL2Range(t *testing.T) {
const cap = 2 * 25736
for i, row := range LSPCodebookL2 {
for j, v := range row {
if int(v) > cap || int(v) < -cap {
t.Errorf("LSPCodebookL2[%d][%d] = %d out of sane range ±%d",
i, j, v, cap)
}
}
}
}

func TestLSPCodebookL3Shape(t *testing.T) {
if len(LSPCodebookL3) != 32 {
t.Fatalf("LSPCodebookL3: rows = %d, want 32", len(LSPCodebookL3))
}
for i, row := range LSPCodebookL3 {
if len(row) != 5 {
t.Fatalf("LSPCodebookL3[%d]: cols = %d, want 5", i, len(row))
}
}
}

func TestLSPCodebookL3Range(t *testing.T) {
const cap = 2 * 25736
for i, row := range LSPCodebookL3 {
for j, v := range row {
if int(v) > cap || int(v) < -cap {
t.Errorf("LSPCodebookL3[%d][%d] = %d out of sane range ±%d",
i, j, v, cap)
}
}
}
}

func TestMAPredictorsLSPShape(t *testing.T) {
if len(MAPredictorsLSP) != 2 {
t.Fatalf("MAPredictorsLSP: selectors = %d, want 2", len(MAPredictorsLSP))
}
for s, sel := range MAPredictorsLSP {
if len(sel) != 4 {
t.Fatalf("MAPredictorsLSP[%d]: taps = %d, want 4", s, len(sel))
}
for k, tap := range sel {
if len(tap) != 10 {
t.Fatalf("MAPredictorsLSP[%d][%d]: dims = %d, want 10", s, k, len(tap))
}
}
}
}

func TestMAPredictorsLSPRange(t *testing.T) {
for s, sel := range MAPredictorsLSP {
for k, tap := range sel {
for i, v := range tap {
if v > 32767 || v < -32768 {
t.Errorf("MAPredictorsLSP[%d][%d][%d] = %d out of Q15 range",
s, k, i, v)
}
}
}
}
}
