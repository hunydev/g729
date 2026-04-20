package fixed

// LMult returns 2*a*b saturated to Word32. The only saturation case is
// a = b = Min16, where the mathematical result 2^31 overflows to Max32.
func LMult(a, b Word16) Word32 {
	if a == Min16 && b == Min16 {
		return Max32
	}
	return Word32(a) * Word32(b) << 1
}
