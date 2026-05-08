package lsp

import "github.com/hunydev/g729/internal/fixed"

// LSF stability constants per ITU-T G.729 §3.2.4 / §4.1.1, expressed
// in Q13 (π ≈ 25736 in Q13).
//
//   - lsfMinEdge   = 0.005  rad → 41    (ω̂_1 floor, post-predictor)
//   - lsfMinGap    = 0.0391 rad → 320   (post-predictor min adjacent gap)
//   - lsfMaxEdge   = 3.135  rad → 25682 (ω̂_10 ceiling)
//   - lsfRearrJ1   = 0.0012 rad → 10    (pre-predictor pass 1 gap)
//   - lsfRearrJ2   = 0.0006 rad → 5     (pre-predictor pass 2 gap)
const (
	lsfMinEdge int16 = 41
	lsfMinGap  int16 = 320
	lsfMaxEdge int16 = 25682
	lsfRearrJ1 int16 = 10
	lsfRearrJ2 int16 = 5
)

// enforceLSFStability applies the post-predictor stability rules from
// ITU-T G.729 §3.2.4 to ω̂ in place:
//
//  1. order ω̂ in increasing value;
//  2. ω̂_1   ≥ lsfMinEdge;
//  3. ω̂_{i+1} − ω̂_i ≥ lsfMinGap, i = 1..9;
//  4. ω̂_10  ≤ lsfMaxEdge.
//
// If the upper-edge clamp would violate the gap rule, the ceiling is
// back-propagated so the result remains strictly monotone.
//
// The function is O(n²) on n=10 (insertion sort) plus a single linear
// pass; allocations are zero.
func enforceLSFStability(lsf *[10]int16) {
	insertionSort10(lsf)

	if lsf[0] < lsfMinEdge {
		lsf[0] = lsfMinEdge
	}
	for i := 1; i < 10; i++ {
		minNext := fixed.Add(lsf[i-1], lsfMinGap)
		if lsf[i] < minNext {
			lsf[i] = minNext
		}
	}
	if lsf[9] > lsfMaxEdge {
		lsf[9] = lsfMaxEdge
		for i := 9; i > 0; i-- {
			maxPrev := fixed.Sub(lsf[i], lsfMinGap)
			if lsf[i-1] > maxPrev {
				lsf[i-1] = maxPrev
			}
		}
	}
}

// rearrangeAdjacent applies the pre-predictor pair-rearrangement of
// ITU-T G.729 §3.2.4 to the residual l̂ in place. For each adjacent
// pair whose gap is below J, the two values are spread to differ by
// exactly J around their midpoint:
//
//	l̂_{i-1} = (l̂_i + l̂_{i-1} − J) / 2
//	l̂_i     = (l̂_i + l̂_{i-1} + J) / 2
//
// The spec applies this routine twice per frame, with J = 0.0012 then
// J = 0.0006 (lsfRearrJ1 / lsfRearrJ2). One forward pass is sufficient
// because each rewrite leaves the two elements gap-J apart.
func rearrangeAdjacent(lsf *[10]int16, J int16) {
	for i := 1; i < 10; i++ {
		if lsf[i]-lsf[i-1] < J {
			sum := int32(lsf[i]) + int32(lsf[i-1])
			lsf[i-1] = int16((sum - int32(J)) / 2)
			lsf[i] = int16((sum + int32(J)) / 2)
		}
	}
}

// insertionSort10 sorts a 10-element int16 array ascending in place.
// Stable, allocation-free, branch-friendly for nearly-sorted input.
func insertionSort10(a *[10]int16) {
	for i := 1; i < 10; i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}
