package decoder

// PHASE 1o D-3.ter DISPOSITION — KEEP-WITH-NOTE.
//
// The hypothesis investigated by this diagnostic file has been closed by
// the gate 17 PSTdomain demotion (Phase 1o D-1b, commit 6633b28) and/or
// the Phase 1o D-3 state-bearing root-cause cycle (commits aa27ad1,
// 0428df7, bd37512, da089b5, be80eaf, c81645b — closure c81645b/this-cycle).
// Retained as evidence-trail and a verification-path demonstrator that
// future Phase-2 encoder cross-reference work may want to re-walk; do NOT
// extend this file — open a new dated diagnostic file instead. See
// session-state checkpoints 011..020 for the gate 17 / 28-cycle history,
// and docs/superpowers/plans/2026-05-09-phase1o-decoder-domain-closure-plan.md
// §3 D-3.ter for the housekeeping decision rationale.

import (
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pcm"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/postfilter"
	"github.com/hunydev/g729/internal/synth"
)

// TestDiagnostic_Phase0c2WantStageTrace — Phase 0c-2 (P0c-2) want-stage
// chain identification (`docs/.../2026-05-05-phase0c-reentry-want-domain
// -reinterpret-plan.md` §Task 2).
//
// Goal: identify which chain stage S* (syn / sPf / postHP / postX2)
// best matches ALGTHM.PST frame 0 want[0..79] under (argmin Σ|Δ|,
// max sign-match). Current production assumes S* = postX2 (post-AGC +
// HP + ×2 = §A.4.2.5 chain end). gate 17 RED on sample 5..7 leaves
// open the possibility S* ≠ postX2.
//
// ABSOLUTE CONSTRAINTS (E0/E2/E5):
//   - clean-room MIT: no ITU C / bcg729 / Sipro Lab / FFmpeg / Annex A
//     binary reference; only the cited spec PDF + READMETV.txt + public
//     textbooks (Kondoz, Spanias).
//   - production = 0 line change (E2): test replicates the per-subframe
//     chain inline calling production functions only.
//   - measurement-only: hard-asserts only spec-derivable invariants
//     (slice lengths, no-saturation precondition); S* identification
//     surfaces via t.Logf verdict.
//
// ============================================================================
// SPEC VERBATIM CITATIONS (mandatory — Task 2 §195–202)
// ============================================================================
//
// (1) READMETV.txt (g729AnnexA test_vectors tree) — silent on chain
//
//	stage of *.pst:
//
//	   "decoder file.bit file.pst"
//
//	README only declares *.pst as the decoder's *output*; it does NOT
//	state whether the output is post-postfilter, post-HP, or post-×2.
//	The chain-stage interpretation must be derived from the PDF.
//
// (2) PDF §4.2 — postfilter definition (long-term + short-term + tilt
//
//   - AGC). Section header + key sentences:
//
//     "4.2 Post-processing"
//     "Post-processing consists of three functions: adaptive
//     postfiltering, high-pass filtering and signal up-scaling.
//     The adaptive postfilter is the cascade of three filters:
//     a long-term postfilter Hp(z), a short-term postfilter Hf(z),
//     and a tilt compensation filter Ht(z), followed by an
//     adaptive gain control procedure."
//
//     => sPf = §4.2 adaptive postfilter cascade output (Hp ∘ Hf ∘ Ht
//     ∘ AGC). For Annex A this collapses to the simplified cascade
//     described in §A.4.2 but the chain-output role of sPf is
//     unchanged.
//
// (3) PDF §A.4.2.5 — post-processing (HP filter + ×2 multiplier).
//
//	Verbatim as cited in plan §0:
//
//	   "A.4.2.5 Post-processing
//	    Same as described in clause 4.2.5."
//
//	§4.2.5 contains the operative sentence:
//
//	   "The filtered signal is multiplied by a factor 2 to restore
//	    the input signal level."
//
//	Q-format unspecified in spec text (UNDETERMINED per E4); we
//	observe that production pcm.ScaleUpSat applies ×2 with int16
//	saturation at frame end.
//
// (4) PDF §4.1.6 — synthesis IIR producing syn (= postfilter input).
//
//	   "4.1.6 Computation of the reconstructed speech
//	    The reconstructed speech is computed by filtering the
//	    excitation u(n) through the LP synthesis filter
//	        1 / Â(z)
//	    The reconstructed speech for the subframe of size L=40 is
//	    given by
//	        ŝ(n) = u(n) − Σ_{i=1..10} â_i · ŝ(n-i),  n=0..39."
//
//	=> syn = §4.1.6 ŝ(n) for 80 samples (two subframes).
//
// ============================================================================
// CHAIN MAPPING (production → stage label)
// ============================================================================
//
//	syn    = synth.Synthesizer.Filter output           (§4.1.6)
//	sPf    = postfilter.Postfilter.Filter output       (§4.2 / §A.4.2)
//	postHP = decoder.hpFilter output                   (§A.4.2.5 step 1)
//	postX2 = pcm.ScaleUpSat(postHP)                    (§A.4.2.5 step 2)
//	       = current production PST candidate.
//
// ============================================================================
// CONVENTIONS
// ============================================================================
//
// sign(0) is treated as "matches any" — i.e. signMatch counts a sample
// as matching whenever stage[i] == 0 OR want[i] == 0 OR they share
// strict sign. This avoids penalizing genuinely-zero samples in either
// domain (the §0.6 measurement at sample 5..7 already established
// strict-sign disagreement only for nonzero want values).
//
// sumAbsDiff sums |int32(stage[i]) − int32(want[i])| across i=0..79
// with int32 to avoid int16 overflow on intermediate subtractions.
//
// Δ pattern classifier:
//
//	"zero"                     — all stage[i] == want[i].
//	"sample-uniform-constant"  — non-zero, all (stage[i]−want[i]) equal.
//	"sign-uniform"             — non-constant Δ, but every nonzero
//	                             stage[i] has same sign as want[i].
//	"random"                   — otherwise.
//
// ============================================================================
// HARD ASSERTIONS — spec-derivable invariants only
// ============================================================================
//   - len(want)=len(syn)=len(sPf)=len(postHP)=len(postX2) = 80.
//   - |postX2[i]| < INT16_MAX for all i (no ×2 saturation in this
//     frame). Required so postHP/postX2 ratio remains the spec-defined
//     bijective ×2; if violated, option A inline replication is the
//     only safe path (we use option A regardless).
//
// We do NOT hard-assert S* = postX2 — the verdict is logged only.
func TestDiagnostic_Phase0c2WantStageTrace(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	// ------------------------------------------------------------
	// Replicate frame 0 decode chain inline (option A, see file
	// header). Production code path mirrored from decoder.Decode +
	// decoder.decodeSubframe. Each subframe contributes 40 samples
	// to syn/sPf/postHP/postX2; concatenate to length 80 to align
	// with PST want frame layout.
	// ------------------------------------------------------------
	if bads[0] {
		t.Fatalf("frame 0 bad-flag set; cannot proceed (Phase 0c chain "+
			"trace requires clean frame). bads[0]=%v", bads[0])
	}

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	var lspDec lsp.Decoder
	lspDec.Reset()
	sf1A, sf2A := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

	var (
		syn    [frameSamples]int16
		sPf    [frameSamples]int16
		postHP [frameSamples]int16
		postX2 [frameSamples]int16
	)

	var (
		pastExc   [pastExcLen]int16
		gnDec     gain.Decoder
		synthSt   synth.Synthesizer
		pst       postfilter.Postfilter
		hpX       [2]int16
		hpY       [2]int32
		prevGpQ14 int16
	)
	gnDec.Reset()

	type sfSpec struct {
		sfA       *[lpcOrder + 1]int16
		tInt      int
		tFrac     int
		C         uint16
		S         uint8
		GA, GB    uint8
		outOffset int
	}
	subframes := [2]sfSpec{
		{&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1), 0},
		{&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2), subframeLen},
	}

	for _, sf := range subframes {
		betaQ14 := fcb.ClampPitchGainForEnhancement(prevGpQ14)

		var v [subframeLen]int16
		pitch.AdaptiveCodebook(sf.tInt, sf.tFrac, pastExc[:], &v)

		var c [subframeLen]int16
		fcb.Decode(fcb.Indices{Positions: sf.C, Signs: sf.S}, sf.tInt, betaQ14, &c)

		gpQ14, gcMant_gcQ12, gcExp_gcQ12 := gnDec.Decode(gain.Indices{GA: sf.GA, GB: sf.GB}, &c)

		var u [subframeLen]int16
		synth.BuildExcitation(gpQ14, gcMant_gcQ12, gcExp_gcQ12, &v, &c, &u)

		var s [subframeLen]int16
		synthSt.Filter(sf.sfA, &u, &s)
		copy(syn[sf.outOffset:sf.outOffset+subframeLen], s[:])

		var sp [subframeLen]int16
		pst.Filter(sf.sfA, sf.tInt, &s, &sp)
		copy(sPf[sf.outOffset:sf.outOffset+subframeLen], sp[:])

		// HP filter — replicate decoder.hpFilter inline (its receiver
		// is *Decoder so we cannot call it here without constructing
		// a Decoder, which would also run the chain). Use the same
		// arithmetic with local hpX/hpY state.
		var hp [subframeLen]int16
		hpFilterStateless(&sp, &hp, &hpX, &hpY)
		copy(postHP[sf.outOffset:sf.outOffset+subframeLen], hp[:])

		var x2 [subframeLen]int16
		pcm.ScaleUpSat(hp[:], x2[:])
		copy(postX2[sf.outOffset:sf.outOffset+subframeLen], x2[:])

		copy(pastExc[:pastExcLen-subframeLen], pastExc[subframeLen:])
		copy(pastExc[pastExcLen-subframeLen:], u[:])
		prevGpQ14 = gpQ14
	}

	want := wantFrames[0]

	// ------------------------------------------------------------
	// Hard assertions (spec-derivable invariants).
	// ------------------------------------------------------------
	if got := len(want); got != frameSamples {
		t.Fatalf("len(want) = %d, want %d", got, frameSamples)
	}
	if len(syn) != frameSamples || len(sPf) != frameSamples ||
		len(postHP) != frameSamples || len(postX2) != frameSamples {
		t.Fatalf("stage slice length mismatch: syn=%d sPf=%d postHP=%d postX2=%d (want all %d)",
			len(syn), len(sPf), len(postHP), len(postX2), frameSamples)
	}
	for i := 0; i < frameSamples; i++ {
		if postX2[i] == 32767 || postX2[i] == -32768 {
			// Saturation candidate; check whether postHP*2 actually
			// saturated (postHP would be > 16383 or < -16384).
			if postHP[i] > 16383 || postHP[i] < -16384 {
				t.Fatalf("postX2[%d]=%d saturated (postHP[%d]=%d, |postHP|>16383): "+
					"frame 0 saturation breaks ×2 bijectivity precondition",
					i, postX2[i], i, postHP[i])
			}
		}
	}

	// ------------------------------------------------------------
	// Per-stage classifier.
	// ------------------------------------------------------------
	stages := []struct {
		name string
		v    [frameSamples]int16
	}{
		{"syn   ", syn},
		{"sPf   ", sPf},
		{"postHP", postHP},
		{"postX2", postX2},
	}

	stats := make([]wantStageStat, len(stages))
	for si := range stages {
		s := &stats[si]
		s.name = stages[si].name
		stage := stages[si].v
		s.signMatch = computeSignMatchWantStage(&stage, &want)
		s.sumAbsDiff = computeSumAbsDiffWantStage(&stage, &want)
		s.deltaPat = classifyDeltaPatternWantStage(&stage, &want)
	}

	star, verdict := classifyWantStage(stats)

	// ------------------------------------------------------------
	// t.Logf dump: per-stage matrix (compact: differ-from-want only),
	// per-stage stats, verdict.
	// ------------------------------------------------------------
	t.Logf("──────── Phase 0c-2 want-stage trace (ALGTHM frame 0, 80 samples) ────────")
	t.Logf("PST want sample 5..7 = [%+d %+d %+d]   (sanity: prior measurement [-1,-1,-1])",
		want[5], want[6], want[7])
	for _, st := range stages {
		t.Logf("  [%s] sample 5..7 = [%+6d %+6d %+6d]   signs=[%s %s %s]",
			st.name, st.v[5], st.v[6], st.v[7],
			signOfInt16(st.v[5]), signOfInt16(st.v[6]), signOfInt16(st.v[7]))
	}

	t.Logf("──────── per-stage statistics (80-sample) ────────")
	for _, s := range stats {
		t.Logf("  [%s] signMatch=%2d/80  sumAbsDiff=%d  ΔPattern=%s",
			s.name, s.signMatch, s.sumAbsDiff, s.deltaPat)
	}

	t.Logf("──────── compact differ-from-want index dump per stage ────────")
	for _, st := range stages {
		var diffIdx []int
		for i := 0; i < frameSamples; i++ {
			if st.v[i] != want[i] {
				diffIdx = append(diffIdx, i)
			}
		}
		t.Logf("  [%s] differ-from-want indices (%d): %v",
			st.name, len(diffIdx), diffIdx)
	}

	t.Logf("──────── full 4×80 stage matrix (sample i: syn / sPf / postHP / postX2 / want) ────────")
	for i := 0; i < frameSamples; i++ {
		t.Logf("  i=%2d  syn=%+7d  sPf=%+7d  postHP=%+7d  postX2=%+7d  want=%+7d",
			i, syn[i], sPf[i], postHP[i], postX2[i], want[i])
	}

	t.Logf("──────── verdict ────────")
	t.Logf("  S* (argmin sumAbsDiff, sign-mismatch tie-break) = %s", star)
	t.Logf("  signMatch[postX2] = %d/80 (escape-hatch threshold: ≥ 78)", stats[3].signMatch)
	t.Logf("  Phase 0c-2 verdict: %s", verdict)
}

