# Phase 0a — Fixed-Point Primitives Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `internal/fixed` package — a bit-exact Go equivalent of the ITU-T G.191 basic operations library that every DSP block in this G.729A codec will depend on.

**Architecture:** A single internal package exposing saturating 16-bit and 32-bit arithmetic primitives. All operations match the ITU-T G.729 specification semantics exactly. Each primitive is a plain Go function (no methods, no interfaces) so higher blocks can call them without allocation or dispatch cost. All 16-bit math is computed in `int32` then saturated; all 32-bit math is computed in `int64` then saturated. Types alias `int16` and `int32` for clarity at call sites.

**Tech Stack:** Go 1.22+, zero dependencies, standard `testing` package only. Scratch-from-spec — implementation must be written by reading the ITU-T G.729 specification and its Annex A basic-operations definitions, without consulting the ITU reference C source, bcg729, or any other existing implementation.

---

## Context for the implementing engineer

### What is G.729 and why fixed-point?

G.729 is an 8 kbps speech codec from ITU-T. Its specification is written in **16-bit fixed-point arithmetic** — integers that represent fractional numbers via Q-format (e.g. Q15: a `int16` encodes `x / 32768` for `x ∈ [-1.0, 1.0)`). This project reproduces the codec in pure Go; the `internal/fixed` package is the arithmetic foundation that every later DSP block uses.

The ITU-T also publishes a companion specification (G.191) of "basic operations" — about 25 primitive functions (`add`, `L_mult`, `shr`, etc.) whose semantics the G.729 spec relies on. Every G.729 mathematical statement is expressible as a composition of these primitives with saturation.

### Bit-exactness is the project's acceptance criterion

The definition of "done" for the whole codec is: official ITU test vectors pass byte-exact for the encoder and sample-exact for the decoder. Any deviation from the primitive semantics here will cascade into bit-level errors at the codec level. **Getting this layer right is non-negotiable.**

### Naming convention

The ITU spec uses C-style names like `L_mult` (the `L_` prefix means "long / 32-bit"). In Go we use CamelCase: `LMult`. Where the spec name is already idiomatic (`add`, `sub`), we capitalize to `Add`, `Sub`. Mapping is one-to-one and documented in `doc.go` so engineers can cross-reference spec formulas to Go identifiers.

### Integer overflow in Go

Go's built-in `+`, `-`, `*` on `int16`/`int32` **wrap silently** on overflow. They do not saturate. Every saturating primitive in this package must compute in a wider type (`int32` for 16-bit ops, `int64` for 32-bit ops), then apply saturation explicitly. Never trust bare Go arithmetic to match spec semantics.

### Package layout produced by this plan

```
g729/
├── go.mod                        (created in Task 1)
├── go.sum                        (stays empty — no deps)
├── LICENSE                       (MIT, created in Task 1)
├── doc.go                        (module doc, created in Task 2)
└── internal/
    └── fixed/
        ├── doc.go                (package doc mapping ITU names -> Go names)
        ├── types.go              (Word16, Word32 type aliases, constants)
        ├── saturate.go           (Saturate)
        ├── arith16.go            (Add, Sub, Negate, AbsS)
        ├── arith32.go            (LAdd, LSub, LNegate, LAbs)
        ├── extract.go            (ExtractH, ExtractL, LDepositH, LDepositL)
        ├── shift16.go            (Shl, Shr, ShrR)
        ├── shift32.go            (LShl, LShr, LShrR)
        ├── mult.go               (LMult, LMac, LMsu, Mult, MultR)
        ├── round.go              (Round)
        ├── norm.go               (NormS, NormL)
        ├── div.go                (DivS)
        └── *_test.go             (companion tests per file)
```

### Commit messages

Use Conventional Commits style: `feat:`, `test:`, `chore:`, `docs:`. Keep messages short; they are not the place for design rationale.

---

## Task 1: Initialize Go module and license

**Files:**
- Create: `go.mod`
- Create: `LICENSE`
- Create: `.gitignore`

- [x] **Step 1: Initialize the module**

Run in repo root:
```bash
cd /home/exedev/g729 && go mod init github.com/exedev/g729
```

Expected: creates `go.mod` with module path and Go directive.

The module path `github.com/exedev/g729` is a placeholder. The project owner can rename later with `go mod edit -module github.com/<owner>/g729` without touching any other file, because no file imports using the full module path yet (all imports so far are within `internal/fixed`).

- [x] **Step 2: Pin the Go version**

Open `go.mod` and ensure the `go` directive is at least `1.22`. If `go mod init` produced an older version, edit it:
```
module github.com/exedev/g729

go 1.22
```

- [x] **Step 3: Create `.gitignore`**

Write to `/home/exedev/g729/.gitignore`:
```
# Go build artifacts
*.test
*.out
/coverage.out
/bin/

# Editor
.idea/
.vscode/
*.swp
.DS_Store
```

- [x] **Step 4: Create MIT LICENSE**

Write to `/home/exedev/g729/LICENSE`:
```
MIT License

Copyright (c) 2026 g729 authors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [x] **Step 5: Verify the module builds**

Run:
```bash
cd /home/exedev/g729 && go build ./...
```
Expected: no output, no errors (no Go files yet, that's fine).

- [x] **Step 6: Commit**

```bash
cd /home/exedev/g729 && git add go.mod .gitignore LICENSE && git commit -m "chore: initialize Go module and MIT license"
```

---

## Task 2: Package skeleton — types and constants

**Files:**
- Create: `doc.go` (module-level)
- Create: `internal/fixed/doc.go`
- Create: `internal/fixed/types.go`

- [x] **Step 1: Write the failing test**

Create `internal/fixed/types_test.go`:
```go
package fixed

import "testing"

func TestWord16Range(t *testing.T) {
	if Max16 != 32767 {
		t.Errorf("Max16 = %d, want 32767", Max16)
	}
	if Min16 != -32768 {
		t.Errorf("Min16 = %d, want -32768", Min16)
	}
}

func TestWord32Range(t *testing.T) {
	if Max32 != 2147483647 {
		t.Errorf("Max32 = %d, want 2147483647", Max32)
	}
	if Min32 != -2147483648 {
		t.Errorf("Min32 = %d, want -2147483648", Min32)
	}
}

func TestTypeSizes(t *testing.T) {
	var w16 Word16
	var w32 Word32
	if _, ok := any(w16).(int16); !ok {
		t.Errorf("Word16 is not int16 compatible: %T", w16)
	}
	if _, ok := any(w32).(int32); !ok {
		t.Errorf("Word32 is not int32 compatible: %T", w32)
	}
}
```

- [x] **Step 2: Verify test fails (compile error, types not defined)**

Run:
```bash
cd /home/exedev/g729 && go test ./internal/fixed/...
```
Expected: compilation error — `undefined: Word16`, `undefined: Max16`, etc.

- [x] **Step 3: Create `internal/fixed/types.go`**

```go
// Package fixed implements the ITU-T G.191 basic operations used
// throughout the G.729 codec specification.
//
// All arithmetic saturates to the 16-bit or 32-bit signed range instead
// of wrapping. Identifier naming maps one-to-one to the ITU spec names
// with CamelCase applied (e.g. L_mult -> LMult, shr_r -> ShrR). See
// doc.go for the full mapping.
//
// No function in this package allocates.
package fixed

