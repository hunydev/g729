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

func TestDecode_AllZeroFrameDeterministic(t *testing.T) {
var packed [10]byte
var d1, d2 Decoder
var out1, out2 [80]int16
if err := d1.Decode(packed[:], false, out1[:]); err != nil {
t.Fatalf("d1: %v", err)
}
if err := d2.Decode(packed[:], false, out2[:]); err != nil {
t.Fatalf("d2: %v", err)
}
if out1 != out2 {
t.Fatal("two identical calls diverged")
}
}

func TestDecode_ShortInputRejected(t *testing.T) {
var d Decoder
var short [9]byte
var out [80]int16
out[0] = 42
if err := d.Decode(short[:], false, out[:]); err != ErrShortInput {
t.Fatalf("want ErrShortInput, got %v", err)
}
if out[0] != 42 {
t.Fatal("out mutated despite ErrShortInput")
}
}

func TestDecode_ShortOutputRejected(t *testing.T) {
var d Decoder
var packed [10]byte
var short [79]int16
if err := d.Decode(packed[:], false, short[:]); err != ErrShortOutput {
t.Fatalf("want ErrShortOutput, got %v", err)
}
}

func TestDecode_TwoFramesStateAdvance(t *testing.T) {
var d Decoder
var packed [10]byte
packed[0] = 0x40
var outA, outB [80]int16
if err := d.Decode(packed[:], false, outA[:]); err != nil {
t.Fatal(err)
}
if err := d.Decode(packed[:], false, outB[:]); err != nil {
t.Fatal(err)
}
if outA == outB {
t.Fatal("state did not advance between two identical frames")
}
}

func TestDecode_ResetRestoresDeterminism(t *testing.T) {
var d Decoder
var packed [10]byte
var throwaway [80]int16
_ = d.Decode(packed[:], false, throwaway[:])

var freshOut, resetOut [80]int16
var fresh Decoder
if err := fresh.Decode(packed[:], false, freshOut[:]); err != nil {
t.Fatal(err)
}
d.Reset()
if err := d.Decode(packed[:], false, resetOut[:]); err != nil {
t.Fatal(err)
}
if freshOut != resetOut {
t.Fatal("Reset did not restore zero-value decode output")
}
}

func TestDecode_BadFlagAcceptedButIgnored(t *testing.T) {
var d1, d2 Decoder
var packed [10]byte
var out1, out2 [80]int16
_ = d1.Decode(packed[:], false, out1[:])
_ = d2.Decode(packed[:], true, out2[:])
if out1 != out2 {
t.Fatal("Phase 1g must ignore the bad flag; Phase 1h will add concealment")
}
}
