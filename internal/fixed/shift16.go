package fixed

// Shl returns a shifted left by n positions with saturation. A negative
// n is interpreted as right-shift by -n (per ITU spec). Large shifts
// saturate to Max16 or Min16 according to the sign of a.
func Shl(a, n Word16) Word16 {
	if n <= 0 {
		if n < -16 {
			n = -16
		}
		return Shr(a, -n)
	}
	if n >= 16 {
		if a > 0 {
			return Max16
		}
		if a < 0 {
			return Min16
		}
		return 0
	}
	return Saturate(Word32(a) << uint(n))
}

// Shr returns a arithmetically shifted right by n positions. A negative
// n is interpreted as left-shift by -n. For n >= 15 the result is 0 if
// a is non-negative and -1 if a is negative.
func Shr(a, n Word16) Word16 {
	if n < 0 {
		if n < -16 {
			n = -16
		}
		return Shl(a, -n)
	}
	if n >= 15 {
		if a < 0 {
			return -1
		}
		return 0
	}
	return a >> uint(n)
}

// ShrR returns a shifted right by n with rounding. For n <= 0 this is
// equivalent to Shl(a, -n). For n > 15 the result is 0.
func ShrR(a, n Word16) Word16 {
	if n <= 0 {
		return Shl(a, -n)
	}
	if n > 15 {
		return 0
	}
	out := Shr(a, n)
	if a&(1<<uint(n-1)) != 0 {
		out = Add(out, 1)
	}
	return out
}
