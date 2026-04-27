package decoder

import "testing"

// TestDecode_Frame0Sample0_MatchesALGTHM is the Phase 1i regression
// guard: ALGTHM frame 0 sample 0 must equal the ITU reference's
// sample 0. ANY change in this phase that regresses this assertion
// must be rolled back per Phase 1k spec §7.2.
func TestDecode_Frame0Sample0_MatchesALGTHM(t *testing.T) {
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
	if out[0] != wantFrames[0][0] {
		t.Errorf("frame 0 sample 0: got=%d want=%d (Δ=%d)",
			out[0], wantFrames[0][0], int32(out[0])-int32(wantFrames[0][0]))
	}
}

// TestDecode_Frame0SF1_DiagnosticLog observes sf1 (samples 0..39)
// against ALGTHM.PST. No assertions — purely diagnostic. Used during
// Stage F as a moving target: as the 14 dB fix lands, this output
// shows how many sf1 samples now match.
//
// In Stage V (Task 9) this becomes an assertion test for the full
// frame (samples 0..79).
func TestDecode_Frame0SF1_DiagnosticLog(t *testing.T) {
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
	for n := 0; n < 40; n++ {
		t.Logf("sf1 sample %2d: got=%6d want=%6d Δ=%+d",
			n, out[n], wantFrames[0][n],
			int32(out[n])-int32(wantFrames[0][n]))
	}
}
