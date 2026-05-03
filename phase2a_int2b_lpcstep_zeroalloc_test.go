package g729

import "testing"

// TestNoAllocationInLPCStep pins I4 (zero-allocation contract) on the
// package-internal lpcStep entry point used by Phase 2a..2f. Construct
// a fresh Encoder (NewEncoder primes lspOld and freqPrev as production
// does), then call lpcStep on a fixed 80-sample int16 frame and assert
// testing.AllocsPerRun returns exactly 0.
//
// Phase 2a INT-2-b gate. See docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md
// §8 Task INT-2-b Step 3.
func TestNoAllocationInLPCStep(t *testing.T) {
	enc := NewEncoder()

	// Mildly periodic non-trivial frame so the LPC chain follows its
	// regular code path (autocorr fast-path + lag-window + Levinson +
	// LP→LSP root extraction + LSP VQ search), not the silence
	// degenerate branch.
	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}

	// Warm up: prime any lazy init in the dependent packages so the
	// measured runs only see steady-state alloc behaviour.
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("lpcStep warmup returned error: %v", err)
	}

	allocs := testing.AllocsPerRun(128, func() {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("lpcStep returned error: %v", err)
		}
	})
	if allocs != 0 {
		t.Fatalf("Encoder.lpcStep allocated %.2f times per call; want 0 (I4)", allocs)
	}
}
