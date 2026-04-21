# Phase 1b — internal/pitch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `internal/pitch` package — decoder-side pitch delay decoding (8-bit P1 + 1-bit parity P0 for subframe 1, 5-bit P2 for subframe 2) and adaptive codebook vector construction via 1/3-sample fractional interpolation on the past-excitation signal. Output a 40-sample adaptive codebook vector `v[40]` (Q0 int16) for each subframe that Phase 1e's synthesis filter and Phase 1d's gain application will consume.

**Architecture:** Stateless public functions: `CheckParity`, `DecodeDelaySubframe1`, `DecodeDelaySubframe2`, `AdaptiveCodebook`. The past-excitation ring buffer is NOT owned by this package — it is Phase 1g's responsibility. `AdaptiveCodebook` takes a caller-owned `[]int16` past-excitation slice and writes into a caller-owned `*[40]int16`. This lets Phase 1b be tested with synthetic excitation (impulse responses, known sinusoids) in isolation, and defers the cross-cutting ring-buffer design to the top-level decoder where the ACELP (Phase 1c), gain (Phase 1d), and synthesis (Phase 1e) sub-blocks also need to read and write it.

**Tech Stack:** Go 1.22+. Depends on `internal/fixed` for saturating arithmetic primitives (`LMac`, `Round`, `Mult`, `LMult`, `Add`, `Sub`, `Shl`) and `internal/tables` for the 1/3-sample interpolation FIR coefficient table introduced by this plan. Scratch-from-spec: algorithm from ITU-T G.729 §3.7 / §4.1.3 / §4.1.4 + Annex A; FIR coefficient table transcribed from the ITU reference distribution's `tab_ld8a.c` data-array initializers under the merger-doctrine exception (see `MEMORY.md` project policy note).

---

## Context for the implementing engineer

### What this package exists for

The G.729 decoder reconstructs the glottal excitation from two additive contributions per subframe:

1. **Adaptive codebook** — a delayed and fractionally-interpolated copy of the past excitation, scaled by the pitch gain `g_p`. This models the periodic (voiced) part of the signal. *This package produces the un-scaled vector.*
2. **Fixed (algebraic) codebook** — 4 sparse unit pulses from the ACELP decoder, scaled by the codebook gain `g_c`. Models the noise-like residual. *Phase 1c's job.*

The adaptive codebook's delay is the pitch period. Because the true pitch may fall between integer sample positions at 8 kHz, G.729 uses 1/3-sample resolution: the transmitted delay is `T = T_int + T_frac/3` with `T_frac ∈ {-1, 0, +1}`. A sinc-based 1/3-sample interpolation FIR reconstructs the sub-sample values from integer-aligned past-excitation samples.

Subframe 1's delay `T1` is coded absolutely (8 bits, range `[19 1/3, 143]`). Subframe 2's delay `T2` is coded relative to a rounded version of `T1` (5 bits, small range around `T1` with 1/3 resolution). This exploits slow pitch evolution within one 10 ms frame.

A 1-bit parity `P0` protects the upper 6 bits of `P1` against single-bit transmission errors; this package reports the parity result, but what to do on parity failure (erasure concealment) is Phase 1h's job.

### ITU-T G.729 sections used in this phase

| Section | Topic | What we read |
|---|---|---|
| §3.7.1 | Closed-loop pitch analysis | Pitch delay encoding convention for P1 / P2, 1/3-sample resolution, interpolation FIR definition |
| §3.7.2 | Computation of the pitch index | Parity protection formula for P0 |
| §3.8 | Adaptive codebook contribution | Equation for `v(n)` in terms of past excitation and interpolation filter |
| §4.1.3 | Decoding of the adaptive codebook vector | Decoder mirror of §3.7.1 — same delay encoding, same interpolation |
| §4.1.4 | Decoding of the pitch parity bit | Decoder mirror of §3.7.2; what the parity bit protects and how to recompute it |

**Read the spec text for every equation, parity formula, bit layout, and FIR coefficient index scheme.** The plan tells you the shape of the math and the interface boundaries; the spec is the authority for the exact bit positions, delay ranges, and filter coefficient layout.

### Pitch delay encoding (plan's derivation — verify against §3.7.1)

The plan states the encoding convention that most G.729 literature uses. If §3.7.1 differs on bit boundaries, ranges, or rounding tie-breaks, trust the spec.

**Subframe 1 (P1, 8 bits, 256 values):**

- Low range: delays in `[19 1/3, 84 2/3]` use full 1/3-sample resolution.
  - 66 integer positions × 3 fractions = 198 combinations, encoded as index `0..197`.
  - Encoding: `index = (T_int − 19) · 3 + (T_frac + 1)` with `T_frac ∈ {-1, 0, 1}`.
  - At the low edge, `T_int = 19, T_frac = -1` decodes to delay `19 − 1/3`; at `T_int = 19, T_frac = 0` to `19`; etc.
- High range: delays in `[85, 143]` use integer resolution only.
  - 59 integer positions, encoded as index `198..256`.
  - Encoding: `index = T_int + 113`.
  - Total 198 + 59 = 257 values, but only 256 fit in 8 bits; one value (typically at the top edge, `T_int = 143`) may be disallowed. Confirm per §3.7.1.

**Subframe 2 (P2, 5 bits, 32 values):**

- Relative to a rounded-to-integer `T1_rounded`: the decoded `T2` covers a range `[T1_rounded − 5 1/3, T1_rounded + 4 2/3]` with 1/3 resolution.
  - 10 integer positions × 3 fractions = 30 values, fitting in 5 bits (with 2 unused).
  - Encoding: `index = (T2_int − (T1_rounded − 5)) · 3 + (T2_frac + 1)`.
- Rounding rule for `T1_rounded`: round the decoded `T1 = T1_int + T1_frac/3` to the nearest integer, with ties broken per spec.

**Parity (P0, 1 bit):**

