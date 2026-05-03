package g729

import (
	"testing"

	"github.com/exedev/g729/internal/pitch/closedloop"
)

// Phase 2d INT-2 zero-allocation gates pin I4 on the new fcbStep
// hot path and on the full encoder analysis pipeline composed of
// lpcStep + openloopStep + 2 × (closedloopStep + fcbStep). The
// per-step primitives (fcbsearch.{AdjustedTarget, CorrelationD,
// SignsFromD, PhiPrime, SearchDepthFirst, BuildCode, FilterCode},
// gainquant.{PredictedGcQ12, SearchConjugate, Tame,
// UpdatePastQuaEn}) are individually pinned to zero-alloc by their
// unit tests; this gate pins the encoder driver itself and asserts
// the FCB scratch (xPrime, d, signs, dAbs, phi, positions, c, z,
// gainquant scratch) does not escape to the heap.
//
// Plan §6 Task INT-2 Step 1 + §3.1 working-tree gate.

// prepFcbInputs mirrors the closedloopStep prelude (up to GpAndY)
// for one subframe so the test can exercise fcbStep in isolation
// with production-shaped (x, y, h, v, gp) inputs while keeping
// the encoder state otherwise undisturbed. It mutates only local
// scratch — it does NOT touch encoder commit fields.
func prepFcbInputs(
	e *Encoder,
	sub int,
	x, y, h, v *[closedloop.SubframeLen]int16,
) (gp int16) {
	var aHat = &e.aHatSF1
	if sub == 1 {
		aHat = &e.aHatSF2
	}

	sStart := 160 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, xb [closedloop.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	closedloop.TargetSignal(aHat, &r, &e.swMemErr, x)
	closedloop.ImpulseResponse(aHat, h)
	closedloop.BackwardFilter(x, h, &xb)

	var centre int16
	if sub == 0 {
		centre = e.tOp
	} else {
		centre = e.intT1
	}

	var excSearch [closedloop.PitchMaxInt + closedloop.SubframeLen]int16
	copy(excSearch[:closedloop.PitchMaxInt],
		e.oldExc[len(e.oldExc)-closedloop.PitchMaxInt:])
	copy(excSearch[closedloop.PitchMaxInt:], r[:])
	excSlice := excSearch[:]
	intLag, _ := closedloop.SearchInteger(&xb, excSlice, centre, sub)
	frac := closedloop.RefineFraction(&xb, excSlice, intLag, intLag < 85)

	closedloop.AdaptiveVector(excSlice, intLag, frac, v)
	gp = closedloop.GpAndY(x, v, h, y)
	return gp
}

// TestPhase2dINT2_FcbStepZeroAlloc pins fcbStep itself to 0
// allocs/op for both subframes. Inputs are computed once outside
// the AllocsPerRun closure; fcbStep is deterministic given its
// inputs and the encoder state (which it then mutates), so
// repeated invocation with the same inputs is a valid zero-alloc
// probe — only heap escape of the FCB / gain-VQ scratch arrays
// would surface here.
func TestPhase2dINT2_FcbStepZeroAlloc(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}

	// Warm up the full chain so encoder state (aHatSF1/2, tOp,
	// intT1, lpResidualMemQ, swMemErr, oldExc, pastQuaEn,
	// prevGpQ14, prevTaming) is production-shaped.
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		t.Fatalf("lpcStep warmup returned error: %v", err)
	}
	_ = enc.openloopStep()
	_, _ = enc.closedloopStep(0)
	_, _ = enc.closedloopStep(1)

	for sub := 0; sub < 2; sub++ {
		var x, y, h, v [closedloop.SubframeLen]int16
		gp := prepFcbInputs(enc, sub, &x, &y, &h, &v)

		allocs := testing.AllocsPerRun(128, func() {
			enc.fcbStep(sub, &x, &y, &h, &v, gp)
		})
		if allocs != 0 {
			t.Fatalf("Encoder.fcbStep(%d) allocated %.2f times per call; want 0 (I4)", sub, allocs)
		}
	}
}

// TestPhase2dINT2_FullFramePipelineZeroAlloc pins the full Phase 2d
// production hot path — lpcStep + openloopStep + 2 ×
// (closedloopStep + fcbStep) — to 0 allocs/op. closedloopStep
// invokes fcbStep internally per the Phase 2d INT-0 wiring, so
// this exercises the entire eq. A.9 / A.10 commit chain end to
// end every iteration.
func TestPhase2dINT2_FullFramePipelineZeroAlloc(t *testing.T) {
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

	allocs := testing.AllocsPerRun(128, func() {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("lpcStep returned error: %v", err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	})
	if allocs != 0 {
		t.Fatalf("lpcStep+openloopStep+2×(closedloopStep+fcbStep) allocated %.2f times per call; want 0 (I4)", allocs)
	}
}

// BenchmarkPhase2dINT2_FullFramePipeline captures Phase 2d INT-2
// bench numbers for the closure report — per-frame ns/op + B/op +
// allocs/op for the full Phase 2d production hot path
// (lpcStep + openloopStep + 2 × (closedloopStep + fcbStep)).
//
// Soft target per plan §3.1: fcbStep MUST NOT regress beyond 2× the
// Phase 2c BenchmarkClosedloopStep budget (14964 ns/op baseline).
func BenchmarkPhase2dINT2_FullFramePipeline(b *testing.B) {
	enc := NewEncoder()
	var pcm [FrameSamples]int16
	for i := range pcm {
		pcm[i] = int16(((i * 137) & 0x3FFF) - 0x2000)
	}
	if _, err := enc.lpcStep(pcm[:]); err != nil {
		b.Fatalf("lpcStep warmup returned error: %v", err)
	}
	_ = enc.openloopStep()
	_, _ = enc.closedloopStep(0)
	_, _ = enc.closedloopStep(1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			b.Fatalf("lpcStep returned error: %v", err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	}
}
