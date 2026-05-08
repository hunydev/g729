package decoder

import (
	"bytes"
	"os"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3mFrameLocalFieldSensitivity_SPEECH runs frame-local bitstream
// group replacements on the worst production frames. It starts each probe from
// the production decoder state immediately before that frame, mutates only the
// current frame's transmitted parameters, and compares only the current
// 80-sample frame against SPEECH.PST.
func TestPhase3mFrameLocalFieldSensitivity_SPEECH(t *testing.T) {
	if os.Getenv("G729_DECODER_FRAME_LOCAL_SENSITIVITY") != "1" {
		t.Skip("set G729_DECODER_FRAME_LOCAL_SENSITIVITY=1 to run frame-local field sensitivity")
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
	packedFrames := phase3mReadPackedFrames(t, bitData, frames)
	states, production := phase3mDecodeWithStateSnapshots(t, packedFrames)
	worst := blackboxWorstFrames(ref, production, 24)

	variants := []phase3mLocalVariant{
		{name: "lsp_prev", apply: func(cur, other *bitstream.Frame) {
			cur.L0, cur.L1, cur.L2, cur.L3 = other.L0, other.L1, other.L2, other.L3
		}, sourceOffset: -1},
		{name: "lsp_next", apply: func(cur, other *bitstream.Frame) {
			cur.L0, cur.L1, cur.L2, cur.L3 = other.L0, other.L1, other.L2, other.L3
		}, sourceOffset: +1},
		{name: "pitch_prev", apply: func(cur, other *bitstream.Frame) { cur.P1, cur.P0, cur.P2 = other.P1, other.P0, other.P2 }, sourceOffset: -1},
		{name: "pitch_next", apply: func(cur, other *bitstream.Frame) { cur.P1, cur.P0, cur.P2 = other.P1, other.P0, other.P2 }, sourceOffset: +1},
		{name: "fcb_prev", apply: func(cur, other *bitstream.Frame) {
			cur.C1, cur.S1, cur.C2, cur.S2 = other.C1, other.S1, other.C2, other.S2
		}, sourceOffset: -1},
		{name: "fcb_next", apply: func(cur, other *bitstream.Frame) {
			cur.C1, cur.S1, cur.C2, cur.S2 = other.C1, other.S1, other.C2, other.S2
		}, sourceOffset: +1},
		{name: "gain_prev", apply: func(cur, other *bitstream.Frame) {
			cur.GA1, cur.GB1, cur.GA2, cur.GB2 = other.GA1, other.GB1, other.GA2, other.GB2
		}, sourceOffset: -1},
		{name: "gain_next", apply: func(cur, other *bitstream.Frame) {
			cur.GA1, cur.GB1, cur.GA2, cur.GB2 = other.GA1, other.GB1, other.GA2, other.GB2
		}, sourceOffset: +1},
		{name: "all_prev", apply: func(cur, other *bitstream.Frame) { *cur = *other }, sourceOffset: -1},
		{name: "all_next", apply: func(cur, other *bitstream.Frame) { *cur = *other }, sourceOffset: +1},
		{name: "sf1_gain_from_sf2", apply: func(cur, _ *bitstream.Frame) { cur.GA1, cur.GB1 = cur.GA2, cur.GB2 }},
		{name: "sf2_gain_from_sf1", apply: func(cur, _ *bitstream.Frame) { cur.GA2, cur.GB2 = cur.GA1, cur.GB1 }},
		{name: "sf1_fcb_from_sf2", apply: func(cur, _ *bitstream.Frame) { cur.C1, cur.S1 = cur.C2, cur.S2 }},
		{name: "sf2_fcb_from_sf1", apply: func(cur, _ *bitstream.Frame) { cur.C2, cur.S2 = cur.C1, cur.S1 }},
	}

	results := make([]phase3mLocalResult, 0, len(worst))
	counts := make(map[string]int)
	for _, w := range worst {
		refFrame := ref[w.frame*frameSamples : (w.frame+1)*frameSamples]
		prodFrame := production[w.frame*frameSamples : (w.frame+1)*frameSamples]
		prodM := blackboxMeasure(refFrame, prodFrame, 5)
		best := phase3mVariantMetric{name: "production", m: prodM}
		for _, v := range variants {
			if w.frame+v.sourceOffset < 0 || w.frame+v.sourceOffset >= frames {
				continue
			}
			out := phase3mDecodeOneFrameVariant(t, states[w.frame], packedFrames, w.frame, v)
			m := blackboxMeasure(refFrame, out[:], 5)
			cand := phase3mVariantMetric{name: v.name, m: m}
			if phase3mBetterLocal(cand, best) {
				best = cand
			}
		}
		counts[best.name]++
		results = append(results, phase3mLocalResult{frame: w.frame, prod: prodM, best: best})
	}

	t.Logf("Phase 3m frame-local field sensitivity - SPEECH.BIT/SPEECH.PST (%d frames)", frames)
	t.Logf("worst-frame sample=%d; each probe starts from production state before the target frame", len(results))
	t.Logf("")
	t.Logf("%-7s %9s %8s %9s %8s %10s %10s %9s",
		"frame", "prodSNR", "prodCorr", "bestSNR", "bestCorr", "best", "dSNR", "dCorr")
	t.Logf("%-7s %9s %8s %9s %8s %10s %10s %9s",
		"-----", "-------", "--------", "-------", "--------", "----", "----", "-----")
	for _, r := range results {
		t.Logf("%-7d %9.2f %8.3f %9.2f %8.3f %10s %10.2f %9.3f",
			r.frame, r.prod.globalSNR, r.prod.corr,
			r.best.m.globalSNR, r.best.m.corr, r.best.name,
			r.best.m.globalSNR-r.prod.globalSNR, r.best.m.corr-r.prod.corr)
	}
	t.Logf("")
	t.Logf("best-variant counts: %s", phase3mFormatCounts(counts))
	t.Logf("verdict: %s", phase3mLocalVerdict(results))
}

type phase3mLocalVariant struct {
	name         string
	sourceOffset int
	apply        func(cur, other *bitstream.Frame)
}

type phase3mVariantMetric struct {
	name string
	m    blackboxMetrics
}

type phase3mLocalResult struct {
	frame int
	prod  blackboxMetrics
	best  phase3mVariantMetric
}

func phase3mReadPackedFrames(t *testing.T, bitData []byte, frames int) [][bitstream.FrameBytes]byte {
	t.Helper()
	out := make([][bitstream.FrameBytes]byte, frames)
	r := bytes.NewReader(bitData)
	for f := 0; f < frames; f++ {
		if _, err := bitstream.ReadG192Frame(r, out[f][:]); err != nil {
			t.Fatalf("ReadG192Frame[phase3m] frame %d: %v", f, err)
		}
	}
	return out
}

func phase3mDecodeWithStateSnapshots(t *testing.T, frames [][bitstream.FrameBytes]byte) ([]Decoder, []int16) {
	t.Helper()
	states := make([]Decoder, len(frames))
	out := make([]int16, len(frames)*frameSamples)
	var dec Decoder
	for f := range frames {
		states[f] = dec
		if err := dec.Decode(frames[f][:], false, out[f*frameSamples:(f+1)*frameSamples]); err != nil {
			t.Fatalf("Decode[phase3m] frame %d: %v", f, err)
		}
	}
	return states, out
}

func phase3mDecodeOneFrameVariant(
	t *testing.T,
	state Decoder,
	packedFrames [][bitstream.FrameBytes]byte,
	frame int,
	variant phase3mLocalVariant,
) [frameSamples]int16 {
	t.Helper()
	var cur bitstream.Frame
	if err := bitstream.Unpack(packedFrames[frame][:], &cur); err != nil {
		t.Fatalf("Unpack[phase3m] frame %d: %v", frame, err)
	}
	var other bitstream.Frame
	src := frame + variant.sourceOffset
	if src >= 0 && src < len(packedFrames) {
		if err := bitstream.Unpack(packedFrames[src][:], &other); err != nil {
			t.Fatalf("Unpack[phase3m] source frame %d: %v", src, err)
		}
	}
	variant.apply(&cur, &other)
	var packed [bitstream.FrameBytes]byte
	if err := bitstream.Pack(&cur, packed[:]); err != nil {
		t.Fatalf("Pack[phase3m] frame %d variant %s: %v", frame, variant.name, err)
	}
	var out [frameSamples]int16
	if err := state.Decode(packed[:], false, out[:]); err != nil {
		t.Fatalf("Decode[phase3m] frame %d variant %s: %v", frame, variant.name, err)
	}
	return out
}

func phase3mBetterLocal(cand, best phase3mVariantMetric) bool {
	if cand.m.corr-best.m.corr > 0.03 {
		return true
	}
	if best.m.corr-cand.m.corr > 0.03 {
		return false
	}
	return cand.m.globalSNR > best.m.globalSNR
}

func phase3mFormatCounts(counts map[string]int) string {
	type kv struct {
		k string
		v int
	}
	rows := make([]kv, 0, len(counts))
	for k, v := range counts {
		rows = append(rows, kv{k: k, v: v})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].v != rows[j].v {
			return rows[i].v > rows[j].v
		}
		return rows[i].k < rows[j].k
	})
	out := ""
	for i, r := range rows {
		if i > 0 {
			out += ", "
		}
		out += r.k + "=" + phase3mItoa(r.v)
	}
	return out
}

func phase3mItoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func phase3mLocalVerdict(results []phase3mLocalResult) string {
	material := 0
	for _, r := range results {
		if r.best.name != "production" && (r.best.m.globalSNR-r.prod.globalSNR > 1.0 || r.best.m.corr-r.prod.corr > 0.1) {
			material++
		}
	}
	if material >= len(results)/3 {
		return "neighbor/subframe field replacement materially improves many worst frames; inspect dominant replacement group"
	}
	return "neighbor/subframe field replacement does not consistently rescue worst frames; no one-frame field misalignment signal"
}
