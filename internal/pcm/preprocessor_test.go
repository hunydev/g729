package pcm

import "testing"

func TestPreProcessor_ZeroValueIsUsable(t *testing.T) {
	// A zero-value PreProcessor must behave like a freshly Reset one:
	// its filter state should all be zero, so feeding zeros produces zeros.
	var p PreProcessor
	in := make([]int16, FrameLength)
	out := make([]int16, FrameLength)
	p.Process(in, out)
	for i, v := range out {
		if v != 0 {
			t.Errorf("out[%d] = %d, want 0 on zero-state zero-input", i, v)
		}
	}
}

func TestFrameLength(t *testing.T) {
	if FrameLength != 80 {
		t.Fatalf("FrameLength = %d, want 80 (10 ms at 8 kHz)", FrameLength)
	}
}

func TestPreProcessor_ResetClearsState(t *testing.T) {
var p PreProcessor
p.x1 = 1234
p.x2 = -5678
p.y1 = 9_000_000
p.y2 = -3_000_000

p.Reset()

if p.x1 != 0 || p.x2 != 0 || p.y1 != 0 || p.y2 != 0 {
t.Errorf("Reset did not clear state: %+v", p)
}
}

func TestPreProcessor_RejectsDC(t *testing.T) {
var p PreProcessor
const (
n  = 1024
dc = int16(2000)
)
in := make([]int16, n)
out := make([]int16, n)
for i := range in {
in[i] = dc
}
p.Process(in, out)

const tailTol = 4
for i := n - 16; i < n; i++ {
if out[i] > tailTol || out[i] < -tailTol {
t.Fatalf("DC not rejected: out[%d] = %d (tol ±%d)", i, out[i], tailTol)
}
}
}

func TestPreProcessor_ZeroInputAfterNonzeroTailsToZero(t *testing.T) {
var p PreProcessor
impulse := make([]int16, 64)
impulse[0] = 10_000
out1 := make([]int16, 64)
p.Process(impulse, out1)

zeros := make([]int16, 4096)
out2 := make([]int16, 4096)
p.Process(zeros, out2)

for i := len(out2) - 16; i < len(out2); i++ {
if out2[i] > 4 || out2[i] < -4 {
t.Fatalf("did not decay to zero: out2[%d] = %d", i, out2[i])
}
}
}

func TestPreProcessor_ZeroInputStaysZero(t *testing.T) {
var p PreProcessor
in := make([]int16, FrameLength*4)
out := make([]int16, FrameLength*4)
p.Process(in, out)
for i, v := range out {
if v != 0 {
t.Fatalf("out[%d] = %d, want 0", i, v)
}
}
}

func TestPreProcessor_ImpulseIsNonZero(t *testing.T) {
var p PreProcessor
in := make([]int16, FrameLength)
out := make([]int16, FrameLength)
in[0] = 32_000
p.Process(in, out)
if out[0] == 0 {
t.Fatalf("impulse response out[0] = 0, expected non-zero")
}
}

func TestPreProcessor_ChunkedEqualsOneShot(t *testing.T) {
rand := func(seed uint32) func() int16 {
s := seed
return func() int16 {
s = s*1664525 + 1013904223
return int16(int32(s>>16) - 16384)
}
}

const n = FrameLength * 10
next := rand(0xDEADBEEF)
in := make([]int16, n)
for i := range in {
in[i] = next()
}

var pWhole PreProcessor
outWhole := make([]int16, n)
pWhole.Process(in, outWhole)

var pChunked PreProcessor
outChunked := make([]int16, n)
for off := 0; off < n; off += FrameLength {
pChunked.Process(in[off:off+FrameLength], outChunked[off:off+FrameLength])
}

for i := range outWhole {
if outWhole[i] != outChunked[i] {
t.Fatalf("mismatch at %d: whole=%d chunked=%d",
i, outWhole[i], outChunked[i])
}
}
}

func TestPreProcessor_ImpulseMatchesReference(t *testing.T) {
const (
magnitude = 16384.0
n         = 32
// 6 LSB tolerance: Q13 coefficient quantization plus int16 output
// rounding accumulates ~4.2 LSB by sample 31. 6 LSB still catches
// gross transcription errors (those would diff in the hundreds).
tol       = 6.0
)
const (
a1 = 1.9059465
a2 = -0.9114024
b0 = 0.46363718
b1 = -0.92724705
b2 = 0.46363718
)
var refX1, refX2, refY1, refY2 float64
ref := make([]float64, n)
for i := 0; i < n; i++ {
var x float64
if i == 0 {
x = magnitude
}
y := a1*refY1 + a2*refY2 + b0*x + b1*refX1 + b2*refX2
ref[i] = y
refX2 = refX1
refX1 = x
refY2 = refY1
refY1 = y
}

var p PreProcessor
in := make([]int16, n)
out := make([]int16, n)
in[0] = int16(magnitude)
p.Process(in, out)

for i := 0; i < n; i++ {
diff := mathAbs(float64(out[i]) - ref[i])
if diff > tol {
t.Errorf("sample %d: impl=%d ref=%.3f diff=%.3f (tol %.1f)",
i, out[i], ref[i], diff, tol)
}
}
}

func mathAbs(x float64) float64 {
if x < 0 {
return -x
}
return x
}
