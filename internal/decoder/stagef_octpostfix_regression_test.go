package decoder

import "testing"

// TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput is the
// Phase 1k Stage F-oct-postfix RED→GREEN regression. ALGTHM
// frame 0 sf0 sample 5..7 must equal the ITU reference's PST
// want sample 5..7 (= [-1, -1, -1] per F-oct-prelim-5-4 §3.2
// raw measurement). Pre-fix production output is [+1, +1, +1]
// (positive); post-fix (Task F-oct-postfix-2) must be
// [-1, -1, -1] (or signs match).
//
// Spec ground-truth: ITU-T G.729 (06/2012) §A.4.2.3 (PDF p.43)
// — γ_t = 0.8 if k1' < 0 else 0 (Annex A); main §4.2.3 (PDF p.29)
// — γ_t = 0.9 if k1' < 0 else 0.2. Production constants currently
// match main §4.2.3 (0.9 / 0.2). The defect being repaired is the
// *branch condition*, not the constants.
//
// Pre-Task-2 commit: this test must be RED (intentional baseline).
// Post-Task-2 commit: this test must be GREEN (fix verification).
func TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput(t *testing.T) {
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

	for n := 5; n <= 7; n++ {
		got, want := out[n], wantFrames[0][n]
		if got != want {
			t.Errorf("frame 0 sample %d: got=%d want=%d (Δ=%d)",
				n, got, want, int32(got)-int32(want))
		}
	}
}
