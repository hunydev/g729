package openloop

const (
	wideSubMultipleLift      = 8.0
	wideSubMultipleTolerance = 20
	wideRangeOverrideMargin  = 1.10
)

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
	if isNearSubmultipleWide(int(h.lag), int(op.lag)) {
		return liftedStrictGreaterWide(h.r, h.e, op.r, op.e)
	}
	return strictGreaterWide(h.r, h.e, op.r, op.e)
}

func isNearSubmultipleWide(higher, lower int) bool {
	if lower <= 0 {
		return false
	}
	for k := 2; k <= 7; k++ {
		d := higher - k*lower
		if d < 0 {
			d = -d
		}
		if d <= wideSubMultipleTolerance {
			return true
		}
		if k*lower > higher+wideSubMultipleTolerance {
			return false
		}
	}
	return false
}

func strictGreaterWide(rH, eH, rOp, eOp int64) bool {
	if eH <= 0 || rH <= 0 {
		return false
	}
	if eOp <= 0 || rOp <= 0 {
		return true
	}
	return (float64(rH)*float64(rH))/float64(eH) >
		wideRangeOverrideMargin*(float64(rOp)*float64(rOp))/float64(eOp)
}

func liftedStrictGreaterWide(rH, eH, rOp, eOp int64) bool {
	if eH <= 0 || rH <= 0 {
		return false
	}
	if eOp <= 0 || rOp <= 0 {
		return true
	}
	return (float64(rH)*float64(rH))/float64(eH) >
		wideSubMultipleLift*(float64(rOp)*float64(rOp))/float64(eOp)
}
