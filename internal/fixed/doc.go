// Package fixed — ITU-T G.191 basic operations in Go.
//
// Name mapping (ITU spec -> Go):
//
//   saturate       -> Saturate
//   add, sub       -> Add, Sub
//   negate, abs_s  -> Negate, AbsS
//   L_add, L_sub   -> LAdd, LSub
//   L_negate       -> LNegate
//   L_abs          -> LAbs
//   extract_h      -> ExtractH
//   extract_l      -> ExtractL
//   L_deposit_h    -> LDepositH
//   L_deposit_l    -> LDepositL
//   shl, shr       -> Shl, Shr
//   shr_r          -> ShrR
//   L_shl, L_shr   -> LShl, LShr
//   L_shr_r        -> LShrR
//   L_mult         -> LMult
//   L_mac, L_msu   -> LMac, LMsu
//   mult, mult_r   -> Mult, MultR
//   round          -> Round
//   norm_s, norm_l -> NormS, NormL
//   div_s          -> DivS
package fixed