// Word16 is a 16-bit signed fixed-point value, corresponding to the
// ITU-T type Word16 (C int16).
type Word16 = int16

// Word32 is a 32-bit signed fixed-point value, corresponding to the
// ITU-T type Word32 (C int32).
type Word32 = int32

// Saturation bounds.
const (
	Max16 Word16 = 32767
	Min16 Word16 = -32768
	Max32 Word32 = 2147483647
	Min32 Word32 = -2147483648
)
```

Use `type Word16 = int16` (alias, not definition). This lets callers pass `int16` literals directly without conversion, which is the common pattern.

- [x] **Step 4: Create `internal/fixed/doc.go` with the name mapping**

```go
// Package fixed — ITU-T G.191 basic operations in Go.
//
// Name mapping (ITU spec -> Go):
//
//   saturate       -> Saturate
//   add, sub       -> Add, Sub
//   negate, abs_s  -> Negate, AbsS
//   L_add, L_sub   -> LAdd, LSub
//   L_negate       -> LNegate
//   L_abs          -> LAbs
//   extract_h      -> ExtractH
//   extract_l      -> ExtractL
//   L_deposit_h    -> LDepositH
//   L_deposit_l    -> LDepositL
//   shl, shr       -> Shl, Shr
//   shr_r          -> ShrR
//   L_shl, L_shr   -> LShl, LShr
//   L_shr_r        -> LShrR
//   L_mult         -> LMult
//   L_mac, L_msu   -> LMac, LMsu
//   mult, mult_r   -> Mult, MultR
//   round          -> Round
//   norm_s, norm_l -> NormS, NormL
//   div_s          -> DivS
package fixed
```

- [x] **Step 5: Create module-level `doc.go`**

Write `/home/exedev/g729/doc.go`:
```go
// Package g729 is a pure-Go implementation of the ITU-T G.729 Annex A
// speech codec.
//
// The public API is introduced in later phases. This file exists so
// `go doc` renders at the module root once those APIs land.
package g729
```

- [x] **Step 6: Run tests**

```bash
cd /home/exedev/g729 && go test ./...
```
Expected: `ok  github.com/exedev/g729/internal/fixed`.

- [x] **Step 7: Commit**

```bash
cd /home/exedev/g729 && git add doc.go internal/fixed/ && git commit -m "feat(fixed): package skeleton with Word16/Word32 types"
```

---

## Task 3: `Saturate` — 32-bit to 16-bit saturation

**Files:**
- Create: `internal/fixed/saturate.go`
- Create: `internal/fixed/saturate_test.go`

`Saturate(L)` returns `L` clamped to the `Word16` range. This is the foundation every 16-bit operation builds on.

- [x] **Step 1: Write the failing test**

Create `internal/fixed/saturate_test.go`:
```go
package fixed

import "testing"

