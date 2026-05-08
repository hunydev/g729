package closedloop

import (
	"testing"

	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

// fracTestExcLen is the past-excitation buffer length used in the
// FR-1 unit tests. ≥ PitchMaxInt(143) + Linter(10) + slack.
const fracTestExcLen = 256

// TestInterpolate3_FracZeroIsIntegerCopy: when frac = 0 the
// fractional-delay primitive must return the past-excitation sample at integer
// delay intLag, i.e. u[len(u) − intLag], with no FIR computation. The stored
// b30(0) interpolation coefficient is deliberately not used for this exact
// integer-delay helper path.
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

// TestInterpolate3_FracPositiveImpulse: frac = +1 means a pitch delay
// intLag + 1/3, so Interpolate3 evaluates u(−intLag − 1/3). With a
// single-sample impulse at u(−intLag − 1), eq. (40) collapses to the
// b30(2) tap.
//
// The asserted golden value is computed from the same fixed-point
// primitives the production code uses (LMult doubles the Q15·Q14
// product into Q30; Round adds 0x8000 and extracts the high half):
//
//	val   = 1 << 14
//	want  = Round(LMult(b30(2), val)) = 7351
//
// Spec: §3.7.1 eq. (40), G729E.txt line 1162.
func TestInterpolate3_FracPositiveImpulse(t *testing.T) {
	const intLag = int16(40)
	var u [fracTestExcLen]int16
	u[fracTestExcLen-SubframeLen-int(intLag)-1] = 1 << 14

	got := Interpolate3(u[:], intLag, +1)
	want := fixed.Round(fixed.LMult(tables.PitchInterpFIR[2], 1<<14))
	if got != want {
		t.Errorf("Interpolate3 impulse frac=+1 at offset 0: got %d, want %d",
			got, want)
	}
	// Pin the literal too — guards against silent table drift.
	if want != 7351 {
		t.Fatalf("b30(2) tap drift: Round(LMult(fir[2], 2^14))=%d, want 7351", want)
	}
}

// TestInterpolate3_FracNegativeImpulse: frac = −1 means a pitch delay
// intLag − 1/3, so Interpolate3 evaluates u(−intLag + 1/3). With an
// impulse placed at u(−intLag), only the backward-sum i=0 term
// contributes through b30(1).
//
//	want = Round(LMult(b30(1), val)) = 12604
//
// Spec: §3.7.1 eq. (40), G729E.txt line 1162.
func TestInterpolate3_FracNegativeImpulse(t *testing.T) {
	const intLag = int16(40)
	var u [fracTestExcLen]int16
	u[fracTestExcLen-SubframeLen-int(intLag)] = 1 << 14

	got := Interpolate3(u[:], intLag, -1)
	want := fixed.Round(fixed.LMult(tables.PitchInterpFIR[1], 1<<14))
	if got != want {
		t.Errorf("Interpolate3 impulse frac=-1: got %d, want %d", got, want)
	}
	if want != 12604 {
		t.Fatalf("b30(1) tap drift: Round(LMult(fir[1], 2^14))=%d, want 12604", want)
	}
}

// TestInterpolate3_FractionDelayDirection uses a monotonic ramp to pin the
// sign convention: frac=-1 is a smaller pitch delay than the integer case, so
// it reads a later source point and must be larger than frac=0; frac=+1 is a
// larger pitch delay, so it reads an earlier source point and must be smaller.
func TestInterpolate3_FractionDelayDirection(t *testing.T) {
	var u [fracTestExcLen]int16
	for i := range u {
		u[i] = int16(i*100 - 20000)
	}

	for _, K := range []int16{40, 60, 100} {
		neg := Interpolate3(u[:], K, -1)
		zero := Interpolate3(u[:], K, 0)
		pos := Interpolate3(u[:], K, +1)
		if !(neg > zero && zero > pos) {
			t.Fatalf("K=%d: got frac(-1,0,+1)=(%d,%d,%d), want descending delay direction neg > zero > pos",
				K, neg, zero, pos)
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
