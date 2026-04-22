package fixed

// LShl returns a shifted left by n with saturation to Word32. Negative
// n means right shift by -n.
func LShl(a Word32, n Word16) Word32 {
	if n <= 0 {
		if n < -32 {
			n = -32
		}
		return LShr(a, -n)
	}
	if n >= 32 {
		if a > 0 {
			setOverflow()
			return Max32
		}
		if a < 0 {
			setOverflow()
			return Min32
		}
		return 0
	}
	return saturate64(int64(a) << uint(n))
}

// LShr returns a arithmetically shifted right by n. Negative n means
// left shift. For n >= 31 the result is 0 for non-negative a and -1 for
// negative a.
func LShr(a Word32, n Word16) Word32 {
	if n < 0 {
		if n < -32 {
			n = -32
		}
		return LShl(a, -n)
	}
	if n >= 31 {
		if a < 0 {
			return -1
		}
		return 0
	}
	return a >> uint(n)
}

// LShrR returns a shifted right by n with rounding. For n <= 0 equals
// LShl(a, -n). For n > 31 returns 0.
func LShrR(a Word32, n Word16) Word32 {
	if n <= 0 {
		return LShl(a, -n)
	}
	if n > 31 {
		return 0
	}
	out := LShr(a, n)
	if a&(1<<uint(n-1)) != 0 {
		out = LAdd(out, 1)
	}
	return out
}
