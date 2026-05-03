package g729

import "testing"

// TestNoAllocationInClosedloopStep pins I4 (zero-allocation contract)
// on the package-internal closedloopStep entry point introduced in
// Phase 2c. Construct a fresh Encoder, prime it with one lpcStep +
// openloopStep call (so e.aHatSF{1,2}, e.tOp, e.lpResidualMemQ,
// e.swMemErr and e.oldExc carry production-shaped state), then call
// closedloopStep(0) followed by closedloopStep(1) repeatedly and
// assert testing.AllocsPerRun returns exactly 0 for each.
//
// The 183-sample on-stack scratch buffer (excSearch =
// [PitchMaxInt+SubframeLen]int16) introduced by the OQ-K<40 LP-residual
// extension refactor must not escape to the heap; the closedloop
// SearchInteger / RefineFraction / AdaptiveVector / Interpolate3
// primitives are individually pinned to zero-alloc by their unit
// tests, so the only remaining heap risk is the encoder driver itself.
//
// Phase 2c INT-2 gate. See
// docs/superpowers/plans/2026-05-09-phase2c-closed-loop-pitch-plan.md
// §6 Task INT-2 Step 1 + §7 INT-2 alloc/bench table.
func TestNoAllocationInClosedloopStep(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}

	// Warm up: prime LPC + open-loop state so closedloopStep sees
	// production-shaped inputs (aHatSF{1,2} populated, tOp valid,
	// lpResidualMemQ and swMemErr non-degenerate).
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("lpcStep warmup returned error: %v", err)
	}
	_ = enc.openloopStep()
	_, _ = enc.closedloopStep(0)
	_, _ = enc.closedloopStep(1)

	// Subframe 0 in isolation.
	allocsSF0 := testing.AllocsPerRun(64, func() {
		_, _ = enc.closedloopStep(0)
	})
	if allocsSF0 != 0 {
		t.Fatalf("Encoder.closedloopStep(0) allocated %.2f times per call; want 0 (I4)", allocsSF0)
	}

	// Subframe 1 in isolation.
	allocsSF1 := testing.AllocsPerRun(64, func() {
		_, _ = enc.closedloopStep(1)
	})
	if allocsSF1 != 0 {
		t.Fatalf("Encoder.closedloopStep(1) allocated %.2f times per call; want 0 (I4)", allocsSF1)
	}
}

// TestNoAllocationInLPCStepPlusOpenloopPlusClosedloop pins I4 on the
// end-to-end lpcStep + openloopStep + 2× closedloopStep composition
// for one frame — the full Phase 2c production hot path the encoder
// executes every 10 ms.
//
// Phase 2c INT-2 gate per plan §6 Task INT-2 Step 1.
func TestNoAllocationInLPCStepPlusOpenloopPlusClosedloop(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}

	// Warm up the full chain.
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("lpcStep warmup returned error: %v", err)
	}
	_ = enc.openloopStep()
	_, _ = enc.closedloopStep(0)
	_, _ = enc.closedloopStep(1)

	allocs := testing.AllocsPerRun(64, func() {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("lpcStep returned error: %v", err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	})
	if allocs != 0 {
		t.Fatalf("lpcStep+openloopStep+2×closedloopStep allocated %.2f times per call; want 0 (I4)", allocs)
	}
}

// BenchmarkClosedloopStep captures Phase 2c INT-2 bench numbers for
// the closure report. Reports per-subframe ns/op + B/op + allocs/op
// for the production hot path.
func BenchmarkClosedloopStep(b *testing.B) {
	enc := NewEncoder()
	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		b.Fatalf("lpcStep warmup returned error: %v", err)
	}
	_ = enc.openloopStep()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	}
}
