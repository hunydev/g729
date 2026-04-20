package pcm

import "testing"

func TestNoAllocation_ProcessAndScaleUpSat(t *testing.T) {
	var p PreProcessor
	in := make([]int16, FrameLength)
	out := make([]int16, FrameLength)
	for i := range in {
		in[i] = int16(i*37 - 1000)
	}

	cases := []struct {
		name string
		fn   func()
	}{
		{"PreProcessor.Process", func() { p.Process(in, out) }},
		{"ScaleUpSat", func() { ScaleUpSat(in, out) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(1000, tc.fn)
			if allocs != 0 {
				t.Errorf("%s allocated %.2f times per call, want 0",
					tc.name, allocs)
			}
		})
	}
}
