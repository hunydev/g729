package g729

import (
	"encoding/binary"
	"os"
	"testing"
)

// TestINT1D10CorpusDiagnostic — read-only diagnostic for FIX-3-B.
// Reports per-frame got/want around frame 596, the LSP-reuse counter,
// and convergence rates (first 50 frames vs last 500 frames).
func TestINT1D10CorpusDiagnostic(t *testing.T) {
	const (
		inPath  = "testdata/itu/G729_Release3/g729/test_vectors/LSP.IN"
		bitPath = "testdata/itu/G729_Release3/g729/test_vectors/LSP.BIT"

		samplesPerFrame  = 80
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 2232
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Skipf("LSP.IN unavailable: %v", err)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Skipf("LSP.BIT unavailable: %v", err)
	}

	enc := NewEncoder()
	var pcm [samplesPerFrame]int16

	type frameRes struct{ got, want [4]uint8 }
	results := make([]frameRes, totalFrames)

	for f := 0; f < totalFrames; f++ {
		off := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[off+2*i : off+2*i+2]))
		}
		idx, err := enc.lpcStep(pcm[:])
		if err != nil {
			t.Fatalf("frame %d: lpcStep error: %v", f, err)
		}
		bo := f * bytesPerBitFrame
		w0, w1, w2, w3 := extractLSPFieldsFromG192(bitData[bo : bo+bytesPerBitFrame])
		results[f] = frameRes{
			got:  [4]uint8{idx.L0, idx.L1, idx.L2, idx.L3},
			want: [4]uint8{w0, w1, w2, w3},
		}
	}

	t.Logf("LSP reuse count (FIX-3-B fired): %d / %d frames", enc.LSPReuseCount(), totalFrames)

	// Frame-596 specifics.
	for _, f := range []int{595, 596, 597, 598} {
		r := results[f]
		t.Logf("frame %d: got=(%d,%d,%d,%d) want=(%d,%d,%d,%d)",
			f, r.got[0], r.got[1], r.got[2], r.got[3],
			r.want[0], r.want[1], r.want[2], r.want[3])
	}

	rate := func(lo, hi int) (l0, l1, l2, l3 int) {
		for i := lo; i < hi; i++ {
			r := results[i]
			if r.got[0] == r.want[0] {
				l0++
			}
			if r.got[1] == r.want[1] {
				l1++
			}
			if r.got[2] == r.want[2] {
				l2++
			}
			if r.got[3] == r.want[3] {
				l3++
			}
		}
		return
	}
	pct := func(n, d int) float64 { return 100.0 * float64(n) / float64(d) }

	a0, a1, a2, a3 := rate(0, totalFrames)
	t.Logf("FULL  [0..%d): L0=%5.2f%% L1=%5.2f%% L2=%5.2f%% L3=%5.2f%%",
		totalFrames, pct(a0, totalFrames), pct(a1, totalFrames), pct(a2, totalFrames), pct(a3, totalFrames))

	f0, f1, f2, f3 := rate(0, 50)
	t.Logf("FIRST [0..50): L0=%5.2f%% L1=%5.2f%% L2=%5.2f%% L3=%5.2f%%",
		pct(f0, 50), pct(f1, 50), pct(f2, 50), pct(f3, 50))

	l0, l1, l2, l3 := rate(totalFrames-500, totalFrames)
	t.Logf("LAST  [%d..%d): L0=%5.2f%% L1=%5.2f%% L2=%5.2f%% L3=%5.2f%%",
		totalFrames-500, totalFrames, pct(l0, 500), pct(l1, 500), pct(l2, 500), pct(l3, 500))

	m0, m1, m2, m3 := rate(596, 1096)
	t.Logf("POST596 [596..1096): L0=%5.2f%% L1=%5.2f%% L2=%5.2f%% L3=%5.2f%%",
		pct(m0, 500), pct(m1, 500), pct(m2, 500), pct(m3, 500))
}
