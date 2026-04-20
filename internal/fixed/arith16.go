package fixed

// Add returns a + b with saturation to Word16.
func Add(a, b Word16) Word16 {
	return Saturate(Word32(a) + Word32(b))
}

// Sub returns a - b with saturation to Word16.
func Sub(a, b Word16) Word16 {
	return Saturate(Word32(a) - Word32(b))
}

// Negate returns -a with saturation (Min16 -> Max16).
func Negate(a Word16) Word16 {
	if a == Min16 {
		return Max16
	}
	return -a
}

// AbsS returns |a| with saturation (Min16 -> Max16).
func AbsS(a Word16) Word16 {
	if a == Min16 {
		return Max16
	}
	if a < 0 {
		return -a
	}
	return a
}
