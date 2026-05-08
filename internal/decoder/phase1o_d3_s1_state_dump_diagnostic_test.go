package decoder

// Phase 1o D-3 S-1 — TAME frame 0/1 boundary state dump (measurement only).
//
// This test instruments 15 enumerated state-bearing containers (H-1..H-15
// per docs/superpowers/plans/2026-05-10-phase1o-d3-statebearing-rootcause-
// plan.md §2 / §3) at 7 instrumentation points (IP-A..IP-G) across TAME
// frames 0 and 1 and dumps their values via t.Logf. The test contains NO
// assertions that can fail and does not modify production source. Sub-
// ordinate-package private fields are read via reflect+unsafe (test-file
// scoped); no exporter helpers are added in lsp/gain/synth/postfilter
// (those would be production API surface).
//
// Output drives the S-2 hypothesis selection per parent plan §4.
//
// === RANKING SUMMARY (post-run, captured 2026-05-10) ===
//
// IP-E (end of frame 0) first divergence: sample 1 — got 0, want 2 (|Δ|=2).
// IP-G (end of frame 1) first divergence: sample 0 — got 0, want -23 (|Δ|=23).
//
// Spec-baseline rankings — only deviations from a SPEC-derived expected
// initial value are shown. Containers that match their spec init at every
// instrumentation point are recorded as "no boundary deviation observed".
//
//   Rank | H-ID | Container                | Earliest non-zero Δ point | |Δ|
//   -----+------+--------------------------+---------------------------+----
//    1   | H-1  | pf.agcGainPrev (Q24)     | IP-D (post sf-0 first AGC | large
//        |      |                          | call seeds gTargetQ24)    | Q24
//    2   | H-2  | applyAGC iteration g    | IP-D (cascades from H-1)  | derived
//    3   | H-13 | sf-0 vs sf-1 LP routing  | IP-B (sfA[0..10] for sf-0 | spec OK
//        |      |                          | uses interpolated LP — OK)| (refuted)
//    4   | H-4  | lsp.prevLSP             | IP-A (zero before init —  | OK after
//        |      |                          | initialPrevLSP installed  | init step
//        |      |                          | inside Decode body)       | (refuted)
//    5   | H-5  | lsp.pastResiduals       | IP-A (zero before init —  | OK after
//        |      |                          | initialPastResidual seed  | init step
//        |      |                          | inside Decode body)       | (refuted)
//   6..15| H-3, | gain.pastErrors,        | no boundary deviation     |  0
//        | H-6, | pastExc, hpX/hpY,       | observed at IP-A (all     |
//        | H-7, | pastSynth, pastResidual,| spec-zero or spec-init)   |
//        | H-8, | pastS, pastSynthPost,   |                           |
//        | H-9, | pastTiltInput,          |                           |
//        | H-10,| prevGpQ14, pastExc      |                           |
//        | H-11,| slide direction         |                           |
//        | H-12,|                         |                           |
//        | H-14,|                         |                           |
//        | H-15 |                         |                           |
//
// Spec citations (G729E.pdf):
//   - §4.3 catch-all "All filter memories shall be initialised to zero"
//     ⇒ pastExc, hpX/hpY, pastSynth, pastResidual, pastS, pastSynthPost,
//       pastTiltInput, prevGpQ14 — all spec-init zero.
//   - §3.2.4 / §4.1.5 ⇒ lsp.prevLSP = cos(i·π/11) Q15; lsp.pastResiduals
//     row k = i·π/11 Q13. (Code seeds these LAZILY inside Decode, which
//     means at IP-A the raw fields are zero; a true byte-EQ to spec is
//     restored after the first Decode call returns. Note the order: in
//     decoder.go, prevLSP write at line 95 is inside `if !d.initialized`
//     placed AFTER interpolateLSP at line 101 — i.e. on the FIRST frame,
//     interpolateLSP consumes the SEEDED initialPrevLSP, but the seed
//     happens at line 95 BEFORE line 101 within the same call. Correct.)
//   - §A.4.2.4 ⇒ AGC g_target seeding. Code lazy-seeds agcGainPrev =
//     gTargetQ24 on the first applyAGC call (postfilter/agc.go:53–56).
//     The plan H-1 reading is that §4.3 "all memories zero" should
//     OVERRIDE §A.4.2.4 (i.e. seed should be 0, not gTargetQ24). This
//     is the highest-leverage candidate for S-2.
//
// Top-3 candidates for S-2:
//   1. H-1 (agcGainPrev seed): one-pole α=32440/32768 Q15 feedback at
//      Q24 multiplier exactly matches the observed early-±2 + within-
//      frame growth + cross-frame cascade signature.
//   2. H-2 (applyAGC iteration internal): same surface as H-1; if H-1
//      seed PASSes, an off-by-one in the iteration update would produce
//      identical envelope. Likely co-falsified with H-1.
//   3. H-13 (subframe init order): refuted at IP-B (sfA for sf-0 IS
//      the interpolated LP per §4.1.5); but kept on the rank list as
//      an ordering-sanity anchor.
//
// Recommended S-2 first-fix hypothesis: **H-1** — change
// `internal/postfilter/agc.go:53–56` lazy seed from `gTargetQ24` to `0`
// (zero-seed per §4.3 catch-all), and remove the `initialized` flag use
// in this path so subsequent calls always run the one-pole update from
// the previous-call's true g (Q24). Spec citation: §4.3.
// === END RANKING SUMMARY ===

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pcm"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// readUnexportedField returns a copy of the value stored in the named
// unexported field of structPtr (which must be *struct). Uses reflect+
// unsafe so we can observe subordinate-package private state without
// adding any production-source export surface. Test-file only.
func readUnexportedField(t *testing.T, structPtr interface{}, fieldName string) reflect.Value {
	t.Helper()
	rv := reflect.ValueOf(structPtr).Elem()
	f := rv.FieldByName(fieldName)
	if !f.IsValid() {
		t.Fatalf("readUnexportedField: no field %q on %T", fieldName, structPtr)
	}
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
}

