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
	wasmFuncs = append(wasmFuncs, roundTripFunc, decodeFunc)

	api := js.Global().Get("Object").New()
	api.Set("roundTripPCM16", roundTripFunc)
	api.Set("decodePayload", decodeFunc)
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
