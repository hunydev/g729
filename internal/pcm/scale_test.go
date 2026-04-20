package pcm

import "testing"

func TestScaleUpSat_InRange(t *testing.T) {
	cases := []struct {
		in, want int16
	}{
		{0, 0},
		{1, 2},
		{-1, -2},
		{100, 200},
		{-100, -200},
		{16_383, 32_766},
		{-16_384, -32_768},
	}
	for _, tc := range cases {
		in := []int16{tc.in}
		out := make([]int16, 1)
		ScaleUpSat(in, out)
		if out[0] != tc.want {
			t.Errorf("ScaleUpSat(%d) = %d, want %d", tc.in, out[0], tc.want)
		}
	}
}

func TestScaleUpSat_Saturates(t *testing.T) {
	cases := []struct {
		in, want int16
	}{
		{16_384, 32_767},
		{17_000, 32_767},
		{32_767, 32_767},
		{-16_385, -32_768},
		{-32_768, -32_768},
	}
	for _, tc := range cases {
		in := []int16{tc.in}
		out := make([]int16, 1)
		ScaleUpSat(in, out)
		if out[0] != tc.want {
			t.Errorf("ScaleUpSat(%d) = %d, want %d (saturation)",
				tc.in, out[0], tc.want)
		}
	}
}

func TestScaleUpSat_LengthMismatch(t *testing.T) {
	in := []int16{1, 2, 3, 4}
	out := make([]int16, 2)
	ScaleUpSat(in, out)
	if out[0] != 2 || out[1] != 4 {
		t.Errorf("got %v, want [2 4]", out)
	}
}

func TestScaleUpSat_AliasingSafe(t *testing.T) {
	buf := []int16{3, -5, 16_384, -16_385, 0}
	want := []int16{6, -10, 32_767, -32_768, 0}
	ScaleUpSat(buf, buf)
	for i, v := range buf {
		if v != want[i] {
			t.Errorf("buf[%d] = %d, want %d", i, v, want[i])
		}
	}
}
