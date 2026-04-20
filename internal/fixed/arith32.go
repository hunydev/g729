package fixed

// saturate64 clamps a 64-bit value to the Word32 range.
func saturate64(x int64) Word32 {
	switch {
	case x > int64(Max32):
		return Max32
	case x < int64(Min32):
		return Min32
	default:
		return Word32(x)
	}
}

// LAdd returns a + b with saturation to Word32.
func LAdd(a, b Word32) Word32 {
	return saturate64(int64(a) + int64(b))
}

// LSub returns a - b with saturation to Word32.
func LSub(a, b Word32) Word32 {
	return saturate64(int64(a) - int64(b))
}

// LNegate returns -a with saturation (Min32 -> Max32).
func LNegate(a Word32) Word32 {
	if a == Min32 {
		return Max32
	}
	return -a
}

// LAbs returns |a| with saturation (Min32 -> Max32).
func LAbs(a Word32) Word32 {
	if a == Min32 {
		return Max32
	}
	if a < 0 {
		return -a
	}
	return a
}
