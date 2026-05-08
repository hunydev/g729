package fcb

import "testing"

func TestDecode_PulsesOnlyAtBetaZero(t *testing.T) {
	idx := Indices{Positions: 0, Signs: 0x0F}
	var c [40]int16
	Decode(idx, 40, 0, &c)

	want := [40]int16{}
	for _, p := range []int{0, 1, 2, 3} {
		want[p] = PulseAmplitude
	}
	if c != want {
		t.Fatalf("Decode with β=0 should be bare pulses; got c=%v", c)
	}
}

func TestDecode_MixedSignsNoEnhancement(t *testing.T) {
	idx := Indices{Positions: 0x053C, Signs: 0b1001}
	var c [40]int16
	Decode(idx, 40, 0, &c)

	want := map[int]int16{
		20: +PulseAmplitude,
		36: -PulseAmplitude,
		22: -PulseAmplitude,
		8:  +PulseAmplitude,
	}
	for p, w := range want {
		if c[p] != w {
			t.Errorf("c[%d] = %d, want %d", p, c[p], w)
		}
	}
	for n := 0; n < 40; n++ {
		if _, isPulse := want[n]; !isPulse && c[n] != 0 {
			t.Errorf("c[%d] = %d, want 0", n, c[n])
		}
	}
}

func TestDecode_AppliesEnhancementAfterPulses(t *testing.T) {
	idx := Indices{Positions: 0, Signs: 0x01}
	var c [40]int16
	Decode(idx, 20, 8192, &c)

	if c[0] != PulseAmplitude {
		t.Errorf("c[0] = %d, want %d", c[0], PulseAmplitude)
	}
	if diff := c[20] - 4096; diff > 1 || diff < -1 {
		t.Errorf("c[20] = %d, want ≈4096", c[20])
	}
	if diff := c[21] - (-4096); diff > 1 || diff < -1 {
		t.Errorf("c[21] = %d, want ≈-4096", c[21])
	}
}

func TestDecode_MatchesPiecewiseComposition(t *testing.T) {
	idx := Indices{Positions: 0x1234, Signs: 0b1010}
	t2 := 25
	beta := int16(10000)

	var got, want [40]int16
	Decode(idx, t2, beta, &got)

	positions := decodePositions(idx.Positions)
	placePulses(positions, idx.Signs, &want)
	ApplyPitchEnhancement(&want, t2, beta)

	if got != want {
		t.Fatalf("Decode differs from piecewise composition:\n got=%v\nwant=%v", got, want)
	}
}
