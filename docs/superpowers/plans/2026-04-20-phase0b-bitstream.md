# Phase 0b — Bitstream Pack/Unpack and G.192 I/O Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `internal/bitstream` package — packing/unpacking the 15 G.729 frame parameters to and from the canonical 80-bit (10-byte) wire format, plus reading and writing the ITU-T G.192 `.bit` serial bitstream format used by the official G.729 test vectors.

**Architecture:** One internal package with three layers: (1) small MSB-first bit I/O helpers (`BitWriter`, `BitReader`), (2) `Pack` / `Unpack` using those helpers against a fixed `Frame` struct whose field order matches ITU's transmission order, (3) G.192 `.bit` file I/O that wraps raw frame bytes with sync and length words. No arithmetic — just bit shuffling and endianness. All hot-path functions are zero-allocation; G.192 I/O uses small internal scratch buffers and is not on the encoder hot path.

**Tech Stack:** Go 1.22+, standard library only (`io`, `encoding/binary`, `errors`). Scratch-from-spec — bit field allocations and G.192 format constants must be taken from the ITU specifications (G.729 + Annex A for the frame format, G.191 STL documentation for G.192), never from existing implementation code.

---

## Context for the implementing engineer

### What this package is for

A G.729 encoder produces 15 named parameters per 10 ms frame, quantized into integer indices of specific bit widths. Those 15 parameters are serialized into 80 consecutive bits forming one frame on the wire (10 bytes). The decoder reverses the process. This package is the only place in the codec that understands the wire layout; every other block works in terms of the `Frame` struct or in terms of raw parameter values in memory.

Separately, ITU publishes test vectors in the G.192 **serial bitstream format** — a 16-bit-word encoding where each word represents either one source bit (`0x0081` for `1`, `0x007F` for `0`) or a sync / length marker. The test vector files we will commit to `testdata/itu/` are in this format, so the bitstream package must read and write G.192 too.

### Bit allocation per frame (ITU-T G.729 + Annex A)

The 80 bits are transmitted in a fixed order. Each parameter carries a fixed number of bits, MSB first within the parameter, and parameters are concatenated MSB-first into the output byte stream.

| Order | Name | Bits | Meaning |
|---|---|---|---|
| 1 | `L0`  | 1 | LSP MA predictor switch |
| 2 | `L1`  | 7 | LSP 1st-stage codebook index |
| 3 | `L2`  | 5 | LSP 2nd-stage lower-split index |
| 4 | `L3`  | 5 | LSP 2nd-stage upper-split index |
| 5 | `P1`  | 8 | Pitch delay, subframe 1 |
| 6 | `P0`  | 1 | Parity of the 6 MSBs of `P1` |
| 7 | `C1`  | 13 | ACELP fixed-codebook positions, subframe 1 |
| 8 | `S1`  | 4 | ACELP signs, subframe 1 |
| 9 | `GA1` | 3 | Gain stage-1 codebook, subframe 1 |
| 10 | `GB1` | 4 | Gain stage-2 codebook, subframe 1 |
| 11 | `P2`  | 5 | Pitch delay, subframe 2 |
| 12 | `C2`  | 13 | ACELP positions, subframe 2 |
| 13 | `S2`  | 4 | ACELP signs, subframe 2 |
| 14 | `GA2` | 3 | Gain stage-1 codebook, subframe 2 |
| 15 | `GB2` | 4 | Gain stage-2 codebook, subframe 2 |

Total: 1+7+5+5+8+1+13+4+3+4+5+13+4+3+4 = **80 bits** = 10 bytes.

### Byte and bit ordering

- **Within a byte:** most significant bit is written first. The first bit of the frame (the `L0` value) occupies bit 7 of `out[0]`.
- **Across bytes:** `out[0]` holds the first 8 bits (L0, L1[6..0]), `out[1]` the next 8 bits, and so on.
- **Within a parameter:** MSB first. If `L1 = 0b1010101` (7 bits), the most significant `1` is written before the following bits.

### Parity bit `P0`

`P0` is the XOR of the 6 most-significant bits of `P1` (i.e. of `P1 >> 2` taken as a 6-bit value). It is set by the encoder and checked by the decoder to help detect transmission errors in the pitch delay. The encoder writes the computed parity into the frame. This package provides a `Parity(p1)` helper; callers (the encoder in a later phase) are responsible for invoking it when assembling a `Frame`.

### G.192 serial bitstream format

Each G.729 frame in a G.192 `.bit` file occupies 82 contiguous 16-bit words:

1. **Sync word** — `0x6B21` for a good frame, `0x6B20` for a "bad frame" / erasure marker.
2. **Length word** — the literal integer `80` (i.e. `0x0050`), the number of data bits that follow.
3. **80 data words** — one per source bit: `0x0081` for a `1` bit, `0x007F` for a `0` bit. Source bits are emitted in the same order they appear on the wire (MSB of `out[0]` first).

On disk the 16-bit words are stored **little-endian**. This matches the distributed ITU G.729 test vectors and modern toolchains. If a future vector set is encountered in big-endian, a separate reader variant can be added; for now we commit to little-endian.

One G.192 frame is therefore `82 * 2 = 164` bytes on disk.

### Package layout produced by this plan

```
g729/internal/bitstream/
├── doc.go                (package doc with ITU references)
├── types.go              (Frame struct, FrameBytes, FrameBits)
├── errors.go             (sentinel errors)
├── bitio.go              (BitWriter, BitReader)
├── bitio_test.go
├── pack.go               (Pack, Unpack)
├── pack_test.go
├── parity.go             (Parity helper)
├── parity_test.go
├── g192.go               (G.192 constants, WriteG192Frame, ReadG192Frame, ReadG192File)
├── g192_test.go
├── alloc_test.go         (zero-allocation assertions)
└── bench_test.go         (Pack / Unpack benchmarks)
```

### Dependency rule

This package depends only on the Go standard library (`io`, `encoding/binary`, `errors`). It does **not** depend on `internal/fixed` — bitstream manipulation is pure bit shuffling, not fixed-point arithmetic.

### Commit style

Same conventional-commit style as Phase 0a: `feat(bitstream)`, `test(bitstream)`, `docs(bitstream)`, etc.

---

## Task 1: Package skeleton, `Frame` struct, and errors

**Files:**
- Create: `internal/bitstream/doc.go`
- Create: `internal/bitstream/types.go`
- Create: `internal/bitstream/errors.go`
- Create: `internal/bitstream/types_test.go`

- [x] **Step 1: Write the failing test**

