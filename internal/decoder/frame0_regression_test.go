package decoder

import "testing"

// TestDecode_Frame0Sample0_MatchesALGTHM is the Phase 1i regression
// guard, re-pinned after the strict decoder output scale was restored to
// the spec post-HP x2 gain. It
// keeps the ALGTHM frame-0 sample-0 output deliberate instead of allowing
// silent drift.
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
	const wantProd int16 = 2
	const wantPST int16 = 2
	if wantFrames[0][0] != wantPST {
		t.Fatalf("ALGTHM.PST frame 0 sample 0 changed: got=%d want=%d",
			wantFrames[0][0], wantPST)
	}
	if out[0] != wantProd {
		t.Errorf("frame 0 sample 0: got=%d want production pin %d (PST=%d, ΔPST=%d)",
			out[0], wantProd, wantFrames[0][0], int32(out[0])-int32(wantFrames[0][0]))
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
