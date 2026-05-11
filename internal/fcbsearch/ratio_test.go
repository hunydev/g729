package fcbsearch

import "testing"

func TestRatioGreater_LargeNearTieUsesExactIntegerCriterion(t *testing.T) {
	// Exact comparison:
	// (1_000_000_001^2 * 1_000_000_000) >
	// (1_000_000_000^2 * 1_000_000_002) by 1_000_000_000.
	// A float64 quotient compare rounds this near-tie to equality.
	if !ratioGreater(1_000_000_001, 1_000_000_002, 1_000_000_000, 1_000_000_000) {
		t.Fatal("ratioGreater rejected exact larger C^2/E near tie")
	}
}

func TestRatioGreater_EqualRatioIsNotGreater(t *testing.T) {
	if ratioGreater(2_000_000_000, 4, 1_000_000_000, 1) {
		t.Fatal("ratioGreater reported strict improvement for equal ratios")
	}
}
