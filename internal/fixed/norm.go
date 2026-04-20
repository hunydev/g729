package fixed

// NormS returns the number of left shifts needed to normalize x so that
// further shifting would saturate. Returns 0 for x = 0.
func NormS(x Word16) Word16 {
	if x == 0 {
		return 0
	}
	if x == -1 {
		return 15
	}
	if x < 0 {
		x = ^x
	}
	var n Word16
	for x < 0x4000 {
		x <<= 1
		n++
	}
	return n
}

// NormL returns the number of left shifts to normalize a Word32.
func NormL(x Word32) Word16 {
	if x == 0 {
		return 0
	}
	if x == -1 {
		return 31
	}
	if x < 0 {
		x = ^x
	}
	var n Word16
	for x < 0x40000000 {
		x <<= 1
		n++
	}
	return n
}