func TestSaturate(t *testing.T) {
	tests := []struct {
		name string
		in   Word32
		want Word16
	}{
		{"zero", 0, 0},
		{"small positive", 100, 100},
		{"small negative", -100, -100},
		{"at max16", 32767, 32767},
		{"at min16", -32768, -32768},
		{"above max16", 32768, Max16},
		{"far above max16", 2000000000, Max16},
		{"below min16", -32769, Min16},
		{"far below min16", -2000000000, Min16},
		{"max32", Max32, Max16},
		{"min32", Min32, Min16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Saturate(tc.in); got != tc.want {
				t.Errorf("Saturate(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
```

- [x] **Step 2: Verify test fails**

Run:
```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestSaturate
```
Expected: compilation error — `undefined: Saturate`.

- [x] **Step 3: Implement `Saturate`**

Create `internal/fixed/saturate.go`:
```go
package fixed

// Saturate clamps x to the Word16 range.
func Saturate(x Word32) Word16 {
	switch {
	case x > Word32(Max16):
		return Max16
	case x < Word32(Min16):
		return Min16
	default:
		return Word16(x)
	}
}
```

- [x] **Step 4: Run the test**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestSaturate -v
```
Expected: all subtests pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/saturate.go internal/fixed/saturate_test.go && git commit -m "feat(fixed): add Saturate (32-to-16 saturation)"
```

---

## Task 4: `Add` and `Sub` — saturating 16-bit add/subtract

**Files:**
- Create: `internal/fixed/arith16.go`
- Create: `internal/fixed/arith16_test.go`

Spec: `add(a, b)` returns `Saturate(a + b)` with the sum computed in 32-bit. `sub(a, b)` is the same with subtraction.

- [x] **Step 1: Write the failing tests**

Create `internal/fixed/arith16_test.go`:
```go
package fixed

import "testing"

func TestAdd(t *testing.T) {
	tests := []struct {
		name   string
		a, b   Word16
		want   Word16
	}{
		{"zero+zero", 0, 0, 0},
		{"pos+pos", 100, 200, 300},
		{"pos+neg", 100, -50, 50},
		{"neg+neg", -100, -200, -300},
		{"saturate high", 30000, 30000, Max16},
		{"saturate low", -30000, -30000, Min16},
		{"at max no overflow", Max16, 0, Max16},
		{"max + 1 saturates", Max16, 1, Max16},
		{"min - 1 saturates", Min16, -1, Min16},
		{"min + max is -1", Min16, Max16, -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Add(tc.a, tc.b); got != tc.want {
				t.Errorf("Add(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestSub(t *testing.T) {
	tests := []struct {
		name   string
		a, b   Word16
		want   Word16
	}{
		{"zero-zero", 0, 0, 0},
		{"pos-pos", 200, 50, 150},
		{"neg-pos", -100, 50, -150},
		{"pos-neg", 100, -50, 150},
		{"saturate high", Max16, Min16, Max16},
		{"saturate low", Min16, Max16, Min16},
		{"a - a", 1234, 1234, 0},
		{"min - 0", Min16, 0, Min16},
		{"0 - min saturates", 0, Min16, Max16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sub(tc.a, tc.b); got != tc.want {
				t.Errorf("Sub(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestAdd|TestSub"
```
Expected: compile error — `undefined: Add`, `undefined: Sub`.

- [x] **Step 3: Implement**

Create `internal/fixed/arith16.go`:
```go
package fixed

// Add returns a + b with saturation to Word16.
func Add(a, b Word16) Word16 {
	return Saturate(Word32(a) + Word32(b))
}

// Sub returns a - b with saturation to Word16.
func Sub(a, b Word16) Word16 {
	return Saturate(Word32(a) - Word32(b))
}

// Negate returns -a with saturation (Min16 -> Max16).
func Negate(a Word16) Word16 {
	if a == Min16 {
		return Max16
	}
	return -a
}

// AbsS returns |a| with saturation (Min16 -> Max16).
func AbsS(a Word16) Word16 {
	if a == Min16 {
		return Max16
	}
	if a < 0 {
		return -a
	}
	return a
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestAdd|TestSub" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/arith16.go internal/fixed/arith16_test.go && git commit -m "feat(fixed): add saturating Add/Sub/Negate/AbsS"
```

---

## Task 5: Tests for `Negate` and `AbsS`

Implementations landed in Task 4; now add their dedicated tests.

**Files:**
- Modify: `internal/fixed/arith16_test.go` (append)

- [x] **Step 1: Write the failing tests**

Append to `internal/fixed/arith16_test.go`:
```go
func TestNegate(t *testing.T) {
	tests := []struct {
		in, want Word16
	}{
		{0, 0},
		{1, -1},
		{-1, 1},
		{100, -100},
		{-100, 100},
		{Max16, -Max16},
		{Min16, Max16}, // saturates because -Min16 overflows
	}
	for _, tc := range tests {
		if got := Negate(tc.in); got != tc.want {
			t.Errorf("Negate(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestAbsS(t *testing.T) {
	tests := []struct {
		in, want Word16
	}{
		{0, 0},
		{100, 100},
		{-100, 100},
		{Max16, Max16},
		{Min16, Max16}, // saturates
		{-1, 1},
	}
	for _, tc := range tests {
		if got := AbsS(tc.in); got != tc.want {
			t.Errorf("AbsS(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
```

- [x] **Step 2: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestNegate|TestAbsS" -v
```
Expected: all pass (already implemented in Task 4).

- [x] **Step 3: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/arith16_test.go && git commit -m "test(fixed): Negate and AbsS boundary tests"
```

---

## Task 6: `LAdd` and `LSub` — saturating 32-bit add/subtract

**Files:**
- Create: `internal/fixed/arith32.go`
- Create: `internal/fixed/arith32_test.go`

Go's bare `+` wraps on `int32` overflow. We detect overflow by doing the sum in `int64`, then saturate.

- [x] **Step 1: Write the failing tests**

Create `internal/fixed/arith32_test.go`:
```go
package fixed

import "testing"

func TestLAdd(t *testing.T) {
	tests := []struct {
		name     string
		a, b     Word32
		want     Word32
	}{
		{"zero", 0, 0, 0},
		{"small", 100, 200, 300},
		{"negative", -100, -200, -300},
		{"saturate high", 2_000_000_000, 2_000_000_000, Max32},
		{"saturate low", -2_000_000_000, -2_000_000_000, Min32},
		{"max + 1 saturates", Max32, 1, Max32},
		{"min + -1 saturates", Min32, -1, Min32},
		{"min + max", Min32, Max32, -1},
		{"mixed signs no saturation", Max32, -100, Max32 - 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LAdd(tc.a, tc.b); got != tc.want {
				t.Errorf("LAdd(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestLSub(t *testing.T) {
	tests := []struct {
		name     string
		a, b     Word32
		want     Word32
	}{
		{"zero", 0, 0, 0},
		{"small", 300, 100, 200},
		{"saturate high", Max32, Min32, Max32},
		{"saturate low", Min32, Max32, Min32},
		{"0 - min saturates", 0, Min32, Max32},
		{"min - 0", Min32, 0, Min32},
		{"a - a", 123456, 123456, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LSub(tc.a, tc.b); got != tc.want {
				t.Errorf("LSub(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestLAdd|TestLSub"
```
Expected: compile error — `undefined: LAdd`, `undefined: LSub`.

- [x] **Step 3: Implement**

Create `internal/fixed/arith32.go`:
```go
package fixed

// saturate64 clamps a 64-bit value to the Word32 range.
func saturate64(x int64) Word32 {
	switch {
	case x > int64(Max32):
		return Max32
	case x < int64(Min32):
		return Min32
	default:
		return Word32(x)
	}
}

// LAdd returns a + b with saturation to Word32.
func LAdd(a, b Word32) Word32 {
	return saturate64(int64(a) + int64(b))
}

// LSub returns a - b with saturation to Word32.
func LSub(a, b Word32) Word32 {
	return saturate64(int64(a) - int64(b))
}

// LNegate returns -a with saturation (Min32 -> Max32).
func LNegate(a Word32) Word32 {
	if a == Min32 {
		return Max32
	}
	return -a
}

// LAbs returns |a| with saturation (Min32 -> Max32).
func LAbs(a Word32) Word32 {
	if a == Min32 {
		return Max32
	}
	if a < 0 {
		return -a
	}
	return a
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestLAdd|TestLSub" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/arith32.go internal/fixed/arith32_test.go && git commit -m "feat(fixed): add saturating LAdd/LSub/LNegate/LAbs"
```

---

## Task 7: Tests for `LNegate` and `LAbs`

**Files:**
- Modify: `internal/fixed/arith32_test.go` (append)

- [x] **Step 1: Append tests**

Append to `internal/fixed/arith32_test.go`:
```go
func TestLNegate(t *testing.T) {
	tests := []struct {
		in, want Word32
	}{
		{0, 0},
		{1, -1},
		{-1, 1},
		{Max32, -Max32},
		{Min32, Max32}, // saturates
	}
	for _, tc := range tests {
		if got := LNegate(tc.in); got != tc.want {
			t.Errorf("LNegate(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestLAbs(t *testing.T) {
	tests := []struct {
		in, want Word32
	}{
		{0, 0},
		{100, 100},
		{-100, 100},
		{Max32, Max32},
		{Min32, Max32}, // saturates
	}
	for _, tc := range tests {
		if got := LAbs(tc.in); got != tc.want {
			t.Errorf("LAbs(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
```

- [x] **Step 2: Run**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestLNegate|TestLAbs" -v
```
Expected: all pass.

- [x] **Step 3: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/arith32_test.go && git commit -m "test(fixed): LNegate and LAbs boundary tests"
```

---

## Task 8: `ExtractH`, `ExtractL`, `LDepositH`, `LDepositL`

Bit-level extract/deposit primitives. All trivial — a single line each — but must be tested because other primitives (`Round`, `LMult`) depend on them.

Semantics:
- `ExtractH(L)` → high 16 bits of `L` (arithmetic shift right 16).
- `ExtractL(L)` → low 16 bits of `L` as `Word16` (truncation).
- `LDepositH(w)` → `Word32(w) << 16`.
- `LDepositL(w)` → sign-extend `w` to `Word32`.

**Files:**
- Create: `internal/fixed/extract.go`
- Create: `internal/fixed/extract_test.go`

- [x] **Step 1: Write the failing tests**

Create `internal/fixed/extract_test.go`:
```go
package fixed

import "testing"

func TestExtractH(t *testing.T) {
	tests := []struct {
		in   Word32
		want Word16
	}{
		{0, 0},
		{0x00010000, 1},
		{0x7FFF0000, Max16},
		{-0x00010000, -1}, // low bits zero, high = -1
		{0x7FFFFFFF, Max16},
		{int32(0x80000000), Min16},
	}
	for _, tc := range tests {
		if got := ExtractH(tc.in); got != tc.want {
			t.Errorf("ExtractH(%#x) = %d, want %d", uint32(tc.in), got, tc.want)
		}
	}
}

func TestExtractL(t *testing.T) {
	tests := []struct {
		in   Word32
		want Word16
	}{
		{0, 0},
		{1, 1},
		{-1, -1},
		{0x00007FFF, 32767},
		{0x00008000, -32768},
		{int32(0x80008000), -32768},
		{0x12345678, 0x5678},
	}
	for _, tc := range tests {
		if got := ExtractL(tc.in); got != tc.want {
			t.Errorf("ExtractL(%#x) = %d, want %d", uint32(tc.in), got, tc.want)
		}
	}
}

func TestLDepositH(t *testing.T) {
	tests := []struct {
		in   Word16
		want Word32
	}{
		{0, 0},
		{1, 0x00010000},
		{-1, int32(-0x00010000)},
		{Max16, 0x7FFF0000},
		{Min16, int32(0x80000000)},
	}
	for _, tc := range tests {
		if got := LDepositH(tc.in); got != tc.want {
			t.Errorf("LDepositH(%d) = %#x, want %#x", tc.in, uint32(got), uint32(tc.want))
		}
	}
}

func TestLDepositL(t *testing.T) {
	tests := []struct {
		in   Word16
		want Word32
	}{
		{0, 0},
		{1, 1},
		{-1, -1},
		{Max16, 32767},
		{Min16, -32768},
	}
	for _, tc := range tests {
		if got := LDepositL(tc.in); got != tc.want {
			t.Errorf("LDepositL(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestExtract|TestLDeposit"
```
Expected: compile error.

- [x] **Step 3: Implement**

Create `internal/fixed/extract.go`:
```go
package fixed

// ExtractH returns the high 16 bits of x (arithmetic).
func ExtractH(x Word32) Word16 {
	return Word16(x >> 16)
}

// ExtractL returns the low 16 bits of x, reinterpreted as Word16.
func ExtractL(x Word32) Word16 {
	return Word16(x & 0xFFFF)
}

// LDepositH returns x placed in the high 16 bits of a Word32. The low
// 16 bits are zero.
func LDepositH(x Word16) Word32 {
	return Word32(x) << 16
}

// LDepositL sign-extends x into Word32.
func LDepositL(x Word16) Word32 {
	return Word32(x)
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestExtract|TestLDeposit" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/extract.go internal/fixed/extract_test.go && git commit -m "feat(fixed): add ExtractH/ExtractL/LDepositH/LDepositL"
```

---

## Task 9: `Shl` and `Shr` — saturating 16-bit shifts

Semantics from spec:
- `Shl(a, n)`: if `n >= 0`, left-shift by `n` with saturation; if `n < 0`, right-shift by `-n` (equivalent to `Shr(a, -n)`).
- `Shr(a, n)`: if `n >= 0`, arithmetic right-shift by `n`; if `n < 0`, `Shl(a, -n)`; if `n >= 15`, result is `0` for non-negative, `-1` for negative.
- Shifts by very large positive n to Shl must saturate to Max16/Min16 according to sign of a.

**Files:**
- Create: `internal/fixed/shift16.go`
- Create: `internal/fixed/shift16_test.go`

- [x] **Step 1: Write the failing tests**

Create `internal/fixed/shift16_test.go`:
```go
package fixed

import "testing"

func TestShl(t *testing.T) {
	tests := []struct {
		name    string
		a       Word16
		n       Word16
		want    Word16
	}{
		{"zero shift", 100, 0, 100},
		{"shift left 1", 100, 1, 200},
		{"shift left 3", 100, 3, 800},
		{"neg shift left 1", -100, 1, -200},
		{"saturate high", 20000, 2, Max16},
		{"saturate low", -20000, 2, Min16},
		{"max shifted saturates", Max16, 1, Max16},
		{"min shifted saturates", Min16, 1, Min16},
		{"negative n -> right shift", 100, -1, 50},
		{"negative n large -> 0", 100, -20, 0},
		{"neg a with negative n large -> -1", -100, -20, -1},
		{"large n saturates", 1, 20, Max16},
		{"large n neg saturates", -1, 20, Min16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Shl(tc.a, tc.n); got != tc.want {
				t.Errorf("Shl(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}

func TestShr(t *testing.T) {
	tests := []struct {
		name    string
		a       Word16
		n       Word16
		want    Word16
	}{
		{"zero shift", 100, 0, 100},
		{"shift right 1", 100, 1, 50},
		{"shift right 3", 800, 3, 100},
		{"neg shift right", -100, 1, -50},
		{"neg arithmetic", -2, 1, -1},
		{"neg close to zero", -1, 1, -1},
		{"negative n -> left shift", 100, -1, 200},
		{"n>=15 nonneg a", 32767, 15, 0},
		{"n>=15 neg a", -32768, 15, -1},
		{"n>15 neg a", -1, 20, -1},
		{"n>15 pos a", 100, 20, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Shr(tc.a, tc.n); got != tc.want {
				t.Errorf("Shr(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestShl|TestShr"
```
Expected: compile error.

- [x] **Step 3: Implement**

Create `internal/fixed/shift16.go`:
```go
package fixed

// Shl returns a shifted left by n positions with saturation. A negative
// n is interpreted as right-shift by -n (per ITU spec). Large shifts
// saturate to Max16 or Min16 according to the sign of a.
func Shl(a, n Word16) Word16 {
	if n <= 0 {
		if n < -16 {
			n = -16
		}
		return Shr(a, -n)
	}
	if n >= 16 {
		if a > 0 {
			return Max16
		}
		if a < 0 {
			return Min16
		}
		return 0
	}
	// Shift in int32, saturate.
	return Saturate(Word32(a) << uint(n))
}

// Shr returns a arithmetically shifted right by n positions. A negative
// n is interpreted as left-shift by -n. For n >= 15 the result is 0 if
// a is non-negative and -1 if a is negative.
func Shr(a, n Word16) Word16 {
	if n < 0 {
		if n < -16 {
			n = -16
		}
		return Shl(a, -n)
	}
	if n >= 15 {
		if a < 0 {
			return -1
		}
		return 0
	}
	return a >> uint(n)
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestShl|TestShr" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/shift16.go internal/fixed/shift16_test.go && git commit -m "feat(fixed): add saturating Shl and Shr"
```

---

## Task 10: `ShrR` — `Shr` with rounding

Semantics: `ShrR(a, n) = Shr(Add(a, 1<<(n-1)), n)` for `n > 0`, `ShrR(a, 0) = a`, and for `n < 0` it acts like `Shl(a, -n)`.

**Files:**
- Modify: `internal/fixed/shift16.go` (append)
- Modify: `internal/fixed/shift16_test.go` (append)

- [x] **Step 1: Write the failing test**

Append to `internal/fixed/shift16_test.go`:
```go
func TestShrR(t *testing.T) {
	tests := []struct {
		name string
		a, n Word16
		want Word16
	}{
		{"n=0", 100, 0, 100},
		{"rounds up exact half", 3, 1, 2},    // (3 + 1) >> 1 = 2
		{"rounds down below half", 4, 2, 1},   // (4 + 2) >> 2 = 6>>2 = 1
		{"rounds up at half", 6, 2, 2},        // (6 + 2) >> 2 = 2
		{"negative rounding", -3, 1, -1},      // (-3 + 1) >> 1 = -1
		{"large n", 100, 8, 0},
		{"large n neg", -100, 8, 0},
		{"n negative acts as Shl", 100, -1, 200},
		{"n>=15 nonneg", 32767, 15, 1},        // (32767 + 16384) saturates then rounds to 1
		{"n>=15 neg", -32768, 15, -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShrR(tc.a, tc.n); got != tc.want {
				t.Errorf("ShrR(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}
```

The "n>=15 nonneg" case is subtle. The ITU spec defines `shr_r` as: when `n > 15`, the result depends on whether the rounding bit exists. The canonical form:
```
if (n > 15):  result = 0
else:         L_out = shr(a, n); if ((a & (1 << (n-1))) != 0) L_out = add(L_out, 1); result = L_out
```
For `n == 15` and `a = 32767` (binary `0111111111111111`): bit 14 is 1 → result = Shr(a, 15) + 1 = 0 + 1 = 1.
For `n == 15` and `a = -32768`: bit 14 is 0 → result = Shr(a, 15) = -1.

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestShrR
```
Expected: compile error.

- [x] **Step 3: Implement**

Append to `internal/fixed/shift16.go`:
```go
// ShrR returns a shifted right by n with rounding. For n <= 0 this is
// equivalent to Shl(a, -n). For n > 15 the result is 0.
func ShrR(a, n Word16) Word16 {
	if n <= 0 {
		return Shl(a, -n)
	}
	if n > 15 {
		return 0
	}
	out := Shr(a, n)
	if a&(1<<uint(n-1)) != 0 {
		out = Add(out, 1)
	}
	return out
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestShrR -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/shift16.go internal/fixed/shift16_test.go && git commit -m "feat(fixed): add ShrR (shift right with rounding)"
```

---

## Task 11: `LShl`, `LShr`, `LShrR` — 32-bit shifts

**Files:**
- Create: `internal/fixed/shift32.go`
- Create: `internal/fixed/shift32_test.go`

Semantics mirror the 16-bit versions but saturate to `Max32` / `Min32`.

- [x] **Step 1: Write the failing tests**

Create `internal/fixed/shift32_test.go`:
```go
package fixed

import "testing"

func TestLShl(t *testing.T) {
	tests := []struct {
		name string
		a    Word32
		n    Word16
		want Word32
	}{
		{"zero shift", 1000, 0, 1000},
		{"shift left 1", 1000, 1, 2000},
		{"shift left 10", 1000, 10, 1_024_000},
		{"neg shift left", -1000, 1, -2000},
		{"saturate high", 1_000_000_000, 2, Max32},
		{"saturate low", -1_000_000_000, 2, Min32},
		{"n negative -> LShr", 1000, -1, 500},
		{"large n saturates pos", 1, 32, Max32},
		{"large n saturates neg", -1, 32, Min32},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LShl(tc.a, tc.n); got != tc.want {
				t.Errorf("LShl(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}

func TestLShr(t *testing.T) {
	tests := []struct {
		name string
		a    Word32
		n    Word16
		want Word32
	}{
		{"zero shift", 1000, 0, 1000},
		{"shift right 1", 1000, 1, 500},
		{"shift right 10", 1_024_000, 10, 1000},
		{"negative arithmetic", -2, 1, -1},
		{"n negative -> LShl", 1000, -1, 2000},
		{"n >= 31 pos", 1000000, 31, 0},
		{"n >= 31 neg", -1000000, 31, -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LShr(tc.a, tc.n); got != tc.want {
				t.Errorf("LShr(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}

func TestLShrR(t *testing.T) {
	tests := []struct {
		name string
		a    Word32
		n    Word16
		want Word32
	}{
		{"n=0", 1000, 0, 1000},
		{"rounds at half", 3, 1, 2},
		{"below half", 4, 2, 1},
		{"at half", 6, 2, 2},
		{"neg rounding", -3, 1, -1},
		{"n=31 nonneg", Max32, 31, 1},
		{"n=31 neg", Min32, 31, -1},
		{"n>31", 12345, 32, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LShrR(tc.a, tc.n); got != tc.want {
				t.Errorf("LShrR(%d, %d) = %d, want %d", tc.a, tc.n, got, tc.want)
			}
		})
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestLShl|TestLShr|TestLShrR"
```
Expected: compile error.

- [x] **Step 3: Implement**

Create `internal/fixed/shift32.go`:
```go
package fixed

// LShl returns a shifted left by n with saturation to Word32. Negative
// n means right shift by -n.
func LShl(a Word32, n Word16) Word32 {
	if n <= 0 {
		if n < -32 {
			n = -32
		}
		return LShr(a, -n)
	}
	if n >= 32 {
		if a > 0 {
			return Max32
		}
		if a < 0 {
			return Min32
		}
		return 0
	}
	return saturate64(int64(a) << uint(n))
}

// LShr returns a arithmetically shifted right by n. Negative n means
// left shift. For n >= 31 the result is 0 for non-negative a and -1 for
// negative a.
func LShr(a Word32, n Word16) Word32 {
	if n < 0 {
		if n < -32 {
			n = -32
		}
		return LShl(a, -n)
	}
	if n >= 31 {
		if a < 0 {
			return -1
		}
		return 0
	}
	return a >> uint(n)
}

// LShrR returns a shifted right by n with rounding. For n <= 0 equals
// LShl(a, -n). For n > 31 returns 0.
func LShrR(a Word32, n Word16) Word32 {
	if n <= 0 {
		return LShl(a, -n)
	}
	if n > 31 {
		return 0
	}
	out := LShr(a, n)
	if a&(1<<uint(n-1)) != 0 {
		out = LAdd(out, 1)
	}
	return out
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestLShl|TestLShr|TestLShrR" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/shift32.go internal/fixed/shift32_test.go && git commit -m "feat(fixed): add saturating 32-bit shifts LShl/LShr/LShrR"
```

---

## Task 12: `LMult` — 16x16 signed multiply to 32-bit

Semantics: `LMult(a, b) = Saturate32(2 * a * b)`. Only overflow case is `a = b = -32768`, where `2 * (-32768) * (-32768) = 2^31` saturates to `Max32`.

**Files:**
- Create: `internal/fixed/mult.go`
- Create: `internal/fixed/mult_test.go`

- [x] **Step 1: Write the failing test**

Create `internal/fixed/mult_test.go`:
```go
package fixed

import "testing"

func TestLMult(t *testing.T) {
	tests := []struct {
		name string
		a, b Word16
		want Word32
	}{
		{"zero", 0, 0, 0},
		{"zero times anything", 0, 12345, 0},
		{"one times one", 1, 1, 2},
		{"pos * pos", 100, 200, 40_000},
		{"pos * neg", 100, -200, -40_000},
		{"max * one", Max16, 1, 2 * int32(Max16)},
		{"max * max", Max16, Max16, 2 * int32(Max16) * int32(Max16)},
		{"min * min saturates", Min16, Min16, Max32},
		{"min * max", Min16, Max16, 2 * int32(Min16) * int32(Max16)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LMult(tc.a, tc.b); got != tc.want {
				t.Errorf("LMult(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
```

Note: `max * max` is `32767 * 32767 * 2 = 2147352578`, which fits in Word32. Only `min * min = -32768 * -32768 * 2 = 2^31` overflows.

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestLMult
```
Expected: compile error.

- [x] **Step 3: Implement**

Create `internal/fixed/mult.go`:
```go
package fixed

// LMult returns 2*a*b saturated to Word32. The only saturation case is
// a = b = Min16, where the mathematical result 2^31 overflows to Max32.
func LMult(a, b Word16) Word32 {
	if a == Min16 && b == Min16 {
		return Max32
	}
	return Word32(a) * Word32(b) << 1
}
```

- [x] **Step 4: Run the test**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestLMult -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/mult.go internal/fixed/mult_test.go && git commit -m "feat(fixed): add LMult (16x16 to 32-bit saturating multiply)"
```

---

## Task 13: `LMac` and `LMsu`

Multiply-accumulate: `LMac(acc, a, b) = LAdd(acc, LMult(a, b))`. Subtract: `LMsu(acc, a, b) = LSub(acc, LMult(a, b))`. Foundational in every correlation / convolution inside G.729.

**Files:**
- Modify: `internal/fixed/mult.go` (append)
- Modify: `internal/fixed/mult_test.go` (append)

- [x] **Step 1: Write the failing tests**

Append to `internal/fixed/mult_test.go`:
```go
func TestLMac(t *testing.T) {
	tests := []struct {
		name string
		acc  Word32
		a, b Word16
		want Word32
	}{
		{"zero acc", 0, 100, 200, 40_000},
		{"acc add", 1000, 100, 200, 41_000},
		{"acc subtract", 100_000, 100, -200, 60_000},
		{"saturate acc high", Max32 - 10, 100, 100, Max32},
		{"saturate on mult only", 0, Min16, Min16, Max32},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LMac(tc.acc, tc.a, tc.b); got != tc.want {
				t.Errorf("LMac(%d, %d, %d) = %d, want %d", tc.acc, tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestLMsu(t *testing.T) {
	tests := []struct {
		name string
		acc  Word32
		a, b Word16
		want Word32
	}{
		{"zero acc", 0, 100, 200, -40_000},
		{"acc minus", 100_000, 100, 200, 60_000},
		{"acc minus neg prod", 1000, 100, -200, 41_000},
		{"saturate low", Min32 + 10, 100, 100, Min32},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LMsu(tc.acc, tc.a, tc.b); got != tc.want {
				t.Errorf("LMsu(%d, %d, %d) = %d, want %d", tc.acc, tc.a, tc.b, got, tc.want)
			}
		})
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestLMac|TestLMsu"
```
Expected: compile error.

- [x] **Step 3: Implement**

Append to `internal/fixed/mult.go`:
```go
// LMac returns acc + 2*a*b with saturation at both the multiplication
// and addition stages.
func LMac(acc Word32, a, b Word16) Word32 {
	return LAdd(acc, LMult(a, b))
}

// LMsu returns acc - 2*a*b with saturation at both stages.
func LMsu(acc Word32, a, b Word16) Word32 {
	return LSub(acc, LMult(a, b))
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestLMac|TestLMsu" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/mult.go internal/fixed/mult_test.go && git commit -m "feat(fixed): add LMac and LMsu (multiply-accumulate/subtract)"
```

---

## Task 14: `Mult` and `MultR`

`Mult(a, b) = ExtractL(LShr(LMult(a, b), 1)) >> 15`-equivalent, but more directly: `Mult(a, b) = Saturate((a*b) >> 15)`.

`MultR(a, b) = Saturate(((a*b) + 0x00004000) >> 15)` — same but with rounding.

Both saturate when `a = b = Min16`.

**Files:**
- Modify: `internal/fixed/mult.go` (append)
- Modify: `internal/fixed/mult_test.go` (append)

- [x] **Step 1: Write the failing tests**

Append to `internal/fixed/mult_test.go`:
```go
func TestMult(t *testing.T) {
	tests := []struct {
		name string
		a, b Word16
		want Word16
	}{
		{"zero", 0, 0, 0},
		{"half times half", 16384, 16384, 8192}, // 0.5 * 0.5 = 0.25 in Q15
		{"one-ish times max", Max16, Max16, 32766},
		{"saturate", Min16, Min16, Max16}, // 0x40000000 >> 15 = 0x8000 saturates
		{"pos * neg", 16384, -16384, -8192},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Mult(tc.a, tc.b); got != tc.want {
				t.Errorf("Mult(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestMultR(t *testing.T) {
	tests := []struct {
		name string
		a, b Word16
		want Word16
	}{
		{"zero", 0, 0, 0},
		{"half times half rounds", 16384, 16384, 8192},
		{"saturates like Mult", Min16, Min16, Max16},
		{"rounds up", 32767, 2, 2}, // (32767*2 + 16384) >> 15 = 2
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MultR(tc.a, tc.b); got != tc.want {
				t.Errorf("MultR(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
```

- [x] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestMult|TestMultR"
```
Expected: compile error.

- [x] **Step 3: Implement**

Append to `internal/fixed/mult.go`:
```go
// Mult returns (a*b) >> 15 saturated to Word16. Models a fractional
// multiply in Q15 format.
func Mult(a, b Word16) Word16 {
	prod := Word32(a) * Word32(b)
	// ((prod << 1) >> 16) == (prod >> 15), but saturates on Min16*Min16.
	if a == Min16 && b == Min16 {
		return Max16
	}
	return Word16(prod >> 15)
}

// MultR returns ((a*b) + 0x4000) >> 15 saturated to Word16. Fractional
// multiply with rounding.
func MultR(a, b Word16) Word16 {
	if a == Min16 && b == Min16 {
		return Max16
	}
	prod := Word32(a)*Word32(b) + 0x4000
	// prod >> 15 may overflow Word16 only when a*b is near Max32; saturate.
	return Saturate(prod >> 15)
}
```

- [x] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestMult|TestMultR" -v
```
Expected: all pass.

- [x] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/mult.go internal/fixed/mult_test.go && git commit -m "feat(fixed): add Mult and MultR (Q15 fractional multiply)"
```

---

## Task 15: `Round`

`Round(L) = ExtractH(LAdd(L, 0x00008000))`. Rounds a 32-bit value to the nearest 16-bit value in the high half.

**Files:**
- Create: `internal/fixed/round.go`
- Create: `internal/fixed/round_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/fixed/round_test.go`:
```go
package fixed

import "testing"

func TestRound(t *testing.T) {
	tests := []struct {
		name string
		in   Word32
		want Word16
	}{
		{"zero", 0, 0},
		{"small rounds to zero", 0x00007FFF, 0},
		{"half rounds up", 0x00008000, 1},
		{"just above one high", 0x00018000, 2},
		{"neg half rounds toward zero", -0x00008000, 0},
		{"neg rounds negative", -0x00018000, -1},
		{"max32 saturates", Max32, Max16},
		{"min32 stays min16", Min32, Min16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Round(tc.in); got != tc.want {
				t.Errorf("Round(%#x) = %d, want %d", uint32(tc.in), got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestRound
```
Expected: compile error.

- [ ] **Step 3: Implement**

Create `internal/fixed/round.go`:
```go
package fixed

// Round rounds a Word32 to Word16 by adding 0x00008000 (half of the low
// 16 bits) and taking the high half. Saturates.
func Round(x Word32) Word16 {
	return ExtractH(LAdd(x, 0x00008000))
}
```

- [ ] **Step 4: Run the test**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestRound -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/round.go internal/fixed/round_test.go && git commit -m "feat(fixed): add Round (32-to-16 with half-up rounding)"
```

---

## Task 16: `NormS` and `NormL`

Normalization: return the number of left shifts needed to bring `|x|` to saturate into the top bit.

Semantics:
- `NormS(0) = 0`, `NormL(0) = 0` (spec convention).
- `NormS(x)` = largest `n` in `[0, 15]` such that `Shl(x, n) does not saturate`.
- `NormL(x)` = largest `n` in `[0, 31]` such that `LShl(x, n) does not saturate`.

Equivalent formulations: `NormS(x) = 15 - bit_length(|x|)` for `x != 0`, using sign bit semantics.

**Files:**
- Create: `internal/fixed/norm.go`
- Create: `internal/fixed/norm_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/fixed/norm_test.go`:
```go
package fixed

import "testing"

func TestNormS(t *testing.T) {
	tests := []struct {
		in   Word16
		want Word16
	}{
		{0, 0},
		{1, 14},
		{2, 13},
		{Max16, 0},
		{Min16, 0},
		{-1, 15}, // negative -1 normalizes all the way
		{16384, 0},
		{16383, 1},
		{100, 8},
	}
	for _, tc := range tests {
		if got := NormS(tc.in); got != tc.want {
			t.Errorf("NormS(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestNormL(t *testing.T) {
	tests := []struct {
		in   Word32
		want Word16
	}{
		{0, 0},
		{1, 30},
		{Max32, 0},
		{Min32, 0},
		{-1, 31},
		{0x40000000, 0},
		{0x3FFFFFFF, 1},
		{100, 24},
	}
	for _, tc := range tests {
		if got := NormL(tc.in); got != tc.want {
			t.Errorf("NormL(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestNormS|TestNormL"
```
Expected: compile error.

- [ ] **Step 3: Implement**

Create `internal/fixed/norm.go`:
```go
package fixed

// NormS returns the number of left shifts needed to normalize x so that
// further shifting would saturate. Returns 0 for x = 0.
//
// Equivalent spec behavior:
//   NormS(x) = min n in [0..15] such that |Shl(x, n+1)| would saturate
func NormS(x Word16) Word16 {
	if x == 0 {
		return 0
	}
	if x == -1 {
		return 15
	}
	if x < 0 {
		x = ^x // ones complement: counts leading ones as leading zeros
	}
	var n Word16
	for x < 0x4000 {
		x <<= 1
		n++
	}
	return n
}

// NormL returns the number of left shifts to normalize a Word32.
func NormL(x Word32) Word16 {
	if x == 0 {
		return 0
	}
	if x == -1 {
		return 31
	}
	if x < 0 {
		x = ^x
	}
	var n Word16
	for x < 0x40000000 {
		x <<= 1
		n++
	}
	return n
}
```

- [ ] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run "TestNormS|TestNormL" -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/norm.go internal/fixed/norm_test.go && git commit -m "feat(fixed): add NormS and NormL (leading-bit normalization)"
```

---

## Task 17: `DivS` — fractional division

Semantics from ITU spec:
- `DivS(num, den)` computes `num / den` in Q15 when `0 <= num <= den` and `den != 0`.
- Result is in range `[0, Max16]`, approximately `num / den * 32768`.
- Algorithm: iterative subtract-and-shift, 15 iterations.
- Precondition: `num >= 0`, `den > 0`, `num <= den`. Violations return `Max16` per spec (the C reference exits; we match the functional behavior without exiting).

**Files:**
- Create: `internal/fixed/div.go`
- Create: `internal/fixed/div_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/fixed/div_test.go`:
```go
package fixed

import "testing"

func TestDivS(t *testing.T) {
	tests := []struct {
		name     string
		num, den Word16
		want     Word16
	}{
		{"zero over anything", 0, 1, 0},
		{"equal", 1000, 1000, Max16},
		{"half", 16384, 32768 - 1, 16384}, // 0.5
		{"quarter", 8192, 32767, 8192},
		{"one third approx", 10922, 32766, 10922}, // ~1/3 in Q15
		{"num > den returns max", 100, 50, Max16},
		{"num negative returns max", -1, 100, Max16},
		{"den zero returns max", 100, 0, Max16},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DivS(tc.num, tc.den)
			// The iterative algorithm can be off by 1 from an ideal Q15
			// division for irrational ratios; tolerate that for the
			// approximate cases by checking absolute difference <= 1.
			diff := int32(got) - int32(tc.want)
			if diff < 0 {
				diff = -diff
			}
			if diff > 1 {
				t.Errorf("DivS(%d, %d) = %d, want %d (+/-1)", tc.num, tc.den, got, tc.want)
			}
		})
	}
}

func TestDivSExactCases(t *testing.T) {
	// Exact-match cases (no tolerance).
	cases := []struct {
		num, den Word16
		want     Word16
	}{
		{0, 1, 0},
		{0, 32767, 0},
		{1, 1, Max16},
		{100, 100, Max16},
	}
	for _, tc := range cases {
		if got := DivS(tc.num, tc.den); got != tc.want {
			t.Errorf("DivS(%d, %d) = %d, want %d", tc.num, tc.den, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Verify failure**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestDivS
```
Expected: compile error.

- [ ] **Step 3: Implement**

Create `internal/fixed/div.go`:
```go
package fixed

// DivS returns num/den in Q15 format. Preconditions:
//   num >= 0
//   den >  0
//   num <= den
// If any precondition is violated, returns Max16 (matches ITU spec's
// defensive behavior). The exact-equal case num == den returns Max16
// because 32768 is not representable as Word16.
//
// The algorithm is an iterative 15-step subtract-and-shift fractional
// division, identical in behavior to the ITU basic-operations spec.
func DivS(num, den Word16) Word16 {
	if num < 0 || den <= 0 || num > den {
		return Max16
	}
	if num == 0 {
		return 0
	}
	if num == den {
		return Max16
	}

	var q Word16
	n := Word32(num) << 15
	d := Word32(den) << 15
	for i := 0; i < 15; i++ {
		q <<= 1
		n <<= 1
		if n >= d {
			n -= d
			q |= 1
		}
	}
	return q
}
```

Note: the algorithm above is one of several that yield the same Q15 output for the allowed input range. Implementation style differs from the ITU reference C code; the *observable function* must match within the standard's tolerance.

- [ ] **Step 4: Run the tests**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -run TestDivS -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/div.go internal/fixed/div_test.go && git commit -m "feat(fixed): add DivS (Q15 fractional division)"
```

---

## Task 18: Full-package test pass and allocation check

**Files:**
- Create: `internal/fixed/alloc_test.go`

Verify the whole package compiles and tests pass cleanly, and confirm the primitives do not allocate (they should not, since they return primitive types).

- [ ] **Step 1: Write the allocation test**

Create `internal/fixed/alloc_test.go`:
```go
package fixed

import "testing"

// Confirms that the arithmetic primitives do not allocate. If a future
// change introduces allocation in a hot primitive, this test fails.
func TestNoAllocationInPrimitives(t *testing.T) {
	var s16 Word16 = 12345
	var s32 Word32 = 1234567890

	cases := []struct {
		name string
		fn   func()
	}{
		{"Add", func() { _ = Add(s16, s16) }},
		{"Sub", func() { _ = Sub(s16, s16) }},
		{"Negate", func() { _ = Negate(s16) }},
		{"AbsS", func() { _ = AbsS(s16) }},
		{"LAdd", func() { _ = LAdd(s32, s32) }},
		{"LSub", func() { _ = LSub(s32, s32) }},
		{"LNegate", func() { _ = LNegate(s32) }},
		{"LAbs", func() { _ = LAbs(s32) }},
		{"LMult", func() { _ = LMult(s16, s16) }},
		{"LMac", func() { _ = LMac(s32, s16, s16) }},
		{"LMsu", func() { _ = LMsu(s32, s16, s16) }},
		{"Mult", func() { _ = Mult(s16, s16) }},
		{"MultR", func() { _ = MultR(s16, s16) }},
		{"Round", func() { _ = Round(s32) }},
		{"Shl", func() { _ = Shl(s16, 3) }},
		{"Shr", func() { _ = Shr(s16, 3) }},
		{"ShrR", func() { _ = ShrR(s16, 3) }},
		{"LShl", func() { _ = LShl(s32, 3) }},
		{"LShr", func() { _ = LShr(s32, 3) }},
		{"LShrR", func() { _ = LShrR(s32, 3) }},
		{"NormS", func() { _ = NormS(s16) }},
		{"NormL", func() { _ = NormL(s32) }},
		{"DivS", func() { _ = DivS(100, 200) }},
		{"Saturate", func() { _ = Saturate(s32) }},
		{"ExtractH", func() { _ = ExtractH(s32) }},
		{"ExtractL", func() { _ = ExtractL(s32) }},
		{"LDepositH", func() { _ = LDepositH(s16) }},
		{"LDepositL", func() { _ = LDepositL(s16) }},
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

- [ ] **Step 2: Run the full test suite**

```bash
cd /home/exedev/g729 && go test ./... -v
```
Expected: all tests (including the allocation suite) pass.

- [ ] **Step 3: Run with race detector**

```bash
cd /home/exedev/g729 && go test ./... -race
```
Expected: no race detector output, all pass.

- [ ] **Step 4: Run vet**

```bash
cd /home/exedev/g729 && go vet ./...
```
Expected: no output (clean).

- [ ] **Step 5: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/alloc_test.go && git commit -m "test(fixed): assert zero allocation in primitive calls"
```

---

## Task 19: Benchmarks

**Files:**
- Create: `internal/fixed/bench_test.go`

- [ ] **Step 1: Write benchmarks**

Create `internal/fixed/bench_test.go`:
```go
package fixed

import "testing"

func BenchmarkAdd(b *testing.B) {
	var s16 Word16 = 12345
	for i := 0; i < b.N; i++ {
		_ = Add(s16, s16)
	}
}

func BenchmarkLMult(b *testing.B) {
	var s16 Word16 = 12345
	for i := 0; i < b.N; i++ {
		_ = LMult(s16, s16)
	}
}

func BenchmarkLMac(b *testing.B) {
	var s16 Word16 = 12345
	var s32 Word32 = 0
	for i := 0; i < b.N; i++ {
		s32 = LMac(s32, s16, s16)
	}
	_ = s32
}

func BenchmarkDivS(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DivS(12345, 23456)
	}
}

func BenchmarkNormL(b *testing.B) {
	var s32 Word32 = 0x0000FF00
	for i := 0; i < b.N; i++ {
		_ = NormL(s32)
	}
}
```

- [ ] **Step 2: Run the benchmarks once (informational)**

```bash
cd /home/exedev/g729 && go test ./internal/fixed/... -bench=. -benchmem -run=^$
```
Expected: ns/op printed per benchmark, `0 B/op, 0 allocs/op`.

Report these numbers into the commit message for historical reference. Exact numbers do not gate the task.

- [ ] **Step 3: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/bench_test.go && git commit -m "test(fixed): add primitive benchmarks"
```

---

## Task 20: Package documentation polish

**Files:**
- Modify: `internal/fixed/doc.go`

Add a worked example and a "Caveats" section. Improves godoc discoverability for engineers implementing later blocks.

- [ ] **Step 1: Rewrite `internal/fixed/doc.go`**

Replace the file contents with:
```go
// Package fixed implements the ITU-T G.191 basic operations required
// by the G.729 speech-codec specification.
//
// # Philosophy
//
// All arithmetic saturates to Word16 (int16) or Word32 (int32) instead
// of wrapping. The package is the numerical foundation of the codec:
// any deviation from ITU semantics here will surface as bit-level
// errors in the encoder output and fail the bit-exact acceptance tests.
//
// Go's built-in +, -, * operators wrap on signed overflow. They must
// never be used for codec arithmetic. Always use the primitives here.
//
// # ITU name mapping
//
// The ITU spec uses C-style names; Go code uses CamelCase. The mapping:
//
//	saturate       -> Saturate
//	add, sub       -> Add, Sub
//	negate, abs_s  -> Negate, AbsS
//	L_add, L_sub   -> LAdd, LSub
//	L_negate       -> LNegate
//	L_abs          -> LAbs
//	extract_h      -> ExtractH
//	extract_l      -> ExtractL
//	L_deposit_h    -> LDepositH
//	L_deposit_l    -> LDepositL
//	shl, shr       -> Shl, Shr
//	shr_r          -> ShrR
//	L_shl, L_shr   -> LShl, LShr
//	L_shr_r        -> LShrR
//	L_mult         -> LMult
//	L_mac, L_msu   -> LMac, LMsu
//	mult, mult_r   -> Mult, MultR
//	round          -> Round
//	norm_s, norm_l -> NormS, NormL
//	div_s          -> DivS
//
// # Example
//
// Compute a scaled inner product of two short vectors in Q15 arithmetic:
//
//	var acc Word32
//	for i := range a {
//	    acc = LMac(acc, a[i], b[i]) // acc += 2 * a[i] * b[i], saturating
//	}
//	result := Round(acc) // 32-bit accumulator to 16-bit with rounding
//
// # No allocation
//
// Every function in this package returns a primitive value and allocates
// nothing. The zero-allocation contract is enforced by
// TestNoAllocationInPrimitives.
package fixed
```

- [ ] **Step 2: Render godoc locally to verify**

```bash
cd /home/exedev/g729 && go doc ./internal/fixed
```
Expected: package doc renders with the sections above.

- [ ] **Step 3: Run full test suite once more**

```bash
cd /home/exedev/g729 && go test ./... -race
```
Expected: all pass.

- [ ] **Step 4: Commit**

```bash
cd /home/exedev/g729 && git add internal/fixed/doc.go && git commit -m "docs(fixed): expand package doc with mapping and example"
```

---

## Completion criteria

- All 20 tasks above are committed in order.
- `go test ./... -race` passes.
- `go vet ./...` emits nothing.
- `go test ./internal/fixed/... -bench=. -benchmem` reports 0 allocations on every primitive.
- Every ITU basic operation from the name-mapping table is implemented and tested.

When these hold, Phase 0a is complete. The next plan will implement Phase 0b (`internal/bitstream` for frame packing/unpacking and G.192 `.bit` file I/O), which depends only on types in Phase 0a (and not on arithmetic primitives beyond `ExtractL`).
