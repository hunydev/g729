package g729

import (
	"io"
	"math"
	"runtime"
	"sort"
	"testing"
	"time"
)

func BenchmarkThroughput_EncodeFrame(b *testing.B) {
	benchmarkThroughputEncodeFrameProfile(b, EncoderProfileCore)
}

func BenchmarkThroughput_EncodeFrameCoreFast(b *testing.B) {
	benchmarkThroughputEncodeFrameProfile(b, EncoderProfileCoreFast)
}

func benchmarkThroughputEncodeFrameProfile(b *testing.B, profile EncoderProfile) {
	defer pinBenchmarkToSingleThread(b)()

	enc := NewEncoderWithProfile(profile)
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
	b.StopTimer()
	reportFrameThroughput(b, 1)
}

func BenchmarkThroughput_DecodeFrame(b *testing.B) {
	defer pinBenchmarkToSingleThread(b)()

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
	b.StopTimer()
	reportFrameThroughput(b, 1)
}

func BenchmarkThroughput_StreamingWrite800(b *testing.B) {
	benchmarkThroughputStreamingWrite800Profile(b, EncoderProfileCore)
}

func BenchmarkThroughput_StreamingWrite800CoreFast(b *testing.B) {
	benchmarkThroughputStreamingWrite800Profile(b, EncoderProfileCoreFast)
}

func benchmarkThroughputStreamingWrite800Profile(b *testing.B, profile EncoderProfile) {
	defer pinBenchmarkToSingleThread(b)()

	const framesPerWrite = 10
	se := NewStreamingEncoderWithProfile(io.Discard, profile)
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
	b.StopTimer()
	reportFrameThroughput(b, framesPerWrite)
}

func BenchmarkRealtimeJitter_EncodeFrame(b *testing.B) {
	benchmarkRealtimeJitterEncodeFrameProfile(b, EncoderProfileCore)
}

func BenchmarkRealtimeJitter_EncodeFrameCoreFast(b *testing.B) {
	benchmarkRealtimeJitterEncodeFrameProfile(b, EncoderProfileCoreFast)
}

func benchmarkRealtimeJitterEncodeFrameProfile(b *testing.B, profile EncoderProfile) {
	defer pinBenchmarkToSingleThread(b)()

	enc := NewEncoderWithProfile(profile)
	pcm := pcmDeterministic()
	var out [FrameBytes]byte
	for i := 0; i < 16; i++ {
		if err := enc.EncodeFrame(pcm[:], out[:]); err != nil {
			b.Fatal(err)
		}
	}
	durations := make([]int64, b.N)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		if err := enc.EncodeFrame(pcm[:], out[:]); err != nil {
			b.Fatal(err)
		}
		durations[i] = time.Since(start).Nanoseconds()
	}
	b.StopTimer()

	reportFrameTimeDistribution(b, durations)
}

func BenchmarkRealtimeJitter_DecodeFrame(b *testing.B) {
	defer pinBenchmarkToSingleThread(b)()

	bits := benchmarkEncodedFrame(b)
	dec := NewDecoder()
	var out [FrameSamples]int16
	for i := 0; i < 16; i++ {
		if err := dec.DecodeFrame(bits[:], out[:]); err != nil {
			b.Fatal(err)
		}
	}
	durations := make([]int64, b.N)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		if err := dec.DecodeFrame(bits[:], out[:]); err != nil {
			b.Fatal(err)
		}
		durations[i] = time.Since(start).Nanoseconds()
	}
	b.StopTimer()

	reportFrameTimeDistribution(b, durations)
}

func BenchmarkRealtimeJitter_EncodeDecodeLoopback(b *testing.B) {
	benchmarkRealtimeJitterEncodeDecodeLoopbackProfile(b, EncoderProfileCore)
}

func BenchmarkRealtimeJitter_EncodeDecodeLoopbackCoreFast(b *testing.B) {
	benchmarkRealtimeJitterEncodeDecodeLoopbackProfile(b, EncoderProfileCoreFast)
}

