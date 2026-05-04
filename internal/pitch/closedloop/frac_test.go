package closedloop

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// fracTestExcLen is the past-excitation buffer length used in the
// FR-1 unit tests. ≥ PitchMaxInt(143) + Linter(10) + slack.
const fracTestExcLen = 256

// TestInterpolate3_FracZeroIsIntegerCopy: when frac = 0 the
// fractional-delay primitive must return the past-excitation sample
// at integer delay intLag, i.e. u[len(u) − intLag], with no FIR
// computation (the spec equation (40) center tap b30(0) = 1.0 is
// implicit and the integer path is a direct copy).
//
// Spec: ITU-T G.729 §3.7.1 eq. (40), G729E.txt line 1162.
func TestInterpolate3_FracZeroIsIntegerCopy(t *testing.T) {
	var u [fracTestExcLen]int16
	for i := range u {
		u[i] = int16(i - 128) // arbitrary signed pattern
	}
	for _, intLag := range []int16{20, 40, 80, 143} {
		got := Interpolate3(u[:], intLag, 0)
		want := u[fracTestExcLen-SubframeLen-int(intLag)]
		if got != want {
			t.Errorf("Interpolate3(intLag=%d, frac=0) = %d, want %d",
				intLag, got, want)
		}
	}
}

// TestInterpolate3_FracPositiveImpulse: with a single-sample impulse
// placed exactly at u[N − intLag] and frac = +1, eq. (40) collapses
// to one term b30(t + 3·0)·u(−intLag) with t = 1, where in the
// internal/tables.PitchInterpFIR storage convention (shared with the
// decoder, see internal/tables/pitch_interp.go) b30(1) lives at
// PitchInterpFIR[1] = 25207.
//
// The asserted golden value is computed from the same fixed-point
// primitives the production code uses (LMult doubles the Q15·Q14
// product into Q30; Round adds 0x8000 and extracts the high half):
//
//	val   = 1 << 14
//	want  = Round(LMult(b30(1), val)) = 12604
//
// Spec: §3.7.1 eq. (40), G729E.txt line 1162.
func TestInterpolate3_FracPositiveImpulse(t *testing.T) {
	const intLag = int16(40)
	var u [fracTestExcLen]int16
	u[fracTestExcLen-SubframeLen-int(intLag)] = 1 << 14

	got := Interpolate3(u[:], intLag, +1)
	want := fixed.Round(fixed.LMult(tables.PitchInterpFIR[1], 1<<14))
	if got != want {
		t.Errorf("Interpolate3 impulse frac=+1 at offset 0: got %d, want %d",
			got, want)
	}
	// Pin the literal too — guards against silent table drift.
	if want != 12604 {
		t.Fatalf("b30(1) tap drift: Round(LMult(fir[1], 2^14))=%d, want 12604", want)
	}
}

// TestInterpolate3_FracNegativeImpulse: frac = −1 maps to (k =
// intLag − 1, t = 2) per eq. (40). With an impulse placed at
// u[N − intLag] = u[N − (k+1)], only the backward-sum term i = 1
// contributes through the tap b30(t + 3·1) = b30(5), stored at
// PitchInterpFIR[5] = −5850.
//
//	want = Round(LMult(b30(5), val)) = −2925
//
// Spec: §3.7.1 eq. (40), G729E.txt line 1162.
func TestInterpolate3_FracNegativeImpulse(t *testing.T) {
	const intLag = int16(40)
	var u [fracTestExcLen]int16
	u[fracTestExcLen-SubframeLen-int(intLag)] = 1 << 14

	got := Interpolate3(u[:], intLag, -1)
	want := fixed.Round(fixed.LMult(tables.PitchInterpFIR[5], 1<<14))
	if got != want {
		t.Errorf("Interpolate3 impulse frac=-1: got %d, want %d", got, want)
	}
	if want != -2925 {
		t.Fatalf("b30(5) tap drift: Round(LMult(fir[5], 2^14))=%d, want -2925", want)
	}
}

