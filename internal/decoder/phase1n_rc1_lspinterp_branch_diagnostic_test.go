package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/lsp"
)

// TestDiagnostic_Phase1nRc1LSPInterpBranchALGTHM — Phase 1n
// R-C-empirical Task RC-1: sf-1 LSP interpolation rounding-mode
// branch-test on ALGTHM frame 0 sample 5..7.
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-08-phase1n-stage-r-c-empirical-plan.md
//	§Task RC-1.
//
// CYCLE CONTEXT (cumulative, 26 prior diagnostic cycles):
//   - 25 sub-hypotheses refuted, defect = 0.
//   - 4 hard-spec invariants confirmed verbatim:
//     I-1 §4.2.4 AGC carryover (HP-1); I-2 §4.3 catch-all zero-init
//     (HP-2); I-3 §A.4.2.5 IIR pole-pair impulse decay (HP-2);
//     I-4 §3.2.4 LSP init formulas (CE-3 / 5232411).
//   - 3 R-blocking ambiguities inventoried (21894d3): R-A (§3.9.3
//     reorder map values verbatim absent), R-B (§3.8.2 sign/position
//     bit-string layout verbatim absent), R-C (§3.2.5 eq. (24) sf-1
//     rounding mode unspecified).
//   - Gate 17 `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`
//     SKIP'd at 9ab1c91 with R-C empirical disposition listed as a
//     reactivation trigger.
//
// CE-3 (5232411) FINDING (already-established LSP-domain measurement):
// on ALGTHM frame 0, two of the ten sf-1 LSP cells fall in the
// odd-half-sum regime where floor `>> 1` and round-half-away-from-zero
// disagree by exactly 1 LSB:
//
//	i=1: floor=31954, sym=31955  (sum = +63909, odd, sum >= 0)
//	i=5: floor= 7351, sym= 7352  (sum = +14703, odd, sum >= 0)
//
// All other 8 cells are byte-EQ between modes (sum even, or sign of
// half-sum makes both rounding modes coincide). RC-1 measures whether
// this 2-cell +1 LSB drift propagates through §3.2.6 Chebyshev
// expansion → a[1..10] LP coefficients → §3.10 synth.Filter → sample
// 5..7 sign at the §3.10/§4 decoder output.
//
// ABSOLUTE CONSTRAINTS (E1/E2'/E3/E4/E5):
//   - Clean-room MIT (E1): no ITU C / bcg729 / Sipro / FFmpeg / Annex
//     A binary reference. Symmetric-round = round-half-away-from-zero
//     per general numerical analysis definition only; spec source =
//     §3.2.5 verbatim quote (reproduced below).
//   - E2' relaxation (this cycle only): production knob
//     `lspInterpRoundMode` (default 0 = floor) added to
//     `internal/lsp/interpolate.go`. Default branch is byte-EQ to the
//     cycle-entry expression `(int32(prev[i]) + int32(curr[i])) >> 1`.
//     Mode 1 (round-half-away-from-zero) is exercised here via
//     `lsp.SetLSPInterpRoundModeForTest(1)` and restored on defer.
//     The knob is removed at RC-3 cycle close per the plan's Phase 7
//     hard requirement.
//   - E3: gate 17 t.Skip is NOT removed in this commit. RC-3
//     synthesis dispatches the user gate G-XS7 (Defect verdict only).
//   - E4: classifier = strict equality. "Direction match" /
//     "magnitude reduced" / "1-LSB closer to want" categorisations
//     are forbidden. Only sign-flip-to-want (-1) or unchanged/diverge
//     are recognised verdict cells.
//   - E5: this is a measurement test. No regression-gate auto-
//     promotion; the t.Errorf below guards only the E2' default-branch
//     byte-EQ invariant (i.e. the cycle-entry production baseline).
//
// ============================================================================
// SPEC VERBATIM CITATION — ITU-T G.729 (06/2012), PDF p. 14, §3.2.5
// "Interpolation of the LSP coefficients", lines 901..919; extracted
// via:
//
//	pdftotext -layout docs/superpowers/specs/itu/G729E.pdf -
//
// (re-extracted at RC-1 commit; verbatim status unchanged from CE-3):
//
//	"3.2.5  Interpolation of the LSP coefficients
//	 The quantized (and unquantized) LP coefficients are used for the
//	 second subframe. For the first subframe, the quantized (and
//	 unquantized) LP coefficients are obtained by linear interpolation
//	 of the corresponding parameters in the adjacent subframes. The
//	 interpolation is done on the LSP coefficients in the cosine
//	 domain. Let q_i^(current) be the LSP coefficients computed for
//	 the current 10 ms frame, and q_i^(previous) the LSP coefficients
//	 computed in the previous 10 ms frame. The (unquantized)
//	 interpolated LSP coefficients in each of the two subframes are
//	 given by:
//	    Subframe 1 : q_i^(1) = 0.5 q_i^(previous) + 0.5 q_i^(current)
//	                                       i = 1,...,10
//	    Subframe 2 : q_i^(2) = q_i^(current)        i = 1,...,10
//	                                                           (24)
//	 The same interpolation procedure is used for the interpolation
//	 of the quantized LSP coefficients by substituting q_i by q̂_i in
//	 equation (24)."
//
// R-C ambiguity: equation (24) gives the sf-1 weight as a real-valued
// "0.5·prev + 0.5·curr" with NO mention of the fixed-point rounding
// mode for the half-sum. Production resolves this with arithmetic
// right shift `>> 1` on a signed int32 sum (= floor toward −∞);
// symmetric rounding (round-half-away-from-zero) is the alternative
// resolution measured here.
//
// ============================================================================
// CLASSIFIER (per plan §1, table "Pass / Fail / Partial 정의"):
//
//	want = wantFrames[0][5..7] = [-1, -1, -1] (per ALGTHM.PST raw
//	measurement, Phase 1k F-oct-prelim-5-4 §3.2)
//	floorOut = sample 5..7 with knob = 0 (= cycle-entry production).
//	symOut   = sample 5..7 with knob = 1 (round-half-away-from-zero).
//
//	matches := count(symOut[k] == want[k] for k in 0..2)
//
//	matches == 3 → DEFECT_3of3      (Defect-confirmed; G-XS7 dispatch)
//	matches == 2 → DEFECT_2of3      (Defect-confirmed w/ rationale)
//	matches == 1 → PARTIAL_1of3     (Partial)
//	matches == 0 →
//	   if symOut == floorOut       → REFUTE_unchanged
//	   else                         → REFUTE_diverge
//
// E4 prohibitions enforced: "+1 → 0" is NOT a sign-to-want match
// (want is -1, 0 is also unmatched). Strict int16 equality only.
//
// t.Errorf is reserved for the E2' default-branch byte-EQ invariant
// guard (floorOut MUST equal the cycle-entry baseline of [+1, +1, +1]
// per F-oct-postfix evidence). All verdict reporting is t.Logf.
func TestDiagnostic_Phase1nRc1LSPInterpBranchALGTHM(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)
	if len(frames) == 0 {
		t.Fatalf("ALGTHM.BIT yielded zero frames")
	}

	want := [3]int16{wantFrames[0][5], wantFrames[0][6], wantFrames[0][7]}

	// ---- Sub-measurement A: a[] LP coefficients via lsp.Decoder ----
	// Drive a fresh lsp.Decoder under each mode to capture the sf1A
	// (Q12 LP) coefficient deltas that propagate from the +1 LSB sf-1
	// LSP drift identified in CE-3.
	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}
	idx := lsp.Indices{
		L0: uint8(f.L0),
		L1: uint8(f.L1),
		L2: uint8(f.L2),
		L3: uint8(f.L3),
	}

	var lsFloor lsp.Decoder
	sf1AFloor, sf2AFloor := lsFloor.Decode(idx)

	restoreA := lsp.SetLSPInterpRoundModeForTest(1)
	var lsSym lsp.Decoder
	sf1ASym, sf2ASym := lsSym.Decode(idx)
	restoreA()

	t.Logf("=== sub-A: a[] Q12 LP coefficient diff (sf-1 only; sf-2 = current LSP, mode-invariant) ===")
	t.Logf("  k    floor       sym      Δ(sym-floor)")
	anyA := false
	for k := 0; k < 11; k++ {
		d := int32(sf1ASym[k]) - int32(sf1AFloor[k])
		mark := ""
		if d != 0 {
			mark = "  ←Δ"
			anyA = true
		}
		t.Logf("  %2d %7d %7d %+8d%s", k, sf1AFloor[k], sf1ASym[k], d, mark)
	}
	if !anyA {
		t.Logf("  (no a[] coefficients changed under mode-1; sf-1 LSP drift fully absorbed by §3.2.6 expansion)")
	}
	for k := 0; k < 11; k++ {
		if sf2ASym[k] != sf2AFloor[k] {
			t.Errorf("sf2A[%d] differs between modes (got=%d want=%d): sf-2 path must be mode-invariant per eq. (24) sf-2",
				k, sf2ASym[k], sf2AFloor[k])
		}
	}

	// ---- Sub-measurement B: full Decoder.Decode sample 5..7 ----
	// Each sub-test instantiates its own Decoder so that pastSynth /
	// HP / postfilter state are zero-init'd identically and the only
	// difference between runs is the rounding-mode knob.

	captureSamples := func(setMode int) [3]int16 {
		var d Decoder
		var out [frameSamples]int16
		if setMode != 0 {
			restore := lsp.SetLSPInterpRoundModeForTest(setMode)
			defer restore()
		}
		if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
			t.Fatalf("Decode frame 0 (mode=%d): %v", setMode, err)
		}
		return [3]int16{out[5], out[6], out[7]}
	}

	floorOut := captureSamples(0)

	// E2' guard: knob default MUST yield byte-EQ to the cycle-entry
	// production baseline of [+1, +1, +1]. Any deviation here means
	// the default branch is not byte-EQ to the original `>> 1`
	// expression — an E2' violation.
	// E2' guard: knob default MUST yield byte-EQ to the cycle-entry
	// production baseline of [+2, +2, +2] per F-oct-postfix evidence
	// (post-ScaleUpSat ×2). Any deviation here means the default
	// branch is not byte-EQ to the original `>> 1` expression — an
	// E2' violation.
	// (NOTE: plan §1 prose says "[+1, +1, +1]" with Δ = +3; the +3
	// is consistent only with [+2, +2, +2] vs want [-1, -1, -1].
	// Verified empirically at RC-1 RED→GREEN.)
	wantBaseline := [3]int16{2, 2, 2}
	for k := 0; k < 3; k++ {
		if floorOut[k] != wantBaseline[k] {
			t.Errorf("E2' default-branch byte-EQ invariant violated: floor sample %d = %d, want cycle-entry baseline %d",
				k+5, floorOut[k], wantBaseline[k])
		}
	}

	symOut := captureSamples(1)

	// ---- Cell matrix: 2 modes × 3 samples ----
	t.Logf("=== RC-1 cell matrix: ALGTHM frame 0 sample 5..7 (2 modes × 3 samples) ===")
	t.Logf("  sample   floor (mode=0)   symmetric (mode=1)   want   Δ(sym-floor)   sym==want")
	matches := 0
	for k := 0; k < 3; k++ {
		match := symOut[k] == want[k]
		if match {
			matches++
		}
		t.Logf("  n=%d         %+6d            %+6d            %+6d      %+6d         %v",
			5+k, floorOut[k], symOut[k], want[k],
			int32(symOut[k])-int32(floorOut[k]), match)
	}

	// ---- Classifier verdict (plan §1 strict equality) ----
	var verdict string
	switch matches {
	case 3:
		verdict = "DEFECT_3of3"
	case 2:
		verdict = "DEFECT_2of3"
	case 1:
		verdict = "PARTIAL_1of3"
	default:
		if symOut == floorOut {
			verdict = "REFUTE_unchanged"
		} else {
			verdict = "REFUTE_diverge"
		}
	}
	t.Logf("=== RC-1 verdict: %s (matches=%d/3) ===", verdict, matches)
	t.Logf("  floor=%v sym=%v want=%v", floorOut, symOut, want)
	t.Logf("  RC-3 synthesis dispatch:")
	t.Logf("    DEFECT_*    → G-XS7 user gate (knob removal + symmetric-round promotion + gate 17 reactivation, plan §8)")
	t.Logf("    PARTIAL_1of3 → knob removal, expression revert to floor, (c) corrigendum + R-C secondary mechanism (plan §7)")
	t.Logf("    REFUTE_*    → knob removal, expression revert to floor (cycle-entry byte-EQ), (c) corrigendum or (a) multi-frame escalate (plan §7)")
	t.Logf("  Knob removal commitment recorded for RC-3 (plan Phase 7 hard requirement).")
}
