package fcbsearch_test

import (
	"testing"

	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/fcbsearch"
)

// CB-5 RED tests for §3.9 eq. 64 (G729E.txt §3.9, line ~1340):
//
//	z(n) = Σ_{i=0..n} c(i) · h(n − i)        n = 0,...,39
//
// Q-format. c is Q13 (CB-4 convention; PulseAmplitude = 8192 = +1.0 in
// Q13). h is Q12 (HI-1 impulse-response convention). The product
// c(i)·h(n−i) accumulates in Q25; the eq. 64 result z lands in Q12
// after an arithmetic right-shift by 13 with Word16 saturation.
//
// Pulse-amplitude convenience constant (mirrors code_test.go).
const cbFiltPAmp = fcb.PulseAmplitude // 8192, Q13

// hRamp returns h[n] = n+1 (Q12 abstract — values are tiny so no
// saturation can occur). With c containing a single Q13 unit pulse at
// position k, z(n) reduces to h(n−k) for n≥k because
// (8192 * (n−k+1)) >> 13 == (n−k+1).
func hRamp() [40]int16 {
	var h [40]int16
	for n := 0; n < 40; n++ {
		h[n] = int16(n + 1)
	}
	return h
}

// TestFilterCode_DeltaAtZero: c = δ_0 ⇒ z(n) = h(n).
func TestFilterCode_DeltaAtZero(t *testing.T) {
	var c [40]int16
	c[0] = +cbFiltPAmp
	h := hRamp()
	var z [40]int16
	fcbsearch.FilterCode(&c, &h, &z)

	for n := 0; n < 40; n++ {
		want := h[n]
		if z[n] != want {
			t.Fatalf("z[%d] = %d, want %d (h[n] for δ_0)", n, z[n], want)
		}
	}
}

// TestFilterCode_DeltaAtK: c = δ_k ⇒ z(n) = h(n−k) for n≥k, else 0.
func TestFilterCode_DeltaAtK(t *testing.T) {
	const k = 7
	var c [40]int16
	c[k] = +cbFiltPAmp
	h := hRamp()
	var z [40]int16
	fcbsearch.FilterCode(&c, &h, &z)

	for n := 0; n < 40; n++ {
		var want int16
		if n >= k {
			want = h[n-k]
		}
		if z[n] != want {
			t.Fatalf("z[%d] = %d, want %d (δ_%d)", n, z[n], want, k)
		}
	}
}

// TestFilterCode_TwoPulseGolden: c = +δ_0 − δ_5; with h[n]=n+1,
// z(n) = h(n) for n<5 and z(n) = h(n) − h(n−5) = 5 for n≥5.
func TestFilterCode_TwoPulseGolden(t *testing.T) {
	var c [40]int16
	c[0] = +cbFiltPAmp
	c[5] = -cbFiltPAmp
	h := hRamp()
	var z [40]int16
	fcbsearch.FilterCode(&c, &h, &z)

	for n := 0; n < 40; n++ {
		var want int16
		if n < 5 {
			want = h[n]
		} else {
			want = 5
		}
		if z[n] != want {
			t.Fatalf("z[%d] = %d, want %d (two-pulse golden)", n, z[n], want)
		}
	}
}

// TestFilterCode_NegativeSign: c = −δ_3 ⇒ z(n) = −h(n−3) for n≥3.
func TestFilterCode_NegativeSign(t *testing.T) {
	const k = 3
	var c [40]int16
	c[k] = -cbFiltPAmp
	h := hRamp()
	var z [40]int16
	fcbsearch.FilterCode(&c, &h, &z)

	for n := 0; n < 40; n++ {
		var want int16
		if n >= k {
			want = -h[n-k]
		}
		if z[n] != want {
			t.Fatalf("z[%d] = %d, want %d (−δ_%d)", n, z[n], want, k)
		}
	}
}

// TestFilterCode_CB4Trace cross-checks CB-5 against CB-4 output: feed
// BuildCode's c[40] (positions={0,6,12,23}, all signs +1, T=40 so
// enhancement is bypassed per eq. 48) into FilterCode with h[n]=n+1.
// The Q13 pulses collapse to ±1 unit each via the >>13 shift, so the
// expected z(n) is the lower-triangular sum Σ_{p∈pulses, p≤n} h(n−p).
func TestFilterCode_CB4Trace(t *testing.T) {
	positions := [4]int8{0, 6, 12, 23}
	var signs [40]int16
	signs[0] = +1
	signs[6] = +1
	signs[12] = +1
	signs[23] = +1

	var c [40]int16
	fcbsearch.BuildCode(&positions, &signs, 40, 8192, &c)
	h := hRamp()
	var z [40]int16
	fcbsearch.FilterCode(&c, &h, &z)

	pulses := []int{0, 6, 12, 23}
	for n := 0; n < 40; n++ {
		var want int32
		for _, p := range pulses {
			if p <= n {
				want += int32(h[n-p])
			}
		}
		if int32(z[n]) != want {
			t.Fatalf("z[%d] = %d, want %d (CB-4 trace cross-check)", n, z[n], want)
		}
	}
	// Spot-check a few hand-computed indices.
	cases := map[int]int16{
		0:  1,   // h[0]
		6:  8,   // h[6]+h[0] = 7+1
		12: 21,  // h[12]+h[6]+h[0] = 13+7+1
		23: 55,  // h[23]+h[17]+h[11]+h[0] = 24+18+12+1
		39: 119, // h[39]+h[33]+h[27]+h[16] = 40+34+28+17
	}
	for n, want := range cases {
		if z[n] != want {
			t.Fatalf("z[%d] = %d, want %d (hand-computed)", n, z[n], want)
		}
	}
}

// TestFilterCode_AllocsZero gates I3 / I4: caller-owned outputs, no
// hidden allocations.
func TestFilterCode_AllocsZero(t *testing.T) {
	var c [40]int16
	c[0] = +cbFiltPAmp
	c[5] = -cbFiltPAmp
	c[12] = +cbFiltPAmp
	c[23] = +cbFiltPAmp
	h := hRamp()
	var z [40]int16

	if got := testing.AllocsPerRun(128, func() {
		fcbsearch.FilterCode(&c, &h, &z)
	}); got != 0 {
		t.Fatalf("FilterCode allocations/op = %v, want 0", got)
	}
}