Create `internal/bitstream/types_test.go`:
```go
package bitstream

import "testing"

func TestConstants(t *testing.T) {
	if FrameBits != 80 {
		t.Errorf("FrameBits = %d, want 80", FrameBits)
	}
	if FrameBytes != 10 {
		t.Errorf("FrameBytes = %d, want 10", FrameBytes)
	}
}

func TestFrameZeroValue(t *testing.T) {
	var f Frame
	// All fields must default-zero, and reading one via interface must compile.
	if f.L0 != 0 || f.GB2 != 0 {
		t.Errorf("Frame zero value should have all fields zero")
	}
}
```

- [x] **Step 2: Verify the test fails**

Run:
```bash
cd /home/exedev/g729 && go test ./internal/bitstream/...
```
Expected: compile error — `undefined: FrameBits`, `undefined: Frame`, etc.

- [x] **Step 3: Create `internal/bitstream/types.go`**

```go
package bitstream

// Frame format constants.
const (
	// FrameBits is the number of bits per G.729 frame on the wire.
	FrameBits = 80
	// FrameBytes is the number of bytes per packed G.729 frame.
	FrameBytes = FrameBits / 8
)

// Frame is one decoded G.729 frame's transmitted parameters. Field order
// matches the ITU-T G.729 transmission order (Annex A Table 8). All fields
// hold unsigned integer indices; the declared range of each is given in
// the doc comment and enforced implicitly by the bit width in Pack.
type Frame struct {
	L0  uint16 // 1 bit  — LSP MA predictor switch
	L1  uint16 // 7 bits — LSP 1st stage codebook index
	L2  uint16 // 5 bits — LSP 2nd stage lower split
	L3  uint16 // 5 bits — LSP 2nd stage upper split
	P1  uint16 // 8 bits — pitch delay subframe 1
	P0  uint16 // 1 bit  — parity of upper 6 bits of P1
	C1  uint16 // 13 bits — ACELP positions subframe 1
	S1  uint16 // 4 bits — ACELP signs subframe 1
	GA1 uint16 // 3 bits — gain stage 1 subframe 1
	GB1 uint16 // 4 bits — gain stage 2 subframe 1
	P2  uint16 // 5 bits — pitch delay subframe 2
	C2  uint16 // 13 bits — ACELP positions subframe 2
	S2  uint16 // 4 bits — ACELP signs subframe 2
	GA2 uint16 // 3 bits — gain stage 1 subframe 2
	GB2 uint16 // 4 bits — gain stage 2 subframe 2
}
```

- [x] **Step 4: Create `internal/bitstream/errors.go`**

```go
package bitstream

import "errors"

// ErrShortOutput is returned when an output buffer is smaller than the
// required fixed size (FrameBytes for Pack, 80 samples for Decode).
var ErrShortOutput = errors.New("bitstream: output buffer too small")

// ErrShortInput is returned when an input buffer is shorter than the
// required fixed size (FrameBytes for Unpack).
var ErrShortInput = errors.New("bitstream: input buffer too short")

// ErrBadG192Sync is returned when a G.192 frame's sync word is neither
// the good-frame nor bad-frame marker.
var ErrBadG192Sync = errors.New("bitstream: invalid G.192 sync word")

// ErrBadG192Length is returned when a G.192 frame's length word does
// not equal FrameBits.
var ErrBadG192Length = errors.New("bitstream: invalid G.192 length word")

// ErrBadG192Bit is returned when a G.192 data word is neither the
// 0-bit nor the 1-bit marker.
var ErrBadG192Bit = errors.New("bitstream: invalid G.192 data word")
```

- [x] **Step 5: Create `internal/bitstream/doc.go`**

```go
// Package bitstream converts G.729 frame parameters to and from their
// canonical 80-bit wire representation, and reads/writes the ITU-T
// G.192 serial bitstream file format used by the official G.729 test
// vectors.
//
// # Wire format
//
// A G.729 frame is 80 bits, transmitted MSB-first within each byte.
// Parameters are concatenated in the order declared by Frame (see the
// Frame struct field order), each contributing its declared bit width
// MSB-first. Total: 10 bytes per frame.
//
// # G.192 file format
//
// Each frame in a .bit file is 82 little-endian 16-bit words: one sync
// word, one length word (= FrameBits), and 80 data words. Data words
// encode the bit value, not pack it: 0x0081 for 1, 0x007F for 0.
//
// # References
//
//   - ITU-T G.729 and G.729 Annex A, "Coding of speech at 8 kbit/s
//     using CS-ACELP", parameter transmission tables.
//   - ITU-T G.191 Software Tools Library, Section on serial bitstream
//     (G.192) format.
package bitstream
```

- [x] **Step 6: Run tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -v
```
Expected: `TestConstants` and `TestFrameZeroValue` pass.

- [x] **Step 7: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/ && git commit -m "feat(bitstream): package skeleton with Frame struct and errors"
```

---

## Task 2: `BitWriter` — MSB-first bit writing into a byte slice

**Files:**
- Create: `internal/bitstream/bitio.go`
- Create: `internal/bitstream/bitio_test.go`

A tiny helper. `bitPos` tracks the next bit to write (bit 0 of `bitPos = 0` is the MSB of `buf[0]`). `Write(value, n)` writes the low `n` bits of `value`, MSB of that slice first.

- [x] **Step 1: Write the failing test**

Create `internal/bitstream/bitio_test.go`:
```go
package bitstream

import "testing"

func TestBitWriter_SingleBit(t *testing.T) {
	var buf [1]byte
	var w BitWriter
	w.Init(buf[:])
	w.Write(1, 1)
	if buf[0] != 0b10000000 {
		t.Errorf("buf[0] = %#08b, want 10000000", buf[0])
	}
	if w.BitPos() != 1 {
		t.Errorf("BitPos = %d, want 1", w.BitPos())
	}
}

func TestBitWriter_EightOnes(t *testing.T) {
	var buf [1]byte
	var w BitWriter
	w.Init(buf[:])
	for i := 0; i < 8; i++ {
		w.Write(1, 1)
	}
	if buf[0] != 0xFF {
		t.Errorf("buf[0] = %#08b, want 11111111", buf[0])
	}
}

func TestBitWriter_MultiBitField(t *testing.T) {
	var buf [2]byte
	var w BitWriter
	w.Init(buf[:])
	// Write a 7-bit value 0b1010101 at bit 0 of byte 0. That value's MSB
	// lands in buf[0] bit 7 (=1), next in bit 6 (=0), ... so the layout
	// is: 1 0 1 0 1 0 1 _ — buf[0] = 0b10101010 = 0xAA.
	w.Write(0b1010101, 7)
	if buf[0] != 0b10101010 {
		t.Errorf("buf[0] = %#08b, want 10101010", buf[0])
	}
	if w.BitPos() != 7 {
		t.Errorf("BitPos = %d, want 7", w.BitPos())
	}
}

func TestBitWriter_CrossesByteBoundary(t *testing.T) {
	var buf [2]byte
	var w BitWriter
	w.Init(buf[:])
	w.Write(0x3, 2) // 11 at bit pos 0..1 of buf[0]
	w.Write(0x3F, 8) // 00111111 at bit pos 2..9
	// buf[0] bits: 1 1 0 0 1 1 1 1 = 0xCF
	// buf[1] bits: 1 1 _ _ _ _ _ _ = 0xC0
	if buf[0] != 0xCF || buf[1] != 0xC0 {
		t.Errorf("buf = [%#02x, %#02x], want [0xCF, 0xC0]", buf[0], buf[1])
	}
	if w.BitPos() != 10 {
		t.Errorf("BitPos = %d, want 10", w.BitPos())
	}
}
```

