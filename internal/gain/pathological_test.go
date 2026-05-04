package gain

import (
	"testing"
)

// TestDecode_AllZeroCodebookIsBounded verifies that Decode returns the
// zero-energy guard tuple (mant=0, exp=0) when the fixed codebook is
// all zeros, instead of producing a saturating bogus gain. (Per REF-1
// §2 invariant.)
func TestDecode_AllZeroCodebookIsBounded(t *testing.T) {
	var d Decoder
	var c [40]int16

	gpQ14, mant, exp := d.Decode(Indices{GA: 3, GB: 7}, &c)

	if gpQ14 < -32768 || gpQ14 > 32767 {
		t.Errorf("gpQ14 out of int16 range: %d", gpQ14)
	}
	if mant != 0 || exp != 0 {
		t.Fatalf("all-zero codebook: got (mant=%d, exp=%d), want zero-energy guard (0, 0)", mant, exp)
	}
}

// TestDecode_LowEnergyCodebookIsSmooth — single-pulse codebook. The
// (mant, exp) representation must be well-formed (mant in the Q14
// fundamental octave, exp finite) and g_p must stay in the unit-Q14
// envelope.
func TestDecode_LowEnergyCodebookIsSmooth(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[0] = 8192
	gpQ14, mant, exp := d.Decode(Indices{GA: 3, GB: 7}, &c)
	if gpQ14 < 0 || gpQ14 > 32767 {
		t.Errorf("gpQ14 out of expected range [0, 32767]: %d", gpQ14)
	}
	if mant < 16384 || mant > 32767 {
		t.Fatalf("single-pulse codebook: mant=%d outside Q14 fundamental octave [16384, 32767]", mant)
	}
	if exp < -64 || exp > 64 {
		t.Errorf("single-pulse codebook: exp=%d implausible (corpus envelope is [-15, +9])", exp)
	}
}

// TestDecode_HighEnergyCodebookIsBounded — canonical 4-pulse output.
// (mant, exp) must remain well-formed even at the high-energy end.
func TestDecode_HighEnergyCodebookIsBounded(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[5] = 8192
	c[11] = 8192
	c[22] = 8192
	c[33] = 8192
	_, mant, exp := d.Decode(Indices{GA: 3, GB: 7}, &c)
	if mant < 16384 || mant > 32767 {
		t.Fatalf("4-pulse codebook: mant=%d outside [16384, 32767]", mant)
	}
	if exp < -64 || exp > 64 {
		t.Errorf("4-pulse codebook: exp=%d implausible (corpus envelope is [-15, +9])", exp)
	}
}

// TestDecode_SucceedsAcrossAllGainIndices — full (GA, GB) sweep on the
// canonical 4-pulse codebook. Every combination must yield a
// well-formed (mant, exp) pair — REF-1 §2 invariants. Linear g_c may
// legitimately exceed the legacy Q12 envelope (DIAG-1 documents 100%
// int16 wrap on gc0); the new representation accommodates that range.
func TestDecode_SucceedsAcrossAllGainIndices(t *testing.T) {
	var c [40]int16
	c[5] = 8192
	c[11] = 8192
	c[22] = 8192
	c[33] = 8192

	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			var d Decoder
			_, mant, exp := d.Decode(Indices{GA: ga, GB: gb}, &c)
			if mant < 16384 || mant > 32767 {
				t.Errorf("(GA=%d, GB=%d) mant=%d out of [16384, 32767]", ga, gb, mant)
			}
			if exp < -64 || exp > 64 {
				t.Errorf("(GA=%d, GB=%d) exp=%d out of [-64, +64]", ga, gb, exp)
			}
		}
	}
}
