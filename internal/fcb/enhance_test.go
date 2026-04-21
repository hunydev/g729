package fcb

import "testing"

func TestClampPitchGainForEnhancement(t *testing.T) {
	const (
		lowerQ14 = 3277
		upperQ14 = 13107
	)
	cases := []struct {
		name  string
		input int16
		want  int16
	}{
		{"below lower bound (zero)", 0, lowerQ14},
		{"at lower bound", lowerQ14, lowerQ14},
		{"just above lower bound", lowerQ14 + 1, lowerQ14 + 1},
		{"mid-range 0.5", 8192, 8192},
		{"just below upper bound", upperQ14 - 1, upperQ14 - 1},
		{"at upper bound", upperQ14, upperQ14},
		{"above upper bound (1.0 Q14)", 16384, upperQ14},
		{"max int16", 32767, upperQ14},
		{"negative clamps to lower", -1000, lowerQ14},
		{"most negative int16", -32768, lowerQ14},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClampPitchGainForEnhancement(tc.input)
			if got != tc.want {
				t.Errorf("ClampPitchGainForEnhancement(%d) = %d, want %d",
					tc.input, got, tc.want)
			}
		})
	}
}
