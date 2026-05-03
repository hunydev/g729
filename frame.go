package g729

// EncodeFrame is a top-level convenience for callers that prefer a
// function over a method. Delegates to (*Encoder).EncodeFrame.
func EncodeFrame(e *Encoder, pcm []int16, out []byte) error {
	return e.EncodeFrame(pcm, out)
}

// DecodeFrame is a top-level convenience for callers that prefer a
// function over a method. Delegates to (*Decoder).DecodeFrame.
func DecodeFrame(d *Decoder, bits []byte, out []int16) error {
	return d.DecodeFrame(bits, out)
}
