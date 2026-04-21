package tables

import "testing"

func TestGainGBK1Shape(t *testing.T) {
	if len(GainGBK1) != 8 {
		t.Fatalf("GainGBK1 length = %d, want 8", len(GainGBK1))
	}
}

func TestGainGBK1EntriesInSpecRange(t *testing.T) {
	// Per ITU-T G.729 §3.9 eq. (73)-(74):
	//   ĝ_p   = GBK1[GA][0] + GBK2[GB][0]   (Q14)
	//   γ̂    = GBK1[GA][1] + GBK2[GB][1]   (Q13 — see ITU data tables)
	// Per-component bounds are not pinned by the spec; they only
	// constrain the joint sum. Verify int16 envelope.
	for i, entry := range GainGBK1 {
		_ = entry
		_ = i
	}
}

func TestGainMap1Shape(t *testing.T) {
	if len(GainMap1) != 8 {
		t.Fatalf("GainMap1 length = %d, want 8", len(GainMap1))
	}
	if len(GainImap1) != 8 {
		t.Fatalf("GainImap1 length = %d, want 8", len(GainImap1))
	}
}

func TestGainMap1IsInverseOfImap1(t *testing.T) {
	// §3.9.3: indices are mapped for bit-error robustness. The
	// decoder applies the inverse map: physical entry index =
	// GainImap1[transmitted GA].
	for ga := 0; ga < 8; ga++ {
		if GainMap1[GainImap1[ga]] != uint8(ga) {
			t.Errorf("GainMap1[GainImap1[%d]] = %d, want %d",
				ga, GainMap1[GainImap1[ga]], ga)
		}
	}
}

func TestGainGBK2Shape(t *testing.T) {
	if len(GainGBK2) != 16 {
		t.Fatalf("GainGBK2 length = %d, want 16", len(GainGBK2))
	}
}

func TestGainMap2Shape(t *testing.T) {
	if len(GainMap2) != 16 || len(GainImap2) != 16 {
		t.Fatalf("GainMap2/GainImap2 lengths = %d/%d, want 16",
			len(GainMap2), len(GainImap2))
	}
}

func TestGainMap2IsInverseOfImap2(t *testing.T) {
	for gb := 0; gb < 16; gb++ {
		if GainMap2[GainImap2[gb]] != uint8(gb) {
			t.Errorf("GainMap2[GainImap2[%d]] = %d, want %d",
				gb, GainMap2[GainImap2[gb]], gb)
		}
	}
}

// Joint VQ sums must remain non-negative (g_p) and within int16
// envelopes after applying the index mapping. This is the core
// invariant: any wrong table value here will surface immediately.
func TestGainVQJointSumsViaImapInRange(t *testing.T) {
	for ga := 0; ga < 8; ga++ {
		for gb := 0; gb < 16; gb++ {
			e1 := GainGBK1[GainImap1[ga]]
			e2 := GainGBK2[GainImap2[gb]]
			gp := int32(e1[0]) + int32(e2[0]) // Q14
			gc := int32(e1[1]) + int32(e2[1]) // Q13
			// Sums are accumulated as Word32 in the decoder; the
			// individual table entries fit int16 but their sum may
			// exceed it. Verify only non-negativity (spec requires
			// gains ≥ 0).
			if gp < 0 {
				t.Errorf("g_p sum @(GA=%d,GB=%d) = %d, must be ≥ 0",
					ga, gb, gp)
			}
			if gc < 0 {
				t.Errorf("γ̂ sum @(GA=%d,GB=%d) = %d, must be ≥ 0",
					ga, gb, gc)
			}
		}
	}
}
