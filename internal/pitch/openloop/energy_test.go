package openloop

import (
	"testing"

	"github.com/hunydev/g729/internal/fixed"
)

// TestEnergy_Zero pins eq. A.5 denominator trivial case: an all-zero
// wsp window must yield E(k) = 0 for every k ∈ [20,143].
func TestEnergy_Zero(t *testing.T) {
	var wsp [223]int16
	for k := 20; k <= 143; k++ {
		if got := energy(&wsp, k); got != 0 {
			t.Fatalf("energy(zero, k=%d) = %d, want 0", k, got)
		}
	}
}

// TestEnergy_ConstantInput pins the count of taps and the read pattern.
// With wsp[i] = 1024 ∀i, every read sw(2n−k) = 1024 so E(k) = 40·1024² =
// 41943040 for any k ∈ [20,143] (eq. A.5: 40 even-indexed taps).
func TestEnergy_ConstantInput(t *testing.T) {
	var wsp [223]int16
	for i := range wsp {
		wsp[i] = 1024
	}
	const want fixed.Word32 = 40 * 1024 * 1024
	for _, k := range []int{20, 39, 40, 79, 80, 100, 143} {
		if got := energy(&wsp, k); got != want {
			t.Fatalf("energy(const-1024, k=%d) = %d, want %d", k, got, want)
		}
	}
}

// TestEnergy_BoundaryReadPattern pins the index translation
// sw(2n − k) → wsp[143 + 2n − k]. Set a single non-zero sample at
// wsp[78]; for k = 143, n = 39 we have 143 + 2·39 − 143 = 78, so the
// last tap of the n-loop hits it. E(143) must equal 4096² = 16777216.
// Off-by-one in the index translation would miss this sample.
func TestEnergy_BoundaryReadPattern(t *testing.T) {
	var wsp [223]int16
	wsp[78] = 4096
	if got := energy(&wsp, 143); got != 4096*4096 {
		t.Fatalf("energy(impulse@78, k=143) = %d, want %d", got, 4096*4096)
	}
	// k = 142 reads wsp[143 + 2n − 142] = wsp[1 + 2n] (odd indices) for
	// n = 0..39 → never touches wsp[78] (even) → E = 0.
	if got := energy(&wsp, 142); got != 0 {
		t.Fatalf("energy(impulse@78, k=142) = %d, want 0", got)
	}
}

// TestEnergy_Saturation pins the Word32-saturating accumulator. With
// wsp[i] = 32767, each squared tap is 32767² ≈ 1.07e9; the sum of 40
// such terms is ≈ 4.29e10 which exceeds Max32 = 2^31 − 1. The
// accumulator must saturate (via fixed.LAdd) to Max32 rather than
// silently wrap. This is the Word32 ceiling that compareNormalized
// must tolerate without int64 overflow.
func TestEnergy_Saturation(t *testing.T) {
	var wsp [223]int16
	for i := range wsp {
		wsp[i] = 32767
	}
	if got := energy(&wsp, 20); got != fixed.Max32 {
		t.Fatalf("energy(sat, k=20) = %d, want Max32 = %d", got, fixed.Max32)
	}
}

// TestCompareNormalized_AbeatsB pins the R²/E maximization criterion of
// eq. A.5 in cross-multiplicative form. Candidate 1 has score
// R₁²/E₁ = 100²/10 = 1000; candidate 2 has 50²/100 = 25; cand1 wins.
func TestCompareNormalized_AbeatsB(t *testing.T) {
	if !compareNormalized(100, 10, 50, 100) {
		t.Fatalf("compareNormalized(100,10, 50,100) = false, want true (R²/E: 1000 ≥ 25)")
	}
	if compareNormalized(50, 100, 100, 10) {
		t.Fatalf("compareNormalized(50,100, 100,10) = true, want false (R²/E: 25 < 1000)")
	}
}

// TestCompareNormalized_TieBreak pins ≥ (not strict >): equal scores
// return true so the caller (OL-3) can implement lower-lag-wins by
// scanning ascending and replacing on strict-greater externally.
func TestCompareNormalized_TieBreak(t *testing.T) {
	// 200²/40 = 1000; 100²/10 = 1000. Equal scores.
	if !compareNormalized(200, 40, 100, 10) {
		t.Fatalf("compareNormalized tie (200²/40 vs 100²/10) = false, want true (≥)")
	}
	if !compareNormalized(100, 10, 200, 40) {
		t.Fatalf("compareNormalized tie (100²/10 vs 200²/40) = false, want true (≥)")
	}
}

// TestCompareNormalized_ZeroEnergy pins the E = 0 edge case. A
// candidate with E = 0 (and therefore R = 0 by Cauchy-Schwarz) is
// treated as score 0 — the worst possible. A non-zero score beats it.
func TestCompareNormalized_ZeroEnergy(t *testing.T) {
	// cand1 has positive score, cand2 zero-energy → cand1 wins.
	if !compareNormalized(100, 10, 0, 0) {
		t.Fatalf("compareNormalized(100,10, 0,0) = false, want true")
	}
	// cand1 zero-energy, cand2 positive → cand2 wins → return false.
	if compareNormalized(0, 0, 100, 10) {
		t.Fatalf("compareNormalized(0,0, 100,10) = true, want false")
	}
	// Both zero-energy → tie → true.
	if !compareNormalized(0, 0, 0, 0) {
		t.Fatalf("compareNormalized(0,0, 0,0) = false, want true (tie)")
	}
}

// TestCompareNormalized_MaxMagnitude pins overflow safety with the
// worst-case Word32 magnitudes that OL-1's correlate (saturating
// accumulator) and energy (saturating accumulator) can return. Naïve
// int64 R²·E would be 2³¹·2³¹·2³¹ = 2⁹³ — wraps catastrophically.
// Implementation must normalize. Here both scores are equal at the
// ceiling; ≥ must return true without overflow-induced sign flip.
func TestCompareNormalized_MaxMagnitude(t *testing.T) {
	if !compareNormalized(fixed.Max32, fixed.Max32, fixed.Max32, fixed.Max32) {
		t.Fatalf("compareNormalized(Max32,Max32, Max32,Max32) = false, want true (tie at ceiling)")
	}
	// Slightly perturb cand2 energy upward (impossible since Max32 is
	// the ceiling, but we drop cand1's R to half to make cand2 win).
	if compareNormalized(fixed.Max32/2, fixed.Max32, fixed.Max32, fixed.Max32) {
		t.Fatalf("compareNormalized((Max32/2)²·Max32 vs Max32²·Max32) = true, want false")
	}
}

// TestEnergy_NoAlloc enforces I4 on the energy hot path.
func TestEnergy_NoAlloc(t *testing.T) {
	var wsp [223]int16
	for i := range wsp {
		wsp[i] = int16(i & 0x3FF)
	}
	allocs := testing.AllocsPerRun(100, func() {
		_ = energy(&wsp, 80)
	})
	if allocs != 0 {
		t.Fatalf("energy allocates %v/op, want 0", allocs)
	}
}

// TestCompareNormalized_NoAlloc enforces I4 on the comparator.
func TestCompareNormalized_NoAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(1000, func() {
		_ = compareNormalized(fixed.Max32, 12345, 67890, fixed.Max32)
	})
	if allocs != 0 {
		t.Fatalf("compareNormalized allocates %v/op, want 0", allocs)
	}
}
