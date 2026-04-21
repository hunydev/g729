package lsp

// Indices are the bit-field values delivered per frame for LSP
// decoding. Values are the raw integer indices after bitstream
// unpacking, not bit-slices.
type Indices struct {
	L0 uint8 // 1 bit  - MA predictor selector (0 or 1)
	L1 uint8 // 7 bits - first-stage VQ index (0..127)
	L2 uint8 // 5 bits - second-stage lower-split index (0..31)
	L3 uint8 // 5 bits - second-stage upper-split index (0..31)
}
