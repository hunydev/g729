package lsp

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/lpc"
	"github.com/exedev/g729/internal/pcm"
	"github.com/exedev/g729/internal/tables"
)

func requireLSPExhaustiveDiag(t *testing.T) {
	t.Helper()
	if os.Getenv("G729_LSP_EXHAUSTIVE_DIAG") != "1" {
		t.Skip("set G729_LSP_EXHAUSTIVE_DIAG=1 to run exhaustive LSP surface diagnostics")
	}
}

type lspRankStats struct {
	total        int
	exact        int
	top3         int
	top8         int
	sumRank      int
	firstMissSet bool
	firstMiss    struct {
		frame     int
		want      uint8
		got       int
		rank      int
		wantCost  int64
		bestCost  int64
		gotFields [4]uint8
		refFields [4]uint8
	}
}

func (s *lspRankStats) add(frame int, want uint8, costs [32]int64, gotFields, refFields [4]uint8) {
	rank := rankCost32(costs, int(want))
	got := argmin64(costs[:])
	s.total++
	s.sumRank += rank
	if rank == 1 {
		s.exact++
	} else if !s.firstMissSet {
		s.firstMissSet = true
		s.firstMiss.frame = frame
		s.firstMiss.want = want
		s.firstMiss.got = got
		s.firstMiss.rank = rank
		s.firstMiss.wantCost = costs[want]
		s.firstMiss.bestCost = costs[got]
		s.firstMiss.gotFields = gotFields
		s.firstMiss.refFields = refFields
	}
	if rank <= 3 {
		s.top3++
	}
	if rank <= 8 {
		s.top8++
	}
}

func (s lspRankStats) log(t *testing.T, label string) {
	if s.total == 0 {
		t.Logf("%s: no samples", label)
		return
	}
	t.Logf("%s: exact %d/%d %.2f%% top3 %d/%d %.2f%% top8 %d/%d %.2f%% avg-rank %.2f",
		label,
		s.exact, s.total, pct(s.exact, s.total),
		s.top3, s.total, pct(s.top3, s.total),
		s.top8, s.total, pct(s.top8, s.total),
		float64(s.sumRank)/float64(s.total))
	if s.firstMissSet {
		m := s.firstMiss
		t.Logf("%s first miss: frame=%d want=%d got=%d rank=%d wantCost=%d bestCost=%d gotFields=%v refFields=%v",
			label, m.frame, m.want, m.got, m.rank, m.wantCost, m.bestCost, m.gotFields, m.refFields)
	}
}

type lspFinalCostStats struct {
	total         int
	refLower      int
	refEqual      int
	refWithin10   int
	gotL0, gotL1  int
	gotL2, gotL3  int
	gotAll        int
	sumCostRatioQ int64
	firstRefLower struct {
		set       bool
		frame     int
		gotCost   int64
		refCost   int64
		gotFields [4]uint8
		refFields [4]uint8
	}
}

func (s *lspFinalCostStats) add(frame int, got, ref Indices, gotCost, refCost int64) {
	s.total++
	gotFields := [4]uint8{got.L0, got.L1, got.L2, got.L3}
	refFields := [4]uint8{ref.L0, ref.L1, ref.L2, ref.L3}
	if got.L0 == ref.L0 {
		s.gotL0++
	}
	if got.L1 == ref.L1 {
		s.gotL1++
	}
	if got.L2 == ref.L2 {
		s.gotL2++
	}
	if got.L3 == ref.L3 {
		s.gotL3++
	}
	if got == ref {
		s.gotAll++
	}
	if refCost < gotCost {
		s.refLower++
		if !s.firstRefLower.set {
			s.firstRefLower.set = true
			s.firstRefLower.frame = frame
			s.firstRefLower.gotCost = gotCost
			s.firstRefLower.refCost = refCost
			s.firstRefLower.gotFields = gotFields
			s.firstRefLower.refFields = refFields
		}
	}
	if refCost == gotCost {
		s.refEqual++
	}
	if gotCost > 0 && refCost <= gotCost+gotCost/10 {
		s.refWithin10++
	}
	if gotCost > 0 {
		s.sumCostRatioQ += (refCost * 10000) / gotCost
	}
}

func (s lspFinalCostStats) log(t *testing.T) {
	t.Logf("fields: L0 %d/%d %.2f%% L1 %d/%d %.2f%% L2 %d/%d %.2f%% L3 %d/%d %.2f%% all %d/%d %.2f%%",
		s.gotL0, s.total, pct(s.gotL0, s.total),
		s.gotL1, s.total, pct(s.gotL1, s.total),
		s.gotL2, s.total, pct(s.gotL2, s.total),
		s.gotL3, s.total, pct(s.gotL3, s.total),
		s.gotAll, s.total, pct(s.gotAll, s.total))
	t.Logf("final full-cost oracle tuple: lower %d/%d %.2f%% equal %d/%d %.2f%% within +10%% %d/%d %.2f%% avg oracle/prod cost %.2fx",
		s.refLower, s.total, pct(s.refLower, s.total),
		s.refEqual, s.total, pct(s.refEqual, s.total),
		s.refWithin10, s.total, pct(s.refWithin10, s.total),
		float64(s.sumCostRatioQ)/10000.0/float64(s.total))
	if s.firstRefLower.set {
		m := s.firstRefLower
		t.Logf("first frame where oracle tuple has lower local final cost: frame=%d gotCost=%d refCost=%d gotFields=%v refFields=%v",
			m.frame, m.gotCost, m.refCost, m.gotFields, m.refFields)
	}
}

func TestINT1D11L23SurfaceDiagnostic(t *testing.T) {
	if testing.Short() {
		t.Skip("L2/L3 surface diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	var finalStats lspFinalCostStats
	var l2WhenL0L1 lspRankStats
	var l3WhenL0L1L2 lspRankStats
	var l2OraclePrefix lspRankStats
	var l3OraclePrefix lspRankStats

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
		ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}

		got := Quantize(&omega, &freqPrev)
		gotFields := [4]uint8{got.L0, got.L1, got.L2, got.L3}
		refFields := [4]uint8{ref.L0, ref.L1, ref.L2, ref.L3}

		var weights [10]int16
		weightsLSF(&omega, &weights)
		gotCost := finalLSPTupleCost(got, &memSnap, &omega, &weights)
		refCost := finalLSPTupleCost(ref, &memSnap, &omega, &weights)
		finalStats.add(f, got, ref, gotCost, refCost)

		l2CostsOraclePrefix := computeL2PerRowCost(ref.L1, ref.L0, &memSnap, &omega, &weights)
		l2OraclePrefix.add(f, ref.L2, l2CostsOraclePrefix, gotFields, refFields)
		l3CostsOraclePrefix := computeL3PerRowCost(ref.L1, ref.L2, ref.L0, &memSnap, &omega, &weights)
		l3OraclePrefix.add(f, ref.L3, l3CostsOraclePrefix, gotFields, refFields)

		if got.L0 == ref.L0 && got.L1 == ref.L1 {
			l2WhenL0L1.add(f, ref.L2, l2CostsOraclePrefix, gotFields, refFields)
		}
		if got.L0 == ref.L0 && got.L1 == ref.L1 && got.L2 == ref.L2 {
			l3Costs := computeL3PerRowCost(ref.L1, ref.L2, ref.L0, &memSnap, &omega, &weights)
			l3WhenL0L1L2.add(f, ref.L3, l3Costs, gotFields, refFields)
		}
	}

	finalStats.log(t)
	l2WhenL0L1.log(t, "L2 rank when L0/L1 already match")
	l3WhenL0L1L2.log(t, "L3 rank when L0/L1/L2 already match")
	l2OraclePrefix.log(t, "L2 rank under oracle L0/L1 prefix")
	l3OraclePrefix.log(t, "L3 rank under oracle L0/L1/L2 prefix")
}

func TestINT1D20TargetInterpolationDiagnostic(t *testing.T) {
	if testing.Short() {
		t.Skip("target interpolation diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	alphas := []int{0, 10, 25, 50, 75, 90, 100}
	exactByAlpha := make([]int, len(alphas))
	top8ByAlpha := make([]int, len(alphas))
	var thresholdHist [8]int
	var first []string

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
		ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}

		got := Quantize(&omega, &freqPrev)

		var refHat [10]int16
		reconstructLSFTuple(ref, &memSnap, &refHat)

		threshold := len(alphas)
		var rankAt0, rankAt100 int
		for ai, alpha := range alphas {
			blend := interpolateLSPTarget(&omega, &refHat, alpha)
			var weights [10]int16
			weightsLSF(&blend, &weights)
			costs := computeL2L3PairCost(ref.L1, ref.L0, &memSnap, &blend, &weights)
			rank := rankPairCost(costs, int(ref.L2), int(ref.L3))
			if alpha == 0 {
				rankAt0 = rank
			}
			if alpha == 100 {
				rankAt100 = rank
			}
			if rank == 1 {
				exactByAlpha[ai]++
				if threshold == len(alphas) {
					threshold = ai
				}
			}
			if rank <= 8 {
				top8ByAlpha[ai]++
			}
		}
		thresholdHist[threshold]++
		if got != ref && len(first) < 10 {
			first = append(first, fmt.Sprintf(
				"frame=%d got=(%d,%d,%d,%d) ref=(%d,%d,%d,%d) rank@0=%d rank@100=%d omega=%v refHat=%v",
				f, got.L0, got.L1, got.L2, got.L3,
				ref.L0, ref.L1, ref.L2, ref.L3,
				rankAt0, rankAt100, omega, refHat,
			))
		}
	}

	for ai, alpha := range alphas {
		t.Logf("alpha=%3d%% toward refHat: ref pair rank1 %d/%d %.2f%% top8 %d/%d %.2f%%",
			alpha, exactByAlpha[ai], totalFrames, pct(exactByAlpha[ai], totalFrames),
			top8ByAlpha[ai], totalFrames, pct(top8ByAlpha[ai], totalFrames))
	}
	for ai, alpha := range alphas {
		t.Logf("first alpha where ref pair rank1 == %3d%%: %d/%d %.2f%%",
			alpha, thresholdHist[ai], totalFrames, pct(thresholdHist[ai], totalFrames))
	}
	t.Logf("ref pair never rank1 by 100%% refHat target: %d/%d %.2f%%",
		thresholdHist[len(alphas)], totalFrames, pct(thresholdHist[len(alphas)], totalFrames))
	for i, msg := range first {
		t.Logf("mismatch[%d]: %s", i, msg)
	}
}

