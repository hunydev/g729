package fixed

// Round rounds a Word32 to Word16 by adding 0x00008000 (half of the low
// 16 bits) and taking the high half. Saturates.
func Round(x Word32) Word16 {
	return ExtractH(LAdd(x, 0x00008000))
}
