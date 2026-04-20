package fixed

// Saturate clamps x to the Word16 range.
func Saturate(x Word32) Word16 {
	switch {
	case x > Word32(Max16):
		return Max16
	case x < Word32(Min16):
		return Min16
	default:
		return Word16(x)
	}
}
