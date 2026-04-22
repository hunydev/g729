package decoder

import (
	"math"
	"testing"
)

func TestHPFilter_ZeroInputIsZero(t *testing.T) {
	var d Decoder
	var in, out [40]int16
	d.hpFilter(&in, out[:])
	var zero [40]int16
	if out != zero {
		t.Fatalf("zero input produced non-zero output: %v", out)
	}
	if d.hpX != ([2]int16{}) || d.hpY != ([2]int32{}) {
		t.Fatalf("zero input advanced state: hpX=%v hpY=%v", d.hpX, d.hpY)
	}
}

func TestHPFilter_DCStepDecaysToZero(t *testing.T) {
	var d Decoder
	var in [40]int16
	for i := range in {
		in[i] = 1000
	}
	var out [40]int16
	for k := 0; k < 20; k++ {
		d.hpFilter(&in, out[:])
	}
	for _, v := range out {
		if v < -50 || v > 50 {
			t.Fatalf("DC step did not decay: sample=%d", v)
		}
	}
}

func TestHPFilter_ImpulseResponseNonTrivial(t *testing.T) {
	var d Decoder
	var in [40]int16
	in[0] = 10000
	var out [40]int16
	d.hpFilter(&in, out[:])
	want0 := int16(9398)
	if out[0] < want0-20 || out[0] > want0+20 {
		t.Fatalf("y[0]: want %d ± 20, got %d", want0, out[0])
	}
	var energy int64
	for _, v := range out {
		energy += int64(v) * int64(v)
	}
	if energy < 1_000_000 {
		t.Fatalf("impulse response energy too low: %d", energy)
	}
}

func TestHPFilter_StatePropagatesAcrossCalls(t *testing.T) {
	var full [80]int16
	for i := range full {
		full[i] = int16(5000 * math.Sin(float64(i)*math.Pi/20))
	}

	var dSplit Decoder
	var outSplit [80]int16
	var firstHalf, secondHalf [40]int16
	copy(firstHalf[:], full[:40])
	copy(secondHalf[:], full[40:])
	dSplit.hpFilter(&firstHalf, outSplit[:40])
	dSplit.hpFilter(&secondHalf, outSplit[40:])

	diff := int(outSplit[40]) - int(outSplit[39])
	if diff < -10000 || diff > 10000 {
		t.Fatalf("state propagation failure: jump at split = %d", diff)
	}
}

// TestHpFilter_ImpulseFirstSample asserts the exact first-sample value
// of an impulse response y[0] = b0·x[0] given zero state.
func TestHpFilter_ImpulseFirstSample(t *testing.T) {
var d Decoder
var in [subframeLen]int16
in[0] = 32767

var out [subframeLen]int16
d.hpFilter(&in, out[:])

const want = int16(30795)
if out[0] != want {
t.Fatalf("hpFilter impulse out[0] = %d; want %d (b0·32767>>13 rounded)", out[0], want)
}
}

func TestHpFilter_ZeroInputZeroState(t *testing.T) {
var d Decoder
var in [subframeLen]int16

var out [subframeLen]int16
d.hpFilter(&in, out[:])

for i, v := range out {
if v != 0 {
t.Fatalf("hpFilter(zero) out[%d] = %d; want 0", i, v)
}
}
}

func TestHpFilter_DCRejection(t *testing.T) {
var d Decoder
var in [subframeLen]int16
for i := range in {
in[i] = 1000
}

var out [subframeLen]int16
for i := 0; i < 8; i++ {
d.hpFilter(&in, out[:])
}

for i, v := range out {
if v < -100 || v > 100 {
t.Fatalf("hpFilter(DC=1000) out[%d] (subframe 8) = %d; want |·| < 100", i, v)
}
}
}
