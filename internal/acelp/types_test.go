package acelp

import "testing"

func TestSearcher_Reset_ZeroValueIsSafe(t *testing.T) {
	var s Searcher
	s.Reset()
}

func TestSearcher_Search_StubReturnsNotImplemented(t *testing.T) {
	var s Searcher
	var (
		target [SubframeSamples]int16
		h      [SubframeSamples]int16
		out    Result
	)
	if err := s.Search(target[:], h[:], &out); err == nil {
		t.Fatal("Search returned nil; expected stub error")
	}
}
