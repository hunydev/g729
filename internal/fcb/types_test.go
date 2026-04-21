package fcb

import "testing"

func TestIndicesZeroValue(t *testing.T) {
	var idx Indices
	if idx.Positions != 0 || idx.Signs != 0 {
		t.Fatalf("zero-value Indices = %+v, want all zero", idx)
	}
}

func TestPulseAmplitudeIsOneQ13(t *testing.T) {
	if PulseAmplitude != 8192 {
		t.Fatalf("PulseAmplitude = %d, want 8192 (= +1.0 Q13)", PulseAmplitude)
	}
}