func interpolateLSPTarget(a, b *[10]int16, alphaPercent int) [10]int16 {
	var out [10]int16
	for i := range out {
		d := int(b[i]) - int(a[i])
		out[i] = int16(int(a[i]) + (d*alphaPercent+50)/100)
	}
	return out
}

func TestINT1D21ColdStartAnalysisTrace(t *testing.T) {
	if testing.Short() {
		t.Skip("cold-start analysis trace; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
		traceFrames      = 13
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	for f := 0; f < traceFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		lspErr := LPToLSP(&aQ12, &qQ15)
		reused := false
		if lspErr != nil {
			if lspErr != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, lspErr)
			}
			qQ15 = lspOld
			reused = true
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev
		got := Quantize(&omega, &freqPrev)

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
		ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}
		var refHat [10]int16
		reconstructLSFTuple(ref, &memSnap, &refHat)

		t.Logf("frame=%d pcmEnergy=%d processedEnergy=%d oldSpeechEnergy=%d oldSpeechTailEnergy=%d reusedLSP=%t maxAbsA=%d got=(%d,%d,%d,%d) ref=(%d,%d,%d,%d)",
			f,
			signalEnergy16(pcmFrame[:]),
			signalEnergy16(processed[:]),
			signalEnergy16(oldSpeech[:]),
			signalEnergy16(oldSpeech[160:240]),
			reused,
			maxAbsA(&aQ12),
			got.L0, got.L1, got.L2, got.L3,
			ref.L0, ref.L1, ref.L2, ref.L3)
		t.Logf("frame=%d aQ12=%v", f, aQ12)
		t.Logf("frame=%d qQ15=%v omega=%v refHat=%v omega-refHat=%v",
			f, qQ15, omega, refHat, diffLSP10(&omega, &refHat))
	}
}

func TestINT1D22AnalysisWindowOffsetDiagnostic(t *testing.T) {
	if testing.Short() {
		t.Skip("analysis-window offset diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	processed := preprocessLSPInputForDiag(t, inData)
	offsets := []int{-240, -200, -160, -120, -80, -40, 0, 40, 80}
	for _, offset := range offsets {
		var an lpc.Analyzer
		var freqPrev [4][10]int16
		var lspOld [10]int16
		InitFreqPrev(&freqPrev)
		InitLSPOld(&lspOld)
		var l0, l1, l2, l3, all, reused int
		firstSet := false
		var first string

		for f := 0; f < totalFrames; f++ {
			speech := analysisWindowAtOffset(processed, f*samplesPerFrame+offset)
			var aQ12 [lpc.LPCOrder + 1]int16
			if err := an.Analyze(&speech, &aQ12); err != nil {
				t.Fatalf("offset %d frame %d: lpc.Analyze: %v", offset, f, err)
			}

			var qQ15 [10]int16
			if err := LPToLSP(&aQ12, &qQ15); err != nil {
				if err != ErrLPCNonStable {
					t.Fatalf("offset %d frame %d: LPToLSP: %v", offset, f, err)
				}
				qQ15 = lspOld
				reused++
			} else {
				lspOld = qQ15
			}
			var omega [10]int16
			LSPToLSF(&qQ15, &omega)
			got := Quantize(&omega, &freqPrev)

			bitOff := f * bytesPerBitFrame
			refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
			if got.L0 == refL0 {
				l0++
			}
			if got.L1 == refL1 {
				l1++
			}
			if got.L2 == refL2 {
				l2++
			}
			if got.L3 == refL3 {
				l3++
			}
			if got.L0 == refL0 && got.L1 == refL1 && got.L2 == refL2 && got.L3 == refL3 {
				all++
			} else if !firstSet {
				firstSet = true
				first = fmt.Sprintf("frame=%d got=(%d,%d,%d,%d) ref=(%d,%d,%d,%d) aQ12=%v omega=%v",
					f, got.L0, got.L1, got.L2, got.L3, refL0, refL1, refL2, refL3, aQ12, omega)
			}
		}

		t.Logf("analysis offset %+4d: L0 %d/%d %.2f%% L1 %d/%d %.2f%% L2 %d/%d %.2f%% L3 %d/%d %.2f%% all4 %d/%d %.2f%% reused=%d",
			offset,
			l0, totalFrames, pct(l0, totalFrames),
			l1, totalFrames, pct(l1, totalFrames),
			l2, totalFrames, pct(l2, totalFrames),
			l3, totalFrames, pct(l3, totalFrames),
			all, totalFrames, pct(all, totalFrames),
			reused)
		if firstSet {
			t.Logf("analysis offset %+4d first mismatch: %s", offset, first)
		}
	}
}

func TestINT1D23Frame0InitialMemorySweepDiagnostic(t *testing.T) {
	if testing.Short() {
		t.Skip("frame-0 initial-memory sweep diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read LSP.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read LSP.BIT: %v", err)
	}
	if len(inData) < bytesPerInFrame {
		t.Fatalf("LSP.IN size = %d, want at least %d", len(inData), bytesPerInFrame)
	}
	if len(bitData) < bytesPerBitFrame {
		t.Fatalf("LSP.BIT size = %d, want at least %d", len(bitData), bytesPerBitFrame)
	}

	var pcmFrame [samplesPerFrame]int16
	for i := 0; i < samplesPerFrame; i++ {
		pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[2*i : 2*i+2]))
	}
	var pp pcm.PreProcessor
	var processed [samplesPerFrame]int16
	pp.Process(pcmFrame[:], processed[:])

	var oldSpeech [240]int16
	copy(oldSpeech[160:240], processed[:])
	var an lpc.Analyzer
	var aQ12 [lpc.LPCOrder + 1]int16
	if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
		t.Fatalf("frame 0: lpc.Analyze: %v", err)
	}
	var qQ15 [10]int16
	if err := LPToLSP(&aQ12, &qQ15); err != nil {
		t.Fatalf("frame 0: LPToLSP: %v", err)
	}
	var omega [10]int16
	LSPToLSF(&qQ15, &omega)
	var weights [10]int16
	weightsLSF(&omega, &weights)

	refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[:bytesPerBitFrame])
	ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}
	refResidual := residualForIndices(ref)
	impliedUniform := impliedUniformInitialMemoryForFrame0(ref, &omega)

	var initMem [4][10]int16
	InitFreqPrev(&initMem)
	var zeroMem [4][10]int16
	var refResidualMem [4][10]int16
	var impliedMem [4][10]int16
	for k := 0; k < 4; k++ {
		refResidualMem[k] = refResidual
		impliedMem[k] = impliedUniform
	}

	variants := []struct {
		name string
		mem  [4][10]int16
	}{
		{name: "production-initPastResidual", mem: initMem},
		{name: "zero-memory", mem: zeroMem},
		{name: "frame0-implied-uniform-memory", mem: impliedMem},
		{name: "reference-residual-repeated", mem: refResidualMem},
	}

	t.Logf("frame0 pcmEnergy=%d processedEnergy=%d oldSpeechEnergy=%d aQ12=%v qQ15=%v omega=%v ref=(%d,%d,%d,%d) refResidual=%v impliedUniformMemory=%v",
		signalEnergy16(pcmFrame[:]), signalEnergy16(processed[:]), signalEnergy16(oldSpeech[:]),
		aQ12, qQ15, omega, ref.L0, ref.L1, ref.L2, ref.L3, refResidual, impliedUniform)

	for _, v := range variants {
		mem := v.mem
		got, _ := quantizeNoCommit(&omega, &mem)

		var target [10]int16
		computeTargetLSF(ref.L0, &mem, &omega, &target)
		l1Rank, l1Best, l1WantCost, l1BestCost := rankL1Target(&target, int(ref.L1))

		pairCosts := computeL2L3PairCost(ref.L1, ref.L0, &mem, &omega, &weights)
		bestL2, bestL3 := bestPairCost(pairCosts)
		pairRank := rankPairCost(pairCosts, int(ref.L2), int(ref.L3))

		gotCost := finalLSPTupleCost(got, &mem, &omega, &weights)
		refCost := finalLSPTupleCost(ref, &mem, &omega, &weights)
		fullRank := rankFullLSPTuple(ref, &mem, &omega, &weights)

		var gotHat, refHat [10]int16
		reconstructLSFTuple(got, &mem, &gotHat)
		reconstructLSFTuple(ref, &mem, &refHat)

		t.Logf("variant=%s got=(%d,%d,%d,%d) ref=(%d,%d,%d,%d) gotCost=%d refCost=%d refFullRank=%d",
			v.name, got.L0, got.L1, got.L2, got.L3, ref.L0, ref.L1, ref.L2, ref.L3,
			gotCost, refCost, fullRank)
		t.Logf("variant=%s L1 target selector=%d best=%d ref=%d rank=%d refCost=%d bestCost=%d target=%v",
			v.name, ref.L0, l1Best, ref.L1, l1Rank, l1WantCost, l1BestCost, target)
		t.Logf("variant=%s ref-prefix pair best=(%d,%d) ref=(%d,%d) rank=%d refCost=%d bestCost=%d gotHat=%v refHat=%v omega-refHat=%v",
			v.name, bestL2, bestL3, ref.L2, ref.L3, pairRank,
			pairCosts[ref.L2][ref.L3], pairCosts[bestL2][bestL3],
			gotHat, refHat, diffLSP10(&omega, &refHat))
	}
}

