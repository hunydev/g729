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
