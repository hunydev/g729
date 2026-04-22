package fixed

import "testing"

func TestClearOverflow_IsReadBack(t *testing.T) {
	setOverflow()
	if !Overflow() {
		t.Fatal("setOverflow then Overflow() should report true")
	}
	ClearOverflow()
	if Overflow() {
		t.Fatal("after ClearOverflow, Overflow() must be false")
	}
}

func TestLAdd_Saturation_SetsOverflow(t *testing.T) {
	ClearOverflow()
	_ = LAdd(Word32(0x7FFFFFFF), Word32(1))
	if !Overflow() {
		t.Fatal("LAdd saturating to MAX_WORD32 must set overflow flag")
	}
}

func TestLSub_Saturation_SetsOverflow(t *testing.T) {
	ClearOverflow()
	_ = LSub(Word32(-2147483647-1), Word32(1))
	if !Overflow() {
		t.Fatal("LSub saturating to MIN_WORD32 must set overflow flag")
	}
}

func TestLAdd_NoSaturation_DoesNotSetOverflow(t *testing.T) {
	ClearOverflow()
	_ = LAdd(Word32(100), Word32(200))
	if Overflow() {
		t.Fatal("LAdd on in-range inputs must not set overflow flag")
	}
}

func TestLShl_Saturation_SetsOverflow(t *testing.T) {
	ClearOverflow()
	_ = LShl(Word32(0x10000000), Word16(4))
	if !Overflow() {
		t.Fatal("LShl saturating must set overflow flag")
	}
}

func TestSaturate_Clamp_SetsOverflow(t *testing.T) {
	ClearOverflow()
	_ = Saturate(Word32(40000))
	if !Overflow() {
		t.Fatal("Saturate clamping above Word16 max must set overflow flag")
	}
}

func TestLMac_Saturation_SetsOverflow(t *testing.T) {
	ClearOverflow()
	_ = LMac(Word32(0x7FFFFFFF), Word16(32767), Word16(1))
	if !Overflow() {
		t.Fatal("LMac saturating must set overflow flag")
	}
}

func TestLMsu_Saturation_SetsOverflow(t *testing.T) {
	ClearOverflow()
	_ = LMsu(Word32(-2147483647-1), Word16(32767), Word16(1))
	if !Overflow() {
		t.Fatal("LMsu saturating must set overflow flag")
	}
}
