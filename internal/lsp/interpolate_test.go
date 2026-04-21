package lsp

import "testing"

func TestInterpolateSubframe1Midpoint(t *testing.T) {
	prev := [10]int16{0, 100, 200, 300, 400, 500, 600, 700, 800, 900}
	curr := [10]int16{200, 300, 400, 500, 600, 700, 800, 900, 1000, 1100}
	var sf1, sf2 [10]int16
	interpolateLSP(&prev, &curr, &sf1, &sf2)
	for i := 0; i < 10; i++ {
		want := (prev[i] + curr[i]) / 2
		if sf1[i] != want {
			t.Errorf("sf1[%d] = %d, want %d (midpoint)", i, sf1[i], want)
		}
		if sf2[i] != curr[i] {
			t.Errorf("sf2[%d] = %d, want %d (current)", i, sf2[i], curr[i])
		}
	}
}

func TestInterpolateNegativeRange(t *testing.T) {
	prev := [10]int16{-32000, -24000, -16000, -8000, 0, 8000, 16000, 24000, 32000, 32500}
	curr := [10]int16{-31000, -23000, -15000, -7000, 1000, 9000, 17000, 25000, 32100, 32600}
	var sf1, sf2 [10]int16
	interpolateLSP(&prev, &curr, &sf1, &sf2)
	for i := 0; i < 10; i++ {
		want := (int32(prev[i]) + int32(curr[i])) / 2
		if int32(sf1[i]) != want {
			t.Errorf("sf1[%d] = %d, want %d", i, sf1[i], want)
		}
	}
}
