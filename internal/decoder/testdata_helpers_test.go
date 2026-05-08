package decoder

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// readG192Frames loads a G.192 bitstream file (ITU Annex A .bit format)
// from path, returning packed frame bytes and bad-flag slice.
func readG192Frames(tb testing.TB, path string) ([][]byte, []bool) {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("readG192Frames: %v", err)
	}
	frames, bads, err := bitstream.ReadG192File(bytes.NewReader(data))
	if err != nil {
		tb.Fatalf("ReadG192File(%s): %v", path, err)
	}
	return frames, bads
}

// readPSTFrames loads a raw 16-bit little-endian PCM file (ITU Annex A
// .pst format) from path, split into consecutive 80-sample frames.
func readPSTFrames(tb testing.TB, path string) [][80]int16 {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("readPSTFrames: %v", err)
	}
	if len(data)%(frameSamples*2) != 0 {
		tb.Fatalf("readPSTFrames(%s): size %d is not a multiple of %d",
			path, len(data), frameSamples*2)
	}
	nFrames := len(data) / (frameSamples * 2)
	out := make([][80]int16, nFrames)
	for i := 0; i < nFrames; i++ {
		for n := 0; n < frameSamples; n++ {
			off := (i*frameSamples + n) * 2
			out[i][n] = int16(binary.LittleEndian.Uint16(data[off : off+2]))
		}
	}
	return out
}

// vectorPath builds a path into the Annex A test-vector tree.
func vectorPath(name string) string {
	return filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", name)
}

// ensureTestdataPresent skips the test if any path is missing.
func ensureTestdataPresent(tb testing.TB, paths ...string) {
	tb.Helper()
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			tb.Skipf("missing test vector %s: %v", p, err)
		}
	}
}
