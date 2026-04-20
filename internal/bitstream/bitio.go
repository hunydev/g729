package bitstream

// BitWriter writes bits MSB-first into a caller-owned byte slice. The
// slice must be zero-initialized before use; BitWriter only sets 1
// bits, never clears existing 1 bits. Callers typically zero the
// buffer themselves (or rely on Go's zero-initialization) before Init.
type BitWriter struct {
	buf    []byte
	bitPos int
}

// Init resets the writer to the start of buf. The previous contents
// of buf are left untouched.
func (w *BitWriter) Init(buf []byte) {
	w.buf = buf
	w.bitPos = 0
}

// BitPos returns the number of bits written so far.
func (w *BitWriter) BitPos() int { return w.bitPos }

// Write writes the low n bits of value into the buffer, MSB of those
// n bits first. Bits beyond position len(buf)*8 are dropped silently;
// callers must ensure the buffer is large enough for all writes.
func (w *BitWriter) Write(value uint16, n int) {
	for i := n - 1; i >= 0; i-- {
		if (value>>uint(i))&1 == 1 {
			byteIdx := w.bitPos >> 3
			bitIdx := 7 - (w.bitPos & 7)
			if byteIdx < len(w.buf) {
				w.buf[byteIdx] |= 1 << uint(bitIdx)
			}
		}
		w.bitPos++
	}
}
