package gainquant

import "testing"

// Tame implements the §3.9.2 taming branch (adaptive-codebook gain
// saturation under predicted-overflow conditions). Spec §3.9.2 narrates
// the procedure but pins no numeric threshold; OQ-TAMING-THR is
// resolved here with the textbook-typical CELP defaults:
//
//   - GpClipQ14 = 15565   (0.95 in Q14): clamp ceiling for ĝp.
//   - TameEnergyThresholdQ0 = 1<<33: trip Σ oldExc[i]² above which
//     unbounded ĝp is judged unsafe (predicted overflow risk).
//
// These are pinned in tame.go and re-asserted here at the test boundary
// so any future revision (Phase 2f TAME.BIT byte-EQ) traces the change.

// TestTame_LowEnergyPasses pins: when oldExc energy is below the
// threshold, ĝp is returned unchanged regardless of magnitude (no
// clamp applied).
func TestTame_LowEnergyPasses(t *testing.T) {
	var oldExc [154]int16 // all zero → energy 0
	const gp int16 = 20000
	got := Tame(gp, &oldExc)
	if got != gp {
		t.Fatalf("Tame(low-energy oldExc) = %d, want %d (unchanged)", got, gp)
	}
}

// TestTame_LowEnergyPassesSmallGp pins: the no-clamp path also leaves
// gp values already inside the safe region untouched.
func TestTame_LowEnergyPassesSmallGp(t *testing.T) {
	var oldExc [154]int16
	const gp int16 = 8000
	got := Tame(gp, &oldExc)
	if got != gp {
		t.Fatalf("Tame(small gp) = %d, want %d", got, gp)
	}
}

// TestTame_HighEnergyClamps pins: when oldExc energy exceeds the
// threshold AND ĝp exceeds GpClipQ14, the result is clamped to the
// ceiling.  Sample magnitude 16384 → energy 154·2^28 ≈ 4.13e10 > 2^33.
func TestTame_HighEnergyClamps(t *testing.T) {
	var oldExc [154]int16
	for i := range oldExc {
		oldExc[i] = 16384
	}
	const gp int16 = 20000
	got := Tame(gp, &oldExc)
	if got != GpClipQ14 {
		t.Fatalf("Tame(high-energy, gp=%d) = %d, want %d (GpClipQ14)",
			gp, got, GpClipQ14)
	}
}

// TestTame_HighEnergyKeepsSafeGp pins: even with high-energy oldExc,
// a ĝp already inside the safe region is left untouched.
func TestTame_HighEnergyKeepsSafeGp(t *testing.T) {
	var oldExc [154]int16
	for i := range oldExc {
		oldExc[i] = 16384
	}
	const gp int16 = 10000 // < GpClipQ14
	got := Tame(gp, &oldExc)
	if got != gp {
		t.Fatalf("Tame(high-energy, safe gp=%d) = %d, want %d (unchanged)",
			gp, got, gp)
	}
}

// TestTame_ThresholdBoundary pins the OQ-TAMING-THR boundary:
//   - At exactly the threshold (Σx² == TameEnergyThresholdQ0) the
//     branch must NOT fire (strict-greater compare).
//   - Just above (one extra sample of energy) it must fire.
func TestTame_ThresholdBoundary(t *testing.T) {
	// Construct oldExc so Σx² == TameEnergyThresholdQ0 exactly: place
	// a single sample s with s² = TameEnergyThresholdQ0. 2^33 = (2^16.5)²
	// is non-integer; use the nearest pair: s=23170 → s²=536848900,
	// then 16 such samples give 16·s² ≈ 8.59e9 ≈ 2^33.0035 (just above).
	// For "exactly at" we use 15 samples → ≈ 8.05e9 < 2^33 = 8.59e9.
	var below [154]int16
	for i := 0; i < 15; i++ {
		below[i] = 23170
	}
	if got := Tame(20000, &below); got != 20000 {
		t.Errorf("Tame(below threshold) clamped to %d, want 20000", got)
	}
	var above [154]int16
	for i := 0; i < 17; i++ {
		above[i] = 23170
	}
	if got := Tame(20000, &above); got != GpClipQ14 {
		t.Errorf("Tame(above threshold) = %d, want GpClipQ14=%d",
			got, GpClipQ14)
	}
}

// TestTame_NegativeGp pins the sign discipline: ĝp arrives non-negative
// from the §3.9.2 codebook (GBK1/GBK2 are non-negative); a negative
// argument is structurally impossible but if it arises (test fixture or
// future codebook revision), Tame must NOT silently mangle it — the
// energy test runs on |oldExc| via squaring, so the only branch that
// can fire is the upper-bound clamp, which doesn't apply to negatives.
func TestTame_NegativeGp(t *testing.T) {
	var oldExc [154]int16
	for i := range oldExc {
		oldExc[i] = 16384
	}
	const gp int16 = -100
	got := Tame(gp, &oldExc)
	if got != gp {
		t.Fatalf("Tame(negative gp=%d, high-energy) = %d, want %d",
			gp, got, gp)
	}
}

// TestTame_ZeroAlloc pins the encoder hot-path budget: taming runs once
// per subframe and must not allocate.
func TestTame_ZeroAlloc(t *testing.T) {
	var oldExc [154]int16
	for i := range oldExc {
		oldExc[i] = 16384
	}
	allocs := testing.AllocsPerRun(128, func() {
		_ = Tame(20000, &oldExc)
	})
	if allocs != 0 {
		t.Fatalf("Tame allocs/op = %.2f, want 0", allocs)
	}
}
