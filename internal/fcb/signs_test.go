package fcb

import "testing"

func TestPlacePulses_AllPositive(t *testing.T) {
	var c [40]int16
	positions := [4]int{0, 1, 2, 3}
	placePulses(positions, 0x0F, &c)
	for _, p := range positions {
		if c[p] != PulseAmplitude {
			t.Errorf("c[%d] = %d, want +%d", p, c[p], PulseAmplitude)
		}
	}
	for n := 0; n < 40; n++ {
		isPulse := false
		for _, p := range positions {
			if n == p {
				isPulse = true
				break
			}
		}
		if !isPulse && c[n] != 0 {
			t.Errorf("c[%d] = %d, want 0 (non-pulse position)", n, c[n])
		}
	}
}

func TestPlacePulses_AllNegative(t *testing.T) {
	var c [40]int16
	positions := [4]int{5, 6, 7, 8}
	placePulses(positions, 0x00, &c)
	for _, p := range positions {
		if c[p] != -PulseAmplitude {
			t.Errorf("c[%d] = %d, want -%d", p, c[p], PulseAmplitude)
		}
	}
}

func TestPlacePulses_MixedSignsMSBFirst(t *testing.T) {
	var c [40]int16
	positions := [4]int{10, 15, 20, 25}
	placePulses(positions, 0b1010, &c)

	want := map[int]int16{
		10: +PulseAmplitude,
		15: -PulseAmplitude,
		20: +PulseAmplitude,
		25: -PulseAmplitude,
	}
	for p, w := range want {
		if c[p] != w {
			t.Errorf("c[%d] = %d, want %d", p, c[p], w)
		}
	}
}

func TestPlacePulses_ClearsPriorContent(t *testing.T) {
	var c [40]int16
	for i := range c {
		c[i] = 123
	}
	positions := [4]int{0, 1, 2, 3}
	placePulses(positions, 0x0F, &c)
	for n := 0; n < 40; n++ {
		isPulse := false
		for _, p := range positions {
			if n == p {
				isPulse = true
				break
			}
		}
		if !isPulse && c[n] != 0 {
			t.Errorf("c[%d] = %d, want 0 (dirty non-pulse position not cleared)", n, c[n])
		}
	}
}
