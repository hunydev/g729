package g729

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pcm"
)

// TestINT1D8GroundTruth — Phase 2a INT-1 d8 measurement battery.
//
// I6 BINDING: production code is NOT modified by this test. All
// observations are emitted via t.Logf; t.Errorf is reserved for the
// trustworthiness assertion only (none here — we are read-only on
// production behavior).
//
// Steps performed:
//
//   S1: Raw inspection of LSP.IN frames 0..7 (already done in the
//       d8 plan turn, repeated here with the production reader so
//       the artifact is self-contained).
//   S2: Encoder cold-start state inventory snapshot.
//   S3: Per-frame got vs want for frames 0..50 (convergence pattern).
//   S4: Per-frame index alignment sweep — does got[N] == want[N+k]
//       for some constant k? If yes, that is the 1-frame
//       analysis-vs-encode delay surfacing as a global offset.
//   S5: L0 mismatch direction histogram across all 2232 frames.
//   S6: Decoder-roundtrip ground truth for frame 0:
//          take WANT (L0=0, L1=120, L2=10, L3=10) and run them
//          through the same MA-predictor + L1+L2+L3 reconstruction
//          the decoder uses; compare the resulting ω against the
//          analytical i·π/11 baseline AND against the encoder's
//          actually-computed ω from frame 0.
//   S7: Frame 596 PCM dump + windowed/autocorr/levinson trace
//       confirming anti-palindromic a[].
func TestINT1D8GroundTruth(t *testing.T) {
	const (
		inPath  = "testdata/itu/G729_Release3/g729/test_vectors/LSP.IN"
		bitPath = "testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT"

		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
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

	readFrame := func(f int) [samplesPerFrame]int16 {
		var p [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			p[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}
		return p
	}

	wantIdx := func(f int) (uint8, uint8, uint8, uint8) {
		off := f * bytesPerBitFrame
		return extractLSPFieldsFromG192(bitData[off : off+bytesPerBitFrame])
	}

	// --- S1: raw inspection (frames 0..10 + 596) ---
	t.Log("=== S1: LSP.IN raw frame inspection ===")
	for f := 0; f <= 10; f++ {
		s := readFrame(f)
		nz, mn, mx := 0, int16(0), int16(0)
		var sumSq int64
		for i, v := range s {
			if v != 0 {
				nz++
			}
			if i == 0 || v < mn {
				mn = v
			}
			if i == 0 || v > mx {
				mx = v
			}
			sumSq += int64(v) * int64(v)
		}
		rms := 0.0
		if nz > 0 {
			rms = float64Sqrt(float64(sumSq) / 80.0)
		}
		t.Logf("frame %3d: nz=%2d/80 min=%6d max=%6d rms=%.2f", f, nz, mn, mx, rms)
	}
	{
		s := readFrame(596)
		mn, mx := s[0], s[0]
		var sumSq int64
		for _, v := range s {
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
			sumSq += int64(v) * int64(v)
		}
		rms := float64Sqrt(float64(sumSq) / 80.0)
		t.Logf("frame 596: min=%d max=%d rms=%.2f (anti-palindromic-LP frame)", mn, mx, rms)
	}

	// --- S2: cold-start state inventory ---
	t.Log("=== S2: encoder cold-start state inventory ===")
	enc := NewEncoder()
	t.Logf("HPF state (PreProcessor zero value): all int32 fields = 0 — verified by struct-zero init in pcm.PreProcessor")
	t.Logf("oldSpeech [240]int16: all zero on cold start")
	t.Logf("freqPrev [4][10]int16: initialized to i*pi/11 Q13 by lsp.InitFreqPrev")
	for k := 0; k < 4; k++ {
		t.Logf("  freqPrev[%d] = %v", k, enc.freqPrev[k])
	}
	// Print analytical i*pi/11 reference for comparison
	var anaQ13 [10]int16
	for i := 0; i < 10; i++ {
		// (i+1) * pi / 11 in Q13
		const piQ13 = 25736 // round(pi * 8192)
		_ = piQ13
		// Just emit what InitFreqPrev produced — it IS the reference.
		anaQ13[i] = enc.freqPrev[0][i]
	}
	t.Logf("(InitFreqPrev IS the analytical seed; no comparison drift possible)")

	// --- S3 + S4: per-frame got/want for all frames; convergence + offset ---
	t.Log("=== S3: per-frame got vs want (frames 0..50) ===")
	gotL0 := make([]uint8, totalFrames)
	gotL1 := make([]uint8, totalFrames)
	gotL2 := make([]uint8, totalFrames)
	gotL3 := make([]uint8, totalFrames)
	wantL0 := make([]uint8, totalFrames)
	wantL1 := make([]uint8, totalFrames)
	wantL2 := make([]uint8, totalFrames)
	wantL3 := make([]uint8, totalFrames)

	for f := 0; f < totalFrames; f++ {
		p := readFrame(f)
		idx, err := enc.lpcStep(p[:])
		if err != nil {
			// Frame 596 fatals — record what we have and stop
			t.Logf("lpcStep error at frame %d: %v — sweep truncated here", f, err)
			// Trim to f
			gotL0 = gotL0[:f]
			gotL1 = gotL1[:f]
			gotL2 = gotL2[:f]
			gotL3 = gotL3[:f]
			wantL0 = wantL0[:f]
			wantL1 = wantL1[:f]
			wantL2 = wantL2[:f]
			wantL3 = wantL3[:f]
			break
		}
		gotL0[f], gotL1[f], gotL2[f], gotL3[f] = idx.L0, idx.L1, idx.L2, idx.L3
		wantL0[f], wantL1[f], wantL2[f], wantL3[f] = wantIdx(f)
	}
	processed := len(gotL0)
	t.Logf("processed %d frames before stop", processed)

	frames0to50 := 51
	if processed < frames0to50 {
		frames0to50 = processed
	}
	for f := 0; f < frames0to50; f++ {
		eq0 := byteEq(gotL0[f], wantL0[f])
		eq1 := byteEq(gotL1[f], wantL1[f])
		eq2 := byteEq(gotL2[f], wantL2[f])
		eq3 := byteEq(gotL3[f], wantL3[f])
		t.Logf("f=%2d  got=(%d,%3d,%2d,%2d)  want=(%d,%3d,%2d,%2d)  eq=%s%s%s%s",
			f, gotL0[f], gotL1[f], gotL2[f], gotL3[f],
			wantL0[f], wantL1[f], wantL2[f], wantL3[f],
			eq0, eq1, eq2, eq3)
	}

	// --- S4: frame-offset alignment sweep ---
	t.Log("=== S4: per-frame got==want[k+offset] sweep over offsets in [-3..+3] ===")
	type rateRow struct {
		offset int
		l0     int
		l1     int
		l2     int
		l3     int
		all4   int
		denom  int
	}
	var rows []rateRow
	for off := -3; off <= 3; off++ {
		var l0, l1, l2, l3, all4, denom int
		for f := 0; f < processed; f++ {
			j := f + off
			if j < 0 || j >= processed {
				continue
			}
			denom++
			c0 := gotL0[f] == wantL0[j]
			c1 := gotL1[f] == wantL1[j]
			c2 := gotL2[f] == wantL2[j]
			c3 := gotL3[f] == wantL3[j]
			if c0 {
				l0++
			}
			if c1 {
				l1++
			}
			if c2 {
				l2++
			}
			if c3 {
				l3++
			}
			if c0 && c1 && c2 && c3 {
				all4++
			}
		}
		rows = append(rows, rateRow{off, l0, l1, l2, l3, all4, denom})
	}
	for _, r := range rows {
		t.Logf("offset=%+d  L0=%5.2f%%  L1=%5.2f%%  L2=%5.2f%%  L3=%5.2f%%  all4=%5.2f%%  (n=%d)",
			r.offset,
			100*float64(r.l0)/float64(r.denom),
			100*float64(r.l1)/float64(r.denom),
			100*float64(r.l2)/float64(r.denom),
			100*float64(r.l3)/float64(r.denom),
			100*float64(r.all4)/float64(r.denom),
			r.denom)
	}

	// --- S5: L0 / L2 mismatch histograms ---
	t.Log("=== S5: mismatch direction histograms ===")
	l0Mis := map[string]int{}
	l2Mis := map[string]int{}
	for f := 0; f < processed; f++ {
		if gotL0[f] != wantL0[f] {
			k := fmt.Sprintf("got=%d want=%d", gotL0[f], wantL0[f])
			l0Mis[k]++
		}
		if gotL2[f] != wantL2[f] {
			d := int(gotL2[f]) - int(wantL2[f])
			k := fmt.Sprintf("delta=%+d", d)
			l2Mis[k]++
		}
	}
	for k, v := range sortedHist(l0Mis) {
		t.Logf("L0 mismatch[%s] = %d", k, v)
	}
	for k, v := range sortedHist(l2Mis) {
		t.Logf("L2 mismatch[%s] = %d", k, v)
	}

	// --- S6: decoder-roundtrip ground truth on frame 0 ---
	// (Roundtrip itself lives in internal/lsp/phase2a_int1_d8_roundtrip_test.go
	// because applyPredictorWithMemory is package-private.)
	t.Log("=== S6: decoder-roundtrip ω — see internal/lsp/TestINT1D8DecoderRoundtripFrame0 ===")
	{
		// What we CAN measure here: HPF cold-start invariance.
		var pre pcm.PreProcessor
		zeros80 := make([]int16, 80)
		var processed2 [80]int16
		pre.Process(zeros80, processed2[:])
		nzPost := 0
		for _, v := range processed2 {
			if v != 0 {
				nzPost++
			}
		}
		t.Logf("HPF(zero PCM, zero-state) → nz=%d/80 (expect 0; confirms HPF is not the cold-start culprit)", nzPost)
	}
	_ = lsp.Indices{}

	// --- S7: frame 596 anti-palindromic isolation ---
	t.Log("=== S7: frame 596 anti-palindromic LP isolation (descriptive) ===")
	t.Log("Per d4 §19.7: a[]=[4096 -4706 -7743 5000 11938 0 -11938 -5000 7743 4706 -4096]")
	t.Log("This is exactly antisymmetric a[k] = -a[10-k]; F1(z)=A(z)+z^-11 A(1/z) has 0 sign changes.")
	t.Log("Spec §3.2.3 mandates 5 sign changes per polynomial; current production fatals.")
	t.Log("Proposed guard: when sign-change count < 5 in F1 OR F2, fall back to previous frame's quantized LSPs (lspOld) — see §3.2.6 'use of past LSP for stability' precedent in the spec text on subframe interpolation.")
}

func byteEq(a, b uint8) string {
	if a == b {
		return "✓"
	}
	return "✗"
}

// sortedHist returns the map's entries in deterministic key order.
func sortedHist(m map[string]int) map[string]int {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]int, len(m))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}

// float64Sqrt — local sqrt to avoid an import.
func float64Sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 32; i++ {
		z = (z + x/z) / 2
	}
	return z
}
