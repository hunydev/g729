// Package acelp implements the G.729 Annex A fast algebraic codebook
// search: 4 pulses with sign, distributed over interleaved tracks T0..T3,
// 17-bit codeword (positions 13 bits + signs 4 bits). The search uses
// the §A.3 depth-first focused variant of the §3.8 full search.
//
// Phase 2-0 ships only the type skeleton; real arithmetic is wired
// in Phase 2d.
package acelp
