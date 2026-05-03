// Package fcbsearch implements ITU-T G.729 Annex A §A.3.8 fixed-codebook
// (algebraic ACELP) search support: the adjusted target signal x'(n) of
// §3.8.1 eq. 50, the backward-filtered correlation d(n) of §3.8.1 eq. 52,
// the sign decomposition of §3.8.1 eq. 56, the φ′ matrix and the
// depth-first focused search per §A.3.8.1.
//
// All routines are pure (write only through caller-supplied output
// buffers) and zero-allocation per I3/I4.
package fcbsearch

// SubframeLen is the G.729 subframe length in samples.
const SubframeLen = 40
