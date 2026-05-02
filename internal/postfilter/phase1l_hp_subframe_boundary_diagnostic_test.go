package postfilter

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_Phase1lHp1SubframeBoundaryTrace — Phase 1l HP-1 (F-non-Hpost
// cycle Task 1) postfilter sub-state carryover/reset diagnostic at the
// subframe-1 → subframe-2 boundary (sample 39 → sample 40) of frame 0.
//
// Reference plan:
//   docs/superpowers/plans/2026-05-06-phase1l-stage-f-non-hpost-plan.md
//   §Task 1 (HP-1).
//
// ABSOLUTE CONSTRAINTS (E1/E2/E4/E5):
//   - clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 / Annex A
//     binary reference. spec source = G729E.pdf + READMETV.txt + textbooks.
//   - production = 0 line change (E2): test only mirrors the per-subframe
//     pipeline; nothing in postfilter.go / decoder/* is modified.
//   - measurement-only (E5): hard-asserts only the spec-derivable
//     `len(produced) == 80` invariant. Sub-state values, deltas, and
//     verdicts are reported via t.Logf only.
//   - verdicts are binary EQ/NE; UNDETERMINED is reserved for sections
//     where the spec is verbatim silent on the carryover/reset policy
//     (E4 ambiguity).
//
// ============================================================================
// SPEC VERBATIM CITATIONS (mandatory) — extracted via:
//   pdftotext -layout docs/superpowers/specs/itu/G729E.pdf -
//
// (1) §4.2.1 "Long-term postfilter" (PDF lines 1565..1625):
//
//     "The long-term postfilter is given by:
//        H_p(z) = 1 / (1 + γ_p g_l) · (1 + γ_p g_l z^{-T})
//      where T is the pitch delay, and g_l is the gain coefficient. ...
//      The long-term delay and gain are computed from the residual signal
//      r̂(n) obtained by filtering the speech ŝ(n) through Â(z/γ_n) ..."
//
//     §4.2.1 spells out the H_p TRANSFER FUNCTION and how T, g_l are
//     RECOMPUTED per subframe from r̂(n), but is verbatim SILENT on
//     whether the residual buffer r̂(·) (production: pf.pastResidual)
//     is carried over or zeroed at the sf-1 → sf-2 boundary. No
//     "reset", "initialize", "clear" wording appears in §4.2.1.
//     ⇒ specPolicy(Hp) = UNDETERMINED (E4 ambiguity).
//
// (2) §4.2.2 "Short-term postfilter" (PDF lines 1626..1645):
//
//     "The short-term postfilter is given by:
//        H_f(z) = (1/g_f) · Â(z/γ_n)/Â(z/γ_d)
//      where Â(z) is the received quantized LP inverse filter ...
//      γ_n = 0.55, γ_d = 0.7. The gain term g_f is calculated on the
//      truncated impulse response h_f(n) of the filter
//      Â(z/γ_n)/Â(z/γ_d) ..."
//
//     §4.2.2 defines H_f as an IIR cascade. The IIR memory rows
//     (production: pf.pastS for Â(z/γ_n) numerator history,
//     pf.pastSynthPost for Â(z/γ_d) denominator history) are NOT
//     mentioned with respect to subframe-boundary handling. No
//     "reset between subframes" wording appears.
//     ⇒ specPolicy(Hf-pastS, Hf-pastSynthPost) = UNDETERMINED.
//
// (3) §4.2.3 "Tilt compensation" (PDF lines 1646..1668):
//
//     "The filter H_t(z) compensates for the tilt in the short-term
//      postfilter H_f(z) and is given by:
//        H_t(z) = (1/g_t) · (1 + γ_t k1' z^{-1})
//      where γ_t k1' is a tilt factor k1' being the first reflection
//      coefficient calculated from h_f(n) ... Two values for γ_t are
//      used depending on the sign of k1'. If k1' is negative, γ_t = 0.9,
//      and if k1' is positive, γ_t = 0.2."
//
//     The tilt filter contains a single z^{-1} delay, whose state in
//     production is pf.pastTiltInput. §4.2.3 does NOT describe whether
//     this delay is carried over or zeroed at the subframe boundary.
//     ⇒ specPolicy(γ_t past tilt input) = UNDETERMINED.
//
// (4) §4.2.4 "Adaptive gain control" (PDF lines 1669..1686) — this is
//     the ONLY postfilter sub-state for which the spec gives an explicit
//     verbatim subframe-boundary carryover statement:
//
//     "g(n) = 0.85 g(n−1) + 0.15 G   n = 0,...,39
//      The initial value of g(–1) = 1.0 is used. Then for each new
//      subframe, g(–1) is set equal to g(39) of the previous subframe."
//
//     => specPolicy(AGC g(n−1)) = CARRYOVER (verbatim mandated).
//
// (5) Annex A simplifications (PDF lines 2236..2293):
//
//     §A.4.2.1: "The only difference from clause 4.2.1 is that the
//                long-term delay T is always an integer delay and it
//                is computed by searching the range [Tcl – 3, Tcl + 3]."
//     §A.4.2.2: "The only difference from clause 4.2.2 is that the gain
//                factor g_f is eliminated."
//     §A.4.2.3: "...The value of γ_t = 0.8 is used if k1' < 0 and γ_t
//                is set to zero if k1' ≥ 0. The gain factor g_t which
//                is used in clause 4.2.3 is eliminated."
//     §A.4.2.4: "The same as described in clause 4.2.4, with the only
//                difference being that the gain scaling factor G for
//                the present subframe is computed by [sum of squares
//                form] ... and g(n) is given by:
//                g(n) = 0.9 g(n−1) + 0.1 G,  n = 0,...,39"
//
//     Annex A modifies coefficients / search ranges but DOES NOT
//     introduce any subframe-boundary reset language for Hp / Hf /
//     γ_t state. §A.4.2.4 inherits the §4.2.4 carryover statement
//     ("the same as described in clause 4.2.4") for AGC state — only
//     the smoothing constants and the energy form change.
//     ⇒ specPolicy carries over unchanged from §4.2.x to §A.4.2.x.
//
// (6) §4.3 "Encoder and decoder initialization" (PDF lines 1695..1708):
//
//     "All static encoder and decoder variables should be initialized
//      to zero, except the variables listed in Table 9.   ...   g(–1)
//      reference §4.2.4 initial value 1.0."
//
//     §4.3 governs FRAME-0 / call-start initialization, NOT the
//     subframe boundary. It is cited here to confirm that there is
//     no spec hook for "reset between subframes". (Production
//     initialized=false → seeds agcGainPrev from g_target on the very
//     first applyAGC call, which is the §A.4.2.4 first-call init.)
//
// ============================================================================
// EXPECTED-POLICY MATRIX (4 sub-states × {spec verbatim})
// ============================================================================
//
//   sub-state              | spec verbatim section          | specPolicy
//   -----------------------+--------------------------------+--------------
//   Hp (pastResidual)      | §4.2.1 / §A.4.2.1              | UNDETERMINED
//   Hf-pastS               | §4.2.2 / §A.4.2.2              | UNDETERMINED
//   Hf-pastSynthPost       | §4.2.2 / §A.4.2.2              | UNDETERMINED
//   γ_t (pastTiltInput)    | §4.2.3 / §A.4.2.3              | UNDETERMINED
//   AGC (agcGainPrev)      | §4.2.4 / §A.4.2.4              | CARRYOVER
//
// ============================================================================
// SUBFRAME-BOUNDARY SNAPSHOT TIMING (per plan §Task 1, R-B)
// ============================================================================
//
//   A — state captured RIGHT AFTER pf.Filter() returns for sf-1
//       (= "right after sample 39 processed, before sf-2 begins").
//   B — state captured JUST BEFORE pf.Filter() is invoked for sf-2
//       (= "just before sample 40 processed"). In production this
//       runs back-to-back with no intervening mutator, so B == A
//       trivially unless a code path between subframes touches pf.
//   C — state captured RIGHT AFTER pf.Filter() returns for sf-2
//       (= "right after sample 79 processed"). Used to verify that
//       the state actually advanced (B → C should be non-trivial).
//
//   In Postfilter.Filter() (postfilter.go), the per-call state
//   mutations happen at well-defined points within the chain:
//     - pf.pastResidual: slid + tail-written near the top of Filter()
//       (BEFORE refinePitch). Carries new r(·) into the buffer.
//     - pf.pastS:        updated inside applyShortTerm (numerator IIR
//                        history rolls forward).
//     - pf.pastSynthPost:updated inside applyShortTerm (denominator
//                        IIR history rolls forward).
//     - pf.pastTiltInput:updated inside applyTiltWithMu (single z^{-1}
//                        delay).
//     - pf.agcGainPrev / pf.initialized: updated inside applyAGC
//                        (first-call seeds agcGainPrev = g_target_Q24,
//                        sets initialized = true; subsequent calls
//                        smooth toward G).
//   None of the state fields are touched outside Filter() in the
//   postfilter package, so the A == B identity is by construction.
//   The diagnostic still measures A and B independently to make any
//   future regression visible.
//
// ============================================================================
// VERDICT CLASSIFIER
// ============================================================================
//
//   ΔAB = B − A   (elementwise for vectors, scalar for γ_t/AGC)
//
//   productionPolicy =
//     "carryover"  if ΔAB == 0 across every element of the sub-state
//     "reset"      if B is all-zero AND A had ≥1 nonzero entry
//     "partial"    otherwise (some elements changed, not all to zero)
//
//   verdict =
//     "EQ"            if specPolicy is concrete AND productionPolicy
//                     matches it.
//     "NE"            if specPolicy is concrete AND productionPolicy
//                     differs.
//     "UNDETERMINED"  if specPolicy = UNDETERMINED (§4.2.x silent on
//                     subframe-boundary policy; E4 ambiguity).
//
// ============================================================================
// HARD ASSERTIONS (spec-derivable invariants only)
// ============================================================================
//   - len(produced) == 80 per vector frame (sf-1 sPf || sf-2 sPf).
//   No sub-state values or verdicts are hard-asserted.
//
// ============================================================================
// R-D HIGH-ENERGY CHECK
// ============================================================================
//   Frame-0 max|sPf| is logged for ALGTHM/FIXED/PITCH so the
//   "FIXED + PITCH = high-energy interior [40..64] Δ" precondition
//   from the plan's cross-vector evidence (P0c-3) can be validated
//   here at frame 0 (i.e., it is not silence at frame 0).
func TestDiagnostic_Phase1lHp1SubframeBoundaryTrace(t *testing.T) {
	type vectorSpec struct {
		name    string
		bitFile string
	}
	vectors := []vectorSpec{
		{"ALGTHM", "ALGTHM.BIT"},
		{"FIXED", "FIXED.BIT"},
		{"PITCH", "PITCH.BIT"},
	}

	type cellResult struct {
		vector           string
		substate         string
		productionPolicy string
		specPolicy       string
		verdict          string
	}
	var matrix []cellResult

	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			bitPath := vectorPath(v.bitFile)
			ensureTestdataPresent(t, bitPath)

			frames, bads := readG192Frames(t, bitPath)
			if len(frames) == 0 {
				t.Fatalf("vector %s: empty bitstream", v.name)
			}
			if bads[0] {
				t.Fatalf("vector %s: frame 0 bad-flag set; cannot proceed", v.name)
			}

			var f bitstream.Frame
			if err := bitstream.Unpack(frames[0], &f); err != nil {
				t.Fatalf("vector %s: Unpack frame 0: %v", v.name, err)
			}

			// Per-stream state (mirrors decoder.Decoder fields touched
			// by frame-0 decoding; no decoder package import to avoid
			// import cycle since this test lives in postfilter).
			var lspDec lsp.Decoder
			lspDec.Reset()
			var gnDec gain.Decoder
			gnDec.Reset()
			var syn synth.Synthesizer
			syn.Reset()
			var pf Postfilter // zero value = §4.3 / §A.4.3 init

			const pastExcLen = pitchMax + 10 // 153, per pitch.AdaptiveCodebook contract
			var pastExc [pastExcLen]int16
			var prevGpQ14 int16

			sf1A, sf2A := lspDec.Decode(lsp.Indices{
				L0: uint8(f.L0), L1: uint8(f.L1),
				L2: uint8(f.L2), L3: uint8(f.L3),
			})

			tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
			_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
			tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

			runSubframe := func(
				sfA *[lpcOrder + 1]int16,
				tInt, tFrac int,
				C uint16, S uint8,
				GA, GB uint8,
				out *[subframeLen]int16,
			) {
				betaQ14 := fcb.ClampPitchGainForEnhancement(prevGpQ14)

				var v40 [subframeLen]int16
				pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v40)

				var c [subframeLen]int16
				fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

				gpQ14, gcQ12 := gnDec.Decode(gain.Indices{GA: GA, GB: GB}, &c)

				var u [subframeLen]int16
				synth.BuildExcitation(gpQ14, gcQ12, &v40, &c, &u)

				var s [subframeLen]int16
				syn.Filter(sfA, &u, &s)

				pf.Filter(sfA, tInt, &s, out)

				copy(pastExc[:pastExcLen-subframeLen], pastExc[subframeLen:])
				copy(pastExc[pastExcLen-subframeLen:], u[:])
				prevGpQ14 = gpQ14
			}

			// ── Snapshot helpers ────────────────────────────────────
			type snapshot struct {
				pastResidual  [pitchMax + subframeLen]int16
				pastS         [lpcOrder]int16
				pastSynthPost [lpcOrder]int16
				pastTiltInput int16
				agcGainPrev   int32
				initialized   bool
			}
			snap := func() snapshot {
				return snapshot{
					pastResidual:  pf.pastResidual,
					pastS:         pf.pastS,
					pastSynthPost: pf.pastSynthPost,
					pastTiltInput: pf.pastTiltInput,
					agcGainPrev:   pf.agcGainPrev,
					initialized:   pf.initialized,
				}
			}

			// ── sf-1: process samples 0..39 ─────────────────────────
			var sPf1 [subframeLen]int16
			runSubframe(&sf1A, tInt1, tFrac1,
				f.C1, uint8(f.S1), uint8(f.GA1), uint8(f.GB1),
				&sPf1)
			snapA := snap() // A: end of sf-1 (right after sample 39)

			// ── B: start of sf-2 (just before sample 40) ───────────
			// In production, no postfilter mutator runs between sf-1
			// completion and sf-2 entry, so this is the same instant
			// as A. We snapshot independently to make any future drift
			// visible.
			snapB := snap()

			// ── sf-2: process samples 40..79 ────────────────────────
			var sPf2 [subframeLen]int16
			runSubframe(&sf2A, tInt2, tFrac2,
				f.C2, uint8(f.S2), uint8(f.GA2), uint8(f.GB2),
				&sPf2)
			snapC := snap() // C: end of sf-2 (right after sample 79)

			// Concatenate produced samples (postfilter chain output,
			// pre-HP, pre-×2). This is the diagnostic's notion of
			// "produced": it is 80 samples by construction.
			var produced [80]int16
			copy(produced[:subframeLen], sPf1[:])
			copy(produced[subframeLen:], sPf2[:])

			// ── Hard assertion: spec-derivable invariant only ──────
			if len(produced) != 80 {
				t.Fatalf("vector %s: produced length %d, want 80",
					v.name, len(produced))
			}

			// ── R-D high-energy / silence check ────────────────────
			maxAbs := int32(0)
			for _, x := range produced {
				a := int32(x)
				if a < 0 {
					a = -a
				}
				if a > maxAbs {
					maxAbs = a
				}
			}
			t.Logf("[R-D] vector %s frame-0 max|sPf| = %d (postfilter chain output, pre-HP/×2)",
				v.name, maxAbs)

			// ── ΔAB classification per sub-state ───────────────────
			classify := func(allZeroB, equalAB, hadNonzeroA bool) string {
				switch {
				case equalAB:
					return "carryover"
				case allZeroB && hadNonzeroA:
					return "reset"
				default:
					return "partial"
				}
			}

			// Hp (pastResidual)
			var (
				hpEqual    = true
				hpAllZeroB = true
				hpAnyA     = false
			)
			for i := 0; i < pitchMax+subframeLen; i++ {
				if snapA.pastResidual[i] != snapB.pastResidual[i] {
					hpEqual = false
				}
				if snapB.pastResidual[i] != 0 {
					hpAllZeroB = false
				}
				if snapA.pastResidual[i] != 0 {
					hpAnyA = true
				}
			}

			// Hf-pastS
			var (
				pSEqual    = true
				pSAllZeroB = true
				pSAnyA     = false
			)
			for i := 0; i < lpcOrder; i++ {
				if snapA.pastS[i] != snapB.pastS[i] {
					pSEqual = false
				}
				if snapB.pastS[i] != 0 {
					pSAllZeroB = false
				}
				if snapA.pastS[i] != 0 {
					pSAnyA = true
				}
			}

			// Hf-pastSynthPost
			var (
				pSPEqual    = true
				pSPAllZeroB = true
				pSPAnyA     = false
			)
			for i := 0; i < lpcOrder; i++ {
				if snapA.pastSynthPost[i] != snapB.pastSynthPost[i] {
					pSPEqual = false
				}
				if snapB.pastSynthPost[i] != 0 {
					pSPAllZeroB = false
				}
				if snapA.pastSynthPost[i] != 0 {
					pSPAnyA = true
				}
			}

			// γ_t (pastTiltInput) — scalar
			tiltEqual := snapA.pastTiltInput == snapB.pastTiltInput
			tiltAllZeroB := snapB.pastTiltInput == 0
			tiltAnyA := snapA.pastTiltInput != 0

			// AGC (agcGainPrev + initialized) — composite
			agcEqual := snapA.agcGainPrev == snapB.agcGainPrev &&
				snapA.initialized == snapB.initialized
			agcAllZeroB := snapB.agcGainPrev == 0 && !snapB.initialized
			agcAnyA := snapA.agcGainPrev != 0 || snapA.initialized

			subResults := []struct {
				label      string
				prodPolicy string
				specPolicy string
			}{
				{"Hp(pastResidual)", classify(hpAllZeroB, hpEqual, hpAnyA), "UNDETERMINED"},
				{"Hf(pastS)", classify(pSAllZeroB, pSEqual, pSAnyA), "UNDETERMINED"},
				{"Hf(pastSynthPost)", classify(pSPAllZeroB, pSPEqual, pSPAnyA), "UNDETERMINED"},
				{"γ_t(pastTiltInput)", classify(tiltAllZeroB, tiltEqual, tiltAnyA), "UNDETERMINED"},
				{"AGC(agcGainPrev)", classify(agcAllZeroB, agcEqual, agcAnyA), "CARRYOVER"},
			}

			t.Logf("──────── HP-1 snapshot summary  vector=%s ────────", v.name)
			t.Logf("A (end-sf-1): pastTiltInput=%d agcGainPrev=%d (Q24) initialized=%v",
				snapA.pastTiltInput, snapA.agcGainPrev, snapA.initialized)
			t.Logf("B (start-sf-2): pastTiltInput=%d agcGainPrev=%d (Q24) initialized=%v",
				snapB.pastTiltInput, snapB.agcGainPrev, snapB.initialized)
			t.Logf("C (end-sf-2): pastTiltInput=%d agcGainPrev=%d (Q24) initialized=%v",
				snapC.pastTiltInput, snapC.agcGainPrev, snapC.initialized)
			t.Logf("A.pastS         = %v", snapA.pastS)
			t.Logf("B.pastS         = %v", snapB.pastS)
			t.Logf("C.pastS         = %v", snapC.pastS)
			t.Logf("A.pastSynthPost = %v", snapA.pastSynthPost)
			t.Logf("B.pastSynthPost = %v", snapB.pastSynthPost)
			t.Logf("C.pastSynthPost = %v", snapC.pastSynthPost)
			// Compact pastResidual digest: the tail (last subframeLen
			// entries) is what was just written for sf-1; the head
			// (first pitchMax entries) is the slid history.
			t.Logf("A.pastResidual tail[%d..%d] = %v",
				pitchMax, pitchMax+subframeLen-1,
				snapA.pastResidual[pitchMax:pitchMax+subframeLen])
			t.Logf("B.pastResidual tail[%d..%d] = %v",
				pitchMax, pitchMax+subframeLen-1,
				snapB.pastResidual[pitchMax:pitchMax+subframeLen])
			t.Logf("C.pastResidual tail[%d..%d] = %v",
				pitchMax, pitchMax+subframeLen-1,
				snapC.pastResidual[pitchMax:pitchMax+subframeLen])

			// classifyHp1State — required helper signature per plan.
			classifyHp1State := func(_ string, substate string) (string, string, string) {
				for _, sr := range subResults {
					if sr.label == substate {
						return sr.prodPolicy, sr.specPolicy, verdictOf(sr.prodPolicy, sr.specPolicy)
					}
				}
				return "?", "?", "UNDETERMINED"
			}

			t.Logf("──────── HP-1 verdict matrix  vector=%s ────────", v.name)
			t.Logf("  %-22s | %-10s | %-13s | %-12s",
				"sub-state", "production", "spec", "verdict")
			for _, sr := range subResults {
				prod, spec, verd := classifyHp1State(v.name, sr.label)
				t.Logf("  %-22s | %-10s | %-13s | %-12s",
					sr.label, prod, spec, verd)
				matrix = append(matrix, cellResult{
					vector:           v.name,
					substate:         sr.label,
					productionPolicy: prod,
					specPolicy:       spec,
					verdict:          verd,
				})
			}

			// Defensive: A → C must NOT be a no-op (state must advance
			// across sf-2 processing). Logged only.
			if snapA.pastTiltInput == snapC.pastTiltInput &&
				snapA.agcGainPrev == snapC.agcGainPrev &&
				snapA.pastS == snapC.pastS &&
				snapA.pastSynthPost == snapC.pastSynthPost {
				t.Logf("[note] vector %s: A == C across all measured "+
					"sub-states; sf-2 processing left state unchanged "+
					"(possible silent / zero-input subframe).", v.name)
			}
		})
	}

	// Aggregate matrix dump (3 vectors × 5 sub-states).
	t.Logf("════════ HP-1 aggregate verdict matrix (3 × 5) ════════")
	t.Logf("  %-7s | %-22s | %-10s | %-13s | %-12s",
		"vector", "sub-state", "production", "spec", "verdict")
	neCount := 0
	for _, c := range matrix {
		t.Logf("  %-7s | %-22s | %-10s | %-13s | %-12s",
			c.vector, c.substate, c.productionPolicy, c.specPolicy, c.verdict)
		if c.verdict == "NE" {
			neCount++
		}
	}
	t.Logf("HP-1 NE-count = %d (escape hatch: ≥1 NE → (Hpost-state-defect) "+
		"evidence; 0 NE with no UNDETERMINED → HP-1 폐기 → Task 2)", neCount)
}

// verdictOf maps (production, spec) policy pair to a binary EQ/NE verdict
// (or UNDETERMINED when the spec section is verbatim silent).
func verdictOf(prod, spec string) string {
	switch spec {
	case "CARRYOVER":
		if prod == "carryover" {
			return "EQ"
		}
		return "NE"
	case "RESET":
		if prod == "reset" {
			return "EQ"
		}
		return "NE"
	case "UNDETERMINED":
		return "UNDETERMINED"
	default:
		return "UNDETERMINED"
	}
}
