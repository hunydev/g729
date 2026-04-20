package fixed

// ExtractH returns the high 16 bits of x (arithmetic).
func ExtractH(x Word32) Word16 {
	return Word16(x >> 16)
}

// ExtractL returns the low 16 bits of x, reinterpreted as Word16.
func ExtractL(x Word32) Word16 {
	return Word16(x & 0xFFFF)
}

// LDepositH returns x placed in the high 16 bits of a Word32. The low
// 16 bits are zero.
func LDepositH(x Word16) Word32 {
	return Word32(x) << 16
}

// LDepositL sign-extends x into Word32.
func LDepositL(x Word16) Word32 {
	return Word32(x)
}
