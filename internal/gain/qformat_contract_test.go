package gain

import (
	"math"
	"testing"

	"github.com/hunydev/g729/internal/fixed"
)

// TestQFormatContract_FixedCodebookEnergyIsQ26 — fixedCodebookEnergy
// returns Σ c[n]² as a Word32. For c at Q13, each squared term is
// Q26; the sum is therefore Q26. This contract is the foundation of
// Phase 1j's gain Q-format diagnosis.
func TestQFormatContract_FixedCodebookEnergyIsQ26(t *testing.T) {
	tests := []struct {
		name  string
		setup func(c *[40]int16)
		want  fixed.Word32
	}{
		{"single pulse +1.0 Q13", func(c *[40]int16) { c[0] = 8192 }, 1 << 26},
		{"four pulses canonical", func(c *[40]int16) {
			c[5] = 8192
			c[11] = 8192
			c[22] = 8192
			c[33] = 8192
		}, 1 << 28},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c [40]int16
			tt.setup(&c)
			got := fixedCodebookEnergy(&c)
			if got != tt.want {
				t.Errorf("got=%d want=%d (= Σc²·2^26-style accumulation)",
					got, tt.want)
			}
		})
	}
}

// TestQFormatContract_Log2FixedTreatsInputAsQ0 — log2Fixed returns
// log2(x)·1024 (Q10) treating x as a Q0 integer. So log2Fixed(2^k)
// = k·1024. Caller is responsible for adjusting if its input has
// a non-zero Q-shift.
func TestQFormatContract_Log2FixedTreatsInputAsQ0(t *testing.T) {
	tests := []struct {
		name string
		x    fixed.Word32
		want fixed.Word32
	}{
		{"x=1", 1, 0},
		{"x=2", 2, 1024},
		{"x=2^10", 1 << 10, 10 * 1024},
		{"x=2^26", 1 << 26, 26 * 1024},
		{"x=2^28", 1 << 28, 28 * 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := log2Fixed(tt.x)
			diff := int32(got) - int32(tt.want)
			if diff < -2 || diff > 2 {
				t.Errorf("got=%d want=%d (Δ=%d, max ±2 LSB Q10)",
					got, tt.want, diff)
			}
		})
	}
}

// TestQFormatContract_Pow2FixedReturnsQ0 — pow2Fixed(input Q10) returns
// 2^(input/1024) as Q0. So pow2Fixed(0) = 1, pow2Fixed(1024) = 2,
// pow2Fixed(10*1024) = 1024.
func TestQFormatContract_Pow2FixedReturnsQ0(t *testing.T) {
	tests := []struct {
		name string
		x    fixed.Word32
		want fixed.Word32
	}{
		{"2^0", 0, 1},
		{"2^1", 1024, 2},
		{"2^10", 10 * 1024, 1024},
		{"2^14", 14 * 1024, 1 << 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pow2Fixed(tt.x)
			rel := math.Abs(float64(int32(got)-int32(tt.want))) /
				math.Max(float64(tt.want), 1)
			if rel > 0.01 {
				t.Errorf("got=%d want=%d (rel err=%.4f, max 1%%)",
					got, tt.want, rel)
			}
		})
	}
}

// TestQFormatContract_LogDomainConstants — verify the four magic
// numbers in decode.go match their physical identities. These are
// pure compile-time invariants; if a future refactor changes them,
// this test catches the drift.
func TestQFormatContract_LogDomainConstants(t *testing.T) {
	wantDbPerLog2Q13 := int(math.Round(10 * math.Log10(2) * (1 << 13)))
	if dbPerLog2Q13 != wantDbPerLog2Q13 {
		t.Errorf("dbPerLog2Q13 = %d, want %d", dbPerLog2Q13, wantDbPerLog2Q13)
	}
	wantTenLog10_40Q10 := int(math.Round(10 * math.Log10(40) * (1 << 10)))
	if tenLog10_40Q10 != wantTenLog10_40Q10 {
		t.Errorf("tenLog10_40Q10 = %d, want %d",
			tenLog10_40Q10, wantTenLog10_40Q10)
	}
	wantInvDbScaleQ15 := int(math.Round(1.0 / (20 * math.Log10(2)) * (1 << 15)))
	if invDbScaleQ15 != wantInvDbScaleQ15 {
		t.Errorf("invDbScaleQ15 = %d, want %d",
			invDbScaleQ15, wantInvDbScaleQ15)
	}
	wantDbPerLog2Q10 := int(math.Round(20 * math.Log10(2) * (1 << 10)))
	if dbPerLog2Q10 != wantDbPerLog2Q10 {
		t.Errorf("dbPerLog2Q10 = %d, want %d", dbPerLog2Q10, wantDbPerLog2Q10)
	}
}

// TestQFormatContract_PastErrorsDefaultIsMinus14dBQ10 — initial value
// of MA-predictor history per ITU-T G.729 §3.9.1 / Table 6.
func TestQFormatContract_PastErrorsDefaultIsMinus14dBQ10(t *testing.T) {
	const wantDbQ10 int16 = -14 * 1024
	if pastErrorsDefault != wantDbQ10 {
		t.Fatalf("pastErrorsDefault = %d, want %d (= −14 dB Q10)",
			pastErrorsDefault, wantDbQ10)
	}
}