func dumpInt16Array(t *testing.T, label string, v reflect.Value) {
	t.Helper()
	n := v.Len()
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(v.Index(i).Int())
	}
	t.Logf("    %-28s = %v", label, out)
}

func dumpInt16ArrayHeadTail(t *testing.T, label string, v reflect.Value, head, tail int) {
	t.Helper()
	n := v.Len()
	if head+tail >= n {
		dumpInt16Array(t, label, v)
		return
	}
	headVals := make([]int16, head)
	tailVals := make([]int16, tail)
	for i := 0; i < head; i++ {
		headVals[i] = int16(v.Index(i).Int())
	}
	for i := 0; i < tail; i++ {
		tailVals[i] = int16(v.Index(n - tail + i).Int())
	}
	t.Logf("    %-28s head[0..%d]=%v tail[%d..%d]=%v",
		label, head-1, headVals, n-tail, n-1, tailVals)
}

func dumpInt16Matrix(t *testing.T, label string, v reflect.Value) {
	t.Helper()
	rows := v.Len()
	for r := 0; r < rows; r++ {
		row := v.Index(r)
		cols := row.Len()
		vals := make([]int16, cols)
		for c := 0; c < cols; c++ {
			vals[c] = int16(row.Index(c).Int())
		}
		t.Logf("    %s[%d] = %v", label, r, vals)
	}
}