func preprocessLSPInputForDiag(t *testing.T, inData []byte) []int16 {
	t.Helper()
	const samplesPerFrame = 80
	totalFrames := len(inData) / (2 * samplesPerFrame)
	out := make([]int16, totalFrames*samplesPerFrame)
	var pp pcm.PreProcessor
	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * 2 * samplesPerFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}
		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(out[f*samplesPerFrame:(f+1)*samplesPerFrame], processed[:])
	}
	return out
}

func analysisWindowAtOffset(processed []int16, start int) [240]int16 {
	var speech [240]int16
	for i := range speech {
		src := start + i
		if src >= 0 && src < len(processed) {
			speech[i] = processed[src]
		}
	}
	return speech
}

func signalEnergy16(x []int16) int64 {
	var out int64
	for _, v := range x {
		out += int64(v) * int64(v)
	}
	return out
}

func diffLSP10(a, b *[10]int16) [10]int16 {
	var out [10]int16
	for i := range out {
		out[i] = a[i] - b[i]
	}
	return out
}

type lspOmegaBucketStats struct {
	total           int
	refCloser       int
	equalDist       int
	sumGotDist      int64
	sumRefDist      int64
	sumGotWDist     int64
	sumRefWDist     int64
	sumAAbsMax      int64
	sumOmegaRefAbs  int64
	sumOmegaGotAbs  int64
	sumRefGotHatAbs int64
	first           struct {
		set       bool
		frame     int
		got       [4]uint8
		ref       [4]uint8
		aAbsMax   int16
		omega     [10]int16
		gotHat    [10]int16
		refHat    [10]int16
		gotDist   int64
		refDist   int64
		gotWDist  int64
		refWDist  int64
		omegaDiff [10]int16
	}
}

func (s *lspOmegaBucketStats) add(frame int, got, ref Indices, aQ12 *[11]int16, omega, weights, gotHat, refHat *[10]int16) {
	gotDist := lsfDistance(omega, gotHat)
	refDist := lsfDistance(omega, refHat)
	gotWDist := lsfWeightedDistance(omega, gotHat, weights)
	refWDist := lsfWeightedDistance(omega, refHat, weights)
	aAbsMax := maxAbsA(aQ12)

	s.total++
	s.sumGotDist += gotDist
	s.sumRefDist += refDist
	s.sumGotWDist += gotWDist
	s.sumRefWDist += refWDist
	s.sumAAbsMax += int64(aAbsMax)
	s.sumOmegaGotAbs += lsfAbsDistance(omega, gotHat)
	s.sumOmegaRefAbs += lsfAbsDistance(omega, refHat)
	s.sumRefGotHatAbs += lsfAbsDistance(refHat, gotHat)
	if refDist < gotDist {
		s.refCloser++
	} else if refDist == gotDist {
		s.equalDist++
	}
	if !s.first.set {
		s.first.set = true
		s.first.frame = frame
		s.first.got = [4]uint8{got.L0, got.L1, got.L2, got.L3}
		s.first.ref = [4]uint8{ref.L0, ref.L1, ref.L2, ref.L3}
		s.first.aAbsMax = aAbsMax
		s.first.omega = *omega
		s.first.gotHat = *gotHat
		s.first.refHat = *refHat
		s.first.gotDist = gotDist
		s.first.refDist = refDist
		s.first.gotWDist = gotWDist
		s.first.refWDist = refWDist
		for i := 0; i < 10; i++ {
			s.first.omegaDiff[i] = omega[i] - refHat[i]
		}
	}
}

func (s lspOmegaBucketStats) log(t *testing.T, label string) {
	if s.total == 0 {
		t.Logf("%s: no frames", label)
		return
	}
	t.Logf("%s: n=%d ref-closer %d/%d %.2f%% equal %d/%d %.2f%% avgDist got=%d ref=%d avgWDist got=%d ref=%d avgAbs omega-got=%d omega-ref=%d refHat-gotHat=%d avgMaxAbsA=%d",
		label,
		s.total,
		s.refCloser, s.total, pct(s.refCloser, s.total),
		s.equalDist, s.total, pct(s.equalDist, s.total),
		s.sumGotDist/int64(s.total), s.sumRefDist/int64(s.total),
		s.sumGotWDist/int64(s.total), s.sumRefWDist/int64(s.total),
		s.sumOmegaGotAbs/int64(s.total), s.sumOmegaRefAbs/int64(s.total),
		s.sumRefGotHatAbs/int64(s.total), s.sumAAbsMax/int64(s.total))
	f := s.first
	t.Logf("%s first: frame=%d got=%v ref=%v maxAbsA=%d gotDist=%d refDist=%d gotWDist=%d refWDist=%d",
		label, f.frame, f.got, f.ref, f.aAbsMax, f.gotDist, f.refDist, f.gotWDist, f.refWDist)
	t.Logf("%s first omega=%v gotHat=%v refHat=%v omega-refHat=%v",
		label, f.omega, f.gotHat, f.refHat, f.omegaDiff)
}

func TestINT1D12OmegaTrajectoryDiagnostic(t *testing.T) {
	if testing.Short() {
		t.Skip("omega trajectory diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	buckets := map[string]*lspOmegaBucketStats{
		"all-match": {},
		"L0-first":  {},
		"L1-first":  {},
		"L2-first":  {},
		"L3-first":  {},
	}

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
		ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}

		got := Quantize(&omega, &freqPrev)

		var weights [10]int16
		weightsLSF(&omega, &weights)
		var gotHat, refHat [10]int16
		reconstructLSFTuple(got, &memSnap, &gotHat)
		reconstructLSFTuple(ref, &memSnap, &refHat)

		label := firstDivergentLSPField(got, ref)
		buckets[label].add(f, got, ref, &aQ12, &omega, &weights, &gotHat, &refHat)
	}

	for _, label := range []string{"all-match", "L0-first", "L1-first", "L2-first", "L3-first"} {
		buckets[label].log(t, label)
	}
}

type lspOmegaCoordStats struct {
	total          int
	sumSigned      [10]int64
	sumAbs         [10]int64
	sumQAbs        [10]int64
	sumRefHat      [10]int64
	sumOmega       [10]int64
	sumQ           [10]int64
	sumRefHatQ     [10]int64
	maxAbs         [10]int64
	maxAbsFrame    [10]int
	maxQAbs        [10]int64
	maxQAbsFrame   [10]int
	firstFrame     int
	firstGot       [4]uint8
	firstRef       [4]uint8
	firstA         [11]int16
	firstQ         [10]int16
	firstOmega     [10]int16
	firstRefHat    [10]int16
	firstRefHatQ   [10]int16
	firstOmegaDiff [10]int16
	firstQDiff     [10]int16
}

func (s *lspOmegaCoordStats) add(frame int, got, ref Indices, aQ12 *[11]int16, qQ15, omega, refHat *[10]int16) {
	if s.total == 0 {
		s.firstFrame = frame
		s.firstGot = [4]uint8{got.L0, got.L1, got.L2, got.L3}
		s.firstRef = [4]uint8{ref.L0, ref.L1, ref.L2, ref.L3}
		s.firstA = *aQ12
		s.firstQ = *qQ15
		s.firstOmega = *omega
		s.firstRefHat = *refHat
	}
	s.total++
	for i := 0; i < 10; i++ {
		refQ := lsfToLSP(refHat[i])
		if s.total == 1 {
			s.firstRefHatQ[i] = refQ
			s.firstOmegaDiff[i] = omega[i] - refHat[i]
			s.firstQDiff[i] = qQ15[i] - refQ
		}
		d := int64(omega[i]) - int64(refHat[i])
		qd := int64(qQ15[i]) - int64(refQ)
		s.sumSigned[i] += d
		s.sumAbs[i] += abs64(d)
		s.sumQAbs[i] += abs64(qd)
		s.sumRefHat[i] += int64(refHat[i])
		s.sumOmega[i] += int64(omega[i])
		s.sumQ[i] += int64(qQ15[i])
		s.sumRefHatQ[i] += int64(refQ)
		if abs64(d) > s.maxAbs[i] {
			s.maxAbs[i] = abs64(d)
			s.maxAbsFrame[i] = frame
		}
		if abs64(qd) > s.maxQAbs[i] {
			s.maxQAbs[i] = abs64(qd)
			s.maxQAbsFrame[i] = frame
		}
	}
}

