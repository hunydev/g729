package decoder

import "testing"

// TestDecode_AlgthmFrame0Sf0Sample5to7_KnownPSTDomainDifference preserves
// the historical gate-17 sample window while asserting the current outcome.
//
// Older clean-room diagnostics treated production [0,0,0] vs PST [-1,-1,-1]
// as a documented ambiguity because no admitted numeric oracle explained the
// output HP rounding domain. The 2026-05-15 reference-execution numeric oracle
// resolved that path: the output HP filter must keep its feedback state in the
// native accumulator domain and round only after the final output-scale shift.
//
// The test now pins the corrected behavior: ALGTHM frame 0 samples 5..7 match
// the official PST window [-1,-1,-1]. It remains as a regression guard because
// this small window catches loss of the HP accumulator half-LSB.
func TestDecode_AlgthmFrame0Sf0Sample5to7_KnownPSTDomainDifference(t *testing.T) {
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

	// PST-domain reference for the historical gate-17 window. The previous
	// PASS-by-design pin documented a [0,0,0] production value here; after
	// HP output rounding correction the strict decoder now matches PST.
	wantPST := [3]int16{
		wantFrames[0][5], wantFrames[0][6], wantFrames[0][7],
	}
	expectedPST := [3]int16{-1, -1, -1}
	if wantPST != expectedPST {
		// Reactivation trigger: the documented PST reference has
		// shifted. Re-evaluate the gate 17 disposition before
		// deciding whether to update the pinned values.
		t.Errorf("PST reference for ALGTHM frame 0 sample 5..7 "+
			"changed: got %v, expected %v (per Phase 1k "+
			"F-oct-prelim-5-4 §3.2 raw measurement). Investigate "+
			"before silently updating the documented difference.",
			wantPST, expectedPST)
	}

	got := [3]int16{out[5], out[6], out[7]}
	if got != wantPST {
		t.Errorf("production output for ALGTHM frame 0 sample 5..7 "+
			"= %v; want PST-domain %v after HP output rounding fix.",
			got, wantPST)
	}

	t.Logf("Phase 1o D-1b gate-17 window now matches PST after HP output rounding fix: %v", got)
}
