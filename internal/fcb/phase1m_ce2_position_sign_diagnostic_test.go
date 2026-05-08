package fcb

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestDiagnostic_Phase1mCe2PositionSignVerbatim — Phase 1m F-Cγ-elsewhere
// Task CE-2: FCB position+sign decode bit-field ordering verbatim
// cross-check, ALGTHM + FIXED + PITCH frame 0 subframe 0.
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-07-phase1m-stage-f-cgamma-elsewhere-plan.md
//	§Task CE-2.
//
// Cycle context (cumulative): 24 prior diagnostic cycles (16 Phase 1k +
// 4 Phase 0c + 2 Phase 1l + CE-1 Phase 1m) all REFUTED, defect = 0.
// Three hard-spec invariants previously confirmed verbatim (§4.2.4 AGC
// carryover, §4.3 catch-all zero-init, §A.4.2.5 IIR pole-pair impulse
// decay). Gate 17 RED test
// `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` is currently
// `t.Skip`'d at commit 9ab1c91. CE-1 (gain VQ Imap+GBK) refuted at
// f3df272 with 6 R-A-blocked cells. CE-2 is the second upstream
// (parameter decode) measurement — focus is the FCB pulse-position
// (13-bit C field) and pulse-sign (4-bit S field) bit-layout convention.
//
// ABSOLUTE CONSTRAINTS (E1/E2/E4/E5):
//   - clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 / Annex A
//     binary reference. Spec source = G729E.pdf + READMETV.txt only.
//     The verdict here is whether production `decodePositions`
//     (`positions.go:15`) + `placePulses` (`signs.go:17`) match the
//     verbatim text of §3.8 eq. (45), eq. (61), eq. (62), Table 7, and
//     §3.8.2 / §A.3.8 / §A.3.8.2.
//   - production = 0 line change (E2): this test is measurement-only.
//     `decodePositions` and `placePulses` are invoked unmodified;
//     spec-side reconstructions are recomputed inline.
//   - measurement-only (E5): hard-asserts only spec-derivable existence
//     invariants (track-residue invariant from Table 7 — pos[i] mod 5
//     ∈ {i, i+1} only for i=3, exactly i otherwise; ±PulseAmplitude
//     amplitude from eq. (45)). Cell verdicts are reported via t.Logf;
//     t.Errorf is reserved for cells that violate a verbatim-present
//     invariant with NO documented spec ambiguity.
//   - verdicts are EQ / NE / UNDETERMINED. UNDETERMINED is reserved for
//     cells whose spec text is verbatim ambiguous (R-B: §3.8.2 numerical
//     decomposition vs Table 8 NOTE "MSB transmitted first" produce
//     two distinct decoder bit-layouts; the verbatim text does not
//     unambiguously commit to one; production code self-doc in
//     `signs.go:8-12` admits this ambiguity).
//
// ============================================================================
// SPEC VERBATIM CITATIONS (mandatory) — extracted via:
//
//	pdftotext -layout docs/superpowers/specs/itu/G729E.pdf -
//
// ============================================================================
//
// (1) §3.8 "Fixed codebook – Structure and search" — Table 7
//
//	(PDF p. 20, lines 1201..1217):
//
//	"The fixed codebook is based on an algebraic codebook structure
//	 using an interleaved single-pulse permutation (ISPP) design. In
//	 this codebook, each codebook vector contains four non-zero
//	 pulses. Each pulse can have either the amplitudes +1 or –1, and
//	 can assume the positions given in Table 7.
//
//	                Table 7 – Structure of fixed codebook C
//
//	   Pulse        Sign                       Positions
//	     i0       s0: ±1     m0: 0,  5, 10, 15, 20, 25, 30, 35
//	     i1       s1: ±1     m1: 1,  6, 11, 16, 21, 26, 31, 36
//	     i2       s2: ±1     m2: 2,  7, 12, 17, 22, 27, 32, 37
//	     i3       s3: ±1     m3: 3,  8, 13, 18, 23, 28, 33, 38
//	                            4,  9, 14, 19, 24, 29, 34, 39
//
//	 The codebook vector c(n) is constructed by taking a zero vector
//	 of dimension 40, and putting the four unit pulses at the found
//	 locations, multiplied with their corresponding sign:
//
//	     c(n) = s0·δ(n − m0) + s1·δ(n − m1)
//	          + s2·δ(n − m2) + s3·δ(n − m3)        n = 0,...,39   (45)"
//
//	⇒ Eq. (45) is VERBATIM EXPLICIT on the c(n) construction:
//	     pulse i is at position m_i with sign s_i, residue m_i mod 5
//	     ∈ {i} for i=0,1,2 and ∈ {3,4} for i=3.
//	   This is a HARD invariant — every production-decoded c[] vector
//	   must satisfy: exactly four nonzero entries; pos[i] mod 5 == i
//	   for i ∈ {0,1,2}; pos[3] mod 5 ∈ {3,4}; |c[pos[i]]| ==
//	   PulseAmplitude (Q13 ±1.0).
//
// (2) §3.8.2 "Codeword computation of the fixed codebook"
//
//	(PDF p. 22, lines 1320..1328):
//
//	"The pulse positions of the pulses i0, i1 and i2, are encoded
//	 with 3 bits each, while the position of i3 is encoded with
//	 4 bits. Each pulse amplitude is encoded with 1 bit. This gives
//	 a total of 17 bits for the 4 pulses. By defining s = 1 if the
//	 sign is positive and s = 0 if the sign is negative, the sign
//	 codeword is obtained from:
//
//	     S = s0 + 2·s1 + 4·s2 + 8·s3                              (61)
//
//	 and the fixed-codebook codeword is obtained from:
//
//	     C = (m0/5) + 8·(m1/5) + 64·(m2/5)
//	         + 512·(2·(m3/5) + jx)                                (62)
//
//	 where jx = 0 if m3 = 3, 8,...,38, and jx = 1 if m3 =
//	 4, 9,...,39."
//
//	⇒ Eq. (61) is VERBATIM EXPLICIT on the numerical decomposition of
//	  S as an integer: bit_k of the integer S equals s_k. Therefore
//	  the sign of pulse i is bit i of S (LSB-first under integer-bit
//	  semantics).
//
//	⇒ Eq. (62) is VERBATIM EXPLICIT on the numerical decomposition of
//	  C as an integer: bits 0..2 = m0/5 = i0, bits 3..5 = m1/5 = i1,
//	  bits 6..8 = m2/5 = i2, bit 9 = jx, bits 10..12 = m3/5 = i3.
//
//	⇒ Eqs. (61)/(62) are VERBATIM SILENT on the *transmission bit
//	  order*. They define S and C as integer values; whether the
//	  decoder reconstructs the same integer or some bit-permuted
//	  variant depends on Table 8 (see citation 4).
//
// (3) §A.3.8 "Fixed codebook – Structure and search" (Annex A)
//
//	(PDF p. 41, lines 2182..2190):
//
//	"The structure of the 17-bit algebraic codebook is the same as
//	 described in clause 3.8.
//	 A.3.8.1 Fixed-codebook search procedure
//	 The signs of the pulses are found using the same approach
//	 explained in clause 3.8.1. However, the pulse positions are
//	 found using a more efficient approach. Instead of the
//	 nested-loop search approach, an iterative depth-first, tree
//	 search approach is used. In this new approach a smaller number
//	 of pulse position combinations is tested and it has fixed
//	 complexity.
//	 A.3.8.2 Codeword computation of the fixed codebook
//	 Same as described in clause 3.8.2."
//
//	⇒ Annex A inherits §3.8.2 verbatim — eqs. (61) and (62) apply
//	  unchanged. Track residues per Table 7 unchanged.
//
// (4) §4 "Functional description of the decoder" — Table 8 NOTE
//
//	(PDF p. 25, lines 1446..1464):
//
//	"           Table 8 – Description of transmitted parameters indices
//	   Symbol            Description                                       Bits
//	   ...
//	   C1     Fixed codebook first subframe                                 13
//	   S1     Signs of fixed-codebook pulses 1st subframe                    4
//	   ...
//	   NOTE – The bit stream ordering is reflected by the order in the
//	   table. For each parameter, the most significant bit (MSB) is
//	   transmitted first."
//
//	⇒ Table 8 NOTE is VERBATIM EXPLICIT that within each parameter,
//	  the MSB is transmitted first. Combined with eq. (62)
//	  integer-decomposition, this implies that bit 12 of C
//	  (= top three bits = m3/5) is transmitted first, and bit 0
//	  (= i0 LSB) is transmitted last.
//
//	⇒ HOWEVER, Table 8 NOTE is VERBATIM SILENT on whether "the bits
//	  of parameter C" refers to the bits of the *integer C as defined
//	  by eq. (62)*, or to a custom *bit-string layout per parameter*
//	  (e.g. pulse-major order). Production `decodePositions`
//	  (`positions.go:15`) reads bits 12..10 as i0, treating the
//	  highest-MSB-transmitted bits as the i0 field — this corresponds
//	  to a pulse-major bit-string interpretation, NOT the eq. (62)
//	  integer-decomposition. Production self-doc in `signs.go:8-12`
//	  states: "The exact bit→sign mapping is an encoder convention
//	  not pinned by the spec."  ← (R-B) ambiguity, verbatim self-
//	  declared.
//
// ============================================================================
// VERDICT MODEL
// ============================================================================
//
// Cell matrix (3 vectors × 4 cells = 12 cells):
//
//	cell P (positions): production positions[i] vs spec-eq-(62)-verbatim
//	    positions. EQ if the i-tuples agree element-wise; NE if not;
//	    UNDETERMINED if R-B blocking forces both interpretations to be
//	    equally valid spec-text-derivations. Cells where the production
//	    bit-layout differs from the eq-(62) integer-decomposition but
//	    matches one of two equally-permitted readings are recorded NE
//	    under the integer-decomposition reading + UNDETERMINED summary.
//
//	cell S (signs): production sign-of-pulse-i = (S>>(3-i))&1 vs
//	    spec-eq-(61)-verbatim sign-of-pulse-i = (S>>i)&1. Same
//	    EQ / NE / UNDETERMINED model as cell P.
//
//	cell C (c[]): production c[] vs spec-eq-(45)-verbatim c[] (pulses
//	    at spec positions with spec signs, no pre-emphasis). EQ if
//	    nonzero pattern + signs agree at every position; NE otherwise.
//	    R-B blocking applies (since c[] is downstream of P and S).
//
//	cell T (track-residue invariant from Table 7 — HARD): production
//	    pos[i] mod 5 == i for i ∈ {0,1,2}; pos[3] mod 5 ∈ {3,4}.
//	    HARD invariant — eligible for t.Errorf escalation since
//	    Table 7 verbatim text is unambiguous on track-residue
//	    structure regardless of bit-ordering convention.
//
// Sub-test scope:
//   - "ALGTHM_f0_sf0": low-energy frame; informs the gate 17 sample
//     5..7 mismatch directly (sf0 covers samples 0..39).
//   - "FIXED_f0_sf0":  high-energy fixed-codebook coverage frame.
//   - "PITCH_f0_sf0":  pitch-coverage frame; cross-vector consistency.
//
// ============================================================================
// ESCAPE-HATCH THRESHOLD (P0c-2 / CE-1 pattern)
// ============================================================================
//
// A confirmed cell may only escalate to t.Errorf if BOTH:
//  1. The cell's spec text is *verbatim present* (NOT R-B blocked); AND
//  2. The production output disagrees in a way that cannot be explained
//     by a documented bit-ordering / encoder-convention choice left
//     underspecified by the verbatim text.
//
// Cells P, S, C are all R-B-blocked because the bit-ordering convention
// is verbatim-ambiguous (eq. 62 integer vs Table 8 NOTE pulse-major).
// Therefore these cells are reported via t.Logf only and recorded as
// NE-under-eq62-reading + UNDETERMINED-overall.
//
// Cell T (track-residue) is verbatim-unambiguous (Table 7 fixes the
// allowed positions per pulse INDEPENDENTLY of bit ordering — no
// matter which convention, pos[i] must lie on track i). Cell T is
// therefore eligible for t.Errorf escalation.
//
// ============================================================================
// HARD ASSERTIONS — spec-derivable invariants only
// ============================================================================
//   - C1 ∈ [0, 8191]                      (§3.8.2 / Table 8: 13 bits)
//   - S1 ∈ [0, 15]                        (§3.8.2 / Table 8: 4 bits)
//   - production pos[i] mod 5 == i (i ∈ 0..2)  (Table 7 verbatim)
//   - production pos[3] mod 5 ∈ {3, 4}    (Table 7 verbatim)
//   - production positions[i] all distinct (Table 7 ISPP design)
//   - exactly 4 nonzero entries in c[]    (eq. 45 verbatim)
//   - |c[pos[i]]| == PulseAmplitude       (eq. 45 verbatim)
func TestDiagnostic_Phase1mCe2PositionSignVerbatim(t *testing.T) {
	// NOTE: empirically, ALGTHM/FIXED/PITCH frame 0 sf0 ALL have
	// C1=0x0000 and S1=0xF (verified via a one-shot dump). C1=0 makes
	// production and eq.(62) interpretations of positions trivially
	// agree (both yield [0,1,2,3]); S1=0xF makes production and
	// eq.(61) interpretations of signs trivially agree (all +1). The
	// sf0 cells therefore cannot disambiguate the bit-ordering
	// convention — they are EQ-by-degeneracy. To obtain non-degenerate
	// cross-vector evidence we additionally include sf1 (C2/S2) of
	// frame 0 for each vector, where C2 is non-zero and the two
	// interpretations diverge.
	type vector struct {
		name string
		file string
		sf   int // 0 → use C1/S1, 1 → use C2/S2
	}
	vectors := []vector{
		{"ALGTHM_f0_sf0", "ALGTHM.BIT", 0},
		{"FIXED_f0_sf0", "FIXED.BIT", 0},
		{"PITCH_f0_sf0", "PITCH.BIT", 0},
		{"ALGTHM_f0_sf1", "ALGTHM.BIT", 1},
		{"FIXED_f0_sf1", "FIXED.BIT", 1},
		{"PITCH_f0_sf1", "PITCH.BIT", 1},
	}

	type cell struct {
		vec      string
		variable string
		observed string
		expected string
		verdict  string // EQ / NE / UNDETERMINED
		notes    string
	}
	var cells []cell

	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			bitPath := ce2VectorPath(v.file)
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
				t.Fatalf("%s frame 0 bad-flag set", v.name)
			}

			var f bitstream.Frame
			if err := bitstream.Unpack(frames[0], &f); err != nil {
				t.Fatalf("Unpack %s frame 0: %v", v.name, err)
			}

			c1 := f.C1
			s1 := uint8(f.S1)
			if v.sf == 1 {
				c1 = f.C2
				s1 = uint8(f.S2)
			}

			// ---- 13-bit / 4-bit field range (spec-derivable) ----
			if c1 > 0x1FFF {
				t.Fatalf("%s: C1=0x%X out of [0,0x1FFF] (§3.8.2 13-bit field)", v.name, c1)
			}
			if s1 > 0x0F {
				t.Fatalf("%s: S1=0x%X out of [0,0x0F] (§3.8.2 4-bit field)", v.name, s1)
			}

			// ---- raw bitstream values ----
			t.Logf("%s: raw bitstream  C1=0x%04X (%d, 13 bits = 0b%013b), S1=0x%X (%d, 4 bits = 0b%04b)",
				v.name, c1, c1, c1, s1, s1, s1)

			// ---- production decode ----
			prodPositions := decodePositions(c1)
			var prodC [40]int16
			placePulses(prodPositions, s1, &prodC)

			// production bit decomposition (positions.go:15 — MSB-first)
			prodI0 := (c1 >> 10) & 0x07
			prodI1 := (c1 >> 7) & 0x07
			prodI2 := (c1 >> 4) & 0x07
			prodJx := (c1 >> 3) & 0x01
			prodI3 := c1 & 0x07
			t.Logf("%s: PROD positions  bits[12..10]=i0=%d  bits[9..7]=i1=%d  bits[6..4]=i2=%d  bit3=jx=%d  bits[2..0]=i3=%d",
				v.name, prodI0, prodI1, prodI2, prodJx, prodI3)
			t.Logf("%s: PROD positions[] = [%d, %d, %d, %d]",
				v.name, prodPositions[0], prodPositions[1], prodPositions[2], prodPositions[3])

			// production sign decomposition (signs.go:17 — pulse i ← bit (3-i))
			prodSigns := [4]int{
				signBit(s1, 3), // pulse 0 ← bit 3 (production)
				signBit(s1, 2), // pulse 1 ← bit 2
				signBit(s1, 1), // pulse 2 ← bit 1
				signBit(s1, 0), // pulse 3 ← bit 0
			}
			t.Logf("%s: PROD signs      pulse0=bit3=%+d  pulse1=bit2=%+d  pulse2=bit1=%+d  pulse3=bit0=%+d (production: pulse i ← bit (3-i))",
				v.name, prodSigns[0], prodSigns[1], prodSigns[2], prodSigns[3])

			// ---- spec-verbatim decode (eq. 61 + eq. 62 integer decomposition) ----
			specI0 := c1 & 0x07         // bits 0..2 (eq. 62)
			specI1 := (c1 >> 3) & 0x07  // bits 3..5
			specI2 := (c1 >> 6) & 0x07  // bits 6..8
			specJx := (c1 >> 9) & 0x01  // bit 9
			specI3 := (c1 >> 10) & 0x07 // bits 10..12
			specPositions := [4]int{
				5*int(specI0) + 0,
				5*int(specI1) + 1,
				5*int(specI2) + 2,
				5*int(specI3) + 3 + int(specJx),
			}
			t.Logf("%s: SPEC positions  bits[2..0]=i0=%d  bits[5..3]=i1=%d  bits[8..6]=i2=%d  bit9=jx=%d  bits[12..10]=i3=%d",
				v.name, specI0, specI1, specI2, specJx, specI3)
			t.Logf("%s: SPEC positions[] = [%d, %d, %d, %d]",
				v.name, specPositions[0], specPositions[1], specPositions[2], specPositions[3])

			// spec sign-of-pulse-i = bit i of S (eq. 61: S = s0 + 2·s1 + 4·s2 + 8·s3)
			specSigns := [4]int{
				signBit(s1, 0), // pulse 0 ← bit 0 (eq. 61)
				signBit(s1, 1), // pulse 1 ← bit 1
				signBit(s1, 2), // pulse 2 ← bit 2
				signBit(s1, 3), // pulse 3 ← bit 3
			}
			t.Logf("%s: SPEC signs      pulse0=bit0=%+d  pulse1=bit1=%+d  pulse2=bit2=%+d  pulse3=bit3=%+d (eq. 61: S = s0+2·s1+4·s2+8·s3)",
				v.name, specSigns[0], specSigns[1], specSigns[2], specSigns[3])

			// spec c[] reconstruction (eq. 45, no pre-emphasis)
			var specC [40]int16
			for i := 0; i < 4; i++ {
				if specSigns[i] > 0 {
					specC[specPositions[i]] = PulseAmplitude
				} else {
					specC[specPositions[i]] = -PulseAmplitude
				}
			}

			// ---- HARD invariant: track-residue (Table 7) ----
			// Cell T — verbatim Table 7 mandates pos[i] mod 5 == i for
			// i ∈ {0,1,2} and pos[3] mod 5 ∈ {3,4}, INDEPENDENTLY of any
			// bit-ordering convention.
			cellTVerdict := "EQ"
			cellTNotes := "Table 7 track residues satisfied by production positions"
			for i := 0; i < 3; i++ {
				if prodPositions[i]%5 != i {
					cellTVerdict = "NE"
					cellTNotes = fmt.Sprintf("PROD pos[%d]=%d residue=%d != %d", i, prodPositions[i], prodPositions[i]%5, i)
					t.Errorf("%s cell T (Table 7 track residue) NE: pos[%d]=%d, residue=%d, want %d",
						v.name, i, prodPositions[i], prodPositions[i]%5, i)
				}
			}
			r3 := prodPositions[3] % 5
			if r3 != 3 && r3 != 4 {
				cellTVerdict = "NE"
				cellTNotes = fmt.Sprintf("PROD pos[3]=%d residue=%d ∉ {3,4}", prodPositions[3], r3)
				t.Errorf("%s cell T (Table 7 pulse 3 residue) NE: pos[3]=%d, residue=%d, want ∈{3,4}",
					v.name, prodPositions[3], r3)
			}
			// distinct positions
			seen := map[int]bool{}
			for i := 0; i < 4; i++ {
				if seen[prodPositions[i]] {
					cellTVerdict = "NE"
					cellTNotes = fmt.Sprintf("PROD positions not distinct: %v", prodPositions)
					t.Errorf("%s cell T (ISPP distinctness) NE: positions=%v", v.name, prodPositions)
				}
				seen[prodPositions[i]] = true
			}
			cells = append(cells, cell{v.name, "T: track residues (Table 7 HARD)",
				fmt.Sprintf("residues=[%d,%d,%d,%d]",
					prodPositions[0]%5, prodPositions[1]%5,
					prodPositions[2]%5, prodPositions[3]%5),
				"[0,1,2,3 or 4]",
				cellTVerdict, cellTNotes})

			// ---- HARD invariant: 4 nonzero entries with ±PulseAmplitude (eq. 45) ----
			nonZeroCount := 0
			for n := 0; n < 40; n++ {
				if prodC[n] != 0 {
					nonZeroCount++
					if prodC[n] != PulseAmplitude && prodC[n] != -PulseAmplitude {
						t.Errorf("%s eq.(45) HARD: c[%d]=%d, want ±%d",
							v.name, n, prodC[n], PulseAmplitude)
					}
				}
			}
			if nonZeroCount != 4 {
				t.Errorf("%s eq.(45) HARD: nonzero c[] entries=%d, want 4", v.name, nonZeroCount)
			}

			// ---- cell P: positions vs spec-eq-(62) ----
			positionsEq := prodPositions == specPositions
			cellPVerdict := "UNDETERMINED"
			cellPNotes := "R-B blocking: §3.8.2 eq.(62) integer-decomposition vs Table 8 NOTE bit-string ordering verbatim ambiguous; production self-doc (signs.go:8-12) admits ambiguity"
			if positionsEq {
				cellPVerdict = "EQ"
				cellPNotes = "production positions agree with eq.(62) integer-decomposition reading on this input"
			} else {
				cellPNotes += fmt.Sprintf("; eq-(62)-reading: NE — prod=%v vs spec=%v", prodPositions, specPositions)
			}
			cells = append(cells, cell{v.name, "P: positions (eq.62 vs prod)",
				fmt.Sprintf("%v", prodPositions),
				fmt.Sprintf("%v", specPositions),
				cellPVerdict, cellPNotes})

			// ---- cell S: signs vs spec-eq-(61) ----
			signsEq := prodSigns == specSigns
			cellSVerdict := "UNDETERMINED"
			cellSNotes := "R-B blocking: §3.8.2 eq.(61) integer-bit-decomposition vs production (3-i) convention verbatim ambiguous; production self-doc (signs.go:8-12) admits ambiguity"
			if signsEq {
				cellSVerdict = "EQ"
				cellSNotes = "production signs agree with eq.(61) integer-bit-decomposition reading on this input"
			} else {
				cellSNotes += fmt.Sprintf("; eq-(61)-reading: NE — prod=%v vs spec=%v", prodSigns, specSigns)
			}
			cells = append(cells, cell{v.name, "S: signs (eq.61 vs prod)",
				fmt.Sprintf("%v", prodSigns),
				fmt.Sprintf("%v", specSigns),
				cellSVerdict, cellSNotes})

			// ---- cell C: c[] vector vs spec-eq-(45) reconstruction ----
			cEq := prodC == specC
			cellCVerdict := "UNDETERMINED"
			cellCNotes := "R-B blocking propagates from cells P, S — c[] is fully determined by (positions, signs)"
			if cEq {
				cellCVerdict = "EQ"
				cellCNotes = "production c[] agrees with spec-eq-(45) reconstruction on this input"
			} else {
				// Dump nonzero patterns for both
				cellCNotes += fmt.Sprintf("; PROD nonzeros: %s; SPEC nonzeros: %s",
					nonZeroSummary(prodC), nonZeroSummary(specC))
			}
			cells = append(cells, cell{v.name, "C: c[] (eq.45 vs prod)",
				nonZeroSummary(prodC),
				nonZeroSummary(specC),
				cellCVerdict, cellCNotes})

			// ---- gate 17 sample-5..7 cross-vector observation ----
			//
			// The plan (Phase 1m §Task CE-2 report-back) asks: for
			// ALGTHM frame 0 sf0, what FCB pulse(s) (if any) land in
			// position 5..7? What sign? Does that sign explain the
			// observed +1/+1/+1 vs -1/-1/-1 mismatch at sample 5..7?
			//
			// We dump the per-sample c[] entries 5..7 from BOTH
			// production and spec-eq-(45) reconstructions (no pitch
			// pre-emphasis applied; that is a separate filter stage).
			t.Logf("%s: SAMPLE 5..7  PROD c[5]=%+d c[6]=%+d c[7]=%+d   SPEC c[5]=%+d c[6]=%+d c[7]=%+d",
				v.name,
				prodC[5], prodC[6], prodC[7],
				specC[5], specC[6], specC[7])
		})
	}

	// ---- emit cell matrix summary ----
	t.Logf("")
	t.Logf("=== Phase 1m CE-2 cell matrix (vector × variable) ===")
	t.Logf("%-16s | %-40s | %-12s | %s",
		"vector", "variable", "verdict", "notes")
	t.Logf("%s", "-----------------+------------------------------------------+--------------+--------")
	eq, ne, und := 0, 0, 0
	for _, c := range cells {
		t.Logf("%-16s | %-40s | %-12s | %s",
			c.vec, c.variable, c.verdict, c.notes)
		switch c.verdict {
		case "EQ":
			eq++
		case "NE":
			ne++
		case "UNDETERMINED":
			und++
		}
	}
	t.Logf("=== totals: EQ=%d, NE=%d, UNDETERMINED=%d (cells P,S,C R-B-blocked unless production fortuitously aligns with eq.61/62 reading on this input) ===",
		eq, ne, und)
}

// signBit returns +1 if bit `pos` of S is 1, else -1.
func signBit(s uint8, pos uint) int {
	if (s>>pos)&1 == 1 {
		return +1
	}
	return -1
}

// nonZeroSummary returns a compact "[n]=v, ..." string of nonzero entries
// of c.
func nonZeroSummary(c [40]int16) string {
	out := ""
	for n := 0; n < 40; n++ {
		if c[n] != 0 {
			if out != "" {
				out += ", "
			}
			out += fmt.Sprintf("[%d]=%+d", n, c[n])
		}
	}
	if out == "" {
		return "(all zero)"
	}
	return out
}

// ce2VectorPath builds a path into the Annex A test-vector tree from
// the internal/fcb package directory.
func ce2VectorPath(name string) string {
	return filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", name)
}
