package decoder

import (
	"os"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// TestDiagnostic_Phase0c3CrossVectorPattern — Phase 0c-3 (P0c-3) cross-
// vector Δ pattern probe (`docs/superpowers/plans/2026-05-05-phase0c-
// reentry-want-domain-reinterpret-plan.md` §Task 3).
//
// Goal: extend the ALGTHM-only frame-0 measurement of P0c-1/P0c-2 to
// SPEECH / FIXED / PITCH so that the boundary-cluster Δ pattern
// observed in P0c-2 (postX2 differ-from-want = 36/80 indices, clustered
// at i ∈ [1..21] ∪ [65..79], middle [22..64] EQ; sign-mismatch = 4/80
// at samples {5,6,7, +1 unidentified}) can be checked for systemic
// recurrence across vectors.
//
// ABSOLUTE CONSTRAINTS (E0/E2/E5):
//   - clean-room MIT: no ITU C / bcg729 / Sipro Lab / FFmpeg / Annex A
//     binary reference; only the cited spec PDF + READMETV.txt + public
//     textbooks (Kondoz, Spanias).
//   - production = 0 line change (E2): test calls (*Decoder).Decode
//     unmodified.
//   - measurement-only: hard-asserts only spec-derivable invariants
//     (PST file size multiple of 160; production frame length 80).
//     Δ pattern is logged via t.Logf, never asserted (E5).
//
// ============================================================================
// SPEC VERBATIM CITATIONS (mandatory)
// ============================================================================
//
// (1) READMETV.txt (g729AnnexA test_vectors tree) — decoder invocation
//     and PCM format:
//
//        "decoder file.bit file.pst"
//        "Format: all files contain 16 bit sampled data using the
//         Intel (PC) format."
//
//     => *.pst is the decoder's output, 16-bit little-endian PCM. The
//     README does NOT specify the chain stage written to *.pst (no
//     "post-postfilter" / "post-HP" / "post-×2" qualifier). P0c-2
//     verified S* = postX2 (current production assumption holds).
//
// (2) READMETV.txt — vector size lines (file size in bytes), used here
//     to derive the multiple-of-160 (= 80 samples × 2 bytes) invariant:
//
//        "    5600  algthm.pst"       (35 frames × 160)
//        "  600000  speech.pst"       (3750 frames × 160)
//        "   19200  fixed.pst"        (120 frames × 160)
//        "  293600  pitch.pst"        (1835 frames × 160)
//
// (3) Vector substitution note: SPEED.PST and SINE.PST are NOT present
//     in the Annex A test_vectors directory. The plan §Task 3 selects
//     SPEECH / FIXED / PITCH as the actually-available substitutes
//     (ALGTHM = baseline). This is evidence-based per the file-listing
//     section of READMETV.txt above, not a cherry-pick.
//
// ============================================================================
// CHAIN-STAGE NOTE
// ============================================================================
//
// (*Decoder).Decode (decode.go) ends with pcm.ScaleUpSat applied
// in-place to the 80-sample frame buffer, so its output is exactly
// postX2 = §A.4.2.5 chain end (postfilter → HP → ×2). P0c-2 verdict
// confirmed S* = postX2 for ALGTHM frame 0; we reuse that mapping here
// without re-deriving it per vector (the P0c-3 probe is about Δ pattern
// shape, not chain-stage identification).
//
// ============================================================================
// Δ PATTERN CLASSIFIER
// ============================================================================
//
// classifyDeltaPattern(deltas []int16) string returns one of:
//
//   "zero"                     — all Δ == 0 (vector-EQ; no mismatch).
//   "sample-uniform-constant"  — all Δ identical and != 0 (post-
//                                processing constant offset candidate).
//   "sign-uniform-jitter"      — every nonzero Δ shares the same sign
//                                AND |Δ| ≤ 2 ∀ i (LSB rounding
//                                candidate).
//   "boundary-cluster"         — non-empty differ-set, ALL differ
//                                indices satisfy i < 22 OR i ≥ 65
//                                (HP filter transient / frame-boundary
//                                state mechanism candidate; edges
//                                derived from the P0c-2 cluster
//                                observation [1..21] ∪ [65..79]).
//   "random"                   — otherwise.
//
// Categories are checked in the order above; first match wins. The
// "boundary-cluster" check uses the 80-sample window only; the 16-
// sample variant degenerates because indices 0..15 are entirely inside
// the leading edge band (i < 22) so any nonzero Δ would trivially
// classify as boundary-cluster — therefore we ALSO log the 80-sample
// classification and use it for the "boundary-cluster" decision in
// the verdict matrix's right-most column.
//
// ============================================================================
// HARD ASSERTIONS — spec-derivable invariants only
// ============================================================================
//   - len(rawPST) % 160 == 0   (READMETV.txt size lines).
//   - len(production frame) == 80 (frameSamples).
//
// We do NOT hard-assert any specific Δ pattern (let t.Logf report).
func TestDiagnostic_Phase0c3CrossVectorPattern(t *testing.T) {
	type vectorSpec struct {
		name    string
		bitFile string
		pstFile string
	}
	vectors := []vectorSpec{
		{"ALGTHM", "ALGTHM.BIT", "ALGTHM.PST"},
		{"SPEECH", "SPEECH.BIT", "SPEECH.PST"},
		{"FIXED", "FIXED.BIT", "FIXED.PST"},
		{"PITCH", "PITCH.BIT", "PITCH.PST"},
	}

	type vectorResult struct {
		name           string
		skipped        bool
		patternS016    string
		patternS080    string
		delta5to7      [3]int16
		signMatchS016  int
		signMatchS080  int
		differIdxS080  []int
		boundaryOnly   bool
		sumAbsDiffS080 int64
	}

	results := make([]vectorResult, 0, len(vectors))

	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			res := vectorResult{name: v.name}

			bitPath := vectorPath(v.bitFile)
			pstPath := vectorPath(v.pstFile)

			if missing := vectorFilesMissing(bitPath, pstPath); len(missing) > 0 {
				res.skipped = true
				results = append(results, res)
				t.Logf("Phase 0c-3 vector %s SKIPPED — missing files: %v",
					v.name, missing)
				t.Skip()
				return
			}

			frames, bads := readG192Frames(t, bitPath)
			wantFrames := readPSTFrames(t, pstPath)

			// ----- file-size invariant (spec-derivable, READMETV.txt) -----
			// readPSTFrames already enforces multiple-of-160 via Fatalf,
			// but we re-state it here as the explicit hard assertion for
			// the P0c-3 contract.
			if len(wantFrames) == 0 {
				t.Fatalf("vector %s: zero frames in PST (expected ≥ 1 per "+
					"READMETV.txt size lines)", v.name)
			}

			if bads[0] {
				t.Fatalf("vector %s: frame 0 bad-flag set; cannot proceed",
					v.name)
			}

			var d Decoder
			var prod [frameSamples]int16
			if err := d.Decode(frames[0], false, prod[:]); err != nil {
				t.Fatalf("vector %s: Decode frame 0: %v", v.name, err)
			}
			// Defensive: contract is "writes 80 samples" per decode.go.
			if got := len(prod); got != frameSamples {
				t.Fatalf("vector %s: production frame length %d, want %d",
					v.name, got, frameSamples)
			}
			_ = bitstream.FrameBytes // import-pin (reuses production bitstream API)

			want := wantFrames[0]

			// ----- Δ vectors -----
			deltas16 := make([]int16, 16)
			for i := 0; i < 16; i++ {
				deltas16[i] = int16(int32(prod[i]) - int32(want[i]))
			}
			deltas80 := make([]int16, frameSamples)
			for i := 0; i < frameSamples; i++ {
				deltas80[i] = int16(int32(prod[i]) - int32(want[i]))
			}

			res.patternS016 = classifyDeltaPattern(deltas16)
			res.patternS080 = classifyDeltaPattern(deltas80)
			res.delta5to7 = [3]int16{deltas16[5], deltas16[6], deltas16[7]}
			res.signMatchS016 = signMatchCrossVector(prod[:16], want[:16])
			res.signMatchS080 = signMatchCrossVector(prod[:], want[:])
			for i := 0; i < frameSamples; i++ {
				if prod[i] != want[i] {
					res.differIdxS080 = append(res.differIdxS080, i)
					d := int32(prod[i]) - int32(want[i])
					if d < 0 {
						d = -d
					}
					res.sumAbsDiffS080 += int64(d)
				}
			}
			res.boundaryOnly = differIndicesInBoundary(res.differIdxS080)

			t.Logf("──────── vector %s frame 0 ────────", v.name)
			t.Logf("  prod[0..15]=%v", prod[:16])
			t.Logf("  want[0..15]=%v", want[:16])
			t.Logf("  Δ[0..15]   =%v", deltas16)
			t.Logf("  Δ[5..7]    =[%+d %+d %+d]",
				res.delta5to7[0], res.delta5to7[1], res.delta5to7[2])
			t.Logf("  pattern(s0..15) = %s", res.patternS016)
			t.Logf("  pattern(s0..79) = %s", res.patternS080)
			t.Logf("  signMatch(s0..15) = %d/16",
				res.signMatchS016)
			t.Logf("  signMatch(s0..79) = %d/80",
				res.signMatchS080)
			t.Logf("  sumAbsDiff(s0..79) = %d", res.sumAbsDiffS080)
			t.Logf("  differ-from-want (s0..79) count=%d indices=%v",
				len(res.differIdxS080), res.differIdxS080)
			t.Logf("  differ-cluster boundary-only (i<22 OR i≥65) = %v",
				res.boundaryOnly)

			results = append(results, res)
		})
	}

	// ----- final verdict matrix (top-level Logf so it appears once) -----
	t.Logf("──────── Phase 0c-3 verdict matrix ────────")
	t.Logf("| vector  | Δ pat (s0..15)         | Δ s5..7         | sign-match s0..15 | sign-match s0..79 | differ-cluster (s0..79) |")
	t.Logf("|---------|------------------------|-----------------|-------------------|-------------------|-------------------------|")
	for _, r := range results {
		if r.skipped {
			t.Logf("| %-7s | SKIPPED                | -               | -                 | -                 | -                       |",
				r.name)
			continue
		}
		cluster := "mixed/no-pattern"
		if len(r.differIdxS080) == 0 {
			cluster = "EMPTY (Δ=0)"
		} else if r.boundaryOnly {
			cluster = "BOUNDARY-ONLY"
		} else {
			cluster = "MIXED (interior+boundary)"
		}
		t.Logf("| %-7s | %-22s | (%+d,%+d,%+d)      | %2d/16             | %2d/80             | %-23s |",
			r.name,
			r.patternS016,
			r.delta5to7[0], r.delta5to7[1], r.delta5to7[2],
			r.signMatchS016,
			r.signMatchS080,
			cluster,
		)
	}
}

