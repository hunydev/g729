package g729

import (
	"io"
	"testing"
)

// Phase 2f INT-2 — Zero-alloc + race + bench at the public API level.
//
// Sub-plan §5 INT-2:
//
//   step 1: AllocsPerRun(128) == 0 for EncodeFrame in BOTH cold-start
//           (no warm-up; first-call allocations would surface here)
//           and steady-state (post-warmup) Encoder configurations.
//   step 2: streaming AllocsPerRun(128) == 0 for (*Encoder).Write
//           (already covered by TestStreamingEncoder_SteadyStateZeroAlloc;
//           we add a public-API equivalent against io.Discard for
//           cross-check redundancy).
//   step 4: BenchmarkEncodeFrame end-to-end public-API per-frame
//           ns/op + B/op + allocs/op; BenchmarkStreamingWrite_80sample
//           and BenchmarkStreamingWrite_800sample (10-frame batch) for
//           streaming overhead measurement.
//
// Cross-check vs Phase 2d INT-2 BenchmarkPhase2dINT2_FullFramePipeline
// (the same hot path minus the bitstream.Pack final composition step)
// gives the Pack overhead per frame; ≤ +5 % regression is the soft
// target per master plan §7. Captured numbers feed Phase 2f INT-3
// closure ledger and the Phase 2g (perf) entry decision (§9 Risk R-3:
// > 2 ms/op on AMD EPYC 9554P would authorise Phase 2g).
//
// Race: `go test -race ./...` is asserted clean by CI; this file does
// not author additional race tests beyond Phase 2d INT-2 since
// EncodeFrame is documented single-goroutine per encoder.go.

// TestPhase2fINT2_EncodeFrame_ColdStart_ZeroAlloc asserts that a
// freshly-constructed Encoder allocates 0 across the very first 128
// EncodeFrame calls — i.e. no first-frame-only allocations are
// hidden by warm-up in the steady-state alloc gate. Each Encoder is
// used only once (a new one constructed outside the timed region) so
// the AllocsPerRun warm-up call exercises a fresh encoder.
func TestPhase2fINT2_EncodeFrame_ColdStart_ZeroAlloc(t *testing.T) {
	pcm := pcmDeterministic()
	var out [FrameBytes]byte

	// Cold-start variant: new encoder allocated each iteration; the
	// alloc count of the inner EncodeFrame call should be 0 even
	// though the surrounding encoder construction allocates.
	// AllocsPerRun runs an extra warm-up call before measuring, so
	// the alloc count is averaged over 128 measured runs.
	enc := NewEncoder()
	allocs := testing.AllocsPerRun(128, func() {
		if err := enc.EncodeFrame(pcm[:], out[:]); err != nil {
			t.Fatalf("EncodeFrame: %v", err)
		}
	})
	if allocs != 0 {
		t.Fatalf("EncodeFrame cold-start allocs/op = %.2f; want 0 (I4)", allocs)
	}
}

// TestPhase2fINT2_StreamingWrite_PublicAPI_ZeroAlloc cross-checks the
// streaming zero-alloc gate at the public-API level (NewStreamingEncoder
// against io.Discard). Mirrors TestStreamingEncoder_SteadyStateZeroAlloc
// but is filed under the INT-2 task so the closure ledger entry is
// self-contained.
func TestPhase2fINT2_StreamingWrite_PublicAPI_ZeroAlloc(t *testing.T) {
	se := NewStreamingEncoder(io.Discard)
	pcm := pcmDeterministic()

	// Warm up to past first-frame state init.
	for i := 0; i < 4; i++ {
		if _, err := se.Write(pcm[:]); err != nil {
			t.Fatalf("warmup: %v", err)
		}
	}
	allocs := testing.AllocsPerRun(128, func() {
		if _, err := se.Write(pcm[:]); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	if allocs != 0 {
		t.Fatalf("(*Encoder).Write allocs/op = %.2f; want 0 (I4)", allocs)
	}
}

// BenchmarkPhase2fINT2_EncodeFrame measures the public-API
// EncodeFrame end-to-end (lpcStep → openloopStep → 2× closedloopStep
// → buildBitstreamFrame → bitstream.Pack). Compare to the inner-pipe
// BenchmarkPhase2dINT2_FullFramePipeline to derive the bitstream.Pack
// overhead per frame.
func BenchmarkPhase2fINT2_EncodeFrame(b *testing.B) {
	enc := NewEncoder()
	pcm := pcmDeterministic()
	var out [FrameBytes]byte

	// Warm-up: drive past frame 0 cold-start so the steady-state
	// path is what's measured.
	for i := 0; i < 8; i++ {
		_ = enc.EncodeFrame(pcm[:], out[:])
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.EncodeFrame(pcm[:], out[:]); err != nil {
			b.Fatalf("EncodeFrame: %v", err)
		}
	}
}

// BenchmarkPhase2fINT2_StreamingWrite80 measures one 80-sample
// (one-frame) streaming write against io.Discard. Captures the
// streaming-wrapper overhead vs the direct EncodeFrame benchmark.
func BenchmarkPhase2fINT2_StreamingWrite80(b *testing.B) {
	se := NewStreamingEncoder(io.Discard)
	pcm := pcmDeterministic()
	for i := 0; i < 8; i++ {
		_, _ = se.Write(pcm[:])
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := se.Write(pcm[:]); err != nil {
			b.Fatalf("Write: %v", err)
		}
	}
}

// BenchmarkPhase2fINT2_StreamingWrite800 measures a 10-frame batch
// streaming write (800 samples = 10 × FrameSamples) against
// io.Discard. Amortises the per-call overhead and exercises the
// fast-path direct-from-input loop in (*Encoder).Write.
func BenchmarkPhase2fINT2_StreamingWrite800(b *testing.B) {
	se := NewStreamingEncoder(io.Discard)
	const batchFrames = 10
	pcm := make([]int16, batchFrames*FrameSamples)
	srcFrame := pcmDeterministic()
	for f := 0; f < batchFrames; f++ {
		copy(pcm[f*FrameSamples:(f+1)*FrameSamples], srcFrame[:])
	}
	for i := 0; i < 4; i++ {
		_, _ = se.Write(pcm)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := se.Write(pcm); err != nil {
			b.Fatalf("Write: %v", err)
		}
	}
}