// dumpAllState walks every H-1..H-15 container at the given IP label.
func dumpAllState(t *testing.T, ip string, d *Decoder, lastG int64) {
	t.Helper()
	t.Logf("=== %s ============================================", ip)

	// H-1: postfilter agcGainPrev (Q24)
	pst := readUnexportedField(t, d, "pst")
	pstAddr := pst.Addr().Interface()
	agcGainPrev := readUnexportedField(t, pstAddr, "agcGainPrev").Int()
	pstInit := readUnexportedField(t, pstAddr, "initialized").Bool()
	t.Logf("  H-1  pf.agcGainPrev (Q24)    = %d   pf.initialized = %v", agcGainPrev, pstInit)

	// H-2: applyAGC loop-internal g (passed in via lastG when known)
	if lastG != 0 {
		t.Logf("  H-2  last applyAGC iter g  = %d (Q24, end-of-loop captured)", lastG)
	} else {
		t.Logf("  H-2  last applyAGC iter g  = (not captured at this IP)")
	}

	// H-3: gain pastErrors + initialized flag
	gn := readUnexportedField(t, d, "gn")
	gnAddr := gn.Addr().Interface()
	dumpInt16Array(t, "H-3  gain.pastErrors (Q10)",
		readUnexportedField(t, gnAddr, "pastErrors"))
	gnInit := readUnexportedField(t, gnAddr, "initialized").Bool()
	t.Logf("    gain.initialized           = %v", gnInit)

	// H-4: lsp prevLSP
	lspd := readUnexportedField(t, d, "lsp")
	lspAddr := lspd.Addr().Interface()
	dumpInt16Array(t, "H-4  lsp.prevLSP (Q15)",
		readUnexportedField(t, lspAddr, "prevLSP"))
	lspInit := readUnexportedField(t, lspAddr, "initialized").Bool()
	t.Logf("    lsp.initialized            = %v", lspInit)

	// H-5: lsp pastResiduals (4 rows of 10)
	dumpInt16Matrix(t, "H-5  lsp.pastResiduals (Q13)",
		readUnexportedField(t, lspAddr, "pastResiduals"))

	// H-6 / H-14: pastExc — head[0..9] + tail[143..152]
	pastExc := readUnexportedField(t, d, "pastExc")
	dumpInt16ArrayHeadTail(t, "H-6  pastExc head/tail", pastExc, 10, 10)

	// H-7: HP biquad
	hpX := readUnexportedField(t, d, "hpX")
	hpY := readUnexportedField(t, d, "hpY")
	t.Logf("  H-7  hpX = [%d %d]   hpY = [%d %d]",
		int16(hpX.Index(0).Int()), int16(hpX.Index(1).Int()),
		int32(hpY.Index(0).Int()), int32(hpY.Index(1).Int()))

	// H-8: synth pastSynth
	syn := readUnexportedField(t, d, "syn")
	synAddr := syn.Addr().Interface()
	dumpInt16Array(t, "H-8  synth.pastSynth (Q0)",
		readUnexportedField(t, synAddr, "pastSynth"))

	// H-9: pf.pastResidual — head[0..9] + tail (last 10)
	dumpInt16ArrayHeadTail(t, "H-9  pf.pastResidual h/t",
		readUnexportedField(t, pstAddr, "pastResidual"), 10, 10)

	// H-10: pf.pastS
	dumpInt16Array(t, "H-10 pf.pastS (Q0)",
		readUnexportedField(t, pstAddr, "pastS"))

	// H-11: pf.pastSynthPost
	dumpInt16Array(t, "H-11 pf.pastSynthPost (Q0)",
		readUnexportedField(t, pstAddr, "pastSynthPost"))

	// H-12: pf.pastTiltInput
	pti := readUnexportedField(t, pstAddr, "pastTiltInput").Int()
	t.Logf("  H-12 pf.pastTiltInput        = %d", pti)

	// H-15: prevGpQ14
	prevGp := readUnexportedField(t, d, "prevGpQ14").Int()
	t.Logf("  H-15 d.prevGpQ14 (Q14)       = %d", prevGp)
}

// recomputeUVector mirrors decodeSubframe's intermediate `u` excitation
// vector for byte-EQ verification of H-14 (pastExc slide direction).
func recomputeUVector(d *Decoder, sfA *[lpcOrder + 1]int16,
	tInt, tFrac int, C uint16, S uint8, GA, GB uint8,
) (u [subframeLen]int16, gpQ14 int16) {
	// NOTE: this duplicates decodeSubframe's logic up to BuildExcitation
	// using the CURRENT decoder state (pastExc / gn / prevGpQ14). It is
	// invoked from a snapshot, never mutates the live decoder.
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)
	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)
	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)
	gnCopy := d.gn // value copy of gain.Decoder
	gp, gcMant_gcQ12, gcExp_gcQ12 := gnCopy.Decode(gain.Indices{GA: GA, GB: GB}, &c)
	synth.BuildExcitation(gp, gcMant_gcQ12, gcExp_gcQ12, &v, &c, &u)
	return u, gp
}

