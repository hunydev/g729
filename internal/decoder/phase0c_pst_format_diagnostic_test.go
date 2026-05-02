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
	"encoding/binary"
	"os"
	"testing"
)

// TestDiagnostic_Phase0c1PstFormatTrace — measurement-only diagnostic for
// Phase 0c-1 (cycle P0c-reentry, plan
// docs/superpowers/plans/2026-05-05-phase0c-reentry-want-domain-reinterpret-plan.md
// Task 1).
//
// Purpose: re-verify, against verbatim spec citations, the four-dimensional
// format assumption (byte-order / header / frame-count / unit) that
// readPSTFrames(internal/decoder/testdata_helpers_test.go) currently makes
// for Annex A *.PST test vectors. Production = 0 lines changed (E2).
//
// READMETV.txt verbatim quotations (cited from
// testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt, the only
// authoritative source describing the *.PST file format under invariant E1):
//
//	"Format: all files contain 16 bit sampled data using the Intel (PC)
//	 format."
//	"*.out  - output files"
//	"  decoder file.bit file.pst"
//	"    5600  algthm.pst"
//
// E4 ambiguity (explicitly noted, must NOT be cherry-picked away):
//   - README states the bit-width (16) and the byte-order (Intel = little-
//     endian on the PC) but is SILENT on:
//     (a) the Q-format / scaling unit of each 16-bit sample (Q0 raw int16
//     vs Q15 normalized fractional vs ×2 pre-scaled),
//     (b) the sign convention (signed two's-complement vs unsigned),
//     (c) the chain stage that produced the *.pst file (decoder output is
//     stated, but post-processing inclusion is not spelled out).
//
// PDF §A.4.2.5 verbatim quotation (extracted via `pdftotext -layout` from
// docs/superpowers/specs/itu/G729E.pdf, lines 2292–2293):
//
//	"A.4.2.5    High-pass filtering and upscaling"
//	"Same as described in clause 4.2.5."
//
// PDF §4.2.5 verbatim quotation (lines 1687–1693, the upstream definition
// that A.4.2.5 inherits by reference):
//
//	"4.2.5    High-pass filtering and upscaling
//	 A high-pass filter with a cut-off frequency of 100 Hz is applied to
//	 the reconstructed postfiltered speech sf'(n). The filter is given by:
//	     H_h2(z) = (0.93980581 - 1.8795834 z^-1 + 0.93980581 z^-2)
//	             / (1 - 1.9330735 z^-1 + 0.93589199 z^-2)
//	 The filtered signal is multiplied by a factor 2 to restore the input
//	 signal level."
//
// Note on extractability: PDF §A.4.2.5 quote was successfully extracted via
// `pdftotext -layout` (binary PDF parsed without external tooling beyond
// the system pdftotext utility). The §A.4.2.5 paragraph itself defers
// entirely to §4.2.5, which establishes the ×2 multiplier semantics ("to
// restore the input signal level") — this is consistent with a unit
// hypothesis where the PST file holds Q0 raw int16 samples already scaled
// by ×2 relative to the synthesis IIR output magnitude. However neither
// §4.2.5 nor §A.4.2.5 explicitly names the Q-format of the resulting
// stream; "input signal level" is qualitative. Therefore the unit
// dimension below remains UNDETERMINED per the plan's polarity expectation.
func TestDiagnostic_Phase0c1PstFormatTrace(t *testing.T) {
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, pstPath)

	data, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%s): %v", pstPath, err)
	}

	// Hard-asserted invariants derived from cited spec text:
	//   - len > 0 (file non-empty)
	//   - len == 5600 (READMETV.txt: "  5600  algthm.pst")
	//   - len % 160 == 0 (no leaked header bytes; 160 = 80 samples × 2 bytes)
	//   - len/160 == 35 (matches 5600/160 = 35 frames at 10 ms/frame)
	if len(data) == 0 {
		t.Fatalf("ALGTHM.PST is empty")
	}
	if len(data) != 5600 {
		t.Fatalf("len(ALGTHM.PST) = %d, want 5600 (READMETV.txt)", len(data))
	}
	if len(data)%(frameSamples*2) != 0 {
		t.Fatalf("len(ALGTHM.PST) = %d not divisible by %d (frame*2)",
			len(data), frameSamples*2)
	}
	frameCount := len(data) / (frameSamples * 2)
	if frameCount != 35 {
		t.Fatalf("frameCount = %d, want 35 (READMETV.txt)", frameCount)
	}

	// Byte-level invariant: bytes covering frame 0 sample 5..7 (offsets
	// 10..15, 6 bytes) under little-endian decoding must be ff ff ff ff
	// ff ff (this was established by cycle F-oct-postfix2-prelim-3 / M5,
	// commit cb9529d).
	wantSample57Bytes := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	gotSample57Bytes := data[10:16]
	if !bytesEq(gotSample57Bytes, wantSample57Bytes) {
		t.Fatalf("frame0 sample 5..7 bytes = % x, want % x (M5 commit cb9529d)",
			gotSample57Bytes, wantSample57Bytes)
	}

	// First 32 bytes (covers frame 0 sample 0..15).
	t.Logf("first32_bytes_hex = % x", data[:32])
	// Last 8 bytes (covers frame 34 sample 76..79).
	t.Logf("last8_bytes_hex   = % x", data[len(data)-8:])
	t.Logf("file_length        = %d bytes", len(data))
	t.Logf("frame_count        = %d (len/160; 160 = 80 samples * 2 bytes)",
		frameCount)

	// First 16 samples decoded as little-endian int16 (the README "Intel
	// (PC) format" hypothesis — what readPSTFrames currently uses).
	leSamples := decodeFirst16(data, binary.LittleEndian)
	t.Logf("first16_samples_le = %v", leSamples)
	// First 16 samples decoded as big-endian int16 (control / null
	// hypothesis — would indicate README is being misread).
	beSamples := decodeFirst16(data, binary.BigEndian)
	t.Logf("first16_samples_be = %v", beSamples)

	// Sample 5..7 under three unit hypotheses.
	q0 := leSamples[5:8] // Q0 raw int16
	t.Logf("sample5..7_unit_Q0_raw_int16        = %v", q0)

	q15 := [3]float64{
		float64(leSamples[5]) / 32768.0,
		float64(leSamples[6]) / 32768.0,
		float64(leSamples[7]) / 32768.0,
	}
	t.Logf("sample5..7_unit_Q15_normalized_f64  = [%g %g %g]",
		q15[0], q15[1], q15[2])

	// ×2 pre-scaled hypothesis: invert the §4.2.5 ×2 multiplier to
	// recover the pre-multiplier magnitude.
	x2recover := [3]float64{
		float64(leSamples[5]) / 2.0,
		float64(leSamples[6]) / 2.0,
		float64(leSamples[7]) / 2.0,
	}
	t.Logf("sample5..7_unit_x2_prescaled_recov  = [%g %g %g]",
		x2recover[0], x2recover[1], x2recover[2])

	verdict := classifyPstFormat(data, leSamples)
	t.Logf("classifyPstFormat verdict 4-tuple:")
	t.Logf("  byte-order  = %s", verdict.byteOrder)
	t.Logf("  header      = %s", verdict.header)
	t.Logf("  frame-count = %s", verdict.frameCount)
	t.Logf("  unit        = %s", verdict.unit)
	t.Logf("4-tuple summary: [byte-order=%s header=%s frame-count=%s unit=%s]",
		verdict.byteOrder, verdict.header, verdict.frameCount, verdict.unit)
}

