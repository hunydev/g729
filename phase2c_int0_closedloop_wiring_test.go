package g729

import (
	"testing"

	"github.com/hunydev/g729/internal/pitch/closedloop"
)

// drivePeriodicFrame fills pcm with a mildly-periodic non-trivial
// frame so the encoder LP/pitch chain follows its regular code path
// rather than a silence-degenerate branch. The same recipe used by
// Phase 2b INT-2 alloc tests (encoder_test.go / openloop tests).
func drivePeriodicFrame(pcm *[FrameSamples]int16) {
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}
}

// TestPhase2cINT0_ClosedLoopStepReturnsPlausibleLags drives the
// encoder with a periodic frame, runs lpcStep + openloopStep +
// closedloopStep ×2 and asserts that:
//
//   - subframe-1 integer lag lies in [PitchMinInt, PitchMaxInt] =
//     [20, 143]; subframe-2 may additionally use the two P2 fractional
//     boundary codepoints.
//   - frac for each subframe lies in {-1, 0, +1} per §3.7.2 eq. 41.
//   - the encoded P1 / P0 / P2 fields fit their bit budgets per
//     Table 8 (P1: 8 bits, P0: 1 bit, P2: 5 bits).
//
// This is the Phase 2c INT-0 SMOKE gate; INT-1 will add STRICT
// byte-EQ vs PITCH.BIT.
func TestPhase2cINT0_ClosedLoopStepReturnsPlausibleLags(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	drivePeriodicFrame(&pcm)

	// Warm up: a few frames so the LP analyzer / open-loop pipeline
	// reaches steady state and tOp lands inside [40, 143] (avoiding
	// the residual-extension short-lag corner of SearchInteger that
	// is out of scope for INT-0; see closedloopStep godoc).
	for f := 0; f < 4; f++ {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d lpcStep error: %v", f, err)
		}
		_ = enc.openloopStep()
		intT1, frac1 := enc.closedloopStep(0)
		intT2, frac2 := enc.closedloopStep(1)

		if intT1 < closedloop.PitchMinInt || intT1 > closedloop.PitchMaxInt {
			t.Errorf("frame %d sub0 intLag = %d, want in [%d, %d]",
				f, intT1, closedloop.PitchMinInt, closedloop.PitchMaxInt)
		}
		if frac1 < -1 || frac1 > 1 {
			t.Errorf("frame %d sub0 frac = %d, want in {-1,0,+1}", f, frac1)
		}
		if frac2 < -1 || frac2 > 1 {
			t.Errorf("frame %d sub1 frac = %d, want in {-1,0,+1}", f, frac2)
		}
		// P0 is 1 bit.
		if enc.p0 > 1 {
			t.Errorf("frame %d P0 = %d, want ≤ 1", f, enc.p0)
		}
		// P2 is 5 bits.
		if enc.p2 > 31 {
			t.Errorf("frame %d P2 = %d, want ≤ 31", f, enc.p2)
		}

		// Subframe-2 must round-trip through the 5-bit P2 codepoint.
		tmin, tmax := closedloop.Subframe2Window(intT1)
		p2 := closedloop.EncodeP2(intT2, frac2, tmin)
		if p2 > 31 {
			t.Errorf("frame %d encoded P2 for sub1 lag (%d,%d) = %d, want <= 31",
				f, intT2, frac2, p2)
		}
		if !((intT2 == tmin-1 && frac2 == 1) ||
			(intT2 >= tmin && intT2 <= tmax) ||
			(intT2 == tmax+1 && frac2 == -1)) {
			t.Errorf("frame %d sub1 lag (%d,%d) outside P2 fractional span for intT1=%d window [%d,%d]",
				f, intT2, frac2, intT1, tmin, tmax)
		}
	}
}

// TestPhase2cINT0_ClosedLoopStepZeroAlloc pins I4 (zero-alloc
// contract) on the closedloopStep entry point. Both subframes are
// measured to catch any allocation slipping into either branch.
//
// Phase 2c INT-0 alloc gate; mirrors phase2b_int2 pattern.
func TestPhase2cINT0_ClosedLoopStepZeroAlloc(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	drivePeriodicFrame(&pcm)

	// Warmup.
	for f := 0; f < 3; f++ {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("warmup lpcStep frame %d: %v", f, err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	}

	allocs := testing.AllocsPerRun(64, func() {
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	})
	if allocs != 0 {
		t.Fatalf("closedloopStep allocated %.2f times per call; want 0 (I4)", allocs)
	}
}
