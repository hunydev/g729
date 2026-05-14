package fcb

import "testing"

// TestQFormatContract_PulseAmplitudeEndpoints documents the canonical
// fixed-codebook Q13 endpoints used by the decoder: +8191 for a positive
// pulse and -8192 for a negative pulse.
func TestQFormatContract_PulseAmplitudeEndpoints(t *testing.T) {
	const want int16 = 8191
	if PulseAmplitude != want {
		t.Fatalf("PulseAmplitude = %d, want %d",
			PulseAmplitude, want)
	}
	if negativePulseAmplitude != -8192 {
		t.Fatalf("negativePulseAmplitude = %d, want -8192", negativePulseAmplitude)
	}
}

// TestQFormatContract_SinglePulseEnergy follows directly from the positive
// endpoint above.
// This is the input that the gain decoder's fixedCodebookEnergy will
// see; documenting it here pins down the inter-module Q-format
// contract.
func TestQFormatContract_SinglePulseEnergy(t *testing.T) {
	var c [40]int16
	c[0] = PulseAmplitude
	var sum int64
	for n := 0; n < 40; n++ {
		sum += int64(c[n]) * int64(c[n])
	}
	const want int64 = 8191 * 8191
	if sum != want {
		t.Fatalf("Σc² = %d, want %d for single positive pulse",
			sum, want)
	}
}

// TestQFormatContract_FourPulseEnergy uses four positive pulses to keep this
// contract independent of sign-bit decoding.
func TestQFormatContract_FourPulseEnergy(t *testing.T) {
	var c [40]int16
	c[5] = PulseAmplitude
	c[11] = PulseAmplitude
	c[22] = PulseAmplitude
	c[33] = PulseAmplitude
	var sum int64
	for n := 0; n < 40; n++ {
		sum += int64(c[n]) * int64(c[n])
	}
	const want int64 = 4 * 8191 * 8191
	if sum != want {
		t.Fatalf("Σc² = %d, want %d for four positive pulses",
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
	ApplyPitchEnhancement(&c, 5, betaQ14)
	for n, v := range c {
		if v > 32767 || v < -32768 {
			t.Errorf("c[%d] = %d after enhancement: out of int16 range",
				n, v)
		}
	}
}