// pstFormatVerdict holds the 4-dimensional EQ/NE/UNDETERMINED classification
// from classifyPstFormat. Strings are restricted to {"EQ","NE","UNDETERMINED"}
// per invariant E4 (binary verdict, no hedging — UNDETERMINED is reserved for
// dimensions where the cited spec text is silent).
type pstFormatVerdict struct {
	byteOrder  string
	header     string
	frameCount string
	unit       string
}

// classifyPstFormat applies the four format-dimension classifiers described
// in the plan (Phase 2 Task 1).
//
// Sanity threshold for byte-order: PCM speech samples after the §4.2.5
// post-processing chain must lie within the int16 range (always true by
// construction) AND have a magnitude distribution consistent with audio
// (here we use |sample| < 30000 for at least 14 of the first 16 samples
// as a loose audio-plausibility floor). This threshold is documented and
// is not derived from any external G.729 implementation — it is a
// generic 16-bit-PCM sanity check.
func classifyPstFormat(data []byte, leSamples [16]int16) pstFormatVerdict {
	v := pstFormatVerdict{
		byteOrder:  "UNDETERMINED",
		header:     "UNDETERMINED",
		frameCount: "UNDETERMINED",
		unit:       "UNDETERMINED",
	}

	// byte-order: README says "Intel (PC) format" → little-endian. EQ if
	// little-endian decoding produces audio-plausible magnitudes.
	saneCount := 0
	for _, s := range leSamples {
		abs := int32(s)
		if abs < 0 {
			abs = -abs
		}
		if abs < 30000 {
			saneCount++
		}
	}
	if saneCount >= 14 {
		v.byteOrder = "EQ"
	} else {
		v.byteOrder = "NE"
	}

	// header: EQ if no header bytes (file size is an exact multiple of
	// frame*2 = 160 bytes).
	if len(data)%(frameSamples*2) == 0 {
		v.header = "EQ"
	} else {
		v.header = "NE"
	}

	// frame-count: EQ if len/160 == 35 (matches READMETV.txt "5600
	// algthm.pst" at 10 ms/frame, 8 kHz, int16).
	if len(data)/(frameSamples*2) == 35 {
		v.frameCount = "EQ"
	} else {
		v.frameCount = "NE"
	}

	// unit: README is silent on Q-format / sign convention. PDF §A.4.2.5
	// defers entirely to §4.2.5 ("Same as described in clause 4.2.5.").
	// §4.2.5 establishes the ×2 multiplier semantics ("to restore the
	// input signal level") but does NOT explicitly name the Q-format of
	// the resulting stream — "input signal level" is qualitative.
	// Therefore unit dimension is UNDETERMINED per E4 (cherry-pick
	// avoidance: must not infer Q0/Q15 from silence).
	v.unit = "UNDETERMINED"

	return v
}

func decodeFirst16(data []byte, order binary.ByteOrder) [16]int16 {
	var out [16]int16
	for i := 0; i < 16; i++ {
		out[i] = int16(order.Uint16(data[i*2 : i*2+2]))
	}
	return out
}

func bytesEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