func (s lspOmegaCoordStats) log(t *testing.T, label string) {
	if s.total == 0 {
		t.Logf("%s: no frames", label)
		return
	}
	var avgSigned, avgAbs, avgQAbs [10]int64
	var avgOmega, avgRefHat, avgQ, avgRefHatQ [10]int64
	for i := 0; i < 10; i++ {
		avgSigned[i] = s.sumSigned[i] / int64(s.total)
		avgAbs[i] = s.sumAbs[i] / int64(s.total)
		avgQAbs[i] = s.sumQAbs[i] / int64(s.total)
		avgOmega[i] = s.sumOmega[i] / int64(s.total)
		avgRefHat[i] = s.sumRefHat[i] / int64(s.total)
		avgQ[i] = s.sumQ[i] / int64(s.total)
		avgRefHatQ[i] = s.sumRefHatQ[i] / int64(s.total)
	}
	t.Logf("%s: n=%d avg omega-refHat signed=%v abs=%v avg q-lsfToLSP(refHat) abs=%v",
		label, s.total, avgSigned, avgAbs, avgQAbs)
	t.Logf("%s: avg omega=%v refHat=%v q=%v refHatQ=%v",
		label, avgOmega, avgRefHat, avgQ, avgRefHatQ)
	t.Logf("%s: max abs omega-refHat=%v frames=%v max abs q-refHatQ=%v frames=%v",
		label, s.maxAbs, s.maxAbsFrame, s.maxQAbs, s.maxQAbsFrame)
	t.Logf("%s first frame=%d got=%v ref=%v", label, s.firstFrame, s.firstGot, s.firstRef)
	t.Logf("%s first aQ12=%v", label, s.firstA)
	t.Logf("%s first q=%v omega=%v refHat=%v refHatQ=%v",
		label, s.firstQ, s.firstOmega, s.firstRefHat, s.firstRefHatQ)
	t.Logf("%s first omega-refHat=%v q-refHatQ=%v",
		label, s.firstOmegaDiff, s.firstQDiff)
}

func TestINT1D13OmegaSourceFrameTrace(t *testing.T) {
	if testing.Short() {
		t.Skip("omega source frame trace; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	traceFrames := map[int]bool{0: true, 5: true, 6: true, 7: true, 11: true, 28: true}
	buckets := map[string]*lspOmegaCoordStats{
		"all-match": {},
		"L0-first":  {},
		"L1-first":  {},
		"L2-first":  {},
		"L3-first":  {},
	}

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
		ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}

		got := Quantize(&omega, &freqPrev)

		var refHat [10]int16
		reconstructLSFTuple(ref, &memSnap, &refHat)
		label := firstDivergentLSPField(got, ref)
		buckets[label].add(f, got, ref, &aQ12, &qQ15, &omega, &refHat)

		if traceFrames[f] {
			var gotHat [10]int16
			reconstructLSFTuple(got, &memSnap, &gotHat)
			logOmegaSourceTraceFrame(t, f, got, ref, &aQ12, &qQ15, &omega, &gotHat, &refHat)
		}
	}

	for _, label := range []string{"all-match", "L0-first", "L1-first", "L2-first", "L3-first"} {
		buckets[label].log(t, label)
	}
}

func TestINT1D14WeightVariantDiagnostic(t *testing.T) {
	if testing.Short() {
		t.Skip("weight variant diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	var l2Weighted, l2NoBoost, l2Unweighted lspRankStats
	var l3Weighted, l3NoBoost, l3Unweighted lspRankStats
	var frame0Logged bool

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
		ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}

		got := Quantize(&omega, &freqPrev)
		gotFields := [4]uint8{got.L0, got.L1, got.L2, got.L3}
		refFields := [4]uint8{ref.L0, ref.L1, ref.L2, ref.L3}

		var weights, weightsNoBoost [10]int16
		weightsLSF(&omega, &weights)
		weightsLSFNoBoost(&omega, &weightsNoBoost)

		l2WCosts := computeL2PerRowCost(ref.L1, ref.L0, &memSnap, &omega, &weights)
		l2NBCosts := computeL2PerRowCost(ref.L1, ref.L0, &memSnap, &omega, &weightsNoBoost)
		l2UCosts := computeL2PerRowCostUnweighted(ref.L1, ref.L0, &memSnap, &omega)
		l2Weighted.add(f, ref.L2, l2WCosts, gotFields, refFields)
		l2NoBoost.add(f, ref.L2, l2NBCosts, gotFields, refFields)
		l2Unweighted.add(f, ref.L2, l2UCosts, gotFields, refFields)

		l3WCosts := computeL3PerRowCost(ref.L1, ref.L2, ref.L0, &memSnap, &omega, &weights)
		l3NBCosts := computeL3PerRowCost(ref.L1, ref.L2, ref.L0, &memSnap, &omega, &weightsNoBoost)
		l3UCosts := computeL3PerRowCostUnweighted(ref.L1, ref.L2, ref.L0, &memSnap, &omega)
		l3Weighted.add(f, ref.L3, l3WCosts, gotFields, refFields)
		l3NoBoost.add(f, ref.L3, l3NBCosts, gotFields, refFields)
		l3Unweighted.add(f, ref.L3, l3UCosts, gotFields, refFields)

		if f == 0 && !frame0Logged {
			frame0Logged = true
			t.Logf("frame0 weights=%v noBoost=%v", weights, weightsNoBoost)
			t.Logf("frame0 L2 weighted argmin=%d want=%d gotCost=%d wantCost=%d",
				argmin64(l2WCosts[:]), ref.L2, l2WCosts[argmin64(l2WCosts[:])], l2WCosts[ref.L2])
			t.Logf("frame0 L2 noBoost argmin=%d want=%d gotCost=%d wantCost=%d",
				argmin64(l2NBCosts[:]), ref.L2, l2NBCosts[argmin64(l2NBCosts[:])], l2NBCosts[ref.L2])
			t.Logf("frame0 L2 unweighted argmin=%d want=%d gotCost=%d wantCost=%d",
				argmin64(l2UCosts[:]), ref.L2, l2UCosts[argmin64(l2UCosts[:])], l2UCosts[ref.L2])
			t.Logf("frame0 L3 weighted argmin=%d want=%d gotCost=%d wantCost=%d",
				argmin64(l3WCosts[:]), ref.L3, l3WCosts[argmin64(l3WCosts[:])], l3WCosts[ref.L3])
			t.Logf("frame0 L3 noBoost argmin=%d want=%d gotCost=%d wantCost=%d",
				argmin64(l3NBCosts[:]), ref.L3, l3NBCosts[argmin64(l3NBCosts[:])], l3NBCosts[ref.L3])
			t.Logf("frame0 L3 unweighted argmin=%d want=%d gotCost=%d wantCost=%d",
				argmin64(l3UCosts[:]), ref.L3, l3UCosts[argmin64(l3UCosts[:])], l3UCosts[ref.L3])
		}
	}

	l2Weighted.log(t, "L2 oracle-prefix weighted")
	l2NoBoost.log(t, "L2 oracle-prefix no-boost")
	l2Unweighted.log(t, "L2 oracle-prefix unweighted")
	l3Weighted.log(t, "L3 oracle-prefix weighted")
	l3NoBoost.log(t, "L3 oracle-prefix no-boost")
	l3Unweighted.log(t, "L3 oracle-prefix unweighted")
}

type lspPairRankStats struct {
	total          int
	exact          int
	top3           int
	top8           int
	top32          int
	sumRank        int
	greedyExact    int
	exhaustGreedy  int
	firstMissSet   bool
	firstMissFrame int
	firstWantL2    uint8
	firstWantL3    uint8
	firstBestL2    uint8
	firstBestL3    uint8
	firstRank      int
	firstWantCost  int64
	firstBestCost  int64
	firstGreedyL2  uint8
	firstGreedyL3  uint8
}

func (s *lspPairRankStats) add(frame int, wantL2, wantL3 uint8, costs [32][32]int64, greedyL2, greedyL3 uint8) {
	bestL2, bestL3 := argminPairCost(costs)
	rank := rankPairCost(costs, int(wantL2), int(wantL3))
	s.total++
	s.sumRank += rank
	if rank == 1 {
		s.exact++
	}
	if rank <= 3 {
		s.top3++
	}
	if rank <= 8 {
		s.top8++
	}
	if rank <= 32 {
		s.top32++
	}
	if greedyL2 == wantL2 && greedyL3 == wantL3 {
		s.greedyExact++
	}
	if greedyL2 == bestL2 && greedyL3 == bestL3 {
		s.exhaustGreedy++
	}
	if rank != 1 && !s.firstMissSet {
		s.firstMissSet = true
		s.firstMissFrame = frame
		s.firstWantL2 = wantL2
		s.firstWantL3 = wantL3
		s.firstBestL2 = bestL2
		s.firstBestL3 = bestL3
		s.firstRank = rank
		s.firstWantCost = costs[wantL2][wantL3]
		s.firstBestCost = costs[bestL2][bestL3]
		s.firstGreedyL2 = greedyL2
		s.firstGreedyL3 = greedyL3
	}
}

func (s lspPairRankStats) log(t *testing.T, label string) {
	if s.total == 0 {
		t.Logf("%s: no samples", label)
		return
	}
	t.Logf("%s: pair exact %d/%d %.2f%% top3 %d/%d %.2f%% top8 %d/%d %.2f%% top32 %d/%d %.2f%% avg-rank %.2f greedy-exact %d/%d %.2f%% exhaustive==greedy %d/%d %.2f%%",
		label,
		s.exact, s.total, pct(s.exact, s.total),
		s.top3, s.total, pct(s.top3, s.total),
		s.top8, s.total, pct(s.top8, s.total),
		s.top32, s.total, pct(s.top32, s.total),
		float64(s.sumRank)/float64(s.total),
		s.greedyExact, s.total, pct(s.greedyExact, s.total),
		s.exhaustGreedy, s.total, pct(s.exhaustGreedy, s.total))
	if s.firstMissSet {
		t.Logf("%s first miss: frame=%d want=(%d,%d) best=(%d,%d) greedy=(%d,%d) rank=%d wantCost=%d bestCost=%d",
			label, s.firstMissFrame, s.firstWantL2, s.firstWantL3,
			s.firstBestL2, s.firstBestL3, s.firstGreedyL2, s.firstGreedyL3,
			s.firstRank, s.firstWantCost, s.firstBestCost)
	}
}

