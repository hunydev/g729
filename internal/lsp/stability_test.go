package lsp

import "testing"

func TestStabilityAlreadyMonotonic(t *testing.T) {
	in := [10]int16{2000, 4000, 6000, 8000, 10000, 12000, 14000, 16000, 18000, 20000}
	out := in
	enforceLSFStability(&out)
	if out != in {
		t.Fatalf("stable input was modified: got %v, want %v", out, in)
	}
}

func TestStabilityOutOfOrder(t *testing.T) {
	in := [10]int16{5000, 3000, 4000, 6000, 8000, 10000, 12000, 14000, 16000, 18000}
	enforceLSFStability(&in)
	for i := 1; i < 10; i++ {
		if in[i] <= in[i-1] {
			t.Fatalf("after enforce, not strictly monotone at i=%d: %v", i, in)
		}
	}
}

func TestStabilityTooClose(t *testing.T) {
	in := [10]int16{2000, 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009}
	enforceLSFStability(&in)
	const minGap = 320 // ITU §3.2.4: J = 0.0391 rad ≈ 320 in Q13
	for i := 1; i < 10; i++ {
		if in[i]-in[i-1] < minGap {
			t.Fatalf("gap at i=%d is %d, want >= %d: %v", i, in[i]-in[i-1], minGap, in)
		}
	}
}

func TestRearrangeAdjacentTooClose(t *testing.T) {
	// Adjacent pair within J → both moved so their gap equals J.
	const J int16 = 10
	in := [10]int16{1000, 1005, 4000, 6000, 8000, 10000, 12000, 14000, 16000, 18000}
	rearrangeAdjacent(&in, J)
	if g := in[1] - in[0]; g < J {
		t.Errorf("after rearrange, gap[0..1] = %d < J = %d (in=%v)", g, J, in)
	}
}

func TestRearrangeAdjacentNoChangeWhenSpaced(t *testing.T) {
	const J int16 = 10
	in := [10]int16{1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000}
	out := in
	rearrangeAdjacent(&out, J)
	if out != in {
		t.Fatalf("well-spaced input modified: got %v, want %v", out, in)
	}
}
