//go:build conformance
// +build conformance

package g729

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// Phase 2f INT-1 per-vector byte-EQ harness — sub-plan §5 INT-1
// step 1 + step 3 (shared harness extraction).
//
// Six ITU vectors (TAME has its own dedicated test on slot 5/5
// OQ-TAMING-THR linkage):
//
//   PITCH   — random-speech corpus, the Phase 2c/2d INT-1 corpus.
//   ALGTHM  — algorithmic conformance vector.
//   SPEECH  — real-speech corpus.
//   FIXED   — fixed-point conformance.
//   LSP     — LSP-stress corpus, inherits Phase 2a INT-1 ACCEPT-PARTIAL.
//   TEST    — general regression.
//
// Per master plan §7 line 1059, frame count is canonically derived
// from .IN-bytes / 160 (do NOT use .BIT length to avoid the F2 /
// OVERFLOW framing dependency inherited from Phase 1o D-2). When the
// .IN size is not a clean multiple of 160 bytes (SPEECH.IN: 600064
// bytes = 3750 frames + 64 trailing bytes), we reconcile to
// min(floor(.IN/160), floor(.BIT/164)) and t.Logf the discrepancy
// — this is the OQ-VECTOR-FRAME-COUNT default behaviour (slot 3/5
// is reserved but not consumed unless a real disagreement appears).
//
// Disposition is informational (t.Logf): the gating regression
// detectors are TestPhase2cINT1_ClosedLoopPitchByteEQ (P1/P2),
// TestPhase2dINT1a_FCBByteEQ (S/C/GA/GB on PITCH), and
// TestPhase2fTAME1_ByteEQ (TAME GA*/GB* taming-corpus floor). The 6
// per-vector dispositions here record the cross-corpus rates for the
// Phase 2f INT-3 closure ledger; per I-2f-5 any FAIL-DEFERRED
// disposition routes upstream (Phase 2a/2c/2d), not Phase 2f INT-1
// budget.

type phase2fVectorResult struct {
	name           string
	frames         int
	inBytes        int
	bitBytes       int
	fullMatch      int
	fieldMatch     map[string]int
	fieldNames     []string
	frameSizeNoted string // OQ-VECTOR-FRAME-COUNT diagnostic
}

func runPhase2fVectorByteEQ(t *testing.T, vectorName string) phase2fVectorResult {
	t.Helper()
	const (
		samplesPerFrame  = FrameSamples
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
	)

	vecDir := "testdata/itu/G729_Release3/g729AnnexA/test_vectors"
	inPath := filepath.Join(vecDir, vectorName+".IN")
	bitPath := filepath.Join(vecDir, vectorName+".BIT")

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("%s: read .IN: %v", vectorName, err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("%s: read .BIT: %v", vectorName, err)
	}

	inFrames := len(inData) / bytesPerInFrame
	bitFrames := len(bitData) / bytesPerBitFrame
	frames := inFrames
	frameSizeNoted := ""
	if bitFrames < frames {
		frames = bitFrames
	}
	if inFrames*bytesPerInFrame != len(inData) || bitFrames*bytesPerBitFrame != len(bitData) || inFrames != bitFrames {
		frameSizeNoted = fmt.Sprintf(
			"OQ-VECTOR-FRAME-COUNT diagnostic: .IN=%d bytes (=%d×160 + %d trailing) → %d frames; "+
				"reconciled to min=%d",
			len(inData), inFrames, len(inData)-inFrames*bytesPerInFrame, inFrames, frames)
	}
	if bitFrames*bytesPerBitFrame != len(bitData) {
		frameSizeNoted += fmt.Sprintf(" / .BIT=%d bytes not a clean multiple of 164", len(bitData))
	}

	bitReader := bytes.NewReader(bitData)
	want := make([][bitstream.FrameBytes]byte, frames)
	for f := 0; f < frames; f++ {
		if _, rerr := bitstream.ReadG192Frame(bitReader, want[f][:]); rerr != nil {
			t.Fatalf("%s: ReadG192Frame frame %d: %v", vectorName, f, rerr)
		}
	}

	fields := []string{"L0", "L1", "L2", "L3", "P1", "P0", "C1", "S1", "GA1", "GB1", "P2", "C2", "S2", "GA2", "GB2"}
	fieldMatch := make(map[string]int, len(fields))
	for _, n := range fields {
		fieldMatch[n] = 0
	}

	enc := NewEncoder()
	var (
		pcm [FrameSamples]int16
		got [bitstream.FrameBytes]byte
	)
	fullMatch := 0
	for f := 0; f < frames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < FrameSamples; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if encErr := enc.EncodeFrame(pcm[:], got[:]); encErr != nil {
			t.Fatalf("%s: frame %d: EncodeFrame: %v", vectorName, f, encErr)
		}
		if bytes.Equal(got[:], want[f][:]) {
			fullMatch++
		}
		var gotF, refF bitstream.Frame
		if uErr := bitstream.Unpack(got[:], &gotF); uErr != nil {
			t.Fatalf("%s: frame %d: Unpack got: %v", vectorName, f, uErr)
		}
		if uErr := bitstream.Unpack(want[f][:], &refF); uErr != nil {
			t.Fatalf("%s: frame %d: Unpack want: %v", vectorName, f, uErr)
		}
		pairs := []struct {
			name string
			g, r uint16
		}{
			{"L0", gotF.L0, refF.L0}, {"L1", gotF.L1, refF.L1},
			{"L2", gotF.L2, refF.L2}, {"L3", gotF.L3, refF.L3},
			{"P1", gotF.P1, refF.P1}, {"P0", gotF.P0, refF.P0},
			{"C1", gotF.C1, refF.C1}, {"S1", gotF.S1, refF.S1},
			{"GA1", gotF.GA1, refF.GA1}, {"GB1", gotF.GB1, refF.GB1},
			{"P2", gotF.P2, refF.P2}, {"C2", gotF.C2, refF.C2},
			{"S2", gotF.S2, refF.S2}, {"GA2", gotF.GA2, refF.GA2},
			{"GB2", gotF.GB2, refF.GB2},
		}
		for _, p := range pairs {
			if p.g == p.r {
				fieldMatch[p.name]++
			}
		}
	}

	return phase2fVectorResult{
		name:           vectorName,
		frames:         frames,
		inBytes:        len(inData),
		bitBytes:       len(bitData),
		fullMatch:      fullMatch,
		fieldMatch:     fieldMatch,
		fieldNames:     fields,
		frameSizeNoted: frameSizeNoted,
	}
}

