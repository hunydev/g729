package gain

import "testing"

func TestIndicesZeroValue(t *testing.T) {
	var idx Indices
	if idx.GA != 0 || idx.GB != 0 {
		t.Fatalf("zero-value Indices = %+v, want all zero", idx)
	}
}

func TestDecoderZeroValueIsValid(t *testing.T) {
	var d Decoder
	for i, v := range d.pastErrors {
		if v != 0 {
			t.Errorf("pastErrors[%d] = %d, want 0", i, v)
		}
	}
	if d.initialized {
		t.Errorf("initialized = true on zero value, want false")
	}
}

func TestDecoderResetClearsState(t *testing.T) {
	d := Decoder{
		pastErrors:  [4]int16{1, 2, 3, 4},
		initialized: true,
	}
	d.Reset()
	for i, v := range d.pastErrors {
		if v != 0 {
			t.Errorf("after Reset, pastErrors[%d] = %d, want 0", i, v)
		}
	}
	if d.initialized {
		t.Errorf("after Reset, initialized = true, want false")
	}
}
