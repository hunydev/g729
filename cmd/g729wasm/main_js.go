//go:build js && wasm

package main

import (
	"encoding/binary"
	"fmt"
	"syscall/js"

	"github.com/hunydev/g729"
)

var wasmFuncs []js.Func

func main() {
	roundTripFunc := js.FuncOf(roundTripPCM16)
	decodeFunc := js.FuncOf(decodePayload)
	newStreamFunc := js.FuncOf(newLoopbackStream)
	wasmFuncs = append(wasmFuncs, roundTripFunc, decodeFunc, newStreamFunc)

	api := js.Global().Get("Object").New()
	api.Set("roundTripPCM16", roundTripFunc)
	api.Set("decodePayload", decodeFunc)
	api.Set("newLoopbackStream", newStreamFunc)
	api.Set("sampleRate", g729.SampleRate)
	api.Set("frameSamples", g729.FrameSamples)
	api.Set("frameBytes", g729.FrameBytes)
	js.Global().Set("g729Wasm", api)

	if customEvent := js.Global().Get("CustomEvent"); customEvent.Truthy() && js.Global().Get("dispatchEvent").Type() == js.TypeFunction {
		js.Global().Call("dispatchEvent", customEvent.New("g729wasmready"))
	}
	select {}
}

func roundTripPCM16(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return jsError("roundTripPCM16 expects one Uint8Array argument")
	}
	in, err := jsBytes(args[0])
	if err != nil {
		return jsError(err.Error())
	}
	if len(in)%2 != 0 {
		return jsError(fmt.Sprintf("PCM byte length %d is not even", len(in)))
	}

	samples := len(in) / 2
	frames := (samples + g729.FrameSamples - 1) / g729.FrameSamples
	paddedSamples := frames*g729.FrameSamples - samples

	enc := g729.NewEncoder()
	dec := g729.NewDecoder()
	framePCM := make([]int16, g729.FrameSamples)
	frameBits := make([]byte, g729.FrameBytes)
	frameDecoded := make([]int16, g729.FrameSamples)
	encoded := make([]byte, 0, frames*g729.FrameBytes)
	decoded := make([]byte, 0, frames*g729.FrameSamples*2)

	for frame := 0; frame < frames; frame++ {
		for i := range framePCM {
			sampleIndex := frame*g729.FrameSamples + i
			if sampleIndex >= samples {
				framePCM[i] = 0
				continue
			}
			off := sampleIndex * 2
			framePCM[i] = int16(binary.LittleEndian.Uint16(in[off:]))
		}

		if err := enc.EncodeFrame(framePCM, frameBits); err != nil {
			return jsError(err.Error())
		}
		encoded = append(encoded, frameBits...)

		if err := dec.DecodeFrame(frameBits, frameDecoded); err != nil {
			return jsError(err.Error())
		}
		appendPCM16LE(&decoded, frameDecoded)
	}

	out := js.Global().Get("Object").New()
	out.Set("ok", true)
	out.Set("encoded", bytesToJS(encoded))
	out.Set("decodedPCM16", bytesToJS(decoded))
	out.Set("frames", frames)
	out.Set("inputSamples", samples)
	out.Set("decodedSamples", frames*g729.FrameSamples)
	out.Set("paddedSamples", paddedSamples)
	out.Set("sampleRate", g729.SampleRate)
	return out
}

type loopbackStream struct {
	enc *g729.Encoder
	dec *g729.Decoder

	encoded      []byte
	frameDecoded [g729.FrameSamples]int16

	inputSamples   int
	encodedFrames  int
	decodedSamples int
}

func newLoopbackStream(_ js.Value, _ []js.Value) any {
	stream := &loopbackStream{}
	stream.reset()

	writeFunc := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 {
			return jsError("stream.write expects one Uint8Array argument")
		}
		return stream.write(args[0])
	})
	flushFunc := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		return stream.flush()
	})
	resetFunc := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		stream.reset()
		return js.ValueOf(true)
	})

	wasmFuncs = append(wasmFuncs, writeFunc, flushFunc, resetFunc)

	out := js.Global().Get("Object").New()
	out.Set("write", writeFunc)
	out.Set("flush", flushFunc)
	out.Set("reset", resetFunc)
	return out
}

func (s *loopbackStream) reset() {
	s.encoded = s.encoded[:0]
	s.dec = g729.NewDecoder()
	s.enc = g729.NewStreamingEncoder(s)
	s.inputSamples = 0
	s.encodedFrames = 0
	s.decodedSamples = 0
}

func (s *loopbackStream) Write(p []byte) (int, error) {
	s.encoded = append(s.encoded, p...)
	return len(p), nil
}

