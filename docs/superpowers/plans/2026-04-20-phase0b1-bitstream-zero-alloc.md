# Phase 0b.1 — Bitstream G.192 Zero-Allocation Refactor Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `WriteG192Frame` and `ReadG192Frame` in `internal/bitstream` so both functions report `0 allocs/op` in benchmarks, without changing their public signatures or observable behavior. Bring the G.192 I/O path into line with the original zero-allocation contract the user requested during brainstorming.

**Architecture:** Replace the per-call `make([]uint16, G192FrameWords)` + `binary.{Write,Read}` path with a stack-resident fixed-size `[G192FrameBytes]byte` buffer. Convert individual 16-bit words with `binary.LittleEndian.{PutUint16,Uint16}` (both are zero-alloc value-level calls) and submit the byte buffer as a single `io.Writer.Write` or `io.ReadFull` call. `ReadG192File` is a documented test-only helper and stays allocating — explicitly out of scope.

**Tech Stack:** Go 1.22+, standard library only (`encoding/binary`, `io`, `errors`). Scratch-from-spec — do not consult ITU reference C, bcg729, Sipro Lab, or any other G.729 implementation. The G.192 format details are taken from ITU-T G.191 STL documentation, not from code.

---

## Context for the implementing engineer

### Why this refactor exists

Phase 0b implemented `WriteG192Frame` and `ReadG192Frame` using `binary.Write` / `binary.Read` on a freshly-allocated `[]uint16` slice per call. That works, but `binary.Write` on a slice uses reflection and forces a heap allocation, and the `make([]uint16, …)` itself allocates. Benchmarks show `352 B/op, 2 allocs/op` for `WriteG192Frame`; `ReadG192Frame` is the same shape.

The project's original zero-allocation contract (established during brainstorming) covers *all* streaming wire-format I/O, not just `Pack` / `Unpack`. The Phase 0b plan narrowed the contract to `Pack` / `Unpack` / `Parity` and marked G.192 I/O as allocating. That narrowing was reverted by the user: G.192 I/O must also be zero-alloc. `ReadG192File` is the one intentional exception because it returns a slice of frames — the outer loop must allocate by nature.

### Why a fixed-size byte buffer works

