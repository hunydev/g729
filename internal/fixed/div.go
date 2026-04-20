package fixed

// DivS returns num/den in Q15 format. Preconditions:
//
//	num >= 0
//	den >  0
//	num <= den
//
// If any precondition is violated, returns Max16. The exact-equal case
// num == den returns Max16 because 32768 is not representable as Word16.
func DivS(num, den Word16) Word16 {
	if num < 0 || den <= 0 || num > den {
		return Max16
	}
	if num == 0 {
		return 0
	}
	if num == den {
		return Max16
	}

	var q Word16
	n := Word32(num) << 15
	d := Word32(den) << 15
	for i := 0; i < 15; i++ {
		q <<= 1
		n <<= 1
		if n >= d {
			n -= d
			q |= 1
		}
	}
	return q
}
