package openloop

func mergeThreeRangesWide(r1 rangeScoreWide, r2 rangeScoreWide, r3 rangeScoreWide) int16 {
	best := r1
	if shouldOverrideWide(r2, best) {
		best = r2
	}
	if shouldOverrideWide(r3, best) {
		best = r3
	}
	return best.lag
}

func shouldOverrideWide(h, op rangeScoreWide) bool {
	if isNearSubmultiple(int(h.lag), int(op.lag)) {
		return liftedStrictGreaterWide(h.r, h.e, op.r, op.e)
	}
	return strictGreaterWide(h.r, h.e, op.r, op.e)
}

func strictGreaterWide(rH, eH, rOp, eOp int64) bool {
	return !compareNormalizedWide(rOp, eOp, rH, eH)
}

func liftedStrictGreaterWide(rH, eH, rOp, eOp int64) bool {
	if eH <= 0 || rH <= 0 {
		return false
	}
	if eOp <= 0 || rOp <= 0 {
		return true
	}
	return (float64(rH)*float64(rH))/float64(eH) >
		(float64(oq1SubMultipleLiftNumerator)/float64(oq1SubMultipleLiftDenominator))*
			(float64(rOp)*float64(rOp))/float64(eOp)
}