func TestINT1D15SplitSearchExhaustiveDiagnostic(t *testing.T) {
	requireLSPExhaustiveDiag(t)
	if testing.Short() {
		t.Skip("split-search exhaustive diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	var weighted, unweighted lspPairRankStats
	var weightedWhenL0L1Match, unweightedWhenL0L1Match lspPairRankStats

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])

		got := Quantize(&omega, &freqPrev)

		var weights [10]int16
		weightsLSF(&omega, &weights)
		greedyL2, _ := searchL2(refL1, refL0, &memSnap, &omega, &weights)
		greedyL3, _ := searchL3(refL1, uint8(greedyL2), refL0, &memSnap, &omega, &weights)
		wCosts := computeL2L3PairCost(refL1, refL0, &memSnap, &omega, &weights)
		weighted.add(f, refL2, refL3, wCosts, uint8(greedyL2), uint8(greedyL3))

		uCosts := computeL2L3PairCostUnweighted(refL1, refL0, &memSnap, &omega)
		uL2Costs := computeL2PerRowCostUnweighted(refL1, refL0, &memSnap, &omega)
		uGreedyL2 := argmin64(uL2Costs[:])
		uL3Costs := computeL3PerRowCostUnweighted(refL1, uint8(uGreedyL2), refL0, &memSnap, &omega)
		uGreedyL3 := argmin64(uL3Costs[:])
		unweighted.add(f, refL2, refL3, uCosts, uint8(uGreedyL2), uint8(uGreedyL3))

		if got.L0 == refL0 && got.L1 == refL1 {
			weightedWhenL0L1Match.add(f, refL2, refL3, wCosts, uint8(greedyL2), uint8(greedyL3))
			unweightedWhenL0L1Match.add(f, refL2, refL3, uCosts, uint8(uGreedyL2), uint8(uGreedyL3))
		}

		if f == 0 {
			bestWL2, bestWL3 := argminPairCost(wCosts)
			bestUL2, bestUL3 := argminPairCost(uCosts)
			t.Logf("frame0 weighted exhaustive best=(%d,%d) oracle=(%d,%d) greedy=(%d,%d) oracleRank=%d oracleCost=%d bestCost=%d",
				bestWL2, bestWL3, refL2, refL3, greedyL2, greedyL3,
				rankPairCost(wCosts, int(refL2), int(refL3)), wCosts[refL2][refL3], wCosts[bestWL2][bestWL3])
			t.Logf("frame0 unweighted exhaustive best=(%d,%d) oracle=(%d,%d) greedy=(%d,%d) oracleRank=%d oracleCost=%d bestCost=%d",
				bestUL2, bestUL3, refL2, refL3, uGreedyL2, uGreedyL3,
				rankPairCost(uCosts, int(refL2), int(refL3)), uCosts[refL2][refL3], uCosts[bestUL2][bestUL3])
		}
	}

	weighted.log(t, "oracle-prefix weighted L2xL3 exhaustive")
	unweighted.log(t, "oracle-prefix unweighted L2xL3 exhaustive")
	weightedWhenL0L1Match.log(t, "L0/L1-match weighted L2xL3 exhaustive")
	unweightedWhenL0L1Match.log(t, "L0/L1-match unweighted L2xL3 exhaustive")
}

type lspFieldMatchStats struct {
	total             int
	l0, l1, l2, l3    int
	all               int
	refCostLower      int
	refCostWithin10   int
	sumRefOverGotCost int64
	firstMissSet      bool
	firstMissFrame    int
	firstGot          [4]uint8
	firstRef          [4]uint8
	firstGotCost      int64
	firstRefCost      int64
}

func (s *lspFieldMatchStats) add(frame int, got, ref Indices, gotCost, refCost int64) {
	s.total++
	if got.L0 == ref.L0 {
		s.l0++
	}
	if got.L1 == ref.L1 {
		s.l1++
	}
	if got.L2 == ref.L2 {
		s.l2++
	}
	if got.L3 == ref.L3 {
		s.l3++
	}
	if got == ref {
		s.all++
	}
	if refCost < gotCost {
		s.refCostLower++
	}
	if gotCost > 0 && refCost <= gotCost+gotCost/10 {
		s.refCostWithin10++
	}
	if gotCost > 0 {
		s.sumRefOverGotCost += (refCost * 10000) / gotCost
	}
	if got != ref && !s.firstMissSet {
		s.firstMissSet = true
		s.firstMissFrame = frame
		s.firstGot = [4]uint8{got.L0, got.L1, got.L2, got.L3}
		s.firstRef = [4]uint8{ref.L0, ref.L1, ref.L2, ref.L3}
		s.firstGotCost = gotCost
		s.firstRefCost = refCost
	}
}

func (s lspFieldMatchStats) log(t *testing.T, label string) {
	if s.total == 0 {
		t.Logf("%s: no samples", label)
		return
	}
	t.Logf("%s: L0 %d/%d %.2f%% L1 %d/%d %.2f%% L2 %d/%d %.2f%% L3 %d/%d %.2f%% all %d/%d %.2f%% refLower %d/%d %.2f%% refWithin10 %d/%d %.2f%% avgRef/GotCost %.2fx",
		label,
		s.l0, s.total, pct(s.l0, s.total),
		s.l1, s.total, pct(s.l1, s.total),
		s.l2, s.total, pct(s.l2, s.total),
		s.l3, s.total, pct(s.l3, s.total),
		s.all, s.total, pct(s.all, s.total),
		s.refCostLower, s.total, pct(s.refCostLower, s.total),
		s.refCostWithin10, s.total, pct(s.refCostWithin10, s.total),
		float64(s.sumRefOverGotCost)/10000.0/float64(s.total))
	if s.firstMissSet {
		t.Logf("%s first miss: frame=%d got=%v ref=%v gotCost=%d refCost=%d",
			label, s.firstMissFrame, s.firstGot, s.firstRef, s.firstGotCost, s.firstRefCost)
	}
}

func TestINT1D16PredictorTrajectoryDiagnostic(t *testing.T) {
	if testing.Short() {
		t.Skip("predictor trajectory diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var prodMem, refMem [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&prodMem)
	InitFreqPrev(&refMem)
	InitLSPOld(&lspOld)

	var prodStats, refCommitStats lspFieldMatchStats

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		var weights [10]int16
		weightsLSF(&omega, &weights)

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])
		ref := Indices{L0: refL0, L1: refL1, L2: refL2, L3: refL3}

		prodGot, prodResidual := quantizeNoCommit(&omega, &prodMem)
		prodGotCost := finalLSPTupleCost(prodGot, &prodMem, &omega, &weights)
		prodRefCost := finalLSPTupleCost(ref, &prodMem, &omega, &weights)
		prodStats.add(f, prodGot, ref, prodGotCost, prodRefCost)
		commitPredictorMemory(&prodMem, &prodResidual)

		refCommitGot, _ := quantizeNoCommit(&omega, &refMem)
		refCommitGotCost := finalLSPTupleCost(refCommitGot, &refMem, &omega, &weights)
		refCommitRefCost := finalLSPTupleCost(ref, &refMem, &omega, &weights)
		refCommitStats.add(f, refCommitGot, ref, refCommitGotCost, refCommitRefCost)
		refResidual := residualForIndices(ref)
		commitPredictorMemory(&refMem, &refResidual)

		if f < 8 {
			t.Logf("frame=%d prodMem got=(%d,%d,%d,%d) ref=(%d,%d,%d,%d) refMem got=(%d,%d,%d,%d)",
				f,
				prodGot.L0, prodGot.L1, prodGot.L2, prodGot.L3,
				ref.L0, ref.L1, ref.L2, ref.L3,
				refCommitGot.L0, refCommitGot.L1, refCommitGot.L2, refCommitGot.L3)
		}
	}

	prodStats.log(t, "production-commit predictor memory")
	refCommitStats.log(t, "oracle-commit predictor memory")
}

type lspPairSurfaceMode int

const (
	pairSurfaceFinal lspPairSurfaceMode = iota
	pairSurfaceResidualJ1Only
	pairSurfaceNoResidualRearrange
	pairSurfacePostPredictorJ1
	pairSurfaceFinalNoStability
)

func (m lspPairSurfaceMode) String() string {
	switch m {
	case pairSurfaceFinal:
		return "final-J1J2-prepred-stability"
	case pairSurfaceResidualJ1Only:
		return "residual-J1-only"
	case pairSurfaceNoResidualRearrange:
		return "no-residual-rearrange"
	case pairSurfacePostPredictorJ1:
		return "post-predictor-J1"
	case pairSurfaceFinalNoStability:
		return "final-no-stability"
	default:
		return "unknown"
	}
}

func TestINT1D17ColdStartSurfaceVariantDiagnostic(t *testing.T) {
	requireLSPExhaustiveDiag(t)
	if testing.Short() {
		t.Skip("cold-start surface variant diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	modes := []lspPairSurfaceMode{
		pairSurfaceFinal,
		pairSurfaceResidualJ1Only,
		pairSurfaceNoResidualRearrange,
		pairSurfacePostPredictorJ1,
		pairSurfaceFinalNoStability,
	}
	stats := make(map[lspPairSurfaceMode]*lspPairRankStats, len(modes))
	for _, mode := range modes {
		stats[mode] = &lspPairRankStats{}
	}

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])

		got := Quantize(&omega, &freqPrev)
		for _, mode := range modes {
			costs := computeL2L3PairCostMode(refL1, refL0, &memSnap, &omega, mode)
			bestL2, bestL3 := argminPairCost(costs)
			stats[mode].add(f, refL2, refL3, costs, bestL2, bestL3)
			if f == 0 {
				t.Logf("frame0 mode=%s best=(%d,%d) oracle=(%d,%d) rank=%d oracleCost=%d bestCost=%d prodGot=(%d,%d,%d,%d)",
					mode, bestL2, bestL3, refL2, refL3,
					rankPairCost(costs, int(refL2), int(refL3)),
					costs[refL2][refL3], costs[bestL2][bestL3],
					got.L0, got.L1, got.L2, got.L3)
			}
		}
	}

	for _, mode := range modes {
		stats[mode].log(t, mode.String())
	}
}

