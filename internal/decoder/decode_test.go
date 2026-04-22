package decoder

import "testing"

func TestDecoderZeroValueIsUsable(t *testing.T) {
	var d Decoder
	d.Reset()
}

func TestResetAfterUse(t *testing.T) {
	var d Decoder
	d.prevGpQ14 = 12345
	d.hpX[0] = 42
	d.hpY[1] = 99
	d.pastExc[0] = 7
	d.Reset()
	if d.prevGpQ14 != 0 || d.hpX[0] != 0 || d.hpY[1] != 0 || d.pastExc[0] != 0 {
		t.Fatalf("Reset did not clear state: %+v", d)
	}
}
