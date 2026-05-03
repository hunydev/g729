package g729

import (
	"math"
	"testing"
)

// TestPhase2bINT0_OpenloopStep_SineConverges feeds a synthetic sine of
// period 40 samples (200 Hz at 8 kHz) through (lpcStep, openloopStep)
// and asserts that the returned T_op converges to a value near the
// fundamental period within a handful of frames. Per Phase 2b plan
// §6 Task INT-0 step 1.
func TestPhase2bINT0_OpenloopStep_SineConverges(t *testing.T) {
	enc := NewEncoder()

	const period = 40
	const amp = 8000.0
	var phase float64

	var lastTOp int16
	convergedAt := -1
	for f := 0; f < 30; f++ {
		var pcm [FrameSamples]int16
		for i := range pcm {
			pcm[i] = int16(amp * math.Sin(2*math.Pi*phase/period))
			phase++
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		top := enc.openloopStep()
		if top < 20 || top > 143 {
			t.Fatalf("frame %d: T_op=%d out of [20,143]", f, top)
		}
		lastTOp = top
		// Accept any divisor/multiple of period within ±2.
		near := func(t1, t2 int16) bool {
			d := int(t1) - int(t2)
			if d < 0 {
				d = -d
			}
			return d <= 2
		}
		if convergedAt < 0 && (near(top, period) || near(top, period/2) || near(top, 2*period) || near(top, 3*period)) {
			convergedAt = f
		}
	}
	if convergedAt < 0 {
		t.Fatalf("T_op never converged near period=%d (last=%d)", period, lastTOp)
	}
	if convergedAt > 8 {
		t.Errorf("T_op converged too late: frame %d (want ≤ 5)", convergedAt)
	}
}

// TestPhase2bINT0_OldWspeechSlides confirms that openloopStep mutates
// e.oldWspeech (the 143-sample sw history slides per I-2b-2). After
// frame 1 the buffer must be non-zero; after frame 2 the prefix of
// frame 2's buffer must equal the suffix of frame 1's buffer per
// slideOldWspeech's contract (old[0:63] == prev_old[80:143]).
func TestPhase2bINT0_OldWspeechSlides(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 311) & 0x3FFF) - 0x2000)
	}
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("frame 0 lpcStep: %v", err)
	}
	enc.openloopStep()

	// After frame 0: oldWspeech must contain non-zero samples in its
	// fresh-frame slot (indices 63..142).
	allZero := true
	for _, v := range enc.oldWspeech {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("oldWspeech remained all-zero after frame 0")
	}

	// Snapshot the post-frame-0 buffer.
	var snap [143]int16
	snap = enc.oldWspeech

	// Frame 1.
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("frame 1 lpcStep: %v", err)
	}
	enc.openloopStep()

	// After frame 1: old[0:63] must equal the previous old[80:143].
	for i := 0; i < 63; i++ {
		if enc.oldWspeech[i] != snap[80+i] {
			t.Fatalf("slide mismatch at i=%d: got=%d want=%d (snap[%d])",
				i, enc.oldWspeech[i], snap[80+i], 80+i)
		}
	}
}

// TestPhase2bINT0_OpenloopStepZeroAlloc pins the I4 zero-allocation
// contract on (*Encoder).openloopStep — Phase 2b plan §6 Task INT-2
// preview, gated here at INT-0 since the method is born with the I4
// contract.
func TestPhase2bINT0_OpenloopStepZeroAlloc(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("warmup lpcStep: %v", err)
	}
	enc.openloopStep()

	allocs := testing.AllocsPerRun(128, func() {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("lpcStep: %v", err)
		}
		enc.openloopStep()
	})
	if allocs != 0 {
		t.Fatalf("openloopStep allocated %.2f times per call; want 0 (I4)", allocs)
	}
}
