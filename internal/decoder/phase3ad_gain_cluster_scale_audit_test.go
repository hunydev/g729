package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
)

// TestPhase3adGainClusterScaleAudit applies fixed-codebook contribution
// scaling only to gain-index clusters identified by the field/error audit.
// This checks whether the GA=6 low-envelope concentration can be corrected
// locally without the waveform damage caused by global scaling.
func TestPhase3adGainClusterScaleAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_CLUSTER_SCALE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_GAIN_CLUSTER_SCALE_AUDIT=1 to run gain-cluster scale audit")
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
	phase3adReportG192(t, "SPEECH.BIT", speechRef, bitData, speechFrames)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk gain-cluster scale audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3adReportRaw(t, "Asterisk", astRef, raw, astFrames)
}

type phase3adClusterScale struct {
	name string
	num  int
	den  int
	mask func(uint8) bool
}

func phase3adScales() []phase3adClusterScale {
	ga6 := func(ga uint8) bool { return ga == 6 }
	ga036 := func(ga uint8) bool { return ga == 0 || ga == 3 || ga == 6 }
	ga367 := func(ga uint8) bool { return ga == 3 || ga == 6 || ga == 7 }
	return []phase3adClusterScale{
		{name: "production", num: 1, den: 1, mask: func(uint8) bool { return false }},
		{name: "GA6_x5/4", num: 5, den: 4, mask: ga6},
		{name: "GA6_x3/2", num: 3, den: 2, mask: ga6},
		{name: "GA6_x7/4", num: 7, den: 4, mask: ga6},
		{name: "GA036_x5/4", num: 5, den: 4, mask: ga036},
		{name: "GA036_x3/2", num: 3, den: 2, mask: ga036},
		{name: "GA036_x7/4", num: 7, den: 4, mask: ga036},
		{name: "GA036_x2", num: 2, den: 1, mask: ga036},
		{name: "GA367_x5/4", num: 5, den: 4, mask: ga367},
		{name: "GA367_x3/2", num: 3, den: 2, mask: ga367},
	}
}

func phase3adReportG192(t *testing.T, label string, ref []int16, bitData []byte, frames int) {
	t.Helper()
	baseOut := decodeVariant(t, bitData, frames, nil, nil)
	phase3adReport(t, label, ref, baseOut, func(s phase3adClusterScale) []int16 {
		return phase3adDecodeG192Scale(t, bitData, frames, s)
	})
}

func phase3adReportRaw(t *testing.T, label string, ref []int16, raw []byte, frames int) {
	t.Helper()
	baseOut := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "production"})
	phase3adReport(t, label, ref, baseOut, func(s phase3adClusterScale) []int16 {
		return phase3adDecodeRawScale(t, raw, frames, s)
	})
}

func phase3adReport(t *testing.T, label string, ref, baseOut []int16, decode func(phase3adClusterScale) []int16) {
	t.Helper()
	base := blackboxMeasure(ref, baseOut, 40)
	baseEnv := phase3pEnvelopeCompare(ref, baseOut)
	rows := make([]phase3xRow, 0, len(phase3adScales()))

	t.Logf("Phase 3ad gain-cluster fixed contribution scale audit - %s", label)
	t.Logf("%-12s %9s %7s %10s %10s %8s %9s %9s %9s",
		"scale", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "clipped")
	t.Logf("%-12s %9s %7s %10s %10s %8s %9s %9s %9s",
		"-----", "---", "----", "------", "-----", "------", "--------", "-------", "-------")
	for _, s := range phase3adScales() {
		out := baseOut
		if s.name != "production" {
			out = decode(s)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, phase3xRow{name: s.name, m: m, env: env})
		t.Logf("%-12s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			s.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, phase3xCountClipped(out))
	}
	phase3adLogBest(t, phase3xRow{name: "production", m: base, env: baseEnv}, rows)
}

func phase3adLogBest(t *testing.T, base phase3xRow, rows []phase3xRow) {
	t.Helper()
	best := rows[0]
	bestSeg := rows[0]
	bestCorr := rows[0]
	bestEnv := rows[0]
	for _, r := range rows[1:] {
		if r.m.globalSNR > best.m.globalSNR {
			best = r
		}
		if r.m.segSNR > bestSeg.m.segSNR {
			bestSeg = r
		}
		if r.m.corr > bestCorr.m.corr {
			bestCorr = r
		}
		if r.env.lowRatioFrames < bestEnv.env.lowRatioFrames {
			bestEnv = r
		}
	}
	t.Logf("best global: %s %.2f (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-base.m.globalSNR)
	t.Logf("best seg:    %s %.2f (delta=%+.2f)", bestSeg.name, bestSeg.m.segSNR, bestSeg.m.segSNR-base.m.segSNR)
	t.Logf("best corr:   %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-base.m.corr)
	t.Logf("best env:    %s low<0.5=%d (delta=%+d)", bestEnv.name, bestEnv.env.lowRatioFrames, bestEnv.env.lowRatioFrames-base.env.lowRatioFrames)
	t.Logf("verdict: %s", phase3adVerdict(base, best, bestSeg, bestCorr, bestEnv))
}

func phase3adDecodeG192Scale(t *testing.T, bitData []byte, frames int, s phase3adClusterScale) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3aaGainDecoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", s.name, f, err)
		}
		if err := dec.decodeFramePhase3adScale(packed[:], out[f*frameSamples:(f+1)*frameSamples], &gd, s); err != nil {
			t.Fatalf("decodeFramePhase3adScale[%s] frame %d: %v", s.name, f, err)
		}
	}
	return out
}

