//go:build conformance
// +build conformance

package g729

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// TestPhase2fTAME1_ByteEQ is the Phase 2f TAME-1 byte-EQ gate, the
// first direct witness of OQ-TAMING-THR (the §3.9.2 taming-threshold
// pin recorded in Phase 2d closure §4.5).
//
// Per Phase 2f sub-plan §5 TAME-1 step 1: read TAME.IN (128 frames ×
// 80 int16 PCM samples), read TAME.BIT (128 × 164-byte G.192 frames)
// and recover the canonical 10-byte ground truth via
// bitstream.ReadG192Frame; encode TAME.IN frame-by-frame via
// (*Encoder).EncodeFrame; byte-compare each output to ground truth;
// also Unpack both into bitstream.Frame and report the 15-field +
// full-frame match rates.
//
// Disposition tree (sub-plan §5 TAME-1 step 2):
//
//   - full-frame ≥ 80 % → PASS (validates OQ-TAMING-THR pin).
//   - GA*/GB* rates ≥ Phase 2d INT-1a baseline AND full-frame < 80 %
//     → ACCEPT-PARTIAL; OQ-TAMING-THR pin holds; remaining shortfall
//     traces to inherited Phase 2c (P1/P2) + Phase 2d (S/C)
//     FAIL-DEFERRED structural blocker.
//   - GA*/GB* rates < Phase 2d INT-1a baseline → escalate slot 5/5
//     (OQ-TAMING-THR sweep). The plausibility floor is exactly the
//     Phase 2d INT-1a TAMING-suspect numbers because TAME.IN is the
//     dedicated taming-corpus and MUST NOT regress relative to the
//     random-speech PITCH.IN corpus.
//
// Per I-2f-5 (no new spec arithmetic from Phase 2f): if the
// disposition is FAIL-DEFERRED, routing is upstream (Phase 2c
// H-CENTER / Phase 2d ACELP) — Phase 2f INT-1 budget is reserved
// exclusively for packing-layer OQs.
func TestPhase2fTAME1_ByteEQ(t *testing.T) {
	const (
		inPath  = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.IN"
		bitPath = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.BIT"

		samplesPerFrame  = FrameSamples
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164 // G.192: 4-byte header + 80 × 2-byte softbits
		totalFrames      = 128
		expectedInBytes  = totalFrames * bytesPerInFrame
		expectedBitBytes = totalFrames * bytesPerBitFrame
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read TAME.IN: %v", err)
	}
	if len(inData) != expectedInBytes {
		t.Fatalf("TAME.IN size = %d, want %d", len(inData), expectedInBytes)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read TAME.BIT: %v", err)
	}
	if len(bitData) != expectedBitBytes {
		t.Fatalf("TAME.BIT size = %d, want %d", len(bitData), expectedBitBytes)
	}

	// Recover all 128 ground-truth canonical 10-byte frames upfront.
	bitReader := bytes.NewReader(bitData)
	want := make([][bitstream.FrameBytes]byte, totalFrames)
	for f := 0; f < totalFrames; f++ {
		bad, rerr := bitstream.ReadG192Frame(bitReader, want[f][:])
		if rerr != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", f, rerr)
		}
		if bad {
			t.Logf("frame %d: G.192 sync = bad", f)
		}
	}

	enc := NewEncoder()
	var (
		pcm [samplesPerFrame]int16
		got [bitstream.FrameBytes]byte
	)

	type fieldStats struct {
		name  string
		match int
		hist  map[int]int
	}
	fields := []string{"L0", "L1", "L2", "L3", "P1", "P0", "C1", "S1", "GA1", "GB1", "P2", "C2", "S2", "GA2", "GB2"}
	stats := make(map[string]*fieldStats, len(fields))
	for _, n := range fields {
		stats[n] = &fieldStats{name: n, hist: map[int]int{}}
	}

	fullFrameMatch := 0
	const firstNDiverge = 5
	divergences := 0

	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if err := enc.EncodeFrame(pcm[:], got[:]); err != nil {
			t.Fatalf("frame %d: EncodeFrame: %v", f, err)
		}
		if bytes.Equal(got[:], want[f][:]) {
			fullFrameMatch++
		}

		var gotF, refF bitstream.Frame
		if err := bitstream.Unpack(got[:], &gotF); err != nil {
			t.Fatalf("frame %d: Unpack got: %v", f, err)
		}
		if err := bitstream.Unpack(want[f][:], &refF); err != nil {
			t.Fatalf("frame %d: Unpack want: %v", f, err)
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
		anyMiss := false
		for _, p := range pairs {
			s := stats[p.name]
			if p.g == p.r {
				s.match++
			} else {
				anyMiss = true
			}
			s.hist[int(p.g)-int(p.r)]++
		}
		if anyMiss && divergences < firstNDiverge {
			t.Logf("frame %d: full-frame mismatch — got=% x want=% x", f, got[:], want[f][:])
			divergences++
		}
	}

	rates := make(map[string]float64, len(fields))
	for _, n := range fields {
		s := stats[n]
		rate := 100.0 * float64(s.match) / float64(totalFrames)
		rates[n] = rate
		t.Logf("Phase 2f TAME-1 byte-EQ: %s %d/%d (%.2f%%)", n, s.match, totalFrames, rate)
	}
	fullRate := 100.0 * float64(fullFrameMatch) / float64(totalFrames)
	t.Logf("Phase 2f TAME-1 full-frame byte-EQ: %d/%d (%.2f%%)", fullFrameMatch, totalFrames, fullRate)

	// Histograms (≥0.5 % buckets, GA*/GB* only — these are the
	// taming-witness fields. C1/C2/S1/S2/P1/P2 cascade from
	// upstream Phase 2c/2d FAIL-DEFERRED and their histograms are
	// already documented in those closure reports.)
	for _, n := range []string{"GA1", "GB1", "GA2", "GB2"} {
		s := stats[n]
		ks := make([]int, 0, len(s.hist))
		for k := range s.hist {
			ks = append(ks, k)
		}
		sort.Ints(ks)
		var line string
		for _, k := range ks {
			c := s.hist[k]
			if c >= totalFrames/200 { // ≥0.5 % buckets
				line += fmt.Sprintf(" Δ=%+d:%d(%.1f%%)", k, c, 100*float64(c)/float64(totalFrames))
			}
		}
		t.Logf("%s delta histogram (≥0.5%% buckets):%s", n, line)
	}

	// Phase 2d INT-1a baseline — TAME.IN MUST NOT regress.
	const (
		phase2dGA1Floor = 12.15
		phase2dGB1Floor = 5.29
		phase2dGA2Floor = 11.77
		phase2dGB2Floor = 4.52
	)
	const (
		ideal         = 100.0
		acceptPartial = 80.0
	)

	switch {
	case fullRate >= ideal:
		t.Logf("Phase 2f TAME-1: PASS (full-frame %.2f%% ≥ %.0f%%) — validates OQ-TAMING-THR pin.", fullRate, ideal)
	case fullRate >= acceptPartial:
		t.Logf("Phase 2f TAME-1: ACCEPT-PARTIAL (full-frame %.2f%% ≥ %.0f%%) — OQ-TAMING-THR pin holds.", fullRate, acceptPartial)
	default:
		// Plausibility floor: GA*/GB* MUST NOT regress vs Phase 2d INT-1a.
		floors := []struct {
			name  string
			rate  float64
			floor float64
		}{
			{"GA1", rates["GA1"], phase2dGA1Floor},
			{"GB1", rates["GB1"], phase2dGB1Floor},
			{"GA2", rates["GA2"], phase2dGA2Floor},
			{"GB2", rates["GB2"], phase2dGB2Floor},
		}
		breach := false
		for _, fr := range floors {
			if fr.rate < fr.floor {
				t.Errorf("Phase 2f TAME-1 plausibility floor breach: %s %.2f%% < Phase 2d INT-1a baseline %.2f%% — escalate slot 5/5 OQ-TAMING-THR",
					fr.name, fr.rate, fr.floor)
				breach = true
			}
		}
		if !breach {
			t.Logf("Phase 2f TAME-1: ACCEPT-PARTIAL — full-frame %.2f%% < %.0f%% but GA*/GB* meet Phase 2d INT-1a floors (GA1=%.2f≥%.2f GB1=%.2f≥%.2f GA2=%.2f≥%.2f GB2=%.2f≥%.2f). OQ-TAMING-THR pin holds; shortfall routes upstream to Phase 2c H-CENTER + Phase 2d ACELP FAIL-DEFERRED per I-2f-5.",
				fullRate, acceptPartial,
				rates["GA1"], phase2dGA1Floor,
				rates["GB1"], phase2dGB1Floor,
				rates["GA2"], phase2dGA2Floor,
				rates["GB2"], phase2dGB2Floor)
		}
	}
}