// vectorFilesMissing returns the subset of paths that do not exist on
// disk. Used to drive the per-sub-test t.Skip() branch when a vector
// is unavailable in the local checkout.
func vectorFilesMissing(paths ...string) []string {
	var missing []string
	for _, p := range paths {
		if !fileExists(p) {
			missing = append(missing, p)
		}
	}
	return missing
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// classifyDeltaPattern — Phase 0c-3 classifier. See file header for
// category definitions and order. Operates on a slice (not a fixed-size
// array) so that the same classifier covers both the s[0..15] and
// s[0..79] windows.
func classifyDeltaPattern(deltas []int16) string {
	if len(deltas) == 0 {
		return "zero"
	}
	allZero := true
	for _, d := range deltas {
		if d != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return "zero"
	}

	first := deltas[0]
	uniformConst := true
	for _, d := range deltas {
		if d != first {
			uniformConst = false
			break
		}
	}
	if uniformConst {
		return "sample-uniform-constant"
	}

	const jitterCap = 2
	signUniformJitter := true
	var sign int
	for _, d := range deltas {
		ad := int32(d)
		if ad < 0 {
			ad = -ad
		}
		if ad > jitterCap {
			signUniformJitter = false
			break
		}
		switch {
		case d > 0:
			if sign == 0 {
				sign = +1
			} else if sign != +1 {
				signUniformJitter = false
			}
		case d < 0:
			if sign == 0 {
				sign = -1
			} else if sign != -1 {
				signUniformJitter = false
			}
		}
		if !signUniformJitter {
			break
		}
	}
	if signUniformJitter {
		return "sign-uniform-jitter"
	}

	if len(deltas) == frameSamples {
		boundaryOnly := true
		for i, d := range deltas {
			if d == 0 {
				continue
			}
			if !(i < 22 || i >= 65) {
				boundaryOnly = false
				break
			}
		}
		if boundaryOnly {
			return "boundary-cluster"
		}
	}

	return "random"
}

// differIndicesInBoundary — true iff every index in idxs falls in
// [0..21] ∪ [65..79] (the boundary band derived from the P0c-2
// observation). Empty input returns false (no differ ⇒ "boundary-only"
// is meaningless; caller surfaces "EMPTY" separately).
func differIndicesInBoundary(idxs []int) bool {
	if len(idxs) == 0 {
		return false
	}
	for _, i := range idxs {
		if !(i < 22 || i >= 65) {
			return false
		}
	}
	return true
}

// signMatchCrossVector — sign(0) matches anything; otherwise strict
// sign equality. Mirrors the convention of P0c-2's
// computeSignMatchWantStage but operates on slices so it can serve
// both the 16- and 80-sample windows.
func signMatchCrossVector(stage, want []int16) int {
	n := len(stage)
	if len(want) < n {
		n = len(want)
	}
	matches := 0
	for i := 0; i < n; i++ {
		s := stage[i]
		w := want[i]
		if s == 0 || w == 0 {
			matches++
			continue
		}
		if (s > 0 && w > 0) || (s < 0 && w < 0) {
			matches++
		}
	}
	return matches
}