// ----------------------------------------------------------------------------
// Diagnostic helpers (test-only, suffix "WantStage" to avoid colliding with
// existing stagef_* helpers; package-level so the test reads compactly).
// ----------------------------------------------------------------------------

// computeSignMatchWantStage — sign(0) matches anything; otherwise
// strict sign equality between stage[i] and want[i].
func computeSignMatchWantStage(stage, want *[frameSamples]int16) int {
	n := 0
	for i := 0; i < frameSamples; i++ {
		s := stage[i]
		w := want[i]
		if s == 0 || w == 0 {
			n++
			continue
		}
		if (s > 0 && w > 0) || (s < 0 && w < 0) {
			n++
		}
	}
	return n
}

func computeSumAbsDiffWantStage(stage, want *[frameSamples]int16) int64 {
	var sum int64
	for i := 0; i < frameSamples; i++ {
		d := int32(stage[i]) - int32(want[i])
		if d < 0 {
			d = -d
		}
		sum += int64(d)
	}
	return sum
}

// classifyDeltaPatternWantStage — see file header for category defs.
func classifyDeltaPatternWantStage(stage, want *[frameSamples]int16) string {
	allEqual := true
	for i := 0; i < frameSamples; i++ {
		if stage[i] != want[i] {
			allEqual = false
			break
		}
	}
	if allEqual {
		return "zero"
	}
	first := int32(stage[0]) - int32(want[0])
	uniformConst := true
	for i := 0; i < frameSamples; i++ {
		if int32(stage[i])-int32(want[i]) != first {
			uniformConst = false
			break
		}
	}
	if uniformConst {
		return "sample-uniform-constant"
	}
	signUniform := true
	for i := 0; i < frameSamples; i++ {
		s := stage[i]
		w := want[i]
		if s == 0 || w == 0 {
			continue
		}
		if (s > 0) != (w > 0) {
			signUniform = false
			break
		}
	}
	if signUniform {
		return "sign-uniform"
	}
	return "random"
}

