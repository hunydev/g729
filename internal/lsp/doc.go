// Package lsp implements ITU-T G.729 + Annex A sections 3.2.4 and
// 4.1.1-4.1.2 line-spectral-pair decoding. Given the 18 LSP bits from
// one frame (L0 / L1 / L2 / L3), it produces the two 11-coefficient
// LP filter vectors that the synthesis filter uses for the first and
// second subframe.
//
// The package owns the cross-frame state required for MA prediction
// and LSP interpolation. It is not safe for concurrent use; each
// decoder channel needs its own Decoder instance.
package lsp
