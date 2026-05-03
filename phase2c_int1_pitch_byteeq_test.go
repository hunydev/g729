package g729

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"testing"
)

// extractPitchBitsFromG192 reads (P1, P0, P2) from one G.192-framed
// PITCH.BIT frame. Per ITU-T G.729 §4 Table 8 the in-frame bit
// layout (zero-indexed across the 80-bit payload, MSB-first per
// field) is:
//
//	L0(1) L1(7) L2(5) L3(5) P1(8) P0(1) C1(13) S1(4) GA1(3) GB1(4)
//	P2(5) C2(13) S2(4) GA2(3) GB2(4)
//
// so P1 occupies bits [18..25], P0 is bit 26, and P2 occupies bits
// [51..55]. The G.192 frame layout is 4-byte header (sync 0x6B21 +
// length 0x0050) followed by 80 × 2-byte bit-words (0x007F = 0,
// 0x0081 = 1).
func extractPitchBitsFromG192(frame []byte) (p1, p0, p2 uint16) {
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
	p1 = getField(18, 8)
	p0 = getBit(26)
	p2 = getField(51, 5)
	return
}

// TestPhase2cINT1_ClosedLoopPitchByteEQ is the Phase 2c INT-1 STRICT
// byte-EQ gate. It drives the encoder LPC + open-loop + closed-loop
// pitch chain (lpcStep → openloopStep → closedloopStep×2) on every
// frame of the ITU PITCH.IN test vector, extracts the emitted P1 /
// P0 / P2 bit-fields, and compares to PITCH.BIT decoded per §4
// Table 8. Histogram of per-field deltas is printed for diagnostic.
//
// Threshold per Phase 2c plan §8: IDEAL = 100%; ACCEPT-PARTIAL ≥
// 80% on each field; FAIL < 50%.
func TestPhase2cINT1_ClosedLoopPitchByteEQ(t *testing.T) {
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

	var p1Match, p0Match, p2Match int
	p1DeltaHist := map[int]int{}
	p2DeltaHist := map[int]int{}

	const firstNDiverge = 8
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
		refP1, refP0, refP2 := extractPitchBitsFromG192(bitFrame)
		gotP1 := uint16(enc.p1)
		gotP0 := uint16(enc.p0)
		gotP2 := uint16(enc.p2)

		if gotP1 == refP1 {
			p1Match++
		}
		if gotP0 == refP0 {
			p0Match++
		}
		if gotP2 == refP2 {
			p2Match++
		}
		p1DeltaHist[int(gotP1)-int(refP1)]++
		p2DeltaHist[int(gotP2)-int(refP2)]++

		if (gotP1 != refP1 || gotP0 != refP0 || gotP2 != refP2) && divergences < firstNDiverge {
			t.Logf("frame %d divergence: P1 ref=%d got=%d | P0 ref=%d got=%d | P2 ref=%d got=%d",
				f, refP1, gotP1, refP0, gotP0, refP2, gotP2)
			divergences++
		}
	}

	p1Rate := 100.0 * float64(p1Match) / float64(totalFrames)
	p0Rate := 100.0 * float64(p0Match) / float64(totalFrames)
	p2Rate := 100.0 * float64(p2Match) / float64(totalFrames)
	t.Logf("Phase 2c INT-1 byte-EQ rates: P1 %d/%d (%.2f%%)  P0 %d/%d (%.2f%%)  P2 %d/%d (%.2f%%)",
		p1Match, totalFrames, p1Rate,
		p0Match, totalFrames, p0Rate,
		p2Match, totalFrames, p2Rate)

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
	logHist("P1", p1DeltaHist)
	logHist("P2", p2DeltaHist)

	const ideal = 100.0
	const acceptPartial = 80.0
	const failBelow = 50.0

	check := func(name string, rate float64) {
		switch {
		case rate >= ideal:
			t.Logf("%s: IDEAL (%.2f%%)", name, rate)
		case rate >= acceptPartial:
			t.Logf("%s: ACCEPT-PARTIAL (%.2f%%)", name, rate)
		case rate >= failBelow:
			t.Errorf("%s: BELOW ACCEPT-PARTIAL (%.2f%% < 80%%) — needs I5 escalation", name, rate)
		default:
			t.Errorf("%s: FAIL (%.2f%% < 50%%) — escalate I5", name, rate)
		}
	}
	check("P1", p1Rate)
	check("P0", p0Rate)
	check("P2", p2Rate)
}
