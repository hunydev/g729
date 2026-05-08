package decoder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
	"github.com/hunydev/g729/internal/tables"
)

// TestPhase3aaGainMapFFmpegAudit checks whether the decoder's gain VQ
// transmitted-index mapping explains the local-vs-FFmpeg quality gap. FFmpeg
// remains an executable black-box decoder.
func TestPhase3aaGainMapFFmpegAudit(t *testing.T) {
	if os.Getenv("G729_DECODER_GAIN_MAP_FFMPEG_AUDIT") != "1" {
		t.Skip("set G729_DECODER_GAIN_MAP_FFMPEG_AUDIT=1 to run gain-map audit")
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
	phase3aaReportG192(t, "SPEECH.BIT", speechRef, bitData, speechFrames)

	rawPath := filepath.Join("..", "..", "testdata", "external", "asterisk_payload.g729")
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Logf("Asterisk gain-map audit skipped: %v", err)
		return
	}
	if len(raw)%bitstream.FrameBytes != 0 {
		t.Fatalf("raw g729 payload length %d not divisible by %d", len(raw), bitstream.FrameBytes)
	}
	astFrames := len(raw) / bitstream.FrameBytes
	astRef := phase3uFFmpegDecodeRaw(t, rawPath, astFrames, "asterisk")
	phase3aaReportRaw(t, "Asterisk", astRef, raw, astFrames)
}

type phase3aaGainMapMode int

const (
	phase3aaGainMapProduction phase3aaGainMapMode = iota
	phase3aaGainMapIdentityBoth
	phase3aaGainMapForwardBoth
	phase3aaGainMapIdentityGA
	phase3aaGainMapIdentityGB
	phase3aaGainMapForwardGA
	phase3aaGainMapForwardGB
)

type phase3aaGainMapVariant struct {
	name string
	mode phase3aaGainMapMode
}

type phase3aaGainDecoder struct {
	initialized bool
	pastErrors  [4]int16
}

func phase3aaVariants() []phase3aaGainMapVariant {
	return []phase3aaGainMapVariant{
		{name: "production_imap_both", mode: phase3aaGainMapProduction},
		{name: "identity_both", mode: phase3aaGainMapIdentityBoth},
		{name: "forward_map_both", mode: phase3aaGainMapForwardBoth},
		{name: "identity_GA_only", mode: phase3aaGainMapIdentityGA},
		{name: "identity_GB_only", mode: phase3aaGainMapIdentityGB},
		{name: "forward_GA_only", mode: phase3aaGainMapForwardGA},
		{name: "forward_GB_only", mode: phase3aaGainMapForwardGB},
	}
}

func phase3aaReportG192(t *testing.T, label string, ref []int16, bitData []byte, frames int) {
	t.Helper()
	baseOut := decodeVariant(t, bitData, frames, nil, nil)
	phase3aaReport(t, label, ref, baseOut, func(v phase3aaGainMapVariant) []int16 {
		return phase3aaDecodeG192Variant(t, bitData, frames, v)
	})
}

func phase3aaReportRaw(t *testing.T, label string, ref []int16, raw []byte, frames int) {
	t.Helper()
	baseOut := phase3tDecodeRawVariant(t, raw, frames, phase3eVariant{name: "production"})
	phase3aaReport(t, label, ref, baseOut, func(v phase3aaGainMapVariant) []int16 {
		return phase3aaDecodeRawVariant(t, raw, frames, v)
	})
}

