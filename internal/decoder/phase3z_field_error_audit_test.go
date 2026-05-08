package decoder

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/pitch"
)

// TestPhase3zFieldErrorAudit groups local-vs-FFmpeg frame errors by decoded
// bitstream fields. It is diagnostic-only and keeps FFmpeg as an executable
// black-box decoder.
func TestPhase3zFieldErrorAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_FIELD_ERROR_AUDIT") != "1" {
		t.Skip("set G729_DECODER_FIELD_ERROR_AUDIT=1 to run field/error audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	bitPath := vectorPath("SPEECH.BIT")
	ensureTestdataPresent(t, bitPath)
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	speechFrames := len(bitData) / bitstream.G192FrameBytes
	speechRef := phase3uFFmpegDecodeG192(t, bitData, speechFrames, "speech-bit")
	speechGot := decodeVariant(t, bitData, speechFrames, nil, nil)
	speechFields := phase3zReadG192Fields(t, bitData, speechFrames)
	phase3zReport(t, "SPEECH.BIT", speechRef, speechGot, speechFields)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk field/error audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	astGot := phase3tDecodeRawVariant(t, raw, astFrames, phase3eVariant{name: "production"})
	astFields := phase3zReadRawFields(t, raw, astFrames)
	phase3zReport(t, "Asterisk", astRef, astGot, astFields)
}

type phase3zFrameFields struct {
	frame int
	f     bitstream.Frame
	t1    int
	frac1 int
	t2    int
	frac2 int
}

type phase3zFrameMetric struct {
	fields phase3zFrameFields
	refRMS float64
	gotRMS float64
	ratio  float64
	corr   float64
	snr    float64
}

type phase3zGroupStats struct {
	name     string
	total    int
	lowRatio int
	lowCorr  int
	negCorr  int
	ratioSum float64
	corrSum  float64
	snrSum   float64
	ratios   []float64
	corrs    []float64
}

func phase3zReadG192Fields(t *testing.T, bitData []byte, frames int) []phase3zFrameFields {
	t.Helper()
	out := make([]phase3zFrameFields, 0, frames)
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for frame := 0; frame < frames; frame++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", frame, err)
		}
		out = append(out, phase3zDecodeFields(t, frame, packed[:]))
	}
	return out
}

func phase3zReadRawFields(t *testing.T, raw []byte, frames int) []phase3zFrameFields {
	t.Helper()
	out := make([]phase3zFrameFields, 0, frames)
	for frame := 0; frame < frames; frame++ {
		packed := raw[frame*bitstream.FrameBytes : (frame+1)*bitstream.FrameBytes]
		out = append(out, phase3zDecodeFields(t, frame, packed))
	}
	return out
}

func phase3zDecodeFields(t *testing.T, frame int, packed []byte) phase3zFrameFields {
	t.Helper()
	var f bitstream.Frame
	if err := bitstream.Unpack(packed, &f); err != nil {
		t.Fatalf("Unpack frame %d: %v", frame, err)
	}
	t1, frac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
	t2, frac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), t1)
	return phase3zFrameFields{frame: frame, f: f, t1: t1, frac1: frac1, t2: t2, frac2: frac2}
}

func phase3zReport(t *testing.T, label string, ref, got []int16, fields []phase3zFrameFields) {
	t.Helper()
	metrics := phase3zFrameMetrics(ref, got, fields)
	t.Logf("Phase 3z field/error audit - %s", label)
	t.Logf("active frames: %d", len(metrics))
	phase3zLogGroups(t, "frac", metrics, phase3zFracGroup)
	phase3zLogGroups(t, "pitch-range", metrics, phase3zPitchRangeGroup)
	phase3zLogGroups(t, "L0", metrics, phase3zL0Group)
	phase3zLogGroups(t, "gain-A", metrics, phase3zGainAGroup)
	phase3zLogWorst(t, metrics, 10)
}

func phase3zFrameMetrics(ref, got []int16, fields []phase3zFrameFields) []phase3zFrameMetric {
	frames := len(ref) / frameSamples
	if gf := len(got) / frameSamples; gf < frames {
		frames = gf
	}
	if ff := len(fields); ff < frames {
		frames = ff
	}
	metrics := make([]phase3zFrameMetric, 0, frames)
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		r := ref[off : off+frameSamples]
		g := got[off : off+frameSamples]
		refRMS := envelopeRMS(r)
		if refRMS < 500 {
			continue
		}
		gotRMS := envelopeRMS(g)
		metrics = append(metrics, phase3zFrameMetric{
			fields: fields[frame],
			refRMS: refRMS,
			gotRMS: gotRMS,
			ratio:  gotRMS / refRMS,
			corr:   envelopeCorr(r, g),
			snr:    envelopeSNRDB(r, g),
		})
	}
	return metrics
}