// wantStageStat — per-stage classifier record (test-only, package
// scope so classifyWantStage can be defined non-generically).
type wantStageStat struct {
	name       string
	signMatch  int
	sumAbsDiff int64
	deltaPat   string
}

// classifyWantStage — argmin sumAbsDiff (tie-break: max signMatch);
// returns verdict EQ if S* = postX2 AND signMatch[postX2] >= 78, else NE.
func classifyWantStage(stats []wantStageStat) (string, string) {
	bestIdx := 0
	for i := 1; i < len(stats); i++ {
		if stats[i].sumAbsDiff < stats[bestIdx].sumAbsDiff {
			bestIdx = i
			continue
		}
		if stats[i].sumAbsDiff == stats[bestIdx].sumAbsDiff &&
			stats[i].signMatch > stats[bestIdx].signMatch {
			bestIdx = i
		}
	}
	star := stats[bestIdx].name
	// postX2 is index 3 in the caller's stage list (syn/sPf/postHP/postX2).
	const postX2Idx = 3
	if bestIdx == postX2Idx && stats[postX2Idx].signMatch >= 78 {
		return star, "EQ (S* = postX2 AND signMatch[postX2] ≥ 78 — current spec assumption holds)"
	}
	return star,
		"NE (S* ≠ postX2 OR signMatch[postX2] < 78 — spec assumption challenged; user gate required per plan §Task 2 escape hatch)"
}