func (s *loopbackStream) write(v js.Value) any {
	in, err := jsBytes(v)
	if err != nil {
		return jsError(err.Error())
	}
	if len(in)%2 != 0 {
		return jsError(fmt.Sprintf("PCM byte length %d is not even", len(in)))
	}

	samples := make([]int16, len(in)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(in[i*2:]))
	}

	n, err := g729.Write(s.enc, samples)
	if err != nil {
		return jsError(err.Error())
	}
	s.inputSamples += n

	decoded, frames, err := s.consumeEncoded()
	if err != nil {
		return jsError(err.Error())
	}
	return s.result(decoded, frames)
}

func (s *loopbackStream) flush() any {
	if err := g729.Flush(s.enc); err != nil {
		return jsError(err.Error())
	}
	decoded, frames, err := s.consumeEncoded()
	if err != nil {
		return jsError(err.Error())
	}
	return s.result(decoded, frames)
}

func (s *loopbackStream) consumeEncoded() ([]byte, int, error) {
	if len(s.encoded)%g729.FrameBytes != 0 {
		return nil, 0, fmt.Errorf("internal stream payload length %d is not divisible by %d", len(s.encoded), g729.FrameBytes)
	}

	frames := len(s.encoded) / g729.FrameBytes
	decoded := make([]byte, 0, frames*g729.FrameSamples*2)
	for off := 0; off < len(s.encoded); off += g729.FrameBytes {
		if err := s.dec.DecodeFrame(s.encoded[off:off+g729.FrameBytes], s.frameDecoded[:]); err != nil {
			return nil, 0, err
		}
		appendPCM16LE(&decoded, s.frameDecoded[:])
	}
	s.encoded = s.encoded[:0]
	s.encodedFrames += frames
	s.decodedSamples += frames * g729.FrameSamples
	return decoded, frames, nil
}

func (s *loopbackStream) result(decoded []byte, frames int) js.Value {
	bufferedSamples := s.inputSamples - (s.encodedFrames * g729.FrameSamples)
	if bufferedSamples < 0 {
		bufferedSamples = 0
	}
	out := js.Global().Get("Object").New()
	out.Set("ok", true)
	out.Set("decodedPCM16", bytesToJS(decoded))
	out.Set("frames", frames)
	out.Set("decodedSamples", len(decoded)/2)
	out.Set("totalInputSamples", s.inputSamples)
	out.Set("totalEncodedFrames", s.encodedFrames)
	out.Set("totalDecodedSamples", s.decodedSamples)
	out.Set("bufferedSamples", bufferedSamples)
	out.Set("sampleRate", g729.SampleRate)
	return out
}

func decodePayload(_ js.Value, args []js.Value) any {
	if len(args) != 1 {
		return jsError("decodePayload expects one Uint8Array argument")
	}
	in, err := jsBytes(args[0])
	if err != nil {
		return jsError(err.Error())
	}
	if len(in)%g729.FrameBytes != 0 {
		return jsError(fmt.Sprintf("G.729 payload byte length %d is not divisible by %d", len(in), g729.FrameBytes))
	}

	dec := g729.NewDecoder()
	frameDecoded := make([]int16, g729.FrameSamples)
	decoded := make([]byte, 0, (len(in)/g729.FrameBytes)*g729.FrameSamples*2)
	for off := 0; off < len(in); off += g729.FrameBytes {
		if err := dec.DecodeFrame(in[off:off+g729.FrameBytes], frameDecoded); err != nil {
			return jsError(err.Error())
		}
		appendPCM16LE(&decoded, frameDecoded)
	}

	out := js.Global().Get("Object").New()
	out.Set("ok", true)
	out.Set("decodedPCM16", bytesToJS(decoded))
	out.Set("frames", len(in)/g729.FrameBytes)
	out.Set("decodedSamples", len(decoded)/2)
	out.Set("sampleRate", g729.SampleRate)
	return out
}

func appendPCM16LE(dst *[]byte, samples []int16) {
	var pair [2]byte
	for _, sample := range samples {
		binary.LittleEndian.PutUint16(pair[:], uint16(sample))
		*dst = append(*dst, pair[:]...)
	}
}

func jsBytes(v js.Value) ([]byte, error) {
	if !v.Truthy() || v.Get("byteLength").Type() != js.TypeNumber {
		return nil, fmt.Errorf("argument must be a Uint8Array-like value")
	}
	b := make([]byte, v.Get("byteLength").Int())
	js.CopyBytesToGo(b, v)
	return b, nil
}

func bytesToJS(b []byte) js.Value {
	out := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(out, b)
	return out
}

func jsError(msg string) js.Value {
	out := js.Global().Get("Object").New()
	out.Set("ok", false)
	out.Set("error", msg)
	return out
}
