package decoder

import (
	"bytes"
	"os"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/fcb"
	"github.com/hunydev/g729/internal/gain"
	"github.com/hunydev/g729/internal/lsp"
	"github.com/hunydev/g729/internal/pitch"
	"github.com/hunydev/g729/internal/synth"
)

// TestPhase3kLSPRouteVariantAudit_SPEECH probes LSP bit-field
// interpretation and per-subframe LP routing variants with final-output
// black-box metrics. By default it runs SPEECH.BIT; set
// G729_DECODER_FFMPEG_ENVELOPE_VECTOR to focus another vector such as TAME.
func TestPhase3kLSPRouteVariantAudit_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_LSP_ROUTE_AUDIT") != "1" {
		t.Skip("set G729_DECODER_LSP_ROUTE_AUDIT=1 to audit LSP route variants")
	}

	vector := phase3oSelectedVector()
	bitPath := vectorPath(vector)
	ensureTestdataPresent(t, bitPath)

	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read %s: %v", vector, err)
	}

	frames := len(bitData) / bitstream.G192FrameBytes
	if frames <= 0 {
		t.Fatalf("frames reconciled to %d", frames)
	}
	ref, ok := phase3oReadVectorPST(t, vector, frames)
	if !ok {
		t.Fatalf("missing companion PST for %s", vector)
	}
	production := decodeVariant(t, bitData, frames, nil, nil)
	prodMetrics := blackboxMeasure(ref, production, 40)

	variants := []phase3kVariant{
		{name: "production"},
		{name: "lsp_l0_flip", transform: func(f *bitstream.Frame) { f.L0 ^= 1 }},
		{name: "lsp_force_l0_0", transform: func(f *bitstream.Frame) { f.L0 = 0 }},
		{name: "lsp_force_l0_1", transform: func(f *bitstream.Frame) { f.L0 = 1 }},
		{name: "lsp_l1_bitrev7", transform: func(f *bitstream.Frame) { f.L1 = uint16(phase3kReverseN(uint8(f.L1), 7)) }},
		{name: "lsp_l2_bitrev5", transform: func(f *bitstream.Frame) { f.L2 = uint16(phase3kReverseN(uint8(f.L2), 5)) }},
		{name: "lsp_l3_bitrev5", transform: func(f *bitstream.Frame) { f.L3 = uint16(phase3kReverseN(uint8(f.L3), 5)) }},
		{name: "lsp_l2_l3_swap", transform: func(f *bitstream.Frame) { f.L2, f.L3 = f.L3, f.L2 }},
		{name: "lp_sf1_current", route: phase3kRouteSF1Current},
		{name: "lp_sf1_prev_current", route: phase3kRouteSF1PrevCurrent},
		{name: "lp_swap_sf1_sf2", route: phase3kRouteSwapSubframes},
		{name: "lp_both_sf1_interp", route: phase3kRouteBothSF1},
		{name: "lp_both_sf2_current", route: phase3kRouteBothSF2},
		{name: "lsp_reset_each_frame", route: phase3kRouteResetEachFrame},
	}

	type row struct {
		name string
		m    blackboxMetrics
	}
	rows := make([]row, 0, len(variants))

	t.Logf("Phase 3k LSP route variant audit - %s (%d frames)", vector, frames)
	t.Logf("baseline production: rms=%.2f peak=%d gSNR=%.2f seg=%.2f corr=%.3f bestLag=%+d bestSNR=%.2f",
		prodMetrics.rms, prodMetrics.peak, prodMetrics.globalSNR, prodMetrics.segSNR,
		prodMetrics.corr, prodMetrics.bestSNRLag, prodMetrics.bestSNR)
	t.Logf("")
	t.Logf("%-24s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"variant", "rms", "peak", "gSNR@0", "seg@0", "corr@0", "dGsnr", "dCorr", "lagSNR", "bestSNR")
	t.Logf("%-24s %9s %7s %10s %10s %8s %10s %9s %10s %10s",
		"-------", "---", "----", "------", "-----", "------", "-----", "-----", "------", "-------")
	for _, v := range variants {
		out := production
		switch {
		case v.transform != nil:
			out = decodeVariant(t, bitData, frames, v.transform, nil)
		case v.route != phase3kRouteProduction:
			out = phase3kDecodeRouteVariant(t, bitData, frames, v)
		}
		m := blackboxMeasure(ref, out, 40)
		rows = append(rows, row{name: v.name, m: m})
		t.Logf("%-24s %9.2f %7d %10.2f %10.2f %8.3f %10.2f %9.3f %10d %10.2f",
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
	t.Logf("verdict: %s", phase3kVerdict(prodMetrics, best, bestCorr))
}

type phase3kRouteMode int

const (
	phase3kRouteProduction phase3kRouteMode = iota
	phase3kRouteSF1Current
	phase3kRouteSF1PrevCurrent
	phase3kRouteSwapSubframes
	phase3kRouteBothSF1
	phase3kRouteBothSF2
	phase3kRouteResetEachFrame
)

type phase3kVariant struct {
	name      string
	transform func(*bitstream.Frame)
	route     phase3kRouteMode
}

func phase3kReverseN(v uint8, n uint) uint8 {
	var out uint8
	for i := uint(0); i < n; i++ {
		out = (out << 1) | ((v >> i) & 1)
	}
	return out
}

func phase3kDecodeRouteVariant(t *testing.T, bitData []byte, frames int, variant phase3kVariant) []int16 {
	t.Helper()
	out := make([]int16, frames*frameSamples)
	var dec Decoder
	var packed [bitstream.FrameBytes]byte
	r := bytes.NewReader(bitData)
	var prevCurrentA [lpcOrder + 1]int16
	var hasPrevCurrent bool
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, packed[:]); err != nil {
			t.Fatalf("ReadG192Frame[%s] frame %d: %v", variant.name, f, err)
		}
		if variant.route == phase3kRouteResetEachFrame {
			dec.lsp.Reset()
		}
		if err := dec.decodeFramePhase3kRouteVariant(packed[:], out[f*frameSamples:(f+1)*frameSamples], variant, &prevCurrentA, &hasPrevCurrent); err != nil {
			t.Fatalf("decodeFramePhase3kRouteVariant[%s] frame %d: %v", variant.name, f, err)
		}
	}
	return out
}

