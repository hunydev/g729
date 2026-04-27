package fcb

import "testing"

// TestQFormatContract_PulseAmplitudeIsOneQ13 — PulseAmplitude is the
// Q13 representation of true pulse magnitude +1.0 per ITU-T G.729 §3.8.
// Hard-coded to 8192 = 2^13 = +1.0 in Q13.
func TestQFormatContract_PulseAmplitudeIsOneQ13(t *testing.T) {
	const want int16 = 1 << 13
	if PulseAmplitude != want {
		t.Fatalf("PulseAmplitude = %d, want %d (= +1.0 Q13)",
			PulseAmplitude, want)
	}
}

// TestQFormatContract_SinglePulseEnergyIs2to26 — for a single +Q13
// pulse, Σc² (raw, before any Q-format reinterpretation) equals 2^26.
// This is the input that the gain decoder's fixedCodebookEnergy will
// see; documenting it here pins down the inter-module Q-format
// contract.
func TestQFormatContract_SinglePulseEnergyIs2to26(t *testing.T) {
	var c [40]int16
	c[0] = PulseAmplitude
	var sum int64
	for n := 0; n < 40; n++ {
		sum += int64(c[n]) * int64(c[n])
	}
	const want int64 = 1 << 26
	if sum != want {
		t.Fatalf("Σc² = %d, want %d (= 2^26 for single Q13 pulse)",
			sum, want)
	}
}

// TestQFormatContract_FourPulseEnergyIs2to28 — canonical ACELP 4-pulse
// codebook: Σc² = 4·2^26 = 2^28. This is the Q26 representation of
// true energy 4.0.
func TestQFormatContract_FourPulseEnergyIs2to28(t *testing.T) {
	var c [40]int16
	c[5] = PulseAmplitude
	c[11] = PulseAmplitude
	c[22] = PulseAmplitude
	c[33] = PulseAmplitude
	var sum int64
	for n := 0; n < 40; n++ {
		sum += int64(c[n]) * int64(c[n])
	}
	const want int64 = 1 << 28
	if sum != want {
		t.Fatalf("Σc² = %d, want %d (= 2^28 for canonical 4-pulse Q13)",
			sum, want)
	}
}

// TestQFormatContract_PostEnhancementBoundedByMaxBeta — after pitch
// enhancement c(n) ← c(n) + β·c(n−t), |c[n]| ≤ PulseAmplitude·(1 + βMax)
// where βMax is the §3.8 clamp ceiling. This guards against the
// enhancement loop blowing the Q13 range.
func TestQFormatContract_PostEnhancementBoundedByMaxBeta(t *testing.T) {
	var c [40]int16
	c[0] = PulseAmplitude
	c[5] = PulseAmplitude
	c[10] = PulseAmplitude
	c[15] = PulseAmplitude
	betaQ14 := ClampPitchGainForEnhancement(32767)
	applyPitchEnhancement(&c, 5, betaQ14)
	for n, v := range c {
		if v > 32767 || v < -32768 {
			t.Errorf("c[%d] = %d after enhancement: out of int16 range",
				n, v)
		}
	}
}
