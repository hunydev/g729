package synth

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
)

// Phase 1k Stage F-non-Cgamma-revisit-2 (Task 2, G-2) — synth IIR memory
// + Y magnitude trace.
//
// Plan: docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-cgamma-revisit-plan.md
// (commit c743116, Phase 2 §Task 2).
//
// 측정 의무 (1줄): §4.1.6 IIR memory pre/post sample 5..7 EQ/NE +
// Y magnitude +6 perturbation 적용 시 syn[5..7] 부호 EQ/NE.
//
// 외부 G.729 구현 0 참조 (E1/E4): spec source = ITU-T G.729 (06/2012)
// PDF + READMETV.txt only. Annex A binary 사용 0 (G1).
//
// Production 변경 0 라인 (E2/E5). assertion 0 (측정-only); promotion =
// Task 3 synthesis 결정 후.
//
// Spec 인용 (PDF verbatim, ITU-T G.729 (06/2012)):
//
//   §3.10 / §4.1.6 LP synthesis filter:
//     "ŝ(n) = u(n) − Σ_{i=1..10} a_i · ŝ(n−i)"
//     direct-form IIR with mem_syn = ŝ(n-1..n-10).
//   §4.3 Table 9: codec-start initial mem_syn = 0.
//
// Sub-test A (TestDiagnostic_FnonCgammaRevisit2SynthIIRMemoryTrace):
//   §4.1.6 synthesis IIR filter memory at ALGTHM frame 0 sf0 sample
//   5..7. mem_syn[0..9] pre-state (entering sample 5) + post-state
//   after each of sample 5, 6, 7. Reference = inline replay using
//   fixed primitives + identical a[0..10]. EQ vs reference per state.
//
// Sub-test B (TestDiagnostic_FnonCgammaRevisit2YMagnitudePerturbationTrace):
//   a[1..10] +6 magnitude perturbation (sign 보존, magnitude only;
//   F-sept-2 max|Δa|=6 jitter scope). a[0]=4096 (Q12 unity) 보존.
//   §4.1.6 재실행. baseline syn[5..7] = [+1,+1,+1] (F-non-prelim-1).
//   perturbed syn[5..7] sign EQ vs baseline → magnitude refute,
//   NE → magnitude mechanism 후보.