type lspSecondStageInterpretationMode int

const (
	secondStageCurrent lspSecondStageInterpretationMode = iota
	secondStageNegateLower
	secondStageNegateUpper
	secondStageNegateBoth
	secondStageSwapLowerUpper
	secondStageSwapNegateBoth
)

func (m lspSecondStageInterpretationMode) String() string {
	switch m {
	case secondStageCurrent:
		return "current-L1-plus-L2L3"
	case secondStageNegateLower:
		return "negate-lower"
	case secondStageNegateUpper:
		return "negate-upper"
	case secondStageNegateBoth:
		return "negate-both"
	case secondStageSwapLowerUpper:
		return "swap-L2-L3"
	case secondStageSwapNegateBoth:
		return "swap-L2-L3-negate-both"
	default:
		return "unknown"
	}
}

func TestINT1D18SecondStageInterpretationDiagnostic(t *testing.T) {
	requireLSPExhaustiveDiag(t)
	if testing.Short() {
		t.Skip("second-stage interpretation diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	modes := []lspSecondStageInterpretationMode{
		secondStageCurrent,
		secondStageNegateLower,
		secondStageNegateUpper,
		secondStageNegateBoth,
		secondStageSwapLowerUpper,
		secondStageSwapNegateBoth,
	}
	stats := make(map[lspSecondStageInterpretationMode]*lspPairRankStats, len(modes))
	for _, mode := range modes {
		stats[mode] = &lspPairRankStats{}
	}

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])

		got := Quantize(&omega, &freqPrev)
		for _, mode := range modes {
			costs := computeL2L3PairCostSecondStageMode(refL1, refL0, &memSnap, &omega, mode)
			bestL2, bestL3 := argminPairCost(costs)
			stats[mode].add(f, refL2, refL3, costs, bestL2, bestL3)
			if f == 0 {
				t.Logf("frame0 mode=%s best=(%d,%d) oracle=(%d,%d) rank=%d oracleCost=%d bestCost=%d prodGot=(%d,%d,%d,%d)",
					mode, bestL2, bestL3, refL2, refL3,
					rankPairCost(costs, int(refL2), int(refL3)),
					costs[refL2][refL3], costs[bestL2][bestL3],
					got.L0, got.L1, got.L2, got.L3)
			}
		}
	}

	for _, mode := range modes {
		stats[mode].log(t, mode.String())
	}
}

type lspPredictorVariantMode int

const (
	predictorVariantCurrent lspPredictorVariantMode = iota
	predictorVariantSelectorSwap
	predictorVariantTapReverse
	predictorVariantComp32768
	predictorVariantZeroMemory
	predictorVariantNoPredictor
)

func (m lspPredictorVariantMode) String() string {
	switch m {
	case predictorVariantCurrent:
		return "current-predictor"
	case predictorVariantSelectorSwap:
		return "selector-swap"
	case predictorVariantTapReverse:
		return "tap-reverse"
	case predictorVariantComp32768:
		return "comp-32768"
	case predictorVariantZeroMemory:
		return "zero-memory"
	case predictorVariantNoPredictor:
		return "no-predictor"
	default:
		return "unknown"
	}
}

func TestINT1D19MAPredictorVariantDiagnostic(t *testing.T) {
	requireLSPExhaustiveDiag(t)
	if testing.Short() {
		t.Skip("MA predictor variant diagnostic; -short")
	}

	const (
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.IN")
	bitPath := filepath.Join("..", "..", "testdata", "itu",
		"G729_Release3", "g729", "test_vectors", "LSP.BIT")
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

	modes := []lspPredictorVariantMode{
		predictorVariantCurrent,
		predictorVariantSelectorSwap,
		predictorVariantTapReverse,
		predictorVariantComp32768,
		predictorVariantZeroMemory,
		predictorVariantNoPredictor,
	}
	stats := make(map[lspPredictorVariantMode]*lspPairRankStats, len(modes))
	for _, mode := range modes {
		stats[mode] = &lspPairRankStats{}
	}

	var pp pcm.PreProcessor
	var an lpc.Analyzer
	var oldSpeech [240]int16
	var freqPrev [4][10]int16
	var lspOld [10]int16
	InitFreqPrev(&freqPrev)
	InitLSPOld(&lspOld)

	for f := 0; f < totalFrames; f++ {
		var pcmFrame [samplesPerFrame]int16
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcmFrame[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}

		var processed [samplesPerFrame]int16
		pp.Process(pcmFrame[:], processed[:])
		copy(oldSpeech[0:160], oldSpeech[80:240])
		copy(oldSpeech[160:240], processed[:])

		var aQ12 [lpc.LPCOrder + 1]int16
		if err := an.Analyze(&oldSpeech, &aQ12); err != nil {
			t.Fatalf("frame %d: lpc.Analyze: %v", f, err)
		}

		var qQ15 [10]int16
		if err := LPToLSP(&aQ12, &qQ15); err != nil {
			if err != ErrLPCNonStable {
				t.Fatalf("frame %d: LPToLSP: %v", f, err)
			}
			qQ15 = lspOld
		} else {
			lspOld = qQ15
		}

		var omega [10]int16
		LSPToLSF(&qQ15, &omega)
		memSnap := freqPrev

		bitOff := f * bytesPerBitFrame
		refL0, refL1, refL2, refL3 := extractLSPFieldsFromG192d2(bitData[bitOff : bitOff+bytesPerBitFrame])

		got := Quantize(&omega, &freqPrev)
		for _, mode := range modes {
			costs := computeL2L3PairCostPredictorVariant(refL1, refL0, &memSnap, &omega, mode)
			bestL2, bestL3 := argminPairCost(costs)
			stats[mode].add(f, refL2, refL3, costs, bestL2, bestL3)
			if f == 0 {
				t.Logf("frame0 mode=%s best=(%d,%d) oracle=(%d,%d) rank=%d oracleCost=%d bestCost=%d prodGot=(%d,%d,%d,%d)",
					mode, bestL2, bestL3, refL2, refL3,
					rankPairCost(costs, int(refL2), int(refL3)),
					costs[refL2][refL3], costs[bestL2][bestL3],
					got.L0, got.L1, got.L2, got.L3)
			}
		}
	}

	for _, mode := range modes {
		stats[mode].log(t, mode.String())
	}
}

func finalLSPTupleCost(idx Indices, mem *[4][10]int16, omega, weights *[10]int16) int64 {
	var residual, omegaHat [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = tables.LSPCodebookL1[idx.L1][i] + tables.LSPCodebookL2[idx.L2][i]
		residual[5+i] = tables.LSPCodebookL1[idx.L1][5+i] + tables.LSPCodebookL3[idx.L3][i]
	}
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)
	applyPredictorWithMemory(idx.L0, mem, &residual, &omegaHat)
	enforceLSFStability(&omegaHat)

	var cost int64
	for i := 0; i < 10; i++ {
		d := int64(omega[i]) - int64(omegaHat[i])
		cost += int64(weights[i]) * d * d
	}
	return cost
}

func quantizeNoCommit(omega *[10]int16, freqPrev *[4][10]int16) (Indices, [10]int16) {
	var weights [10]int16
	weightsLSF(omega, &weights)

	var (
		bestSel                uint8
		bestL1, bestL2, bestL3 uint8
		bestResidual           [10]int16
		bestCost               int64 = -1

		target   [10]int16
		residual [10]int16
		omegaHat [10]int16
	)

	for sel := uint8(0); sel < 2; sel++ {
		computeTargetLSF(sel, freqPrev, omega, &target)
		l1, _ := searchL1(&target)
		l2, _ := searchL2(uint8(l1), sel, freqPrev, omega, &weights)
		l3, _ := searchL3(uint8(l1), uint8(l2), sel, freqPrev, omega, &weights)

		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[l2][i]
			residual[5+i] = tables.LSPCodebookL1[l1][5+i] + tables.LSPCodebookL3[l3][i]
		}
		rearrangeAdjacent(&residual, lsfRearrJ1)
		rearrangeAdjacent(&residual, lsfRearrJ2)
		applyPredictorWithMemory(sel, freqPrev, &residual, &omegaHat)
		enforceLSFStability(&omegaHat)

		var cost int64
		for i := 0; i < 10; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			cost += int64(weights[i]) * d * d
		}

		if bestCost < 0 || cost < bestCost {
			bestCost = cost
			bestSel = sel
			bestL1 = uint8(l1)
			bestL2 = uint8(l2)
			bestL3 = uint8(l3)
			bestResidual = residual
		}
	}

	return Indices{L0: bestSel, L1: bestL1, L2: bestL2, L3: bestL3}, bestResidual
}

func residualForIndices(idx Indices) [10]int16 {
	var residual [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = tables.LSPCodebookL1[idx.L1][i] + tables.LSPCodebookL2[idx.L2][i]
		residual[5+i] = tables.LSPCodebookL1[idx.L1][5+i] + tables.LSPCodebookL3[idx.L3][i]
	}
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)
	return residual
}

