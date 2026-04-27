package synth

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
)

// TestQFormatContract_BuildExcitationPitchTermIsQ15 — for a unit
// pitch gain (gpQ14 = 16384 = 1.0 true) and unit pitch sample
// (v_Q0 = 1), the LMult result is 2·1·1 = 2 in Q-encoded form,
// representing 1.0 at Q15 (since LMult auto-shifts left by 1).
//
// Documented: pf.LMult(Q14, Q0) = 2·a·b is at Q15.
func TestQFormatContract_BuildExcitationPitchTermIsQ15(t *testing.T) {
	const gpQ14 int16 = 1 << 14
	const vQ0 int16 = 1
	got := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(vQ0))
	const want fixed.Word32 = 1 << 15 // 1.0 stored as Q15
	if got != want {
		t.Fatalf("LMult(gpQ14=1.0, v=1) = %d, want %d (= 1.0·2^15 stored)",
			got, want)
	}
}

// TestQFormatContract_BuildExcitationCodeTermIsQ26ThenQ15 — for unit
// fixed-codebook gain (gcQ12 = 4096 = 1.0 true) and unit code pulse
// (c_Q13 = 8192 = 1.0 true), LMult yields 2·gc·c = 2·4096·8192 in
// Q-encoded form (Q26-stored value of 1.0 true product). After LShr
// by 11, value is at Q15.
func TestQFormatContract_BuildExcitationCodeTermIsQ26ThenQ15(t *testing.T) {
	const gcQ12 int16 = 1 << 12
	const cQ13 int16 = 1 << 13
	lMultRaw := fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(cQ13))
	const wantQ26 fixed.Word32 = 1 << 26
	if lMultRaw != wantQ26 {
		t.Errorf("LMult(gc=1.0 Q12, c=1.0 Q13) = %d, want %d (Q26)",
			lMultRaw, wantQ26)
	}
	lCodeQ15 := fixed.LShr(lMultRaw, 11)
	const wantQ15 fixed.Word32 = 1 << 15
	if lCodeQ15 != wantQ15 {
		t.Errorf("LShr(Q26, 11) = %d, want %d (Q15)", lCodeQ15, wantQ15)
	}
}

// TestQFormatContract_BuildExcitationSinglePulseProducesGcQ12 — when
// gpQ14 = 0 and v = 0, with c being a single Q13 pulse and gcQ12 a
// known value, u[0] should equal round-to-int(gcQ12 / 4096) =
// round-to-int(true gc).
func TestQFormatContract_BuildExcitationSinglePulseProducesGcQ12(t *testing.T) {
	tests := []struct {
		name  string
		gcQ12 int16
		want  int16
	}{
		{"gc=1.0 (Q12=4096)", 4096, 1},
		{"gc=2.0 (Q12=8192)", 8192, 2},
		{"gc=5.5 (Q12=22528)", 22528, 6},
		{"gc=0.0", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v, c, u [40]int16
			c[0] = 1 << 13
			BuildExcitation(0, tt.gcQ12, &v, &c, &u)
			if u[0] != tt.want {
				t.Errorf("u[0] = %d, want %d (= round(gcQ12/4096))",
					u[0], tt.want)
			}
		})
	}
}

// TestQFormatContract_FilterSubframeAcceptsAOneQ12 — the LP synthesis
// filter expects a[0] = 4096 (= +1.0 Q12) per ITU-T G.729 §4.1.6.
// With a[i]=0 for i≥1 (trivial filter) and u being a unit excitation,
// s should equal u.
func TestQFormatContract_FilterSubframeAcceptsAOneQ12(t *testing.T) {
	var synth Synthesizer
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var u, s [40]int16
	for n := 0; n < 40; n++ {
		u[n] = int16(n + 1)
	}
	synth.Filter(&a, &u, &s)
	for n := 0; n < 40; n++ {
		if s[n] != u[n] {
			t.Errorf("s[%d] = %d, want %d (trivial filter passthrough)",
				n, s[n], u[n])
		}
	}
}