// hpFilterStateless — inline replica of decoder.hpFilter using
// caller-supplied state. Bit-exact reproduction of decoder/hpfilter.go
// (constants and arithmetic identical) so syn/sPf/postHP/postX2 chain
// matches production semantics without instantiating a *Decoder (which
// would also re-run the chain end-to-end and emit only postX2 via Decode).
//
// E2 invariant: this is a TEST-ONLY duplicate; production hpFilter
// remains unchanged. If production hpFilter constants ever drift, the
// chain-trace verdict here would diverge — guard rail is the ALGTHM
// decode_test.go regression which covers the production path.
func hpFilterStateless(in, out *[subframeLen]int16, hpX *[2]int16, hpY *[2]int32) {
	x1 := hpX[0]
	x2 := hpX[1]
	y1 := hpY[0]
	y2 := hpY[1]
	for n := 0; n < subframeLen; n++ {
		xn := in[n]
		ff := int32(hpB0Q13)*int32(xn) +
			int32(hpB1Q13)*int32(x1) +
			int32(hpB2Q13)*int32(x2)
		ff >>= 1
		fb := int64(hpNegA1Q12) * int64(y1)
		fb >>= 12
		fb -= (int64(hpA2Q13) * int64(y2)) >> 13
		acc := int64(ff) + fb
		yn := (acc + (1 << 11)) >> 12
		if yn > 32767 {
			yn = 32767
		} else if yn < -32768 {
			yn = -32768
		}
		out[n] = int16(yn)
		x2 = x1
		x1 = xn
		y2 = y1
		y1 = int32(acc)
	}
	hpX[0] = x1
	hpX[1] = x2
	hpY[0] = y1
	hpY[1] = y2
}