func phase3adDecodeRawScale(t *testing.T, raw []byte, frames int, s phase3adClusterScale) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3aaGainDecoder
	for f := 0; f < frames; f++ {
		packed := raw[f*bitstream.FrameBytes : (f+1)*bitstream.FrameBytes]
		if err := dec.decodeFramePhase3adScale(packed, out[f*frameSamples:(f+1)*frameSamples], &gd, s); err != nil {
			t.Fatalf("decodeFramePhase3adScale[%s] frame %d: %v", s.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3adScale(packed []byte, out []int16, gd *phase3aaGainDecoder, s phase3adClusterScale) error {
	if len(packed) < bitstream.FrameBytes {
		return ErrShortInput
	}
	if len(out) < frameSamples {
		return ErrShortOutput
	}

	var fr bitstream.Frame
	if err := bitstream.Unpack(packed, &fr); err != nil {
		return err
	}

	sf1A, sf2A := d.lsp.Decode(lsp.Indices{
		L0: uint8(fr.L0),
		L1: uint8(fr.L1),
		L2: uint8(fr.L2),
		L3: uint8(fr.L3),
	})

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	_ = pitch.CheckParity(uint8(fr.P1), uint8(fr.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)

	d.decodeSubframePhase3adScale(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], gd, s)
	d.decodeSubframePhase3adScale(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], gd, s)
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3adScale(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16,
	S uint8,
	GA uint8,
	GB uint8,
	out []int16,
	gd *phase3aaGainDecoder,
	s phase3adClusterScale,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := gd.decode(gain.Indices{GA: GA, GB: GB}, &c, phase3aaGainMapProduction)

	num, den := 1, 1
	if s.mask(GA) {
		num, den = s.num, s.den
	}
	var u [subframeLen]int16
	phase3abBuildExcitationFixedScale(gpQ14, gcMant, gcExp, &v, &c, &u, num, den)

	var synthOut [subframeLen]int16
	d.syn.Filter(sfA, &u, &synthOut)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &synthOut, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.prevGpQ14 = gpQ14
}

func phase3adVerdict(base, best, bestSeg, bestCorr, bestEnv phase3xRow) string {
	if best.name != base.name && best.m.globalSNR-base.m.globalSNR > 0.5 && best.m.segSNR >= base.m.segSNR-0.1 && best.m.corr >= base.m.corr-0.01 {
		return "gain-cluster scale improves global SNR with bounded waveform damage; inspect " + best.name
	}
	if bestSeg.name != base.name && bestSeg.m.segSNR-base.m.segSNR > 0.5 && bestSeg.m.globalSNR >= base.m.globalSNR-0.1 && bestSeg.m.corr >= base.m.corr-0.01 {
		return "gain-cluster scale improves segmental SNR with bounded waveform damage; inspect " + bestSeg.name
	}
	if bestCorr.name != base.name && bestCorr.m.corr-base.m.corr > 0.03 && bestCorr.m.globalSNR >= base.m.globalSNR-0.1 {
		return "gain-cluster scale materially improves correlation; inspect " + bestCorr.name
	}
	if bestEnv.name != base.name && base.env.lowRatioFrames-bestEnv.env.lowRatioFrames > base.env.activeFrames/10 {
		return "gain-cluster scale reduces low-ratio frames but is not waveform-safe"
	}
	return "no gain-cluster scale variant is production-safe"
}
