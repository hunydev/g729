package tables

import "testing"

// ENC-1 invariant: GainMap{1,2} are *bijective* permutations and exact
// inverses of GainImap{1,2} (§3.9.3). This pins both directions —
// GainMap[GainImap[i]] = i (decoder-side check, also covered by the
// older TestGainMap1IsInverseOfImap1) and the encoder-critical
// GainImap[GainMap[i]] = i, which guarantees PackGains followed by
// the decoder's index lookup recovers the original physical entry.
func TestGainMap1AndImap1AreMutualInverses(t *testing.T) {
	for i := 0; i < 8; i++ {
		if got := GainMap1[GainImap1[i]]; got != uint8(i) {
			t.Errorf("GainMap1[GainImap1[%d]] = %d, want %d", i, got, i)
		}
		if got := GainImap1[GainMap1[i]]; got != uint8(i) {
			t.Errorf("GainImap1[GainMap1[%d]] = %d, want %d", i, got, i)
		}
	}
}

func TestGainMap2AndImap2AreMutualInverses(t *testing.T) {
	for i := 0; i < 16; i++ {
		if got := GainMap2[GainImap2[i]]; got != uint8(i) {
			t.Errorf("GainMap2[GainImap2[%d]] = %d, want %d", i, got, i)
		}
		if got := GainImap2[GainMap2[i]]; got != uint8(i) {
			t.Errorf("GainImap2[GainMap2[%d]] = %d, want %d", i, got, i)
		}
	}
}
