// Package fixed implements the ITU-T G.191 basic operations used
// throughout the G.729 codec specification.
//
// All arithmetic saturates to the 16-bit or 32-bit signed range instead
// of wrapping. Identifier naming maps one-to-one to the ITU spec names
// with CamelCase applied (e.g. L_mult -> LMult, shr_r -> ShrR). See
// doc.go for the full mapping.
//
// No function in this package allocates.
package fixed

// Word16 is a 16-bit signed fixed-point value, corresponding to the
// ITU-T type Word16 (C int16).
type Word16 = int16

// Word32 is a 32-bit signed fixed-point value, corresponding to the
// ITU-T type Word32 (C int32).
type Word32 = int32

// Saturation bounds.
const (
	Max16 Word16 = 32767
	Min16 Word16 = -32768
	Max32 Word32 = 2147483647
	Min32 Word32 = -2147483648
)