func (d *Decoder) decodeFramePhase3kRouteVariant(
	packed []byte,
	out []int16,
	variant phase3kVariant,
	prevCurrentA *[lpcOrder + 1]int16,
	hasPrevCurrent *bool,
) error {
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
	switch variant.route {
	case phase3kRouteSF1Current:
		sf1A = sf2A
	case phase3kRouteSF1PrevCurrent:
		if *hasPrevCurrent {
			sf1A = *prevCurrentA
		}
	case phase3kRouteSwapSubframes:
		sf1A, sf2A = sf2A, sf1A
	case phase3kRouteBothSF1:
		sf2A = sf1A
	case phase3kRouteBothSF2:
		sf1A = sf2A
	}
	*prevCurrentA = sf2A
	*hasPrevCurrent = true

	tInt1, tFrac1 := pitch.DecodeDelaySubframe1(uint8(fr.P1))
	_ = pitch.CheckParity(uint8(fr.P1), uint8(fr.P0))
	tInt2, tFrac2 := pitch.DecodeDelaySubframe2(uint8(fr.P2), tInt1)

	d.decodeSubframePhase3kRouteVariant(&sf1A, tInt1, tFrac1, fr.C1, uint8(fr.S1), uint8(fr.GA1), uint8(fr.GB1), out[:subframeLen])
	d.decodeSubframePhase3kRouteVariant(&sf2A, tInt2, tFrac2, fr.C2, uint8(fr.S2), uint8(fr.GA2), uint8(fr.GB2), out[subframeLen:frameSamples])
	scaleDecoderOutput(out[:frameSamples])
	return nil
}

func (d *Decoder) decodeSubframePhase3kRouteVariant(
	sfA *[lpcOrder + 1]int16,
	tInt, tFrac int,
	C uint16, S uint8,
	GA, GB uint8,
	out []int16,
) {
	betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)

	var v [subframeLen]int16
	decodeAdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)

	var c [subframeLen]int16
	fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)

	gpQ14, gcMantQ14, gcExp := d.gn.Decode(gain.Indices{GA: GA, GB: GB}, &c)

	var u [subframeLen]int16
	synth.BuildExcitation(gpQ14, gcMantQ14, gcExp, &v, &c, &u)

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

func phase3kVerdict(prod blackboxMetrics, best, bestCorr struct {
	name string
	m    blackboxMetrics
}) string {
	if best.name != "production" && best.m.globalSNR-prod.globalSNR > 1.0 {
		return "LSP route variant materially improves SNR; inspect " + best.name
	}
	if bestCorr.name != "production" && bestCorr.m.corr-prod.corr > 0.05 {
		return "LSP route variant materially improves correlation; inspect " + bestCorr.name
	}
	return "no simple LSP bit-field/routing variant improves output; move to frame-local bitstream field sensitivity"
}
