package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3ajStageProxyContextAudit groups local decoder stage metrics by
// FFmpeg-relative envelope ratio. It looks for runtime-available selector
// proxies after the oracle gate showed that a low-envelope selector would help.
func TestPhase3ajStageProxyContextAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_STAGE_PROXY_CONTEXT_AUDIT") != "1" {
		t.Skip("set G729_DECODER_STAGE_PROXY_CONTEXT_AUDIT=1 to run stage proxy context audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	t.Logf("Phase 3aj stage proxy context audit")
	for _, vector := range []string{"SPEECH.BIT", "LSP.BIT", "PITCH.BIT", "TEST.BIT", "ALGTHM.BIT"} {
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
		local, taps := decodeG192WithTapsForEnvelopeAudit(t, bitData, frames)
		phase3ajReport(t, vector, ref, local, taps)
	}

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk stage proxy context audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	ref := phase3uFFmpegDecodeRaw(t, rawPath, frames, "asterisk")
	local, taps := phase3ajDecodeRawWithTaps(t, raw, frames)
	phase3ajReport(t, "Asterisk", ref, local, taps)
}

type phase3ajBin struct {
	name   string
	lo     float64
	hi     float64
	count  int
	ratio  float64
	outRMS float64
	uRMS   float64
	sRMS   float64
	spfRMS float64
	fixU   float64
	pitU   float64
	sU     float64
	gp     float64
	gc     float64
	pred   float64
	logG   float64
}

func phase3ajReport(t *testing.T, label string, ref, local []int16, taps []Phase3DiagFrameTaps) {
	t.Helper()
	bins := []phase3ajBin{
		{name: "ratio<0.50", lo: -1, hi: 0.50},
		{name: "0.50..0.75", lo: 0.50, hi: 0.75},
		{name: "0.75..1.25", lo: 0.75, hi: 1.25},
		{name: "1.25..1.75", lo: 1.25, hi: 1.75},
		{name: "ratio>=1.75", lo: 1.75, hi: 1e9},
	}
	frames := len(ref) / frameSamples
	if lf := len(local) / frameSamples; lf < frames {
		frames = lf
	}
	if tf := len(taps); tf < frames {
		frames = tf
	}
	for frame := 0; frame < frames; frame++ {
		off := frame * frameSamples
		refFrame := ref[off : off+frameSamples]
		localFrame := local[off : off+frameSamples]
		refRMS := envelopeRMS(refFrame)
		if refRMS < 500 {
			continue
		}
		localRMS := envelopeRMS(localFrame)
		ratio := localRMS / refRMS
		stages := envelopeStageSummary(taps[frame])
		for i := range bins {
			if ratio >= bins[i].lo && ratio < bins[i].hi {
				bins[i].add(ratio, stages)
				break
			}
		}
	}
	t.Logf("%s stage proxy bins:", label)
	t.Logf("  %-12s %7s %8s %8s %8s %8s %8s %8s %8s %7s %7s %8s %8s",
		"prodRatio", "frames", "ratio", "outRMS", "uRMS", "sRMS", "fix/u", "pit/u", "s/u", "gpMax", "gcMax", "pred", "logGain")
	for _, b := range bins {
		t.Logf("  %-12s %7d %8.3f %8.1f %8.1f %8.1f %8.3f %8.3f %8.3f %7.3f %7.3f %8.2f %8.2f",
			b.name, b.count, b.mean(b.ratio), b.mean(b.outRMS), b.mean(b.uRMS), b.mean(b.sRMS),
			b.mean(b.fixU), b.mean(b.pitU), b.mean(b.sU), b.mean(b.gp), b.mean(b.gc),
			b.mean(b.pred)/1024.0, b.mean(b.logG)/1024.0)
	}
}

func (b *phase3ajBin) add(ratio float64, stages envelopeStageMetrics) {
	b.count++
	b.ratio += ratio
	b.outRMS += stages.outRMS
	b.uRMS += stages.uRMS
	b.sRMS += stages.sRMS
	b.spfRMS += stages.spfRMS
	b.fixU += safeRatioFloat64(stages.fixedRMS, stages.uRMS)
	b.pitU += safeRatioFloat64(stages.pitchRMS, stages.uRMS)
	b.sU += safeRatioFloat64(stages.sRMS, stages.uRMS)
	b.gp += stages.gpMax
	b.gc += stages.gcMax
	b.pred += stages.predictedAvgQ10
	b.logG += stages.logGainAvgQ10
}

func (b *phase3ajBin) mean(sum float64) float64 {
	if b.count == 0 {
		return 0
	}
	return sum / float64(b.count)
}

func phase3ajDecodeRawWithTaps(t *testing.T, raw []byte, frames int) ([]int16, []Phase3DiagFrameTaps) {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	taps := make([]Phase3DiagFrameTaps, frames)
	var dec Decoder
	for frame := 0; frame < frames; frame++ {
		packed := raw[frame*bitstream.FrameBytes : (frame+1)*bitstream.FrameBytes]
		frameTaps, err := dec.DecodeWithTaps(packed)
		if err != nil {
			t.Fatalf("DecodeWithTaps raw frame %d: %v", frame, err)
		}
		taps[frame] = frameTaps
		copy(out[frame*frameSamples:(frame+1)*frameSamples], frameTaps.Output[:])
	}
	return out, taps
}
