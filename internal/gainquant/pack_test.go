package gainquant_test

import (
	"testing"

	"github.com/hunydev/g729/internal/gainquant"
	"github.com/hunydev/g729/internal/tables"
)

// ENC-1 RED for §3.9.3 forward index mapping. SearchConjugate (GQ-2)
// returns the physical codebook indices (ga, gb) into GainGBK1 /
// GainGBK2; §3.9.3 specifies that the *transmitted* indices are the
// permuted GA / GB so that single-bit channel errors map to nearby
// codebook entries. PackGains must apply tables.GainMap1 / GainMap2.
func TestPackGains_AppliesForwardMaps(t *testing.T) {
	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			ga3, gb4 := gainquant.PackGains(ga, gb)
			if ga3 != tables.GainMap1[ga] {
				t.Errorf("PackGains(%d,%d).ga3 = %d, want GainMap1[%d]=%d",
					ga, gb, ga3, ga, tables.GainMap1[ga])
			}
			if gb4 != tables.GainMap2[gb] {
				t.Errorf("PackGains(%d,%d).gb4 = %d, want GainMap2[%d]=%d",
					ga, gb, gb4, gb, tables.GainMap2[gb])
			}
			if ga3 >= 8 {
				t.Errorf("ga3=%d out of 3-bit range", ga3)
			}
			if gb4 >= 16 {
				t.Errorf("gb4=%d out of 4-bit range", gb4)
			}
		}
	}
}

// Round-trip: PackGains then apply the decoder's inverse map should
// recover the physical indices.
func TestPackGains_RoundTripViaImap(t *testing.T) {
	for ga := uint8(0); ga < 8; ga++ {
		for gb := uint8(0); gb < 16; gb++ {
			ga3, gb4 := gainquant.PackGains(ga, gb)
			if got := tables.GainImap1[ga3]; got != ga {
				t.Errorf("ga=%d: GainImap1[PackGains.ga3]=%d, want %d", ga, got, ga)
			}
			if got := tables.GainImap2[gb4]; got != gb {
				t.Errorf("gb=%d: GainImap2[PackGains.gb4]=%d, want %d", gb, got, gb)
			}
		}
	}
}

func TestPackGains_NoAlloc(t *testing.T) {
	if got := testing.AllocsPerRun(128, func() {
		_, _ = gainquant.PackGains(3, 11)
	}); got != 0 {
		t.Fatalf("PackGains allocations/op = %v, want 0", got)
	}
}
