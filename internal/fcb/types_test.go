package fcb

import "testing"

func TestIndicesZeroValue(t *testing.T) {
	var idx Indices
	if idx.Positions != 0 || idx.Signs != 0 {
		t.Fatalf("zero-value Indices = %+v, want all zero", idx)
	}
}

func TestPulseAmplitudeEndpoints(t *testing.T) {
	if PulseAmplitude != 8191 {
		t.Fatalf("PulseAmplitude = %d, want 8191", PulseAmplitude)
	}
	if negativePulseAmplitude != -8192 {
		t.Fatalf("negativePulseAmplitude = %d, want -8192", negativePulseAmplitude)
	}
}
