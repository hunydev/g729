package openloop

import (
	"math"
	"testing"
)

// fillPeriodicSine populates wsp with a period-N sinusoid scaled to
// ±2000 so that R(k) = Σ sw(2n)·sw(2n−k) over 40 taps stays within
// the Word32 dynamic range without saturating LMac (worst case
// 40 · 2·2000² ≈ 3.2·10⁸ ≪ 2³¹).
func fillPeriodicSine(wsp *[223]int16, period float64) {
	for i := range wsp {
		wsp[i] = int16(math.Round(2000 * math.Sin(2*math.Pi*float64(i)/period)))
	}
}

// TestSearch_PeriodicInput_Period100 covers the §A.3.4 composition
// smoke: a clean period-100 sinusoid feeds the three OL-3 ranges. With
// the Annex A raw-correlation per-range maximum, finite-window rounding
// makes lag 97 the retained high-range candidate.
func TestSearch_PeriodicInput_Period100(t *testing.T) {
	var wsp [223]int16
	fillPeriodicSine(&wsp, 100)
	got := Search(&wsp)
	if got != 97 {
		t.Fatalf("Search(period-100 sine) = %d, want 97", got)
	}
	heuristic := SearchWithRangesNormalized(&wsp).Top
	if heuristic < 99 || heuristic > 101 {
		t.Fatalf("SearchWithRangesNormalized(period-100 sine) = %d, want 100±1", heuristic)
	}
}

// TestSearch_PeriodicInput_Period40 exercises the [40,79] range as the
// dominant candidate. Period 40: R(40)=1, R(20)=R(60)=R(80)=1, so the
// sub-multiple lift in the merger pushes the answer toward 20 (lower
// range). Verifies the cross-range merge participates correctly: the
// returned lag must be near either 20 (merger lifts) or 40 (no lift).
func TestSearch_PeriodicInput_Period40(t *testing.T) {
	var wsp [223]int16
	fillPeriodicSine(&wsp, 40)
	got := Search(&wsp)
	if !(got >= 19 && got <= 21) && !(got >= 39 && got <= 41) {
		t.Fatalf("Search(period-40 sine) = %d, want 20±1 or 40±1", got)
	}
}

// TestSearch_RangeBounds checks the §A.3.4 contract that T_op always
// lies in the canonical [20, 143] open-loop range, regardless of the
// input statistics. Three diverse inputs are exercised: a flat ramp,
// random-ish values, and a period-100 sine.
func TestSearch_RangeBounds(t *testing.T) {
	cases := []struct {
		name string
		fill func(*[223]int16)
	}{
		{"ramp", func(w *[223]int16) {
			for i := range w {
				w[i] = int16(i)
			}
		}},
		{"checker", func(w *[223]int16) {
			for i := range w {
				w[i] = int16((i & 0x3FF) - 0x200)
			}
		}},
		{"sine100", func(w *[223]int16) { fillPeriodicSine(w, 100) }},
	}
	for _, c := range cases {
		var wsp [223]int16
		c.fill(&wsp)
		got := Search(&wsp)
		if got < 20 || got > 143 {
			t.Fatalf("%s: Search returned %d, want lag ∈ [20,143]", c.name, got)
		}
	}
}

// TestSearch_NoAlloc enforces I4 across the full §A.3.4 composition
// (three OL-3 scans + OL-4 merger).
func TestSearch_NoAlloc(t *testing.T) {
	var wsp [223]int16
	fillPeriodicSine(&wsp, 100)
	allocs := testing.AllocsPerRun(50, func() {
		_ = Search(&wsp)
	})
	if allocs != 0 {
		t.Fatalf("Search allocates %v/op, want 0", allocs)
	}
}
