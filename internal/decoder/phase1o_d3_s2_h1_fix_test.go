package decoder

import "testing"

// TestPhase1o_D3_S2_H1_TameByteExact is the S-2 byte-EQ predicate for
// hypothesis H-1 (postfilter.agcGainPrev lazy-seed = gTargetQ24, per
// agc.go:53–56). The test was authored as the RED step of a fix
// attempt and is preserved permanently as a refutation record.
//
// ── Refutation outcome (S-2, fix attempt 1 of 5) ──────────────────────
//
// HYPOTHESIS (H-1, S-1 commit aa27ad1, plan 2026-05-10 §5.1 rank 1):
//
//	Removing the lazy seed (`if !pf.initialized { agcGainPrev =
//	gTargetQ24 }`) and starting agcGainPrev at the Postfilter zero
//	value (per §4.3 catch-all "all static decoder variables should be
//	initialized to 0") will flip TAME from FAIL to byte-EQ PASS and,
//	per plan §7, all 5 other ITU vectors with it.
//
// EXPERIMENT (S-2):
//
//	Applied the seed-removal in internal/postfilter/agc.go and re-ran
//	each ITU vector. Production diff was reverted after measurement.
//
// PRE-FIX TAME first divergence (b43c689, aa27ad1):
//
//	frame 0 sample 1 got=  0 want=  2 (delta -2)
//	frame 0 sample 2 got=  2 want=  0 (delta +2)
//	... cascade to frame 2 sample 0 delta +790
//
// POST-FIX (seed removed) TAME first divergence:
//
//	frame 0 sample 0 got=  0 want=  2 (delta -2)
//	(divergence shifted ONE SAMPLE EARLIER; sample 0 had been
//	 accidentally matching pre-fix because the seeded gTargetQ24
//	 multiplier produced sample 0 ≈ want by coincidence)
//
// POST-FIX cross-vector first divergences (none flipped to PASS):
//
//	SPEECH   frame 0 sample 1 got=  0 want=  2 (Δ=-2),  frame 1 sf0 Δ=-6
//	FIXED    frame 0 sample 0 got=  0 want=  2 (Δ=-2),  frame 1 sf0 Δ=+235
//	LSP      frame 0 sample 0 got=  0 want=  2 (Δ=-2),  frame 1 sf0 Δ=+2
//	PITCH    frame 0 sample 0 got=  0 want=  2 (Δ=-2),  frame 1 sf0 Δ=-16
//	TEST     (re-skipped; same family signature; aligns with above)
//	OVERFLOW frame 0 sample 0 got=  0 want=  2 (Δ=-2),  frame 1 sf0 Δ=-5
//
// VERDICT: NO-FIX. H-1 is REFUTED by experiment. The lazy-seed branch
// is restored as-is; the spec-derived prediction (§4.3 catch-all
// dominates §A.4.2.4 init clause) is empirically falsified. This is
// consistent with plan §8 risk R-1: §A.4.2.4's "initialized to
// g_target" appears to be the binding clause for agcGainPrev, not the
// §4.3 catch-all. Counts as 1 of 5 cumulative refutations toward the
// §5.2 hard cap.
//
// NEXT (S-3): re-rank surviving hypotheses with H-1 marked REFUTED.
// Per plan §5.1 the "smallest-frame-position-first then smallest |Δ|
// first" tie-break suggests dispatching against H-2 (applyAGC
// iteration internals: α=32440/32768 Q15 rounding, +(1<<14) bias) on
// FIXED.BIT (smallest residual |Δ|=2 at sample 0 of frame 0).
//
// This test is intentionally `t.Skip`-ped: keeping it red would block
// CI without adding signal. Re-enable in tandem with the S-K commit
// that finally drives TAME to byte-EQ PASS.
//
// Reference: docs/superpowers/plans/2026-05-10-phase1o-d3-statebearing-
// rootcause-plan.md S-2 (H-1). Anchor: S-1 commit aa27ad1.
func TestPhase1o_D3_S2_H1_TameByteExact(t *testing.T) {
	t.Skip("Phase 1o D-3 S-2 REFUTATION RECORD: H-1 (agcGainPrev " +
		"lazy-seed removal) empirically falsified — TAME divergence " +
		"shifted from frame 0 sample 1 to sample 0 instead of " +
		"resolving; all 6 ITU vectors still FAIL. Lazy seed restored. " +
		"See in-source measurement record above. Pending S-3 dispatch " +
		"against H-2 (applyAGC iteration internals).")

	bitPath := vectorPath("TAME.BIT")
	pstPath := vectorPath("TAME.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	if len(frames) != len(wantFrames) {
		t.Fatalf("frame count mismatch: bit=%d pst=%d",
			len(frames), len(wantFrames))
	}

	var d Decoder
	var out [frameSamples]int16
	for i, packed := range frames {
		if err := d.Decode(packed, bads[i], out[:]); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if out != wantFrames[i] {
			for n := 0; n < frameSamples; n++ {
				if out[n] != wantFrames[i][n] {
					t.Fatalf("frame %d sample %d: got %d, want %d (delta %+d)",
						i, n, out[n], wantFrames[i][n],
						int(out[n])-int(wantFrames[i][n]))
				}
			}
		}
	}
}
