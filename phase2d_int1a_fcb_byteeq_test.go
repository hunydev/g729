package g729

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"testing"
)

// extractFCBBitsFromG192 reads the FCB-side and gain fields of one
// G.192-framed PITCH.BIT frame.
//
// Per ITU-T G.729 Table 8 (matching internal/bitstream/pack.go) the
// in-frame bit layout (zero-indexed across the 80-bit payload, MSB
// of each field first) is:
//
//	L0(1) L1(7) L2(5) L3(5) P1(8) P0(1) C1(13) S1(4) GA1(3) GB1(4)
//	P2(5) C2(13) S2(4) GA2(3) GB2(4)
//
// so the FCB+gain fields land at:
//
//	C1 :27..39   S1 :40..43   GA1:44..46   GB1:47..50
//	C2 :56..68   S2 :69..72   GA2:73..75   GB2:76..79
//
// G.192 frame layout is a 4-byte header (sync 0x6B21 + length
// 0x0050) followed by 80 × 2-byte bit-words (0x007F = 0,
// 0x0081 = 1).
func extractFCBBitsFromG192(frame []byte) (c1, s1, ga1, gb1, c2, s2, ga2, gb2 uint16) {
	const g192Bit1 uint16 = 0x0081
	getBit := func(idx int) uint16 {
		off := 4 + 2*idx
		if binary.LittleEndian.Uint16(frame[off:off+2]) == g192Bit1 {
			return 1
		}
		return 0
	}
	getField := func(start, n int) uint16 {
		var v uint16
		for i := 0; i < n; i++ {
			v = (v << 1) | getBit(start+i)
		}
		return v
	}
	c1 = getField(27, 13)
	s1 = getField(40, 4)
	ga1 = getField(44, 3)
	gb1 = getField(47, 4)
	c2 = getField(56, 13)
	s2 = getField(69, 4)
	ga2 = getField(73, 3)
	gb2 = getField(76, 4)
	return
}

