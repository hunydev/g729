package decoder

import "testing"

// TestDiagnostic_FoctPostfix2PrelimChainDump dumps the Annex A postfilter
// chain stage outputs for ALGTHM frame 0 sf0 sample 5..7 — the common
// ground-truth for Tasks F-oct-postfix2-prelim-2/3/4 (M5/M6/M1'+M3
// hypothesis differential measurement).
//
// Spec ground-truth: ITU-T G.729 (06/2012) §A.4.2 (PDF p.43) chain
// order = long-term → short-term → tilt → AGC. F-oct-postfix synthesis
// (8907847) §2.4 identifies the sign-determining term as residing
// *outside* tilt compensation (Δ=0 measurement); this dump enables
// stage-by-stage sign tracing.
//
// production 변경 0. assertion 0 (measurement-only).
func TestDiagnostic_FoctPostfix2PrelimChainDump(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
		t.Fatalf("Decode frame 0: %v", err)
	}

	t.Logf("ALGTHM frame 0 sf0 sample 5..7 (PST want = [%d %d %d])",
		wantFrames[0][5], wantFrames[0][6], wantFrames[0][7])
	t.Logf("  decoded out[5..7] (post-hpfilter)            = [%d %d %d]",
		out[5], out[6], out[7])
	t.Logf("  delta vs PST want                            = [%d %d %d]",
		int32(out[5])-int32(wantFrames[0][5]),
		int32(out[6])-int32(wantFrames[0][6]),
		int32(out[7])-int32(wantFrames[0][7]))
	// Additional stage dumps (excitation, synth IIR, postfilter chain)
	// are added in Tasks 2/4 via stage-specific harnesses or Decoder
	// instrumentation hooks if exposed; this baseline records the
	// externally observable terminal output for cross-reference.
}