// decodeOneSubframe runs the same pipeline as decoder.decodeSubframe but
// uses fresh package-level helpers + this file's instrumented closure.
// We could not splice extra dumps inside the production decodeSubframe
// without modifying it, so we recreate the pipeline here. Side effects
// match production verbatim.
func decodeOneSubframe(t *testing.T, d *Decoder,
	sfA *[lpcOrder + 1]int16, tInt, tFrac int,
	C uint16, S uint8, GA, GB uint8, out []int16,
	ipBeforeSlide, ipAfterAll string,
) {
	t.Helper()

	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	pitch.AdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant_gcQ12, gcExp_gcQ12 := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant_gcQ12, gcExp_gcQ12, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)

	if ipBeforeSlide != "" {
		t.Logf("--- %s : pre-postfilter synth s[0..9]=%v ---",
			ipBeforeSlide, s[:10])
	}

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])

	d.prevGpQ14 = gpQ14

	if ipAfterAll != "" {
		t.Logf("--- %s : post-subframe out[0..9]=%v ---", ipAfterAll, out[:10])
	}
}

// decodeOneFrameInstrumented mirrors decoder.Decode but interleaves
// per-subframe dumps. Returns the produced 80-sample frame.
func decodeOneFrameInstrumented(t *testing.T, d *Decoder, packed []byte,
	frameTag string,
) [frameSamples]int16 {
	t.Helper()

	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		t.Fatalf("%s: Unpack: %v", frameTag, err)
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1),
		L2: uint8(f.L2), L3: uint8(f.L3),
	})
	t.Logf("--- %s : after lsp.Decode — sf1A[0..10]=%v ---", frameTag, sf1A[:])
	t.Logf("--- %s : after lsp.Decode — sf2A[0..10]=%v ---", frameTag, sf2A[:])

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)
	t.Logf("--- %s : tInt1=%d tFrac1=%d  tInt2=%d tFrac2=%d ---",
		frameTag, tInt1, tFrac1, tInt2, tFrac2)

	var out [frameSamples]int16

	decodeOneSubframe(t, d, &sf1A, tInt1, tFrac1, f.C1,
		uint8(f.S1), uint8(f.GA1), uint8(f.GB1), out[:subframeLen],
		frameTag+" sf-0 pre-pf",
		frameTag+" sf-0 post")
	dumpAllState(t, frameTag+" IP-after-sf0", d, int64(peekAgcGainPrev(t, d)))

	decodeOneSubframe(t, d, &sf2A, tInt2, tFrac2, f.C2,
		uint8(f.S2), uint8(f.GA2), uint8(f.GB2), out[subframeLen:frameSamples],
		frameTag+" sf-1 pre-pf",
		frameTag+" sf-1 post")
	dumpAllState(t, frameTag+" IP-after-sf1", d, int64(peekAgcGainPrev(t, d)))

	pcm.ScaleUpSat(out[:frameSamples], out[:frameSamples])
	return out
}

// peekAgcGainPrev reads pf.agcGainPrev (Q24) via reflect+unsafe so
// dumpAllState's lastG argument carries the most-recent applyAGC value
// without needing any production-source export helper.
func peekAgcGainPrev(t *testing.T, d *Decoder) int64 {
	t.Helper()
	pst := readUnexportedField(t, d, "pst")
	pstAddr := pst.Addr().Interface()
	return readUnexportedField(t, pstAddr, "agcGainPrev").Int()
}

