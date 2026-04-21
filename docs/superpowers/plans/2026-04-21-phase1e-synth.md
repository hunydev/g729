# Phase 1e Implementation Plan — `internal/synth`: Excitation + LP Synthesis Filter

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the G.729A decoder's sample-producing stage: assemble the 40-sample excitation `u(n) = g_p·v(n) + g_c·c(n)` from Phase 1b/1c/1d outputs, then apply the 10th-order all-pole LP synthesis filter `s(n) = u(n) − Σ a_i·s(n−i)` to produce synthesized speech samples for each subframe.

**Architecture:** A new `internal/synth` package with two concerns cleanly separated:

1. **`BuildExcitation`** — a pure function that mixes the adaptive and fixed codebook contributions into `u[40]`. This is the Q-format glue layer for the three upstream phases (pitch Q0 + gain Q14/Q12 + fcb Q13). The output `u[40]` is Q0 Word16 and will also be fed back into the encoder-side past-excitation ring buffer by the top-level Phase 1g decoder.
2. **`Synthesizer`** — a stateful struct that applies `1/A(z)` per subframe. State: the 10 most recent output samples (needed as the IIR filter memory for the next subframe). All arithmetic routes through `internal/fixed` with saturation.

**Tech Stack:** Go, `internal/fixed` ITU basic-ops, `internal/pitch` output `v[40]`, `internal/fcb` output `c[40]`, `internal/gain.Decoder.Decode` output `(gpQ14, gcQ12)`, `internal/lsp` output `a[11]` (Q12 LP coefficients per subframe).

**Scratch-from-spec:** Every arithmetic choice is derived from ITU-T G.729 §3.10 / §4.1.2 / §4.1.6 directly. No ITU reference C, bcg729, Sipro Lab, or any other existing G.729 implementation is consulted for algorithmic code. The spec text ("synthesis filter is implemented with saturation arithmetic") is taken at face value; we rely on `fixed.LMac`/`LMsu`/`LShl`/`Round` to preserve ITU saturation semantics.

**Q-format map (recap for this phase):**

| Signal         | Source               | Q-format         | Notes                            |
| -------------- | -------------------- | ---------------- | -------------------------------- |
| `v[n]`         | `pitch.AdaptiveCodebook` | Q0 Word16    | Past excitation in Word16 linear |
| `c[n]`         | `fcb.Decode`         | Q13 Word16       | Pulses ±8192 + pitch-enhanced    |
| `gpQ14`        | `gain.Decoder.Decode`| Q14 Word16       | Range [0, ~1.2]                  |
| `gcQ12`        | `gain.Decoder.Decode`| Q12 Word16       | Range (0, ~8) — Phase 1d choice  |
| `a[0..10]`     | `lsp.Decoder.Decode` | Q12 Word16       | `a[0] = 4096`; `a[i]` stable     |
| **`u[n]`**     | **this phase**       | **Q0 Word16**    | Saturated excitation             |
| **`s[n]`**     | **this phase**       | **Q0 Word16**    | Pre-postfilter synthesis         |
| `pastSynth[10]`| Synthesizer state    | Q0 Word16        | Most recent at `[9]`             |

**Q-format alignment note (important):** The two excitation contributions have different natural Q-formats:

