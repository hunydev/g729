package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3agGainClusterContextAudit explains when the GA036_x3/2 candidate
// helps or hurts by grouping frame deltas by the production local-vs-FFmpeg
// envelope ratio. FFmpeg is used only as an executable black-box decoder.
func TestPhase3agGainClusterContextAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_CLUSTER_CONTEXT_AUDIT") != "1" {
		t.Skip("set G729_DECODER_GAIN_CLUSTER_CONTEXT_AUDIT=1 to run gain-cluster context audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	candidate := phase3afFindScale("GA036_x3/2")
	t.Logf("Phase 3ag gain-cluster context audit - %s", candidate.name)

	for _, vector := range []string{"SPEECH.BIT", "FIXED.BIT", "LSP.BIT", "PITCH.BIT", "TEST.BIT", "ALGTHM.BIT"} {
		path := vectorPath(vector)
		ensureTestdataPresent(t, path)
		bitData, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", vector, err)
		}
		frames := len(bitData) / bitstream.G192FrameBytes
		if frames == 0 {
			continue
		}
		ref := phase3uFFmpegDecodeG192(t, bitData, frames, vector)
		prod := decodeVariant(t, bitData, frames, nil, nil)
		cand := phase3adDecodeG192Scale(t, bitData, frames, candidate)
		phase3agReport(t, vector, ref, prod, cand)
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk context audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, "asterisk")
	prod := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "production"})
	cand := phase3adDecodeRawScale(t, raw, frames, candidate)
	phase3agReport(t, "Asterisk", ref, prod, cand)
}

type phase3agFrame struct {
	prodRatio float64
	candRatio float64
	prodSNR   float64
	candSNR   float64
	prodCorr  float64
	candCorr  float64
	refRMS    float64
	prodRMS   float64
	candRMS   float64
}

type phase3agBin struct {
	name      string
	lo        float64
	hi        float64
	frames    []phase3agFrame
	improved  int
	regressed int
}

func phase3agReport(t *testing.T, label string, ref, prod, cand []int16) {
	t.Helper()
	frames := phase3agFrames(ref, prod, cand)
	bins := []phase3agBin{
		{name: "ratio<0.50", lo: -1, hi: 0.50},
		{name: "0.50..0.75", lo: 0.50, hi: 0.75},
		{name: "0.75..1.25", lo: 0.75, hi: 1.25},
		{name: "1.25..1.75", lo: 1.25, hi: 1.75},
		{name: "ratio>=1.75", lo: 1.75, hi: 1e9},
	}
	for _, f := range frames {
		for i := range bins {
			if f.prodRatio >= bins[i].lo && f.prodRatio < bins[i].hi {
				bins[i].frames = append(bins[i].frames, f)
				d := f.candSNR - f.prodSNR
				if d > 0.10 {
					bins[i].improved++
				} else if d < -0.10 {
					bins[i].regressed++
				}
				break
			}
		}
	}

	prodM := blackboxMeasure(ref, prod, 40)
	candM := blackboxMeasure(ref, cand, 40)
	prodEnv := phase3pEnvelopeCompare(ref, prod)
	candEnv := phase3pEnvelopeCompare(ref, cand)
	t.Logf("%s summary: production gSNR %.2f seg %.2f corr %.3f ratioMed %.3f low<0.5 %d ; candidate gSNR %.2f seg %.2f corr %.3f ratioMed %.3f low<0.5 %d",
		label,
		prodM.globalSNR, prodM.segSNR, prodM.corr, prodEnv.ratioMedian, prodEnv.lowRatioFrames,
		candM.globalSNR, candM.segSNR, candM.corr, candEnv.ratioMedian, candEnv.lowRatioFrames)
	t.Logf("%s bins:", label)
	t.Logf("  %-12s %7s %9s %9s %9s %9s %9s %9s %9s",
		"prodRatio", "frames", "dSNRavg", "dSNRmed", "dCorravg", "dRatio", "improved", "regressed", "refRMS")
	for _, b := range bins {
		t.Logf("  %-12s %7d %9.2f %9.2f %9.3f %9.3f %9d %9d %9.1f",
			b.name, len(b.frames), phase3agMeanDeltaSNR(b.frames), phase3agMedianDeltaSNR(b.frames),
			phase3agMeanDeltaCorr(b.frames), phase3agMeanDeltaRatio(b.frames),
			b.improved, b.regressed, phase3agMeanRefRMS(b.frames))
	}
}

func phase3agFrames(ref, prod, cand []int16) []phase3agFrame {
	frames := len(ref) / frameSamples
	if pf := len(prod) / frameSamples; pf < frames {
		frames = pf
	}
	if cf := len(cand) / frameSamples; cf < frames {
		frames = cf
	}
	out := make([]phase3agFrame, 0, frames)
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		r := ref[off : off+frameSamples]
		p := prod[off : off+frameSamples]
		c := cand[off : off+frameSamples]
		refRMS := envelopeRMS(r)
		if refRMS < 500 {
			continue
		}
		prodRMS := envelopeRMS(p)
		candRMS := envelopeRMS(c)
		out = append(out, phase3agFrame{
			prodRatio: prodRMS / refRMS,
			candRatio: candRMS / refRMS,
			prodSNR:   envelopeSNRDB(r, p),
			candSNR:   envelopeSNRDB(r, c),
			prodCorr:  envelopeCorr(r, p),
			candCorr:  envelopeCorr(r, c),
			refRMS:    refRMS,
			prodRMS:   prodRMS,
			candRMS:   candRMS,
		})
	}
	return out
}

func phase3agMeanDeltaSNR(frames []phase3agFrame) float64 {
	if len(frames) == 0 {
		return 0
	}
	var sum float64
	for _, f := range frames {
		sum += f.candSNR - f.prodSNR
	}
	return sum / float64(len(frames))
}

func phase3agMedianDeltaSNR(frames []phase3agFrame) float64 {
	if len(frames) == 0 {
		return 0
	}
	vals := make([]float64, len(frames))
	for i, f := range frames {
		vals[i] = f.candSNR - f.prodSNR
	}
	sort.Float64s(vals)
	return envelopePercentile(vals, 0.5)
}

func phase3agMeanDeltaCorr(frames []phase3agFrame) float64 {
	if len(frames) == 0 {
		return 0
	}
	var sum float64
	for _, f := range frames {
		sum += f.candCorr - f.prodCorr
	}
	return sum / float64(len(frames))
}

func phase3agMeanDeltaRatio(frames []phase3agFrame) float64 {
	if len(frames) == 0 {
		return 0
	}
	var sum float64
	for _, f := range frames {
		sum += f.candRatio - f.prodRatio
	}
	return sum / float64(len(frames))
}

func phase3agMeanRefRMS(frames []phase3agFrame) float64 {
	if len(frames) == 0 {
		return 0
	}
	var sum float64
	for _, f := range frames {
		sum += f.refRMS
	}
	return sum / float64(len(frames))
}
