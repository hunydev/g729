// Package lpc implements G.729 LPC (Linear Predictive Coding) analysis:
// windowed autocorrelation with the §3.2.1 30 ms asymmetric Hamming
// window, lag windowing for spectral smoothing (§3.2.2), and the
// Levinson-Durbin recursion (§3.2.3) producing the order-10 LPC
// coefficients a[1..10] in Q-format.
//
// Phase 2-0 ships only the type skeleton; real arithmetic is wired
// in Phase 2a.
package lpc