// TestInterpolate3_PalindromeSymmetry: b30 is even-symmetric around
// its center tap, so eq. (40) admits a closed-form symmetry between
// the +1/3 and −1/3 fractional positions. Algebraically, swapping
// the two half-sums in eq. (40) and reversing the input sequence
// yields:
//
//	Interpolate3(u, K, +1) == Interpolate3(reverse(u), N − K + 3, −1)
//
// where N = len(u). For a palindrome (reverse(u) == u) this becomes
//
//	Interpolate3(u, K, +1) == Interpolate3(u, N − K + 3, −1)
//
// which is the property the spec's symmetric-kernel construction
// guarantees and which the test exercises.
//
// Spec: §3.7.1 eq. (40), G729E.txt lines 1162–1167 (b30 symmetry
// statement: "truncated at ±29 and padded with zeros at ±30").
func TestInterpolate3_PalindromeSymmetry(t *testing.T) {
	const N = fracTestExcLen
	var u [N]int16
	// Symmetric ramp around the midpoint.
	for i := 0; i < N/2; i++ {
		v := int16(((i * 37) & 0x3FFF) - 0x2000) // bounded pseudo-random
		u[i] = v
		u[N-1-i] = v
	}

	for _, K := range []int{40, 60, 100} {
		// New buffer convention: base = (N − SubframeLen) − K.
		// The symmetry K ↔ K' satisfies (N − SubframeLen) − K' =
		// ((N − SubframeLen) − K) + 3 ⇒ K' = K − 3 by canceling
		// the anchor shift. But that places K' < K which would be
		// degenerate; the well-defined symmetry that actually holds
		// is the original derivation written in absolute buffer
		// indices: with base+ = anchor − K and base− = anchor − K',
		// we need base− = (N − 1) − (base+ + something)... — more
		// simply, re-derive: the +1/−1 sample sums swap under index
		// reflection i → (N − 1) − i, which sends position p →
		// (N − 1) − p. So base+ and base− are mirror images iff
		// base− = (N − 1) − base+ − 3, hence
		// K' = anchor − ((N − 1) − (anchor − K) − 3) =
		//      2·anchor − N + 1 + 3 − K = 2·(N−SubframeLen) − N + 4 − K =
		//      N − 2·SubframeLen + 4 − K.
		mirrorK := int16(N - 2*SubframeLen + 3 - K)
		gotPlus := Interpolate3(u[:], int16(K), +1)
		gotMinus := Interpolate3(u[:], mirrorK, -1)
		if gotPlus != gotMinus {
			t.Errorf("symmetry break: Interpolate3(palindrome, K=%d, +1) = %d, "+
				"Interpolate3(palindrome, mirrorK=%d, -1) = %d",
				K, gotPlus, mirrorK, gotMinus)
		}
	}
}

// TestInterpolate3_NoAlloc enforces I4: the fractional-delay
// primitive must run with zero heap allocations on the encoder hot
// path (it is invoked once per (k, frac) candidate during FR-2
// refinement and 40 times per subframe during VP-1 vector build).
func TestInterpolate3_NoAlloc(t *testing.T) {
	var u [fracTestExcLen]int16
	for i := range u {
		u[i] = int16(i)
	}
	var sink int16
	avg := testing.AllocsPerRun(200, func() {
		sink = Interpolate3(u[:], 60, +1)
		sink ^= Interpolate3(u[:], 60, -1)
		sink ^= Interpolate3(u[:], 60, 0)
	})
	if avg != 0 {
		t.Fatalf("Interpolate3 alloc/op = %v, want 0", avg)
	}
	_ = sink
}

// Compile-time guard: ensure the FIR table size we depend on
// hasn't drifted out from under the FR-1 implementation.
var _ = [1]struct{}{}[31-len(tables.PitchInterpFIR)]
