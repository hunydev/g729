package fcb

import (
	"testing"
)

// TestDecodePositions_C6134 locks in the expected position decoding for
// the specific index that Phase 1g reported as problematic.
func TestDecodePositions_C6134(t *testing.T) {
	got := decodePositions(6134)
	want := [4]int{30, 31, 37, 29}
	if got != want {
		t.Fatalf("decodePositions(6134) = %v, want %v", got, want)
	}
	seen := map[int]bool{}
	for _, p := range got {
		if seen[p] {
			t.Errorf("duplicate position %d in %v", p, got)
		}
		seen[p] = true
	}
}

// TestPlacePulses_AllPositive_C6134 confirms S=0xF produces four
// +PulseAmplitude pulses at the declared positions with all other
// samples zero.
func TestPlacePulses_AllPositive_C6134(t *testing.T) {
	var c [40]int16
	placePulses([4]int{30, 31, 37, 29}, 0xF, &c)
	for i, v := range c {
		switch i {
		case 29, 30, 31, 37:
			if v != PulseAmplitude {
				t.Errorf("c[%d] = %d, want %d (+PulseAmplitude)", i, v, PulseAmplitude)
			}
		default:
			if v != 0 {
				t.Errorf("c[%d] = %d, want 0", i, v)
			}
		}
	}
}

// TestDecode_C6134_AtLeastFourNonZeroSamples is the end-to-end invariant:
// fcb.Decode on this index must yield a c[] with at least 4 non-zero
// samples (pitch enhancement may add more).
func TestDecode_C6134_AtLeastFourNonZeroSamples(t *testing.T) {
	var c [40]int16
	Decode(Indices{Positions: 6134, Signs: 0xF}, 20, 0, &c)

	n := 0
	for _, v := range c {
		if v != 0 {
			n++
		}
	}
	if n < 4 {
		t.Fatalf("Decode((6134, 0xF), 20, 0) produced only %d non-zero samples: %v", n, c)
	}
}

// TestDecode_C6134_WithBetaNonZero exercises the pitch-enhancement
// branch and verifies the output's energy is non-trivial.
func TestDecode_C6134_WithBetaNonZero(t *testing.T) {
	var c [40]int16
	Decode(Indices{Positions: 6134, Signs: 0xF}, 20, 8192, &c)

	var energy int64
	for _, v := range c {
		energy += int64(v) * int64(v)
	}
	const floor int64 = 200_000_000
	if energy < floor {
		t.Fatalf("Decode((6134, 0xF), 20, 8192) energy = %d, want >= %d", energy, floor)
	}
}

// TestDecode_ExhaustiveSignsPreservePulseCount: for every sign mask in
// [0, 15] and a handful of position codes, Decode with β=0 must produce
// exactly 4 non-zero samples.
func TestDecode_ExhaustiveSignsPreservePulseCount(t *testing.T) {
	codes := []uint16{0, 1, 42, 1023, 4096, 6134, 7999, 8191}
	for _, code := range codes {
		for s := uint8(0); s < 16; s++ {
			var c [40]int16
			Decode(Indices{Positions: code, Signs: s}, 20, 0, &c)
			n := 0
			for _, v := range c {
				if v != 0 {
					n++
				}
			}
			if n != 4 {
				t.Errorf("Decode((%d, %d), 20, 0): want 4 non-zero samples, got %d", code, s, n)
			}
		}
	}
}