func phase3aaReport(t *testing.T, label string, ref, baseOut []int16, decode func(phase3aaGainMapVariant) []int16) {
	t.Helper()
	rows := make([]phase3xRow, 0, len(phase3aaVariants()))
	base := blackboxMeasure(ref, baseOut, 40)
	baseEnv := phase3pEnvelopeCompare(ref, baseOut)

	t.Logf("Phase 3aa gain-map FFmpeg audit - %s", label)
	t.Logf("%-22s %9s %7s %10s %10s %8s %9s %9s %9s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "ratioMed", "low<0.5", "corr<0.3")
	t.Logf("%-22s %9s %7s %10s %10s %8s %9s %9s %9s",
		"-------", "---", "----", "------", "-----", "------", "--------", "-------", "--------")
	for _, v := range phase3aaVariants() {
		out := baseOut
		if v.mode != phase3aaGainMapProduction {
			out = decode(v)
		}
		m := blackboxMeasure(ref, out, 40)
		env := phase3pEnvelopeCompare(ref, out)
		rows = append(rows, phase3xRow{name: v.name, m: m, env: env})
		t.Logf("%-22s %9.2f %7d %10.2f %10.2f %8.3f %9.3f %9d %9d",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr, env.ratioMedian, env.lowRatioFrames, env.lowCorrFrames)
	}
	phase3aaLogBest(t, phase3xRow{name: "production_imap_both", m: base, env: baseEnv}, rows)
}

func phase3aaLogBest(t *testing.T, base phase3xRow, rows []phase3xRow) {
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
	t.Logf("verdict: %s", phase3aaVerdict(base, best, bestSeg, bestCorr, bestEnv))
}

func phase3aaDecodeG192Variant(t *testing.T, bitData []byte, frames int, variant phase3aaGainMapVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3aaGainDecoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, f, err)
		}
		if err := dec.decodeFramePhase3aaVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], &gd, variant); err != nil {
			t.Fatalf("decodeFramePhase3aaVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func phase3aaDecodeRawVariant(t *testing.T, raw []byte, frames int, variant phase3aaGainMapVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var gd phase3aaGainDecoder
	for f := 0; f < frames; f++ {
		packed := raw[f*bitstream.FrameBytes : (f+1)*bitstream.FrameBytes]
		if err := dec.decodeFramePhase3aaVariant(packed, out[f*frameSamples:(f+1)*frameSamples], &gd, variant); err != nil {
			t.Fatalf("decodeFramePhase3aaVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3aaVariant(packed []byte, out []int16, gd *phase3aaGainDecoder, variant phase3aaGainMapVariant) error {
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

	d.decodeSubframePhase3aaVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen], gd, variant)
	d.decodeSubframePhase3aaVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples], gd, variant)
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3aaVariant(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16,
	S uint8,
	GA uint8,
	GB uint8,
	out []int16,
	gd *phase3aaGainDecoder,
	variant phase3aaGainMapVariant,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMant, gcExp := gd.decode(gain.Indices{GA: GA, GB: GB}, &c, variant.mode)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMant, gcExp, &v, &c, &u)

	var s [subframeLen]int16
	d.syn.Filter(sfA, &u, &s)

	var sPf [subframeLen]int16
	d.pst.Filter(sfA, tInt, &s, &sPf)

	var hpOut [subframeLen]int16
	d.hpFilter(&sPf, hpOut[:])
	copy(out[:subframeLen], hpOut[:])

	copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])
	copy(d.pastExc[pastExcLen-subframeLen:], u[:])
	d.prevGpQ14 = gpQ14
}

