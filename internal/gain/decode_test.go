package gain

import (
	"math"
	"testing"
)

func TestDecode_ProducesGainsAndUpdatesState(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[5] = 8192
	idx := Indices{GA: 3, GB: 7}

	gp, mant, exp := d.Decode(idx, &c)
	gc := LegacyGcQ12FromMantExp(mant, exp)

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

	_, _, _ = d.Decode(Indices{GA: 0, GB: 0}, &c)
	before := d.pastErrors
	_, _, _ = d.Decode(Indices{GA: 0, GB: 0}, &c)
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
	gp1, gc1Mant, gc1Exp := d1.Decode(idx, &c)
	gc1 := LegacyGcQ12FromMantExp(gc1Mant, gc1Exp)

	var d2 Decoder
	_, _, _ = d2.Decode(idx, &c)
	_, _, _ = d2.Decode(idx, &c)
	d2.Reset()
	gp2, gc2Mant, gc2Exp := d2.Decode(idx, &c)
	gc2 := LegacyGcQ12FromMantExp(gc2Mant, gc2Exp)

	if gp1 != gp2 || gc1 != gc2 {
		t.Errorf("after Reset, outputs differ: (%d, %d) vs (%d, %d)", gp1, gc1, gp2, gc2)
	}
}

func TestTenLog10_40Q10_MatchesSpecDerivation(t *testing.T) {
want := int16(math.Round(10 * math.Log10(40) * 1024))
if tenLog10_40Q10 != want {
t.Fatalf("tenLog10_40Q10 = %d; want %d (= round(10·log10(40)·2^10))", tenLog10_40Q10, want)
}
}

func TestDbPerLog2Q13_MatchesSpecDerivation(t *testing.T) {
want := int16(math.Round(10 * math.Log10(2) * 8192))
if dbPerLog2Q13 != want {
t.Fatalf("dbPerLog2Q13 = %d; want %d (= round(10·log10(2)·2^13))", dbPerLog2Q13, want)
}
}

func TestInvDbScaleQ15_MatchesSpecDerivation(t *testing.T) {
want := int16(math.Round(1 / (20 * math.Log10(2)) * 32768))
if invDbScaleQ15 != want {
t.Fatalf("invDbScaleQ15 = %d; want %d (= round(1/(20·log10(2))·2^15))", invDbScaleQ15, want)
}
}

func TestDbPerLog2Q10_MatchesSpecDerivation(t *testing.T) {
want := int16(math.Round(20 * math.Log10(2) * 1024))
if dbPerLog2Q10 != want {
t.Fatalf("dbPerLog2Q10 = %d; want %d (= round(20·log10(2)·2^10))", dbPerLog2Q10, want)
}
}
