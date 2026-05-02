package bitstream

// =============================================================================
// Phase 1o D-2 — OVERFLOW.BIT loader diagnostic survey (measurement only).
// =============================================================================
//
// Plan: docs/superpowers/plans/2026-05-09-phase1o-decoder-domain-closure-plan.md
//
// Goal: characterize precisely *why* `ReadG192File` rejects
// `testdata/itu/G729_Release3/g729AnnexA/test_vectors/OVERFLOW.BIT`,
// the ITU G.729A test vector designed to exercise §3.10 synthesis-filter
// saturation recovery. The decode-side skip in
// internal/decoder/decode_test.go (TestDecode_ITUVectorOverflowBitExact,
// line ~531) reads verbatim:
//
//   t.Skip("Phase 1h INCOMPLETE: OVERFLOW.BIT fails G.192 parsing in " +
//   "internal/bitstream's ReadG192File with 'invalid G.192 data " +
//   "word'; pre-existing issue independent of Phase 1h. Even " +
//   "with a successful load, the same postfilter/HP-filter " +
//   "divergence as the other vectors would apply. Phase 1i " +
//   "should: (1) reverse-engineer the OVERFLOW.BIT framing " +
//   "variation, (2) once loadable, validate that the §3.10 " +
//   "recovery branch is exercised correctly.")
//
// The pre-cycle exploration note that frame 19 contains a "byte-swapped sync
// 0x216b" was based on a hex-dump misread (the bytes `21 6b` are simply the
// little-endian encoding of canonical `0x6B21`). This test surveys the file
// directly to replace that hypothesis with measured ground truth.
//
// -----------------------------------------------------------------------------
// MEASUREMENT REPORT (populated from the executed subtests below)
// -----------------------------------------------------------------------------
//
// File size  : 62976 bytes = 384 frames × 164 bytes (G192FrameBytes).
// Sync words : 384 / 384 frames carry the canonical good-frame sync
//              0x6B21 (little-endian). 0 frames carry 0x6B20 (bad). 0 frames
//              carry any byte-swapped or other sync value.
// Length word: 384 / 384 frames carry length = 80 (= FrameBits).
// Bit words  : Across all 30 720 bit-words (384 × 80), the only values
//              observed are 0x007F (~17 505), 0x0081 (~13 135), and 0x0000
//              (exactly 80, ALL concentrated in a single frame). No other
//              values appear.
// Anomaly    : Exactly ONE frame — index 19 (zero-based, byte offset 3116) —
//              has all 80 data-words = 0x0000 while still carrying the
//              canonical 0x6B21 sync and length=80. Every other frame is
//              bit-perfectly G.192-conformant per the strict reader.
//
// Frame 19 raw 164-byte hex (sync, length, then 80 zero data-words):
//   216b 5000 0000 0000 ... 0000   (16-bit LE words)
//
// G.192 spec cross-reference
// --------------------------
// The repository ships ITU-T G.729 (G729E.pdf/.txt) only — no G.191/G.192
// document is present under docs/superpowers/specs/. From the conventions
// already encoded in this package's doc.go and READMETV.txt, the canonical
// per-bit softbit values are:
//   0x0081 → source bit '1'
//   0x007F → source bit '0'
//   0x6B21 → good-frame sync   (frame_start)
//   0x6B20 → bad-frame  sync   (frame_erasure)
// G.191 STL (per the broader ITU softbit convention referenced in textbook
// treatments and STL2009 source documentation) additionally defines
// 0x0000 as the "indeterminate" / "unknown" softbit — a placeholder used
// by tools that propagate erasure information at sub-frame granularity, or
// when the encoder-side test harness intentionally emits a degenerate
// payload. We cannot quote chapter-and-verse without the G.191 PDF, so
// this interpretation is recorded as informed inference, not citation.
//
// HYPOTHESIS RANKING (post-measurement)
// -------------------------------------
//   V1 — "entire file is byte-swapped"           : REFUTED.
//        All 384 sync words read as 0x6B21 in little-endian; no frame
//        reads as 0x6B21 only when interpreted big-endian.
//
//   V2 — "mixed-endianness, some frames swapped" : REFUTED.
//        Sync histogram is {0x6B21: 384}. No second cluster.
//
//   V3 — "0x216B is a documented marker"         : REFUTED / N/A.
//        0x216B never appears as a sync value once the bytes are read in
//        little-endian as the writer side does. The earlier diagnosis
//        was a hex-dump artifact.
//
//   V4 — "loader-side scope gap: file is structurally valid in an
//         interpretation we don't yet handle"   : SURVIVING (HIGH confidence).
//        The file is fully G.192-framed (sync + length all canonical).
//        The single anomaly is that frame 19 carries 80 data-words of
//        0x0000 — neither 0x007F nor 0x0081 — which the strict reader
//        rejects via ErrBadG192Bit. Two sub-variants:
//          V4a — 0x0000 is a documented "indeterminate softbit" per
//                G.191 STL convention. The loader should accept it
//                (typically by treating it as logical 0, OR by surfacing
//                a per-bit "indeterminate" flag), preserving the G.192
//                framing contract.
//          V4b — 0x0000 is a vendor-/test-harness-specific marker for
//                "frame intentionally zero-content to drive the
//                §3.10 saturation-recovery path", and should be mapped
//                to all-zero source bits at the decoder's input.
//        Both sub-variants converge on the same byte-level decode of
//        frame 19: 80 zero source bits → an all-zero packed frame
//        (FrameBytes of 0x00). They differ only in *where* the
//        permissive mapping lives (loader vs. test-only normaliser).
//
// PROPOSED FIX VARIANTS  (for user gate G-D2 — production change deferred)
// ------------------------------------------------------------------------
//   F1 — Lenient core reader.
//        Extend ReadG192Frame to treat 0x0000 as logical 0 (alongside
//        0x007F → 0). Smallest diff, but relaxes the strict G.192
//        contract for every caller (including encoder round-trip tests).
//
//   F2 — Variant constructor / option.
//        Add a ReadG192FrameLenient (or a `LenientBits bool` option)
//        path that accepts 0x0000 as logical 0; ReadG192File for ITU
//        test vectors uses it, the strict reader stays strict.
//        Cleanest API separation; keeps round-trip semantics intact.
//
//   F3 — Test-only normaliser.
//        Leave bitstream package untouched. In the decoder test harness,
//        pre-process OVERFLOW.BIT bytes (rewrite each 0x0000 word in
//        place to 0x007F) before handing the buffer to ReadG192File.
//        Zero blast radius on production code, but encodes the workaround
//        outside the loader where it is least discoverable.
//
// RECOMMENDED USER GATE G-D2 QUESTION
// -----------------------------------
//   "OVERFLOW.BIT contains a single frame (#19) whose 80 data-words are
//    0x0000 instead of the G.192 0x007F/0x0081 markers. All other framing
//    is canonical. Pick the production fix:
//      (a) F1 — loosen the core ReadG192Frame to accept 0x0000 ≡ 0;
//      (b) F2 — add a lenient variant (ReadG192FrameLenient) and have
//               ReadG192File / the ITU test harness use it;
//      (c) F3 — leave the loader strict; normalise 0x0000 → 0x007F in
//               the OVERFLOW.BIT decoder test harness only."
// =============================================================================

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const (
	phase1oD2OverflowRelPath = "../../testdata/itu/G729_Release3/g729AnnexA/test_vectors/OVERFLOW.BIT"
	phase1oD2ExpectedFrames  = 384
	phase1oD2ExpectedBytes   = phase1oD2ExpectedFrames * G192FrameBytes // 62976
)

