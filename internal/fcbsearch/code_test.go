package fcbsearch_test

import (
	"testing"

	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/fcbsearch"
)

// CB-4 RED tests for §3.8 eq. 45 + 46 + 47 (G729E.txt lines 1201–1241):
//
//	c(n) = Σ_{i=0..3} sign_i · δ(n − m_i)                        (eq. 45)
//	c'(n) = c(n) + β · c'(n − T)        for n = T..39            (eq. 46)
//	β = clamp(ĝ_p^(m−1), 0.2, 0.8)                               (eq. 47)
//
// The encoder maintains pulse signs in the §3.8.1 sign-decomposition
// form (signs[40] ∈ {−1,+1}, indexed by absolute pulse position) — the
// same convention CB-3 SignsFromD produces. PulseAmplitude is +1.0 in
// Q13 (= 8192). Harmonic enhancement filter is in-place (IIR), so for
// T < 40 the c[n−T] term reads the post-updated value.

const pAmp = fcb.PulseAmplitude // 8192, Q13

// TestBuildCode_PulsesNoEnhancement covers eq. 45 only (T=40 ⇒ enhancement
// is bypassed per eq. 48 / §3.8.1 boundary).
func TestBuildCode_PulsesNoEnhancement(t *testing.T) {
	positions := [4]int8{0, 6, 12, 23}
	var signs [40]int16
	signs[0] = +1
	signs[6] = -1
	signs[12] = +1
	signs[23] = +1

	var c [40]int16
	fcbsearch.BuildCode(&positions, &signs, 40, 8192, &c)

	want := [40]int16{}
	want[0] = +pAmp
	want[6] = -pAmp
	want[12] = +pAmp
	want[23] = +pAmp
	if c != want {
		t.Fatalf("c mismatch (T=40, no enhancement)\n got=%v\nwant=%v", c, want)
	}
}

// TestBuildCode_EnhancementT20 covers eq. 46 with T=20 and β=0.5 (Q14
// 8192, in-range so eq. 47 clamp is a pass-through). For T=20 the
// loop n=20..39 reads c[n−20] from the unmodified head, so the IIR
// reduces to a single-shot copy with β scaling.
func TestBuildCode_EnhancementT20(t *testing.T) {
	positions := [4]int8{0, 6, 12, 23}
	var signs [40]int16
	signs[0] = +1
	signs[6] = -1
	signs[12] = +1
	signs[23] = +1

	var c [40]int16
	fcbsearch.BuildCode(&positions, &signs, 20, 8192, &c)

	// Pulses at original positions retained.
	if c[0] != +pAmp || c[6] != -pAmp || c[12] != +pAmp || c[23] != +pAmp {
		t.Fatalf("base pulses corrupted: c[0..23]=%d,%d,%d,%d", c[0], c[6], c[12], c[23])
	}
	// Eq. 46 contributions for n in 20..39 (T=20). c[n−20]=±pAmp at
	// {0,6,12} → contributions land at {20,26,32}; c[n−20]=0 elsewhere.
	// δ = round(LShl(LMult(8192, ±8192), 1)) = ±round(2^29 / 2^16) = ±8192/2 = ±4096.
	if c[20] != +4096 {
		t.Fatalf("c[20] = %d, want +4096 (β·c[0])", c[20])
	}
	if c[26] != -4096 {
		t.Fatalf("c[26] = %d, want -4096 (β·c[6])", c[26])
	}
	if c[32] != +4096 {
		t.Fatalf("c[32] = %d, want +4096 (β·c[12])", c[32])
	}
	// Spot-check zeros where neither a pulse nor an enhancement
	// contribution lands: n=21,22,24,25 (no pulse, c[n−20]=0).
	for _, n := range [...]int{21, 22, 24, 25} {
		if c[n] != 0 {
			t.Fatalf("c[%d] = %d, want 0", n, c[n])
		}
	}
}

// TestBuildCode_BetaCeilingClamp verifies eq. 47 upper bound: β is
// clamped to 0.8 (Q14 13107) when prevGpQ14 exceeds it. With β=0.8 and
// c[0]=+8192, the n=20 update yields round(13107·8192·2·2 / 2^16) = 6554.
func TestBuildCode_BetaCeilingClamp(t *testing.T) {
	positions := [4]int8{0, 1, 2, 3}
	var signs [40]int16
	signs[0] = +1
	signs[1] = +1
	signs[2] = +1
	signs[3] = +1

	var c [40]int16
	fcbsearch.BuildCode(&positions, &signs, 20, 16000, &c) // 16000 > 13107

	if c[20] != 6554 {
		t.Fatalf("c[20] = %d, want 6554 (β clamped to 0.8 ceiling)", c[20])
	}
}