- [x] **Step 2: Verify the tests fail**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestBitWriter
```
Expected: compile error — `undefined: BitWriter`.

- [x] **Step 3: Implement `BitWriter`**

Create `internal/bitstream/bitio.go`:
```go
package bitstream

// BitWriter writes bits MSB-first into a caller-owned byte slice. The
// slice must be zero-initialized before use; BitWriter only sets 1
// bits, never clears existing 1 bits. Callers typically zero the
// buffer themselves (or rely on Go's zero-initialization) before Init.
type BitWriter struct {
	buf    []byte
	bitPos int
}

// Init resets the writer to the start of buf. The previous contents
// of buf are left untouched.
func (w *BitWriter) Init(buf []byte) {
	w.buf = buf
	w.bitPos = 0
}

// BitPos returns the number of bits written so far.
func (w *BitWriter) BitPos() int { return w.bitPos }

// Write writes the low n bits of value into the buffer, MSB of those
// n bits first. Bits beyond position len(buf)*8 are dropped silently;
// callers must ensure the buffer is large enough for all writes.
func (w *BitWriter) Write(value uint16, n int) {
	for i := n - 1; i >= 0; i-- {
		if (value>>uint(i))&1 == 1 {
			byteIdx := w.bitPos >> 3
			bitIdx := 7 - (w.bitPos & 7)
			if byteIdx < len(w.buf) {
				w.buf[byteIdx] |= 1 << uint(bitIdx)
			}
		}
		w.bitPos++
	}
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestBitWriter -v
```
Expected: all four subtests pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/bitio.go internal/bitstream/bitio_test.go && git commit -m "feat(bitstream): add MSB-first BitWriter"
```

---

## Task 3: `BitReader` — MSB-first bit reading from a byte slice

**Files:**
- Modify: `internal/bitstream/bitio.go` (append)
- Modify: `internal/bitstream/bitio_test.go` (append)

- [x] **Step 1: Append failing tests**

Append to `internal/bitstream/bitio_test.go`:
```go
func TestBitReader_SingleBit(t *testing.T) {
	buf := []byte{0b10000000}
	var r BitReader
	r.Init(buf)
	if got := r.Read(1); got != 1 {
		t.Errorf("Read(1) = %d, want 1", got)
	}
	if r.BitPos() != 1 {
		t.Errorf("BitPos = %d, want 1", r.BitPos())
	}
}

func TestBitReader_Byte(t *testing.T) {
	buf := []byte{0xAB}
	var r BitReader
	r.Init(buf)
	if got := r.Read(8); got != 0xAB {
		t.Errorf("Read(8) = %#x, want 0xAB", got)
	}
}

func TestBitReader_MultiBitField(t *testing.T) {
	buf := []byte{0b10101010} // first 7 bits as a field = 0b1010101 = 85
	var r BitReader
	r.Init(buf)
	if got := r.Read(7); got != 0b1010101 {
		t.Errorf("Read(7) = %#b, want 1010101", got)
	}
	if r.BitPos() != 7 {
		t.Errorf("BitPos = %d, want 7", r.BitPos())
	}
}

func TestBitReader_CrossesByteBoundary(t *testing.T) {
	// Inverse of the TestBitWriter_CrossesByteBoundary layout.
	buf := []byte{0xCF, 0xC0}
	var r BitReader
	r.Init(buf)
	if got := r.Read(2); got != 0x3 {
		t.Errorf("Read(2) #1 = %#x, want 3", got)
	}
	if got := r.Read(8); got != 0x3F {
		t.Errorf("Read(8) = %#x, want 0x3F", got)
	}
}

func TestBitWriter_ReadRoundTrip(t *testing.T) {
	// Sanity: BitWriter and BitReader agree bit-for-bit.
	var buf [2]byte
	var w BitWriter
	w.Init(buf[:])
	fields := []struct {
		value uint16
		bits  int
	}{
		{0x1, 1},
		{0x55, 7},
		{0xA, 4},
		{0x3, 2},
		{0xF, 2},
	}
	for _, f := range fields {
		w.Write(f.value, f.bits)
	}

	var r BitReader
	r.Init(buf[:])
	for i, f := range fields {
		got := r.Read(f.bits)
		want := f.value & ((1 << uint(f.bits)) - 1)
		if got != want {
			t.Errorf("field %d: got %#x, want %#x", i, got, want)
		}
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestBitReader
```
Expected: compile error — `undefined: BitReader`.

- [x] **Step 3: Implement `BitReader`**

Append to `internal/bitstream/bitio.go`:
```go
// BitReader reads bits MSB-first from a caller-owned byte slice.
type BitReader struct {
	buf    []byte
	bitPos int
}

// Init resets the reader to the start of buf.
func (r *BitReader) Init(buf []byte) {
	r.buf = buf
	r.bitPos = 0
}

// BitPos returns the number of bits consumed so far.
func (r *BitReader) BitPos() int { return r.bitPos }

// Read reads the next n bits, MSB first, and returns them as the low
// n bits of the result. Reads past len(buf)*8 return 0 bits silently;
// callers must ensure the buffer is long enough.
func (r *BitReader) Read(n int) uint16 {
	var out uint16
	for i := 0; i < n; i++ {
		byteIdx := r.bitPos >> 3
		bitIdx := 7 - (r.bitPos & 7)
		var bit uint16
		if byteIdx < len(r.buf) {
			bit = uint16((r.buf[byteIdx] >> uint(bitIdx)) & 1)
		}
		out = (out << 1) | bit
		r.bitPos++
	}
	return out
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run "TestBitReader|TestBitWriter_ReadRoundTrip" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/bitio.go internal/bitstream/bitio_test.go && git commit -m "feat(bitstream): add MSB-first BitReader"
```

---

## Task 4: `Pack` — serialize `Frame` to 10 bytes

**Files:**
- Create: `internal/bitstream/pack.go`
- Create: `internal/bitstream/pack_test.go`

- [x] **Step 1: Write the failing test**

Create `internal/bitstream/pack_test.go`:
```go
package bitstream

import (
	"bytes"
	"errors"
	"testing"
)

func TestPack_AllZero(t *testing.T) {
	var f Frame
	var out [FrameBytes]byte
	if err := Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	var want [FrameBytes]byte
	if !bytes.Equal(out[:], want[:]) {
		t.Errorf("Pack(zero) = % x, want % x", out, want)
	}
}

func TestPack_AllOnesAtMaxValues(t *testing.T) {
	// Set every field to the max its bit width allows.
	f := Frame{
		L0:  1, L1: 0x7F, L2: 0x1F, L3: 0x1F,
		P1:  0xFF, P0: 1,
		C1:  0x1FFF, S1: 0xF, GA1: 7, GB1: 0xF,
		P2:  0x1F,
		C2:  0x1FFF, S2: 0xF, GA2: 7, GB2: 0xF,
	}
	var out [FrameBytes]byte
	if err := Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	// Every bit should be 1, so all bytes are 0xFF.
	for i, b := range out {
		if b != 0xFF {
			t.Errorf("out[%d] = %#x, want 0xFF", i, b)
		}
	}
}

func TestPack_OnlyL0_FirstBit(t *testing.T) {
	f := Frame{L0: 1}
	var out [FrameBytes]byte
	if err := Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	want := [FrameBytes]byte{0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(out[:], want[:]) {
		t.Errorf("Pack(L0=1) = % x, want % x", out, want)
	}
}

func TestPack_OnlyL1_Bits1Through7(t *testing.T) {
	// L1 is a 7-bit field starting at bit 1 of byte 0 (MSB-first).
	// L1 = 0b1010101 -> first byte bits: 0 1 0 1 0 1 0 1 = 0x55.
	f := Frame{L1: 0b1010101}
	var out [FrameBytes]byte
	if err := Pack(&f, out[:]); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	want := [FrameBytes]byte{0x55, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(out[:], want[:]) {
		t.Errorf("Pack(L1=0x55) = % x, want % x", out, want)
	}
}

func TestPack_ShortOutput(t *testing.T) {
	var f Frame
	short := make([]byte, FrameBytes-1)
	if err := Pack(&f, short); !errors.Is(err, ErrShortOutput) {
		t.Errorf("Pack short = %v, want ErrShortOutput", err)
	}
}

func TestPack_ReusesBuffer(t *testing.T) {
	// Pack must clear the destination bytes before writing, so stale
	// bits don't leak through.
	out := make([]byte, FrameBytes)
	for i := range out {
		out[i] = 0xFF
	}
	var f Frame // all zero
	if err := Pack(&f, out); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	for i, b := range out {
		if b != 0 {
			t.Errorf("out[%d] = %#x after Pack(zero), want 0", i, b)
		}
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestPack
```
Expected: compile error — `undefined: Pack`.

- [x] **Step 3: Implement `Pack`**

Create `internal/bitstream/pack.go`:
```go
package bitstream

// Pack serializes f into the first FrameBytes bytes of out. The bytes
// are cleared first, so pre-existing content in out[:FrameBytes] is
// overwritten.
//
// Returns ErrShortOutput if len(out) < FrameBytes. Never allocates.
func Pack(f *Frame, out []byte) error {
	if len(out) < FrameBytes {
		return ErrShortOutput
	}
	buf := out[:FrameBytes]
	for i := range buf {
		buf[i] = 0
	}

	var w BitWriter
	w.Init(buf)
	w.Write(f.L0, 1)
	w.Write(f.L1, 7)
	w.Write(f.L2, 5)
	w.Write(f.L3, 5)
	w.Write(f.P1, 8)
	w.Write(f.P0, 1)
	w.Write(f.C1, 13)
	w.Write(f.S1, 4)
	w.Write(f.GA1, 3)
	w.Write(f.GB1, 4)
	w.Write(f.P2, 5)
	w.Write(f.C2, 13)
	w.Write(f.S2, 4)
	w.Write(f.GA2, 3)
	w.Write(f.GB2, 4)

	return nil
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestPack -v
```
Expected: all subtests pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/pack.go internal/bitstream/pack_test.go && git commit -m "feat(bitstream): add Pack (Frame to 10 bytes)"
```

---

## Task 5: `Unpack` — deserialize 10 bytes to `Frame`

**Files:**
- Modify: `internal/bitstream/pack.go` (append)
- Modify: `internal/bitstream/pack_test.go` (append)

- [x] **Step 1: Append failing tests**

Append to `internal/bitstream/pack_test.go`:
```go
func TestUnpack_AllZero(t *testing.T) {
	var bits [FrameBytes]byte
	var f Frame
	if err := Unpack(bits[:], &f); err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	want := Frame{}
	if f != want {
		t.Errorf("Unpack(zero) = %+v, want %+v", f, want)
	}
}

func TestUnpack_AllOnes(t *testing.T) {
	bits := make([]byte, FrameBytes)
	for i := range bits {
		bits[i] = 0xFF
	}
	var f Frame
	if err := Unpack(bits, &f); err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	want := Frame{
		L0: 1, L1: 0x7F, L2: 0x1F, L3: 0x1F,
		P1: 0xFF, P0: 1,
		C1: 0x1FFF, S1: 0xF, GA1: 7, GB1: 0xF,
		P2: 0x1F,
		C2: 0x1FFF, S2: 0xF, GA2: 7, GB2: 0xF,
	}
	if f != want {
		t.Errorf("Unpack(all ones) = %+v, want %+v", f, want)
	}
}

func TestUnpack_ShortInput(t *testing.T) {
	short := make([]byte, FrameBytes-1)
	var f Frame
	if err := Unpack(short, &f); !errors.Is(err, ErrShortInput) {
		t.Errorf("Unpack short = %v, want ErrShortInput", err)
	}
}

func TestPackUnpack_RoundTrip(t *testing.T) {
	cases := []Frame{
		{},
		{L0: 1, L1: 0x7F, L2: 0x1F, L3: 0x1F,
			P1: 0xFF, P0: 1,
			C1: 0x1FFF, S1: 0xF, GA1: 7, GB1: 0xF,
			P2: 0x1F,
			C2: 0x1FFF, S2: 0xF, GA2: 7, GB2: 0xF,
		},
		{L0: 0, L1: 42, L2: 17, L3: 9, P1: 128, P0: 0,
			C1: 0x1234, S1: 5, GA1: 3, GB1: 11,
			P2: 7, C2: 0x0ABC, S2: 9, GA2: 2, GB2: 6,
		},
	}
	for i, f := range cases {
		var buf [FrameBytes]byte
		if err := Pack(&f, buf[:]); err != nil {
			t.Fatalf("case %d Pack: %v", i, err)
		}
		var got Frame
		if err := Unpack(buf[:], &got); err != nil {
			t.Fatalf("case %d Unpack: %v", i, err)
		}
		if got != f {
			t.Errorf("case %d round-trip: got %+v, want %+v", i, got, f)
		}
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run "TestUnpack|TestPackUnpack_RoundTrip"
```
Expected: compile error — `undefined: Unpack`.

- [x] **Step 3: Implement `Unpack`**

Append to `internal/bitstream/pack.go`:
```go
// Unpack deserializes the first FrameBytes bytes of bits into *f.
//
// Returns ErrShortInput if len(bits) < FrameBytes. Never allocates.
func Unpack(bits []byte, f *Frame) error {
	if len(bits) < FrameBytes {
		return ErrShortInput
	}
	var r BitReader
	r.Init(bits[:FrameBytes])
	f.L0 = r.Read(1)
	f.L1 = r.Read(7)
	f.L2 = r.Read(5)
	f.L3 = r.Read(5)
	f.P1 = r.Read(8)
	f.P0 = r.Read(1)
	f.C1 = r.Read(13)
	f.S1 = r.Read(4)
	f.GA1 = r.Read(3)
	f.GB1 = r.Read(4)
	f.P2 = r.Read(5)
	f.C2 = r.Read(13)
	f.S2 = r.Read(4)
	f.GA2 = r.Read(3)
	f.GB2 = r.Read(4)
	return nil
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run "TestUnpack|TestPackUnpack_RoundTrip" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/pack.go internal/bitstream/pack_test.go && git commit -m "feat(bitstream): add Unpack (10 bytes to Frame)"
```

---

## Task 6: `Parity` helper

`Parity(p1)` returns the XOR of the 6 most-significant bits of `p1` (i.e. `p1 >> 2` treated as a 6-bit field). The encoder calls this to compute `P0`; the decoder may call it to check transmission integrity.

**Files:**
- Create: `internal/bitstream/parity.go`
- Create: `internal/bitstream/parity_test.go`

- [x] **Step 1: Write the failing test**

Create `internal/bitstream/parity_test.go`:
```go
package bitstream

import "testing"

func TestParity(t *testing.T) {
	// Reference values computed manually from the definition
	// P0 = XOR of bits 7..2 of P1.
	tests := []struct {
		p1   uint16
		want uint16
	}{
		{0, 0},                  // upper 6 bits = 0 -> XOR = 0
		{0x03, 0},               // lower 2 bits don't count
		{0x04, 1},               // bit 2 only -> XOR = 1
		{0x08, 1},               // bit 3 only
		{0x0C, 0},               // bits 2+3 -> XOR = 0
		{0xFF, 0},               // upper 6 = 0b111111 -> 6 ones -> XOR = 0
		{0xFC, 0},               // upper 6 = 0b111111 -> XOR = 0
		{0x7C, 1},               // upper 6 = 0b011111 -> 5 ones -> XOR = 1
		{0xA0, 0},               // upper 6 = 0b101000 -> 2 ones -> XOR = 0
		{0xA4, 1},               // upper 6 = 0b101001 -> 3 ones -> XOR = 1
	}
	for _, tc := range tests {
		if got := Parity(tc.p1); got != tc.want {
			t.Errorf("Parity(%#x) = %d, want %d", tc.p1, got, tc.want)
		}
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestParity
```
Expected: compile error — `undefined: Parity`.

- [x] **Step 3: Implement**

Create `internal/bitstream/parity.go`:
```go
package bitstream

// Parity returns the XOR of the 6 most significant bits of p1. The
// result is 0 or 1. This is the value encoders store in Frame.P0 and
// decoders compare against the transmitted P0 to detect errors in the
// pitch-delay field.
func Parity(p1 uint16) uint16 {
	x := (p1 >> 2) & 0x3F
	var p uint16
	for i := 0; i < 6; i++ {
		p ^= (x >> uint(i)) & 1
	}
	return p
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestParity -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/parity.go internal/bitstream/parity_test.go && git commit -m "feat(bitstream): add Parity helper for P0 field"
```

---

## Task 7: G.192 format constants and `WriteG192Frame`

**Files:**
- Create: `internal/bitstream/g192.go`
- Create: `internal/bitstream/g192_test.go`

`WriteG192Frame(w, frame, bad)` takes a 10-byte packed frame and emits `1 sync + 1 length + 80 data = 82` little-endian 16-bit words into `w`.

- [x] **Step 1: Write the failing test**

Create `internal/bitstream/g192_test.go`:
```go
package bitstream

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestG192Constants(t *testing.T) {
	if G192SyncGood != 0x6B21 {
		t.Errorf("G192SyncGood = %#x, want 0x6B21", G192SyncGood)
	}
	if G192SyncBad != 0x6B20 {
		t.Errorf("G192SyncBad = %#x, want 0x6B20", G192SyncBad)
	}
	if G192Bit1 != 0x0081 {
		t.Errorf("G192Bit1 = %#x, want 0x0081", G192Bit1)
	}
	if G192Bit0 != 0x007F {
		t.Errorf("G192Bit0 = %#x, want 0x007F", G192Bit0)
	}
	if G192FrameWords != 2+FrameBits {
		t.Errorf("G192FrameWords = %d, want %d", G192FrameWords, 2+FrameBits)
	}
	if G192FrameBytes != 2*G192FrameWords {
		t.Errorf("G192FrameBytes = %d, want %d", G192FrameBytes, 2*G192FrameWords)
	}
}

func TestWriteG192Frame_AllZero(t *testing.T) {
	var frame [FrameBytes]byte
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, frame[:], false); err != nil {
		t.Fatalf("WriteG192Frame: %v", err)
	}
	// Decode the stream: 82 LE uint16 words.
	if buf.Len() != G192FrameBytes {
		t.Fatalf("buf.Len = %d, want %d", buf.Len(), G192FrameBytes)
	}
	words := make([]uint16, G192FrameWords)
	if err := binary.Read(&buf, binary.LittleEndian, words); err != nil {
		t.Fatalf("binary.Read: %v", err)
	}
	if words[0] != G192SyncGood {
		t.Errorf("sync = %#x, want %#x", words[0], G192SyncGood)
	}
	if words[1] != FrameBits {
		t.Errorf("length = %d, want %d", words[1], FrameBits)
	}
	for i := 2; i < G192FrameWords; i++ {
		if words[i] != G192Bit0 {
			t.Errorf("words[%d] = %#x, want %#x (bit 0)", i, words[i], G192Bit0)
		}
	}
}

func TestWriteG192Frame_BadFlagsSync(t *testing.T) {
	var frame [FrameBytes]byte
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, frame[:], true); err != nil {
		t.Fatalf("WriteG192Frame: %v", err)
	}
	var sync uint16
	if err := binary.Read(&buf, binary.LittleEndian, &sync); err != nil {
		t.Fatalf("binary.Read sync: %v", err)
	}
	if sync != G192SyncBad {
		t.Errorf("sync = %#x, want %#x", sync, G192SyncBad)
	}
}

func TestWriteG192Frame_BitsMatchInput(t *testing.T) {
	// First bit (MSB of byte 0) = 1, rest = 0.
	frame := []byte{0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, frame, false); err != nil {
		t.Fatalf("WriteG192Frame: %v", err)
	}
	words := make([]uint16, G192FrameWords)
	if err := binary.Read(&buf, binary.LittleEndian, words); err != nil {
		t.Fatalf("binary.Read: %v", err)
	}
	if words[2] != G192Bit1 {
		t.Errorf("words[2] = %#x, want %#x (first data bit)", words[2], G192Bit1)
	}
	for i := 3; i < G192FrameWords; i++ {
		if words[i] != G192Bit0 {
			t.Errorf("words[%d] = %#x, want %#x", i, words[i], G192Bit0)
		}
	}
}

func TestWriteG192Frame_ShortFrame(t *testing.T) {
	short := make([]byte, FrameBytes-1)
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, short, false); !errors.Is(err, ErrShortInput) {
		t.Errorf("WriteG192Frame short = %v, want ErrShortInput", err)
	}
}

// Placeholder for stdlib; keeps the test file self-contained even if
// editors auto-import differently.
var _ = io.Discard
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run "TestG192Constants|TestWriteG192Frame"
```
Expected: compile error — undefined symbols.

- [x] **Step 3: Implement**

Create `internal/bitstream/g192.go`:
```go
package bitstream

import (
	"encoding/binary"
	"io"
)

// G.192 serial bitstream word values (ITU-T G.191 STL).
const (
	// G192SyncGood starts a correctly received frame.
	G192SyncGood uint16 = 0x6B21
	// G192SyncBad starts a frame-erasure marker (bad frame).
	G192SyncBad uint16 = 0x6B20
	// G192Bit1 represents a logical 1 source bit.
	G192Bit1 uint16 = 0x0081
	// G192Bit0 represents a logical 0 source bit.
	G192Bit0 uint16 = 0x007F

	// G192FrameWords is the number of 16-bit words per G.192 frame
	// (1 sync + 1 length + FrameBits data words).
	G192FrameWords = 2 + FrameBits
	// G192FrameBytes is the on-disk size of one G.192 frame in bytes
	// (little-endian 16-bit words).
	G192FrameBytes = 2 * G192FrameWords
)

// WriteG192Frame writes one G.192-formatted frame to w. frame must be
// exactly FrameBytes long and hold a packed G.729 frame in the wire
// format produced by Pack. If bad is true, the erasure sync marker is
// emitted instead of the good-frame marker.
//
// Allocates one G192FrameBytes-sized buffer internally.
func WriteG192Frame(w io.Writer, frame []byte, bad bool) error {
	if len(frame) < FrameBytes {
		return ErrShortInput
	}
	words := make([]uint16, G192FrameWords)
	if bad {
		words[0] = G192SyncBad
	} else {
		words[0] = G192SyncGood
	}
	words[1] = FrameBits

	for i := 0; i < FrameBits; i++ {
		byteIdx := i >> 3
		bitIdx := 7 - (i & 7)
		if (frame[byteIdx]>>uint(bitIdx))&1 == 1 {
			words[2+i] = G192Bit1
		} else {
			words[2+i] = G192Bit0
		}
	}

	return binary.Write(w, binary.LittleEndian, words)
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run "TestG192Constants|TestWriteG192Frame" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/g192.go internal/bitstream/g192_test.go && git commit -m "feat(bitstream): add G.192 constants and WriteG192Frame"
```

---

## Task 8: `ReadG192Frame`

**Files:**
- Modify: `internal/bitstream/g192.go` (append)
- Modify: `internal/bitstream/g192_test.go` (append)

`ReadG192Frame(r, frame) (bad, err)` reads one G.192 frame from `r`, validates sync and length words, and fills `frame` with the packed bits. Returns whether the sync indicated a bad (erasure) frame.

- [x] **Step 1: Append failing tests**

Append to `internal/bitstream/g192_test.go`:
```go
func TestReadG192Frame_GoodZeroFrame(t *testing.T) {
	// Build a valid G.192 representation of an all-zero frame.
	words := make([]uint16, G192FrameWords)
	words[0] = G192SyncGood
	words[1] = FrameBits
	for i := 0; i < FrameBits; i++ {
		words[2+i] = G192Bit0
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, words); err != nil {
		t.Fatalf("binary.Write: %v", err)
	}

	var frame [FrameBytes]byte
	bad, err := ReadG192Frame(&buf, frame[:])
	if err != nil {
		t.Fatalf("ReadG192Frame: %v", err)
	}
	if bad {
		t.Errorf("bad = true, want false")
	}
	for i, b := range frame {
		if b != 0 {
			t.Errorf("frame[%d] = %#x, want 0", i, b)
		}
	}
}

func TestReadG192Frame_BadFlagPropagates(t *testing.T) {
	words := make([]uint16, G192FrameWords)
	words[0] = G192SyncBad
	words[1] = FrameBits
	for i := 0; i < FrameBits; i++ {
		words[2+i] = G192Bit0
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, words)

	var frame [FrameBytes]byte
	bad, err := ReadG192Frame(&buf, frame[:])
	if err != nil {
		t.Fatalf("ReadG192Frame: %v", err)
	}
	if !bad {
		t.Errorf("bad = false, want true")
	}
}

func TestReadG192Frame_FirstBitSet(t *testing.T) {
	words := make([]uint16, G192FrameWords)
	words[0] = G192SyncGood
	words[1] = FrameBits
	words[2] = G192Bit1
	for i := 1; i < FrameBits; i++ {
		words[2+i] = G192Bit0
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, words)

	var frame [FrameBytes]byte
	if _, err := ReadG192Frame(&buf, frame[:]); err != nil {
		t.Fatalf("ReadG192Frame: %v", err)
	}
	want := [FrameBytes]byte{0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if frame != want {
		t.Errorf("frame = % x, want % x", frame, want)
	}
}

func TestReadG192Frame_BadSync(t *testing.T) {
	words := make([]uint16, G192FrameWords)
	words[0] = 0xFFFF
	words[1] = FrameBits
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, words)

	var frame [FrameBytes]byte
	if _, err := ReadG192Frame(&buf, frame[:]); !errors.Is(err, ErrBadG192Sync) {
		t.Errorf("err = %v, want ErrBadG192Sync", err)
	}
}

func TestReadG192Frame_BadLength(t *testing.T) {
	words := make([]uint16, G192FrameWords)
	words[0] = G192SyncGood
	words[1] = 40 // not FrameBits
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, words)

	var frame [FrameBytes]byte
	if _, err := ReadG192Frame(&buf, frame[:]); !errors.Is(err, ErrBadG192Length) {
		t.Errorf("err = %v, want ErrBadG192Length", err)
	}
}

func TestReadG192Frame_BadDataWord(t *testing.T) {
	words := make([]uint16, G192FrameWords)
	words[0] = G192SyncGood
	words[1] = FrameBits
	words[2] = 0xDEAD
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, words)

	var frame [FrameBytes]byte
	if _, err := ReadG192Frame(&buf, frame[:]); !errors.Is(err, ErrBadG192Bit) {
		t.Errorf("err = %v, want ErrBadG192Bit", err)
	}
}

func TestReadG192Frame_EOF(t *testing.T) {
	var empty bytes.Buffer
	var frame [FrameBytes]byte
	if _, err := ReadG192Frame(&empty, frame[:]); !errors.Is(err, io.EOF) {
		t.Errorf("err = %v, want io.EOF", err)
	}
}

func TestG192RoundTrip(t *testing.T) {
	original := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}
	var buf bytes.Buffer
	if err := WriteG192Frame(&buf, original, false); err != nil {
		t.Fatalf("WriteG192Frame: %v", err)
	}
	var got [FrameBytes]byte
	bad, err := ReadG192Frame(&buf, got[:])
	if err != nil {
		t.Fatalf("ReadG192Frame: %v", err)
	}
	if bad {
		t.Errorf("bad = true, want false")
	}
	if !bytes.Equal(got[:], original) {
		t.Errorf("round-trip: got % x, want % x", got, original)
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run "TestReadG192Frame|TestG192RoundTrip"
```
Expected: compile error — `undefined: ReadG192Frame`.

- [x] **Step 3: Implement**

Append to `internal/bitstream/g192.go`:
```go
// ReadG192Frame reads one G.192 frame from r. frame must be at least
// FrameBytes long and is overwritten with the packed bit pattern.
// Returns bad = true if the sync word indicated a frame erasure.
//
// Returns io.EOF if the reader is empty at the start of a frame, or
// io.ErrUnexpectedEOF if the reader ends mid-frame. Returns
// ErrBadG192Sync / ErrBadG192Length / ErrBadG192Bit if the stream
// content does not match the G.192 conventions.
//
// Allocates one G192FrameBytes-sized buffer internally.
func ReadG192Frame(r io.Reader, frame []byte) (bool, error) {
	if len(frame) < FrameBytes {
		return false, ErrShortOutput
	}
	words := make([]uint16, G192FrameWords)
	if err := binary.Read(r, binary.LittleEndian, words); err != nil {
		return false, err
	}

	var bad bool
	switch words[0] {
	case G192SyncGood:
		bad = false
	case G192SyncBad:
		bad = true
	default:
		return false, ErrBadG192Sync
	}
	if words[1] != FrameBits {
		return false, ErrBadG192Length
	}

	out := frame[:FrameBytes]
	for i := range out {
		out[i] = 0
	}
	for i := 0; i < FrameBits; i++ {
		switch words[2+i] {
		case G192Bit1:
			byteIdx := i >> 3
			bitIdx := 7 - (i & 7)
			out[byteIdx] |= 1 << uint(bitIdx)
		case G192Bit0:
			// nothing to set
		default:
			return false, ErrBadG192Bit
		}
	}
	return bad, nil
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run "TestReadG192Frame|TestG192RoundTrip" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/g192.go internal/bitstream/g192_test.go && git commit -m "feat(bitstream): add ReadG192Frame"
```

---

## Task 9: `ReadG192File` — decode a whole `.bit` file to packed frames

**Files:**
- Modify: `internal/bitstream/g192.go` (append)
- Modify: `internal/bitstream/g192_test.go` (append)

A convenience wrapper: read until EOF, returning a slice of packed frames and a parallel slice of bad-flags. This is what Phase 2 encoder tests will use to load ITU `.bit` files.

- [x] **Step 1: Append failing test**

Append to `internal/bitstream/g192_test.go`:
```go
func TestReadG192File_MultipleFrames(t *testing.T) {
	frames := [][]byte{
		{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A},
		{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80, 0x90, 0xA0},
	}
	bads := []bool{false, true}

	var buf bytes.Buffer
	for i, f := range frames {
		if err := WriteG192Frame(&buf, f, bads[i]); err != nil {
			t.Fatalf("WriteG192Frame[%d]: %v", i, err)
		}
	}

	gotFrames, gotBads, err := ReadG192File(&buf)
	if err != nil {
		t.Fatalf("ReadG192File: %v", err)
	}
	if len(gotFrames) != len(frames) {
		t.Fatalf("frame count = %d, want %d", len(gotFrames), len(frames))
	}
	for i := range frames {
		if !bytes.Equal(gotFrames[i], frames[i]) {
			t.Errorf("frame[%d] = % x, want % x", i, gotFrames[i], frames[i])
		}
		if gotBads[i] != bads[i] {
			t.Errorf("bad[%d] = %v, want %v", i, gotBads[i], bads[i])
		}
	}
}

func TestReadG192File_Empty(t *testing.T) {
	var buf bytes.Buffer
	frames, bads, err := ReadG192File(&buf)
	if err != nil {
		t.Fatalf("ReadG192File empty: %v", err)
	}
	if len(frames) != 0 || len(bads) != 0 {
		t.Errorf("empty file -> %d frames, want 0", len(frames))
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestReadG192File
```
Expected: compile error — `undefined: ReadG192File`.

- [x] **Step 3: Implement**

Append to `internal/bitstream/g192.go`:
```go
// ReadG192File reads G.192 frames from r until EOF. It returns a slice
// of packed frame bytes (each FrameBytes long) and a parallel slice of
// bad-frame flags. A clean EOF at a frame boundary terminates reading
// normally. A truncated last frame returns io.ErrUnexpectedEOF.
//
// Intended for loading ITU test-vector .bit files, not for the hot
// path: it allocates one backing buffer for the full output.
func ReadG192File(r io.Reader) ([][]byte, []bool, error) {
	var frames [][]byte
	var bads []bool
	for {
		frame := make([]byte, FrameBytes)
		bad, err := ReadG192Frame(r, frame)
		if err == io.EOF {
			return frames, bads, nil
		}
		if err != nil {
			return frames, bads, err
		}
		frames = append(frames, frame)
		bads = append(bads, bad)
	}
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestReadG192File -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/g192.go internal/bitstream/g192_test.go && git commit -m "feat(bitstream): add ReadG192File convenience reader"
```

---

## Task 10: Zero-allocation assertions on hot-path functions

**Files:**
- Create: `internal/bitstream/alloc_test.go`

Pack, Unpack, and Parity are called per frame by the encoder/decoder; they must not allocate. G.192 I/O is not hot-path and is allowed to allocate.

- [x] **Step 1: Write the test**

Create `internal/bitstream/alloc_test.go`:
```go
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
```

- [x] **Step 2: Run the test**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -run TestNoAllocation -v
```
Expected: all pass (Pack/Unpack/Parity each 0 allocs).

- [x] **Step 3: Run the full package suite with race detector and vet**

```bash
cd /home/exedev/g729 && go test ./... -race && go vet ./...
```
Expected: all tests pass, `go vet` clean.

- [x] **Step 4: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/alloc_test.go && git commit -m "test(bitstream): assert zero allocation on hot-path functions"
```

---

## Task 11: Benchmarks

**Files:**
- Create: `internal/bitstream/bench_test.go`

- [x] **Step 1: Write benchmarks**

Create `internal/bitstream/bench_test.go`:
```go
package bitstream

import (
	"bytes"
	"testing"
)

func BenchmarkPack(b *testing.B) {
	f := Frame{
		L0: 1, L1: 64, L2: 15, L3: 9,
		P1: 120, P0: 1,
		C1: 0x1ABC, S1: 5, GA1: 3, GB1: 11,
		P2: 15, C2: 0x0DEF, S2: 9, GA2: 2, GB2: 6,
	}
	var out [FrameBytes]byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Pack(&f, out[:])
	}
}

func BenchmarkUnpack(b *testing.B) {
	bits := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}
	var f Frame
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Unpack(bits, &f)
	}
}

func BenchmarkParity(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Parity(uint16(i))
	}
}

func BenchmarkWriteG192Frame(b *testing.B) {
	frame := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}
	var buf bytes.Buffer
	buf.Grow(G192FrameBytes)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = WriteG192Frame(&buf, frame, false)
	}
}
```

- [x] **Step 2: Run benchmarks once (informational)**

```bash
cd /home/exedev/g729 && go test ./internal/bitstream/... -bench=. -benchmem -run=^$
```
Expected: Pack/Unpack/Parity report `0 B/op, 0 allocs/op`. WriteG192Frame will show a small alloc per call (documented behavior).

- [x] **Step 3: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/bench_test.go && git commit -m "test(bitstream): add Pack/Unpack/G192 benchmarks"
```

---

## Task 12: Package documentation polish

**Files:**
- Modify: `internal/bitstream/doc.go`

- [x] **Step 1: Rewrite `internal/bitstream/doc.go`**

Replace the contents of `internal/bitstream/doc.go`:
```go
// Package bitstream is the boundary between the G.729 codec's logical
// frame parameters and their wire representation.
//
// # Layers
//
// The package exposes three layers, each independently useful:
//
//  1. Frame struct — one G.729 frame as 15 named integer parameters
//     (L0..GB2) in the ITU-T transmission order.
//  2. Pack / Unpack — convert between Frame and the 10-byte packed
//     bitstream actually transmitted over RTP. Zero-allocation, safe
//     to call on every encoder / decoder frame.
//  3. G.192 file I/O — WriteG192Frame / ReadG192Frame / ReadG192File
//     read and write the ITU-T G.192 serial bitstream (.bit) format
//     used by the official G.729 test vectors. Not on the hot path;
//     allocates a small frame-size buffer per call.
//
// # Wire byte / bit ordering
//
// Within each byte, the most significant bit is transmitted first.
// Within each parameter, the most significant bit is transmitted
// first. Parameters appear in the order of the Frame struct fields
// (L0 first, GB2 last).
//
// # G.192 file format
//
// Each frame on disk is 82 little-endian 16-bit words: sync (0x6B21 or
// 0x6B20), length (= FrameBits = 80), then one word per source bit
// (0x0081 for 1, 0x007F for 0).
//
// # Parity
//
// The P0 bit is the XOR of the 6 most-significant bits of P1. Use
// Parity to compute or validate it.
//
// # References
//
//   - ITU-T G.729, section on parameter transmission (frame bit
//     allocations).
//   - ITU-T G.729 Annex A, reduced-complexity variant; bitstream
//     layout is identical.
//   - ITU-T G.191 STL, serial bitstream (G.192) format definition.
package bitstream
```

- [x] **Step 2: Render godoc locally**

```bash
cd /home/exedev/g729 && go doc ./internal/bitstream
```
Expected: the sections above render.

- [x] **Step 3: Run the full suite once more**

```bash
cd /home/exedev/g729 && go test ./... -race && go vet ./...
```
Expected: all pass, vet clean.

- [x] **Step 4: Commit**

```bash
cd /home/exedev/g729 && git add internal/bitstream/doc.go && git commit -m "docs(bitstream): expand package doc with layers and ordering"
```

---

## Completion criteria

- All 12 tasks above are committed in order.
- `go test ./... -race` passes.
- `go vet ./...` emits nothing.
- `go test ./internal/bitstream/... -bench=. -benchmem` reports 0 allocs for Pack, Unpack, and Parity.
- Pack + Unpack is a symmetric round-trip for every allowed Frame (tested with random + boundary cases).
- `WriteG192Frame` followed by `ReadG192Frame` is a round-trip, including the bad-frame flag.

When these hold, Phase 0b is complete. The next plan (Phase 0c) will implement `internal/pcm` — the high-pass pre-processing filter and the int16 <-> Q-format scaling helpers that sit between the outside world and the DSP core. It depends on `internal/fixed` (Phase 0a) for arithmetic but not on this package.