func (d *phase3aaGainDecoder) decode(idx gain.Indices, c *[subframeLen]int16, mode phase3aaGainMapMode) (gpQ14, gcMantQ14 int16, gcExp int8) {
	if !d.initialized {
		for i := range d.pastErrors {
			d.pastErrors[i] = gain.PastErrorsDefault
		}
		d.initialized = true
	}

	ecEnergy := gain.FixedCodebookEnergy(c)
	if ecEnergy <= 0 {
		gp, _ := phase3aaDecodeVQ(idx, mode)
		d.shift(gain.PastErrorsDefault)
		return gp, 0, 0
	}

	predicted := gain.PredictedLogGain(&d.pastErrors)
	ecLog2Q10 := int32(gain.Log2Fixed(ecEnergy)) - 26*1024
	ecDBQ10 := (ecLog2Q10*phase3jDBPerLog2Q13 + (1 << 12)) >> 13
	ecBarDBQ10 := fixed.Saturate(fixed.Word32(ecDBQ10 - phase3jTenLog10_40Q10))
	logGainDBQ10 := predicted - int32(ecBarDBQ10)
	log2GcQ10 := (logGainDBQ10*phase3jInvDBScaleQ15 + (1 << 14)) >> 15

	gp, gammaC := phase3aaDecodeVQ(idx, mode)
	gpQ14 = gp
	if gammaC > 0 {
		gammaLog2Q10 := int32(gain.Log2Fixed(fixed.Word32(gammaC))) - 13*1024
		log2GcWithGammaQ10 := log2GcQ10 + gammaLog2Q10
		intPart := log2GcWithGammaQ10 >> 10
		frac := log2GcWithGammaQ10 - (intPart << 10)
		gcMantQ14 = gain.Pow2FracQ14(frac)
		switch {
		case intPart > 127:
			gcExp = 127
		case intPart < -128:
			gcExp = -128
		default:
			gcExp = int8(intPart)
		}
	}

	uCurrent := int16(gain.PastErrorsDefault)
	if gammaC > 0 {
		gammaLog2Q10 := int32(gain.Log2Fixed(fixed.Word32(gammaC))) - 13*1024
		val := (gammaLog2Q10*phase3jDBPerLog2Q10 + (1 << 9)) >> 10
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		uCurrent = int16(val)
	}
	d.shift(uCurrent)
	return
}

func (d *phase3aaGainDecoder) shift(v int16) {
	d.pastErrors[3] = d.pastErrors[2]
	d.pastErrors[2] = d.pastErrors[1]
	d.pastErrors[1] = d.pastErrors[0]
	d.pastErrors[0] = v
}

func phase3aaDecodeVQ(idx gain.Indices, mode phase3aaGainMapMode) (gpQ14, gammaCQ13 int16) {
	gaEntry := phase3aaMapGA(idx.GA, mode)
	gbEntry := phase3aaMapGB(idx.GB, mode)
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][0]), fixed.Word16(tables.GainGBK2[gbEntry][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][1]), fixed.Word16(tables.GainGBK2[gbEntry][1])))
	return
}

func phase3aaMapGA(ga uint8, mode phase3aaGainMapMode) uint8 {
	switch mode {
	case phase3aaGainMapIdentityBoth, phase3aaGainMapIdentityGA:
		return ga
	case phase3aaGainMapForwardBoth, phase3aaGainMapForwardGA:
		return tables.GainMap1[ga]
	default:
		return tables.GainImap1[ga]
	}
}

func phase3aaMapGB(gb uint8, mode phase3aaGainMapMode) uint8 {
	switch mode {
	case phase3aaGainMapIdentityBoth, phase3aaGainMapIdentityGB:
		return gb
	case phase3aaGainMapForwardBoth, phase3aaGainMapForwardGB:
		return tables.GainMap2[gb]
	default:
		return tables.GainImap2[gb]
	}
}

func phase3aaVerdict(base, best, bestSeg, bestCorr, bestEnv phase3xRow) string {
	if best.name != base.name && best.m.globalSNR-base.m.globalSNR > 1.0 && best.m.segSNR >= base.m.segSNR-0.25 && best.m.corr >= base.m.corr-0.02 {
		return "gain-map variant materially improves global SNR without damaging segmental/correlation; inspect " + best.name
	}
	if bestSeg.name != base.name && bestSeg.m.segSNR-base.m.segSNR > 1.0 && bestSeg.m.globalSNR >= base.m.globalSNR-0.25 && bestSeg.m.corr >= base.m.corr-0.02 {
		return "gain-map variant materially improves segmental SNR without damaging global/correlation; inspect " + bestSeg.name
	}
	if bestCorr.name != base.name && bestCorr.m.corr-base.m.corr > 0.05 && bestCorr.m.globalSNR >= base.m.globalSNR-0.25 {
		return "gain-map variant materially improves correlation; inspect " + bestCorr.name
	}
	if bestEnv.name != base.name && base.env.lowRatioFrames-bestEnv.env.lowRatioFrames > base.env.activeFrames/10 {
		return "gain-map variant reduces low-ratio frames but is not waveform-safe"
	}
	return "no gain-map variant is production-safe"
}
