package closedloop

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/tables"
)

// adaptiveExcLen mirrors fracTestExcLen / refineExcLen used by the
// neighbouring FR-1 / FR-2 unit tests. ≥ PitchMaxInt(143) +
// SubframeLen(40) + Linter(10) slack so a frac = ±1 FIR centred at
// any admissible delay stays inside exc.
const adaptiveExcLen = 256

// TestAdaptiveVector_FracZeroIsIntegerCopy: when frac = 0 the
// adaptive-codebook vector v(n) reduces to a direct read of the
// past-excitation buffer at integer delay intLag — no FIR work,
// no attenuation by b30(0). Per ITU-T G.729 §3.7.1 eq. (40)
// (G729E.txt line 1162), the centre tap of the b30 FIR is the
// implicit unity tap; cf. the AdaptiveCodebook decoder reference
// (internal/pitch/adaptive.go) and Interpolate3 (frac.go) which
// document the same fast path.
//
// Buffer convention (shared with SearchInteger / Interpolate3):
// u(0) is anchored at exc[len(exc) − SubframeLen]; for integer
// delay intLag, v[n] = exc[len(exc) − SubframeLen − intLag + n] for
// n ∈ [0, 40). The trailing SubframeLen samples hold the LP-residual
// extension u(0..39) per §A.3.7 line 2161 for the short-pitch case.
func TestAdaptiveVector_FracZeroIsIntegerCopy(t *testing.T) {
	var exc [adaptiveExcLen]int16
	for i := range exc {
		exc[i] = int16(i - 128) // arbitrary signed pattern
	}
	// intLag ≥ SubframeLen so that the integer read window
	// stays inside the past portion (no caller-side LP-residual
	// extension required); the short-pitch boundary case is
	// exercised separately below with a tail-aligned pre-fill.
	for _, intLag := range []int16{40, 80, 143} {
		var v [SubframeLen]int16
		AdaptiveVector(exc[:], intLag, 0, &v)
		base := adaptiveExcLen - SubframeLen - int(intLag)
		for n := 0; n < SubframeLen; n++ {
			if v[n] != exc[base+n] {
				t.Fatalf("intLag=%d n=%d: v=%d want exc[%d]=%d",
					intLag, n, v[n], base+n, exc[base+n])
			}
		}
	}
}

// TestAdaptiveVector_FracPositiveMatchesInterpolate3: for frac = +1
// every output sample must equal Interpolate3(exc, intLag−n, +1),
// i.e. the algebraic identity v(n) = u(n − intLag + 1/3) per
// §3.7.1 eq. (40) / §A.3.7 eq. A.8 (G729E.txt lines 1162, 2178).
// Mirrors RefineFraction's correlateAtFrac mapping (refine.go).
func TestAdaptiveVector_FracPositiveMatchesInterpolate3(t *testing.T) {
	var exc [adaptiveExcLen]int16
	for i := range exc {
		exc[i] = int16((i*37)%251 - 125)
	}
	for _, intLag := range []int16{25, 60, 100, 143} {
		var v [SubframeLen]int16
		AdaptiveVector(exc[:], intLag, +1, &v)
		for n := 0; n < SubframeLen; n++ {
			want := Interpolate3(exc[:], intLag-int16(n), +1)
			if v[n] != want {
				t.Fatalf("frac=+1 intLag=%d n=%d: got %d want %d",
					intLag, n, v[n], want)
			}
		}
	}
}

// TestAdaptiveVector_FracNegativeMatchesInterpolate3: analog for
// frac = −1 (delay intLag − 1/3). Same spec reference as the
// positive-frac counterpart.
func TestAdaptiveVector_FracNegativeMatchesInterpolate3(t *testing.T) {
	var exc [adaptiveExcLen]int16
	for i := range exc {
		exc[i] = int16((i*53)%241 - 120)
	}
	for _, intLag := range []int16{25, 60, 100, 143} {
		var v [SubframeLen]int16
		AdaptiveVector(exc[:], intLag, -1, &v)
		for n := 0; n < SubframeLen; n++ {
			want := Interpolate3(exc[:], intLag-int16(n), -1)
			if v[n] != want {
				t.Fatalf("frac=-1 intLag=%d n=%d: got %d want %d",
					intLag, n, v[n], want)
			}
		}
	}
}

// TestAdaptiveVector_LPResidualBoundary: exercises the boundary
// case where the chosen lag puts the read window flush against the
// anchor — for intLag = 0, v(n) = u(n) for n ∈ [0, 40), which under
// the new buffer convention maps exactly to
// exc[len − SubframeLen : len], the LP-residual extension region
// the encoder driver pre-fills with the current-subframe LP residual
// r(0..39) to support short-pitch search. Spec: §A.3.7 line 2161
// (LP-residual extension for k < SubframeLen); SearchInteger godoc
// (correlate.go) on caller buffer responsibility.
func TestAdaptiveVector_LPResidualBoundary(t *testing.T) {
	const intLag int16 = 0 // flush-zero boundary: read u(0..39) = residual extension
	var exc [adaptiveExcLen]int16
	// Past excitation history: arbitrary non-zero pattern.
	for i := 0; i < adaptiveExcLen-SubframeLen; i++ {
		exc[i] = int16(i%17 - 8)
	}
	// Pre-fill the LP-residual extension region exc[len−40:len].
	for n := 0; n < SubframeLen; n++ {
		exc[adaptiveExcLen-SubframeLen+n] = int16(1000 + n)
	}

	var v [SubframeLen]int16
	AdaptiveVector(exc[:], intLag, 0, &v)
	for n := 0; n < SubframeLen; n++ {
		want := int16(1000 + n)
		if v[n] != want {
			t.Fatalf("boundary n=%d: v=%d want %d", n, v[n], want)
		}
	}
}

// TestAdaptiveVector_FracPositiveImpulse: golden cross-check using
// a single-sample impulse and the same fixed-point primitives the
// production code uses. With u(−intLag) = 1<<14 and frac = +1, the
// output sample at n = 0 collapses to b30(1)·u(−intLag), giving
// Round(LMult(PitchInterpFIR[1], 1<<14)) per §3.7.1 eq. (40).
func TestAdaptiveVector_FracPositiveImpulse(t *testing.T) {
	const intLag int16 = 40
	var exc [adaptiveExcLen]int16
	exc[adaptiveExcLen-SubframeLen-int(intLag)] = 1 << 14

	var v [SubframeLen]int16
	AdaptiveVector(exc[:], intLag, +1, &v)

	want := fixed.Round(fixed.LMult(tables.PitchInterpFIR[1], 1<<14))
	if v[0] != want {
		t.Fatalf("impulse frac=+1 v[0]: got %d want %d", v[0], want)
	}
}

// TestAdaptiveVector_ZeroAlloc: I4 alloc gate. AdaptiveVector is
// called per subframe twice per frame on the encoder hot path and
// must allocate nothing. Mirrors the alloc gates protecting FR-2
// (RefineFraction) and CL-1 (SearchInteger).
func TestAdaptiveVector_ZeroAlloc(t *testing.T) {
	var exc [adaptiveExcLen]int16
	for i := range exc {
		exc[i] = int16(i - 128)
	}
	var v [SubframeLen]int16
	for _, frac := range []int8{-1, 0, +1} {
		f := frac
		allocs := testing.AllocsPerRun(64, func() {
			AdaptiveVector(exc[:], 80, f, &v)
		})
		if allocs != 0 {
			t.Errorf("frac=%d AllocsPerRun = %v, want 0", f, allocs)
		}
	}
}