// TestPhase2dINT1a_FCBByteEQ is the Phase 2d INT-1a STRICT byte-EQ
// gate. It drives the encoder LPC + open-loop + closed-loop pitch +
// fixed-codebook + gain-quantization chain
// (lpcStep → openloopStep → closedloopStep → fcbStep, ×2) on every
// frame of the ITU PITCH.IN test vector, extracts the emitted
// (S1,C1,GA1,GB1,S2,C2,GA2,GB2) bit-fields, and compares to
// PITCH.BIT decoded per §4.1.4 + §3.9.3. Per-field histograms and
// the first-10 divergences are printed for diagnostic.
//
// Plan §8 thresholds: IDEAL = 100% on each of the 8 params;
// ACCEPT-PARTIAL ≥ 80% on each; FAIL-DEFERRED floor =
// max(8 byte-EQ rates) ≥ Phase 2c INT-1 P1 byte-EQ rate
// (9.05%, per Phase 2c closure report §5).
func TestPhase2dINT1a_FCBByteEQ(t *testing.T) {
	const (
		inPath  = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"

		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
		expectedInBytes  = 293628
		expectedBitBytes = totalFrames * bytesPerBitFrame
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read PITCH.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read PITCH.BIT: %v", err)
	}
	if len(inData) != expectedInBytes {
		t.Fatalf("PITCH.IN size = %d, want %d", len(inData), expectedInBytes)
	}
	if len(bitData) != expectedBitBytes {
		t.Fatalf("PITCH.BIT size = %d, want %d", len(bitData), expectedBitBytes)
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16

	type fieldStats struct {
		name  string
		match int
		hist  map[int]int
		gotFn func() uint16
		refFn func() uint16
	}
	var (
		gC1, gS1, gGA1, gGB1, gC2, gS2, gGA2, gGB2 uint16
		rC1, rS1, rGA1, rGB1, rC2, rS2, rGA2, rGB2 uint16
	)
	stats := []*fieldStats{
		{name: "S1", hist: map[int]int{}, gotFn: func() uint16 { return gS1 }, refFn: func() uint16 { return rS1 }},
		{name: "C1", hist: map[int]int{}, gotFn: func() uint16 { return gC1 }, refFn: func() uint16 { return rC1 }},
		{name: "GA1", hist: map[int]int{}, gotFn: func() uint16 { return gGA1 }, refFn: func() uint16 { return rGA1 }},
		{name: "GB1", hist: map[int]int{}, gotFn: func() uint16 { return gGB1 }, refFn: func() uint16 { return rGB1 }},
		{name: "S2", hist: map[int]int{}, gotFn: func() uint16 { return gS2 }, refFn: func() uint16 { return rS2 }},
		{name: "C2", hist: map[int]int{}, gotFn: func() uint16 { return gC2 }, refFn: func() uint16 { return rC2 }},
		{name: "GA2", hist: map[int]int{}, gotFn: func() uint16 { return gGA2 }, refFn: func() uint16 { return rGA2 }},
		{name: "GB2", hist: map[int]int{}, gotFn: func() uint16 { return gGB2 }, refFn: func() uint16 { return rGB2 }},
	}

	const firstNDiverge = 10
	divergences := 0

	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		rC1, rS1, rGA1, rGB1, rC2, rS2, rGA2, rGB2 = extractFCBBitsFromG192(bitFrame)
		gS1 = uint16(enc.s1)
		gC1 = enc.c1
		gGA1 = uint16(enc.ga1)
		gGB1 = uint16(enc.gb1)
		gS2 = uint16(enc.s2)
		gC2 = enc.c2
		gGA2 = uint16(enc.ga2)
		gGB2 = uint16(enc.gb2)

		anyMiss := false
		for _, s := range stats {
			got := s.gotFn()
			ref := s.refFn()
			if got == ref {
				s.match++
			} else {
				anyMiss = true
			}
			s.hist[int(got)-int(ref)]++
		}
		if anyMiss && divergences < firstNDiverge {
			t.Logf("frame %d divergence: "+
				"S1 r=%d g=%d | C1 r=%d g=%d | GA1 r=%d g=%d | GB1 r=%d g=%d | "+
				"S2 r=%d g=%d | C2 r=%d g=%d | GA2 r=%d g=%d | GB2 r=%d g=%d",
				f,
				rS1, gS1, rC1, gC1, rGA1, gGA1, rGB1, gGB1,
				rS2, gS2, rC2, gC2, rGA2, gGA2, rGB2, gGB2)
			divergences++
		}
	}

	rates := make(map[string]float64, len(stats))
	for _, s := range stats {
		rate := 100.0 * float64(s.match) / float64(totalFrames)
		rates[s.name] = rate
		t.Logf("Phase 2d INT-1a byte-EQ: %s %d/%d (%.2f%%)",
			s.name, s.match, totalFrames, rate)
	}

	logHist := func(label string, h map[int]int) {
		ks := make([]int, 0, len(h))
		for k := range h {
			ks = append(ks, k)
		}
		sort.Ints(ks)
		var s string
		for _, k := range ks {
			c := h[k]
			if c >= totalFrames/200 { // ≥0.5% buckets
				s += fmt.Sprintf(" Δ=%+d:%d(%.1f%%)", k, c, 100*float64(c)/float64(totalFrames))
			}
		}
		t.Logf("%s delta histogram (≥0.5%% buckets):%s", label, s)
	}
	for _, s := range stats {
		logHist(s.name, s.hist)
	}

	const ideal = 100.0
	const acceptPartial = 80.0
	// Plan §8 step 4 floor: max(byte-EQ rate) MUST be ≥ Phase 2c
	// INT-1 P1 byte-EQ (9.05% per Phase 2c closure report §5).
	const phase2cP1Floor = 9.05

	maxRate := 0.0
	for _, r := range rates {
		if r > maxRate {
			maxRate = r
		}
	}
	t.Logf("Phase 2d INT-1a max byte-EQ across 8 params: %.2f%% "+
		"(plausibility floor = Phase 2c INT-1 P1 = %.2f%%)",
		maxRate, phase2cP1Floor)

	check := func(name string, rate float64) {
		switch {
		case rate >= ideal:
			t.Logf("%s: IDEAL (%.2f%%)", name, rate)
		case rate >= acceptPartial:
			t.Logf("%s: ACCEPT-PARTIAL (%.2f%%)", name, rate)
		default:
			t.Errorf("%s: BELOW ACCEPT-PARTIAL (%.2f%% < 80%%) — INT-1a I5 candidate", name, rate)
		}
	}
	for _, s := range stats {
		check(s.name, rates[s.name])
	}

	if maxRate < phase2cP1Floor {
		t.Errorf("Phase 2d INT-1a plausibility floor breach: max byte-EQ %.2f%% < Phase 2c INT-1 P1 %.2f%% — escalate I5",
			maxRate, phase2cP1Floor)
	}
}