func phase3zLogGroups(t *testing.T, label string, metrics []phase3zFrameMetric, groupFn func(phase3zFrameMetric) string) {
	t.Helper()
	groups := map[string]*phase3zGroupStats{}
	for _, m := range metrics {
		name := groupFn(m)
		g := groups[name]
		if g == nil {
			g = &phase3zGroupStats{name: name}
			groups[name] = g
		}
		g.add(m)
	}
	rows := make([]*phase3zGroupStats, 0, len(groups))
	for _, g := range groups {
		rows = append(rows, g)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].lowRatio != rows[j].lowRatio {
			return rows[i].lowRatio > rows[j].lowRatio
		}
		return rows[i].name < rows[j].name
	})
	t.Logf("%s groups:", label)
	t.Logf("  %-18s %7s %9s %9s %9s %9s %9s %9s",
		"group", "frames", "low<0.5", "corr<0.3", "ratioMed", "corrMed", "snrMean", "low%")
	for _, g := range rows {
		t.Logf("  %-18s %7d %9d %9d %9.3f %9.3f %9.2f %8.1f%%",
			g.name, g.total, g.lowRatio, g.lowCorr, g.medianRatio(), g.medianCorr(), g.meanSNR(), g.lowRatioPercent())
	}
}

func (g *phase3zGroupStats) add(m phase3zFrameMetric) {
	g.total++
	g.ratioSum += m.ratio
	g.corrSum += m.corr
	g.snrSum += m.snr
	g.ratios = append(g.ratios, m.ratio)
	g.corrs = append(g.corrs, m.corr)
	if m.ratio < 0.5 {
		g.lowRatio++
	}
	if m.corr < 0.3 {
		g.lowCorr++
	}
	if m.corr < 0 {
		g.negCorr++
	}
}

func (g *phase3zGroupStats) medianRatio() float64 {
	return phase3zMedian(g.ratios)
}

func (g *phase3zGroupStats) medianCorr() float64 {
	return phase3zMedian(g.corrs)
}

func (g *phase3zGroupStats) meanSNR() float64 {
	if g.total == 0 {
		return 0
	}
	return g.snrSum / float64(g.total)
}

func (g *phase3zGroupStats) lowRatioPercent() float64 {
	if g.total == 0 {
		return 0
	}
	return 100 * float64(g.lowRatio) / float64(g.total)
}

func phase3zMedian(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	tmp := append([]float64(nil), vals...)
	sort.Float64s(tmp)
	mid := len(tmp) / 2
	if len(tmp)%2 == 1 {
		return tmp[mid]
	}
	return 0.5 * (tmp[mid-1] + tmp[mid])
}

func phase3zFracGroup(m phase3zFrameMetric) string {
	f1 := m.fields.frac1 != 0
	f2 := m.fields.frac2 != 0
	switch {
	case f1 && f2:
		return "both_nonzero"
	case f1:
		return "sf1_nonzero"
	case f2:
		return "sf2_nonzero"
	default:
		return "both_zero"
	}
}

func phase3zPitchRangeGroup(m phase3zFrameMetric) string {
	short := m.fields.t1 < subframeLen || m.fields.t2 < subframeLen
	long := m.fields.t1 >= 80 && m.fields.t2 >= 80
	switch {
	case short:
		return "has_T_lt_40"
	case long:
		return "both_T_ge_80"
	default:
		return "mid_mix"
	}
}

func phase3zL0Group(m phase3zFrameMetric) string {
	return fmt.Sprintf("L0_%d", m.fields.f.L0)
}

func phase3zGainAGroup(m phase3zFrameMetric) string {
	return fmt.Sprintf("GA%d_%d", m.fields.f.GA1, m.fields.f.GA2)
}

func phase3zLogWorst(t *testing.T, metrics []phase3zFrameMetric, n int) {
	t.Helper()
	bySNR := append([]phase3zFrameMetric(nil), metrics...)
	sort.Slice(bySNR, func(i, j int) bool {
		return bySNR[i].snr < bySNR[j].snr
	})
	if n > len(bySNR) {
		n = len(bySNR)
	}
	t.Logf("worst %d active frames by frame SNR:", n)
	for i := 0; i < n; i++ {
		m := bySNR[i]
		f := m.fields.f
		t.Logf("  frame=%d snr=%.2f ratio=%.3f corr=%.3f T=(%d,%+d)/(%d,%+d) L=(%d,%d,%d,%d) P=(%d,%d,%d) C=(%d,%d) S=(%d,%d) G=(%d,%d)/(%d,%d)",
			m.fields.frame, m.snr, m.ratio, m.corr,
			m.fields.t1, m.fields.frac1, m.fields.t2, m.fields.frac2,
			f.L0, f.L1, f.L2, f.L3, f.P0, f.P1, f.P2,
			f.C1, f.C2, f.S1, f.S2, f.GA1, f.GB1, f.GA2, f.GB2)
	}
}
