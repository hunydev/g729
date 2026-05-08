package decoder

import (
	"testing"

	"github.com/hunydev/g729/internal/pitch"
)

func TestDecodeSubframe_ZeroGainProducesZero(t *testing.T) {
	var d Decoder
	sfA := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var out [40]int16

	var d2 Decoder
	var out2 [40]int16

	d.decodeSubframe(&sfA, 40, 0, 0, 0, 0, 0, out[:])
	d2.decodeSubframe(&sfA, 40, 0, 0, 0, 0, 0, out2[:])

	if out != out2 {
		t.Fatalf("two identical calls diverged: %v vs %v", out, out2)
	}
	if d.prevGpQ14 != d2.prevGpQ14 {
		t.Fatalf("prevGpQ14 diverged: %d vs %d", d.prevGpQ14, d2.prevGpQ14)
	}
	if d.pastExc != d2.pastExc {
		t.Fatal("pastExc FIFO diverged")
	}
}

func TestDecodeSubframe_PastExcFIFOSlides(t *testing.T) {
	var d Decoder
	for i := 0; i < 40; i++ {
		d.pastExc[i] = int16(100 + i)
	}
	sfA := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var out [40]int16
	d.decodeSubframe(&sfA, 40, 0, 0, 0, 0, 0, out[:])

	for i := 0; i < pastExcLen-40; i++ {
		if d.pastExc[i] != 0 {
			t.Fatalf("pastExc[%d] = %d (expected 0 after slide)", i, d.pastExc[i])
		}
	}
}

func TestDecodeSubframe_PrevGpUpdated(t *testing.T) {
	var d Decoder
	d.prevGpQ14 = 12345
	sfA := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var out [40]int16
	d.decodeSubframe(&sfA, 40, 0, 0, 0, 0, 0, out[:])
	if d.prevGpQ14 == 12345 {
		t.Fatal("prevGpQ14 not updated")
	}
}

func TestDecodeSubframe_TwoCallsDiffer(t *testing.T) {
	var d Decoder
	sfA := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var out1, out2 [40]int16
	// Use non-trivial gain/codebook indices so the postfilter does not
	// round the small all-zero-index excitation back to zero.
	d.decodeSubframe(&sfA, 40, 0, 0x1FFF, 0xF, 7, 15, out1[:])
	d.decodeSubframe(&sfA, 40, 0, 0x1FFF, 0xF, 7, 15, out2[:])
	if out1 == out2 {
		t.Fatal("two back-to-back calls produced identical output — state did not advance")
	}
}

func TestDecodeAdaptiveCodebookHonorsFractionalDelay(t *testing.T) {
	var pastExc [pastExcLen]int16
	for i := range pastExc {
		pastExc[i] = int16((i*73)%2000 - 1000)
	}

	var got, want, integerOnly [subframeLen]int16
	decodeAdaptiveCodebook(60, 1, pastExc[:], &got)
	pitch.AdaptiveCodebook(60, 1, pastExc[:], &want)
	pitch.AdaptiveCodebook(60, 0, pastExc[:], &integerOnly)

	if got != want {
		t.Fatal("strict ACB wrapper must preserve fractional pitch delay")
	}
	if got == integerOnly {
		t.Fatal("fractional pitch delay collapsed to integer-only ACB")
	}
}

func TestDecodeAdaptiveCodebookIntegerLagOnlyIsExplicitFallback(t *testing.T) {
	var pastExc [pastExcLen]int16
	for i := range pastExc {
		pastExc[i] = int16((i*97)%3000 - 1500)
	}

	var got, want [subframeLen]int16
	decodeAdaptiveCodebookIntegerLagOnly(60, pastExc[:], &got)
	pitch.AdaptiveCodebook(60, 0, pastExc[:], &want)
	if got != want {
		t.Fatal("integer-lag-only ACB fallback diverged from tFrac=0 ACB")
	}
}
