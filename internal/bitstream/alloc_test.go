package bitstream

import "testing"

func TestNoAllocation_PackUnpackParity(t *testing.T) {
	var f Frame
	f.L0 = 1
	f.P1 = 0x55
	var out [FrameBytes]byte

	cases := []struct {
		name string
		fn   func()
	}{
		{"Pack", func() { _ = Pack(&f, out[:]) }},
		{"Unpack", func() {
			var got Frame
			_ = Unpack(out[:], &got)
		}},
		{"Parity", func() { _ = Parity(f.P1) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(1000, tc.fn)
			if allocs != 0 {
				t.Errorf("%s allocated %.2f times per call, want 0", tc.name, allocs)
			}
		})
	}
}
