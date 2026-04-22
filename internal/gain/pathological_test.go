package gain

import (
	"testing"
)

// TestDecode_AllZeroCodebookIsBounded verifies that Decode does not
// saturate gc to int16 extrema when the fixed codebook happens to be
// all zeros.
func TestDecode_AllZeroCodebookIsBounded(t *testing.T) {
	var d Decoder
	var c [40]int16

	gpQ14, gcQ12 := d.Decode(Indices{GA: 3, GB: 7}, &c)

	if gpQ14 < -32768 || gpQ14 > 32767 {
		t.Errorf("gpQ14 out of int16 range: %d", gpQ14)
	}
	if gcQ12 == 32767 || gcQ12 == -32768 {
		t.Fatalf("all-zero codebook drove gcQ12 to int16 extremum: %d", gcQ12)
	}
}

// TestDecode_LowEnergyCodebookIsSmooth — single-pulse codebook.
func TestDecode_LowEnergyCodebookIsSmooth(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[0] = 8192
	gpQ14, gcQ12 := d.Decode(Indices{GA: 3, GB: 7}, &c)
	if gpQ14 < 0 || gpQ14 > 32767 {
		t.Errorf("gpQ14 out of expected range [0, 32767]: %d", gpQ14)
	}
	if gcQ12 == 32767 || gcQ12 == -32768 {
		t.Fatalf("single-pulse codebook drove gcQ12 to int16 extremum: %d", gcQ12)
	}
	if gcQ12 == 0 {
		t.Fatalf("single-pulse codebook produced zero gc — predictor collapsed")
	}
}

// TestDecode_HighEnergyCodebookIsBounded — canonical 4-pulse output.
func TestDecode_HighEnergyCodebookIsBounded(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[5] = 8192
	c[11] = 8192
	c[22] = 8192
	c[33] = 8192
	_, gcQ12 := d.Decode(Indices{GA: 3, GB: 7}, &c)
	if gcQ12 == 32767 || gcQ12 == -32768 {
		t.Fatalf("canonical 4-pulse codebook drove gcQ12 to int16 extremum: %d", gcQ12)
	}
}

// TestDecode_SucceedsAcrossAllGainIndices — full (GA, GB) sweep on the
// canonical 4-pulse codebook. No combination may saturate gc.
func TestDecode_SucceedsAcrossAllGainIndices(t *testing.T) {
	var c [40]int16
	c[5] = 8192
	c[11] = 8192
	c[22] = 8192
	c[33] = 8192

	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			var d Decoder
			_, gcQ12 := d.Decode(Indices{GA: ga, GB: gb}, &c)
			if gcQ12 == 32767 || gcQ12 == -32768 {
				t.Errorf("(GA=%d, GB=%d) saturated gcQ12 to %d", ga, gb, gcQ12)
			}
		}
	}
}
