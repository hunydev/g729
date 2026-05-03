package lsp

import "testing"

// TestApplyPredictorWithMemoryMatchesDecoder verifies that the
// non-destructive encoder-side evaluator produces the identical output
// as Decoder.applyPredictor for the same (memory, selector, residual)
// triple, but does NOT mutate the supplied memory.
func TestApplyPredictorWithMemoryMatchesDecoder(t *testing.T) {
	var seedMem [4][10]int16
	for k := 0; k < 4; k++ {
		for i := 0; i < 10; i++ {
			seedMem[k][i] = int16((k+1)*37 + i*11)
		}
	}
	var residual [10]int16
	for i := range residual {
		residual[i] = int16(200 + i*15)
	}

	for _, sel := range []uint8{0, 1} {
		var d Decoder
		d.pastResiduals = seedMem
		var decOut [10]int16
		d.applyPredictor(sel, &residual, &decOut)

		mem := seedMem
		var encOut [10]int16
		applyPredictorWithMemory(sel, &mem, &residual, &encOut)

		if encOut != decOut {
			t.Fatalf("selector %d: encoder output %v != decoder output %v", sel, encOut, decOut)
		}
		if mem != seedMem {
			t.Fatalf("selector %d: applyPredictorWithMemory mutated memory", sel)
		}
	}
}

// TestCommitPredictorMemoryAdvancesFIFO verifies that after
// commitPredictorMemory, the non-destructive evaluator matches the
// decoder's post-mutation state on a subsequent residual.
func TestCommitPredictorMemoryAdvancesFIFO(t *testing.T) {
	var seedMem [4][10]int16
	for k := 0; k < 4; k++ {
		for i := 0; i < 10; i++ {
			seedMem[k][i] = int16((k+2)*23 + i*7)
		}
	}
	var r1, r2 [10]int16
	for i := range r1 {
		r1[i] = int16(150 + i*9)
		r2[i] = int16(80 - i*3)
	}

	const sel uint8 = 1

	var d Decoder
	d.pastResiduals = seedMem
	var decOut1, decOut2 [10]int16
	d.applyPredictor(sel, &r1, &decOut1)
	d.applyPredictor(sel, &r2, &decOut2)

	mem := seedMem
	var encOut1, encOut2 [10]int16
	applyPredictorWithMemory(sel, &mem, &r1, &encOut1)
	commitPredictorMemory(&mem, &r1)
	applyPredictorWithMemory(sel, &mem, &r2, &encOut2)
	commitPredictorMemory(&mem, &r2)

	if encOut1 != decOut1 {
		t.Fatalf("first call mismatch: enc=%v dec=%v", encOut1, decOut1)
	}
	if encOut2 != decOut2 {
		t.Fatalf("second call mismatch (post-commit): enc=%v dec=%v", encOut2, decOut2)
	}
	if mem != d.pastResiduals {
		t.Fatalf("memory after commit != decoder state: enc=%v dec=%v", mem, d.pastResiduals)
	}
}
