package g729

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/exedev/g729/internal/pitch/openloop"
)

func measureOpenLoopPlausibility(t *testing.T, variant string) (hits int, hist map[int]int) {
	t.Helper()

	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read PITCH.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read PITCH.BIT: %v", err)
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	hist = map[int]int{}

	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}

		s := (*[FrameSamples]int16)(enc.oldSpeech[160:240])
		var top int16
		switch variant {
		case "unquantized":
			top = openloop.Step(&enc.aQ12Latest, s, &enc.lpResidualMem, &enc.swMem, &enc.oldWspeech)
		case "quantized-sf1":
			top = openloop.Step(&enc.aHatSF1, s, &enc.lpResidualMem, &enc.swMem, &enc.oldWspeech)
		case "quantized-sf2":
			top = openloop.Step(&enc.aHatSF2, s, &enc.lpResidualMem, &enc.swMem, &enc.oldWspeech)
		default:
			t.Fatalf("unknown variant %q", variant)
		}

		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		intT1 := decodeP1ToIntegerLag(extractP1FromG192(bitFrame))
		if intT1 >= int(top)-5 && intT1 <= int(top)+4 {
			hits++
		}
		hist[intT1-int(top)]++
	}
	return hits, hist
}

func logHCenterHistogram(t *testing.T, variant string, hist map[int]int) {
	t.Helper()
	deltas := make([]int, 0, len(hist))
	for d := range hist {
		deltas = append(deltas, d)
	}
	sort.Ints(deltas)
	var buckets string
	for _, d := range deltas {
		c := hist[d]
		if c >= 1835/200 {
			buckets += fmt.Sprintf(" Δ=%+d:%d(%.1f%%)", d, c, 100*float64(c)/1835)
		}
	}
	t.Logf("%s delta histogram (buckets >=0.5%%):%s", variant, buckets)
}

func TestPhase2bHCenter_HOQ2QuantizedDiagnostic(t *testing.T) {
	for _, variant := range []string{"unquantized", "quantized-sf1", "quantized-sf2"} {
		hits, hist := measureOpenLoopPlausibility(t, variant)
		t.Logf("H-OQ2 %s: %d/1835 plausibility %.2f%%", variant, hits, 100*float64(hits)/1835)
		logHCenterHistogram(t, variant, hist)
	}
}

func TestPhase2bHCenter_WriteMeasurementArtifacts(t *testing.T) {
	if os.Getenv("G729_WRITE_HCENTER_LOGS") != "1" {
		t.Skip("set G729_WRITE_HCENTER_LOGS=1 to refresh H-CENTER measurement artifacts")
	}

	const (
		inPath           = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		bitPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.BIT"
		outPath          = "testdata/phase2b/hcenter_top_vs_t1.csv"
		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 1835
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read PITCH.IN: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read PITCH.BIT: %v", err)
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16
	var b strings.Builder
	b.WriteString("frame,t_op,int_t1,delta,plausible\n")
	hits := 0
	hist := map[int]int{}
	for f := 0; f < totalFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d: lpcStep: %v", f, err)
		}
		top := enc.openloopStep()
		bitFrame := bitData[f*bytesPerBitFrame : (f+1)*bytesPerBitFrame]
		intT1 := decodeP1ToIntegerLag(extractP1FromG192(bitFrame))
		delta := intT1 - int(top)
		plausible := intT1 >= int(top)-5 && intT1 <= int(top)+4
		if plausible {
			hits++
		}
		hist[delta]++
		b.WriteString(fmt.Sprintf("%d,%d,%d,%d,%t\n", f, top, intT1, delta, plausible))
	}
	if err := os.MkdirAll("testdata/phase2b", 0o755); err != nil {
		t.Fatalf("mkdir testdata/phase2b: %v", err)
	}
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", outPath, err)
	}
	t.Logf("wrote %s: %d/1835 plausibility %.2f%%", outPath, hits, 100*float64(hits)/1835)
	logHCenterHistogram(t, "artifact", hist)
}