- Computed over the 6 most significant bits of P1.
- Parity is typically odd-parity over those 6 bits (sum XOR'd with 1 gives 1-bit result), but the exact polynomial is in §3.7.2.

### Adaptive codebook equation (plan's derivation — verify against §3.8 / §4.1.4)

Let `past[]` be the past-excitation signal indexed so that `past[−1]` is the most recent past sample (one before the current subframe's first output sample). The adaptive codebook vector for the current subframe is:

```
v[n] = Σ_{k = −Linter}^{Linter − 1}  past[n − T_int + k] · h[k, T_frac]      for n ∈ [0, 39]
```

where `h[k, t_frac]` is the 1/3-sample interpolation FIR coefficient for tap `k` and fractional offset `t_frac`. `Linter` is the one-sided filter length (spec-defined — in full G.729 the filter is 21 taps, so `Linter = 10`; Annex A uses the same interpolation filter as full G.729).

**Short-pitch handling (T_int < 40):** When the pitch delay is smaller than the subframe length, the equation above reads into the future of the current subframe (positions 0..39 that have not been computed yet). The spec's workaround is to **extend the adaptive codebook by periodicity**:

```
v[n] = v[n − T_int]      for n ≥ T_int
```

So `v[0..T_int − 1]` is computed via the standard interpolation from past excitation; `v[T_int..39]` is a straight copy from the earlier part of `v` itself (period T_int).

Fractional offset `T_frac` still applies to `v[0..T_int − 1]`; the periodicity extension copies already-interpolated samples.

### Past-excitation slice convention

`AdaptiveCodebook` takes `pastExc []int16` as a slice. Convention:

- `pastExc` represents past excitation samples in chronological order.
- `pastExc[len(pastExc) − 1]` is the most recent sample (one before the subframe's first output).
- `pastExc[len(pastExc) − 1 − T_int + k]` addresses the k-th tap at integer delay `T_int`.
- The caller (Phase 1g) must supply enough history: at least `max_T_int + Linter = 143 + 10 = 153` samples.

**Short-pitch exception:** when `T_int < 40`, the function reads past excitation for the first `T_int` output samples, then reads from `v` itself for the rest. So even short-pitch cases need at most `T_int + Linter` samples of past excitation.

### Package layout produced by this plan

```
g729/internal/pitch/
├── doc.go                    (package doc: role, contracts, ITU refs)
├── types.go                  (Indices type)
├── parity.go                 (CheckParity)
├── parity_test.go
├── delay.go                  (DecodeDelaySubframe1, DecodeDelaySubframe2)
├── delay_test.go
├── adaptive.go               (AdaptiveCodebook, including integer + fractional + short-pitch)
├── adaptive_test.go
├── alloc_test.go             (zero-allocation contract)
└── bench_test.go             (per-subframe benchmark)

g729/internal/tables/
└── pitch_interp.go           (1/3-sample interpolation FIR, §3.7.1)
    + add to lsp_tables_test.go or new pitch_interp_test.go for shape/range
```

### Dependency contract

- `internal/pitch` imports `internal/fixed` and `internal/tables` only.
- Every arithmetic step routes through `internal/fixed`. Built-in `+`, `-`, `*` on `int16` / `int32` is forbidden in codec paths (wraps on overflow). Same rule as prior phases.
- No allocation in any public function. All outputs are written via `*[N]intX` pointers; slices passed in are caller-owned.
- **No package-level state.** All functions are pure; no `Decoder` struct. Phase 1g's top-level decoder will own the past-excitation buffer and call these functions per subframe.

### Indices type

The pitch indices as delivered by the bitstream unpacker:

```go
// Indices are the pitch parameters for one G.729 frame.
type Indices struct {
    P1 uint8 // 8 bits: subframe-1 pitch delay index (0..255)
    P0 uint8 // 1 bit:  parity check bit for P1 (0 or 1)
    P2 uint8 // 5 bits: subframe-2 pitch delay index (0..31)
}
```

The bitstream package produces a larger parameter bag; `pitch.Indices` is a subset. Phase 1g will assemble `pitch.Indices` from the bitstream output.

### Verification strategy

No ITU test-vector integration in Phase 1b (that's Phase 1g's job). Correctness rests on:

- **Parity:** exhaustive enumeration over the 256 × 2 combinations. Hand-compute the parity formula from §3.7.2 and cross-check against `CheckParity` for the full table.
- **Delay decode:** boundary tests — the smallest, largest, and boundary-between-subranges indices for P1; min and max values for P2 at several `T1_rounded` choices. Hand-compute expected `T_int`, `T_frac` from the spec's encoding formula.
- **Adaptive codebook — integer delay:** construct `pastExc` with a known pattern (e.g. `pastExc[i] = int16(i + 1)` for clarity) and check that `v[n]` with `T_int=50, T_frac=0` is a direct copy of `pastExc[len-51..len-12]`.
- **Adaptive codebook — fractional delay:** construct `pastExc = ones`, compare `v` at `T_frac=0` (should be all-ones) to `v` at `T_frac=1` (should still be approximately ones — the sum of the FIR filter's coefficients at any fractional offset is 1.0 Q15, within rounding). This is the "partition of unity" check.
- **Adaptive codebook — short pitch:** construct a `pastExc` with a recognizable tail (e.g. pastExc[last 20] = [1,2,...,20]) and call with `T_int=20, T_frac=0`. Expect `v[0..19] = pastExc[last 20]` and `v[20..39] = v[0..19]`.
- **FIR partition of unity:** `Σ_{k = -Linter}^{Linter − 1} h[k, t_frac] ≈ 2^15` for each `t_frac ∈ {-1, 0, 1}`. Within rounding tolerance (a few LSB).
- **Zero allocation:** `testing.AllocsPerRun = 0` on every public function.

### Frame cadence

`AdaptiveCodebook` is called **once per subframe** (twice per frame). This package has no per-frame state; all state (past-excitation ring buffer, previous delay for subframe-2 relative decoding) is held by the caller.

### Verification commands (run after every task)

- `go test ./internal/pitch/... ./internal/tables/... -race` — must PASS.
- `go vet ./internal/pitch/... ./internal/tables/...` — must print nothing.

Final (Task 9 completion criteria):
- `go test -run TestNoAllocation -v ./internal/pitch/...` — zero-alloc sub-tests PASS.
- `go test -bench=. -benchmem -run=^$ ./internal/pitch/...` — `0 B/op, 0 allocs/op` on benchmarks.

---

## Task 1: Package skeleton + Indices type

**Files:**
- Create: `internal/pitch/doc.go`
- Create: `internal/pitch/types.go`
- Create: `internal/pitch/types_test.go`

Stand up the package with the `Indices` value type before any algorithm lands.

- [ ] **Step 1: Write the failing shape test**

Create `internal/pitch/types_test.go`:

```go
package pitch

import "testing"

func TestIndicesZeroValue(t *testing.T) {
    var idx Indices
    if idx.P1 != 0 || idx.P0 != 0 || idx.P2 != 0 {
        t.Fatalf("zero-value Indices = %+v, want all zero", idx)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/pitch/ -run TestIndicesZeroValue -v`
Expected: FAIL with "undefined: Indices".

- [ ] **Step 3: Create `types.go`**

```go
package pitch

// Indices are the pitch-related bit-field values delivered per frame
// by the bitstream unpacker. Values are raw integer indices, not
// bit-slices.
type Indices struct {
    P1 uint8 // 8 bits — subframe-1 pitch delay index (0..255)
    P0 uint8 // 1 bit  — parity check bit for P1
    P2 uint8 // 5 bits — subframe-2 pitch delay index (0..31)
}
```

- [ ] **Step 4: Create a minimal `doc.go`**

```go
// Package pitch implements ITU-T G.729 + Annex A §3.7 / §4.1.3 /
// §4.1.4 adaptive-codebook decoding: pitch delay reconstruction
// from the transmitted P1, P0, P2 bit fields and construction of
// the 40-sample adaptive codebook vector for one subframe via
// 1/3-sample fractional interpolation of past excitation.
//
// All public functions are stateless. The past-excitation signal is
// owned by the caller (Phase 1g's top-level decoder); each
// AdaptiveCodebook call writes into a caller-owned output array.
package pitch
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/pitch/ -run TestIndicesZeroValue -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/pitch/doc.go internal/pitch/types.go internal/pitch/types_test.go
git commit -m "feat(pitch): package skeleton + Indices type"
```

---

## Task 2: Parity check

**Files:**
- Create: `internal/pitch/parity.go`
- Create: `internal/pitch/parity_test.go`

Per ITU-T G.729 §3.7.2, P0 is a 1-bit parity computed over the 6 most significant bits of P1. The decoder recomputes the parity and compares to the received P0; if they match, the frame is good with respect to pitch; otherwise the frame is flagged for erasure concealment (handled elsewhere).

**The exact parity polynomial is in §3.7.2.** A very common convention is odd parity:

```
P0 = 1 ⊕ b7 ⊕ b6 ⊕ b5 ⊕ b4 ⊕ b3 ⊕ b2
```

where `b7..b0` are bits of P1 with `b7` most significant. Odd parity means `XOR of all bits (including P0) equals 1` — i.e. the total count of 1-bits among `{b7, b6, b5, b4, b3, b2, P0}` is odd.

Verify whether §3.7.2 uses odd or even parity, and whether the bit range is `b7..b2` (upper 6) or another selection.

- [ ] **Step 1: Write the failing parity test**

Create `internal/pitch/parity_test.go`:

```go
package pitch

import "testing"

// ExhaustiveParity verifies that CheckParity returns true iff the
// received P0 matches the parity recomputed from P1's upper 6 bits
// under the spec's odd-parity convention. If §3.7.2 uses a different
// bit selection or even parity, update expectedParity below to match.
func expectedParity(p1 uint8) uint8 {
    // Odd parity over b7..b2 (upper 6 bits of P1).
    bits := (p1 >> 2) & 0x3F
    x := bits ^ (bits >> 4)
    x ^= x >> 2
    x ^= x >> 1
    return (x & 1) ^ 1 // flip for ODD parity; remove flip for even
}

func TestCheckParityExhaustive(t *testing.T) {
    matchCount := 0
    for p1 := 0; p1 < 256; p1++ {
        expected := expectedParity(uint8(p1))
        for p0 := uint8(0); p0 <= 1; p0++ {
            got := CheckParity(uint8(p1), p0)
            want := p0 == expected
            if got != want {
                t.Errorf("CheckParity(p1=%d, p0=%d) = %v, want %v (expected parity %d)",
                    p1, p0, got, want, expected)
            }
            if got {
                matchCount++
            }
        }
    }
    // Exactly half of (p1, p0) combinations must pass — one value of
    // p0 matches and one doesn't, for each of 256 p1 values.
    if matchCount != 256 {
        t.Errorf("parity matches = %d, want 256 (half of 512 combinations)", matchCount)
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/pitch/ -run TestCheckParity -v`
Expected: FAIL with "undefined: CheckParity".

- [ ] **Step 3: Implement `CheckParity`**

Create `internal/pitch/parity.go`:

```go
package pitch

// CheckParity verifies the received parity bit P0 against the parity
// recomputed from the upper 6 bits of P1, per ITU-T G.729 §3.7.2.
// Returns true when the received P0 matches the expected value (i.e.
// the frame's pitch bits pass the parity check). On mismatch, the
// caller should flag this subframe for erasure concealment.
//
// Parity convention: odd parity over bits b7..b2 of P1 — the XOR of
// those 6 bits plus P0 must equal 1. Verify against §3.7.2 and adjust
// the inversion on the final line if the spec uses even parity.
func CheckParity(p1, p0 uint8) bool {
    bits := (p1 >> 2) & 0x3F
    x := bits ^ (bits >> 4)
    x ^= x >> 2
    x ^= x >> 1
    expected := (x & 1) ^ 1 // ODD parity; remove the `^ 1` for even
    return p0 == expected
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/pitch/ -run TestCheckParity -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pitch/parity.go internal/pitch/parity_test.go
git commit -m "feat(pitch): parity check on P1 upper 6 bits"
```

---

## Task 3: Subframe-1 pitch delay decode

**Files:**
- Create: `internal/pitch/delay.go`
- Create: `internal/pitch/delay_test.go`

Decode P1 (8 bits) into integer and fractional pitch delay. Return `T_int` in `[19, 143]` and `T_frac` in `{-1, 0, +1}`.

- [ ] **Step 1: Write the failing subframe-1 decode test**

Create `internal/pitch/delay_test.go`:

```go
package pitch

import "testing"

// Boundary-case table derived from ITU-T G.729 §3.7.1 pitch encoding.
// If the spec disagrees on any row (e.g. bit boundary at a different
// index, or different rounding), trust the spec and update this table.
var subframe1Cases = []struct {
    p1       uint8
    wantInt  int
    wantFrac int
}{
    {0, 19, -1},  // lowest delay: 19 − 1/3
    {1, 19, 0},   // 19
    {2, 19, 1},   // 19 + 1/3
    {3, 20, -1},  // 20 − 1/3
    {197, 84, 1}, // last index of the 1/3-resolution range: 84 + 1/3
    {198, 85, 0}, // first integer-only index: 85
    {199, 86, 0}, // 86
    {255, 142, 0}, // 142 (confirm: ITU may allow up to 143)
}

func TestDecodeDelaySubframe1Boundaries(t *testing.T) {
    for _, tc := range subframe1Cases {
        gotInt, gotFrac := DecodeDelaySubframe1(tc.p1)
        if gotInt != tc.wantInt || gotFrac != tc.wantFrac {
            t.Errorf("DecodeDelaySubframe1(%d) = (%d, %d), want (%d, %d)",
                tc.p1, gotInt, gotFrac, tc.wantInt, tc.wantFrac)
        }
    }
}

func TestDecodeDelaySubframe1RangeInvariants(t *testing.T) {
    for p1 := 0; p1 < 256; p1++ {
        tInt, tFrac := DecodeDelaySubframe1(uint8(p1))
        if tInt < 19 || tInt > 143 {
            t.Errorf("p1=%d → T_int=%d, out of [19, 143]", p1, tInt)
        }
        if tFrac < -1 || tFrac > 1 {
            t.Errorf("p1=%d → T_frac=%d, out of {-1, 0, 1}", p1, tFrac)
        }
    }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/pitch/ -run TestDecodeDelaySubframe1 -v`
Expected: FAIL with "undefined: DecodeDelaySubframe1".

- [ ] **Step 3: Implement `DecodeDelaySubframe1`**

Create `internal/pitch/delay.go`:

```go
package pitch

// DecodeDelaySubframe1 reconstructs the subframe-1 pitch delay from
// the 8-bit P1 index, per ITU-T G.729 §3.7.1.
//
// Returns (T_int, T_frac) with:
//   T_int  ∈ [19, 143]
//   T_frac ∈ {-1, 0, 1} representing sub-sample offsets {-1/3, 0, 1/3}
//
// Encoding convention (verify against §3.7.1):
//   P1 ∈ [0, 197]:     T_int = 19 + P1 / 3, T_frac = (P1 % 3) − 1
//   P1 ∈ [198, 255]:   T_int = P1 − 112, T_frac = 0
func DecodeDelaySubframe1(p1 uint8) (tInt, tFrac int) {
    if p1 < 198 {
        tInt = 19 + int(p1)/3
        tFrac = int(p1)%3 - 1
        return
    }
    tInt = int(p1) - 112 // 198 → 86, 255 → 143. Verify low-edge offset per §3.7.1.
    tFrac = 0
    return
}
```

**Verify each boundary against §3.7.1 before committing.** The plan's encoding is the conventional G.729 formulation; the spec is the tiebreaker for:

1. Whether `P1 = 0` maps to `T = 19 − 1/3` or `T = 19 + 0` (rounding direction of the formula).
2. The exact split point between the 1/3-resolution range and the integer-only range.
3. Whether `P1 = 255` maps to `T_int = 143` (my table says 142) or something else.

Update both `DecodeDelaySubframe1` and the test's `subframe1Cases` if the spec differs.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/pitch/ -run TestDecodeDelaySubframe1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pitch/delay.go internal/pitch/delay_test.go
git commit -m "feat(pitch): subframe-1 pitch delay decoding"
```

---

## Task 4: Subframe-2 pitch delay decode (relative)

**Files:**
- Modify: `internal/pitch/delay.go`
- Modify: `internal/pitch/delay_test.go`

`P2` encodes `T2` relative to `T1_rounded` (round-to-nearest integer of `T1 = T1_int + T1_frac/3`, tie-break per spec).

Range: `T2 ∈ [T1_rounded − 5 1/3, T1_rounded + 4 2/3]` with 1/3 resolution → 30 values in 5 bits.

- [ ] **Step 1: Add the failing subframe-2 decode test**

Append to `internal/pitch/delay_test.go`:

```go
// RoundT1 implements the rounding rule from §3.7.1 for converting
// T1 = T1_int + T1_frac/3 to the integer reference used by subframe 2.
// T_frac ∈ {-1, 0, 1} → {-1/3, 0, 1/3}. The conventional round-to-
// nearest with ties-to-even is implemented below; verify against §3.7.1.
func roundT1(tInt, tFrac int) int {
    switch tFrac {
    case -1:
        return tInt // -1/3 rounds down to the integer
    case 0:
        return tInt
    case 1:
        return tInt + 1 // +1/3 rounds up. Confirm tie-break per spec.
    }
    return tInt
}

func TestDecodeDelaySubframe2Center(t *testing.T) {
    // With T1 = 50 (rounded), P2 = 15 should decode to a delay near
    // the center of the ±5 range. Under the encoding
    //   index = (T2_int - (T1_rounded - 5)) * 3 + (T2_frac + 1)
    // index=15 → T2_int - 45 = 5, T2_frac = 0 → T2_int=50, T2_frac=0.
    // Actually: (T2_int - 45)*3 + (T2_frac+1) = 15
    //   one solution: T2_int=49, T2_frac=0 → (4)*3 + 1 = 13 (no)
    //   T2_int=50, T2_frac=-1 → (5)*3 + 0 = 15 ✓
    // So P2=15 decodes to (50, -1). Verify vs spec.
    gotInt, gotFrac := DecodeDelaySubframe2(15, 50)
    if gotInt != 50 || gotFrac != -1 {
        t.Errorf("DecodeDelaySubframe2(15, 50) = (%d, %d), want (50, -1)",
            gotInt, gotFrac)
    }
}

func TestDecodeDelaySubframe2BoundaryIndices(t *testing.T) {
    // At p2=0, the decoded delay is the lowest in the ±5 window:
    //   T2_int = T1_rounded - 5, T2_frac = -1 (i.e. T1_rounded - 5 - 1/3).
    gotInt, gotFrac := DecodeDelaySubframe2(0, 60)
    if gotInt != 55 || gotFrac != -1 {
        t.Errorf("DecodeDelaySubframe2(0, 60) = (%d, %d), want (55, -1)",
            gotInt, gotFrac)
    }

    // At p2=29, the highest valid index:
    //   (T2_int - 55)*3 + (T2_frac+1) = 29
    //   T2_int=64, T2_frac=1 → 9*3+2 = 29 ✓
    gotInt, gotFrac = DecodeDelaySubframe2(29, 60)
    if gotInt != 64 || gotFrac != 1 {
        t.Errorf("DecodeDelaySubframe2(29, 60) = (%d, %d), want (64, 1)",
            gotInt, gotFrac)
    }
}

func TestDecodeDelaySubframe2LowerClamp(t *testing.T) {
    // When T1_rounded is small, T2 can go below 19. Spec §3.7.1
    // clamps T2 to the global minimum 19. Verify the clamp direction
    // per spec; this test assumes a clamp. If §3.7.1 leaves it
    // unclamped, remove this test.
    gotInt, _ := DecodeDelaySubframe2(0, 20)
    if gotInt < 19 {
        t.Errorf("DecodeDelaySubframe2(0, 20) = T_int=%d, want clamped to >= 19", gotInt)
    }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/pitch/ -run TestDecodeDelaySubframe2 -v`
Expected: FAIL with "undefined: DecodeDelaySubframe2".

- [ ] **Step 3: Implement `DecodeDelaySubframe2`**

Append to `internal/pitch/delay.go`:

```go
// DecodeDelaySubframe2 reconstructs the subframe-2 pitch delay from
// the 5-bit P2 index, relative to the rounded subframe-1 delay
// t1Rounded. Per ITU-T G.729 §3.7.1:
//
//   T2_int  in [t1Rounded − 5, t1Rounded + 4]
//   T2_frac in {-1, 0, 1}
//
// Encoding (plan's derivation — verify against §3.7.1):
//
//   index = (T2_int − (t1Rounded − 5)) · 3 + (T2_frac + 1)
//   index ∈ [0, 29]; values 30 and 31 are reserved/unused.
//
// The function also clamps T2_int to the global pitch range [19, 143];
// §3.7.1 specifies how to handle out-of-range cases — confirm the
// clamp direction matches the spec.
func DecodeDelaySubframe2(p2 uint8, t1Rounded int) (tInt, tFrac int) {
    base := t1Rounded - 5
    tInt = base + int(p2)/3
    tFrac = int(p2)%3 - 1

    // Global clamp per §3.7.1.
    if tInt < 19 {
        tInt = 19
        tFrac = 0
    }
    if tInt > 143 {
        tInt = 143
        tFrac = 0
    }
    return
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/pitch/ -run TestDecodeDelaySubframe2 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pitch/delay.go internal/pitch/delay_test.go
git commit -m "feat(pitch): subframe-2 relative pitch delay decoding"
```

---

## Task 5: Pitch interpolation FIR table

**Files:**
- Create: `internal/tables/pitch_interp.go`
- Create: `internal/tables/pitch_interp_test.go`

ITU-T G.729 §3.7.1 defines the 1/3-sample interpolation FIR filter for reconstructing fractional-delay samples. The filter is a truncated Hamming-windowed sinc with 21 integer-aligned taps (one-sided length `Linter = 10`). Evaluated at fractional offsets `t_frac ∈ {-1/3, 0, +1/3}` it yields three coefficient sets of 21 values each, but because the middle offset 0 is the identity (all zeros except a 1 at the center), only two independent coefficient sets are stored and used.

A common storage layout (used by the ITU reference distribution) is a single flat table of length `3 · (Linter + 1) = 33` or similar, with indexing arithmetic to pick the right tap for a given `(k, t_frac)`. The exact table length and the index formula are in §3.7.1 / §4.1.3.

Transcribe from the ITU reference distribution's `tab_ld8a.c` data-array initializer (per the MEMORY.md merger-doctrine policy). **Do NOT read any algorithmic C file**, only the flat numerical initializer.

- [ ] **Step 1: Write the failing shape test**

Create `internal/tables/pitch_interp_test.go`:

```go
package tables

import "testing"

// PitchInterpFIR shape check. Length is spec-specified; if §3.7.1
// quotes a different size, update the want constant and the table.
func TestPitchInterpFIRShape(t *testing.T) {
    const want = 33 // placeholder — verify per §3.7.1 / tab_ld8a.c
    if len(PitchInterpFIR) != want {
        t.Fatalf("PitchInterpFIR: entries = %d, want %d", len(PitchInterpFIR), want)
    }
}

func TestPitchInterpFIRRange(t *testing.T) {
    // Coefficients are Q15 int16; individual taps can be negative and
    // are bounded in magnitude by ±Max16.
    for i, v := range PitchInterpFIR {
        if v < -32768 || v > 32767 {
            t.Errorf("PitchInterpFIR[%d] = %d outside int16 range", i, v)
        }
    }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/tables/ -run TestPitchInterpFIR -v`
Expected: FAIL with "undefined: PitchInterpFIR".

- [ ] **Step 3: Transcribe the FIR table**

Create `internal/tables/pitch_interp.go`:

```go
package tables

// PitchInterpFIR is the 1/3-sample fractional-delay interpolation
// filter used by the adaptive codebook decoder, from ITU-T G.729
// §3.7.1.
//
// Storage layout (verify against §3.7.1 / tab_ld8a.c):
//   The table is a flat sequence indexed by (k, t_frac) via the
//   spec's index arithmetic. Coefficients are Q15 Word16, windowed-
//   sinc taps. The middle fractional offset (t_frac = 0) is the
//   identity — most implementations omit its explicit storage and
//   handle the integer-delay case as a fast path.
//
// Transcribed from the ITU reference distribution's tab_ld8a.c
// data-array initializer under the merger-doctrine exception: pure
// numerical VoIP-interop data has no creative expression. No
// algorithmic ITU C source was consulted. See MEMORY.md project
// policy for the full rationale.
var PitchInterpFIR = [33]int16{
    // ... Q15 tap values transcribed from §3.7.1 / tab_ld8a.c
}
```

Adjust the array length (`[33]int16`) to match whatever the spec specifies. Update `TestPitchInterpFIRShape`'s `want` constant to match.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/tables/ -run TestPitchInterpFIR -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tables/pitch_interp.go internal/tables/pitch_interp_test.go
git commit -m "feat(tables): add 1/3-sample pitch interpolation FIR from ITU §3.7.1"
```

---

## Task 6: Adaptive codebook with integer delay

**Files:**
- Create: `internal/pitch/adaptive.go`
- Create: `internal/pitch/adaptive_test.go`

Implement the integer-delay fast path first (T_frac = 0): `v[n] = pastExc[len - 1 - T_int + n]` for n in [0, 39], assuming T_int ≥ 40 (short-pitch extension is Task 8).

This is a pure copy with no multiplications. It lets the filter-based implementation (Task 7) be tested against a zero-fraction ground truth.

- [ ] **Step 1: Write the failing integer-delay test**

Create `internal/pitch/adaptive_test.go`:

```go
package pitch

import "testing"

func TestAdaptiveCodebookIntegerDelay(t *testing.T) {
    // pastExc[i] = int16(i). len = 200. The last 160 samples are
    // pastExc[40..199], so T_int=50 looks back 50 samples: v[0] =
    // pastExc[199 - 50 + 0] = pastExc[149] = 149.
    var pastExc [200]int16
    for i := range pastExc {
        pastExc[i] = int16(i)
    }
    var v [40]int16
    AdaptiveCodebook(50, 0, pastExc[:], &v)
    for n := 0; n < 40; n++ {
        want := int16(149 + n) // pastExc[199 - 50 + n]
        if v[n] != want {
            t.Errorf("v[%d] = %d, want %d (integer delay 50)", n, v[n], want)
        }
    }
}

func TestAdaptiveCodebookIntegerDelayLargest(t *testing.T) {
    // T_int = 143 (max) requires 143 + 40 = 183 samples of past excitation
    // at minimum. Use 200 for safety.
    var pastExc [200]int16
    for i := range pastExc {
        pastExc[i] = int16(i - 100) // values span negative and positive ranges
    }
    var v [40]int16
    AdaptiveCodebook(143, 0, pastExc[:], &v)
    for n := 0; n < 40; n++ {
        want := pastExc[199-143+n]
        if v[n] != want {
            t.Errorf("v[%d] = %d, want %d (integer delay 143)", n, v[n], want)
        }
    }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/pitch/ -run TestAdaptiveCodebook -v`
Expected: FAIL with "undefined: AdaptiveCodebook".

- [ ] **Step 3: Implement the integer-delay path**

Create `internal/pitch/adaptive.go`:

```go
package pitch

// AdaptiveCodebook fills v[40] with the 40-sample adaptive codebook
// vector for one subframe, reading from the past-excitation slice at
// an integer delay tInt plus a fractional offset tFrac ∈ {-1, 0, 1}
// representing {-1/3, 0, +1/3}. Implements ITU-T G.729 §3.8 / §4.1.4.
//
// pastExc convention:
//   pastExc[len(pastExc) - 1] is the most recent past sample, i.e.
//   the sample immediately before the current subframe's first
//   output. The caller must supply enough history: at least
//   tInt + Linter samples.
//
// When tInt < 40, the function extends the adaptive codebook by
// periodicity: v[n] = v[n - tInt] for n >= tInt.
//
// AdaptiveCodebook allocates nothing.
func AdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[40]int16) {
    // Integer-delay fast path. Fractional offsets handled in Task 7.
    // Short-pitch extension handled in Task 8.
    if tFrac == 0 && tInt >= 40 {
        base := len(pastExc) - tInt
        for n := 0; n < 40; n++ {
            v[n] = pastExc[base+n]
        }
        return
    }
    // Other cases: zero-fill stub — replaced in Tasks 7 and 8.
    for n := 0; n < 40; n++ {
        v[n] = 0
    }
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/pitch/ -run TestAdaptiveCodebook -v`
Expected: PASS (both integer-delay subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/pitch/adaptive.go internal/pitch/adaptive_test.go
git commit -m "feat(pitch): adaptive codebook with integer delay"
```

---

## Task 7: Adaptive codebook with fractional interpolation

**Files:**
- Modify: `internal/pitch/adaptive.go`
- Modify: `internal/pitch/adaptive_test.go`

Extend `AdaptiveCodebook` to handle `T_frac = ±1` by applying the 1/3-sample interpolation FIR:

```
v[n] = Σ_{k = -Linter + 1}^{Linter}  pastExc[len - 1 - T_int + n + k] · h[k, T_frac]
```

where `h[k, t_frac]` is the FIR coefficient for tap offset `k` and fractional index `t_frac`. The table `tables.PitchInterpFIR` stores these coefficients per the spec's index scheme (§3.7.1).

**Q-format:** FIR taps are Q15, past excitation is Q0 int16, accumulator is Q(15+0+1)=Q16 Word32 via `fixed.LMac` (one LMac per tap contributes `2·h·e`, which places the product in Q16). After `Linter · 2 = 20` LMac calls the accumulator holds the sum in Q16; `fixed.Round` converts to Q0 Word16 with saturation.

The interpolation index-into-PitchInterpFIR for a given `(k, t_frac)` depends on the spec's table layout. The pseudocode below assumes a `h[3][Linter + 1]`-like shape where `h[0][·]` is t_frac = −1/3, `h[1][·]` is t_frac = 0 (identity), `h[2][·]` is t_frac = +1/3; your implementation must match the actual layout transcribed in Task 5.

- [ ] **Step 1: Add the failing fractional-delay tests**

Append to `internal/pitch/adaptive_test.go`:

```go
func TestAdaptiveCodebookFractionalPartitionOfUnity(t *testing.T) {
    // Past excitation = all ones → v[n] should be ≈ 1 for any
    // fractional offset (the FIR is a partition of unity within
    // rounding tolerance of a few LSB).
    var pastExc [200]int16
    for i := range pastExc {
        pastExc[i] = 1
    }
    for _, tFrac := range []int{-1, 0, 1} {
        var v [40]int16
        AdaptiveCodebook(50, tFrac, pastExc[:], &v)
        for n := 0; n < 40; n++ {
            if v[n] < 0 || v[n] > 2 {
                t.Errorf("v[%d] = %d at tFrac=%d, want ≈ 1 (partition of unity)",
                    n, v[n], tFrac)
            }
        }
    }
}

func TestAdaptiveCodebookFractionalVariesWithTFrac(t *testing.T) {
    // Non-constant pastExc should produce different v for different
    // tFrac values (otherwise the interpolator is not wired).
    var pastExc [200]int16
    for i := range pastExc {
        pastExc[i] = int16((i * 37) & 0x3FFF) // pseudorandom-ish
    }
    var v0, vNeg, vPos [40]int16
    AdaptiveCodebook(50, 0, pastExc[:], &v0)
    AdaptiveCodebook(50, -1, pastExc[:], &vNeg)
    AdaptiveCodebook(50, 1, pastExc[:], &vPos)

    if v0 == vNeg {
        t.Error("AdaptiveCodebook tFrac=0 and tFrac=-1 produced identical output")
    }
    if v0 == vPos {
        t.Error("AdaptiveCodebook tFrac=0 and tFrac=+1 produced identical output")
    }
    if vNeg == vPos {
        t.Error("AdaptiveCodebook tFrac=-1 and tFrac=+1 produced identical output")
    }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/pitch/ -run TestAdaptiveCodebookFractional -v`
Expected: FAIL with "v[n] = 0 ... want ≈ 1" from the partition-of-unity test (the current stub zeros out the fractional case).

- [ ] **Step 3: Replace the stub with the FIR-based interpolation**

Modify `internal/pitch/adaptive.go`:

```go
package pitch

import (
    "github.com/exedev/g729/internal/fixed"
    "github.com/exedev/g729/internal/tables"
)

// Linter is the one-sided length of the pitch interpolation FIR.
// Filter total length = 2 · Linter taps covering [-Linter+1, Linter].
// Verify per §3.7.1 — ITU's simplified Annex A filter may have a
// different span than full G.729's.
const Linter = 10

// firCoeff returns the Q15 interpolation FIR coefficient for tap
// offset k (k ∈ [-Linter+1, Linter]) at fractional position tFrac
// (∈ {-1, 0, 1}). The lookup into tables.PitchInterpFIR follows the
// layout used in the spec / tab_ld8a.c initializer — confirm when
// Task 5's table is in place and adjust the index arithmetic if
// needed.
func firCoeff(k, tFrac int) int16 {
    if tFrac == 0 {
        if k == 0 {
            return fixed.Max16 // identity: 1.0 Q15
        }
        return 0
    }
    // Plan's placeholder layout: the table stores one half of the
    // symmetric filter indexed by |k| with a stride of 2 between the
    // tFrac=-1 and tFrac=+1 sets. The spec's actual layout may
    // differ; match tab_ld8a.c's initializer exactly.
    //
    // Example addressing (verify):
    //   tFrac = -1 at tap k:  tables.PitchInterpFIR[3*abs(k) + sign(k)]
    //   tFrac = +1 at tap k:  tables.PitchInterpFIR[3*abs(k) - sign(k)]
    //
    // Because the spec's table layout is non-trivial, implement
    // firCoeff after you have Task 5's table loaded and can inspect
    // the initializer's structure.
    return 0 // temporary; overwritten once the spec layout is confirmed
}

// AdaptiveCodebook fills v[40] with the 40-sample adaptive codebook
// vector for one subframe, reading from the past-excitation slice at
// an integer delay tInt plus a fractional offset tFrac ∈ {-1, 0, 1}
// representing {-1/3, 0, +1/3}. Implements ITU-T G.729 §3.8 / §4.1.4.
//
// pastExc convention:
//   pastExc[len(pastExc) - 1] is the most recent past sample. The
//   caller must supply at least tInt + Linter samples of history.
//
// When tInt < 40 the function extends the adaptive codebook by
// periodicity: v[n] = v[n - tInt] for n >= tInt (Task 8).
func AdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[40]int16) {
    if tFrac == 0 && tInt >= 40 {
        base := len(pastExc) - tInt
        for n := 0; n < 40; n++ {
            v[n] = pastExc[base+n]
        }
        return
    }

    if tInt >= 40 {
        interpolate(tInt, tFrac, pastExc, v, 0, 40)
        return
    }

    // Short-pitch extension: handled in Task 8.
    for n := 0; n < 40; n++ {
        v[n] = 0
    }
}

// interpolate fills v[start:end] with the FIR-based fractional-delay
// interpolation of pastExc. Separated so Task 8's short-pitch path
// can call it for just the first tInt samples.
func interpolate(tInt, tFrac int, pastExc []int16, v *[40]int16, start, end int) {
    base := len(pastExc) - 1 - tInt
    for n := start; n < end; n++ {
        var acc fixed.Word32
        for k := -Linter + 1; k <= Linter; k++ {
            e := pastExc[base+n+k]
            h := firCoeff(k, tFrac)
            acc = fixed.LMac(acc, h, e)
        }
        v[n] = fixed.Round(acc)
    }
}
```

**`firCoeff` needs the spec-exact index formula.** Implement it once Task 5's table is landed — the initializer's structure (striding, sign convention, offset) dictates the arithmetic. The partition-of-unity test in Step 1 will catch gross indexing errors; the "varies with tFrac" test will catch flat-out-wrong lookups.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/pitch/ -run TestAdaptiveCodebook -v`
Expected: PASS (integer-delay, partition-of-unity, and varies-with-tFrac subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/pitch/adaptive.go internal/pitch/adaptive_test.go
git commit -m "feat(pitch): 1/3-sample fractional FIR interpolation in adaptive codebook"
```

---

## Task 8: Short-pitch (T < 40) periodicity extension

**Files:**
- Modify: `internal/pitch/adaptive.go`
- Modify: `internal/pitch/adaptive_test.go`

When `T_int < 40`, only the first `T_int` output samples `v[0..T_int-1]` can be computed from past excitation. The remaining samples are a straight copy:

```
v[n] = v[n - T_int]      for n ∈ [T_int, 40)
```

This uses `v` itself as if past excitation were periodic with period `T_int`. Applies to both `T_frac = 0` (straight copy for first T_int) and fractional (FIR-interpolated for first T_int).

- [ ] **Step 1: Add the failing short-pitch tests**

Append to `internal/pitch/adaptive_test.go`:

```go
func TestAdaptiveCodebookShortPitchIntegerDelay(t *testing.T) {
    // T_int = 20, T_frac = 0. pastExc's last 20 samples are [1..20].
    // v[0..19] should be [1..20]; v[20..39] should repeat [1..20].
    var pastExc [200]int16
    for i := 180; i < 200; i++ {
        pastExc[i] = int16(i - 179) // pastExc[180..199] = 1..20
    }
    var v [40]int16
    AdaptiveCodebook(20, 0, pastExc[:], &v)

    for n := 0; n < 20; n++ {
        want := int16(n + 1)
        if v[n] != want {
            t.Errorf("v[%d] = %d, want %d (pre-replication window)", n, v[n], want)
        }
    }
    for n := 20; n < 40; n++ {
        want := v[n-20]
        if v[n] != want {
            t.Errorf("v[%d] = %d, want v[%d] = %d (replicated)", n, v[n], n-20, want)
        }
    }
}

func TestAdaptiveCodebookShortPitchBoundary(t *testing.T) {
    // T_int = 39 (just below the short/long boundary). v[0..38] from
    // pastExc, v[39] = v[0].
    var pastExc [200]int16
    for i := 100; i < 200; i++ {
        pastExc[i] = int16(i - 100)
    }
    var v [40]int16
    AdaptiveCodebook(39, 0, pastExc[:], &v)
    if v[39] != v[0] {
        t.Errorf("v[39] = %d, want v[0] = %d (T_int=39 replication at last sample)",
            v[39], v[0])
    }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/pitch/ -run TestAdaptiveCodebookShortPitch -v`
Expected: FAIL — the current `AdaptiveCodebook` zero-fills when `tInt < 40`.

- [ ] **Step 3: Implement the short-pitch path**

Modify the `AdaptiveCodebook` function body, replacing the zero-fill fallback:

```go
func AdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[40]int16) {
    if tFrac == 0 && tInt >= 40 {
        base := len(pastExc) - tInt
        for n := 0; n < 40; n++ {
            v[n] = pastExc[base+n]
        }
        return
    }

    if tInt >= 40 {
        interpolate(tInt, tFrac, pastExc, v, 0, 40)
        return
    }

    // Short pitch (tInt < 40): fill v[0..tInt-1] via the normal path,
    // then replicate v[n] = v[n - tInt] for n in [tInt, 40).
    if tFrac == 0 {
        base := len(pastExc) - tInt
        for n := 0; n < tInt; n++ {
            v[n] = pastExc[base+n]
        }
    } else {
        interpolate(tInt, tFrac, pastExc, v, 0, tInt)
    }
    for n := tInt; n < 40; n++ {
        v[n] = v[n-tInt]
    }
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/pitch/ -run TestAdaptiveCodebook -v`
Expected: PASS (all subtests: integer-delay, fractional, short-pitch).

- [ ] **Step 5: Commit**

```bash
git add internal/pitch/adaptive.go internal/pitch/adaptive_test.go
git commit -m "feat(pitch): short-pitch periodicity extension (T_int < 40)"
```

---

## Task 9: Zero-allocation contract + benchmark + doc polish

**Files:**
- Create: `internal/pitch/alloc_test.go`
- Create: `internal/pitch/bench_test.go`
- Modify: `internal/pitch/doc.go`

Lock in the zero-allocation hot path for all public functions and add canonical benchmarks.

- [ ] **Step 1: Write the failing allocation tests**

Create `internal/pitch/alloc_test.go`:

```go
package pitch

import "testing"

func TestNoAllocationInCheckParity(t *testing.T) {
    allocs := testing.AllocsPerRun(128, func() {
        _ = CheckParity(123, 0)
    })
    if allocs != 0 {
        t.Fatalf("CheckParity allocated %.2f times per call; want 0", allocs)
    }
}

func TestNoAllocationInDecodeDelaySubframe1(t *testing.T) {
    allocs := testing.AllocsPerRun(128, func() {
        _, _ = DecodeDelaySubframe1(57)
    })
    if allocs != 0 {
        t.Fatalf("DecodeDelaySubframe1 allocated %.2f times per call; want 0", allocs)
    }
}

func TestNoAllocationInDecodeDelaySubframe2(t *testing.T) {
    allocs := testing.AllocsPerRun(128, func() {
        _, _ = DecodeDelaySubframe2(15, 60)
    })
    if allocs != 0 {
        t.Fatalf("DecodeDelaySubframe2 allocated %.2f times per call; want 0", allocs)
    }
}

func TestNoAllocationInAdaptiveCodebook(t *testing.T) {
    var pastExc [200]int16
    for i := range pastExc {
        pastExc[i] = int16(i)
    }
    var v [40]int16
    slice := pastExc[:]
    allocs := testing.AllocsPerRun(128, func() {
        AdaptiveCodebook(50, 1, slice, &v)
    })
    if allocs != 0 {
        t.Fatalf("AdaptiveCodebook allocated %.2f times per call; want 0", allocs)
    }
}
```

- [ ] **Step 2: Run the allocation tests**

Run: `go test ./internal/pitch/ -run TestNoAllocation -v`
Expected: PASS. If any fails, use `go build -gcflags='-m' ./internal/pitch` to locate the escape site. Common culprits: passing the `pastExc[:]` slice inside the test closure causes the array to escape — the slice variable declared outside the closure prevents that (see the `slice := pastExc[:]` pattern above).

- [ ] **Step 3: Write the benchmark**

Create `internal/pitch/bench_test.go`:

```go
package pitch

import "testing"

func BenchmarkAdaptiveCodebookIntegerDelay(b *testing.B) {
    var pastExc [200]int16
    for i := range pastExc {
        pastExc[i] = int16(i - 100)
    }
    var v [40]int16
    slice := pastExc[:]
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        AdaptiveCodebook(50, 0, slice, &v)
    }
}

func BenchmarkAdaptiveCodebookFractional(b *testing.B) {
    var pastExc [200]int16
    for i := range pastExc {
        pastExc[i] = int16(i - 100)
    }
    var v [40]int16
    slice := pastExc[:]
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        AdaptiveCodebook(50, 1, slice, &v)
    }
}

func BenchmarkAdaptiveCodebookShortPitch(b *testing.B) {
    var pastExc [200]int16
    for i := range pastExc {
        pastExc[i] = int16(i - 100)
    }
    var v [40]int16
    slice := pastExc[:]
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        AdaptiveCodebook(20, 1, slice, &v)
    }
}
```

- [ ] **Step 4: Run the benchmarks and confirm zero allocs**

Run: `go test -bench=. -benchmem -run=^$ ./internal/pitch/`
Expected: `0 B/op   0 allocs/op` on every benchmark.

- [ ] **Step 5: Polish the package documentation**

Rewrite `internal/pitch/doc.go` to match the real pipeline:

```go
// Package pitch implements the G.729 + Annex A decoder's adaptive
// codebook: pitch delay reconstruction from the 14 pitch-related
// bits per frame (P1 8 bits, P0 1 bit, P2 5 bits) plus construction
// of the 40-sample adaptive codebook vector per subframe via
// 1/3-sample fractional interpolation on the past excitation signal.
//
// # Public API
//
//   CheckParity(p1, p0 uint8) bool
//       Per ITU-T G.729 §3.7.2. Returns true when p0 matches the
//       parity computed over P1's upper 6 bits.
//
//   DecodeDelaySubframe1(p1 uint8) (tInt, tFrac int)
//       Per §3.7.1. tInt ∈ [19, 143], tFrac ∈ {-1, 0, 1} at 1/3
//       sample resolution. The 1/3-range covers [19, 84 2/3] with
//       full fractional granularity; the integer-only range covers
//       [85, 143].
//
//   DecodeDelaySubframe2(p2 uint8, t1Rounded int) (tInt, tFrac int)
//       Per §3.7.1. Relative encoding around t1Rounded (round-to-
//       nearest of subframe-1 delay) with a ±5-sample window at
//       1/3 resolution.
//
//   AdaptiveCodebook(tInt, tFrac int, pastExc []int16, v *[40]int16)
//       Per §3.8 / §4.1.4. Writes v[0..39] from pastExc at the
//       specified delay, using the 1/3-sample FIR interpolator from
//       tables.PitchInterpFIR for fractional offsets. Extends by
//       periodicity when tInt < 40.
//
// # Past excitation convention
//
// pastExc represents past excitation in chronological order.
// pastExc[len-1] is the most recent sample (one before the current
// subframe's first output). The caller must supply at least
// tInt + Linter samples of history.
//
// # State ownership
//
// This package holds no state. The past-excitation ring buffer is
// owned by the top-level decoder (Phase 1g), which updates it each
// subframe from the sum of adaptive and fixed codebook contributions
// scaled by the decoded gains.
//
// # Numerical contract
//
//   Indices:     raw integers from the bitstream.
//   Delays:      tInt ∈ [19, 143], tFrac ∈ {-1, 0, 1}.
//   FIR taps:    Q15 int16, from tables.PitchInterpFIR.
//   pastExc, v:  Q0 int16 (excitation domain).
//
// # Scratch-from-spec
//
// Algorithm from ITU-T G.729 §3.7 / §4.1.3 / §4.1.4 + Annex A. FIR
// coefficient table transcribed from the ITU reference distribution
// tab_ld8a.c data-array initializer under the merger-doctrine
// exception. Every arithmetic step routes through internal/fixed.
//
// # Concurrency
//
// All functions are pure and safe for concurrent use. The caller
// owns all state.
package pitch
```

- [ ] **Step 6: Run the full test + vet pass**

Run in parallel:
- `go test -race ./internal/pitch/... ./internal/tables/...` → all PASS
- `go vet ./internal/pitch/... ./internal/tables/...` → silent

- [ ] **Step 7: Commit**

```bash
git add internal/pitch/alloc_test.go internal/pitch/bench_test.go internal/pitch/doc.go
git commit -m "test(pitch): lock zero-alloc + per-call benches; polish doc"
```

---

## Completion criteria

At the end of Task 9:

- All unit tests pass (`go test -race ./internal/pitch/... ./internal/tables/...`).
- `BenchmarkAdaptiveCodebook*` reports `0 B/op, 0 allocs/op`.
- `go vet` is silent.
- 9 commits on `main`, one per task.

Write a completion report at `docs/superpowers/plans/2026-04-21-phase1b-pitch-completion-report.md` summarising:

- Which spec sections were referenced for which code.
- Any spec deviations you applied — especially Task 3/4 (pitch delay encoding boundaries — my plan's split point, low-edge behavior, and subframe-2 clamping are all likely spots where §3.7.1 disagrees with the plan), Task 5 (FIR table layout — the spec's index arithmetic is almost certainly different from the plan's placeholder sketch), and Task 7 (`firCoeff` lookup formula once you see tab_ld8a.c's initializer structure).
- Any test inputs that had to be adjusted to match the spec's actual encoding.
- Benchmark numbers for each of the three adaptive-codebook benches.

---

## What comes next (not in this plan)

- **Phase 1c** — ACELP fixed codebook decode (17 bits → 4 pulses + signs + positions + pre-emphasis filter).
- **Phase 1d** — gain VQ (7 bits → pitch gain `g_p` + fixed codebook gain `g_c`, with MA prediction of `g_c`).
- **Phase 1e** — excitation sum (`g_p · v + g_c · c`), LP synthesis filter `1/A(z)` per subframe.
- **Phase 1f** — adaptive post-filter (Annex A simplified).
- **Phase 1g** — top-level Decoder wiring bitstream → pitch → ACELP → gain → synth → post, owning the shared excitation ring buffer. First ITU `SPEECH.*` / `PITCH.*` test-vector run here.
- **Phase 1h** — erasure concealment (use parity failures from this phase + bad-frame sync from bitstream).

Until Phase 1g wires all sub-blocks together, Phase 1b's correctness rests on the property tests here (parity exhaustive, delay boundary, partition-of-unity, short-pitch replication) and on allocation + benchmark invariants.
