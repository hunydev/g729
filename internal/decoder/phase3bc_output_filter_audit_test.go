package decoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3bcOutputFilterAudit probes whether the remaining local-vs-FFmpeg
// gap is a simple output-domain spectral/temporal tilt. FFmpeg is used only as
// an executable black-box decoder; no implementation source is inspected.
func TestPhase3bcOutputFilterAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_OUTPUT_FILTER_AUDIT") != "1" {
		t.Skip("set G729_DECODER_OUTPUT_FILTER_AUDIT=1 to run output-filter audit")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	datasets := []phase3bcFilterDataset{
		phase3bcLoadG192FilterDataset(t, "SPEECH.BIT"),
	}
	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	if _, err := os.Stat(rawPath); err == nil {
		datasets = append(datasets, phase3bcLoadRawFilterDataset(t, "Asterisk", rawPath))
	} else {
		t.Logf("Asterisk output-filter audit skipped: %v", err)
	}

	type variant struct {
		name string
		fn   func([]int16) []int16
	}
	variants := []variant{
		{name: "identity", fn: phase3bcClone},
		{name: "smooth_1_8_1", fn: func(in []int16) []int16 { return phase3rTemporalBlend(in, 1, 8, 1) }},
		{name: "smooth_1_6_1", fn: func(in []int16) []int16 { return phase3rTemporalBlend(in, 1, 6, 1) }},
		{name: "smooth_1_4_1", fn: func(in []int16) []int16 { return phase3rTemporalBlend(in, 1, 4, 1) }},
		{name: "smooth_1_2_1", fn: func(in []int16) []int16 { return phase3rTemporalBlend(in, 1, 2, 1) }},
		{name: "preemph_1_8", fn: func(in []int16) []int16 { return phase3bcOneTap(in, -1, 8) }},
		{name: "preemph_1_4", fn: func(in []int16) []int16 { return phase3bcOneTap(in, -1, 4) }},
		{name: "deemph_1_8", fn: func(in []int16) []int16 { return phase3bcOneTap(in, +1, 8) }},
		{name: "deemph_1_4", fn: func(in []int16) []int16 { return phase3bcOneTap(in, +1, 4) }},
		{name: "hf_boost_1_8", fn: func(in []int16) []int16 { return phase3bcHFMix(in, 1, 8) }},
		{name: "hf_boost_1_4", fn: func(in []int16) []int16 { return phase3bcHFMix(in, 1, 4) }},
		{name: "hf_cut_1_8", fn: func(in []int16) []int16 { return phase3bcHFMix(in, -1, 8) }},
		{name: "hf_cut_1_4", fn: func(in []int16) []int16 { return phase3bcHFMix(in, -1, 4) }},
	}

	for _, ds := range datasets {
		base := blackboxMeasure(ds.ref, ds.enhanced, 40)
		t.Logf("Phase 3bc output-filter audit - %s (%d frames)", ds.label, len(ds.ref)/frameSamples)
		t.Logf("baseline enhanced: gSNR=%.2f seg=%.2f corr=%.3f rms=%.1f",
			base.globalSNR, base.segSNR, base.corr, base.rms)
		t.Logf("%-18s %9s %9s %8s %9s %9s",
			"variant", "gSNR", "segSNR", "corr", "rms", "deltaG")
		best := phase3bcFilterRow{name: "identity", m: base}
		for _, v := range variants {
			out := v.fn(ds.enhanced)
			m := blackboxMeasure(ds.ref, out, 40)
			if m.globalSNR > best.m.globalSNR {
				best = phase3bcFilterRow{name: v.name, m: m}
			}
			t.Logf("%-18s %9.2f %9.2f %8.3f %9.1f %+9.2f",
				v.name, m.globalSNR, m.segSNR, m.corr, m.rms, m.globalSNR-base.globalSNR)
		}
		t.Logf("best by global SNR: %s %.2f dB (delta=%+.2f)",
			best.name, best.m.globalSNR, best.m.globalSNR-base.globalSNR)
	}
}

type phase3bcFilterDataset struct {
	label    string
	ref      []int16
	enhanced []int16
}

type phase3bcFilterRow struct {
	name string
	m    blackboxMetrics
}

func phase3bcLoadG192FilterDataset(t *testing.T, name string) phase3bcFilterDataset {
	t.Helper()
	path := vectorPath(name)
	ensureTestdataPresent(t, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	frames := len(data) / bitstream.G192FrameBytes
	ref := phase3uFFmpegDecodeG192(t, data, frames, name)
	tmp := t.TempDir()
	rawPath := filepath.Join(tmp, name+".g729")
	writeG192RawForEnvelopeAudit(t, data, frames, rawPath)
	return phase3bcFilterDataset{
		label:    name,
		ref:      ref,
		enhanced: phase3rDecodeRawEnhanced(t, rawPath, frames),
	}
}

func phase3bcLoadRawFilterDataset(t *testing.T, label, rawPath string) phase3bcFilterDataset {
	t.Helper()
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read %s: %v", rawPath, err)
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	frames := len(raw) / bitstream.FrameBytes
	return phase3bcFilterDataset{
		label:    label,
		ref:      phase3uFFmpegDecodeRaw(t, rawPath, frames, label),
		enhanced: phase3rDecodeRawEnhanced(t, rawPath, frames),
	}
}

func phase3bcClone(in []int16) []int16 {
	out := make([]int16, len(in))
	copy(out, in)
	return out
}

func phase3bcOneTap(in []int16, prevNum, den int) []int16 {
	out := make([]int16, len(in))
	if den <= 0 {
		copy(out, in)
		return out
	}
	var prev int
	for i, s := range in {
		acc := int(s)*den + prevNum*prev
		out[i] = phase3bcRoundSat(acc, den)
		prev = int(s)
	}
	return out
}

func phase3bcHFMix(in []int16, num, den int) []int16 {
	out := make([]int16, len(in))
	if den <= 0 {
		copy(out, in)
		return out
	}
	for i, s := range in {
		prev := int(s)
		if i > 0 {
			prev = int(in[i-1])
		}
		next := int(s)
		if i+1 < len(in) {
			next = int(in[i+1])
		}
		low := (prev + 2*int(s) + next) / 4
		high := int(s) - low
		acc := int(s)*den + num*high
		out[i] = phase3bcRoundSat(acc, den)
	}
	return out
}

func phase3bcRoundSat(num, den int) int16 {
	if den <= 0 {
		den = 1
	}
	if num >= 0 {
		num += den / 2
	} else {
		num -= den / 2
	}
	v := num / den
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}