func phase1oD2LoadOverflow(t *testing.T) []byte {
	t.Helper()
	abs, err := filepath.Abs(phase1oD2OverflowRelPath)
	if err != nil {
		t.Fatalf("resolve OVERFLOW.BIT path: %v", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read OVERFLOW.BIT: %v", err)
	}
	return data
}

func TestPhase1o_D2_OverflowDiagnostic(t *testing.T) {
	data := phase1oD2LoadOverflow(t)

	t.Run("Subtest_FileSize", func(t *testing.T) {
		if got := len(data); got != phase1oD2ExpectedBytes {
			t.Fatalf("OVERFLOW.BIT size = %d bytes, want %d (= %d frames × %d bytes)",
				got, phase1oD2ExpectedBytes, phase1oD2ExpectedFrames, G192FrameBytes)
		}
		if len(data)%G192FrameBytes != 0 {
			t.Fatalf("size not a whole-frame multiple: %d %% %d = %d",
				len(data), G192FrameBytes, len(data)%G192FrameBytes)
		}
		t.Logf("OK: %d bytes = %d frames × %d bytes",
			len(data), phase1oD2ExpectedFrames, G192FrameBytes)
	})

	t.Run("Subtest_FrameSyncSurvey", func(t *testing.T) {
		var canonical, swapped, bad, other int
		otherSyncs := map[uint16]int{}
		for i := 0; i < phase1oD2ExpectedFrames; i++ {
			off := i * G192FrameBytes
			s := binary.LittleEndian.Uint16(data[off : off+2])
			switch s {
			case G192SyncGood: // 0x6B21
				canonical++
			case G192SyncBad: // 0x6B20
				bad++
			case 0x216B: // hypothesised byte-swapped good
				swapped++
			default:
				other++
				otherSyncs[s]++
			}
		}
		t.Logf("sync histogram: canonical(0x6B21)=%d swappedHypoth(0x216B)=%d badFrame(0x6B20)=%d other=%d",
			canonical, swapped, bad, other)
		if len(otherSyncs) > 0 {
			t.Logf("other sync values: %v", otherSyncs)
		}
		if canonical != phase1oD2ExpectedFrames {
			t.Errorf("expected all %d frames canonical 0x6B21, got %d",
				phase1oD2ExpectedFrames, canonical)
		}
		if swapped != 0 {
			t.Errorf("byte-swapped-sync hypothesis: expected 0 frames, got %d (REFUTES V1/V2/V3)", swapped)
		}
	})

	t.Run("Subtest_FrameLengthSurvey", func(t *testing.T) {
		lengths := map[uint16]int{}
		for i := 0; i < phase1oD2ExpectedFrames; i++ {
			off := i * G192FrameBytes
			L := binary.LittleEndian.Uint16(data[off+2 : off+4])
			lengths[L]++
		}
		t.Logf("length-word histogram: %v", lengths)
		if lengths[FrameBits] != phase1oD2ExpectedFrames {
			t.Errorf("expected all %d length-words = %d (FrameBits); got hist %v",
				phase1oD2ExpectedFrames, FrameBits, lengths)
		}
	})

	t.Run("Subtest_BitWordSurveyPostSwapped", func(t *testing.T) {
		// Per-frame and global histogram of the 80 data-words.
		globalHist := map[uint16]int{}
		var anomFrames []int
		for i := 0; i < phase1oD2ExpectedFrames; i++ {
			off := i * G192FrameBytes
			anom := false
			for j := 0; j < FrameBits; j++ {
				w := binary.LittleEndian.Uint16(data[off+4+2*j : off+6+2*j])
				globalHist[w]++
				if w != G192Bit0 && w != G192Bit1 {
					anom = true
				}
			}
			if anom {
				anomFrames = append(anomFrames, i)
			}
		}
		t.Logf("global bit-word histogram (LE): 0x007F=%d 0x0081=%d 0x0000=%d others=%d",
			globalHist[G192Bit0], globalHist[G192Bit1], globalHist[0x0000],
			func() int {
				total := 0
				for k, v := range globalHist {
					if k != G192Bit0 && k != G192Bit1 && k != 0x0000 {
						total += v
					}
				}
				return total
			}())
		t.Logf("frames containing non-canonical bit-words: count=%d indices=%v",
			len(anomFrames), anomFrames)

		// For each anomalous frame, dump first 16 data-words.
		for _, idx := range anomFrames {
			off := idx * G192FrameBytes
			words := make([]string, 16)
			for j := 0; j < 16; j++ {
				w := binary.LittleEndian.Uint16(data[off+4+2*j : off+6+2*j])
				words[j] = fmt.Sprintf("0x%04X", w)
			}
			// Verify all 80 words match the same value pattern.
			zeroCount := 0
			for j := 0; j < FrameBits; j++ {
				w := binary.LittleEndian.Uint16(data[off+4+2*j : off+6+2*j])
				if w == 0x0000 {
					zeroCount++
				}
			}
			t.Logf("frame %d: first16=%v zeroWordCount=%d/%d",
				idx, words, zeroCount, FrameBits)
		}
	})

	t.Run("Subtest_FrameAt19_ByteDump", func(t *testing.T) {
		const idx = 19
		off := idx * G192FrameBytes
		frame := data[off : off+G192FrameBytes]
		t.Logf("frame %d (offset %d) raw %d-byte hex:\n%x",
			idx, off, G192FrameBytes, frame)
		sync := binary.LittleEndian.Uint16(frame[0:2])
		length := binary.LittleEndian.Uint16(frame[2:4])
		t.Logf("frame %d: LE sync=0x%04X (canonical=0x%04X) length=%d (FrameBits=%d)",
			idx, sync, G192SyncGood, length, FrameBits)
		if sync != G192SyncGood {
			t.Errorf("expected canonical sync 0x6B21 at frame 19; got 0x%04X", sync)
		}
		if length != FrameBits {
			t.Errorf("expected length=%d at frame 19; got %d", FrameBits, length)
		}
		// Confirm all 80 data-words are 0x0000.
		nonZero := 0
		for j := 0; j < FrameBits; j++ {
			w := binary.LittleEndian.Uint16(frame[4+2*j : 6+2*j])
			if w != 0x0000 {
				nonZero++
			}
		}
		if nonZero != 0 {
			t.Errorf("expected all 80 data-words = 0x0000 at frame 19; %d non-zero", nonZero)
		} else {
			t.Logf("frame 19 confirmed: all 80 data-words = 0x0000 (ErrBadG192Bit trigger)")
		}
	})

	t.Run("Subtest_LoaderFailureReproduction", func(t *testing.T) {
		_, _, err := ReadG192File(bytes.NewReader(data))
		if err == nil {
			t.Fatalf("expected ReadG192File to reject OVERFLOW.BIT; got nil")
		}
		if !errors.Is(err, ErrBadG192Bit) {
			t.Logf("loader error: %v (expected ErrBadG192Bit)", err)
		} else {
			t.Logf("loader error confirmed: %v", err)
		}

		// Also confirm at exactly which frame index the strict reader trips.
		r := bytes.NewReader(data)
		buf := make([]byte, FrameBytes)
		successful := 0
		for {
			_, ferr := ReadG192Frame(r, buf)
			if ferr == io.EOF {
				break
			}
			if ferr != nil {
				t.Logf("ReadG192Frame trips at frame index %d with: %v", successful, ferr)
				break
			}
			successful++
		}
		if successful != 19 {
			t.Logf("strict loader accepted %d frames before tripping (want 19)", successful)
		} else {
			t.Logf("strict loader accepted exactly %d frames before tripping (matches frame-19 anomaly)", successful)
		}
	})

	t.Run("Subtest_G192SpecCrossref", func(t *testing.T) {
		t.Log("G.191/G.192 spec PDF not present under docs/superpowers/specs/.")
		t.Log("Conventions inferred from this package's doc.go + READMETV.txt:")
		t.Log("  0x6B21 sync = good frame; 0x6B20 sync = bad frame (erasure).")
		t.Log("  0x0081 / 0x007F = source bit 1 / 0.")
		t.Log("  0x0000 — broader ITU G.191 STL convention treats this as the")
		t.Log("  'indeterminate softbit' marker; not currently accepted by ReadG192Frame.")
		t.Log("  This interpretation is informed inference, NOT a chapter-and-verse citation.")
	})

	t.Run("Subtest_HypothesisRanking", func(t *testing.T) {
		t.Log("V1 entirely-byte-swapped : REFUTED (all 384 syncs read 0x6B21 LE).")
		t.Log("V2 mixed-endianness      : REFUTED (sync histogram has a single bin).")
		t.Log("V3 0x216B documented marker : REFUTED/N-A (0x216B never appears as a sync).")
		t.Log("V4 loader-side scope gap : SURVIVING (HIGH confidence).")
		t.Log("    Frame 19 is fully G.192-framed but uses 0x0000 data-words.")
		t.Log("    Fix variants: F1 lenient core / F2 lenient variant / F3 test-only normaliser.")
		t.Log("    Recommended user gate G-D2: pick F1 / F2 / F3 (see file-level report).")
	})
}
