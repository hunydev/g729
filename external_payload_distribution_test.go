package g729

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/pitch"
)

func TestExternalPayloadDistribution_AsteriskVsLocalEncoder(t *testing.T) {
	if os.Getenv("G729_EXTERNAL_PAYLOAD_DISTRIBUTION") != "1" {
		t.Skip("set G729_EXTERNAL_PAYLOAD_DISTRIBUTION=1 to compare external payload index distribution")
	}

	asteriskRaw := readFile(t, filepath.Join("testdata", "external", "asterisk_payload.g729"))

	vecDir := filepath.Join("testdata", "itu", "G729_Release3", "g729AnnexA", "test_vectors")
	inData := readFile(t, filepath.Join(vecDir, "SPEECH.IN"))
	src := s16leToSamples(inData)
	tmp := t.TempDir()
	ourPath := filepath.Join(tmp, "speech-our-encoder.g729")
	writeOurEncodedRawG729(t, src, ourPath)
	ourRaw := readFile(t, ourPath)

	asterisk := collectPayloadDistribution(t, "asterisk", asteriskRaw)
	our := collectPayloadDistribution(t, "our_encoder", ourRaw)

	t.Logf("external payload distribution: asterisk frames=%d subframes=%d ; our_encoder frames=%d subframes=%d",
		asterisk.frames, asterisk.subframes, our.frames, our.subframes)
	t.Logf("asterisk top gain pairs: %s", formatPayloadStringHist(asterisk.gainPairs, asterisk.subframes, 12))
	t.Logf("our_encoder top gain pairs: %s", formatPayloadStringHist(our.gainPairs, our.subframes, 12))
	t.Logf("gain-pair overrepresentation in asterisk vs our_encoder: %s",
		formatPayloadStringHistDelta(asterisk.gainPairs, asterisk.subframes, our.gainPairs, our.subframes, 12))
	t.Logf("asterisk GA hist: %s", formatPayloadIntHist(asterisk.ga, asterisk.subframes, 8))
	t.Logf("our_encoder GA hist: %s", formatPayloadIntHist(our.ga, our.subframes, 8))
	t.Logf("asterisk GB hist: %s", formatPayloadIntHist(asterisk.gb, asterisk.subframes, 16))
	t.Logf("our_encoder GB hist: %s", formatPayloadIntHist(our.gb, our.subframes, 16))
	t.Logf("asterisk pitch bucket hist: %s", formatPayloadStringHist(asterisk.pitchBuckets, asterisk.subframes, 8))
	t.Logf("our_encoder pitch bucket hist: %s", formatPayloadStringHist(our.pitchBuckets, our.subframes, 8))
	t.Logf("asterisk pitch fraction hist: %s", formatPayloadIntHist(asterisk.pitchFrac, asterisk.subframes, 5))
	t.Logf("our_encoder pitch fraction hist: %s", formatPayloadIntHist(our.pitchFrac, our.subframes, 5))
}

type payloadDistribution struct {
	name         string
	frames       int
	subframes    int
	ga           map[int]int
	gb           map[int]int
	gainPairs    map[string]int
	pitchBuckets map[string]int
	pitchFrac    map[int]int
}

func collectPayloadDistribution(t *testing.T, name string, raw []byte) payloadDistribution {
	t.Helper()
	if len(raw)%FrameBytes != 0 {
		t.Fatalf("%s raw length %d is not divisible by %d", name, len(raw), FrameBytes)
	}
	d := payloadDistribution{
		name:         name,
		frames:       len(raw) / FrameBytes,
		ga:           map[int]int{},
		gb:           map[int]int{},
		gainPairs:    map[string]int{},
		pitchBuckets: map[string]int{},
		pitchFrac:    map[int]int{},
	}
	for frame := 0; frame < d.frames; frame++ {
		var f bitstream.Frame
		off := frame * FrameBytes
		if err := bitstream.Unpack(raw[off:off+FrameBytes], &f); err != nil {
			t.Fatalf("%s frame %d unpack: %v", name, frame, err)
		}
		t1, frac1 := pitch.DecodeDelaySubframe1(uint8(f.P1))
		t2, frac2 := pitch.DecodeDelaySubframe2(uint8(f.P2), t1)
		d.addSubframe(int(f.GA1), int(f.GB1), t1, frac1)
		d.addSubframe(int(f.GA2), int(f.GB2), t2, frac2)
	}
	return d
}

func (d *payloadDistribution) addSubframe(ga, gb, tInt, tFrac int) {
	d.subframes++
	d.ga[ga]++
	d.gb[gb]++
	d.gainPairs[fmt.Sprintf("GA%d/GB%d", ga, gb)]++
	d.pitchBuckets[payloadPitchBucket(tInt)]++
	d.pitchFrac[tFrac]++
}

func payloadPitchBucket(tInt int) string {
	switch {
	case tInt < 40:
		return "t<40"
	case tInt < 80:
		return "t40..79"
	default:
		return "t>=80"
	}
}

func formatPayloadIntHist(hist map[int]int, total, limit int) string {
	type row struct {
		key   int
		count int
	}
	rows := make([]row, 0, len(hist))
	for key, count := range hist {
		rows = append(rows, row{key: key, count: count})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].key < rows[j].key
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := ""
	for _, r := range rows {
		out += fmt.Sprintf(" %d:%d(%.1f%%)", r.key, r.count, payloadPercent(r.count, total))
	}
	return out
}

func formatPayloadStringHist(hist map[string]int, total, limit int) string {
	type row struct {
		key   string
		count int
	}
	rows := make([]row, 0, len(hist))
	for key, count := range hist {
		rows = append(rows, row{key: key, count: count})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].key < rows[j].key
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := ""
	for _, r := range rows {
		out += fmt.Sprintf(" %s:%d(%.1f%%)", r.key, r.count, payloadPercent(r.count, total))
	}
	return out
}

func formatPayloadStringHistDelta(a map[string]int, totalA int, b map[string]int, totalB int, limit int) string {
	type row struct {
		key   string
		delta float64
		aPct  float64
		bPct  float64
	}
	seen := map[string]bool{}
	for key := range a {
		seen[key] = true
	}
	for key := range b {
		seen[key] = true
	}
	rows := make([]row, 0, len(seen))
	for key := range seen {
		aPct := payloadPercent(a[key], totalA)
		bPct := payloadPercent(b[key], totalB)
		rows = append(rows, row{key: key, delta: aPct - bPct, aPct: aPct, bPct: bPct})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].delta != rows[j].delta {
			return rows[i].delta > rows[j].delta
		}
		return rows[i].key < rows[j].key
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := ""
	for _, r := range rows {
		out += fmt.Sprintf(" %s:%+.1fpp(a=%.1f%% our=%.1f%%)", r.key, r.delta, r.aPct, r.bPct)
	}
	return out
}

func payloadPercent(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(count) / float64(total)
}
