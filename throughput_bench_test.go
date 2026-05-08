package g729

import (
	"io"
	"testing"
)

func BenchmarkThroughput_EncodeFrame(b *testing.B) {
	enc := NewEncoder()
	pcm := pcmDeterministic()
	var out [FrameBytes]byte
	for i := 0; i < 8; i++ {
		_ = enc.EncodeFrame(pcm[:], out[:])
	}

	b.ReportAllocs()
	b.SetBytes(FrameSamples * 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.EncodeFrame(pcm[:], out[:]); err != nil {
			b.Fatal(err)
		}
	}
	reportFrameThroughput(b, 1)
}

func BenchmarkThroughput_DecodeFrame(b *testing.B) {
	bits := benchmarkEncodedFrame(b)
	dec := NewDecoder()
	var out [FrameSamples]int16
	for i := 0; i < 8; i++ {
		_ = dec.DecodeFrame(bits[:], out[:])
	}

	b.ReportAllocs()
	b.SetBytes(FrameBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := dec.DecodeFrame(bits[:], out[:]); err != nil {
			b.Fatal(err)
		}
	}
	reportFrameThroughput(b, 1)
}

func BenchmarkThroughput_StreamingWrite800(b *testing.B) {
	const framesPerWrite = 10
	se := NewStreamingEncoder(io.Discard)
	pcm := make([]int16, framesPerWrite*FrameSamples)
	src := pcmDeterministic()
	for frame := 0; frame < framesPerWrite; frame++ {
		copy(pcm[frame*FrameSamples:(frame+1)*FrameSamples], src[:])
	}
	for i := 0; i < 4; i++ {
		_, _ = se.Write(pcm)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(pcm) * 2))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := se.Write(pcm); err != nil {
			b.Fatal(err)
		}
	}
	reportFrameThroughput(b, framesPerWrite)
}

func benchmarkEncodedFrame(b *testing.B) [FrameBytes]byte {
	b.Helper()
	enc := NewEncoder()
	pcm := pcmDeterministic()
	var bits [FrameBytes]byte
	for i := 0; i < 8; i++ {
		if err := enc.EncodeFrame(pcm[:], bits[:]); err != nil {
			b.Fatal(err)
		}
	}
	return bits
}

func reportFrameThroughput(b *testing.B, framesPerOp int) {
	if elapsed := b.Elapsed(); elapsed > 0 {
		b.ReportMetric(float64(framesPerOp*b.N)/elapsed.Seconds(), "frames/s")
		b.ReportMetric(float64(framesPerOp*b.N*FrameSamples)/elapsed.Seconds(), "samples/s")
	}
}
