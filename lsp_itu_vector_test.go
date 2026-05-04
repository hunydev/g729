//go:build conformance
// +build conformance

package g729

import (
	"encoding/binary"
	"os"
	"testing"
)

// TestEncode_LSPVectorBitExact runs the full §3.2.1–§3.2.4 chain
// (windowed autocorrelation → Levinson-Durbin → LP→LSP → arccos →
// 18-bit two-stage VQ + L0 selector) on every frame of the ITU
// LSP.IN test vector and asserts byte equality of the four indices
// (L0, L1, L2, L3) against the LSP.BIT G.192-framed reference.
//
// This is the Phase 2a first-integration gate. Per plan §0.4
// 강압-적합-금지: on any divergence we report the first-divergent
// frame index and the measured-vs-expected tuples; we do NOT tune
// production to match.
func TestEncode_LSPVectorBitExact(t *testing.T) {
	const (
		inPath  = "testdata/itu/G729_Release3/g729/test_vectors/LSP.IN"
		bitPath = "testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT"

		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame // little-endian int16
		bytesPerBitFrame = 164                 // G.192: (1 sync + 1 len + 80 data) × 2
		totalFrames      = 2232
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read LSP.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read LSP.BIT: %v", err)
	}
	if len(inData) != totalFrames*bytesPerInFrame {
		t.Fatalf("LSP.IN size = %d, want %d", len(inData), totalFrames*bytesPerInFrame)
	}
	if len(bitData) != totalFrames*bytesPerBitFrame {
		t.Fatalf("LSP.BIT size = %d, want %d", len(bitData), totalFrames*bytesPerBitFrame)
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16

	var matchedL0, matchedL1, matchedL2, matchedL3 int
	firstFailFrame := -1
	var firstGotL0, firstGotL1, firstGotL2, firstGotL3 uint8
	var firstWantL0, firstWantL1, firstWantL2, firstWantL3 uint8

	for f := 0; f < totalFrames; f++ {
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		idx, err := enc.lpcStep(pcm[:])
		if err != nil {
			t.Fatalf("frame %d: lpcStep error: %v", f, err)
		}

		bitOff := f * bytesPerBitFrame
		wantL0, wantL1, wantL2, wantL3 := extractLSPFieldsFromG192(
			bitData[bitOff : bitOff+bytesPerBitFrame],
		)

		if idx.L0 == wantL0 {
			matchedL0++
		}
		if idx.L1 == wantL1 {
			matchedL1++
		}
		if idx.L2 == wantL2 {
			matchedL2++
		}
		if idx.L3 == wantL3 {
			matchedL3++
		}

		if firstFailFrame < 0 && (idx.L0 != wantL0 || idx.L1 != wantL1 || idx.L2 != wantL2 || idx.L3 != wantL3) {
			firstFailFrame = f
			firstGotL0, firstGotL1, firstGotL2, firstGotL3 = idx.L0, idx.L1, idx.L2, idx.L3
			firstWantL0, firstWantL1, firstWantL2, firstWantL3 = wantL0, wantL1, wantL2, wantL3
		}
	}

	t.Logf("frame match counts: L0=%d/%d L1=%d/%d L2=%d/%d L3=%d/%d",
		matchedL0, totalFrames, matchedL1, totalFrames,
		matchedL2, totalFrames, matchedL3, totalFrames)

	if firstFailFrame >= 0 {
		t.Fatalf("first divergence at frame %d:\n  got  (L0=%d L1=%d L2=%d L3=%d)\n  want (L0=%d L1=%d L2=%d L3=%d)",
			firstFailFrame,
			firstGotL0, firstGotL1, firstGotL2, firstGotL3,
			firstWantL0, firstWantL1, firstWantL2, firstWantL3)
	}
}
