package fixed

// LMult returns 2*a*b saturated to Word32. The only saturation case is
// a = b = Min16, where the mathematical result 2^31 overflows to Max32.
func LMult(a, b Word16) Word32 {
	if a == Min16 && b == Min16 {
		setOverflow()
		return Max32
	}
	return Word32(a) * Word32(b) << 1
}

// LMac returns acc + 2*a*b with saturation at both the multiplication
// and addition stages.
func LMac(acc Word32, a, b Word16) Word32 {
	return LAdd(acc, LMult(a, b))
}

// LMsu returns acc - 2*a*b with saturation at both stages.
func LMsu(acc Word32, a, b Word16) Word32 {
	return LSub(acc, LMult(a, b))
}

// Mult returns (a*b) >> 15 saturated to Word16. Models a fractional
// multiply in Q15 format.
func Mult(a, b Word16) Word16 {
	if a == Min16 && b == Min16 {
		return Max16
	}
	prod := Word32(a) * Word32(b)
	return Word16(prod >> 15)
}

// MultR returns ((a*b) + 0x4000) >> 15 saturated to Word16. Fractional
// multiply with rounding.
func MultR(a, b Word16) Word16 {
	if a == Min16 && b == Min16 {
		return Max16
	}
	prod := Word32(a)*Word32(b) + 0x4000
	return Saturate(prod >> 15)
}