func computeL2L3PairCostSecondStageMode(l1, selector uint8, mem *[4][10]int16, omega *[10]int16, mode lspSecondStageInterpretationMode) [32][32]int64 {
	var weights [10]int16
	weightsLSF(omega, &weights)

	var out [32][32]int64
	var residual, omegaHat [10]int16
	for l2 := 0; l2 < 32; l2++ {
		for l3 := 0; l3 < 32; l3++ {
			fillResidualSecondStageMode(l1, uint8(l2), uint8(l3), mode, &residual)
			rearrangedResidualToLSF(selector, mem, &residual, &omegaHat)
			var cost int64
			for i := 0; i < 10; i++ {
				d := int64(omega[i]) - int64(omegaHat[i])
				cost += int64(weights[i]) * d * d
			}
			out[l2][l3] = cost
		}
	}
	return out
}

func fillResidualSecondStageMode(l1, l2, l3 uint8, mode lspSecondStageInterpretationMode, residual *[10]int16) {
	for i := 0; i < 5; i++ {
		lower := tables.LSPCodebookL2[l2][i]
		upper := tables.LSPCodebookL3[l3][i]
		switch mode {
		case secondStageNegateLower:
			lower = -lower
		case secondStageNegateUpper:
			upper = -upper
		case secondStageNegateBoth:
			lower = -lower
			upper = -upper
		case secondStageSwapLowerUpper:
			lower = tables.LSPCodebookL3[l2][i]
			upper = tables.LSPCodebookL2[l3][i]
		case secondStageSwapNegateBoth:
			lower = -tables.LSPCodebookL3[l2][i]
			upper = -tables.LSPCodebookL2[l3][i]
		}
		residual[i] = tables.LSPCodebookL1[l1][i] + lower
		residual[5+i] = tables.LSPCodebookL1[l1][5+i] + upper
	}
}

func computeL2L3PairCostPredictorVariant(l1, selector uint8, mem *[4][10]int16, omega *[10]int16, mode lspPredictorVariantMode) [32][32]int64 {
	var weights [10]int16
	weightsLSF(omega, &weights)

	var out [32][32]int64
	var residual, omegaHat [10]int16
	for l2 := 0; l2 < 32; l2++ {
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[l2][i]
		}
		for l3 := 0; l3 < 32; l3++ {
			for i := 0; i < 5; i++ {
				residual[5+i] = tables.LSPCodebookL1[l1][5+i] + tables.LSPCodebookL3[l3][i]
			}
			reconstructResidualToLSFPredictorVariant(selector, mem, &residual, &omegaHat, mode)
			var cost int64
			for i := 0; i < 10; i++ {
				d := int64(omega[i]) - int64(omegaHat[i])
				cost += int64(weights[i]) * d * d
			}
			out[l2][l3] = cost
		}
	}
	return out
}

func reconstructResidualToLSFPredictorVariant(selector uint8, mem *[4][10]int16, residual, omegaHat *[10]int16, mode lspPredictorVariantMode) {
	tmp := *residual
	rearrangeAdjacent(&tmp, lsfRearrJ1)
	rearrangeAdjacent(&tmp, lsfRearrJ2)

	switch mode {
	case predictorVariantCurrent:
		applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
	case predictorVariantSelectorSwap:
		applyPredictorWithMemory(selector^1, mem, &tmp, omegaHat)
	case predictorVariantTapReverse:
		applyPredictorTapReversedExact(selector, mem, &tmp, omegaHat)
	case predictorVariantComp32768:
		applyPredictorComp32768(selector, mem, &tmp, omegaHat)
	case predictorVariantZeroMemory:
		var zero [4][10]int16
		applyPredictorWithMemory(selector, &zero, &tmp, omegaHat)
	case predictorVariantNoPredictor:
		*omegaHat = tmp
	default:
		applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
	}
	enforceLSFStability(omegaHat)
}

func applyPredictorTapReversedExact(selector uint8, mem *[4][10]int16, residual, out *[10]int16) {
	preds := &tables.MAPredictorsLSP[selector]
	for i := 0; i < 10; i++ {
		var sumP int16
		for k := 0; k < 4; k++ {
			sumP = fixed.Add(sumP, preds[k][i])
		}
		comp := fixed.Sub(fixed.Max16, sumP)

		var acc fixed.Word32
		acc = fixed.LMac(acc, comp, residual[i])
		acc = fixed.LMac(acc, preds[3][i], mem[0][i])
		acc = fixed.LMac(acc, preds[2][i], mem[1][i])
		acc = fixed.LMac(acc, preds[1][i], mem[2][i])
		acc = fixed.LMac(acc, preds[0][i], mem[3][i])
		out[i] = fixed.Round(acc)
	}
}

func applyPredictorComp32768(selector uint8, mem *[4][10]int16, residual, out *[10]int16) {
	preds := &tables.MAPredictorsLSP[selector]
	for i := 0; i < 10; i++ {
		var sumP int32
		for k := 0; k < 4; k++ {
			sumP += int32(preds[k][i])
		}
		comp := int64(32768 - sumP)
		acc := comp * int64(residual[i]) * 2
		for k := 0; k < 4; k++ {
			acc += int64(preds[k][i]) * int64(mem[k][i]) * 2
		}
		v := (acc + (1 << 15)) >> 16
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[i] = int16(v)
	}
}

func computeL2L3PairCostMode(l1, selector uint8, mem *[4][10]int16, omega *[10]int16, mode lspPairSurfaceMode) [32][32]int64 {
	var weights [10]int16
	weightsLSF(omega, &weights)

	var out [32][32]int64
	var residual, omegaHat [10]int16
	for l2 := 0; l2 < 32; l2++ {
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[l2][i]
		}
		for l3 := 0; l3 < 32; l3++ {
			for i := 0; i < 5; i++ {
				residual[5+i] = tables.LSPCodebookL1[l1][5+i] + tables.LSPCodebookL3[l3][i]
			}
			reconstructResidualToLSFMode(selector, mem, &residual, &omegaHat, mode)
			var cost int64
			for i := 0; i < 10; i++ {
				d := int64(omega[i]) - int64(omegaHat[i])
				cost += int64(weights[i]) * d * d
			}
			out[l2][l3] = cost
		}
	}
	return out
}

func reconstructResidualToLSFMode(selector uint8, mem *[4][10]int16, residual, omegaHat *[10]int16, mode lspPairSurfaceMode) {
	tmp := *residual
	switch mode {
	case pairSurfaceFinal:
		rearrangeAdjacent(&tmp, lsfRearrJ1)
		rearrangeAdjacent(&tmp, lsfRearrJ2)
		applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
		enforceLSFStability(omegaHat)
	case pairSurfaceResidualJ1Only:
		rearrangeAdjacent(&tmp, lsfRearrJ1)
		applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
		enforceLSFStability(omegaHat)
	case pairSurfaceNoResidualRearrange:
		applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
		enforceLSFStability(omegaHat)
	case pairSurfacePostPredictorJ1:
		applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
		rearrangeAdjacent(omegaHat, lsfRearrJ1)
		enforceLSFStability(omegaHat)
	case pairSurfaceFinalNoStability:
		rearrangeAdjacent(&tmp, lsfRearrJ1)
		rearrangeAdjacent(&tmp, lsfRearrJ2)
		applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
	default:
		rearrangeAdjacent(&tmp, lsfRearrJ1)
		rearrangeAdjacent(&tmp, lsfRearrJ2)
		applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
		enforceLSFStability(omegaHat)
	}
}

func computeL2L3PairCost(l1, selector uint8, mem *[4][10]int16, omega, weights *[10]int16) [32][32]int64 {
	var out [32][32]int64
	var residual, omegaHat [10]int16
	for l2 := 0; l2 < 32; l2++ {
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[l2][i]
		}
		for l3 := 0; l3 < 32; l3++ {
			for i := 0; i < 5; i++ {
				residual[5+i] = tables.LSPCodebookL1[l1][5+i] + tables.LSPCodebookL3[l3][i]
			}
			reconstructResidualCost(selector, mem, &residual, omega, weights, &omegaHat, &out[l2][l3])
		}
	}
	return out
}

func computeL2L3PairCostUnweighted(l1, selector uint8, mem *[4][10]int16, omega *[10]int16) [32][32]int64 {
	var out [32][32]int64
	var residual, omegaHat [10]int16
	for l2 := 0; l2 < 32; l2++ {
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[l2][i]
		}
		for l3 := 0; l3 < 32; l3++ {
			for i := 0; i < 5; i++ {
				residual[5+i] = tables.LSPCodebookL1[l1][5+i] + tables.LSPCodebookL3[l3][i]
			}
			rearrangedResidualToLSF(selector, mem, &residual, &omegaHat)
			var cost int64
			for i := 0; i < 10; i++ {
				d := int64(omega[i]) - int64(omegaHat[i])
				cost += d * d
			}
			out[l2][l3] = cost
		}
	}
	return out
}

func reconstructResidualCost(selector uint8, mem *[4][10]int16, residual, omega, weights, omegaHat *[10]int16, out *int64) {
	rearrangedResidualToLSF(selector, mem, residual, omegaHat)
	var cost int64
	for i := 0; i < 10; i++ {
		d := int64(omega[i]) - int64(omegaHat[i])
		cost += int64(weights[i]) * d * d
	}
	*out = cost
}

func rearrangedResidualToLSF(selector uint8, mem *[4][10]int16, residual, omegaHat *[10]int16) {
	tmp := *residual
	rearrangeAdjacent(&tmp, lsfRearrJ1)
	rearrangeAdjacent(&tmp, lsfRearrJ2)
	applyPredictorWithMemory(selector, mem, &tmp, omegaHat)
	enforceLSFStability(omegaHat)
}

