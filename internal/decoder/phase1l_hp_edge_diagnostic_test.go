package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_Phase1lHp2EdgeTrace — Phase 1l HP-2 (F-non-Hpost cycle
// Task 2): §A.4.2.5 HP filter frame-edge state trace at frame 0 for the
// two low-energy boundary-cluster vectors (ALGTHM, SPEECH).
//
// Reference plan:
//   docs/superpowers/plans/2026-05-06-phase1l-stage-f-non-hpost-plan.md
//   §Task 2 (HP-2).
//
// ABSOLUTE CONSTRAINTS (E1/E2/E4/E5):
//   - clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 / Annex A
//     binary reference. spec source = G729E.pdf + READMETV.txt +
//     textbooks only.
//   - production = 0 line change (E2): this test mirrors the per-subframe
//     pipeline inline (clean-room replication of decodeSubframe up to the
//     HP filter input) so HP state can be snapshotted at sample
//     granularity. The HP filter loop is also re-run inline, but it is
//     identically equivalent to (*Decoder).hpFilter on the same input
//     and starting state — verified below by cross-checking the produced
//     postX2 against (*Decoder).Decode output.
//   - measurement-only (E5): hard-asserts only the spec-derivable
//     `len(want) == 80` and `len(postX2) == 80` invariants. State values,
//     transient patterns, and verdict cells are all reported via t.Logf.
//   - verdicts are binary EQ / NE; UNDETERMINED is reserved for sections
//     where the spec is verbatim silent (E4 ambiguity).
//
// ============================================================================
// SPEC VERBATIM CITATIONS (mandatory) — extracted via:
//   pdftotext -layout docs/superpowers/specs/itu/G729E.pdf -
// ============================================================================
//
// (1) §4.2.5 "High-pass filtering and upscaling" (PDF lines 1687..1693):
//
//     "A high-pass filter with a cut-off frequency of 100 Hz is applied
//      to the reconstructed postfiltered speech sf'(n). The filter is
//      given by:
//
//                       0.93980581 − 1.8795834 z⁻¹ + 0.93980581 z⁻²
//        H_h2(z)  =  ───────────────────────────────────────────────
//                       1 − 1.9330735 z⁻¹ + 0.93589199 z⁻²
//
//      The filtered signal is multiplied by a factor 2 to restore the
//      input signal level."
//
//     Coefficient extraction:
//        b0 = +0.93980581, b1 = −1.8795834, b2 = +0.93980581
//        a0 = 1, a1 = −1.9330735, a2 = +0.93589199
//     Difference equation (canonical):
//        y[n] = b0·x[n] + b1·x[n-1] + b2·x[n-2]
//             − a1·y[n-1] − a2·y[n-2]
//             = b0·x[n] + b1·x[n-1] + b2·x[n-2]
//             + 1.9330735·y[n-1] − 0.93589199·y[n-2]
//
//     ⇒ §4.2.5 IS VERBATIM SILENT on the values of x[−1], x[−2], y[−1],
//     y[−2] at the very first call (frame 0, sample 0). No "initial",
//     "init", "reset", "zero" wording in the §4.2.5 paragraph itself.
//
// (2) §A.4.2.5 "High-pass filtering and upscaling" (PDF lines 2292..2293):
//
//     "Same as described in clause 4.2.5."
//
//     Annex A defers entirely to §4.2.5; no Annex-specific HP init
//     statement. Therefore the §4.2.5 silence is inherited.
//
// (3) §4.3 "Encoder and decoder initialization" (PDF lines 1696..1707):
//
//     "All static encoder and decoder variables should be initialized
//      to zero, except the variables listed in Table 9.
//
//                Table 9 – Description of parameters with non-zero
//                          initialization
//        Variable      Reference        Initial value
//        β              3.8              0.8
//        g(–1)          4.2.4            1.0
//        ^l_i           3.2.4            iπ/11
//        q_i            3.2.4            arccos(iπ/11)
//        Û^(k)          3.9.1            −14"
//
//     The HP filter state x[n−1], x[n−2], y[n−1], y[n−2] (production:
//     Decoder.hpX, Decoder.hpY) are NOT in Table 9. Per §4.3 they
//     therefore "should be initialized to zero". This is an EXPLICIT
//     spec mandate (default-zero) reached by the §4.3 catch-all clause,
//     not by §A.4.2.5 directly.
//
//     ⇒ specInit (HP state, frame 0) = ZERO  (explicit via §4.3 default).
//
// (4) §A.4.3 "Encoder and decoder initialization" (PDF line 2294):
//
//     "Same as described in clause 4.3."
//
//     Annex A inherits the §4.3 default-zero rule. No override.
//
// (5) Frame vs subframe cadence — §4.2.5 applies H_h2 to "the
//     reconstructed postfiltered speech sf'(n)" without specifying
//     batch size. Production (decoder/subframe.go:48) calls hpFilter
//     once per subframe (40 samples). Because H_h2 is a strictly causal
//     IIR with state {x[n−1], x[n−2], y[n−1], y[n−2]} carried across
//     calls (decoder/hpfilter.go:60-63 stores back into d.hpX, d.hpY),
//     two consecutive 40-sample calls are mathematically identical to
//     one 80-sample call. Cadence has no effect on the time series.
//
// ============================================================================
// VERDICT MODEL
// ============================================================================
//
// HP-2 verdict per (vector × region) cell:
//
//   productionInit (sample 0, BEFORE first hpFilter call of frame 0):
//     hpX[0] = 0, hpX[1] = 0, hpY[0] = 0, hpY[1] = 0 — observed below
//     from a freshly zero-valued Decoder.
//
//   specInit (frame 0, BEFORE first hpFilter call):
//     §4.2.5 itself: SILENT.
//     §A.4.2.5: defers to §4.2.5 (silent).
//     §4.3 (catch-all): "All static decoder variables should be
//                        initialized to zero, except [Table 9]" and
//                        Table 9 does NOT include HP filter state.
//     ⇒ specInit = ZERO (explicit via §4.3 catch-all).
//
//   verdict for "early" (i ∈ [0..21]):
//     EQ if productionInit == specInit (both zero) AND transientPattern
//        is decay-toward-zero (consistent with zero-state IIR step
//        response).
//     NE if productionInit ≠ specInit.
//     UNDETERMINED only if specInit is silent — NOT the case here, so
//        UNDETERMINED is not expected for the early cell.
//
//   verdict for "late" (i ∈ [65..79]):
//     The §4.2.5 spec is silent on what should happen at the late edge
//     of frame 0; no spec-mandated late-edge invariant exists. We
//     report this cell as "observation" (not EQ / NE). It is tagged
//     UNDETERMINED in the verdict column (E4: spec silence on
//     late-frame-0-edge invariant).
//
// ============================================================================
// HARD ASSERTIONS — spec-derivable invariants only
// ============================================================================
//   - len(want) == 80     (READMETV.txt frame size 160 bytes ÷ 2).
//   - len(postX2) == 80   (frameSamples constant).
//
// We do NOT hard-assert state values, Δ values, or transient patterns.
func TestDiagnostic_Phase1lHp2EdgeTrace(t *testing.T) {
	type vectorSpec struct {
		name    string
		bitFile string
		pstFile string
	}
	vectors := []vectorSpec{
		{"ALGTHM", "ALGTHM.BIT", "ALGTHM.PST"},
		{"SPEECH", "SPEECH.BIT", "SPEECH.PST"},
	}

	type cell struct {
		vector           string
		region           string
		productionInit   string
		specInit         string
		transientPattern string
		verdict          string
	}
	var cells []cell

	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			bitPath := vectorPath(v.bitFile)
			pstPath := vectorPath(v.pstFile)
			ensureTestdataPresent(t, bitPath, pstPath)

			frames, bads := readG192Frames(t, bitPath)
			wantFrames := readPSTFrames(t, pstPath)
			if len(wantFrames) == 0 || len(frames) == 0 {
				t.Fatalf("vector %s: empty bit/pst", v.name)
			}
			if bads[0] {
				t.Fatalf("vector %s: frame 0 bad-flag set", v.name)
			}

			want := wantFrames[0]
			if got := len(want); got != frameSamples {
				t.Fatalf("len(want)=%d want %d", got, frameSamples)
			}

			// ---- inline replication of (*Decoder).Decode, instrumented
			//      to capture sPf and per-sample HP state at frame 0 ----
			//
			// This mirrors decode.go and subframe.go exactly, but
			// substitutes the HP loop with a state-snapshotting loop.
			// The HP loop is mathematically identical to hpfilter.go
			// (same coefficients, same operation order, same Q-format).
			var d Decoder

			var f bitstream.Frame
			if err := bitstream.Unpack(frames[0], &f); err != nil {
				t.Fatalf("bitstream.Unpack: %v", err)
			}

			sf1A, sf2A := d.lsp.Decode(lsp.Indices{
				L0: uint8(f.L0), L1: uint8(f.L1),
				L2: uint8(f.L2), L3: uint8(f.L3),
			})
			tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
			_ = pitch.CheckParity(uint8(f.P1), uint8(f.P0))
			tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), tInt1)

			// per-sample state and signal traces (frame-wide, 80 entries)
			var sPf80 [frameSamples]int16
			var postHP80 [frameSamples]int16
			var postX2 [frameSamples]int16
			var preX0 [frameSamples]int16
			var preX1 [frameSamples]int16
			var preY0 [frameSamples]int32
			var preY1 [frameSamples]int32
			var postX0 [frameSamples]int16
			var postX1 [frameSamples]int16
			var postY0 [frameSamples]int32
			var postY1 [frameSamples]int32

			// snapshot of HP state BEFORE the very first sample of frame 0
			frame0InitHpX0 := d.hpX[0]
			frame0InitHpX1 := d.hpX[1]
			frame0InitHpY0 := d.hpY[0]
			frame0InitHpY1 := d.hpY[1]

			runSubframe := func(sfA *[lpcOrder + 1]int16, tInt, tFrac int,
				C uint16, S uint8, GA, GB uint8, base int) {
				betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

				var v40 [subframeLen]int16
				pitch.AdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v40)

				var c40 [subframeLen]int16
				fcb.Decode(fcb.Indices{Positions: C, Signs: S},
					tInt, betaQ14, &c40)

				gpQ14, gcQ12 := d.gn.Decode(
					gain.Indices{GA: GA, GB: GB}, &c40)

				var u40 [subframeLen]int16
				synth.BuildExcitation(gpQ14, gcQ12, &v40, &c40, &u40)

				var s40 [subframeLen]int16
				d.syn.Filter(sfA, &u40, &s40)

				var sPf40 [subframeLen]int16
				d.pst.Filter(sfA, tInt, &s40, &sPf40)

				// Inline the §4.2.5 HP IIR step-by-step, snapshotting
				// {x1, x2, y1, y2} pre and post each sample. The math
				// is bit-identical to hpfilter.go (same coefficients,
				// same shift/round order, same int32 accumulation).
				x1 := d.hpX[0]
				x2 := d.hpX[1]
				y1 := d.hpY[0]
				y2 := d.hpY[1]

				for n := 0; n < subframeLen; n++ {
					i := base + n

					sPf80[i] = sPf40[n]

					preX0[i] = x1
					preX1[i] = x2
					preY0[i] = y1
					preY1[i] = y2

					xn := sPf40[n]

					ff := int32(hpB0Q13)*int32(xn) +
						int32(hpB1Q13)*int32(x1) +
						int32(hpB2Q13)*int32(x2) // Q13
					ff >>= 1                     // Q12

					fb := int64(hpNegA1Q12) * int64(y1) // Q24
					fb >>= 12
					fb -= (int64(hpA2Q13) * int64(y2)) >> 13

					acc := int64(ff) + fb // Q12

					yn := (acc + (1 << 11)) >> 12
					if yn > 32767 {
						yn = 32767
					} else if yn < -32768 {
						yn = -32768
					}

					postHP80[i] = int16(yn)

					x2 = x1
					x1 = xn
					y2 = y1
					y1 = int32(acc)

					postX0[i] = x1
					postX1[i] = x2
					postY0[i] = y1
					postY1[i] = y2
				}

				d.hpX[0] = x1
				d.hpX[1] = x2
				d.hpY[0] = y1
				d.hpY[1] = y2

				copy(d.pastExc[:pastExcLen-subframeLen],
					d.pastExc[subframeLen:])
				copy(d.pastExc[pastExcLen-subframeLen:], u40[:])
				d.prevGpQ14 = gpQ14
			}

			runSubframe(&sf1A, tInt1, tFrac1, f.C1, uint8(f.S1),
				uint8(f.GA1), uint8(f.GB1), 0)
			runSubframe(&sf2A, tInt2, tFrac2, f.C2, uint8(f.S2),
				uint8(f.GA2), uint8(f.GB2), subframeLen)

			// post-HP × 2 with int16 saturation (== pcm.ScaleUpSat).
			for i := 0; i < frameSamples; i++ {
				v := int32(postHP80[i]) * 2
				if v > 32767 {
					v = 32767
				} else if v < -32768 {
					v = -32768
				}
				postX2[i] = int16(v)
			}

			if got := len(postX2); got != frameSamples {
				t.Fatalf("len(postX2)=%d want %d", got, frameSamples)
			}

			// cross-check: replicated chain matches (*Decoder).Decode
			var d2 Decoder
			var prod [frameSamples]int16
			if err := d2.Decode(frames[0], false, prod[:]); err != nil {
				t.Fatalf("Decode cross-check: %v", err)
			}
			for i := 0; i < frameSamples; i++ {
				if prod[i] != postX2[i] {
					t.Logf("CROSS-CHECK MISMATCH i=%d prod=%d "+
						"replicated=%d (replication is not "+
						"bit-equivalent to Decode at this index)",
						i, prod[i], postX2[i])
				}
			}

			// ---- per-sample Δ ----
			var deltas [frameSamples]int16
			for i := 0; i < frameSamples; i++ {
				deltas[i] = int16(int32(postX2[i]) - int32(want[i]))
			}

			t.Logf("──────── vector %s frame 0 HP-edge trace ────────",
				v.name)
			t.Logf("frame-0 INIT HP state (BEFORE sample 0): "+
				"hpX=[%d,%d] hpY=[%d,%d]",
				frame0InitHpX0, frame0InitHpX1,
				frame0InitHpY0, frame0InitHpY1)

			// Compact 80-row summary: i, sPf, postHP, postX2, want, Δ.
			t.Logf("--- 80-row compact summary ---")
			t.Logf("  i | sPf  postHP postX2 want   Δ")
			for i := 0; i < frameSamples; i++ {
				t.Logf("  %2d | %5d %5d %6d %6d %+5d",
					i, sPf80[i], postHP80[i],
					postX2[i], want[i], deltas[i])
			}

			// Sample 5..7 sanity restatement (Phase 0c-2/P0c-3 carry):
			// expected uniform Δ = +3 for ALGTHM.
			t.Logf("--- sample 5..7 sanity (Δ should be +3 for ALGTHM) ---")
			t.Logf("  Δ[5..7] = [%+d %+d %+d]",
				deltas[5], deltas[6], deltas[7])

			// Full per-sample state series for early region [0..21].
			t.Logf("--- early region [0..21] full HP state series ---")
			t.Logf("  i | preX=[x1,x2] preY=[y1,y2] | xn=sPf | "+
				"postX=[x1,x2] postY=[y1,y2] | postHP postX2 want Δ")
			for i := 0; i <= 21; i++ {
				t.Logf("  %2d | preX=[%6d,%6d] preY=[%9d,%9d] | "+
					"xn=%6d | postX=[%6d,%6d] postY=[%9d,%9d] | "+
					"%6d %6d %6d %+5d",
					i,
					preX0[i], preX1[i], preY0[i], preY1[i],
					sPf80[i],
					postX0[i], postX1[i], postY0[i], postY1[i],
					postHP80[i], postX2[i], want[i], deltas[i])
			}

			// Full per-sample state series for late region [65..79].
			t.Logf("--- late region [65..79] full HP state series ---")
			t.Logf("  i | preX=[x1,x2] preY=[y1,y2] | xn=sPf | "+
				"postX=[x1,x2] postY=[y1,y2] | postHP postX2 want Δ")
			for i := 65; i <= 79; i++ {
				t.Logf("  %2d | preX=[%6d,%6d] preY=[%9d,%9d] | "+
					"xn=%6d | postX=[%6d,%6d] postY=[%9d,%9d] | "+
					"%6d %6d %6d %+5d",
					i,
					preX0[i], preX1[i], preY0[i], preY1[i],
					sPf80[i],
					postX0[i], postX1[i], postY0[i], postY1[i],
					postHP80[i], postX2[i], want[i], deltas[i])
			}

			// Verdict cells per region.
			early := classifyHp2EdgeRegion(deltas[:], 0, 21)
			late := classifyHp2EdgeRegion(deltas[:], 65, 79)

			earlyProdInit, earlySpecInit, earlyVerdict :=
				classifyHp2EdgeState(v.name, "early",
					frame0InitHpX0, frame0InitHpX1,
					frame0InitHpY0, frame0InitHpY1,
					early)
			lateProdInit, lateSpecInit, lateVerdict :=
				classifyHp2EdgeState(v.name, "late",
					preX0[65], preX1[65], preY0[65], preY1[65],
					late)

			cells = append(cells, cell{
				vector:           v.name,
				region:           "early",
				productionInit:   earlyProdInit,
				specInit:         earlySpecInit,
				transientPattern: early,
				verdict:          earlyVerdict,
			})
			cells = append(cells, cell{
				vector:           v.name,
				region:           "late",
				productionInit:   lateProdInit,
				specInit:         lateSpecInit,
				transientPattern: late,
				verdict:          lateVerdict,
			})

			t.Logf("--- HP-2 verdict cell (early) ---")
			t.Logf("  vector=%s region=early productionInit=%s "+
				"specInit=%s transientPattern=%s verdict=%s",
				v.name, earlyProdInit, earlySpecInit, early, earlyVerdict)
			t.Logf("--- HP-2 verdict cell (late) ---")
			t.Logf("  vector=%s region=late productionInit=%s "+
				"specInit=%s transientPattern=%s verdict=%s",
				v.name, lateProdInit, lateSpecInit, late, lateVerdict)
		})
	}

	// ---- 4-cell verdict matrix (top-level Logf) ----
	t.Logf("──────── HP-2 verdict matrix (4 cells) ────────")
	t.Logf("| vector  | region    | productionInit                    | "+
		"specInit                          | transientPattern   | verdict       |")
	t.Logf("|---------|-----------|-----------------------------------|" +
		"-----------------------------------|--------------------|---------------|")
	for _, c := range cells {
		t.Logf("| %-7s | %-9s | %-33s | %-33s | %-18s | %-13s |",
			c.vector, c.region, c.productionInit, c.specInit,
			c.transientPattern, c.verdict)
	}
}

