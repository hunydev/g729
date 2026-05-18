package g729

import (
	"bytes"
	"errors"
	"testing"
)

func TestEncodeFrame_TopLevelDelegates(t *testing.T) {
	e := NewEncoder()
	pcm := make([]int16, FrameSamples)
	var out [FrameBytes]byte
	if err := EncodeFrame(e, pcm, out[:]); err != nil {
		t.Fatalf("EncodeFrame returned %v; want nil (post API-1 wiring)", err)
	}
}

func TestDecodeFrame_TopLevelDelegates(t *testing.T) {
	d := NewDecoder()
	var (
		bits [FrameBytes]byte
		out  [FrameSamples]int16
	)
	if err := DecodeFrame(d, bits[:], out[:]); err != nil {
		t.Fatalf("unexpected error on zero frame: %v", err)
	}
}

func TestWriteFlush_TopLevelDelegates(t *testing.T) {
	samples := makeRamp(FrameSamples*2 + 17)
	split := FrameSamples + 9

	var want bytes.Buffer
	methodEnc := NewStreamingEncoder(&want)
	if _, err := methodEnc.Write(samples[:split]); err != nil {
		t.Fatalf("method Write first chunk: %v", err)
	}
	if _, err := methodEnc.Write(samples[split:]); err != nil {
		t.Fatalf("method Write second chunk: %v", err)
	}
	if err := methodEnc.Flush(); err != nil {
		t.Fatalf("method Flush: %v", err)
	}

	var got bytes.Buffer
	funcEnc := NewStreamingEncoder(&got)
	n, err := Write(funcEnc, samples[:split])
	if err != nil {
		t.Fatalf("top-level Write first chunk: %v", err)
	}
	if n != split {
		t.Fatalf("top-level Write first chunk n: got %d want %d", n, split)
	}
	n, err = Write(funcEnc, samples[split:])
	if err != nil {
		t.Fatalf("top-level Write second chunk: %v", err)
	}
	if n != len(samples)-split {
		t.Fatalf("top-level Write second chunk n: got %d want %d", n, len(samples)-split)
	}
	if err := Flush(funcEnc); err != nil {
		t.Fatalf("top-level Flush: %v", err)
	}

	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatalf("top-level streaming output differs from method output\n got=% x\nwant=% x", got.Bytes(), want.Bytes())
	}
}

func TestWriteFlush_TopLevelNoSinkErrors(t *testing.T) {
	e := NewEncoder()
	if n, err := Write(e, make([]int16, FrameSamples)); n != 0 || !errors.Is(err, ErrNoStreamSink) {
		t.Fatalf("Write without sink: n=%d err=%v", n, err)
	}
	if err := Flush(e); !errors.Is(err, ErrNoStreamSink) {
		t.Fatalf("Flush without sink: err=%v", err)
	}
}
