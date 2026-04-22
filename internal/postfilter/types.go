package postfilter

const (
	subframeLen = 40
	lpcOrder    = 10
	pitchMax    = 143
)

// Postfilter holds per-channel adaptive-postfilter state per ITU-T G.729
// §A.4.2. The zero value is a valid Reset state.
//
// Not safe for concurrent use. One instance per decoder stream.
type Postfilter struct {
	pastS         [lpcOrder]int16
	pastResidual  [pitchMax + subframeLen]int16
	pastSynthPost [lpcOrder]int16
	pastTiltInput int16
	// agcGainPrev is the AGC gain used in the last sample of the previous
	// subframe, held at Q24 internally for steady-state precision.
	// On the very first applyAGC call (initialized == false), it is
	// seeded to g_target per ITU-T G.729 §A.4.2.4 initialization.
	agcGainPrev int32
	initialized bool
}

// Reset clears all postfilter state to the zero initial condition.
func (pf *Postfilter) Reset() {
	*pf = Postfilter{}
}
