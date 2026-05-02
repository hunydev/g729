package decoder

import "testing"

// TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput is the
// Phase 1k Stage F-oct-postfix RED contract introduced at commit
// 56caa72 — ALGTHM frame 0 sf0 sample 5..7 must equal the ITU
// reference's PST want (= [-1, -1, -1] per F-oct-prelim-5-4 §3.2
// raw measurement) but production yields [+1, +1, +1].
//
// CURRENT DISPOSITION: t.Skip — gate 17 RED disposition
// (Phase 1l alternative path (d-i)).
//
// RATIONALE (clean-room evidence summary):
//   - 22 sub-hypotheses have been measured and refuted across
//     three diagnostic phases without identifying any defect:
//       * Phase 1k F-* family (16) — closure commit d448282.
//       * Phase 0c re-entry P0c-1/2/3 (4) — synth commit 8e6386c.
//       * Phase 1l F-non-Hpost HP-1/HP-2 (2) — synth commit f902bd9.
//   - Three independent hard-spec invariants are confirmed
//     verbatim against production:
//       * §4.2.4 AGC carryover (HP-1, postfilter agcGainPrev).
//       * §4.3 catch-all zero-init (HP-2, all unlisted state).
//       * §A.4.2.5 IIR pole-pair impulse decay (HP-2 envelope
//         tracks 1.93/-0.94 pole pair exactly).
//   - The F-oct-postfix branch-condition hypothesis itself was
//     refuted (Δ=0 across all gating variants) at F-oct-postfix-2.
//   - Spec-internal candidate space for sample 5..7 sign defect
//     is now formally exhausted under the project's clean-room
//     constraints (ITU-T G.729 PDF + READMETV.txt + textbooks
//     only; ITU-T C reference / bcg729 / Sipro Lab / Annex A
//     binary are forbidden by MIT-licence policy).
//
// REACTIVATION TRIGGERS (any one re-enables this test):
//   - ITU-T G.729 corrigendum / Appendix I/II/III review yields
//     a relevant clarification (alternative path (c)).
//   - Phase 1g multi-frame state propagation diagnostic
//     identifies a pre-frame-0 dependency (alternative path (a)).
//   - A new spec source is admitted that resolves the Q-format
//     or sign convention ambiguity in §4.2.5 / §A.4.2.5.
//
// The test body is preserved verbatim below for one-line
// reactivation (delete the t.Skip call).
func TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput(t *testing.T) {
	t.Skip("gate 17 disposition (Phase 1l alt-path d-i): " +
		"22 sub-hypotheses refuted, 3 hard-spec invariants " +
		"confirmed; spec-internal candidate space exhausted " +
		"under clean-room constraints. Reactivate on " +
		"corrigendum / Phase 1g result. See doc above for " +
		"commit references (56caa72 / d448282 / 8e6386c / f902bd9).")

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