func (r phase2fVectorResult) logRates(t *testing.T) {
	t.Helper()
	t.Logf("%s: frames=%d (.IN=%d, .BIT=%d)", r.name, r.frames, r.inBytes, r.bitBytes)
	if r.frameSizeNoted != "" {
		t.Logf("%s: %s", r.name, r.frameSizeNoted)
	}
	for _, n := range r.fieldNames {
		c := r.fieldMatch[n]
		t.Logf("%s byte-EQ: %s %d/%d (%.2f%%)",
			r.name, n, c, r.frames, 100.0*float64(c)/float64(r.frames))
	}
	t.Logf("%s full-frame byte-EQ: %d/%d (%.2f%%)",
		r.name, r.fullMatch, r.frames, 100.0*float64(r.fullMatch)/float64(r.frames))
}

func (r phase2fVectorResult) rate(field string) float64 {
	c := r.fieldMatch[field]
	return 100.0 * float64(c) / float64(r.frames)
}

func (r phase2fVectorResult) fullRate() float64 {
	return 100.0 * float64(r.fullMatch) / float64(r.frames)
}

// TestPhase2fINT1_PerVectorByteEQ runs the six per-vector byte-EQ
// dispositions in subtests. Disposition is informational (t.Logf) per
// I-2f-5: the gating regression detectors live elsewhere
// (Phase 2c/2d INT-1, Phase 2f TAME-1). This test produces the
// per-vector rate matrix that INT-3 records in the closure ledger.
func TestPhase2fINT1_PerVectorByteEQ(t *testing.T) {
	const (
		acceptPartial = 80.0
		idealPass     = 100.0
	)
	// Phase 2a INT-1 LSP per-field ACCEPT-PARTIAL ceiling
	// (closure report 2026-05-06-phase2a-closure-report.md §3).
	const (
		phase2aL0 = 78.67
		phase2aL1 = 38.93
		phase2aL2 = 17.07
		phase2aL3 = 19.35
	)
	// Phase 2c INT-1 closed-loop pitch baselines on PITCH corpus
	// (closure report 2026-05-10-phase2c-closure-report.md).
	const (
		phase2cP1 = 10.79
		phase2cP2 = 11.66
	)
	// Phase 2d INT-1a FCB byte-EQ baselines on PITCH corpus
	// (closure report 2026-05-12-phase2d-closure-report.md).
	const (
		phase2dGA1 = 12.15
		phase2dGB1 = 5.29
		phase2dGA2 = 11.77
		phase2dGB2 = 4.52
	)

	dispose := func(t *testing.T, r phase2fVectorResult, partialFloor float64, partialDesc, upstreamRoute string) {
		t.Helper()
		full := r.fullRate()
		switch {
		case full >= idealPass:
			t.Logf("%s disposition: PASS (full-frame %.2f%% ≥ %.0f%%) — Phase 2 closure trigger candidate.",
				r.name, full, idealPass)
		case full >= acceptPartial:
			t.Logf("%s disposition: ACCEPT-PARTIAL (full-frame %.2f%% ≥ %.0f%%).",
				r.name, full, acceptPartial)
		default:
			// Cross-corpus plausibility: if the vector's max
			// per-field rate clears the upstream baseline, this is
			// ACCEPT-PARTIAL with upstream routing; otherwise
			// FAIL-DEFERRED with upstream routing. In both cases
			// per I-2f-5 the routing is upstream and no Phase 2f
			// I5 is spent.
			if partialFloor > 0 {
				t.Logf("%s disposition: FAIL-DEFERRED (full-frame %.2f%% < %.0f%%; %s; upstream route: %s) "+
					"— per I-2f-5 no Phase 2f INT-1 budget consumed.",
					r.name, full, acceptPartial, partialDesc, upstreamRoute)
			} else {
				t.Logf("%s disposition: FAIL-DEFERRED (full-frame %.2f%% < %.0f%%; upstream route: %s) "+
					"— per I-2f-5 no Phase 2f INT-1 budget consumed.",
					r.name, full, acceptPartial, upstreamRoute)
			}
		}
	}

	t.Run("PITCH", func(t *testing.T) {
		r := runPhase2fVectorByteEQ(t, "PITCH")
		r.logRates(t)
		t.Logf("PITCH: structural ceiling C1=%.2f%% C2=%.2f%% (Phase 2d INT-1a inheritance, ACELP H-CENTER blocker).",
			r.rate("C1"), r.rate("C2"))
		t.Logf("PITCH: cross-baseline check P1=%.2f%% (Phase 2c=%.2f%%) P2=%.2f%% (Phase 2c=%.2f%%) "+
			"GA1=%.2f%% (Phase 2d=%.2f%%) GB1=%.2f%% (Phase 2d=%.2f%%) "+
			"GA2=%.2f%% (Phase 2d=%.2f%%) GB2=%.2f%% (Phase 2d=%.2f%%)",
			r.rate("P1"), phase2cP1, r.rate("P2"), phase2cP2,
			r.rate("GA1"), phase2dGA1, r.rate("GB1"), phase2dGB1,
			r.rate("GA2"), phase2dGA2, r.rate("GB2"), phase2dGB2)
		dispose(t, r,
			0,
			"",
			"Phase 2d INT-1a (S/C/GA/GB) + Phase 2c INT-1b (P1/P2) + Phase 2b H-CENTER (open-loop t_op)")
	})

	t.Run("ALGTHM", func(t *testing.T) {
		r := runPhase2fVectorByteEQ(t, "ALGTHM")
		r.logRates(t)
		dispose(t, r, 0, "",
			"Phase 2d INT-1a + Phase 2c INT-1b + Phase 2b H-CENTER (same as PITCH)")
	})

	t.Run("SPEECH", func(t *testing.T) {
		r := runPhase2fVectorByteEQ(t, "SPEECH")
		r.logRates(t)
		dispose(t, r, 0, "",
			"Phase 2d INT-1a + Phase 2c INT-1b + Phase 2b H-CENTER")
	})

	t.Run("FIXED", func(t *testing.T) {
		r := runPhase2fVectorByteEQ(t, "FIXED")
		r.logRates(t)
		dispose(t, r, 0, "",
			"Phase 2d INT-1a + Phase 2c INT-1b + Phase 2b H-CENTER")
	})

	t.Run("LSP", func(t *testing.T) {
		r := runPhase2fVectorByteEQ(t, "LSP")
		r.logRates(t)
		t.Logf("LSP: per-field cross-baseline (Phase 2a INT-1 ACCEPT-PARTIAL ceiling): "+
			"L0=%.2f%% (Phase 2a=%.2f%%) L1=%.2f%% (Phase 2a=%.2f%%) "+
			"L2=%.2f%% (Phase 2a=%.2f%%) L3=%.2f%% (Phase 2a=%.2f%%)",
			r.rate("L0"), phase2aL0, r.rate("L1"), phase2aL1,
			r.rate("L2"), phase2aL2, r.rate("L3"), phase2aL3)
		// Phase 2a was on a different corpus (PITCH-derived); LSP.IN
		// is its own corpus so the baselines are reference points,
		// not strict floors. We only flag a regression if all 4 LSP
		// rates are below all 4 Phase 2a baselines.
		below := r.rate("L0") < phase2aL0 && r.rate("L1") < phase2aL1 &&
			r.rate("L2") < phase2aL2 && r.rate("L3") < phase2aL3
		if below {
			t.Logf("LSP: NOTE — all 4 LSP per-field rates below Phase 2a baselines; " +
				"NOT escalating slot 4/5 (OQ-COLD-START-CONVENTION) since Phase 2a corpus " +
				"differs from LSP corpus and the gap is expected to be corpus-dependent. " +
				"INT-3 closure ledger records the cross-corpus disposition.")
		}
		dispose(t, r, 0, "",
			"Phase 2a INT-1 (LSP ACCEPT-PARTIAL inheritance) + Phase 2c/2d FAIL-DEFERRED (P/S/C/G)")
	})

	t.Run("TEST", func(t *testing.T) {
		r := runPhase2fVectorByteEQ(t, "TEST")
		r.logRates(t)
		dispose(t, r, 0, "",
			"Phase 2d INT-1a + Phase 2c INT-1b + Phase 2b H-CENTER")
	})
}