// TestDiagnostic_FnonCgammaRevisit2SynthIIRMemoryTrace — Sub-test A.
func TestDiagnostic_FnonCgammaRevisit2SynthIIRMemoryTrace(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// (1) Reproduce ALGTHM frame 0 sf0 canonical inputs (§3.7/§3.8/§3.9
	// + §4.3 zero-init).  Same path as F-oct-postfix2-prelim-4 / -M3.
	var lspDec lsp.Decoder
	lspDec.Reset()
	a, _ := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
	const pastExcLen = 153
	var pastExc [pastExcLen]int16
	var v [40]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [40]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)
	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	var u [40]int16
	BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

	t.Logf("──────── G-2 sub-test A fixture (ALGTHM frame 0 sf0) ────────")
	t.Logf("LP a[0..10] (Q12, a[0]=4096) = %v", a)
	t.Logf("g_p (Q14)=%+d   g_c (Q12)=%+d", gpQ14, gcQ12)
	t.Logf("v[0..7] (adaptive cb) = %v", [8]int16{v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7]})
	t.Logf("c[0..7] (fixed cb, Q13) = %v", [8]int16{c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7]})
	t.Logf("u[0..7] (excitation, Q0) = %v", [8]int16{u[0], u[1], u[2], u[3], u[4], u[5], u[6], u[7]})
	t.Logf("PST want sample 5..7 = [%+d %+d %+d] (signs=[%s %s %s])",
		wantFrames[0][5], wantFrames[0][6], wantFrames[0][7],
		signOfInt16(wantFrames[0][5]), signOfInt16(wantFrames[0][6]), signOfInt16(wantFrames[0][7]))

	// (2) Production Synthesizer.Filter — captures pastSynth at codec-start
	// (= all zero per §4.3 Table 9) and produces s[0..7].
	var syProd Synthesizer
	syProd.Reset()
	preStateProd := syProd.pastSynth // pre sample-0 state; for sample-5 pre we replay below.
	var sProd [40]int16
	fixed.ClearOverflow()
	syProd.Filter(&a, &u, &sProd)
	prodOverflow := fixed.Overflow()
	t.Logf("Synthesizer.Reset pastSynth (codec-start, §4.3 Table 9) = %v", preStateProd)
	t.Logf("Synthesizer.Filter syn[0..7] (production)  = %v",
		[8]int16{sProd[0], sProd[1], sProd[2], sProd[3], sProd[4], sProd[5], sProd[6], sProd[7]})
	t.Logf("post-Filter overflow = %v", prodOverflow)

	// (3) Reference inline replay using fixed primitives — direct
	// implementation of §3.10:  ŝ(n) = u(n) − Σ a_i ŝ(n-i).
	// Identical to filter.go onePass (replayed for measurement; this
	// is the spec-derived expected mem_syn).
	var work [50]int16 // work[0..9] = pastSynth, work[10..49] = output.
	// work[0..9] = 0 per §4.3 Table 9.
	var memDumps [4][10]int16 // pre-sample-5, post-sample-5, post-sample-6, post-sample-7.

	captureMem := func(dst *[10]int16, n int) {
		// At time n, mem_syn = ŝ(n-1..n-10). For n=5 (pre): mem[0]=ŝ(4)=work[14], ..., mem[9]=ŝ(-5)=work[5].
		// dst[i] = ŝ(n-1-i) = work[10+n-1-i].  For pre-sample-5 with n=5:
		// dst[0]=work[14], dst[9]=work[5].
		for i := 0; i < 10; i++ {
			dst[i] = work[10+n-1-i]
		}
	}

	fixed.ClearOverflow()
	// Run samples 0..4 first to establish pre-sample-5 state.
	for n := 0; n < 5; n++ {
		lTemp := fixed.LMult(u[n], a[0])
		for i := 1; i <= 10; i++ {
			lTemp = fixed.LMsu(lTemp, a[i], work[10+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[10+n] = fixed.Round(lTemp)
	}
	captureMem(&memDumps[0], 5) // pre-state entering sample 5.
	// Now run samples 5, 6, 7 capturing post-state after each.
	for n := 5; n <= 7; n++ {
		lTemp := fixed.LMult(u[n], a[0])
		for i := 1; i <= 10; i++ {
			lTemp = fixed.LMsu(lTemp, a[i], work[10+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[10+n] = fixed.Round(lTemp)
		captureMem(&memDumps[n-4], n+1) // post-state after sample n = mem entering sample n+1.
	}
	refOverflow := fixed.Overflow()
	refSyn := [8]int16{
		work[10], work[11], work[12], work[13],
		work[14], work[15], work[16], work[17],
	}
	t.Logf("──────── §4.1.6 reference inline replay (spec-derived mem_syn) ────────")
	t.Logf("reference syn[0..7]    = %v", refSyn)
	t.Logf("reference overflow flag = %v   (Pass-1 only; spec §3.10 direct form)", refOverflow)
	t.Logf("[mem pre-sample-5]   ŝ(4..-5) = %v", memDumps[0])
	t.Logf("[mem post-sample-5]  ŝ(5..-4) = %v", memDumps[1])
	t.Logf("[mem post-sample-6]  ŝ(6..-3) = %v", memDumps[2])
	t.Logf("[mem post-sample-7]  ŝ(7..-2) = %v", memDumps[3])

	// (4) Production-side mem_syn dump derived from sProd (since
	// production Synthesizer does not expose intermediate mem state):
	// mem at time n = previous 10 outputs.  For n=5 pre-state we need
	// sProd[-5..4]; sProd[-5..-1] = preStateProd[5..9] (codec-start
	// zero), sProd[0..4] from production output.
	prodPrev := func(n int) int16 {
		// returns ŝ(n) where n ∈ [-10, 7]; n<0 maps to preStateProd[10+n].
		if n < 0 {
			return preStateProd[10+n]
		}
		return sProd[n]
	}
	prodMem := func(n int) [10]int16 {
		var m [10]int16
		for i := 0; i < 10; i++ {
			m[i] = prodPrev(n - 1 - i)
		}
		return m
	}
	prodPreSample5 := prodMem(5)
	prodPostSample5 := prodMem(6)
	prodPostSample6 := prodMem(7)
	prodPostSample7 := prodMem(8)
	t.Logf("──────── production-derived mem_syn (Synthesizer.Filter outputs) ────────")
	t.Logf("[mem pre-sample-5]   prod = %v", prodPreSample5)
	t.Logf("[mem post-sample-5]  prod = %v", prodPostSample5)
	t.Logf("[mem post-sample-6]  prod = %v", prodPostSample6)
	t.Logf("[mem post-sample-7]  prod = %v", prodPostSample7)

	// (5) Sub-stage EQ/NE verdict per state (production vs reference).
	//
	// EQ = production mem_syn[0..9] == reference mem_syn[0..9] sample-
	// exact (both computed with same a[], u[], pastSynth — Pass-1 path
	// is deterministic so any divergence = production deviation from
	// §3.10 direct form).
	type memState struct {
		label string
		prod  [10]int16
		ref   [10]int16
	}
	states := []memState{
		{"pre-sample-5  (entering n=5)", prodPreSample5, memDumps[0]},
		{"post-sample-5 (entering n=6)", prodPostSample5, memDumps[1]},
		{"post-sample-6 (entering n=7)", prodPostSample6, memDumps[2]},
		{"post-sample-7 (entering n=8)", prodPostSample7, memDumps[3]},
	}
	t.Logf("──────── G-2 sub-stage A EQ/NE verdict per state ────────")
	allEQ := true
	for _, st := range states {
		v := classifyCgammaSynthSubStage(st.prod, st.ref)
		t.Logf("  %s  verdict=%s", st.label, v)
		if v != "EQ" {
			allEQ = false
		}
	}
	t.Logf("[Sub-test A overall] %s",
		map[bool]string{true: "EQ_ALL — synth IIR memory propagation spec 정합 (G-2-IIR 폐기)",
			false: "NE_AT_LEAST_ONE — synth IIR memory deviation 식별 (G-2-IIR mechanism 후보)"}[allEQ])
}

// TestDiagnostic_FnonCgammaRevisit2YMagnitudePerturbationTrace — Sub-test B.
func TestDiagnostic_FnonCgammaRevisit2YMagnitudePerturbationTrace(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	var lspDec lsp.Decoder
	lspDec.Reset()
	a, _ := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
	const pastExcLen = 153
	var pastExc [pastExcLen]int16
	var v [40]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [40]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)
	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	var u [40]int16
	BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

	t.Logf("──────── G-2 sub-test B fixture (ALGTHM frame 0 sf0) ────────")
	t.Logf("baseline a[0..10] (Q12) = %v", a)
	t.Logf("baseline u[0..7] = %v", [8]int16{u[0], u[1], u[2], u[3], u[4], u[5], u[6], u[7]})

	// (1) Baseline syn — matches F-non-prelim-1 measurement [+1,+1,+1].
	var syBase Synthesizer
	syBase.Reset()
	var sBase [40]int16
	fixed.ClearOverflow()
	syBase.Filter(&a, &u, &sBase)
	baseOverflow := fixed.Overflow()
	t.Logf("baseline syn[0..7] = %v   overflow=%v",
		[8]int16{sBase[0], sBase[1], sBase[2], sBase[3], sBase[4], sBase[5], sBase[6], sBase[7]},
		baseOverflow)
	t.Logf("baseline syn[5..7] signs = [%s %s %s]   (F-non-prelim-1 reference [+1,+1,+1])",
		signOfInt16(sBase[5]), signOfInt16(sBase[6]), signOfInt16(sBase[7]))
	t.Logf("PST want sample 5..7 signs = [%s %s %s]",
		signOfInt16(wantFrames[0][5]), signOfInt16(wantFrames[0][6]), signOfInt16(wantFrames[0][7]))

	// (2) Y magnitude perturbation: a[1..10] += 6 along sign (sign-
	// preserving magnitude bump). a[0] = 4096 (Q12 unity) preserved.
	// F-sept-2 L3 jitter scope: max|Δa| = 6.
	var aPert [11]int16
	aPert[0] = a[0]
	for i := 1; i <= 10; i++ {
		switch {
		case a[i] > 0:
			x := int32(a[i]) + 6
			if x > 32767 {
				x = 32767
			}
			aPert[i] = int16(x)
		case a[i] < 0:
			x := int32(a[i]) - 6
			if x < -32768 {
				x = -32768
			}
			aPert[i] = int16(x)
		default:
			// a[i] == 0: no defined sign; leave unchanged (smallest
			// magnitude perturbation that preserves the zero sign-class).
			aPert[i] = 0
		}
	}
	t.Logf("perturbed a[0..10]  = %v   (|Δ|=6 sign-preserving on a[1..10]; a[0] preserved)", aPert)

	// (3) Re-run synthesis with same excitation u[] and codec-start state.
	var syPert Synthesizer
	syPert.Reset()
	var sPert [40]int16
	fixed.ClearOverflow()
	syPert.Filter(&aPert, &u, &sPert)
	pertOverflow := fixed.Overflow()
	t.Logf("perturbed syn[0..7] = %v   overflow=%v",
		[8]int16{sPert[0], sPert[1], sPert[2], sPert[3], sPert[4], sPert[5], sPert[6], sPert[7]},
		pertOverflow)
	t.Logf("perturbed syn[5..7] signs = [%s %s %s]",
		signOfInt16(sPert[5]), signOfInt16(sPert[6]), signOfInt16(sPert[7]))

	// (4) Sub-stage verdict: perturbed syn[5..7] sign EQ vs baseline.
	verdict := classifyCgammaYMagSubStage(
		[3]int16{sBase[5], sBase[6], sBase[7]},
		[3]int16{sPert[5], sPert[6], sPert[7]},
	)
	t.Logf("──────── G-2 sub-stage B verdict ────────")
	t.Logf("baseline  signs = [%s %s %s]",
		signOfInt16(sBase[5]), signOfInt16(sBase[6]), signOfInt16(sBase[7]))
	t.Logf("perturbed signs = [%s %s %s]",
		signOfInt16(sPert[5]), signOfInt16(sPert[6]), signOfInt16(sPert[7]))
	t.Logf("verdict = %s", verdict)
	switch verdict {
	case "EQ":
		t.Logf("[Sub-test B] EQ — Y magnitude +6 perturbation 은 syn[5..7] 부호 변화 유발 X. G-2-Y-mag 폐기 (magnitude 단독은 부호 메커니즘 아님).")
	case "NE":
		t.Logf("[Sub-test B] NE — Y magnitude +6 perturbation 이 syn[5..7] 부호 변화 유발. G-2-Y-mag mechanism 후보 식별.")
	default:
		t.Logf("[Sub-test B] %s — degenerate (zero) sample 포함; spec 와 모순 아님.", verdict)
	}
}

// classifyCgammaSynthSubStage returns the binary EQ/NE verdict for a
// single mem_syn[0..9] state: production vs spec-derived reference.
//
// EQ = sample-exact equality (Pass-1 §3.10 direct form is deterministic;
//      any divergence = production deviation from spec).
// NE = ≥1 sample differs.
//
// Phase 0.4 강압-적합 회피: no "approximately matches" / "within tolerance".
func classifyCgammaSynthSubStage(prod, ref [10]int16) string {
	if prod == ref {
		return "EQ"
	}
	return "NE"
}

// classifyCgammaYMagSubStage returns the binary EQ/NE verdict comparing
// perturbed syn[5..7] sign tuple vs baseline sign tuple.
//
// EQ           — all 3 signs match baseline (magnitude perturbation
//                does NOT flip output sign; G-2-Y-mag refute).
// NE           — ≥1 sign flipped (magnitude perturbation IS a sign
//                mechanism candidate).
// INCONCLUSIVE — ≥1 sample is 0 in either tuple (degenerate; sign
//                undefined; spec polarity not contradicted).
func classifyCgammaYMagSubStage(baseline, perturbed [3]int16) string {
	hasZero := false
	hasFlip := false
	for i := 0; i < 3; i++ {
		sb := signOfInt16(baseline[i])
		sp := signOfInt16(perturbed[i])
		if sb == "0" || sp == "0" {
			hasZero = true
			continue
		}
		if sb != sp {
			hasFlip = true
		}
	}
	switch {
	case hasFlip:
		return "NE"
	case hasZero:
		return "INCONCLUSIVE"
	default:
		return "EQ"
	}
}
