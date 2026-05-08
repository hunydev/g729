package decoder

import (
	"os"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3lBitstreamLayoutVariantAudit_SPEECH probes whole-frame bit layout
// and subframe field cross-wire variants with final-output black-box metrics.
func TestPhase3lBitstreamLayoutVariantAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_BITSTREAM_LAYOUT_AUDIT") != "1" {
		t.Skip("set G729_DECODER_BITSTREAM_LAYOUT_AUDIT=1 to audit bitstream layout variants")
	}

	bitPath := vectorPath("SPEECH.BIT")
	pstPath := vectorPath("SPEECH.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	frames := len(pstData) / (2 * frameSamples)
	if bf := len(bitData) / bitstream.G192FrameBytes; bf < frames {
		frames = bf
	}
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	ref := blackboxReadPCM16LE(t, pstData, frames*frameSamples)
	production := decodeVariant(t, bitData, frames, nil, nil)
	prodMetrics := blackboxMeasure(ref, production, 40)

	type variant struct {
		name            string
		transform       func(*bitstream.Frame)
		packedTransform func(*[bitstream.FrameBytes]byte)
	}
	variants := []variant{
		{name: "production"},
		{name: "payload_shift_left_1", packedTransform: func(p *[bitstream.FrameBytes]byte) { phase3lShiftPayload(p, -1) }},
		{name: "payload_shift_right_1", packedTransform: func(p *[bitstream.FrameBytes]byte) { phase3lShiftPayload(p, +1) }},
		{name: "payload_shift_left_2", packedTransform: func(p *[bitstream.FrameBytes]byte) { phase3lShiftPayload(p, -2) }},
		{name: "payload_shift_right_2", packedTransform: func(p *[bitstream.FrameBytes]byte) { phase3lShiftPayload(p, +2) }},
		{name: "subframe_fcb_swap", transform: func(f *bitstream.Frame) {
			f.C1, f.C2 = f.C2, f.C1
			f.S1, f.S2 = f.S2, f.S1
		}},
		{name: "subframe_gain_swap", transform: func(f *bitstream.Frame) {
			f.GA1, f.GA2 = f.GA2, f.GA1
			f.GB1, f.GB2 = f.GB2, f.GB1
		}},
		{name: "subframe_fcb_gain_swap", transform: func(f *bitstream.Frame) {
			f.C1, f.C2 = f.C2, f.C1
			f.S1, f.S2 = f.S2, f.S1
			f.GA1, f.GA2 = f.GA2, f.GA1
			f.GB1, f.GB2 = f.GB2, f.GB1
		}},
		{name: "pitch_lowbits_swap", transform: func(f *bitstream.Frame) {
			p1Low := f.P1 & 0x1F
			f.P1 = (f.P1 & 0xE0) | f.P2
			f.P2 = p1Low
			f.P0 = bitstream.Parity(f.P1)
		}},
		{name: "sf2_uses_sf1_gain", transform: func(f *bitstream.Frame) {
			f.GA2, f.GB2 = f.GA1, f.GB1
		}},
		{name: "sf1_uses_sf2_gain", transform: func(f *bitstream.Frame) {
			f.GA1, f.GB1 = f.GA2, f.GB2
		}},
		{name: "sf2_uses_sf1_fcb", transform: func(f *bitstream.Frame) {
			f.C2, f.S2 = f.C1, f.S1
		}},
		{name: "sf1_uses_sf2_fcb", transform: func(f *bitstream.Frame) {
			f.C1, f.S1 = f.C2, f.S2
		}},
	}

	type row struct {
		name string
		m    blackboxMetrics
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3l bitstream layout variant audit - SPEECH.BIT/SPEECH.PST (%d frames)", frames)
	t.Logf("baseline production: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR)
	t.Logf("")
	t.Logf("%-26s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "lagSNR", "bestSNR")
	t.Logf("%-26s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"-------", "---", "----", "------", "-----", "------", "-----", "-----", "------", "-------")
	for _, v := range variants {
		out := production
		if v.transform != nil || v.packedTransform != nil {
			out = decodeVariant(t, bitData, frames, v.transform, v.packedTransform)
		}
		m := blackboxMeasure(ref, out, 40)
		rows = append(rows, row{name: v.name, m: m})
		t.Logf("%-26s %9.2f %7d %10.2f %10.2f %8.3f %10.2f %9.3f %10d %10.2f",
			v.name, m.rms, m.peak, m.globalSNR, m.segSNR, m.corr,
			m.globalSNR-prodMetrics.globalSNR, m.corr-prodMetrics.corr,
			m.bestSNRLag, m.bestSNR)
	}

	best := rows[0]
	bestCorr := rows[0]
	for _, r := range rows[1:] {
		if r.m.globalSNR > best.m.globalSNR {
			best = r
		}
		if r.m.corr > bestCorr.m.corr {
			bestCorr = r
		}
	}
	t.Logf("")
	t.Logf("best by gSNR@0: %s %.2f dB (delta=%+.2f)", best.name, best.m.globalSNR, best.m.globalSNR-prodMetrics.globalSNR)
	t.Logf("best by corr@0: %s %.3f (delta=%+.3f)", bestCorr.name, bestCorr.m.corr, bestCorr.m.corr-prodMetrics.corr)
	t.Logf("verdict: %s", phase3lVerdict(prodMetrics, best, bestCorr))
}

func phase3lShiftPayload(p *[bitstream.FrameBytes]byte, shift int) {
	var old [bitstream.FrameBits]uint8
	for i := 0; i < bitstream.FrameBits; i++ {
		old[i] = phase3lGetBit(p, i)
	}
	for i := range p {
		p[i] = 0
	}
	for i := 0; i < bitstream.FrameBits; i++ {
		src := i + shift
		if src < 0 || src >= bitstream.FrameBits {
			continue
		}
		if old[src] == 1 {
			phase3lSetBit(p, i)
		}
	}
}

func phase3lGetBit(p *[bitstream.FrameBytes]byte, idx int) uint8 {
	byteIdx := idx >> 3
	bitIdx := 7 - (idx & 7)
	return (p[byteIdx] >> uint(bitIdx)) & 1
}

func phase3lSetBit(p *[bitstream.FrameBytes]byte, idx int) {
	byteIdx := idx >> 3
	bitIdx := 7 - (idx & 7)
	p[byteIdx] |= 1 << uint(bitIdx)
}

func phase3lVerdict(prod blackboxMetrics, best, bestCorr struct {
	name string
	m    blackboxMetrics
}) string {
	if best.name != "production" && best.m.globalSNR-prod.globalSNR > 1.0 {
		return "bitstream layout variant materially improves SNR; inspect " + best.name
	}
	if bestCorr.name != "production" && bestCorr.m.corr-prod.corr > 0.05 {
		return "bitstream layout variant materially improves correlation; inspect " + bestCorr.name
	}
	return "no bit-offset or subframe field cross-wire variant improves output; simple bitstream layout error is unlikely"
}