- `gpQ14 · v[n]` at Q14·Q0 → Q15 in Word32 via `LMult` (ITU's `L_mult = 2·a·b`).
- `gcQ12 · c[n]` at Q12·Q13 → Q26 in Word32 via `LMult`.

To sum them, we down-shift the code-gain product by 11 bits (Q26 → Q15), add, then shift left by 1 (Q15 → Q16) and round to Q0. The 11-bit down-shift discards fractional-bit precision from `gcQ12`, which is acceptable because:
(a) the top bits of `gcQ12` dominate the sum when the signal is audible;
(b) the same alignment would occur regardless of how `g_c` is stored — keeping 11 bits below the unit position is intrinsic to the `g_c·c` Q-format.

If Phase 1g's ITU-vector validation shows precision loss from this shift, the remedy is to revise `internal/gain` to return `gcQ1` (spec-standard) instead — the structure here is unchanged. This is flagged in §"Phase 1g validation backlog" of the completion report.

**Concurrency:** `Synthesizer` is not safe for concurrent use. One instance per decoder channel.

---

## File structure

```
internal/synth/
├── doc.go              # package doc (Task 11)
├── types.go            # Synthesizer struct (Task 1)
├── excitation.go       # BuildExcitation (Tasks 2-4)
├── excitation_test.go  # BuildExcitation tests
├── filter.go           # Synthesizer.filterSubframe (Tasks 5-7)
├── filter_test.go      # filter tests
├── synthesizer.go      # Synthesize, Reset (Tasks 8-9)
├── synthesizer_test.go # end-to-end tests
├── alloc_test.go       # zero-allocation locks (Task 10)
└── bench_test.go       # benchmarks (Task 11)
```

No new files in `internal/tables` (the LP coefficients come in via a parameter, not a table).

---

## Public API surface (target)

```go
package synth

// BuildExcitation composes the per-subframe excitation
//   u(n) = g_p · v(n) + g_c · c(n), n ∈ [0, 39]
// per ITU-T G.729 §4.1.6.
//
// Q-formats (inputs):
//   gpQ14: Q14 Word16; typical range [0, ~19661] (~1.2 in Q14)
//   gcQ12: Q12 Word16; typical range [0, ~32767]
//   v:     Q0 Word16; adaptive codebook vector from internal/pitch
//   c:     Q13 Word16; fixed codebook vector from internal/fcb
//
// Output:
//   u:     Q0 Word16; saturated excitation
//
// Zero-allocation; all state lives in caller buffers.
func BuildExcitation(gpQ14, gcQ12 int16, v, c *[40]int16, u *[40]int16)

// Synthesizer applies 1/A(z) per subframe with 10-sample state carryover.
//
// The zero value is a valid, reset Synthesizer.
type Synthesizer struct {
    pastSynth [10]int16 // Q0; [0] = s(n-10) ... [9] = s(n-1)
}

// Synthesize computes
//   s(n) = u(n) − Σ_{i=1..10} a[i]·s(n−i), n ∈ [0, 39]
// per ITU-T G.729 §3.10 / §4.1.2.
//
// On entry, pastSynth holds s(-10..-1). On return, pastSynth holds
// s(30..39) (the last 10 samples of the subframe just produced).
//
// Inputs:
//   a: [11]int16, Q12. a[0] = 4096 (present for layout; not used numerically
//      because the synthesis filter is normalized).
//   u: [40]int16, Q0. Excitation from BuildExcitation.
//
// Output:
//   s: [40]int16, Q0. Synthesized samples with saturation arithmetic.
//
// Zero-allocation; all temporaries are stack-resident.
func (synth *Synthesizer) Synthesize(a *[11]int16, u *[40]int16, s *[40]int16)

// Reset clears the synthesis-filter state. After Reset, the next call to
// Synthesize begins with all-zero past synthesis (as required by
// ITU-T G.729 §4.3 for the first frame of a stream).
func (synth *Synthesizer) Reset()
```

---

## Task 1: Package skeleton + Synthesizer type

**Files:**
- Create: `internal/synth/doc.go` (placeholder; full doc in Task 11)
- Create: `internal/synth/types.go`
- Create: `internal/synth/synthesizer_test.go`

- [x] **Step 1: Write the failing test for Synthesizer zero-value**

Write to `internal/synth/synthesizer_test.go`:

```go
package synth

import (
	"testing"
)

func TestSynthesizer_ZeroValueIsReset(t *testing.T) {
	var synth Synthesizer
	for i, v := range synth.pastSynth {
		if v != 0 {
			t.Errorf("pastSynth[%d] = %d, want 0", i, v)
		}
	}
}

func TestSynthesizer_ResetZerosState(t *testing.T) {
	var synth Synthesizer
	for i := range synth.pastSynth {
		synth.pastSynth[i] = int16(100 + i)
	}
	synth.Reset()
	for i, v := range synth.pastSynth {
		if v != 0 {
			t.Errorf("after Reset, pastSynth[%d] = %d, want 0", i, v)
		}
	}
}
```

- [x] **Step 2: Verify the test fails (package doesn't exist yet)**

Run: `go test ./internal/synth/...`

Expected: compile error — `package synth` or `Synthesizer`/`Reset` undefined.

- [x] **Step 3: Create `internal/synth/doc.go` placeholder**

```go
// Package synth implements the G.729 + Annex A decoder's excitation
// assembly and LP synthesis filter. See ITU-T G.729 §3.10, §4.1.2, §4.1.6.
//
// Full package documentation is in doc.go (finalized at the end of Phase 1e).
package synth
```

- [x] **Step 4: Create `internal/synth/types.go`**

```go
package synth

// Synthesizer holds the per-channel LP synthesis filter state.
//
// The zero value is ready for use: pastSynth is all zero, matching the
// G.729 §4.3 first-frame initial condition.
//
// Not safe for concurrent use. Caller owns one Synthesizer per decoder stream.
type Synthesizer struct {
	// pastSynth stores the 10 most recent output samples of Synthesize in Q0.
	// Indexing: pastSynth[0] = s(n-10) (oldest), pastSynth[9] = s(n-1) (most recent).
	pastSynth [10]int16
}

// Reset clears the synthesis-filter state to the all-zero initial condition
// specified by ITU-T G.729 §4.3.
func (synth *Synthesizer) Reset() {
	*synth = Synthesizer{}
}
```

- [x] **Step 5: Verify tests pass**

Run: `go test -race ./internal/synth/...`

Expected: `PASS`.

- [x] **Step 6: Commit**

```bash
git add internal/synth/doc.go internal/synth/types.go internal/synth/synthesizer_test.go
git commit -m "$(cat <<'EOF'
feat(synth): package skeleton + Synthesizer type with Reset

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: BuildExcitation — pitch-only contribution

**Files:**
- Create: `internal/synth/excitation.go`
- Create: `internal/synth/excitation_test.go`

**Spec reference:** ITU-T G.729 §4.1.6 eq. (75): `u(n) = g_p · v(n) + g_c · c(n)`.

In this task we implement only the pitch half, with tests that zero out `c[]` and `gcQ12` so the arithmetic reduces to rounding `g_p · v` to Q0.

**Q-format derivation (pitch half):**
- `gpQ14 · v[n]` is Q14·Q0 = Q14 in Word32.
- `fixed.LMult(gpQ14, v[n])` yields `2 · gpQ14 · v[n]`, i.e. Q15 in Word32.
- `fixed.LShl(lmult_result, 1)` shifts left by 1 → Q16 in Word32.
- `fixed.Round(Q16)` extracts the high 16 bits → Q0 Word16.

Sanity: `gpQ14 = 16384` (1.0 in Q14) and `v[n] = 1000` → `LMult = 32,768,000` (Q15); `LShl(·, 1) = 65,536,000` (Q16); `Round` → `65,536,000 >> 16 = 1000`. Pass-through when g_p = 1.0. ✓

- [x] **Step 1: Write the failing tests**

Write to `internal/synth/excitation_test.go`:

```go
package synth

import (
	"testing"
)

// With gpQ14 = 16384 (= 1.0 in Q14) and gcQ12 = 0, the excitation equals v
// (since gp·v·2 lives in Q15, shifted to Q16, rounded to Q0 returns v).
func TestBuildExcitation_PitchIdentity(t *testing.T) {
	var v, c, u [40]int16
	for i := range v {
		v[i] = int16(100 + i*3) // v = 100, 103, 106, ...
	}
	// c is all zeros; gc is zero too — the code contribution is zero.

	BuildExcitation(16384, 0, &v, &c, &u)

	for i := range u {
		if u[i] != v[i] {
			t.Errorf("u[%d] = %d, want %d (g_p = 1.0)", i, u[i], v[i])
		}
	}
}

// With gpQ14 = 8192 (= 0.5 in Q14) and gcQ12 = 0, expect u ≈ v/2.
// Exact: LMult(8192, v) = 2·8192·v = 16384·v; LShl by 1 = 32768·v;
// Round extracts high16 = v/2 (rounded).
func TestBuildExcitation_PitchHalfGain(t *testing.T) {
	var v, c, u [40]int16
	for i := range v {
		v[i] = 200
	}

	BuildExcitation(8192, 0, &v, &c, &u)

	for i := range u {
		if u[i] != 100 {
			t.Errorf("u[%d] = %d, want 100 (g_p = 0.5, v = 200)", i, u[i])
		}
	}
}

// With gpQ14 = 0 and gcQ12 = 0, u must be all zeros regardless of v.
func TestBuildExcitation_ZeroGains(t *testing.T) {
	var v, c, u [40]int16
	for i := range v {
		v[i] = int16(i * 50)
	}

	BuildExcitation(0, 0, &v, &c, &u)

	for i := range u {
		if u[i] != 0 {
			t.Errorf("u[%d] = %d, want 0 (zero gains)", i, u[i])
		}
	}
}
```

- [x] **Step 2: Verify tests fail (BuildExcitation undefined)**

Run: `go test ./internal/synth/...`

Expected: compile error — `BuildExcitation` undefined.

- [x] **Step 3: Write the initial BuildExcitation (pitch half only)**

Write to `internal/synth/excitation.go`:

```go
package synth

import (
	"github.com/exedev/g729/internal/fixed"
)

// BuildExcitation composes the per-subframe excitation
//   u(n) = g_p · v(n) + g_c · c(n)
// per ITU-T G.729 §4.1.6 eq. (75), using ITU saturation arithmetic throughout.
//
// Q-formats:
//   gpQ14  — Q14 Word16 (adaptive codebook gain)
//   gcQ12  — Q12 Word16 (fixed codebook gain, from internal/gain)
//   v      — Q0  Word16 × 40 (adaptive codebook vector)
//   c      — Q13 Word16 × 40 (fixed codebook vector)
//   u      — Q0  Word16 × 40 (output excitation)
//
// Arithmetic per sample:
//   Lpitch = LMult(gpQ14, v[n])               // Q15 in Word32
//   // (code-gain half added in Task 3)
//   Lsum = LShl(Lpitch, 1)                    // Q16 in Word32
//   u[n] = Round(Lsum)                        // Q0  Word16, saturated
func BuildExcitation(gpQ14, gcQ12 int16, v, c *[40]int16, u *[40]int16) {
	for n := 0; n < 40; n++ {
		lPitch := fixed.LMult(gpQ14, v[n])
		lSum := fixed.LShl(lPitch, 1)
		u[n] = fixed.Round(lSum)
	}
	_ = gcQ12 // used in Task 3
	_ = c     // used in Task 3
}
```

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/synth/...`

Expected: `PASS` for all three tests.

- [x] **Step 5: Commit**

```bash
git add internal/synth/excitation.go internal/synth/excitation_test.go
git commit -m "$(cat <<'EOF'
feat(synth): BuildExcitation pitch contribution per ITU §4.1.6

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: BuildExcitation — add fixed codebook contribution

**Files:**
- Modify: `internal/synth/excitation.go`
- Modify: `internal/synth/excitation_test.go` (add tests)

**Q-format derivation (code half):**
- `gcQ12 · c[n]` is Q12·Q13 = Q25 in Word32.
- `fixed.LMult(gcQ12, c[n])` doubles → Q26 in Word32.
- We want the code half at Q15 to match the pitch half. Shift right by 11:
  `fixed.LShr(LMult(gcQ12, c[n]), 11)` → Q15 in Word32.
- Add to pitch half (also Q15), then `LShl(·, 1)` + `Round` as before.

**Sanity check:** `gcQ12 = 4096` (1.0 in Q12), `c[n] = 8192` (1.0 in Q13), `gpQ14 = 0`:
- `LMult(4096, 8192) = 2 · 4096 · 8192 = 67,108,864` (Q26).
- `LShr(·, 11) = 32,768` (Q15).
- Add pitch (0): still 32,768 (Q15).
- `LShl(·, 1) = 65,536` (Q16).
- `Round` → `65,536 >> 16 = 1`. But the expected analytical value is `1.0 · 1.0 = 1.0` in Q0, which is … 1.

Wait, `c[n] = 8192` in Q13 represents `8192/8192 = 1.0`. So `g_c · c = 1.0 · 1.0 = 1.0`. In Q0 Word16 that is `1`. ✓

**Second sanity:** `gcQ12 = 4096` (1.0), `c[n] = 16384` (2.0 in Q13): expected `u[n] = 2`.
- `LMult = 2 · 4096 · 16384 = 134,217,728` (Q26).
- `LShr(·, 11) = 65,536` (Q15).
- `LShl(·, 1) = 131,072` (Q16).
- `Round` → `131,072 >> 16 = 2`. ✓

- [x] **Step 1: Write the failing tests**

Append to `internal/synth/excitation_test.go`:

```go
// With gpQ14 = 0 and (gcQ12 = 4096 [1.0], c[n] = 8192 [1.0 in Q13]),
// the code contribution rounds to 1 in Q0.
func TestBuildExcitation_CodeIdentity(t *testing.T) {
	var v, c, u [40]int16
	for i := range c {
		c[i] = 8192 // 1.0 in Q13
	}

	BuildExcitation(0, 4096, &v, &c, &u)

	for i := range u {
		if u[i] != 1 {
			t.Errorf("u[%d] = %d, want 1 (g_c = 1.0 Q12, c[i] = 1.0 Q13)", i, u[i])
		}
	}
}

// With gpQ14 = 0 and (gcQ12 = 4096, c[n] = 16384 [2.0 Q13]), expect u = 2.
func TestBuildExcitation_CodeScales(t *testing.T) {
	var v, c, u [40]int16
	for i := range c {
		c[i] = 16384 // 2.0 in Q13
	}

	BuildExcitation(0, 4096, &v, &c, &u)

	for i := range u {
		if u[i] != 2 {
			t.Errorf("u[%d] = %d, want 2 (g_c = 1.0, c[i] = 2.0)", i, u[i])
		}
	}
}

// Combined: gp = 0.5 (8192 Q14), v[n] = 200, gc = 0.5 (2048 Q12), c[n] = 8192 (1.0 Q13).
// Expected: u = 0.5·200 + 0.5·1 = 100 + 0.5 → rounds to 100 or 101 depending on
// accumulator sign; with LMac/LMult saturation, expect 100 (the 0.5 rounds down).
// Tolerance: ±1 LSB from the ±0.5 rounding residual.
func TestBuildExcitation_PitchAndCodeCombined(t *testing.T) {
	var v, c, u [40]int16
	for i := range v {
		v[i] = 200
		c[i] = 8192 // 1.0 in Q13
	}

	BuildExcitation(8192, 2048, &v, &c, &u)

	for i := range u {
		// Analytical: 0.5·200 + 0.5·1.0 = 100.5 → round to 100 or 101
		if u[i] < 100 || u[i] > 101 {
			t.Errorf("u[%d] = %d, want 100 or 101 (combined contribution)", i, u[i])
		}
	}
}
```

- [x] **Step 2: Verify tests fail**

Run: `go test ./internal/synth/ -run 'TestBuildExcitation_Code|TestBuildExcitation_PitchAndCode' -v`

Expected: `FAIL` — `u[i] = 0` because we haven't wired the code half yet.

- [x] **Step 3: Implement the code contribution**

Replace the body of `BuildExcitation` in `internal/synth/excitation.go`:

```go
func BuildExcitation(gpQ14, gcQ12 int16, v, c *[40]int16, u *[40]int16) {
	for n := 0; n < 40; n++ {
		// Pitch contribution: g_p · v[n] in Q15 (Word32).
		lPitch := fixed.LMult(gpQ14, v[n])

		// Code contribution: g_c · c[n] starts at Q26 (Word32), then
		// down-shift by 11 to reach Q15 to match the pitch half.
		lCode := fixed.LShr(fixed.LMult(gcQ12, c[n]), 11)

		// Sum at Q15, shift left by 1 to Q16, then round to Q0 Word16.
		lSum := fixed.LAdd(lPitch, lCode)
		lSum = fixed.LShl(lSum, 1)
		u[n] = fixed.Round(lSum)
	}
}
```

- [x] **Step 4: Verify all tests pass**

Run: `go test -race ./internal/synth/...`

Expected: `PASS` for all (including the Task 2 tests that should still pass).

- [x] **Step 5: Commit**

```bash
git add internal/synth/excitation.go internal/synth/excitation_test.go
git commit -m "$(cat <<'EOF'
feat(synth): BuildExcitation code contribution and sum per ITU §4.1.6

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: BuildExcitation — saturation on overflow

**Files:**
- Modify: `internal/synth/excitation_test.go` (add saturation tests)

**Spec reference:** ITU-T G.729 §4.1.6 + general §1.x on saturation arithmetic.

ITU's `L_mult`/`L_add`/`round` saturate to `MAX_32` / `MIN_32` / `MAX_16` / `MIN_16` on overflow. Verify this is what `fixed.LMult` + `fixed.LAdd` + `fixed.Round` do (they should, per Phase 0 / `internal/fixed` contract), by constructing extreme inputs.

- [x] **Step 1: Write the failing tests**

Append to `internal/synth/excitation_test.go`:

```go
import (
	"math"
)

// Extreme-gain saturation: gpQ14 at max (32767 ≈ 2.0 Q14) and v[n] at max
// should saturate u[n] to MAX_16. 2·32767·32767 at LMult = approx 2^31 (saturates).
// After LShl(·, 1) and Round, expect MAX_16 = 32767.
func TestBuildExcitation_SaturatesOnHighPitchGain(t *testing.T) {
	var v, c, u [40]int16
	for i := range v {
		v[i] = math.MaxInt16 // 32767
	}

	BuildExcitation(math.MaxInt16, 0, &v, &c, &u)

	for i := range u {
		if u[i] != math.MaxInt16 {
			t.Errorf("u[%d] = %d, want MAX_16 (%d)", i, u[i], int16(math.MaxInt16))
		}
	}
}

// Negative extreme: gp = MAX_16, v = MIN_16 → extreme negative product.
// After LMult(32767, -32768) = -2,147,450,880 → LShl(·,1) attempts 2x,
// saturates to MIN_32, Round returns MIN_16 = -32768.
func TestBuildExcitation_SaturatesOnNegativeExtreme(t *testing.T) {
	var v, c, u [40]int16
	for i := range v {
		v[i] = math.MinInt16 // -32768
	}

	BuildExcitation(math.MaxInt16, 0, &v, &c, &u)

	for i := range u {
		if u[i] != math.MinInt16 {
			t.Errorf("u[%d] = %d, want MIN_16 (%d)", i, u[i], int16(math.MinInt16))
		}
	}
}

// Combined-overflow: both contributions at their max; saturation must still
// yield MAX_16 not an undefined wrap-around.
func TestBuildExcitation_SaturatesOnBothContributionsHigh(t *testing.T) {
	var v, c, u [40]int16
	for i := range v {
		v[i] = math.MaxInt16
		c[i] = math.MaxInt16
	}

	BuildExcitation(math.MaxInt16, math.MaxInt16, &v, &c, &u)

	for i := range u {
		if u[i] != math.MaxInt16 {
			t.Errorf("u[%d] = %d, want MAX_16", i, u[i])
		}
	}
}
```

- [x] **Step 2: Run the tests — they should already pass**

Run: `go test -race ./internal/synth/...`

Expected: `PASS`. The existing BuildExcitation already uses saturating primitives.

If a test fails, investigate: the culprit is likely `LShl`/`LShr` not saturating, or the down-shift in the code half interacting oddly with MIN_32. Do **not** short-circuit with manual clamps; instead read `internal/fixed`'s contract and route the offending arithmetic through the correct primitive (e.g. `LShlS` if an explicit saturating left-shift is needed).

- [x] **Step 3: Commit**

```bash
git add internal/synth/excitation_test.go
git commit -m "$(cat <<'EOF'
test(synth): lock saturation semantics for BuildExcitation extremes

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: filterSubframe — direct pass-through (zero LPC tail)

**Files:**
- Create: `internal/synth/filter.go`
- Create: `internal/synth/filter_test.go`

**Spec reference:** ITU-T G.729 §3.10 / §4.1.2: the synthesis filter is
`s(n) = u(n) − Σ_{i=1..10} a_i · s(n−i)`.

With `a[1..10] = 0`, the filter degenerates to `s(n) = u(n)` — a direct-pass test that catches wiring errors.

**Q-format derivation for the filter:**
- `a[i]` is Q12 Word16; `s(n−i)` is Q0 Word16.
- `a[i] · s(n−i)` is Q12 in Word32. `fixed.LMult` doubles → Q13.
- `u(n)` is Q0 Word16. We must sign-extend / scale to Q13 Word32:
  `fixed.LMult(u[n], 4096)` where 4096 is Q12 representing 1.0 — this gives `2 · u[n] · 4096` at Q13 Word32. ✓
- Accumulate: start with `LMult(u[n], 4096)` at Q13, then subtract each `LMult(a[i], s[n−i])` via `LMsu`.
- After the loop, `L_temp` is in Q13. Shift left by 3 → Q16. Round → Q0 Word16.

**Sanity check (zero LPC):** `u[n] = 1000`, `a[1..10] = 0`:
- `L_temp = LMult(1000, 4096) = 2·1000·4096 = 8,192,000` (Q13).
- No subtractions.
- `LShl(·, 3) = 65,536,000` (Q16).
- `Round → 65,536,000 >> 16 = 1000` (Q0). ✓

- [x] **Step 1: Write the failing test**

Write to `internal/synth/filter_test.go`:

```go
package synth

import (
	"testing"
)

// With a = [4096, 0, 0, ..., 0] (i.e. A(z) = 1, synthesis is identity),
// the filter should reproduce u in s, regardless of pastSynth.
func TestFilter_ZeroLPCIsIdentity(t *testing.T) {
	var synth Synthesizer
	var u, s [40]int16
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	for i := range u {
		u[i] = int16(1000 + i*37)
	}

	// Populate pastSynth with non-zero to ensure the filter ignores it
	// when a[1..10] = 0 (the past-state taps are all multiplied by zero).
	for i := range synth.pastSynth {
		synth.pastSynth[i] = int16(9000 - i*100)
	}

	synth.filterSubframe(&a, &u, &s)

	for i := range s {
		if s[i] != u[i] {
			t.Errorf("s[%d] = %d, want %d (zero LPC is identity)", i, s[i], u[i])
		}
	}
}
```

- [x] **Step 2: Verify the test fails**

Run: `go test ./internal/synth/ -run TestFilter_ZeroLPCIsIdentity -v`

Expected: compile error — `filterSubframe` undefined.

- [x] **Step 3: Implement the filter (direct form, no past-state integration yet)**

Write to `internal/synth/filter.go`:

```go
package synth

import (
	"github.com/exedev/g729/internal/fixed"
)

// filterSubframe applies 1/A(z) to u in-place, producing s.
//
// Spec: ITU-T G.729 §3.10 / §4.1.2.
//   s(n) = u(n) − Σ_{i=1..10} a[i] · s(n−i), n ∈ [0, 39]
//
// Q-format pipeline per sample:
//   L_temp = LMult(u[n], 4096)                 // Q13 in Word32 (2·u·4096)
//   for i = 1..10:
//     L_temp = LMsu(L_temp, a[i], s[n-i])      // subtract 2·a[i]·s(n-i) Q13
//   L_temp = LShl(L_temp, 3)                   // Q13 → Q16
//   s[n]   = Round(L_temp)                     // Q0 Word16
//
// Past-state feedback (s(n−i) for n−i < 0) is read from synth.pastSynth;
// on return, synth.pastSynth is updated to hold s[30..39].
//
// Implementation uses a stack-resident 50-sample working buffer to unify
// past and current state indexing; no heap allocations.
func (synth *Synthesizer) filterSubframe(a *[11]int16, u, s *[40]int16) {
	var work [50]int16
	copy(work[:10], synth.pastSynth[:])

	for n := 0; n < 40; n++ {
		lTemp := fixed.LMult(u[n], 4096)
		for i := 1; i <= 10; i++ {
			lTemp = fixed.LMsu(lTemp, a[i], work[10+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[10+n] = fixed.Round(lTemp)
	}

	copy(s[:], work[10:])
	copy(synth.pastSynth[:], work[40:])
}
```

- [x] **Step 4: Verify the test passes**

Run: `go test -race ./internal/synth/ -run TestFilter_ZeroLPCIsIdentity -v`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/synth/filter.go internal/synth/filter_test.go
git commit -m "$(cat <<'EOF'
feat(synth): LP synthesis filter direct-form skeleton per ITU §3.10

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: filterSubframe — 1st-order impulse response

**Files:**
- Modify: `internal/synth/filter_test.go` (add tests)

**Spec reference:** The synthesis filter interprets `a[i]` as LPC coefficients
such that `A(z) = 1 + a_1 z^{-1} + … + a_10 z^{-10}` and the synthesis filter
is `1/A(z)`. Synthesis equation: `s(n) = u(n) − Σ a_i·s(n−i)`.

With a first-order filter `a = [4096, 2048, 0, …]` (a_1 = 0.5 Q12) and
impulse input `u = [4000, 0, 0, …]`:

s(0) = 4000 − 0.5 · pastSynth[9]
s(1) = 0 − 0.5 · s(0)
s(2) = 0 − 0.5 · s(1)
…

With `pastSynth = 0`:
- s(0) = 4000
- s(1) = −0.5 · 4000 = −2000
- s(2) = −0.5 · (−2000) = 1000
- s(3) = −0.5 · 1000 = −500
- s(4) = 250
- …

Check bit-exactly with fixed-point rounding at the last step.

- [x] **Step 1: Write the failing tests**

Append to `internal/synth/filter_test.go`:

```go
// 1st-order filter impulse response. a_1 = 2048 Q12 = 0.5.
// With u = impulse, pastSynth = 0, expect decaying alternating response.
func TestFilter_FirstOrderImpulseResponse(t *testing.T) {
	var synth Synthesizer
	var u, s [40]int16
	a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	u[0] = 4000

	synth.filterSubframe(&a, &u, &s)

	// Expected impulse response: s[n] = (-0.5)^n · 4000, n ≥ 0.
	expected := []int16{4000, -2000, 1000, -500, 250, -125, 62, -31, 16, -8}
	for i, want := range expected {
		if s[i] != want && s[i] != want+1 && s[i] != want-1 {
			t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], want)
		}
	}
	// After index 13 or so the values drop below the ±1 LSB rounding floor
	// and alternate around zero. Don't over-constrain; just check the
	// response has decayed.
	if s[20] > 2 || s[20] < -2 {
		t.Errorf("s[20] = %d, want |·| ≤ 2 after decay", s[20])
	}
}

// Reverse-sign first-order: a_1 = -2048 Q12 = -0.5.
// Synthesis: s(n) = u(n) − (−0.5)·s(n−1) = u(n) + 0.5·s(n−1).
// With u = [4000, 0, 0, ...], pastSynth = 0: s(n) = 4000 · 0.5^n (all same sign).
func TestFilter_FirstOrderPositiveFeedback(t *testing.T) {
	var synth Synthesizer
	var u, s [40]int16
	a := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	u[0] = 4000

	synth.filterSubframe(&a, &u, &s)

	expected := []int16{4000, 2000, 1000, 500, 250, 125}
	for i, want := range expected {
		if s[i] != want && s[i] != want+1 && s[i] != want-1 {
			t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], want)
		}
	}
}
```

- [x] **Step 2: Run the tests — they should already pass**

Run: `go test -race ./internal/synth/ -run 'TestFilter_FirstOrder' -v`

Expected: `PASS` (the implementation from Task 5 already supports arbitrary `a[1..10]`).

If a test fails with an off-by-one in the decaying response (e.g. s[1] = -1999 instead of -2000), this is the Q13→Q16→Round half-LSB rounding direction and the ±1 LSB tolerance in the test should already absorb it. If the error is larger, re-derive the Q-format carefully.

- [x] **Step 3: Commit**

```bash
git add internal/synth/filter_test.go
git commit -m "$(cat <<'EOF'
test(synth): lock 1st-order impulse response for LP synthesis filter

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: filterSubframe — past-state feedback + state update

**Files:**
- Modify: `internal/synth/filter_test.go` (add tests)

**Spec reference:** ITU-T G.729 §4.1.2: "The memory of the synthesis filter is
the 10 last samples of the synthesis signal from the previous subframe."

Tests in this task cover:
1. `pastSynth` contributes correctly to `s[0..9]` (when n−i < 0).
2. After the call, `pastSynth` has been updated to `s[30..39]`.
3. Two back-to-back calls produce a filter output continuous across the
   subframe boundary (i.e. the state carries correctly).

- [x] **Step 1: Write the failing tests**

Append to `internal/synth/filter_test.go`:

```go
// pastSynth contributes to s[0]. With a_1 = 2048 (0.5), pastSynth[9] = 1000,
// u = 0, expect s[0] = 0 − 0.5·1000 = −500 (and decay alternating).
func TestFilter_PastStateContributes(t *testing.T) {
	var synth Synthesizer
	synth.pastSynth[9] = 1000 // s(-1) = 1000
	// older history is zero

	var u, s [40]int16
	a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	synth.filterSubframe(&a, &u, &s)

	// s[0] = -0.5·1000 = -500
	// s[1] = -0.5·(-500) = 250
	// s[2] = -0.5·250 = -125
	want := []int16{-500, 250, -125, 62, -31}
	for i, w := range want {
		if s[i] != w && s[i] != w+1 && s[i] != w-1 {
			t.Errorf("s[%d] = %d, want %d (±1 LSB)", i, s[i], w)
		}
	}
}

// pastSynth is updated to the last 10 output samples after filterSubframe.
// With zero-LPC identity filter and a known u sequence, pastSynth[i] should
// equal u[30+i] on return.
func TestFilter_StateUpdate(t *testing.T) {
	var synth Synthesizer
	var u, s [40]int16
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	for i := range u {
		u[i] = int16(1000 + i)
	}

	synth.filterSubframe(&a, &u, &s)

	for i := 0; i < 10; i++ {
		want := u[30+i] // = 1030 + i
		if synth.pastSynth[i] != want {
			t.Errorf("pastSynth[%d] = %d, want %d (last 10 of s)", i, synth.pastSynth[i], want)
		}
	}
}

// Back-to-back subframes with identity filter should pass u through in two
// contiguous batches and leave pastSynth holding the final 10 samples.
func TestFilter_TwoSubframeContinuity(t *testing.T) {
	var synth Synthesizer
	var u1, u2, s1, s2 [40]int16
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	for i := range u1 {
		u1[i] = int16(100 + i)  // 100..139
		u2[i] = int16(200 + i)  // 200..239
	}

	synth.filterSubframe(&a, &u1, &s1)
	synth.filterSubframe(&a, &u2, &s2)

	// With identity filter, s1 = u1, s2 = u2 regardless of the state carryover.
	for i := range s1 {
		if s1[i] != u1[i] {
			t.Errorf("s1[%d] = %d, want %d", i, s1[i], u1[i])
		}
		if s2[i] != u2[i] {
			t.Errorf("s2[%d] = %d, want %d", i, s2[i], u2[i])
		}
	}
}

// IIR continuity: exponential decay from pastSynth must continue
// smoothly into s[0] (no discontinuity at the subframe boundary).
// Drive u = 0, a_1 = 2048 (0.5), pastSynth[9] = 4000, rest = 0.
// Call filterSubframe twice; after two 40-sample batches, the signal
// should have decayed by (0.5)^80 ≈ 0 (i.e. underflowed to integer 0).
func TestFilter_IIRDecayAcrossBoundary(t *testing.T) {
	var synth Synthesizer
	synth.pastSynth[9] = 4000

	var u, s1, s2 [40]int16
	a := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	synth.filterSubframe(&a, &u, &s1)

	// Check continuity: s1[0] must follow from pastSynth[9] = 4000:
	//   s1[0] = -0.5·4000 = -2000
	//   s1[1] = -0.5·-2000 = 1000, etc.
	if s1[0] != -2000 && s1[0] != -1999 && s1[0] != -2001 {
		t.Errorf("s1[0] = %d, want -2000 (IIR from past state) ±1", s1[0])
	}

	synth.filterSubframe(&a, &u, &s2)

	// By s2[0], the signal should be near-zero. |s2[i]| ≤ 2 for all i.
	for i := range s2 {
		if s2[i] > 2 || s2[i] < -2 {
			t.Errorf("s2[%d] = %d, expected |·| ≤ 2 after long decay", i, s2[i])
		}
	}
}
```

- [x] **Step 2: Run the tests — they should already pass**

Run: `go test -race ./internal/synth/ -run 'TestFilter_PastState|TestFilter_StateUpdate|TestFilter_TwoSubframe|TestFilter_IIRDecay' -v`

Expected: `PASS`. The state-carry logic was written into `filterSubframe` in Task 5 via the 50-element `work` buffer and the final `copy(synth.pastSynth[:], work[40:])`.

If `TestFilter_StateUpdate` fails with `pastSynth[i] = 0`, the copy destination/source are swapped. If `TestFilter_PastStateContributes` returns zero, `copy(work[:10], synth.pastSynth[:])` was omitted.

- [x] **Step 3: Commit**

```bash
git add internal/synth/filter_test.go
git commit -m "$(cat <<'EOF'
test(synth): lock past-state feedback and state update for LP synthesis

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Synthesize — public wiring (excitation + filter)

**Files:**
- Create: `internal/synth/synthesizer.go`
- Modify: `internal/synth/synthesizer_test.go` (add tests)

**Spec reference:** ITU-T G.729 §4.1.6 + §4.1.2: per-subframe decoder sequence is
1. Compose excitation `u(n) = g_p·v(n) + g_c·c(n)`
2. Synthesize `s(n) = u(n) − Σ a_i·s(n−i)`

`Synthesize` is the public entry point composing both. It allocates no heap
temporaries — the intermediate `u[40]` lives on the stack of `Synthesize`.

- [x] **Step 1: Write the failing test**

Append to `internal/synth/synthesizer_test.go`:

```go
// End-to-end identity: with unit LPC, g_p = 1.0, g_c = 0, Synthesize should
// produce s = v (rounded exactly for this well-aligned case).
func TestSynthesize_EndToEndIdentity(t *testing.T) {
	var synth Synthesizer
	var v, c, s [40]int16
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	for i := range v {
		v[i] = int16(500 + i*10)
	}

	synth.Synthesize(&a, &v, &c, 16384, 0, &s)

	for i := range s {
		if s[i] != v[i] {
			t.Errorf("s[%d] = %d, want %d (identity pipeline)", i, s[i], v[i])
		}
	}
}

// Verify Synthesize composes BuildExcitation and filterSubframe: call the
// two underlying primitives by hand, and confirm Synthesize produces the
// same result.
func TestSynthesize_MatchesPiecewiseComposition(t *testing.T) {
	var v, c [40]int16
	a := [11]int16{4096, 1500, -800, 300, -100, 0, 0, 0, 0, 0, 0}

	for i := range v {
		v[i] = int16(i * 17)
		c[i] = int16((i - 20) * 400)
	}

	// Reference: step-by-step.
	var synthRef Synthesizer
	var uRef, sRef [40]int16
	BuildExcitation(10000, 2000, &v, &c, &uRef)
	synthRef.filterSubframe(&a, &uRef, &sRef)

	// Under test: single Synthesize call.
	var synthUUT Synthesizer
	var sUUT [40]int16
	synthUUT.Synthesize(&a, &v, &c, 10000, 2000, &sUUT)

	for i := range sRef {
		if sRef[i] != sUUT[i] {
			t.Errorf("s[%d] = %d, want %d (composition mismatch)", i, sUUT[i], sRef[i])
		}
	}

	// And the state must match too.
	for i := range synthRef.pastSynth {
		if synthRef.pastSynth[i] != synthUUT.pastSynth[i] {
			t.Errorf("pastSynth[%d] = %d, want %d (state mismatch)",
				i, synthUUT.pastSynth[i], synthRef.pastSynth[i])
		}
	}
}
```

- [x] **Step 2: Verify tests fail (Synthesize undefined)**

Run: `go test ./internal/synth/ -run TestSynthesize -v`

Expected: compile error — `Synthesize` undefined on `*Synthesizer`.

- [x] **Step 3: Implement Synthesize**

Write to `internal/synth/synthesizer.go`:

```go
package synth

// Synthesize produces one subframe of synthesized speech by composing
// BuildExcitation and the LP synthesis filter.
//
// Inputs:
//   a        — LP coefficients for this subframe (Q12, a[0] = 4096)
//   v        — adaptive codebook vector (Q0, from internal/pitch)
//   c        — fixed codebook vector (Q13, from internal/fcb)
//   gpQ14    — adaptive codebook gain (Q14, from internal/gain)
//   gcQ12    — fixed codebook gain (Q12, from internal/gain)
//
// Output:
//   s        — synthesized speech samples (Q0, pre-postfilter)
//
// Spec: ITU-T G.729 §4.1.2 / §4.1.6. Updates synth.pastSynth to the last
// 10 samples of s, ready for the next subframe.
//
// Zero-allocation: the intermediate excitation lives on this function's stack.
func (synth *Synthesizer) Synthesize(a *[11]int16, v, c *[40]int16, gpQ14, gcQ12 int16, s *[40]int16) {
	var u [40]int16
	BuildExcitation(gpQ14, gcQ12, v, c, &u)
	synth.filterSubframe(a, &u, s)
}
```

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/synth/...`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/synth/synthesizer.go internal/synth/synthesizer_test.go
git commit -m "$(cat <<'EOF'
feat(synth): Synthesize public entry composes excitation + filter

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Reset determinism + two-subframe propagation

**Files:**
- Modify: `internal/synth/synthesizer_test.go` (add tests)

These tests pin down:
1. Reset clears `pastSynth` to zero.
2. After Reset, the next Synthesize produces the same output as a fresh Synthesizer on identical inputs (determinism).
3. State actually propagates across two Synthesize calls (belt-and-suspenders against accidental regressions in Task 7/8).

- [x] **Step 1: Write the failing tests**

Append to `internal/synth/synthesizer_test.go`:

```go
// After Reset, Synthesize gives the same output as a fresh zero-value
// Synthesizer for identical inputs.
func TestSynthesize_ResetRestoresZeroValueDeterminism(t *testing.T) {
	var v, c [40]int16
	a := [11]int16{4096, 1500, -800, 300, -100, 50, 0, 0, 0, 0, 0}

	for i := range v {
		v[i] = int16(i * 13)
		c[i] = int16((i - 10) * 200)
	}

	// Reference: zero-value Synthesizer.
	var synthRef Synthesizer
	var sRef [40]int16
	synthRef.Synthesize(&a, &v, &c, 12000, 1500, &sRef)

	// Under test: populate state then Reset.
	var synthUUT Synthesizer
	for i := range synthUUT.pastSynth {
		synthUUT.pastSynth[i] = int16(5000 - i*50)
	}
	synthUUT.Reset()

	var sUUT [40]int16
	synthUUT.Synthesize(&a, &v, &c, 12000, 1500, &sUUT)

	for i := range sRef {
		if sRef[i] != sUUT[i] {
			t.Errorf("s[%d] = %d, want %d (Reset non-deterministic)",
				i, sUUT[i], sRef[i])
		}
	}
}

// State must propagate across two subframes so that the IIR signal is
// continuous — the second subframe depends on the first's final 10 samples.
func TestSynthesize_StatePropagatesAcrossSubframes(t *testing.T) {
	var synth Synthesizer
	var v1, v2, c, s1, s2 [40]int16
	a := [11]int16{4096, 2000, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	// Subframe 1: drive a recognisable signal.
	v1[0] = 4000
	// Subframe 2: zero input — the IIR response must decay from subframe 1's state.

	synth.Synthesize(&a, &v1, &c, 16384, 0, &s1)
	synth.Synthesize(&a, &v2, &c, 0, 0, &s2)

	// s2[0] must be non-zero: it reflects the decay of s1[30..39] through the filter.
	// If state propagation is broken, s2 would be all zero.
	anyNonZero := false
	for i := range s2 {
		if s2[i] != 0 {
			anyNonZero = true
			break
		}
	}
	if !anyNonZero {
		t.Error("s2 is all zero — state did not propagate across subframes")
	}
}
```

- [x] **Step 2: Run the tests — they should already pass**

Run: `go test -race ./internal/synth/ -run 'TestSynthesize_Reset|TestSynthesize_StatePropagates' -v`

Expected: `PASS` (Reset is already implemented in Task 1; state propagation in Task 5/8).

If `TestSynthesize_StatePropagatesAcrossSubframes` shows `s2 = 0`, revisit the `copy(synth.pastSynth[:], work[40:])` line in `filterSubframe`.

- [x] **Step 3: Commit**

```bash
git add internal/synth/synthesizer_test.go
git commit -m "$(cat <<'EOF'
test(synth): lock Reset determinism and two-subframe state propagation

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Zero-allocation lock

**Files:**
- Create: `internal/synth/alloc_test.go`

**Spec-informed requirement:** The G.729 decoder is real-time-oriented; the
entire hot path must be zero-allocation. Previous phases (`lsp`, `pitch`,
`fcb`, `gain`) all pass the same test. Phase 1e must too.

**What to watch:** `var u [40]int16` inside `Synthesize` is a stack-resident
value; this is fine. The `var work [50]int16` inside `filterSubframe` is
likewise stack-resident. Arrays-by-pointer (`*[40]int16`, `*[11]int16`) do
not escape. If the go compiler flags an escape (rare; happens if a caller
uses `any`/interface boxing), use `go build -gcflags="-m"` to locate the
escape point.

- [x] **Step 1: Write the failing tests**

Write to `internal/synth/alloc_test.go`:

```go
package synth

import (
	"testing"
)

func TestNoAllocationInBuildExcitation(t *testing.T) {
	var v, c, u [40]int16
	for i := range v {
		v[i] = int16(i * 13)
		c[i] = int16(i * 27)
	}

	allocs := testing.AllocsPerRun(100, func() {
		BuildExcitation(12000, 1500, &v, &c, &u)
	})
	if allocs != 0 {
		t.Errorf("BuildExcitation allocs = %v, want 0", allocs)
	}
}

func TestNoAllocationInSynthesize(t *testing.T) {
	var synth Synthesizer
	var v, c, s [40]int16
	a := [11]int16{4096, 1500, -800, 300, -100, 50, 20, -10, 5, 0, 0}
	for i := range v {
		v[i] = int16(i * 11)
		c[i] = int16((i - 5) * 200)
	}

	allocs := testing.AllocsPerRun(100, func() {
		synth.Synthesize(&a, &v, &c, 12000, 1500, &s)
	})
	if allocs != 0 {
		t.Errorf("Synthesize allocs = %v, want 0", allocs)
	}
}

func TestNoAllocationInReset(t *testing.T) {
	var synth Synthesizer
	for i := range synth.pastSynth {
		synth.pastSynth[i] = int16(i)
	}

	allocs := testing.AllocsPerRun(100, func() {
		synth.Reset()
	})
	if allocs != 0 {
		t.Errorf("Reset allocs = %v, want 0", allocs)
	}
}
```

- [x] **Step 2: Run — should pass. If any allocs are reported, investigate.**

Run: `go test -race ./internal/synth/ -run 'TestNoAllocation' -v`

Expected: `PASS`.

If a test reports non-zero allocs:
- Build with `go build -gcflags="-m" ./internal/synth/ 2>&1 | grep escape`.
- Likely suspects: `u [40]int16` inside `Synthesize` escaping to heap, or `work [50]int16` inside `filterSubframe` escaping.
- Fix: rearrange so the array does not cross a function boundary that forces escape (e.g. pass-by-pointer internally).

- [x] **Step 3: Commit**

```bash
git add internal/synth/alloc_test.go
git commit -m "$(cat <<'EOF'
test(synth): lock zero-allocation on BuildExcitation, Synthesize, Reset

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Benchmarks + final package doc

**Files:**
- Create: `internal/synth/bench_test.go`
- Rewrite: `internal/synth/doc.go`

- [x] **Step 1: Write the benchmarks**

Write to `internal/synth/bench_test.go`:

```go
package synth

import "testing"

var (
	benchA = [11]int16{4096, 1500, -800, 300, -100, 50, 20, -10, 5, -2, 1}
	benchV [40]int16
	benchC [40]int16
	benchS [40]int16
	benchU [40]int16
)

func init() {
	for i := range benchV {
		benchV[i] = int16(i * 17)
		benchC[i] = int16((i - 20) * 200)
	}
}

func BenchmarkBuildExcitation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		BuildExcitation(12000, 1500, &benchV, &benchC, &benchU)
	}
}

func BenchmarkSynthesize(b *testing.B) {
	var synth Synthesizer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		synth.Synthesize(&benchA, &benchV, &benchC, 12000, 1500, &benchS)
	}
}

// filterSubframe in isolation — excludes BuildExcitation overhead.
func BenchmarkFilterSubframe(b *testing.B) {
	var synth Synthesizer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		synth.filterSubframe(&benchA, &benchU, &benchS)
	}
}
```

- [x] **Step 2: Run the benchmarks and record results**

Run: `go test -bench=. -benchmem -run=^$ ./internal/synth/`

Expected output shape (machine-dependent; record verbatim in the completion report):

```
BenchmarkBuildExcitation-N     X   Y ns/op   0 B/op   0 allocs/op
BenchmarkSynthesize-N          X   Y ns/op   0 B/op   0 allocs/op
BenchmarkFilterSubframe-N      X   Y ns/op   0 B/op   0 allocs/op
```

All three must show `0 B/op, 0 allocs/op`. A typical Synthesize budget at modern x86 is <200 ns/op per subframe (the inner loop is 40 × 10 = 400 `LMsu` calls plus 40 rounds).

- [x] **Step 3: Polish the package documentation**

Overwrite `internal/synth/doc.go`:

```go
// Package synth implements the G.729 + Annex A decoder's excitation
// assembly and LP synthesis filter. It consumes per-subframe outputs from
// internal/pitch (adaptive codebook v[40] in Q0), internal/fcb (fixed
// codebook c[40] in Q13), internal/gain (g_p in Q14, g_c in Q12), and
// internal/lsp (LP filter coefficients a[11] in Q12) and produces 40
// synthesized speech samples s[40] in Q0.
//
// # Pipeline
//
// Per ITU-T G.729 §4.1.2, §4.1.6, §3.10:
//
//  1. BuildExcitation:  u(n) = g_p · v(n) + g_c · c(n), saturated to Q0.
//  2. Synthesize:       s(n) = u(n) − Σ_{i=1..10} a[i] · s(n−i),
//                       with s(n−i) for n−i < 0 drawn from pastSynth.
//
// Step 2 carries state across subframes (the 10 most recent synthesized
// samples). A Synthesizer's zero value is a valid Reset state per §4.3.
//
// # Numerical contract
//
//	gpQ14:  Q14 Word16; range [0, ~19661] (≈ 1.2)
//	gcQ12:  Q12 Word16; range (0, ~32767)
//	v[n]:   Q0  Word16 (adaptive codebook)
//	c[n]:   Q13 Word16 (fixed codebook, pulses ±8192)
//	a[i]:   Q12 Word16; a[0] = 4096 (present for layout only)
//	u[n]:   Q0  Word16 (excitation, saturated)
//	s[n]:   Q0  Word16 (synthesis, saturated)
//
// # Q-format alignment in BuildExcitation
//
// The two contributions have different natural Q-formats after multiply:
// the pitch half lands at Q15 in Word32 via LMult(gpQ14, v), while the
// code half lands at Q26 via LMult(gcQ12, c). The code half is down-shifted
// by 11 bits before summation to align at Q15, at the cost of 11 bits of
// gcQ12 fractional precision. Perceptually this is negligible because the
// code half's MSBs dominate the sum for audible signals; bit-exact ITU
// conformance will be verified in Phase 1g.
//
// # Scratch-from-spec
//
// All arithmetic derives from ITU-T G.729 §3.10 / §4.1.2 / §4.1.6 directly.
// No ITU reference C source, bcg729, Sipro Lab, or any other existing
// G.729 implementation was consulted for algorithmic code. The synthesis
// filter uses direct-form saturation arithmetic (no two-pass overflow
// guard); the spec does not require two-pass, and perceptual tests in
// Phase 1g will verify acceptability.
//
// # Concurrency
//
// Synthesizer is not safe for concurrent use. One instance per decoder channel.
package synth
```

- [x] **Step 4: Run all tests one last time**

Run: `go test -race ./...`

Expected: all packages `ok`, no failures.

Run: `go vet ./internal/synth/...`

Expected: silent.

- [x] **Step 5: Commit**

```bash
git add internal/synth/bench_test.go internal/synth/doc.go
git commit -m "$(cat <<'EOF'
test(synth): lock per-subframe benches; polish package doc

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Completion criteria

- [x] All 11 tasks checked off.
- [x] `go test -race ./...` passes.
- [x] `go vet ./...` silent.
- [x] `BenchmarkBuildExcitation`, `BenchmarkSynthesize`, `BenchmarkFilterSubframe` all report `0 B/op, 0 allocs/op`.
- [x] `git log --oneline` shows 11 commits on `main` for Phase 1e, in task order.
- [x] Completion report saved to `docs/superpowers/plans/2026-04-21-phase1e-synth-completion-report.md` (template below).

---

## Completion report template

At the end, record:

1. Spec sections referenced (§3.10, §4.1.2, §4.1.6, §4.3).
2. Any plan deviations encountered and their rationale. Expected candidates:
   - Q-format of the code-gain contribution: if the 11-bit down-shift proved too lossy in tests, document the chosen fix (e.g. revising gain to return Q1, or using a wider accumulator).
   - `Round` direction on half-LSB: the plan's test tolerances allow ±1 LSB; if an implementation choice pins this consistently, note it.
   - If `var u [40]int16` escapes to heap despite best efforts, document the escape location and the fix.
3. Benchmark numbers as observed on the CI/runner host.
4. Open items to verify in Phase 1g:
   - Bit-exact output against ITU test vectors.
   - The 11-bit down-shift precision (compare to Q1 variant).
   - Two-pass overflow guard necessity (if `LShl(·, 3)` overflows in practice for any test vector, the two-pass guard from §3.10 alternative becomes mandatory).
   - First-frame initial condition: Phase 1e uses all-zero `pastSynth`; confirm no Annex A clause alters this.
5. Commit list (oldest → newest).

---

## Self-review checklist (for plan author, not executor)

**Spec coverage.** Each of the following is implemented by a numbered task:

- [x] §4.1.6 eq. (75) excitation composition → Tasks 2-4
- [x] §3.10 / §4.1.2 synthesis filter `1/A(z)` → Tasks 5-7
- [x] §4.3 first-frame zero-state init → Task 1 (zero value) + Task 9 (Reset determinism)
- [x] Per-subframe state carryover → Task 7 + Task 9
- [x] ITU saturation arithmetic → Task 4 (BuildExcitation); filter inherits via LMsu/LMult
- [x] Zero-allocation hot path → Task 10
- [x] Benchmarks → Task 11

**Placeholder scan.** No "TBD", no "add appropriate", no "implement later". Every code block contains the actual code. Every test block contains real tests. Every bash command is complete. ✓

**Type consistency.** `Synthesizer`, `BuildExcitation`, `Synthesize`, `Reset` match across Tasks 1, 2, 3, 5, 8, 9, 10, 11. The hidden `filterSubframe` appears in Tasks 5, 6, 7, 8 with the same signature throughout. Arrays: `v, c, u, s` always `*[40]int16`; `a` always `*[11]int16`. ✓

**Q-format consistency.** `gpQ14` Q14, `gcQ12` Q12 match the Phase 1d output and are used consistently in every test and code block. The 11-bit down-shift is derived once and applied once. ✓

**TDD discipline.** Each task has: write failing test → verify fail → implement → verify pass → commit. Tasks 4, 6, 7, 9 are test-only (the implementation from the previous task is expected to already satisfy them); they still follow the "write → run → commit" cycle. ✓
