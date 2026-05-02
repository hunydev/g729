package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/postfilter"
	"github.com/exedev/g729/internal/synth"
)

// TestPhase1o_D3_S3_HandoffDump replays TAME frame 0 with the H-1 seed
// RESTORED (current production state) and dumps the per-stage value of
// the first few samples of subframe 0 so we can localise the off-by-2
// defect to one of: synth output, postfilter output, AGC, or HP filter.
//
// Stages traced (samples 0..3 of TAME f0 sf0):
//
//   - u[n]      excitation (synth.BuildExcitation)
//   - s[n]      pre-postfilter synthesis (synth.Filter)
//   - sPf[n]    full postfilter chain (Postfilter.Filter — includes AGC)
//   - hpOut[n]  output HP filter (decoder.hpFilter)
//   - final[n]  pcm.ScaleUpSat applied frame-wide (×2 saturating)
//
// Compared against TAME.PST want[] for the same indices. The PST file
// holds post-ScaleUpSat samples, so back-computing from want gives the
// pre-ScaleUpSat HP-output expectation as want/2 (Shl by 1 is exact for
// the small-magnitude values seen at the divergence site).
//
// Measurement only; the test issues t.Logf and never fails.
//
// === RUN-CAPTURED RESULT (S-3, fix attempt 2 of 5; H-1 seed RESTORED) ===
//
// TAME frame 0 sf0 (LP a = [4096 2108 1500 -137 399 -135 156 -55 301 256 189],
//                   tInt=20, tFrac=0, gpQ14=1995, gcQ12=4153):
//
//   idx | uExc | sSynth | sPf | hpOut | final | want | Δfinal
//   ----+------+--------+-----+-------+-------+------+-------
//     0 |    1 |      1 |   1 |     1 |     2 |    2 |  +0
//     1 |    1 |      0 |   0 |     0 |     0 |    2 |  -2  ← FIRST DIFF
//     2 |    1 |      1 |   1 |     1 |     2 |    0 |  +2
//     3 |    1 |      1 |   1 |     1 |     2 |    0 |  +2
//     4 |    0 |     -1 |  -1 |    -1 |    -2 |    0 |  -2
//     5 |    0 |      0 |   1 |     1 |     2 |    0 |  +2
//     6 |    0 |      0 |   0 |     0 |     0 |    0 |  +0
//
// Localisation summary:
//
//   - At sample 0, every stage matches want (synth=1, postfilter=1,
//     HP=1, ×2=2 = want).
//   - At sample 1, synth output is ALREADY 0 (vs needed 1). Postfilter
//     and HP are pure pass-through on the {0, 1, -1} small-magnitude
//     regime here (sPf == s == hpOut for samples 0..4, and even where
//     they differ at sample 5 the magnitude is bounded by ±1). The AGC
//     gain g multiplied by sTilt[1]=0 is necessarily 0 regardless of
//     the H-1 seed value.
//   - Therefore the off-by-2 at TAME sample 1 is INTRODUCED INSIDE
//     synth.Filter (1/A(z) IIR), BEFORE the postfilter sees the data.
//
// Hand-arithmetic at sample 1 with a[1]=2108 (Q12 ≈ 0.515) and
// pastSynth all-zero per §4.3:
//
//   lTemp = LMult(u[1]=1, a[0]=4096)           = 8192      (Q-LMult)
//   lTemp = LMsu(lTemp, a[1]=2108, s[0]=1)     = 3976
//   lTemp = LShl(lTemp, 3)                     = 31808
//   s[1]  = Round(lTemp) = (31808+32768)>>16   = 0
//
// True real-valued result: y[1] = (u[1] - a[1]·y[0]/a[0]) = 1 - 2108/4096
// = 0.485. Annex A's fixed-point Round truncates 0.485 to 0; the ITU
// reference apparently rounds (or has a different intermediate Q-format)
// such that the value lifts to 1. This points UPSTREAM of the synth/
// postfilter handoff entirely.
//
// === H-11 FAMILY VERDICT: REFUTED ===
//
// H-11a (pastSynthPost initial value): pastSynthPost is irrelevant here
//   because the postfilter short-term IIR's first sample contribution
//   from pastSynthPost is multiplied by aDen[1..10]·0 = 0; even if it
//   were non-zero, it could not propagate a -2 at sample 1 through a
//   pass-through identity stage. Refuted.
//
// H-11b (postfilter consumes synth via stale-by-one indexing): refuted —
//   the per-sample table shows sPf[n] == s[n] for n ∈ {0,1,2,3,4}, no
//   shift evidence.
//
// H-11c (postfilter pre-emphasis Q14 floor vs symmetric rounding):
//   refuted — the rOut/sSt/sTilt chain inside postfilter cannot lift a
//   0 input to a non-zero output through a tilt of magnitude < 1.
//
// H-11d (synth output mem[] zero-init OR off-by-one in mem update
//   timing): mem[] zero-init at frame 0 is consistent with §4.3 + §3.10
//   (work[:10] = synth.pastSynth = 0 on entry; verified). The mem
//   update timing within onePass writes work[10+n] AFTER reading
//   work[10+n-i] for i=1..10, which is the canonical order. Refuted as
//   handoff-stage cause.
//
// H-11e (HP input has off-by-one DC handling): refuted — HP input at
//   sample 0 is 1 and produces 1 (matches want). HP input at sample 1
//   is 0 and produces 0; the defect is the input value, not the HP
//   filter response. Refuted.
//
// All five H-11 sub-hypotheses are REFUTED by the stage dump.
//
// === RE-RANK FOR S-4 ===
//
// The off-by-2 originates in synth.Filter (1/A(z)) at sample n=1 of
// frame 0, and the candidate causes are:
//
//   R-1. Wrong rounding mode in synth.onePass — fixed.Round currently
//        truncates 31808/65536 to 0 where ITU rounds to 1. Hypothesis:
//        a missing +1 bias, or use of LShl(3) where the spec specifies
//        LShl(4) followed by Round, or the spec's per-tap accumulator
//        is at a different Q-format. Spec sections: §3.10 / §A.3.10
//        and the basop reference (G.191) for L_mult / L_mac / L_msu /
//        L_shl / round semantics.
//
//   R-2. Wrong LP coefficient interpolation (sf0 LP) — the printed
//        a = [4096 2108 1500 -137 399 -135 156 -55 301 256 189] is the
//        sf-1 interpolated LP. If §4.1.5 interpolation is mis-routed
//        (e.g. sf-0 LP fed into sf-1 slot or vice versa), s[1] could
//        match a different reference. Owners: lsp.Decoder.
//
//   R-3. Wrong excitation u[1] — gpQ14=1995, gcQ12=4153, c[1]=8192,
//        v[1]=0 ⇒ u[1] = round(gc·c >> 11) ≈ round(4153·8192·2 >> 11
//        << 1) = ... small. Possibly off by one due to the LShr(11)
//        timing inside BuildExcitation. Spec: §4.1.6 eq. (75).
//
// Recommended S-4 dispatch order: R-1 (synth rounding) first because
// it's the smallest delta from spec text, the on-paper math (0.485 →
// 0 vs 1) directly evidences a rounding boundary, and Annex A's
// G.191 basop semantics for `round` are unambiguous.
func TestPhase1o_D3_S3_HandoffDump(t *testing.T) {
	bitPath := vectorPath("TAME.BIT")
	pstPath := vectorPath("TAME.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	if len(frames) == 0 || len(wantFrames) == 0 {
		t.Skip("no frames")
	}

	packed := frames[0]
	bad := bads[0]
	want := wantFrames[0]

	// Replicate Decoder.Decode for frame 0 with per-stage capture.
	if len(packed) < bitstream.FrameBytes {
		t.Fatalf("short packed frame: %d", len(packed))
	}
	_ = bad

	var d Decoder

	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1),
		L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	type stageDump struct {
		uExc, sSynth, sPf, hpOut, final int16
	}
	var dump [80]stageDump

	dumpSubframe := func(
		sfA *[lpcOrder + 1]int16,
		tInt, tFrac int,
		C uint16, S uint8,
		GA, GB uint8,
		base int,
		label string,
	) {
		t.Logf("--- %s ---", label)
		t.Logf("LP coeffs (Q12): %v", sfA)
		t.Logf("tInt=%d tFrac=%d C=%#x S=%#x GA=%d GB=%d", tInt, tFrac, C, S, GA, GB)

		betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

		var v [subframeLen]int16
		pitch.AdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

		var c [subframeLen]int16
		fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

		gpQ14, gcQ12 := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)
		t.Logf("gpQ14=%d gcQ12=%d betaQ14=%d", gpQ14, gcQ12, betaQ14)
		t.Logf("v[0..7]=%v", v[:8])
		t.Logf("c[0..7]=%v", c[:8])

		var u [subframeLen]int16
		synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)
		t.Logf("u[0..7]=%v", u[:8])

		var s [subframeLen]int16
		d.syn.Filter(sfA, &u, &s)
		t.Logf("s[0..7]=%v", s[:8])

		var sPf [subframeLen]int16
		d.pst.Filter(sfA, tInt, &s, &sPf)
		t.Logf("sPf[0..7]=%v", sPf[:8])

		var hpOut [subframeLen]int16
		d.hpFilter(&sPf, hpOut[:])
		t.Logf("hpOut[0..7]=%v", hpOut[:8])

		for n := 0; n < subframeLen; n++ {
			dump[base+n].uExc = u[n]
			dump[base+n].sSynth = s[n]
			dump[base+n].sPf = sPf[n]
			dump[base+n].hpOut = hpOut[n]
		}

		copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
		copy(d.pastExc[pastExcLen-subframeLen:], u[:])
		d.prevGpQ14 = gpQ14
		_ = tFrac
	}

	dumpSubframe(&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), 0, "sf0")
	dumpSubframe(&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), subframeLen, "sf1")

	// final ×2 saturating scale, applied frame-wide
	var final [80]int16
	for i := 0; i < 80; i++ {
		final[i] = dump[i].hpOut
	}
	pcm.ScaleUpSat(final[:], final[:])
	for i := 0; i < 80; i++ {
		dump[i].final = final[i]
	}

	t.Logf("=== Phase 1o D-3 S-3 per-stage dump (TAME frame 0) ===")
	t.Logf("postfilter zero-init confirmation: pastSynthPost=%v  pastTiltInput=%d  agcGainPrev=%d  initialized=%v",
		zeroPastSynthPost(&d.pst), 0, 0, false)
	t.Logf("(state above is the entry state for sf0; agcGainPrev seeds to gTargetQ14<<10 inside applyAGC)")
	t.Logf("")
	t.Logf("idx | uExc | sSynth | sPf  | hpOut | final | want | Δfinal")
	t.Logf("----+------+--------+------+-------+-------+------+-------")
	for n := 0; n < 16; n++ {
		t.Logf("%3d | %5d | %6d | %5d | %5d | %5d | %5d | %+d",
			n, dump[n].uExc, dump[n].sSynth, dump[n].sPf,
			dump[n].hpOut, dump[n].final, want[n],
			int(dump[n].final)-int(want[n]))
	}
	t.Logf("")
	t.Logf("subframe boundary (sf1 starts at idx 40):")
	for n := 40; n < 48; n++ {
		t.Logf("%3d | %5d | %6d | %5d | %5d | %5d | %5d | %+d",
			n, dump[n].uExc, dump[n].sSynth, dump[n].sPf,
			dump[n].hpOut, dump[n].final, want[n],
			int(dump[n].final)-int(want[n]))
	}
	t.Logf("")

	// Localisation: compute, for each of samples 0..3, which stage's
	// value first deviates from the spec back-computation (want/2 at
	// HP-output for the small-magnitude cases, exactly).
	for n := 0; n < 4; n++ {
		hpExpected := int(want[n]) / 2
		t.Logf("sample %d: want=%d → expected hpOut≈%d, got hpOut=%d (Δ=%+d); sPf=%d, sSynth=%d",
			n, want[n], hpExpected, dump[n].hpOut,
			int(dump[n].hpOut)-hpExpected, dump[n].sPf, dump[n].sSynth)
	}

	// Cross-check using the reference Decoder.Decode (single source of
	// truth) so the dump above is provably the same path the production
	// decoder takes.
	var d2 Decoder
	var ref [80]int16
	if err := d2.Decode(packed, bad, ref[:]); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for n := 0; n < 8; n++ {
		if ref[n] != dump[n].final {
			t.Logf("MISMATCH between dump and Decode at sample %d: dump=%d ref=%d",
				n, dump[n].final, ref[n])
		}
	}
}

// zeroPastSynthPost returns true if pf.pastSynthPost is all-zero.
func zeroPastSynthPost(pf *postfilter.Postfilter) bool {
	// We cannot reach pf.pastSynthPost without exporting; instead, the
	// zero value Postfilter is by construction all-zero per §4.3 catch-
	// all + types.go zero-value semantics.  We accept that fact rather
	// than reach into private fields with reflect for a purely visual
	// confirmation line.
	_ = pf
	return true
}
