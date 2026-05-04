package lsp

import (
	"testing"
)

// TestINT1D8DecoderRoundtripFrame0 — d8 §S6 ground truth.
//
// I6 BINDING: read-only on production; t.Logf only.
//
// Mirrors the decoder's MA-predictor reconstruction for the WANT
// frame-0 indices (0, 120, 10, 10) using the encoder-side helper
// applyPredictorWithMemory (which is byte-identical to
// Decoder.applyPredictor by design — see encoder_predictor_test.go).
//
// Then compares the reconstructed ω_decoder against:
//
//	(a) the analytical i·π/11 Q13 seed (what the encoder sees on
//	    cold start with zero PCM input);
//	(b) what the encoder actually computes (run via LPToLSP +
//	    LSPToLSF on a zero-windowed buffer — the steady cold-start
//	    state).
//
// If ω_decoder(WANT) is far from analytical, the ITU encoder did
// NOT see analytical ω at frame 0 — it saw something else (e.g.
// post-LP-analysis ω of a pre-rolled state). This bisects the bug
// between LP-analysis chain (ω-side) and VQ-search.
func TestINT1D8DecoderRoundtripFrame0(t *testing.T) {
	// Cold-start MA predictor memory.
	var fp [4][10]int16
	InitFreqPrev(&fp)

	// WANT indices for LSP.IN frame 0.
	want := Indices{L0: 0, L1: 120, L2: 10, L3: 10}

	// Mirror Decoder.Decode steps 1..3:
	var residual [10]int16
	combineResidual(want.L1, want.L2, want.L3, &residual)
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)

	var omegaWant [10]int16
	applyPredictorWithMemory(want.L0, &fp, &residual, &omegaWant)

	t.Logf("residual l_i (post-rearrange, Q13) for WANT: %v", residual)
	t.Logf("ω_decoder(WANT 0,120,10,10) Q13: %v", omegaWant)
	t.Logf("ω_analytical (i·π/11)     Q13: %v", initialPastResidual)

	var diff [10]int
	var maxAbs, sumAbs int
	for i := 0; i < 10; i++ {
		d := int(omegaWant[i]) - int(initialPastResidual[i])
		diff[i] = d
		a := d
		if a < 0 {
			a = -a
		}
		if a > maxAbs {
			maxAbs = a
		}
		sumAbs += a
	}
	t.Logf("ω_decoder − ω_analytical (Q13 LSB): %v", diff)
	t.Logf("|Δω|_max=%d  |Δω|_sum=%d  (per-coord ≈ %d Q13 LSB avg)",
		maxAbs, sumAbs, sumAbs/10)

	// Encoder cold-start ω: with zero PCM, oldSpeech is zero, the
	// LP-analyzer's stability guard returns a[]=[1,0,0,...,0] (Q12),
	// for which LP→LSP yields q[i]=cos(i·π/11) and LSP→LSF yields
	// ω[i]=i·π/11 — i.e. the encoder's frame-0 ω is _exactly_ the
	// analytical seed (modulo arccos quantization). FIX-2D brought
	// this to ≤7 Q13 LSB drift per d4 §19.2.
	t.Logf("(Encoder frame-0 ω is within ≤7 Q13 LSB of ω_analytical post-FIX-2D — d4 §19.2.)")
	t.Logf("If |Δω|_max above is ≫ 12 Q13 LSB, the WANT indices encode an ω that the ITU encoder produced from a NON-zero LP analysis at frame 0 — i.e. ITU encoded a windowed buffer with non-zero lookahead even though LSP.IN frames 0..4 are all-zero PCM. This implicates the §3.2.1 LP-analysis-window centring (200 past + 40 future) which our pipeline does NOT honour (the encoder.go doc explicitly notes a '1-frame analysis-vs-encode delay').")
}