One G.192 frame is exactly `G192FrameBytes = 164` bytes on disk (82 little-endian `uint16` words). `var buf [G192FrameBytes]byte` inside a function body is a stack array — no heap allocation, no escape (the address is never stored beyond the function's lifetime). `binary.LittleEndian.PutUint16(buf[off:off+2], v)` and `binary.LittleEndian.Uint16(buf[off:off+2])` are scalar calls that do not allocate. `io.Writer.Write(buf[:])` takes a slice header, which does not force `buf` to escape because it is consumed synchronously and not retained.

The `PutUint16` / `Uint16` approach also happens to be measurably faster than `binary.Write` because it avoids reflection, so the refactor improves throughput in addition to eliminating allocations.

### Scope — what this plan does and does not change

**In scope:**
- `WriteG192Frame(w io.Writer, frame []byte, bad bool) error` — replace implementation body.
- `ReadG192Frame(r io.Reader, frame []byte) (bool, error)` — replace implementation body.
- Expand the existing `alloc_test.go` to cover both functions.
- Add matching benchmark for `ReadG192Frame` (currently missing).

**Out of scope:**
- Public API / signature changes. Callers must not need edits.
- `ReadG192File` — documented as test-only and allowed to allocate.
- `Pack` / `Unpack` / `Parity` — already 0 allocs/op; no changes.
- `BitWriter` / `BitReader` — unaffected.
- Any non-bitstream package.

### Files touched by this plan

| File | Change |
|---|---|
| `internal/bitstream/g192.go` | Replace bodies of `WriteG192Frame` and `ReadG192Frame`; update their doc comments to drop the "Allocates one G192FrameBytes-sized buffer internally" sentence. |
| `internal/bitstream/alloc_test.go` | Extend existing `TestNoAllocation_PackUnpackParity` (or add companion) to cover `WriteG192Frame` and `ReadG192Frame`. |
| `internal/bitstream/bench_test.go` | Add `BenchmarkReadG192Frame`; leave existing `BenchmarkWriteG192Frame` (it will flip from `352 B/op, 2 allocs/op` to `0 B/op, 0 allocs/op`). |
| `docs/superpowers/plans/2026-04-20-phase0b1-bitstream-zero-alloc.md` | This plan — mark checkboxes as tasks are completed. |

### Verification commands

After every task:
- `go test ./internal/bitstream/... -race` — must PASS with no new failures.
- `go vet ./internal/bitstream/...` — must print nothing.

At the end of the plan:
- `go test -run TestNoAllocation -v ./internal/bitstream/...` — must show `Write` and `Read` cases passing.
- `go test -bench=. -benchmem -run=^$ ./internal/bitstream/...` — `BenchmarkWriteG192Frame` and `BenchmarkReadG192Frame` must both print `0 B/op	       0 allocs/op`.

---

## Task 1: Extend the zero-allocation assertion to cover WriteG192Frame and ReadG192Frame (failing tests first)

**Files:**
- Modify: `internal/bitstream/alloc_test.go`

We add the two new cases first and watch them fail on the current implementation. This is the red step of the TDD cycle.

- [ ] **Step 1: Add the failing test cases**

Open `internal/bitstream/alloc_test.go` and replace the file contents with:

```go
package bitstream

import (
	"bytes"
	"io"
	"testing"
)

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

func TestNoAllocation_G192IO(t *testing.T) {
	frame := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}

	// Pre-build a valid G.192 frame we can read back repeatedly.
	var encoded bytes.Buffer
	encoded.Grow(G192FrameBytes)
	if err := WriteG192Frame(&encoded, frame, false); err != nil {
		t.Fatalf("WriteG192Frame setup: %v", err)
	}
	encodedBytes := encoded.Bytes()

	// discardWriter is an io.Writer that never allocates; bytes.Buffer may
	// grow on the first call, so we re-use a pre-sized buffer and Reset it
	// between iterations.
	var writeBuf bytes.Buffer
	writeBuf.Grow(G192FrameBytes)

	var readBuf [FrameBytes]byte

	writeFn := func() {
		writeBuf.Reset()
		_ = WriteG192Frame(&writeBuf, frame, false)
	}
	readFn := func() {
		r := bytes.NewReader(encodedBytes)
		_, _ = ReadG192Frame(r, readBuf[:])
	}

	cases := []struct {
		name string
		fn   func()
	}{
		{"WriteG192Frame", writeFn},
		{"ReadG192Frame", readFn},
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

// Compile-time assertion that bytes.NewReader still satisfies io.Reader.
var _ io.Reader = (*bytes.Reader)(nil)
```

Note: `bytes.NewReader(encodedBytes)` creates a `*bytes.Reader` which is 24 bytes on 64-bit platforms. `AllocsPerRun` measures heap allocations inside the function; `bytes.NewReader` returns a pointer, but the escape analysis may or may not stack-allocate the `bytes.Reader` depending on whether `ReadG192Frame`'s parameter is inlined. If `bytes.NewReader` escapes to the heap, the test would report 1 alloc not attributable to our code. If that happens in Step 3 verification, swap `bytes.NewReader` for a small reusable struct (see the fallback note below).

- [ ] **Step 2: Run the new assertion and watch it fail**

Run:
```
go test -run TestNoAllocation_G192IO -v ./internal/bitstream/...
```

Expected: both `WriteG192Frame` and `ReadG192Frame` sub-tests FAIL with output of the form `WriteG192Frame allocated 2.00 times per call, want 0` (or similar non-zero count). This confirms the baseline matches the completion report.

- [ ] **Step 3: Commit the red test**

```bash
git add internal/bitstream/alloc_test.go
git commit -m "test(bitstream): assert zero allocation on G.192 frame I/O"
```

**Fallback if the `bytes.NewReader` itself allocates inside the benchmarked function:**
If Step 2 shows `ReadG192Frame` reporting `1.00 allocs/op` even after Task 2's refactor (because the test-local `bytes.NewReader` escapes), switch the test to use a `bytes.Reader` by value reset between iterations:

```go
var br bytes.Reader
readFn := func() {
    br.Reset(encodedBytes)
    _, _ = ReadG192Frame(&br, readBuf[:])
}
```

`bytes.Reader{}` is stack-allocated; only the pointer passed to `ReadG192Frame` is taken, and `io.Reader` method calls do not normally cause escape. This is the canonical zero-alloc test pattern in the standard library.

---

## Task 2: Refactor WriteG192Frame to use a stack-allocated byte buffer

**Files:**
- Modify: `internal/bitstream/g192.go:27-56` (function `WriteG192Frame`)

- [ ] **Step 1: Replace the function body**

In `internal/bitstream/g192.go`, replace the entire `WriteG192Frame` function (the doc comment plus the body — lines starting with `// WriteG192Frame writes ...` through the closing `}`) with the following:

```go
// WriteG192Frame writes one G.192-formatted frame to w. frame must be
// exactly FrameBytes long and hold a packed G.729 frame in the wire
// format produced by Pack. If bad is true, the erasure sync marker is
// emitted instead of the good-frame marker.
//
// Zero-allocation: the implementation serializes through a
// G192FrameBytes-sized stack buffer and performs a single w.Write call.
func WriteG192Frame(w io.Writer, frame []byte, bad bool) error {
	if len(frame) < FrameBytes {
		return ErrShortInput
	}

	var buf [G192FrameBytes]byte

	if bad {
		binary.LittleEndian.PutUint16(buf[0:2], G192SyncBad)
	} else {
		binary.LittleEndian.PutUint16(buf[0:2], G192SyncGood)
	}
	binary.LittleEndian.PutUint16(buf[2:4], FrameBits)

	for i := 0; i < FrameBits; i++ {
		byteIdx := i >> 3
		bitIdx := 7 - (i & 7)
		word := G192Bit0
		if (frame[byteIdx]>>uint(bitIdx))&1 == 1 {
			word = G192Bit1
		}
		// Data words begin at byte offset 4 (after sync + length).
		off := 4 + 2*i
		binary.LittleEndian.PutUint16(buf[off:off+2], word)
	}

	_, err := w.Write(buf[:])
	return err
}
```

The key differences from the previous implementation:
- `var buf [G192FrameBytes]byte` is a fixed-size array on the stack. No `make`, no heap.
- Each uint16 is serialized directly to its byte offset with `binary.LittleEndian.PutUint16`. This is a non-reflective scalar call.
- A single `w.Write(buf[:])` delivers the whole frame. Passing `buf[:]` to `Write` does not force `buf` to escape because the callee does not retain the slice.
- `FrameBits` is a `uint16` constant (`80`) so it can be passed directly to `PutUint16`.

- [ ] **Step 2: Run the package tests**

Run:
```
go test ./internal/bitstream/... -race
```

Expected: all existing tests still PASS (pack/unpack round-trip, G.192 round-trip, error paths). `TestNoAllocation_G192IO/ReadG192Frame` still FAILS (we haven't refactored Read yet), but `TestNoAllocation_G192IO/WriteG192Frame` now PASSES.

Verify the Write sub-test specifically:
```
go test -run TestNoAllocation_G192IO/WriteG192Frame -v ./internal/bitstream/...
```
Expected: PASS.

- [ ] **Step 3: Run the existing benchmark and confirm allocation dropped to zero**

Run:
```
go test -bench=BenchmarkWriteG192Frame -benchmem -run=^$ ./internal/bitstream/...
```

Expected output line shape:
```
BenchmarkWriteG192Frame-<N>   <ops>    <ns> ns/op    0 B/op    0 allocs/op
```

If any allocation remains, stop and investigate before committing. Most likely cause: the `bytes.Buffer` in the benchmark not being pre-grown — that would be a test-harness allocation, not a production allocation; check by temporarily writing to `io.Discard` and re-benchmarking.

- [ ] **Step 4: `go vet`**

Run:
```
go vet ./internal/bitstream/...
```
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/bitstream/g192.go
git commit -m "perf(bitstream): make WriteG192Frame zero-allocation"
```

---

## Task 3: Refactor ReadG192Frame to use a stack-allocated byte buffer

**Files:**
- Modify: `internal/bitstream/g192.go:58-107` (function `ReadG192Frame`)

- [ ] **Step 1: Replace the function body**

In `internal/bitstream/g192.go`, replace the entire `ReadG192Frame` function (doc comment + body, from `// ReadG192Frame reads ...` through the closing `}` of `ReadG192Frame`) with the following:

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
// Zero-allocation: the implementation reads into a G192FrameBytes-sized
// stack buffer and decodes in place.
func ReadG192Frame(r io.Reader, frame []byte) (bool, error) {
	if len(frame) < FrameBytes {
		return false, ErrShortOutput
	}

	var buf [G192FrameBytes]byte

	// First, try to detect a clean EOF at a frame boundary so the caller
	// can terminate iteration cleanly. We do this by reading one byte and
	// then filling the rest; if the first read returns io.EOF we return
	// io.EOF unchanged, otherwise any subsequent short read is an
	// unexpected EOF.
	n, err := io.ReadFull(r, buf[:])
	switch {
	case err == io.EOF:
		return false, io.EOF
	case err == io.ErrUnexpectedEOF:
		// Partial frame at stream end — propagate.
		return false, io.ErrUnexpectedEOF
	case err != nil:
		return false, err
	case n != G192FrameBytes:
		// Defensive: ReadFull guarantees n == len(buf) when err == nil,
		// but guard anyway.
		return false, io.ErrUnexpectedEOF
	}

	sync := binary.LittleEndian.Uint16(buf[0:2])
	var bad bool
	switch sync {
	case G192SyncGood:
		bad = false
	case G192SyncBad:
		bad = true
	default:
		return false, ErrBadG192Sync
	}

	if binary.LittleEndian.Uint16(buf[2:4]) != FrameBits {
		return false, ErrBadG192Length
	}

	out := frame[:FrameBytes]
	for i := range out {
		out[i] = 0
	}
	for i := 0; i < FrameBits; i++ {
		word := binary.LittleEndian.Uint16(buf[4+2*i : 4+2*i+2])
		switch word {
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

Notes on the EOF handling:
- `io.ReadFull` documents: "Reads 0 bytes → returns io.EOF. Reads some but not all → returns io.ErrUnexpectedEOF." That matches the semantics the existing tests (and `ReadG192File`) rely on, so the switch above preserves behavior.
- We return `io.EOF` *only* when no bytes at all were available at the start of the frame. This is what lets `ReadG192File` terminate iteration cleanly.

- [ ] **Step 2: Run all package tests**

Run:
```
go test ./internal/bitstream/... -race
```

Expected: all tests PASS, including `TestNoAllocation_G192IO/ReadG192Frame`.

- [ ] **Step 3: Verify the allocation assertion directly**

Run:
```
go test -run TestNoAllocation_G192IO -v ./internal/bitstream/...
```
Expected: both sub-tests `WriteG192Frame` and `ReadG192Frame` PASS.

If `ReadG192Frame` still reports a non-zero allocation count, apply the Task 1 Step 3 fallback (use `var br bytes.Reader; br.Reset(encodedBytes)`) in the test and re-run. `bytes.Reader` has well-known zero-alloc behavior when used this way; `bytes.NewReader` allocates 24 bytes on the heap if escape analysis decides the pointer escapes.

- [ ] **Step 4: `go vet`**

Run:
```
go vet ./internal/bitstream/...
```
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add internal/bitstream/g192.go
git commit -m "perf(bitstream): make ReadG192Frame zero-allocation"
```

---

## Task 4: Add a matching ReadG192Frame benchmark and run the full bench suite

**Files:**
- Modify: `internal/bitstream/bench_test.go`

- [ ] **Step 1: Add the benchmark**

Open `internal/bitstream/bench_test.go` and append the following function to the end of the file:

```go
func BenchmarkReadG192Frame(b *testing.B) {
	frame := []byte{0xAA, 0x55, 0x01, 0x80, 0xFF, 0x00, 0x12, 0x34, 0x56, 0x78}

	var encoded bytes.Buffer
	encoded.Grow(G192FrameBytes)
	if err := WriteG192Frame(&encoded, frame, false); err != nil {
		b.Fatalf("setup: %v", err)
	}
	encodedBytes := encoded.Bytes()

	var out [FrameBytes]byte
	var r bytes.Reader

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r.Reset(encodedBytes)
		if _, err := ReadG192Frame(&r, out[:]); err != nil {
			b.Fatalf("iter %d: %v", i, err)
		}
	}
}
```

- [ ] **Step 2: Run all benchmarks with -benchmem and record the numbers**

Run:
```
go test -bench=. -benchmem -run=^$ ./internal/bitstream/...
```

Expected for the four hot-path benchmarks (ns/op will vary by host):
```
BenchmarkPack-<N>             ...    0 B/op    0 allocs/op
BenchmarkUnpack-<N>           ...    0 B/op    0 allocs/op
BenchmarkParity-<N>           ...    0 B/op    0 allocs/op
BenchmarkWriteG192Frame-<N>   ...    0 B/op    0 allocs/op
BenchmarkReadG192Frame-<N>    ...    0 B/op    0 allocs/op
```

Any non-zero `B/op` or `allocs/op` on a bench other than a future `ReadG192File` bench (not added here) means the refactor is incomplete — stop and diagnose.

- [ ] **Step 3: Commit**

```bash
git add internal/bitstream/bench_test.go
git commit -m "test(bitstream): add ReadG192Frame benchmark"
```

---

## Task 5: Mark the follow-up in the original Phase 0b plan (bookkeeping)

**Files:**
- Modify: `docs/superpowers/plans/2026-04-20-phase0b-bitstream.md` (append a one-line note only)
- Modify: `docs/superpowers/plans/2026-04-20-phase0b-bitstream-completion-report.md` (append a "Resolved" note)

- [ ] **Step 1: Append the resolution note to the completion report**

In `docs/superpowers/plans/2026-04-20-phase0b-bitstream-completion-report.md`, append at the end of the file:

```markdown

---

## Resolved 2026-04-20 — G.192 I/O is now zero-allocation

The allocation issue flagged in the "주의 사항" section above was closed by the Phase 0b.1 refactor (`docs/superpowers/plans/2026-04-20-phase0b1-bitstream-zero-alloc.md`). `WriteG192Frame` and `ReadG192Frame` both report `0 B/op, 0 allocs/op`; `ReadG192File` intentionally still allocates because it returns a slice of frames.
```

- [ ] **Step 2: Verify plans and reports compile as plain Markdown**

Run:
```
go test ./internal/bitstream/... -race && go vet ./internal/bitstream/...
```
Expected: clean. (No markdown linter is wired up, so we verify via the code suite which is the only hard gate.)

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/plans/2026-04-20-phase0b-bitstream-completion-report.md
git commit -m "docs(plans): note Phase 0b.1 resolves G.192 I/O allocation"
```

---

## Completion criteria

- [ ] `go test ./internal/bitstream/... -race` PASSES.
- [ ] `go vet ./internal/bitstream/...` prints nothing.
- [ ] `go test -run TestNoAllocation -v ./internal/bitstream/...` PASSES for all five sub-tests (`Pack`, `Unpack`, `Parity`, `WriteG192Frame`, `ReadG192Frame`).
- [ ] `go test -bench=. -benchmem -run=^$ ./internal/bitstream/...` shows `0 B/op, 0 allocs/op` for all five benchmarks (`Pack`, `Unpack`, `Parity`, `WriteG192Frame`, `ReadG192Frame`).
- [ ] No public API signature change in `internal/bitstream` — callers elsewhere (when they exist in later phases) require no edits.
- [ ] `ReadG192File` remains intentionally allocating; no refactor needed.
- [ ] Plan checkboxes above are all marked `[x]`.