func argminPairCost(costs [32][32]int64) (uint8, uint8) {
	bestL2, bestL3 := 0, 0
	for l2 := 0; l2 < 32; l2++ {
		for l3 := 0; l3 < 32; l3++ {
			if costs[l2][l3] < costs[bestL2][bestL3] {
				bestL2, bestL3 = l2, l3
			}
		}
	}
	return uint8(bestL2), uint8(bestL3)
}

func rankPairCost(costs [32][32]int64, wantL2, wantL3 int) int {
	rank := 1
	want := costs[wantL2][wantL3]
	for l2 := 0; l2 < 32; l2++ {
		for l3 := 0; l3 < 32; l3++ {
			if costs[l2][l3] < want {
				rank++
			}
		}
	}
	return rank
}

func bestPairCost(costs [32][32]int64) (uint8, uint8) {
	bestL2, bestL3 := 0, 0
	for l2 := 0; l2 < 32; l2++ {
		for l3 := 0; l3 < 32; l3++ {
			if costs[l2][l3] < costs[bestL2][bestL3] {
				bestL2, bestL3 = l2, l3
			}
		}
	}
	return uint8(bestL2), uint8(bestL3)
}

func rankL1Target(target *[10]int16, want int) (rank int, best int, wantCost int64, bestCost int64) {
	rank = 1
	best = 0
	bestCost = -1
	for row := 0; row < len(tables.LSPCodebookL1); row++ {
		var cost int64
		for i := 0; i < 10; i++ {
			d := int64(target[i]) - int64(tables.LSPCodebookL1[row][i])
			cost += d * d
		}
		if row == want {
			wantCost = cost
		}
		if bestCost < 0 || cost < bestCost {
			bestCost = cost
			best = row
		}
	}
	for row := 0; row < len(tables.LSPCodebookL1); row++ {
		if row == want {
			continue
		}
		var cost int64
		for i := 0; i < 10; i++ {
			d := int64(target[i]) - int64(tables.LSPCodebookL1[row][i])
			cost += d * d
		}
		if cost < wantCost {
			rank++
		}
	}
	return rank, best, wantCost, bestCost
}

func rankFullLSPTuple(want Indices, mem *[4][10]int16, omega, weights *[10]int16) int {
	wantCost := finalLSPTupleCost(want, mem, omega, weights)
	rank := 1
	for sel := uint8(0); sel < 2; sel++ {
		for l1 := uint8(0); l1 < 128; l1++ {
			for l2 := uint8(0); l2 < 32; l2++ {
				for l3 := uint8(0); l3 < 32; l3++ {
					idx := Indices{L0: sel, L1: l1, L2: l2, L3: l3}
					if idx == want {
						continue
					}
					if finalLSPTupleCost(idx, mem, omega, weights) < wantCost {
						rank++
					}
				}
			}
		}
	}
	return rank
}

func impliedUniformInitialMemoryForFrame0(ref Indices, omega *[10]int16) [10]int16 {
	residual := residualForIndices(ref)
	preds := &tables.MAPredictorsLSP[ref.L0]
	var mem [10]int16
	for i := 0; i < 10; i++ {
		var sumP int32
		for k := 0; k < 4; k++ {
			sumP += int32(preds[k][i])
		}
		comp := int32(32768) - sumP
		compR := roundQ15Product(comp, int32(residual[i]))
		predContribution := int32(omega[i]) - compR
		mem[i] = int16(roundDivSigned(predContribution*32768, sumP))
	}
	return mem
}

func roundQ15Product(a, b int32) int32 {
	return roundDivSigned(a*b, 32768)
}

func roundDivSigned(num, den int32) int32 {
	if den < 0 {
		num = -num
		den = -den
	}
	if num >= 0 {
		return (num + den/2) / den
	}
	return -((-num + den/2) / den)
}

func computeL2PerRowCostUnweighted(l1, selector uint8, mem *[4][10]int16, omega *[10]int16) [32]int64 {
	var out [32]int64
	var residual, omegaHat [10]int16

	for row := 0; row < 32; row++ {
		for i := 0; i < 5; i++ {
			residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[row][i]
		}
		applyPredictorWithMemory(selector, mem, &residual, &omegaHat)
		for i := 1; i < 5; i++ {
			if omegaHat[i]-omegaHat[i-1] < lsfRearrJ1 {
				sum := int32(omegaHat[i]) + int32(omegaHat[i-1])
				omegaHat[i-1] = int16((sum - int32(lsfRearrJ1)) / 2)
				omegaHat[i] = int16((sum + int32(lsfRearrJ1)) / 2)
			}
		}
		var mse int64
		for i := 0; i < 5; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			mse += d * d
		}
		out[row] = mse
	}
	return out
}

func computeL3PerRowCostUnweighted(l1, l2, selector uint8, mem *[4][10]int16, omega *[10]int16) [32]int64 {
	var out [32]int64
	var residual, omegaHat [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = tables.LSPCodebookL1[l1][i] + tables.LSPCodebookL2[l2][i]
	}
	for row := 0; row < 32; row++ {
		for i := 0; i < 5; i++ {
			residual[5+i] = tables.LSPCodebookL1[l1][5+i] + tables.LSPCodebookL3[row][i]
		}
		applyPredictorWithMemory(selector, mem, &residual, &omegaHat)
		rearrangeAdjacent(&omegaHat, lsfRearrJ1)

		var mse int64
		for i := 5; i < 10; i++ {
			d := int64(omega[i]) - int64(omegaHat[i])
			mse += d * d
		}
		out[row] = mse
	}
	return out
}

func weightsLSFNoBoost(omega *[10]int16, w *[10]int16) {
	w[0] = weightFromArg(omega[1] - lsfQ13Pi04 - lsfQ13One)
	for i := 1; i <= 8; i++ {
		w[i] = weightFromArg(omega[i+1] - omega[i-1] - lsfQ13One)
	}
	w[9] = weightFromArg(lsfQ13Pi92 - omega[8] - lsfQ13One)
}

func reconstructLSFTuple(idx Indices, mem *[4][10]int16, out *[10]int16) {
	var residual [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = tables.LSPCodebookL1[idx.L1][i] + tables.LSPCodebookL2[idx.L2][i]
		residual[5+i] = tables.LSPCodebookL1[idx.L1][5+i] + tables.LSPCodebookL3[idx.L3][i]
	}
	rearrangeAdjacent(&residual, lsfRearrJ1)
	rearrangeAdjacent(&residual, lsfRearrJ2)
	applyPredictorWithMemory(idx.L0, mem, &residual, out)
	enforceLSFStability(out)
}

func rankCost32(costs [32]int64, idx int) int {
	rank := 1
	for i := range costs {
		if costs[i] < costs[idx] {
			rank++
		}
	}
	return rank
}

func firstDivergentLSPField(got, ref Indices) string {
	if got.L0 != ref.L0 {
		return "L0-first"
	}
	if got.L1 != ref.L1 {
		return "L1-first"
	}
	if got.L2 != ref.L2 {
		return "L2-first"
	}
	if got.L3 != ref.L3 {
		return "L3-first"
	}
	return "all-match"
}

func logOmegaSourceTraceFrame(t *testing.T, frame int, got, ref Indices, aQ12 *[11]int16, qQ15, omega, gotHat, refHat *[10]int16) {
	var refHatQ, gotHatQ, omegaMinusRef, omegaMinusGot, qMinusRefHatQ [10]int16
	for i := 0; i < 10; i++ {
		refHatQ[i] = lsfToLSP(refHat[i])
		gotHatQ[i] = lsfToLSP(gotHat[i])
		omegaMinusRef[i] = omega[i] - refHat[i]
		omegaMinusGot[i] = omega[i] - gotHat[i]
		qMinusRefHatQ[i] = qQ15[i] - refHatQ[i]
	}
	t.Logf("trace frame=%d got=(%d,%d,%d,%d) ref=(%d,%d,%d,%d)",
		frame, got.L0, got.L1, got.L2, got.L3, ref.L0, ref.L1, ref.L2, ref.L3)
	t.Logf("trace frame=%d aQ12=%v maxAbsA=%d", frame, *aQ12, maxAbsA(aQ12))
	t.Logf("trace frame=%d qQ15=%v omega=%v", frame, *qQ15, *omega)
	t.Logf("trace frame=%d gotHat=%v gotHatQ=%v omega-gotHat=%v",
		frame, *gotHat, gotHatQ, omegaMinusGot)
	t.Logf("trace frame=%d refHat=%v refHatQ=%v omega-refHat=%v q-refHatQ=%v",
		frame, *refHat, refHatQ, omegaMinusRef, qMinusRefHatQ)
}

func lsfDistance(a, b *[10]int16) int64 {
	var out int64
	for i := 0; i < 10; i++ {
		d := int64(a[i]) - int64(b[i])
		out += d * d
	}
	return out
}

func lsfWeightedDistance(a, b, weights *[10]int16) int64 {
	var out int64
	for i := 0; i < 10; i++ {
		d := int64(a[i]) - int64(b[i])
		out += int64(weights[i]) * d * d
	}
	return out
}

func lsfAbsDistance(a, b *[10]int16) int64 {
	var out int64
	for i := 0; i < 10; i++ {
		d := int64(a[i]) - int64(b[i])
		if d < 0 {
			d = -d
		}
		out += d
	}
	return out
}

func maxAbsA(a *[11]int16) int16 {
	var max int16
	for i := 1; i < len(a); i++ {
		v := a[i]
		if v < 0 {
			v = -v
		}
		if v > max {
			max = v
		}
	}
	return max
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}
