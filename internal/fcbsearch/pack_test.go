package fcbsearch_test

import (
	"testing"

	"github.com/hunydev/g729/internal/fcbsearch"
)

// ENC-1 RED tests for §3.8.2 eq. 61/62 fixed-codebook bit packing.
//
// Eq. 61 (G729E.txt §3.8.2) defines the 4-bit sign field S where the
// pulse signs are read out of signs[positions[i]] in the §3.8.1
// decomposition convention (sign ∈ {−1, +1}). The decoder side is
// fcb.placePulses (internal/fcb/signs.go): bit i = 1 → +pulse,
// bit i = 0 → -pulse. This file pins encoder PackS to match §3.8.2
// eq. (61) and the decoder so round-trip holds.
//
// Eq. 62 defines the 13-bit position code C with the layout:
//
//	bits  2..0  = i0     pos[0] = 5*i0          (track 0)
//	bits  5..3  = i1     pos[1] = 5*i1 + 1      (track 1)
//	bits  8..6  = i2     pos[2] = 5*i2 + 2      (track 2)
//	bit      9  = jx     track-3 half selector
//	bits 12..10 = i3     pos[3] = 5*i3 + 3 + jx (track 3a/3b)
//
// jx convention (matches decoder's decodePositions in
// internal/fcb/positions.go): pos[3] ∈ {3,8,13,18,23,28,33,38} → jx=0;
// pos[3] ∈ {4,9,14,19,24,29,34,39} → jx=1.

// unpackC mirrors fcb.decodePositions on int8 outputs (test-only
// helper kept local to avoid exporting decoder internals).
func unpackC(code uint16) [4]int8 {
	i0 := int8(code & 0x07)
	i1 := int8((code >> 3) & 0x07)
	i2 := int8((code >> 6) & 0x07)
	jx := int8((code >> 9) & 0x01)
	i3 := int8((code >> 10) & 0x07)
	return [4]int8{
		5 * i0,
		5*i1 + 1,
		5*i2 + 2,
		5*i3 + 3 + jx,
	}
}

func TestPackC_RoundTripExhaustive(t *testing.T) {
	for code := uint16(0); code < (1 << 13); code++ {
		positions := unpackC(code)
		got := fcbsearch.PackC(&positions)
		if got != code {
			t.Fatalf("PackC(%v) = 0x%04x, want 0x%04x", positions, got, code)
		}
	}
}

func TestPackC_AllZero(t *testing.T) {
	positions := [4]int8{0, 1, 2, 3}
	got := fcbsearch.PackC(&positions)
	if got != 0 {
		t.Fatalf("PackC(%v) = 0x%04x, want 0", positions, got)
	}
}

func TestPackC_Track3SplitJx0(t *testing.T) {
	// pos[3] ∈ track 3a {3,8,13,18,23,28,33,38} → jx bit clear.
	for _, p3 := range []int8{3, 8, 13, 18, 23, 28, 33, 38} {
		positions := [4]int8{0, 1, 2, p3}
		code := fcbsearch.PackC(&positions)
		if (code>>9)&1 != 0 {
			t.Errorf("pos[3]=%d: jx bit set in 0x%04x, want 0", p3, code)
		}
	}
}

func TestPackC_Track3SplitJx1(t *testing.T) {
	// pos[3] ∈ track 3b {4,9,14,19,24,29,34,39} → jx bit set.
	for _, p3 := range []int8{4, 9, 14, 19, 24, 29, 34, 39} {
		positions := [4]int8{0, 1, 2, p3}
		code := fcbsearch.PackC(&positions)
		if (code>>9)&1 != 1 {
			t.Errorf("pos[3]=%d: jx bit clear in 0x%04x, want 1", p3, code)
		}
	}
}

func TestPackC_NoAlloc(t *testing.T) {
	positions := [4]int8{15, 26, 7, 33}
	if got := testing.AllocsPerRun(128, func() {
		_ = fcbsearch.PackC(&positions)
	}); got != 0 {
		t.Fatalf("PackC allocations/op = %v, want 0", got)
	}
}

// PackS round-trip: build signs[40] with chosen polarity at the four
// pulse positions, pack, then verify each bit matches the decoder
// convention: bit i = 1 ↔ signs[positions[i]] > 0.
func TestPackS_RoundTripAllCombos(t *testing.T) {
	positions := [4]int8{5, 11, 22, 38}
	for mask := 0; mask < 16; mask++ {
		var signs [40]int16
		for n := range signs {
			signs[n] = +1 // benign default at non-pulse indices
		}
		for i := 0; i < 4; i++ {
			if (mask>>uint(i))&1 == 1 {
				signs[positions[i]] = +1
			} else {
				signs[positions[i]] = -1
			}
		}
		got := fcbsearch.PackS(&positions, &signs)
		if got != uint8(mask) {
			t.Fatalf("mask=0x%X: PackS = 0x%X, want 0x%X", mask, got, mask)
		}
		// Decoder-side equivalence: each bit i must reflect the sign
		// at signs[positions[i]] per fcb.placePulses contract.
		for i := 0; i < 4; i++ {
			bit := (got >> uint(i)) & 1
			pos := positions[i]
			if signs[pos] > 0 && bit != 1 {
				t.Errorf("mask=0x%X i=%d: bit=%d, want 1 (signs[%d]=+1)", mask, i, bit, pos)
			}
			if signs[pos] <= 0 && bit != 0 {
				t.Errorf("mask=0x%X i=%d: bit=%d, want 0 (signs[%d]=-1)", mask, i, bit, pos)
			}
		}
	}
}

func TestPackS_ZeroTreatedAsNegative(t *testing.T) {
	// SignsFromD sets signs[n] = +1 for d[n] >= 0; the encoder side
	// of PackS only ever sees signs ∈ {−1, +1} produced by SignsFromD.
	// Pin behavior: a zero (defensive) is treated as non-positive
	// (bit=0), matching fcb.placePulses (`if signs[p] > 0`).
	positions := [4]int8{0, 1, 2, 3}
	var signs [40]int16 // zero
	got := fcbsearch.PackS(&positions, &signs)
	if got != 0 {
		t.Fatalf("PackS(zero signs) = 0x%X, want 0", got)
	}
}

func TestPackS_NoAlloc(t *testing.T) {
	positions := [4]int8{0, 6, 17, 28}
	var signs [40]int16
	for n := range signs {
		signs[n] = +1
	}
	signs[6] = -1
	if got := testing.AllocsPerRun(128, func() {
		_ = fcbsearch.PackS(&positions, &signs)
	}); got != 0 {
		t.Fatalf("PackS allocations/op = %v, want 0", got)
	}
}
