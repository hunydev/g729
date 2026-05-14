package gain

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// TestDiagnostic_Phase1mCe1GainVQTableVerbatim — Phase 1m F-Cγ-elsewhere
// Task CE-1: gain VQ Imap + GBK table verbatim cross-check, ALGTHM
// frame 0 sf0 (+ sf1 cross-subframe sanity).
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-07-phase1m-stage-f-cgamma-elsewhere-plan.md
//	§Task CE-1.
//
// Cycle context (cumulative): 22 prior sub-hypotheses (16 Phase 1k +
// 4 Phase 0c + 2 Phase 1l) all REFUTED, defect = 0. CE-1 is the first
// upstream-direction (parameter decode) measurement task in path (b)
// of user gate G-XS5; all 22 prior refutations were downstream of LP
// synthesis (postfilter / HP filter). Three hard-spec invariants
// previously confirmed verbatim (§4.2.4 AGC carryover, §4.3 catch-all
// zero-init, §A.4.2.5 IIR pole-pair impulse decay). Gate 17 RED test
// `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` is currently
// `t.Skip`'d at commit 9ab1c91 with reactivation triggers documented.
//
// ABSOLUTE CONSTRAINTS (E1/E2/E4/E5):
//   - clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 / Annex A
//     binary reference. spec source = G729E.pdf + READMETV.txt + textbooks
//     only. All numerical comparisons in this test target production
//     `tables.GainImap1`, `tables.GainImap2`, `tables.GainGBK1`, and
//     `tables.GainGBK2`, whose entries are themselves the merger-doctrine
//     transcription documented in `internal/tables/gain_gbk1.go` /
//     `gain_gbk2.go`. The verdict here is whether the *decoder pipeline*
//     (Imap composition + componentwise add + Word16 saturation) matches
//     the spec verbatim text of §3.9 / §3.9.3 / §A.3.9 — NOT a redundant
//     check of the table values themselves.
//   - production = 0 line change (E2): this test is measurement-only.
//     `decodeVQ` is invoked unmodified; intermediate values are also
//     recomputed inline (so this test serves as an audit of the
//     production composition, not a wrapper around it).
//   - measurement-only (E5): hard-asserts only spec-derivable existence
//     invariants (Imap arrays length 8/16, GBK arrays length 8×2 / 16×2,
//     bit fields in [0,7] / [0,15]). Cell verdicts are reported via
//     t.Logf; t.Errorf is reserved for confirmed NE that contradicts the
//     verbatim spec text reproduced below.
//   - verdicts are binary EQ / NE; UNDETERMINED is reserved for cells
//     whose spec text is verbatim absent (E4 / R-A — §3.9.3 reorder
//     paragraph).
//
// ============================================================================
// SPEC VERBATIM CITATIONS (mandatory) — extracted via:
//
//	pdftotext -layout docs/superpowers/specs/itu/G729E.pdf -
//
// ============================================================================
//
// (1) §3.9.2 "Codebook search for gain quantization"
//
//	(PDF p. 23, lines 1386..1406):
//
//	"The adaptive-codebook gain, gp, and the factor γ are vector
//	 quantized using a two-stage conjugate structured codebook. The
//	 first stage consists of a 3 bit two-dimensional codebook GA, and
//	 the second stage consists of a 4 bit two-dimensional codebook GB.
//	 The first element in each codebook represents the quantized
//	 adaptive-codebook gain ĝ_p, and the second element represents the
//	 quantized fixed-codebook gain correction factor γ̂. Given codebook
//	 indices GA and GB for GA and GB, respectively, the quantized
//	 adaptive-codebook gain is given by:
//
//	     ĝ_p = GA1(GA) + GB1(GB)                                 (73)
//
//	 and the quantized fixed-codebook gain by:
//
//	     ĝ_c = g_c' γ̂ = g_c' (GA2(GA) + GB2(GB))                (74)"
//
//	⇒ Algorithmic mandate (componentwise sum across the two stages,
//	  one component for ĝ_p and one component for γ̂) is VERBATIM
//	  PRESENT. Production `decodeVQ` (`vq.go:19`):
//	     gpQ14    = GBK1[entry1][0] + GBK2[entry2][0]
//	     gammaCQ13= GBK1[entry1][1] + GBK2[entry2][1]
//	  matches eq. (73)-(74) in algorithmic shape.
//
//	⇒ §3.9.2 is VERBATIM SILENT on:
//	     - Q-format of the GA/GB table entries (Q14 vs Q13 split is
//	       a numerical-distribution decision, not a spec mandate).
//	     - Saturation behaviour of the sum (Word16 wrap vs saturate).
//
// (2) §3.9.3 "Codeword computation for gain quantizer"
//
//	(PDF p. 24, lines 1407..1409) — full verbatim text:
//
//	"The codewords GA and GB for the gain quantizer are obtained from
//	 the indices corresponding to the best choice. To reduce the impact
//	 of single bit errors the codebook indices are mapped."
//
//	⇒ §3.9.3 ASSERTS THE EXISTENCE of a mapping (encoder side:
//	  physical-entry → transmitted-codeword). The inverse map (decoder
//	  side: transmitted-codeword → physical-entry) is implied by symmetry.
//
//	⇒ §3.9.3 IS VERBATIM SILENT ON THE MAP TABLE CONTENTS — no values
//	  are listed in the §3.9.3 paragraph itself. The map array names
//	  `map1` / `imap1` / `map2` / `ima21 [sic — Annex A typo for imap2]`
//	  appear only in Table 12 ("Summary of the speech coder fixed
//	  tables", PDF lines 1868..1873) without value enumeration.
//
//	⇒ This is the (R-A) ambiguity from the Phase 1m plan: the verbatim
//	  reorder mapping is ABSENT from the main-body PDF text. Production
//	  `tables.GainImap1` / `tables.GainImap2` values therefore rest on
//	  the merger-doctrine transcription of the data-table portion of the
//	  ITU reference distribution (declared in `gain_gbk1.go:24-31`).
//	  THIS TEST DOES NOT VALIDATE THE MAP VALUES THEMSELVES — that
//	  validation belongs to a separate spec-audit task. THIS TEST
//	  VALIDATES that the decoder applies the inverse map at all (i.e.
//	  decoder uses `Imap[idx]` not bare `idx` to index `GBK[]`), which
//	  IS spec-mandated by §3.9.3.
//
// (3) §A.3.9 "Quantization of the gains"
//
//	(PDF p. 41, lines 2191..2192) — full verbatim text:
//
//	"Same as described in clause 3.9."
//
//	⇒ Annex A inherits §3.9 / §3.9.3 in entirety: same eqs (73)-(74),
//	  same reorder mandate, same verbatim silence on map values.
//
// ============================================================================
// VERDICT MODEL
// ============================================================================
//
// Cell matrix (sub-stage × variable) for ALGTHM frame 0 sf0:
//
//	cell A: Imap-applied indexing (decoder uses GainImap*[GA]/[GB], not
//	        bare GA/GB, to index GBK*) — spec mandate §3.9.3 (existence),
//	        §A.3.9 (inheritance). EQ / NE.
//	cell B: ĝ_p assembly = GBK1[e1][0] + GBK2[e2][0] under Word16 add —
//	        spec mandate §3.9.2 eq. (73). EQ / NE.
//	cell C: γ̂  assembly = GBK1[e1][1] + GBK2[e2][1] under Word16 add —
//	        spec mandate §3.9.2 eq. (74). EQ / NE.
//	cell D: γ̂  sign — spec implication: γ̂ is a "correction factor"
//	        (§3.9.1 eq. (72): U(m) = 20·log(γ)) so γ > 0 ⇒ γ̂ > 0 in
//	        physical interpretation. Cell verdict EQ if γ̂ > 0, NE if
//	        γ̂ < 0, UNDETERMINED if γ̂ == 0 (boundary).
//	cell E: saturation flag — spec is verbatim silent on whether the
//	        componentwise sum saturates (Word16) or wraps; production
//	        uses `fixed.Add` which saturates. UNDETERMINED unless an
//	        actual saturation event is observed (in which case E4
//	        ambiguity must be raised).
//	cell F (R-A): Imap table values literal vs §3.9.3 verbatim text —
//	        UNDETERMINED (R-A blocking: §3.9.3 paragraph contains no
//	        map values).
//	cell G (R-A): GBK table values literal vs §3.9 verbatim text —
//	        UNDETERMINED (R-A-extended: §3.9 / §A.3.9 contain no
//	        numerical table; values come from the data-table portion
//	        of the merger-doctrine transcription, not the main-body
//	        text).
//
// Sub-test scope:
//   - "ALGTHM_sf0": primary measurement, frame 0 sf0.
//   - "ALGTHM_sf1": cross-subframe sanity, frame 0 sf1 (verifies that
//     the same Imap+GBK composition is applied identically across
//     subframes — a NE here would imply per-subframe state leakage in
//     the gain VQ decode path itself, which would directly contradict
//     §3.9 (gain VQ is stateless within the lookup; only the MA
//     predictor FIFO carries state, and that does not affect Imap/GBK)).
//
// ============================================================================
// ESCAPE-HATCH THRESHOLD (P0c-2 pattern)
// ============================================================================
//
// A confirmed NE cell may only escalate to t.Errorf if BOTH:
//  1. The cell's spec text above is *verbatim present* (not (R-A)/(R-B)/
//     (R-C) blocked); AND
//  2. The production output disagrees in a way that cannot be explained
//     by a documented Q-format / saturation / arithmetic-rounding choice
//     left underspecified by the verbatim text.
//
// Cells F, G are pre-classified UNDETERMINED (R-A blocking) and are
// therefore ineligible to drive an Errorf in this task. Cells A-E are
// EQ-eligible per the verbatim text reproduced above.
//
// ============================================================================
// HARD ASSERTIONS — spec-derivable invariants only
// ============================================================================
//   - len(tables.GainImap1) == 8         (§3.9.2: GA is 3-bit, 8 entries)
//   - len(tables.GainImap2) == 16        (§3.9.2: GB is 4-bit, 16 entries)
//   - len(tables.GainGBK1)  == 8         (§3.9.2 + Table 12)
//   - len(tables.GainGBK2)  == 16        (§3.9.2 + Table 12)
//   - GA1 ∈ [0,7]                        (§3.9.2: 3 bits)
//   - GB1 ∈ [0,15]                       (§3.9.2: 4 bits)
func TestDiagnostic_Phase1mCe1GainVQTableVerbatim(t *testing.T) {
	// ---- shape invariants (spec-derivable hard assertions) ----
	if got := len(tables.GainImap1); got != 8 {
		t.Fatalf("len(GainImap1)=%d, want 8 (§3.9.2: GA is 3 bits)", got)
	}
	if got := len(tables.GainImap2); got != 16 {
		t.Fatalf("len(GainImap2)=%d, want 16 (§3.9.2: GB is 4 bits)", got)
	}
	if got := len(tables.GainGBK1); got != 8 {
		t.Fatalf("len(GainGBK1)=%d, want 8 (§3.9.2 + Table 12)", got)
	}
	if got := len(tables.GainGBK2); got != 16 {
		t.Fatalf("len(GainGBK2)=%d, want 16 (§3.9.2 + Table 12)", got)
	}

	// ---- load ALGTHM frame 0 from the Annex A test-vector tree ----
	bitPath := ce1VectorPath("ALGTHM.BIT")
	if _, err := os.Stat(bitPath); err != nil {
		t.Skipf("missing test vector %s: %v", bitPath, err)
	}
	raw, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", bitPath, err)
	}
	frames, bads, err := bitstream.ReadG192File(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadG192File(%s): %v", bitPath, err)
	}
	if len(frames) == 0 {
		t.Fatalf("no frames in %s", bitPath)
	}
	if bads[0] {
		t.Fatalf("ALGTHM frame 0 bad-flag set")
	}

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack ALGTHM frame 0: %v", err)
	}

	type subSpec struct {
		name string
		ga   uint8
		gb   uint8
	}
	subs := []subSpec{
		{"ALGTHM_sf0", uint8(f.GA1), uint8(f.GB1)},
		{"ALGTHM_sf1", uint8(f.GA2), uint8(f.GB2)},
	}

	type cell struct {
		sub      string
		variable string
		observed string
		expected string
		verdict  string // EQ / NE / UNDETERMINED
		notes    string
	}
	var cells []cell

	for _, s := range subs {
		s := s
		t.Run(s.name, func(t *testing.T) {
			// ---- 3-bit / 4-bit field range (spec-derivable) ----
			if s.ga > 7 {
				t.Fatalf("%s: GA=%d out of [0,7] (§3.9.2 3-bit field)", s.name, s.ga)
			}
			if s.gb > 15 {
				t.Fatalf("%s: GB=%d out of [0,15] (§3.9.2 4-bit field)", s.name, s.gb)
			}

			// ---- raw bitstream values ----
			t.Logf("%s: raw bitstream  GA=%d (3 bits), GB=%d (4 bits)",
				s.name, s.ga, s.gb)

			// ---- cell A: Imap-applied indexing ----
			//
			// Production decodeVQ uses Imap1[GA]/Imap2[GB] to recover
			// the *physical* GBK row from the *transmitted* codeword
			// (per §3.9.3 reorder + §A.3.9 inheritance). Verify that
			// the production composition does NOT bypass Imap (which
			// would manifest as `bare-index row` ≠ `production row`
			// for any input where Imap is non-identity).
			e1 := tables.GainImap1[s.ga]
			e2 := tables.GainImap2[s.gb]
			t.Logf("%s: Imap-applied   e1=GainImap1[%d]=%d, e2=GainImap2[%d]=%d",
				s.name, s.ga, e1, s.gb, e2)

			imapIsIdentityForThisInput := (uint8(e1) == s.ga) && (uint8(e2) == s.gb)
			cellAVerdict := "EQ"
			cellANotes := "decoder uses Imap[idx] to index GBK[] (production vq.go:20-21)"
			if imapIsIdentityForThisInput {
				cellANotes += "; Imap happens to be identity at THIS input — cell A unfalsifiable on this single sample, EQ asserted via code inspection only"
			}
			cells = append(cells, cell{s.name, "A: Imap-applied indexing",
				"prod uses Imap[idx]", "spec §3.9.3 mandates reorder",
				cellAVerdict, cellANotes})

			// ---- cells B, C: ĝ_p, γ̂ assembly per eq. (73)-(74) ----
			gbk1 := tables.GainGBK1[e1]
			gbk2 := tables.GainGBK2[e2]
			t.Logf("%s: GBK row1       GBK1[%d] = (gp=%d Q14, γ̂=%d Q13)",
				s.name, e1, gbk1[0], gbk1[1])
			t.Logf("%s: GBK row2       GBK2[%d] = (gp=%d Q14, γ̂=%d Q13)",
				s.name, e2, gbk2[0], gbk2[1])

			// Inline recomputation of eq. (73)-(74). ĝ_p is bounded to
			// Word16, while γ̂_c is kept wide because legal Q13 joint
			// sums can exceed 32767.
			gpExpected := int16(fixed.Add(fixed.Word16(gbk1[0]), fixed.Word16(gbk2[0])))
			gammaExpected := int32(gbk1[1]) + int32(gbk2[1])

			// Production output via decodeVQ.
			gpProd, gammaProd := decodeVQ(Indices{GA: s.ga, GB: s.gb})
			t.Logf("%s: production     ĝ_p=%d Q14, γ̂=%d Q13", s.name, gpProd, gammaProd)
			t.Logf("%s: spec-recomputed ĝ_p=%d Q14, γ̂=%d Q13",
				s.name, gpExpected, gammaExpected)

			// cell B verdict.
			cellBVerdict := "EQ"
			cellBNotes := fmt.Sprintf("ĝ_p_prod=%d, ĝ_p_spec=%d", gpProd, gpExpected)
			if gpProd != gpExpected {
				cellBVerdict = "NE"
				t.Errorf("%s cell B (eq. 73 ĝ_p assembly) NE: prod=%d spec=%d",
					s.name, gpProd, gpExpected)
			}
			cells = append(cells, cell{s.name, "B: ĝ_p = GBK1[0]+GBK2[0] (eq. 73)",
				fmt.Sprintf("%d Q14", gpProd),
				fmt.Sprintf("%d Q14", gpExpected),
				cellBVerdict, cellBNotes})

			// cell C verdict.
			cellCVerdict := "EQ"
			cellCNotes := fmt.Sprintf("γ̂_prod=%d, γ̂_spec=%d", gammaProd, gammaExpected)
			if gammaProd != gammaExpected {
				cellCVerdict = "NE"
				t.Errorf("%s cell C (eq. 74 γ̂ assembly) NE: prod=%d spec=%d",
					s.name, gammaProd, gammaExpected)
			}
			cells = append(cells, cell{s.name, "C: γ̂ = GBK1[1]+GBK2[1] (eq. 74)",
				fmt.Sprintf("%d Q13", gammaProd),
				fmt.Sprintf("%d Q13", gammaExpected),
				cellCVerdict, cellCNotes})

			// ---- cell D: γ̂ sign ----
			//
			// Per §3.9.1 eq. (72): U(m) = 20·log(γ). For log() to
			// be defined (real-valued), γ > 0. Therefore γ̂ > 0 is
			// the spec-implied physical regime.
			cellDVerdict := "EQ"
			cellDNotes := "γ̂ > 0 (matches §3.9.1 eq. (72) physical regime: log(γ) requires γ>0)"
			switch {
			case gammaProd == 0:
				cellDVerdict = "UNDETERMINED"
				cellDNotes = "γ̂ == 0 (boundary; spec implication does not exclude exact 0)"
			case gammaProd < 0:
				cellDVerdict = "NE"
				cellDNotes = fmt.Sprintf("γ̂=%d < 0; contradicts §3.9.1 eq. (72) physical regime", gammaProd)
				t.Errorf("%s cell D (γ̂ sign): NE — γ̂=%d < 0", s.name, gammaProd)
			}
			cells = append(cells, cell{s.name, "D: γ̂ sign (positivity)",
				fmt.Sprintf("γ̂=%d", gammaProd),
				"γ̂ > 0 (§3.9.1 eq. 72 implication)",
				cellDVerdict, cellDNotes})

			// ---- cell E: saturation flag ----
			//
			// fixed.Add saturates to Word16. Detect whether saturation
			// actually fired on this input by recomputing the unsaturated
			// 32-bit sum and comparing to the saturated result.
			rawGp := int32(gbk1[0]) + int32(gbk2[0])
			rawGamma := int32(gbk1[1]) + int32(gbk2[1])
			gpSaturated := rawGp > 32767 || rawGp < -32768
			gammaSaturated := rawGamma > 32767 || rawGamma < -32768
			t.Logf("%s: unsat32 sums   ĝ_p=%d, γ̂=%d   (saturated_gp=%v, saturated_γ=%v)",
				s.name, rawGp, rawGamma, gpSaturated, gammaSaturated)

			cellEVerdict := "UNDETERMINED"
			cellENotes := "no saturation event observed on this input; spec verbatim silent on saturation policy (E4)"
			if gpSaturated || gammaSaturated {
				cellEVerdict = "UNDETERMINED"
				cellENotes = "saturation event observed; §3.9.2 verbatim silent on Word16-vs-Word32 sum semantics → E4 ambiguity, NOT escalated"
			}
			cells = append(cells, cell{s.name, "E: saturation policy",
				fmt.Sprintf("gp_sat=%v, γ̂_sat=%v", gpSaturated, gammaSaturated),
				"verbatim silent",
				cellEVerdict, cellENotes})

			// ---- cell F (R-A): Imap table values literal verbatim ----
			cells = append(cells, cell{s.name, "F: Imap[] literal values vs §3.9.3 verbatim",
				fmt.Sprintf("GainImap1=%v, GainImap2=%v", tables.GainImap1, tables.GainImap2),
				"§3.9.3 paragraph contains NO numerical map values",
				"UNDETERMINED",
				"R-A blocking: §3.9.3 verbatim text asserts existence of mapping but lists no values; map values come from merger-doctrine data-table transcription (gain_gbk1.go:24-31)"})

			// ---- cell G (R-A-extended): GBK table values literal verbatim ----
			cells = append(cells, cell{s.name, "G: GBK[][] literal values vs §3.9 verbatim",
				fmt.Sprintf("GBK1[%d]=%v, GBK2[%d]=%v", e1, gbk1, e2, gbk2),
				"§3.9 / §A.3.9 contain NO numerical GBK table",
				"UNDETERMINED",
				"R-A-extended: numerical table absent from §3.9 main-body text; values from merger-doctrine data-table transcription"})
		})
	}

	// ---- emit cell matrix summary ----
	t.Logf("")
	t.Logf("=== Phase 1m CE-1 cell matrix (sub-stage × variable) ===")
	t.Logf("%-12s | %-50s | %-8s | %s",
		"sub", "variable", "verdict", "notes")
	t.Logf("%s", "------------+----------------------------------------------------+----------+--------")
	eq, ne, und := 0, 0, 0
	for _, c := range cells {
		t.Logf("%-12s | %-50s | %-8s | %s",
			c.sub, c.variable, c.verdict, c.notes)
		switch c.verdict {
		case "EQ":
			eq++
		case "NE":
			ne++
		case "UNDETERMINED":
			und++
		}
	}
	t.Logf("=== totals: EQ=%d, NE=%d, UNDETERMINED=%d (cells F,G pre-classified UND per R-A) ===",
		eq, ne, und)
}

// ce1VectorPath builds a path into the Annex A test-vector tree from
// the internal/gain package directory.
func ce1VectorPath(name string) string {
	return filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", name)
}
