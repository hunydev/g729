package openloop

import "math/bits"

// Range2AtLeastRatioOf reports whether the [40,79] range winner's normalized
// score is at least num/den of the currently selected range winner.
func (r SearchResult) Range2AtLeastRatioOf(current int16, num, den int64) bool {
	cur, ok := r.scoreForLag(current)
	if !ok {
		return false
	}
	return normalizedAtLeastRatio(r.Range2, cur, num, den)
}

func (r SearchResult) scoreForLag(lag int16) (RangeScore, bool) {
	switch lag {
	case r.Range1.Lag:
		return r.Range1, true
	case r.Range2.Lag:
		return r.Range2, true
	case r.Range3.Lag:
		return r.Range3, true
	default:
		return RangeScore{}, false
	}
}

func normalizedAtLeastRatio(candidate, current RangeScore, num, den int64) bool {
	if den <= 0 || num < 0 {
		return false
	}
	if candidate.E <= 0 || candidate.R <= 0 {
		return false
	}
	if current.E <= 0 || current.R <= 0 {
		return true
	}
	candR := int64(candidate.R)
	curR := int64(current.R)
	maxR := candR
	if curR > maxR {
		maxR = curR
	}
	var s uint
	if l := bits.Len64(uint64(maxR)); l > 12 {
		s = uint(l - 12)
	}
	candR >>= s
	curR >>= s
	return candR*candR*int64(current.E)*den >= curR*curR*int64(candidate.E)*num
}
