// Package fixed implements the ITU-T G.191 basic operations required
// by the G.729 speech-codec specification.
//
// # Philosophy
//
// All arithmetic saturates to Word16 (int16) or Word32 (int32) instead
// of wrapping. The package is the numerical foundation of the codec:
// any deviation from ITU semantics here will surface as bit-level
// errors in the encoder output and fail the bit-exact acceptance tests.
//
// Go's built-in +, -, * operators wrap on signed overflow. They must
// never be used for codec arithmetic. Always use the primitives here.
//
// # ITU name mapping
//
// The ITU spec uses C-style names; Go code uses CamelCase. The mapping:
//
//	saturate       -> Saturate
//	add, sub       -> Add, Sub
//	negate, abs_s  -> Negate, AbsS
//	L_add, L_sub   -> LAdd, LSub
//	L_negate       -> LNegate
//	L_abs          -> LAbs
//	extract_h      -> ExtractH
//	extract_l      -> ExtractL
//	L_deposit_h    -> LDepositH
//	L_deposit_l    -> LDepositL
//	shl, shr       -> Shl, Shr
//	shr_r          -> ShrR
//	L_shl, L_shr   -> LShl, LShr
//	L_shr_r        -> LShrR
//	L_mult         -> LMult
//	L_mac, L_msu   -> LMac, LMsu
//	mult, mult_r   -> Mult, MultR
//	round          -> Round
//	norm_s, norm_l -> NormS, NormL
//	div_s          -> DivS
//
// # Example
//
// Compute a scaled inner product of two short vectors in Q15 arithmetic:
//
//	var acc Word32
//	for i := range a {
//	    acc = LMac(acc, a[i], b[i]) // acc += 2 * a[i] * b[i], saturating
//	}
//	result := Round(acc) // 32-bit accumulator to 16-bit with rounding
//
// # No allocation
//
// Every function in this package returns a primitive value and allocates
// nothing. The zero-allocation contract is enforced by
// TestNoAllocationInPrimitives.
package fixed
