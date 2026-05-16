package g729

import (
	"errors"
	"testing"
)

func TestErrors_AreSentinels(t *testing.T) {
	cases := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrShortPCM", ErrShortPCM, "g729: input PCM length not multiple of frame size (80)"},
		{"ErrShortOutput", ErrShortOutput, "g729: output buffer too small"},
		{"ErrShortBitstream", ErrShortBitstream, "g729: bitstream length not multiple of 10 bytes"},
		{"ErrUnsupportedAnnexB", ErrUnsupportedAnnexB, "g729: Annex B SID/CNG/DTX is not supported"},
		{"ErrNoStreamSink", ErrNoStreamSink, "g729: encoder has no streaming sink (use NewStreamingEncoder)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.err == nil {
				t.Fatalf("%s is nil", c.name)
			}
			if c.err.Error() != c.msg {
				t.Fatalf("%s: got %q want %q", c.name, c.err.Error(), c.msg)
			}
			if !errors.Is(c.err, c.err) {
				t.Fatalf("%s: errors.Is self-check failed", c.name)
			}
		})
	}
}