func benchmarkRealtimeJitterEncodeDecodeLoopbackProfile(b *testing.B, profile EncoderProfile) {
	defer pinBenchmarkToSingleThread(b)()

	enc := NewEncoderWithProfile(profile)
	dec := NewDecoder()
	pcm := pcmDeterministic()
	var bits [FrameBytes]byte
	var out [FrameSamples]int16
	for i := 0; i < 16; i++ {
		if err := enc.EncodeFrame(pcm[:], bits[:]); err != nil {
			b.Fatal(err)
		}
		if err := dec.DecodeFrame(bits[:], out[:]); err != nil {
			b.Fatal(err)
		}
	}
	durations := make([]int64, b.N)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		if err := enc.EncodeFrame(pcm[:], bits[:]); err != nil {
			b.Fatal(err)
		}
		if err := dec.DecodeFrame(bits[:], out[:]); err != nil {
			b.Fatal(err)
		}
		durations[i] = time.Since(start).Nanoseconds()
	}
	b.StopTimer()

	reportFrameTimeDistribution(b, durations)
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

func pinBenchmarkToSingleThread(b *testing.B) func() {
	b.Helper()
	prevProcs := runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
	return func() {
		runtime.UnlockOSThread()
		runtime.GOMAXPROCS(prevProcs)
	}
}

func reportFrameThroughput(b *testing.B, framesPerOp int) {
	if elapsed := b.Elapsed(); elapsed > 0 {
		frames := framesPerOp * b.N
		frameSeconds := elapsed.Seconds() / float64(frames)
		mediaSeconds := float64(frames*FrameSamples) / float64(SampleRate)
		rtf := elapsed.Seconds() / mediaSeconds
		xRealtime := 1 / rtf

		b.ReportMetric(float64(frames)/elapsed.Seconds(), "frames/s")
		b.ReportMetric(float64(frames*FrameSamples)/elapsed.Seconds(), "samples/s")
		b.ReportMetric(frameSeconds*1e6, "us/frame")
		b.ReportMetric(rtf, "rtf")
		b.ReportMetric(xRealtime, "x-realtime")
		b.ReportMetric(xRealtime, "streams/core")
	}
}

func reportFrameTimeDistribution(b *testing.B, durations []int64) {
	b.Helper()
	if len(durations) == 0 {
		return
	}

	sorted := append([]int64(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total int64
	for _, d := range durations {
		total += d
	}
	mean := float64(total) / float64(len(durations))
	var sumSquares float64
	for _, d := range durations {
		delta := float64(d) - mean
		sumSquares += delta * delta
	}
	stddev := math.Sqrt(sumSquares / float64(len(durations)))

	p50 := percentileNS(sorted, 0.50)
	p95 := percentileNS(sorted, 0.95)
	p99 := percentileNS(sorted, 0.99)
	max := sorted[len(sorted)-1]
	deadlineNS := float64(time.Duration(FrameSamples) * time.Second / time.Duration(SampleRate))
	meanRTF := (mean / 1e9) / (float64(FrameSamples) / float64(SampleRate))

	b.ReportMetric(mean/1e3, "mean-us")
	b.ReportMetric(float64(p50)/1e3, "p50-us")
	b.ReportMetric(float64(p95)/1e3, "p95-us")
	b.ReportMetric(float64(p99)/1e3, "p99-us")
	b.ReportMetric(float64(max)/1e3, "max-us")
	b.ReportMetric(stddev/1e3, "stddev-us")
	b.ReportMetric(float64(p99-p50)/1e3, "p99-jitter-us")
	b.ReportMetric(float64(max-p50)/1e3, "max-jitter-us")
	b.ReportMetric(float64(p99)/deadlineNS, "p99-deadline")
	b.ReportMetric(float64(max)/deadlineNS, "max-deadline")
	b.ReportMetric(meanRTF, "mean-rtf")
	b.ReportMetric(1/meanRTF, "mean-streams/core")
}

func percentileNS(sorted []int64, p float64) int64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}
