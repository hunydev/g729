package g729

// EncodeFrame is a top-level convenience for callers that prefer a
// function over a method. Delegates to (*Encoder).EncodeFrame; see that
// method for the input/output length contract and error semantics.
func EncodeFrame(e *Encoder, pcm []int16, out []byte) error {
	return e.EncodeFrame(pcm, out)
}

// DecodeFrame is a top-level convenience for callers that prefer a
// function over a method. Delegates to (*Decoder).DecodeFrame; see that
// method for the input/output length contract and error semantics.
func DecodeFrame(d *Decoder, bits []byte, out []int16) error {
	return d.DecodeFrame(bits, out)
}

// Write is a top-level convenience for callers that prefer a function over a
// method. Delegates to (*Encoder).Write; see that method for streaming
// buffering, frame-boundary, and error semantics.
func Write(e *Encoder, samples []int16) (int, error) {
	return e.Write(samples)
}

// Flush is a top-level convenience for callers that prefer a function over a
// method. Delegates to (*Encoder).Flush; see that method for trailing partial
// frame zero-padding and error semantics.
func Flush(e *Encoder) error {
	return e.Flush()
}
