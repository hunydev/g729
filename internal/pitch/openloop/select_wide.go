package openloop

type rangeScoreWide struct {
	lag int16
	r   int64
	e   int64
}

func pickBestInRangeWide(wsp *[223]int16, kMin, kMax int) rangeScoreWide {
	if kMin == 80 && kMax == 143 {
		return pickBestEvenWithRefinementWide(wsp)
	}
	return pickBestFullScanWide(wsp, kMin, kMax)
}

func pickBestFullScanWide(wsp *[223]int16, kMin, kMax int) rangeScoreWide {
	best := rangeScoreWide{lag: int16(kMax), r: correlateAtWide(wsp, kMax), e: energyWide(wsp, kMax)}
	for k := kMax - 1; k >= kMin; k-- {
		cand := rangeScoreWide{lag: int16(k), r: correlateAtWide(wsp, k), e: energyWide(wsp, k)}
		if compareNormalizedWide(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}
	return best
}

func pickBestEvenWithRefinementWide(wsp *[223]int16) rangeScoreWide {
	best := rangeScoreWide{lag: 142, r: correlateAtWide(wsp, 142), e: energyWide(wsp, 142)}
	for k := 140; k >= 80; k -= 2 {
		cand := rangeScoreWide{lag: int16(k), r: correlateAtWide(wsp, k), e: energyWide(wsp, k)}
		if compareNormalizedWide(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}

	bestEven := int(best.lag)
	hi := bestEven + 1
	if hi > 143 {
		hi = 143
	}
	lo := bestEven - 1
	if lo < 80 {
		lo = 80
	}
	best = rangeScoreWide{lag: int16(hi), r: correlateAtWide(wsp, hi), e: energyWide(wsp, hi)}
	for k := hi - 1; k >= lo; k-- {
		cand := rangeScoreWide{lag: int16(k), r: correlateAtWide(wsp, k), e: energyWide(wsp, k)}
		if compareNormalizedWide(cand.r, cand.e, best.r, best.e) {
			best = cand
		}
	}
	return best
}

func correlateAtWide(wsp *[223]int16, k int) int64 {
	var acc int64
	for n := 0; n < 40; n++ {
		acc += 2 * int64(wsp[143+2*n]) * int64(wsp[143+2*n-k])
	}
	return acc
}

func energyWide(wsp *[223]int16, k int) int64 {
	var acc int64
	for n := 0; n < 40; n++ {
		s := int64(wsp[143+2*n-k])
		acc += s * s
	}
	return acc
}

func compareNormalizedWide(r1, e1, r2, e2 int64) bool {
	score1Zero := e1 <= 0 || r1 <= 0
	score2Zero := e2 <= 0 || r2 <= 0
	if score1Zero && score2Zero {
		return true
	}
	if score1Zero {
		return false
	}
	if score2Zero {
		return true
	}
	return (float64(r1)*float64(r1))/float64(e1) >= (float64(r2)*float64(r2))/float64(e2)
}
