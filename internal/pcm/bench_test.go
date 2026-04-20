package pcm

import "testing"

func BenchmarkPreProcessor_ProcessFrame(b *testing.B) {
	var p PreProcessor
	in := make([]int16, FrameLength)
	out := make([]int16, FrameLength)
	for i := range in {
		in[i] = int16(i*37 - 1000)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Process(in, out)
	}
}

func BenchmarkScaleUpSat_Frame(b *testing.B) {
	in := make([]int16, FrameLength)
	out := make([]int16, FrameLength)
	for i := range in {
		in[i] = int16(i*101 - 5000)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ScaleUpSat(in, out)
	}
}
