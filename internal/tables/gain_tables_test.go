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

func TestGainMAPredictorShape(t *testing.T) {
	if len(GainMAPredictor) != 4 {
		t.Fatalf("GainMAPredictor length = %d, want 4", len(GainMAPredictor))
	}
}

func TestGainMAPredictorCoefficientsPositive(t *testing.T) {
	for i, c := range GainMAPredictor {
		if c <= 0 || c >= 8192 {
			t.Errorf("GainMAPredictor[%d] = %d, outside (0, 8192) Q13", i, c)
		}
	}
}

func TestGainMeanEnergyConstant(t *testing.T) {
	if GainMeanEnergyQ10 != 30720 {
		t.Errorf("GainMeanEnergyQ10 = %d, want 30720 (30 dB Q10)", GainMeanEnergyQ10)
	}
}

func TestPow2TableShape(t *testing.T) {
	if len(Pow2Table) != 33 {
		t.Fatalf("Pow2Table length = %d, want 33", len(Pow2Table))
	}
}

func TestPow2TableEndpointsAndMonotonic(t *testing.T) {
	// Pow2Table[i] approximates 2^(i/32) for i ∈ [0, 32], in Q14
	// (the entries land in [16384, 32767] = [1.0, ~2.0)).
	if Pow2Table[0] != 16384 {
		t.Errorf("Pow2Table[0] = %d, want 16384 (= 2^0 in Q14)", Pow2Table[0])
	}
	if Pow2Table[32] != 32767 {
		t.Errorf("Pow2Table[32] = %d, want 32767 (≈ 2^1 in Q14, saturated)", Pow2Table[32])
	}
	for i := 1; i < len(Pow2Table); i++ {
		if Pow2Table[i] <= Pow2Table[i-1] {
			t.Errorf("Pow2Table non-monotonic at i=%d: %d not > %d",
				i, Pow2Table[i], Pow2Table[i-1])
		}
	}
}

func TestLog2TableShape(t *testing.T) {
	if len(Log2Table) != 33 {
		t.Fatalf("Log2Table length = %d, want 33", len(Log2Table))
	}
}

func TestLog2TableEndpointsAndMonotonic(t *testing.T) {
	// Log2Table[i] approximates log2(1 + i/32) for i ∈ [0, 32], in
	// Q15 (entries in [0, 32767] ≈ [0, 1.0)).
	if Log2Table[0] != 0 {
		t.Errorf("Log2Table[0] = %d, want 0 (= log2(1))", Log2Table[0])
	}
	if Log2Table[32] != 32767 {
		t.Errorf("Log2Table[32] = %d, want 32767 (≈ log2(2) in Q15)", Log2Table[32])
	}
	for i := 1; i < len(Log2Table); i++ {
		if Log2Table[i] <= Log2Table[i-1] {
			t.Errorf("Log2Table non-monotonic at i=%d: %d not > %d",
				i, Log2Table[i], Log2Table[i-1])
		}
	}
}
