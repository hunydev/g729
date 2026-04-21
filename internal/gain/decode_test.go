package gain

import "testing"

func TestDecode_ProducesGainsAndUpdatesState(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[5] = 8192
	idx := Indices{GA: 3, GB: 7}

	gp, gc := d.Decode(idx, &c)

	if gp <= 0 || gp > 20000 {
		t.Errorf("g_p = %d, out of plausible Q14 range [1, 20000]", gp)
	}
	if gc <= 0 {
		t.Errorf("g_c = %d, want positive", gc)
	}
	if !d.initialized {
		t.Errorf("initialized = false after Decode, want true")
	}
	if d.pastErrors[0] == -14336 {
		t.Errorf("pastErrors[0] unchanged from default after Decode")
	}
}

func TestDecode_TwoSubframesStatePropagation(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[0] = 8192

	_, _ = d.Decode(Indices{GA: 0, GB: 0}, &c)
	before := d.pastErrors
	_, _ = d.Decode(Indices{GA: 0, GB: 0}, &c)
	after := d.pastErrors

	if after[0] != before[0] {
		t.Errorf("after[0]=%d, before[0]=%d — same input should give same log-error", after[0], before[0])
	}
	if after[1] != before[0] {
		t.Errorf("after[1]=%d, before[0]=%d — FIFO shift broken", after[1], before[0])
	}
	if after[2] != before[1] {
		t.Errorf("after[2]=%d, before[1]=%d — FIFO shift broken", after[2], before[1])
	}
}

func TestDecode_ResetRestoresZeroValueDeterminism(t *testing.T) {
	var c [40]int16
	c[0] = 8192
	idx := Indices{GA: 2, GB: 5}

	var d1 Decoder
	gp1, gc1 := d1.Decode(idx, &c)

	var d2 Decoder
	_, _ = d2.Decode(idx, &c)
	_, _ = d2.Decode(idx, &c)
	d2.Reset()
	gp2, gc2 := d2.Decode(idx, &c)

	if gp1 != gp2 || gc1 != gc2 {
		t.Errorf("after Reset, outputs differ: (%d, %d) vs (%d, %d)", gp1, gc1, gp2, gc2)
	}
}