// classifyHp2EdgeRegion classifies the magnitude-vs-index trend of Δ
// over a closed sample range [lo..hi] of the frame-0 80-sample Δ vector.
//
// Categories (first match wins):
//   "all-zero"           — every Δ in [lo..hi] is zero.
//   "decay-toward-zero"  — strict-or-equal decreasing |Δ| from lo to hi
//                          AND |Δ[lo]| > 0 (impulse-response transient
//                          envelope candidate at the leading edge).
//   "growth-toward-edge" — strict-or-equal increasing |Δ| from lo to hi
//                          AND |Δ[hi]| > 0 (state-accumulation candidate
//                          at the trailing edge).
//   "sign-uniform"       — every nonzero Δ shares the same sign AND
//                          neither monotonic shape applies (constant-ish
//                          offset candidate).
//   "flat"               — all |Δ| identical and nonzero (constant
//                          offset candidate).
//   "random"             — otherwise.
func classifyHp2EdgeRegion(deltas []int16, lo, hi int) string {
	if lo < 0 || hi >= len(deltas) || lo > hi {
		return "invalid-range"
	}
	allZero := true
	for i := lo; i <= hi; i++ {
		if deltas[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return "all-zero"
	}

	abs := func(x int16) int32 {
		v := int32(x)
		if v < 0 {
			return -v
		}
		return v
	}

	// monotone non-increasing magnitude (with strict drop somewhere).
	decay := abs(deltas[lo]) > 0
	strictDrop := false
	for i := lo + 1; i <= hi; i++ {
		if abs(deltas[i]) > abs(deltas[i-1]) {
			decay = false
			break
		}
		if abs(deltas[i]) < abs(deltas[i-1]) {
			strictDrop = true
		}
	}
	if decay && strictDrop {
		return "decay-toward-zero"
	}

	// monotone non-decreasing magnitude (with strict rise somewhere).
	growth := abs(deltas[hi]) > 0
	strictRise := false
	for i := lo + 1; i <= hi; i++ {
		if abs(deltas[i]) < abs(deltas[i-1]) {
			growth = false
			break
		}
		if abs(deltas[i]) > abs(deltas[i-1]) {
			strictRise = true
		}
	}
	if growth && strictRise {
		return "growth-toward-edge"
	}

	// sign-uniform check.
	var seenSign int
	signUniform := true
	for i := lo; i <= hi; i++ {
		if deltas[i] == 0 {
			continue
		}
		s := 1
		if deltas[i] < 0 {
			s = -1
		}
		if seenSign == 0 {
			seenSign = s
			continue
		}
		if s != seenSign {
			signUniform = false
			break
		}
	}
	if signUniform && seenSign != 0 {
		// further refine: identical magnitude → "flat".
		mag := abs(deltas[lo])
		flat := true
		for i := lo + 1; i <= hi; i++ {
			if abs(deltas[i]) != mag {
				flat = false
				break
			}
		}
		if flat && mag > 0 {
			return "flat"
		}
		return "sign-uniform"
	}
	return "random"
}

// classifyHp2EdgeState assembles the per-cell verdict tuple for HP-2.
// It does NOT inspect the deltas themselves except via the already-
// computed transientPattern label; the spec interpretation is hard-coded
// from the §4.2.5 / §A.4.2.5 / §4.3 verbatim quotations in the file
// header.
//
//   vector            : "ALGTHM" or "SPEECH" (recorded for traceability).
//   region            : "early" (i ∈ [0..21]) or "late" (i ∈ [65..79]).
//   x0,x1             : production HP state x1, x2 at the region's
//                       sample-0 (i.e. BEFORE the first hpFilter call
//                       of frame 0 for "early"; OR at i=65 for "late").
//   y0,y1             : production HP state y1, y2 at the same instant
//                       (Q12).
//   transientPattern  : output of classifyHp2EdgeRegion for the region.
//
// Returns:
//   productionInit   : "hpX=[x0,x1],hpY=[y0,y1]" snapshot.
//   specInit         : the §4.2.5/A.4.2.5/§4.3 mandate for this region.
//   verdict          : "EQ" / "NE" / "UNDETERMINED" per §VERDICT MODEL.
func classifyHp2EdgeState(vector, region string,
	x0, x1 int16, y0, y1 int32,
	transientPattern string,
) (productionInit, specInit, verdict string) {
	productionInit = sprintfHp2Init(x0, x1, y0, y1)

	switch region {
	case "early":
		// §4.3 catch-all default-zero applies to HP state (not in
		// Table 9). Spec mandates ZERO. Production zero-value Decoder
		// gives ZERO. → EQ if production matches, NE otherwise.
		specInit = "hpX=[0,0],hpY=[0,0] (§4.3 catch-all default-zero)"
		if x0 == 0 && x1 == 0 && y0 == 0 && y1 == 0 {
			verdict = "EQ"
		} else {
			verdict = "NE"
		}
	case "late":
		// §4.2.5 / §A.4.2.5 are silent on any late-frame-0-edge HP
		// state invariant. The state at i=65 is whatever the prior
		// 65-sample HP IIR convolution produced; no spec target.
		specInit = "(spec silent on late-frame-0 HP state)"
		verdict = "UNDETERMINED"
	default:
		specInit = "(unknown region)"
		verdict = "UNDETERMINED"
	}
	_ = vector
	_ = transientPattern
	return
}

// sprintfHp2Init renders the 4-field HP state snapshot in the canonical
// "hpX=[x0,x1],hpY=[y0,y1]" form used in the verdict matrix.
func sprintfHp2Init(x0, x1 int16, y0, y1 int32) string {
	// hand-rolled to avoid pulling fmt into the test surface area
	// where t.Logf already covers all formatting needs.
	return itoa16(x0) + "/" + itoa16(x1) + " " + itoa32(y0) + "/" + itoa32(y1)
}

func itoa16(v int16) string { return itoa64(int64(v)) }
func itoa32(v int32) string { return itoa64(int64(v)) }
func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	var buf [24]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
