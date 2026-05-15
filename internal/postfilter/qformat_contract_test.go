package postfilter

import "testing"

// TestQFormatContract_GammaConstantsAreQ15 — bandwidth-expansion
// constants γ_n ≈ 0.55, γ_d ≈ 0.70 in Q15 per ITU-T G.729 §A.4.2.
func TestQFormatContract_GammaConstantsAreQ15(t *testing.T) {
	tests := []struct {
		name string
		got  int16
		want float64
	}{
		{"gammaNumQ15 (γ_n ≈ 0.55)", gammaNumQ15, 0.55},
		{"gammaDenQ15 (γ_d ≈ 0.70)", gammaDenQ15, 0.70},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotF := float64(tt.got) / 32768.0
			if gotF < tt.want-0.005 || gotF > tt.want+0.005 {
				t.Errorf("got=%.4f want=%.4f (Q15 raw=%d)",
					gotF, tt.want, tt.got)
			}
		})
	}
}

// TestQFormatContract_IsqrtQ14ReturnsQ14 — isqrtQ14(x at Q28) returns
// √x at Q14. So isqrtQ14(2^28) = 2^14 (= 1.0 Q14).
func TestQFormatContract_IsqrtQ14ReturnsQ14(t *testing.T) {
	tests := []struct {
		name string
		xQ28 int64
		want int16
	}{
		{"√1 (Q28)", 1 << 28, 1 << 14},
		{"√0.25 (Q28)", 1 << 26, 1 << 13},
		{"√0", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isqrtQ14(tt.xQ28)
			diff := int(got) - int(tt.want)
			if diff < -1 || diff > 1 {
				t.Errorf("got=%d want=%d (Δ=%d)", got, tt.want, diff)
			}
		})
	}
}

// TestQFormatContract_AGCAlphaIsQ15 — Annex A α = 0.9 at Q15 = 29491.
// ITU-T G.729 Annex A §A.4.2.4.
func TestQFormatContract_AGCAlphaIsQ15(t *testing.T) {
	const wantQ15 int64 = 29491
	const want float64 = 0.9
	if agcAlphaQ15 != wantQ15 {
		t.Fatalf("agcAlphaQ15 = %d, want %d", agcAlphaQ15, wantQ15)
	}
	gotF := float64(agcAlphaQ15) / 32768.0
	if gotF < want-0.001 || gotF > want+0.001 {
		t.Fatalf("alphaQ15 represents %.4f, want %.4f", gotF, want)
	}
}

// TestQFormatContract_AnnexATiltGammaConstants verifies the simplified
// Annex A tilt-compensation branch: γ_t = 0.8 if k1' < 0, else γ_t = 0.
func TestQFormatContract_AnnexATiltGammaConstants(t *testing.T) {
	if gammaTiltNegativeK1Q14 != 13107 {
		t.Fatalf("gammaTiltNegativeK1Q14 = %d, want 13107", gammaTiltNegativeK1Q14)
	}
	if gammaTiltNonNegativeK1Q14 != 0 {
		t.Fatalf("gammaTiltNonNegativeK1Q14 = %d, want 0", gammaTiltNonNegativeK1Q14)
	}
}

// TestQFormatContract_AGCStartsFromUnityQ24 verifies the AGC carry state
// begins at unity and then smooths toward the subframe target.
func TestQFormatContract_AGCStartsFromUnityQ24(t *testing.T) {
	var pf Postfilter
	const gTargetQ14 int16 = 1 << 14
	var sTilt, sPf [subframeLen]int16
	for n := range sTilt {
		sTilt[n] = 100
	}
	pf.applyAGC(&sTilt, gTargetQ14, &sPf)
	if !pf.initialized {
		t.Fatal("applyAGC did not flip the initialized flag")
	}
	const wantQ24 int32 = int32(gTargetQ14) << 10
	if pf.agcGainPrev < wantQ24/2 {
		t.Errorf("agcGainPrev = %d (Q24), expected ~%d (unity target)",
			pf.agcGainPrev, wantQ24)
	}
}
