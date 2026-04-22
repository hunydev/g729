package fixed

// overflow is the sticky overflow flag set by LAdd, LSub, LMac, LMsu,
// LShl, and Saturate whenever their result would exceed the Word32 or
// Word16 representable range and had to be clamped. It is NOT cleared
// automatically; callers must call ClearOverflow before a critical
// section and check Overflow after.
//
// This mirrors ITU-T G.191 BASOP's "Overflow" global in spirit, and
// lets synth §3.10 detect saturation in the synthesis-filter LMsu chain
// without re-computing the accumulator in int64.
//
// The flag is package-global. Decoder is single-threaded per stream
// (see internal/decoder/doc.go), so no locking is required.
var overflow bool

// ClearOverflow clears the sticky overflow flag.
func ClearOverflow() {
	overflow = false
}

// Overflow reports whether any saturating fixed-point operation has
// triggered saturation since the last ClearOverflow call.
func Overflow() bool {
	return overflow
}

// setOverflow marks the flag. Not exported — only saturating ops set it.
func setOverflow() {
	overflow = true
}
