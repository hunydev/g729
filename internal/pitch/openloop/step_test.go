package openloop

import "testing"

func TestStepSplit_MatchesStepWhenCoefficientsEqual(t *testing.T) {
	aHat := [11]int16{4096, -1200, 800, -500, 300, -180, 100, -60, 30, -10, 4}
	var s [80]int16
	for i := range s {
		s[i] = int16((i*97)%3000 - 1500)
	}
	var residualMem1, residualMem2 [10]int16
	var swMem1, swMem2 [10]int16
	var old1, old2 [143]int16
	for i := range residualMem1 {
		residualMem1[i] = int16(i*11 - 50)
		residualMem2[i] = residualMem1[i]
		swMem1[i] = int16(40 - i*7)
		swMem2[i] = swMem1[i]
	}
	for i := range old1 {
		old1[i] = int16((i*13)%200 - 100)
		old2[i] = old1[i]
	}

	top1 := Step(&aHat, &s, &residualMem1, &swMem1, &old1)
	top2 := StepSplit(&aHat, &aHat, &s, &residualMem2, &swMem2, &old2)

	if top1 != top2 {
		t.Fatalf("StepSplit top = %d, want %d", top2, top1)
	}
	if residualMem1 != residualMem2 {
		t.Fatalf("residualMem mismatch\n step=%v\nsplit=%v", residualMem1, residualMem2)
	}
	if swMem1 != swMem2 {
		t.Fatalf("swMem mismatch\n step=%v\nsplit=%v", swMem1, swMem2)
	}
	if old1 != old2 {
		t.Fatalf("oldWspeech mismatch\n step=%v\nsplit=%v", old1, old2)
	}
}
