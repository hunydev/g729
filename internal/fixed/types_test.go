package fixed

import "testing"

func TestWord16Range(t *testing.T) {
	if Max16 != 32767 {
		t.Errorf("Max16 = %d, want 32767", Max16)
	}
	if Min16 != -32768 {
		t.Errorf("Min16 = %d, want -32768", Min16)
	}
}

func TestWord32Range(t *testing.T) {
	if Max32 != 2147483647 {
		t.Errorf("Max32 = %d, want 2147483647", Max32)
	}
	if Min32 != -2147483648 {
		t.Errorf("Min32 = %d, want -2147483648", Min32)
	}
}

func TestTypeSizes(t *testing.T) {
	var w16 Word16
	var w32 Word32
	if _, ok := any(w16).(int16); !ok {
		t.Errorf("Word16 is not int16 compatible: %T", w16)
	}
	if _, ok := any(w32).(int32); !ok {
		t.Errorf("Word32 is not int32 compatible: %T", w32)
	}
}
