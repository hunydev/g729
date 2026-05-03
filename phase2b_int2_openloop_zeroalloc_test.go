package g729

import "testing"

// TestNoAllocationInOpenloopStep pins I4 (zero-allocation contract) on
// the package-internal openloopStep entry point used by Phase 2b.
// Construct a fresh Encoder, prime it with one lpcStep call (so
// e.aQ12Latest and e.oldSpeech[160:240] are populated), then call
// openloopStep on a fixed PCM frame and assert testing.AllocsPerRun
// returns exactly 0.
//
// Phase 2b INT-2 gate. See docs/superpowers/plans/2026-05-07-phase2b-open-loop-pitch-plan.md
// §6 Task INT-2 Step 1 + §7 INT-2 alloc/bench table.
func TestNoAllocationInOpenloopStep(t *testing.T) {
	enc := NewEncoder()

	// Mildly periodic non-trivial frame so the LPC + open-loop chain
	// follows its regular code path rather than a silence-degenerate
	// branch.
	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}

	// Warm up: prime any lazy init (lpcStep populates aQ12Latest +
	// oldSpeech and openloopStep advances residualMem/swMem/oldWspeech)
	// so the measured runs see steady-state alloc behaviour.
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("lpcStep warmup returned error: %v", err)
	}
	_ = enc.openloopStep()

	allocs := testing.AllocsPerRun(128, func() {
		// Re-prime aQ12Latest + oldSpeech each iteration via lpcStep
		// is unnecessary — openloopStep is pure on existing state and
		// only rewrites the four documented memories. Measure it in
		// isolation to satisfy the standalone openloopStep gate.
		_ = enc.openloopStep()
	})
	if allocs != 0 {
		t.Fatalf("Encoder.openloopStep allocated %.2f times per call; want 0 (I4)", allocs)
	}
}

// TestNoAllocationInLPCStepPlusOpenloop pins I4 on the end-to-end
// lpcStep + openloopStep composition for one frame. This is the
// production hot path the encoder will execute every 10 ms.
//
// Phase 2b INT-2 gate per plan §6 Task INT-2 Step 1.
func TestNoAllocationInLPCStepPlusOpenloop(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}

	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("lpcStep warmup returned error: %v", err)
	}
	_ = enc.openloopStep()

	allocs := testing.AllocsPerRun(128, func() {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("lpcStep returned error: %v", err)
		}
		_ = enc.openloopStep()
	})
	if allocs != 0 {
		t.Fatalf("lpcStep + openloopStep allocated %.2f times per call; want 0 (I4)", allocs)
	}
}