// TestBuildCode_BetaFloorClamp verifies eq. 47 lower bound: β is
// clamped to 0.2 (Q14 3277) when prevGpQ14 is below it. With β=0.2
// and c[0]=+8192, n=20 → ExtractH((3277·8192·2·2) + 0x8000) = 1639.
func TestBuildCode_BetaFloorClamp(t *testing.T) {
	positions := [4]int8{0, 1, 2, 3}
	var signs [40]int16
	signs[0] = +1
	signs[1] = +1
	signs[2] = +1
	signs[3] = +1

	var c [40]int16
	fcbsearch.BuildCode(&positions, &signs, 20, 1000, &c) // 1000 < 3277

	if c[20] != 1639 {
		t.Fatalf("c[20] = %d, want 1639 (β clamped to 0.2 floor)", c[20])
	}
}

// TestBuildCode_LagAtSubframeBoundary covers the eq. 48 boundary:
// T == 40 means the enhancement loop is empty (and T > 40 likewise).
// Identical output to placement-only.
func TestBuildCode_LagAtSubframeBoundary(t *testing.T) {
	positions := [4]int8{2, 7, 17, 27}
	var signs [40]int16
	signs[2] = +1
	signs[7] = +1
	signs[17] = -1
	signs[27] = +1

	var cWith, cWithout [40]int16
	fcbsearch.BuildCode(&positions, &signs, 40, 8192, &cWith)
	// "Without" via T=40 again (or a value ≥ 40). The same call is
	// the reference: the test asserts no other index changed.
	fcbsearch.BuildCode(&positions, &signs, 100, 8192, &cWithout)
	if cWith != cWithout {
		t.Fatalf("T=40 vs T=100 differ; both should bypass enhancement\n with=%v\nwithout=%v", cWith, cWithout)
	}
	want := [40]int16{}
	want[2] = +pAmp
	want[7] = +pAmp
	want[17] = -pAmp
	want[27] = +pAmp
	if cWith != want {
		t.Fatalf("c mismatch\n got=%v\nwant=%v", cWith, want)
	}
}

// TestBuildCode_OverlappingIIR verifies the eq. 46 in-place IIR
// behaviour: when T < 40 and a pulse position p plus T equals another
// pulse position q, the enhancement contribution at q reads the
// post-updated c[q−T] (cascaded). With T=6 and pulses at {0,6,12,23}
// signed {+,−,+,+}:
//   - n=6:  c'(6)  = c(6) + β·c'(0)  = −8192 + 0.5·8192  = −4096
//   - n=12: c'(12) = c(12)+ β·c'(6)  = +8192 + 0.5·(−4096) = +6144
//   - n=18: c'(18) = 0     + β·c'(12) = 0    + 0.5·6144   = +3072
func TestBuildCode_OverlappingIIR(t *testing.T) {
	positions := [4]int8{0, 6, 12, 23}
	var signs [40]int16
	signs[0] = +1
	signs[6] = -1
	signs[12] = +1
	signs[23] = +1

	var c [40]int16
	fcbsearch.BuildCode(&positions, &signs, 6, 8192, &c)

	if c[6] != -4096 {
		t.Fatalf("c[6] = %d, want -4096 (IIR step 1)", c[6])
	}
	if c[12] != 6144 {
		t.Fatalf("c[12] = %d, want 6144 (IIR step 2; cascaded from c[6])", c[12])
	}
	if c[18] != 3072 {
		t.Fatalf("c[18] = %d, want 3072 (IIR step 3; cascaded from c[12])", c[18])
	}
}

// TestBuildCode_AllocsZero gates I3 / I4: caller-owned outputs, no
// hidden allocations.
func TestBuildCode_AllocsZero(t *testing.T) {
	positions := [4]int8{0, 6, 12, 23}
	var signs [40]int16
	signs[0] = +1
	signs[6] = -1
	signs[12] = +1
	signs[23] = +1
	var c [40]int16

	if got := testing.AllocsPerRun(128, func() {
		fcbsearch.BuildCode(&positions, &signs, 20, 8192, &c)
	}); got != 0 {
		t.Fatalf("BuildCode allocations/op = %v, want 0", got)
	}
}