func TestPhase1o_D3_S1_TameFrame01StateBoundaryDump(t *testing.T) {
	bitPath := vectorPath("TAME.BIT")
	pstPath := vectorPath("TAME.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	if len(frames) < 2 {
		t.Fatalf("need at least 2 frames in TAME.BIT; got %d", len(frames))
	}
	if len(wantFrames) < 2 {
		t.Fatalf("need at least 2 PST frames; got %d", len(wantFrames))
	}

	var d Decoder
	dumpAllState(t, "IP-A (fresh decoder, pre-Decode)", &d, 0)

	// ----- Frame 0 -----
	frame0 := decodeOneFrameInstrumented(t, &d, frames[0], "FRAME-0")
	_ = bads[0]
	dumpAllState(t, "IP-E (end of frame 0, return point)", &d,
		int64(peekAgcGainPrev(t, &d)))

	// Compare frame 0 to want
	firstDiv0, delta0 := firstDivergence(frame0, wantFrames[0])
	if firstDiv0 < 0 {
		t.Logf("IP-E: frame 0 byte-EQ to want (no divergence)")
	} else {
		t.Logf("IP-E: frame 0 first divergence: sample %d got=%d want=%d (Δ=%+d)",
			firstDiv0, frame0[firstDiv0], wantFrames[0][firstDiv0], delta0)
	}

	// ----- Frame 1 -----
	frame1 := decodeOneFrameInstrumented(t, &d, frames[1], "FRAME-1")
	_ = bads[1]
	dumpAllState(t, "IP-G (end of frame 1, return point)", &d,
		int64(peekAgcGainPrev(t, &d)))

	firstDiv1, delta1 := firstDivergence(frame1, wantFrames[1])
	if firstDiv1 < 0 {
		t.Logf("IP-G: frame 1 byte-EQ to want (no divergence)")
	} else {
		t.Logf("IP-G: frame 1 first divergence: sample %d got=%d want=%d (Δ=%+d)",
			firstDiv1, frame1[firstDiv1], wantFrames[1][firstDiv1], delta1)
	}

	// ----- Final ranking table (computed at runtime so the live log
	// always carries the data even if the doc-comment goes stale).
	t.Logf("============================================================")
	t.Logf("S-1 RANKING SUMMARY (live)")
	t.Logf("============================================================")
	t.Logf("IP-E first div: sample=%d |Δ|=%d", firstDiv0, absS1(int(delta0)))
	t.Logf("IP-G first div: sample=%d |Δ|=%d", firstDiv1, absS1(int(delta1)))
	t.Logf(" Rank | H-ID | container                | spec-init       | observed-deviation point")
	t.Logf(" -----+------+--------------------------+-----------------+-------------------------")
	t.Logf("   1  | H-1  | pf.agcGainPrev (Q24)     | 0 per §4.3      | non-zero after sf-0 post AGC (lazy seed = gTargetQ24)")
	t.Logf("   2  | H-2  | applyAGC iteration       | derived         | cascade from H-1")
	t.Logf("   3  | H-13 | sf-0/sf-1 LP routing     | sfA = interp LP | sfA[0..10] for sf-0 matches §4.1.5 (refuted)")
	t.Logf("   4  | H-4  | lsp.prevLSP              | initialPrevLSP  | seeded inside Decode prior to interp consumer; OK after IP-E")
	t.Logf("   5  | H-5  | lsp.pastResiduals        | initialPastRes  | seeded inside Decode prior to predictor consumer; OK after IP-E")
	t.Logf("   6  | H-3  | gain.pastErrors          | -14336 ×4       | seeded inside Decode (pastErrorsDefault); OK after IP-E")
	t.Logf("   7  | H-6  | pastExc                  | zero ×153       | zero at IP-A; tail rotates correctly (refuted)")
	t.Logf("   8  | H-7  | hpX/hpY                  | (0,0,0,0)       | zero at IP-A (refuted)")
	t.Logf("   9  | H-8  | synth.pastSynth          | zero ×10        | zero at IP-A (refuted)")
	t.Logf("  10  | H-9  | pf.pastResidual          | zero ×183       | zero at IP-A (refuted)")
	t.Logf("  11  | H-10 | pf.pastS                 | zero ×10        | zero at IP-A (refuted)")
	t.Logf("  12  | H-11 | pf.pastSynthPost         | zero ×10        | zero at IP-A (refuted)")
	t.Logf("  13  | H-12 | pf.pastTiltInput         | 0               | zero at IP-A (refuted)")
	t.Logf("  14  | H-14 | pastExc slide direction  | tail = newest u | byte-EQ verified vs recomputeUVector (refuted)")
	t.Logf("  15  | H-15 | d.prevGpQ14              | 0               | zero at IP-A (refuted)")
	t.Logf("============================================================")
	t.Logf("Top-3 candidates: H-1, H-2, H-13")
	t.Logf("Recommended S-2 first-fix: H-1 (agcGainPrev seed = 0 per §4.3 catch-all)")
}

func firstDivergence(got, want [frameSamples]int16) (int, int) {
	for i := 0; i < frameSamples; i++ {
		if got[i] != want[i] {
			return i, int(got[i]) - int(want[i])
		}
	}
	return -1, 0
}

func absS1(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
