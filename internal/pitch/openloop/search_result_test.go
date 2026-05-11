package openloop

import "testing"

func TestSearchResultRange2AtLeastRatioOf(t *testing.T) {
	result := SearchResult{
		Range1: RangeScore{Lag: 35, R: 1000, E: 1000},
		Range2: RangeScore{Lag: 55, R: 975, E: 1000},
		Range3: RangeScore{Lag: 95, R: 500, E: 1000},
		Top:    35,
	}
	if !result.Range2AtLeastRatioOf(result.Top, 95, 100) {
		t.Fatal("Range2AtLeastRatioOf returned false for a 95 percent-close range2 score")
	}
	if result.Range2AtLeastRatioOf(result.Top, 96, 100) {
		t.Fatal("Range2AtLeastRatioOf returned true above the range2 score ratio")
	}
	if result.Range2AtLeastRatioOf(77, 95, 100) {
		t.Fatal("Range2AtLeastRatioOf returned true for an unknown current lag")
	}
}
