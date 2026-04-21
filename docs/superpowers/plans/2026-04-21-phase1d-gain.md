# Phase 1d — internal/gain Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `internal/gain` package — decoder-side gain VQ: from the 7 bits per subframe `(GA 3 bits, GB 4 bits)` of conjugate-structure two-stage VQ indices, and the fixed-codebook vector `c[40]` produced by Phase 1c, decode the two gains used by the excitation sum:

```
u(n) = g_p · v(n) + g_c · c(n)      (Phase 1e will compute u.)
```

`g_p` is looked up directly from the VQ tables; `g_c` is `γ̂_c · g_c0` where `γ̂_c` is also looked up from the VQ and `g_c0` is the MA-predicted open-loop gain derived from `c[40]` energy plus 4 past log-gain prediction errors. This is the **first stateful phase**: the Decoder holds the 4-tap MA predictor state (`pastErrors[4]`) across subframes.

**Architecture:** A `Decoder` struct owns `pastErrors[4]` (Q10 log-gain correction factors from the previous four subframes). Per-subframe `Decode(idx, c) → (g_p, g_c)` reads state, produces the two gains, and updates state. `Reset()` returns to the zero value. Three new tables (GBK1, GBK2, MA coefficients + mean energy) plus one new pow2 LUT land in `internal/tables/`. Two new numerical helpers (`log2Fixed`, `pow2Fixed`) live in `internal/gain/` — they are narrow enough that YAGNI keeps them local until another consumer materializes.

**Tech Stack:** Go 1.22+. Depends on `internal/fixed` for saturating arithmetic primitives (`LMult`, `LMac`, `LShl`, `LShr`, `LAdd`, `Mult`, `MultR`, `Round`, `Add`, `Sub`, `NormL`, `Shl`, `Shr`, `Saturate`) and `internal/tables` for the four new numerical tables. Scratch-from-spec: algorithm from ITU-T G.729 §3.9 (gain VQ structure + MA prediction), §4.1.6 (decoder side + excitation assembly), Annex A §A.3.9 (same tables as full G.729 for the decoder). Four numerical tables transcribed from `tab_ld8a.c` data-array initializers under the merger-doctrine exception; no algorithmic ITU C is consulted.

---

## Context for the implementing engineer

### What this package exists for

At the decoder, after the fixed codebook vector `c[40]` (Phase 1c) and adaptive codebook vector `v[40]` (Phase 1b) are known, the excitation for one subframe is

```
u(n) = g_p · v(n) + g_c · c(n)       for n ∈ [0, 39]
```

This phase decodes `g_p` (pitch gain, Q14) and `g_c` (fixed-codebook gain, Q1), which Phase 1e consumes. The gain VQ quantizes the pair `(g_p, γ̂_c)` where `γ̂_c` is a dimensionless **correction factor** multiplied onto a **predicted open-loop gain** `g_c0` derived from the energy of `c[40]` plus a 4-tap MA prediction of past log-gain correction errors:

```
g_c = γ̂_c · g_c0
```

The MA prediction makes the gain decoder **stateful**: four past log-gain correction factors must be retained across subframes, and updated after each decode with the log of the newly decoded `γ̂_c`.

This package does **not** compute the excitation sum (that is Phase 1e) and does **not** apply clamping for the fixed-codebook pitch enhancement filter's β (that is Phase 1c's `ClampPitchGainForEnhancement`, which the caller calls on the **previous** subframe's `g_p` to drive the **current** subframe's `fcb.Decode`). Phase 1d's only stateful invariant is the MA-predictor tap line.

### ITU-T G.729 sections used in this phase

| Section | Topic | What you will read |
|---|---|---|
| §3.9 | Gain quantization | Conjugate-structure two-stage VQ (7 bits = 3 + 4); energy prediction; MA predictor over past log-gain correction errors; mean-energy constant E̅ |
| §4.1.6 | Decoding of the gains | Decoder-side inversion of §3.9: look up GBK1[GA] + GBK2[GB], compute g_c0 from `c[40]` energy and MA-predicted prediction error, final `g_c = γ̂_c · g_c0`, update past-errors FIFO |
| Annex A §A.3.9 | Reduced-complexity gain VQ | Encoder search is simplified; **the decoder uses the same tables and the same formulas as full G.729** |
| Annex A Table 3 / §3.9 Tables 14–16 | Numerical tables | GBK1 (8 entries of `(g_p₁, γ̂_c₁)`), GBK2 (16 entries of `(g_p₂, γ̂_c₂)`), MA predictor coefficients `b_i`, pow2 LUT |

Read §3.9 and §4.1.6 in full before starting the numerical tasks. **The plan's Q-formats and MA-predictor coefficient values are faithful to standard references but must be verified against the spec text before each commit.** Expect spec deviations similar to Phases 1a–1c and document them in the completion report.

### Bit allocation for one subframe (7 bits)

Two fields per subframe, delivered by the bitstream unpacker as `GA1, GB1` (subframe 1) and `GA2, GB2` (subframe 2):

- **`GA` — 3 bits, range [0, 7].** First-stage VQ index into GBK1.
- **`GB` — 4 bits, range [0, 15].** Second-stage VQ index into GBK2.

### Gain VQ structure (conjugate two-stage)

Per §3.9, the two quantities `(g_p, γ̂_c)` are coded by summing one entry from each of two small codebooks:

```
g_p    = GBK1[GA][0]  +  GBK2[GB][0]       // Q14, range ≈ [0, 1.2]
γ̂_c   = GBK1[GA][1]  +  GBK2[GB][1]       // Q14, correction factor
```

Both additions are `fixed.Add` (saturating Word16). GBK1 has 8 entries, GBK2 has 16 entries — 128 total code points over the joint 7-bit index space.

### Mean-energy constant and MA prediction

Per §3.9, the predicted log-energy of the fixed codebook contribution is

```
Ê(m) = E̅ + Σ_{i=1..4} b_i · Û(m−i)            (in dB, one scalar per subframe)
```

where:

- `E̅` — mean log-energy constant. Common value: `30 dB`. In the fixed-point decoder this is a Q10 constant (one digit after the binary point is plenty — `30 · 2^10 = 30720`).
- `b_i` — 4 MA predictor coefficients, typically `{0.68, 0.58, 0.34, 0.19}` in Q13 ≈ `{5571, 4751, 2785, 1556}`. **Verify against §3.9 eq. (69) / Table 15.**
- `Û(m−i)` — past log-gain correction factors `20·log10(γ̂_c_{m−i})` in Q10 dB, initially set to some default (typically `−14 dB = −14336 Q10`) for the first four subframes.

**Derivation of the predicted open-loop gain `g_c0`.** The fixed codebook vector's raw energy is

```
E_c(m) = Σ_{n=0..39} c(n)^2                     // Word32 Q26 if c is Q13
E̅_c(m) = 10 log10(E_c(m) / 40)                 // subframe average log-energy in dB
```

and the predicted log gain is

```
log(g_c0)(m) · 20 = Ê(m) − E̅_c(m)             // dB
g_c0(m) = 10^((Ê(m) − E̅_c(m)) / 20)
```

Equivalently in log2 form (easier to compute with `NormL` and a pow2 LUT):

```
log2(g_c0) = (Ê − E̅_c) / (20 · log10(2))
           = (Ê − E̅_c) · 0.166           (Q13: 0.166 ≈ 1362)
```

The spec's exact recipe — and in particular whether to fold `log2(10)/20` into the MA coefficients or apply it as a post-multiply — is in §3.9. **The implementing engineer should read the spec's exact equation set (68 through 73 in the 1996 edition) and follow it verbatim rather than the derivation above.** The derivation above is for context only.

### Final g_c

```
g_c = γ̂_c · g_c0                               // Q-format: see below
```

Output `g_c` is conventionally Q1 (range `[0, 16384)` = `[0, 8192.0]`) for Phase 1e's excitation combination `u = g_p·v + g_c·c` where `c` is Q13. The exact shift is determined by the input Q-formats of `γ̂_c` (Q14) and `g_c0` (log-derived). **Confirm the final Q-format against §4.1.6.**

### State update after decode

After computing `γ̂_c`, update the MA predictor state:

```
Û(m) = 20 · log10(γ̂_c)                       // or, equivalently, log2 · (10·log10(2)/log2(2)/20) scale
pastErrors = [Û(m), pastErrors[0], pastErrors[1], pastErrors[2]]
```

Effectively the FIFO shifts right one slot and the freshly-computed log-correction lands at index 0.

### Log/pow fixed-point helpers

Two package-private helpers in `internal/gain/`:

- **`log2Fixed(x fixed.Word32) int16`** — returns `log2(x) · 2^10` (Q10) for `x > 0`. Uses `fixed.NormL` to normalize `x` to a Q31 mantissa, then interpolates within the pow2 LUT (same LUT as pow2, see §3.9). Output is "integer log2 part" in the upper bits and fractional in the lower — a Q10 signed value over roughly `[−30·1024, +30·1024]`. Negative inputs produce zero (caller's responsibility to avoid).

- **`pow2Fixed(xQ10 int16) fixed.Word32`** — inverse of `log2Fixed`. Splits `xQ10` into integer and fractional parts (`int_part = x >> 10`, `frac_part = x & 0x3FF`), interpolates the pow2 LUT for the fractional part, then shifts left by the integer part. Result is a Word32 — the caller shifts/saturates to the desired Q-format.

**Shared `Pow2Table` LUT.** Per §3.9 Table 10 / the pow2 initializer in `tab_ld8a.c`, a small table (typically 33 int16 entries, Q15) provides a piecewise-linear approximation of `2^x` on `x ∈ [0, 1)`. Both `log2Fixed` (inverse lookup) and `pow2Fixed` (forward lookup) use it. Transcribe in Task 5 under merger doctrine.

### API delivered by this phase

```go
package gain

// Indices are the gain-VQ bit-field values for one subframe.
type Indices struct {
    GA uint8  // 3 bits — first-stage codebook index
    GB uint8  // 4 bits — second-stage codebook index
}

// Decoder holds the MA predictor state across subframes.
// The zero value is a valid state (pastErrors initialized lazily on
// the first Decode call to the spec's default of -14 dB Q10).
type Decoder struct {
    pastErrors  [4]int16  // Û(m-1)..Û(m-4), Q10 dB
    initialized bool
}

// Decode decodes one subframe's gains.
//   idx     — 7-bit VQ indices from the bitstream.
//   c       — fixed codebook vector from Phase 1c, Q13.
// Returns (gpQ14, gcQ1). Updates pastErrors as a side-effect.
func (d *Decoder) Decode(idx Indices, c *[40]int16) (gpQ14, gcQ1 int16)

// Reset returns the Decoder to the zero-value state.
func (d *Decoder) Reset()
```

### Package layout produced by this plan

```
g729/internal/gain/
├── doc.go                  (package doc: role, contracts, ITU refs)
├── types.go                (Indices, Decoder)
├── types_test.go
├── energy.go               (fixedCodebookEnergy)
├── energy_test.go
├── log2.go                 (log2Fixed)
├── log2_test.go
├── pow2.go                 (pow2Fixed)
├── pow2_test.go
├── predictor.go            (MA-predicted log gain)
├── predictor_test.go
├── vq.go                   (decodeVQ — GBK1+GBK2 combine)
├── vq_test.go
├── decode.go               (Decoder.Decode, Reset)
├── decode_test.go
├── alloc_test.go           (zero-allocation contract)
└── bench_test.go           (per-subframe benchmark)

g729/internal/tables/
├── gain_gbk1.go            (GainGBK1 — 8 entries of [2]int16, Q14)
├── gain_gbk2.go            (GainGBK2 — 16 entries of [2]int16, Q14)
├── gain_ma.go              (GainMAPredictor [4]int16 Q13, GainMeanEnergyQ10 const)
├── gain_pow2.go            (Pow2Table [33]int16 Q15 LUT)
└── gain_tables_test.go     (shape, range checks)
```

### Dependency contract

- `internal/gain` imports `internal/fixed` and `internal/tables`. No other internal packages.
- Every arithmetic step routes through `internal/fixed`. Built-in `+`, `-`, `*` on `int16`/`int32` is forbidden. Same rule as prior phases.
- No allocation in any public function. `Decoder` methods use pointer receivers; `Decode` returns two primitives, no slices.
- **Decoder is the only state carrier.** `pastErrors` is the sole hidden state; `Reset()` is the only legal mutation path outside of `Decode()`.

### Verification strategy

No ITU test-vector integration in Phase 1d (that's Phase 1g). Correctness rests on:

- **Tables:** shape (lengths), range (entries within spec-stated bounds), and spot checks against spec tables.
- **Energy:** hand-computed `E_c` for small known `c[40]` vectors (single pulse, two pulses, full 4-pulse spec example).
- **log2 / pow2:** closed-form values — `log2Fixed(1 << 31)` should give the Q10 equivalent of log2(2³¹)=31; `pow2Fixed(0)` = `1 << 15` (Q15 = 1.0) or similar; inverse pairs `pow2(log2(x)) ≈ x` within the LUT's interpolation tolerance (a few LSB of Q15).
- **MA predictor:** verify that given all-zero pastErrors the predicted log gain equals `E̅`; given a known non-zero pastErrors vector the result matches hand-computed MA-over-coefficients value.
- **VQ decode:** exhaustive 128-combination check: for every `(GA, GB) ∈ [0,7]×[0,15]`, `decodeVQ` produces the expected sum of table entries. Range check: `g_p ∈ [0, 1.2] Q14`, `γ̂_c ∈ [0, 2] Q14` — all decoded combinations stay within the spec-guaranteed bounds.
- **Full Decode integration:** with a known `c[40]` (single ±8192 pulse), fresh Decoder state (all-default pastErrors), and a chosen `(GA, GB)`, hand-compute the expected `(g_p, g_c)` and verify. Then run a second subframe with a different `(GA, GB)` to confirm pastErrors propagated correctly.
- **Reset:** `Reset()` clears state; two Decoder instances starting from the same Reset'd state and fed identical input sequences produce identical output sequences.
- **Zero allocation:** `testing.AllocsPerRun = 0` on `Decode` (the only public method with a hot-path allocation risk).

### Frame cadence

`Decode` is called **once per subframe** (twice per frame). The MA predictor's tap line spans 4 subframes = 2 frames worth of history.

### Verification commands (run after every task)

- `go test ./internal/gain/... ./internal/tables/... -race` — must PASS.
- `go vet ./internal/gain/... ./internal/tables/...` — must print nothing.

Final (Task 12 completion criteria):

- `go test -run TestNoAllocation -v ./internal/gain/...` — zero-alloc sub-tests PASS.
- `go test -bench=. -benchmem -run=^$ ./internal/gain/...` — `0 B/op, 0 allocs/op` on benchmarks.

---

## Task 1: Package skeleton + Indices and Decoder types

**Files:**
- Create: `internal/gain/doc.go`
- Create: `internal/gain/types.go`
- Create: `internal/gain/types_test.go`

Stand up the package with `Indices` and `Decoder` before any algorithm lands.

- [x] **Step 1: Write the failing shape tests**

Create `internal/gain/types_test.go`:

```go
package gain

import "testing"

func TestIndicesZeroValue(t *testing.T) {
    var idx Indices
    if idx.GA != 0 || idx.GB != 0 {
        t.Fatalf("zero-value Indices = %+v, want all zero", idx)
    }
}

func TestDecoderZeroValueIsValid(t *testing.T) {
    var d Decoder
    // Zero-value Decoder must have all-zero pastErrors and
    // initialized=false.
    for i, v := range d.pastErrors {
        if v != 0 {
            t.Errorf("pastErrors[%d] = %d, want 0", i, v)
        }
    }
    if d.initialized {
        t.Errorf("initialized = true on zero value, want false")
    }
}

func TestDecoderResetClearsState(t *testing.T) {
    d := Decoder{
        pastErrors:  [4]int16{1, 2, 3, 4},
        initialized: true,
    }
    d.Reset()
    for i, v := range d.pastErrors {
        if v != 0 {
            t.Errorf("after Reset, pastErrors[%d] = %d, want 0", i, v)
        }
    }
    if d.initialized {
        t.Errorf("after Reset, initialized = true, want false")
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/gain/ -run Test -v`
Expected: FAIL with "undefined: Indices" / "undefined: Decoder" / "undefined: Reset".

- [x] **Step 3: Create `types.go`**

```go
package gain

// Indices are the gain-VQ bit-field values delivered per subframe
// by the bitstream unpacker. GA and GB are the first- and second-
// stage conjugate-structure VQ indices (ITU-T G.729 §3.9).
type Indices struct {
    GA uint8 // 3 bits — first-stage codebook index (0..7)
    GB uint8 // 4 bits — second-stage codebook index (0..15)
}

// Decoder holds the MA-predictor state used by the gain VQ decoder
// across subframes. The zero value is a valid initial state; the
// first Decode call populates pastErrors with the spec's default.
type Decoder struct {
    pastErrors  [4]int16 // Û(m-1)..Û(m-4), Q10 dB log-gain prediction errors
    initialized bool
}

// Reset returns the Decoder to its zero-value state.
func (d *Decoder) Reset() {
    *d = Decoder{}
}
```

- [x] **Step 4: Create a minimal `doc.go`**

```go
// Package gain implements ITU-T G.729 + Annex A §3.9 / §4.1.6 gain
// VQ decoding: from 7 bits per subframe (GA 3 bits + GB 4 bits)
// and the Phase 1c fixed codebook vector, produce the pitch gain
// g_p (Q14) and fixed codebook gain g_c (Q1) used by the Phase 1e
// excitation sum u(n) = g_p·v(n) + g_c·c(n).
//
// The Decoder holds a 4-tap MA predictor state across subframes.
// Reset() returns to the zero value.
package gain
```

- [x] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/gain/ -run Test -v`
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/gain/doc.go internal/gain/types.go internal/gain/types_test.go
git commit -m "feat(gain): package skeleton + Indices and Decoder types"
```

---

## Task 2: Transcribe GBK1 (gain VQ first-stage codebook)

**Files:**
- Create: `internal/tables/gain_gbk1.go`
- Modify: `internal/tables/gain_tables_test.go` (new file — create this task)

Transcribe the 8-entry first-stage VQ codebook from ITU-T G.729 §3.9 Table 14 / the `gbk1` initializer in `tab_ld8a.c`. Each entry is `[2]int16`: index 0 is the `g_p` component (Q14), index 1 is the `γ̂_c` component (Q14).

**Licensing disclaimer required in the file header** (merger-doctrine transcription — see prior phases).

- [x] **Step 1: Write the failing shape test**

Create `internal/tables/gain_tables_test.go`:

```go
package tables

import "testing"

func TestGainGBK1Shape(t *testing.T) {
    if len(GainGBK1) != 8 {
        t.Fatalf("GainGBK1 length = %d, want 8", len(GainGBK1))
    }
}

func TestGainGBK1EntriesInSpecRange(t *testing.T) {
    // Per §3.9 Table 14: g_p component ∈ [0, 1.2] Q14 (= [0, 19661]);
    // γ̂_c component ∈ [0, 2.0] Q14 (= [0, 32767]). Negative values
    // are legal in the componentwise codebook because the final
    // sums may still lie in the non-negative spec range.
    for i, entry := range GainGBK1 {
        if entry[0] < -19661 || entry[0] > 19661 {
            t.Errorf("GainGBK1[%d][0] = %d, outside spec ±1.2 Q14 range", i, entry[0])
        }
        // γ̂_c component is allowed a wider excursion (the stage-1
        // contribution alone can be negative; the sum across stages
        // is what's constrained). Just verify int16 (implicit).
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tables/ -run TestGainGBK1 -v`
Expected: FAIL with "undefined: GainGBK1".

- [x] **Step 3: Create `gain_gbk1.go`**

Header template:

```go
package tables

// GainGBK1 is the first-stage codebook of the conjugate-structure
// gain VQ per ITU-T G.729 §3.9 Table 14 (equivalently the `gbk1`
// initializer in the ITU reference distribution's tab_ld8a.c).
//
// 8 entries, each a pair (g_p component, γ̂_c component) in Q14.
// The decoder computes
//
//     g_p   = GainGBK1[GA][0] + GainGBK2[GB][0]
//     γ̂_c = GainGBK1[GA][1] + GainGBK2[GB][1]
//
// Values transcribed from the ITU reference distribution's
// tab_ld8a.c data-array initializer under the merger-doctrine
// exception (see repository scratch-from-spec policy).
var GainGBK1 = [8][2]int16{
    // TRANSCRIBE 8 rows from tab_ld8a.c gbk1[]:
    //   each row is {g_p_Q14, γ̂_c_Q14}
    //   verify row order matches the spec's GA → entry mapping.
}
```

**The engineer implementing this task must open `tab_ld8a.c` and transcribe the 8-row initializer verbatim. No algorithmic C code may be consulted from the same file.**

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tables/ -run TestGainGBK1 -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/tables/gain_gbk1.go internal/tables/gain_tables_test.go
git commit -m "feat(tables): add GainGBK1 first-stage gain VQ codebook from ITU §3.9"
```

---

## Task 3: Transcribe GBK2 (gain VQ second-stage codebook)

**Files:**
- Create: `internal/tables/gain_gbk2.go`
- Modify: `internal/tables/gain_tables_test.go`

Transcribe the 16-entry second-stage VQ codebook from §3.9 Table 14 / `gbk2` in `tab_ld8a.c`.

- [x] **Step 1: Write the failing shape test**

Append to `internal/tables/gain_tables_test.go`:

```go
func TestGainGBK2Shape(t *testing.T) {
    if len(GainGBK2) != 16 {
        t.Fatalf("GainGBK2 length = %d, want 16", len(GainGBK2))
    }
}

// For every (GA, GB) combination the componentwise sum must yield
// non-negative g_p and γ̂_c values in their respective spec ranges.
// This is the core VQ invariant — if this test fails, at least one
// table entry is wrong.
func TestGainVQComponentwiseSumsInRange(t *testing.T) {
    for ga := 0; ga < 8; ga++ {
        for gb := 0; gb < 16; gb++ {
            gpSum := int32(GainGBK1[ga][0]) + int32(GainGBK2[gb][0])
            gcSum := int32(GainGBK1[ga][1]) + int32(GainGBK2[gb][1])
            // Per §3.9: g_p ∈ [0, 1.2] Q14 = [0, 19661], γ̂_c ∈ [0, ~2.0] Q14.
            if gpSum < 0 || gpSum > 19661 {
                t.Errorf("g_p sum for (GA=%d, GB=%d) = %d, outside [0, 19661]", ga, gb, gpSum)
            }
            if gcSum < 0 || gcSum > 32767 {
                t.Errorf("γ̂_c sum for (GA=%d, GB=%d) = %d, outside [0, 32767]", ga, gb, gcSum)
            }
        }
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tables/ -run TestGainGBK2 -v`
Expected: FAIL with "undefined: GainGBK2".

- [x] **Step 3: Create `gain_gbk2.go`**

Header template:

```go
package tables

// GainGBK2 is the second-stage codebook of the conjugate-structure
// gain VQ per ITU-T G.729 §3.9 Table 14 (equivalently the `gbk2`
// initializer in tab_ld8a.c).
//
// 16 entries, each a pair (g_p component, γ̂_c component) in Q14.
// Used with GainGBK1 as:
//
//     g_p   = GainGBK1[GA][0] + GainGBK2[GB][0]
//     γ̂_c = GainGBK1[GA][1] + GainGBK2[GB][1]
//
// Values transcribed from tab_ld8a.c data-array initializer under
// the merger-doctrine exception.
var GainGBK2 = [16][2]int16{
    // TRANSCRIBE 16 rows from tab_ld8a.c gbk2[].
}
```

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tables/ -run TestGainGBK2 -v`
Expected: PASS — including `TestGainVQComponentwiseSumsInRange`, which cross-checks both tables together.

- [x] **Step 5: Commit**

```bash
git add internal/tables/gain_gbk2.go internal/tables/gain_tables_test.go
git commit -m "feat(tables): add GainGBK2 second-stage gain VQ codebook from ITU §3.9"
```

---

## Task 4: Transcribe MA predictor coefficients + mean energy

**Files:**
- Create: `internal/tables/gain_ma.go`
- Modify: `internal/tables/gain_tables_test.go`

Transcribe the 4-tap MA predictor coefficients from §3.9 eq. (69) / `pred[]` in `tab_ld8a.c`, plus the mean-energy constant (a scalar, typically 30 dB in Q10).

- [x] **Step 1: Write the failing shape tests**

Append to `internal/tables/gain_tables_test.go`:

```go
func TestGainMAPredictorShape(t *testing.T) {
    if len(GainMAPredictor) != 4 {
        t.Fatalf("GainMAPredictor length = %d, want 4", len(GainMAPredictor))
    }
}

func TestGainMAPredictorCoefficientsPositive(t *testing.T) {
    // Per §3.9 eq. (69) / Table 15: all four b_i are positive Q13
    // values in the range (0, 1). The typical set is
    // {0.68, 0.58, 0.34, 0.19} → Q13 {5571, 4751, 2785, 1556}.
    for i, c := range GainMAPredictor {
        if c <= 0 || c >= 8192 {
            t.Errorf("GainMAPredictor[%d] = %d, outside (0, 8192) Q13 range", i, c)
        }
    }
}

func TestGainMAPredictorCoefficientsSumLessThanOne(t *testing.T) {
    // Stability invariant: sum of positive coefficients < 1 (Q13 = 8192)
    // so the MA predictor itself has unit gain ≤ 1.
    sum := int32(0)
    for _, c := range GainMAPredictor {
        sum += int32(c)
    }
    if sum >= 8192 {
        t.Errorf("Σ b_i = %d Q13, want < 8192 (=1.0)", sum)
    }
}

func TestGainMeanEnergyConstant(t *testing.T) {
    // Per §3.9: E̅ = 30 dB. In Q10 that is 30 · 1024 = 30720.
    if GainMeanEnergyQ10 != 30720 {
        t.Errorf("GainMeanEnergyQ10 = %d, want 30720 (30 dB · 2^10)", GainMeanEnergyQ10)
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tables/ -run TestGainMA -v` and `go test ./internal/tables/ -run TestGainMeanEnergy -v`
Expected: FAIL with "undefined: GainMAPredictor" / "undefined: GainMeanEnergyQ10".

- [x] **Step 3: Create `gain_ma.go`**

Header template:

```go
package tables

// GainMAPredictor holds the 4 MA predictor coefficients b_1..b_4
// used to predict the log-gain correction factor per ITU-T G.729
// §3.9 eq. (69). Values are Q13, transcribed from tab_ld8a.c's
// pred[] initializer under the merger-doctrine exception.
//
// Typical values (verify against spec before committing):
//   b_1 ≈ 0.68, b_2 ≈ 0.58, b_3 ≈ 0.34, b_4 ≈ 0.19
//   Q13:        5571        4751         2785         1556
var GainMAPredictor = [4]int16{
    // TRANSCRIBE 4 values from tab_ld8a.c pred[] in order b_1..b_4.
}

// GainMeanEnergyQ10 is the mean log-energy constant E̅ per ITU-T
// G.729 §3.9 eq. (68). The spec gives E̅ = 30 dB; in Q10 this is
// 30 · 2^10 = 30720. (Q10 is chosen so dB values up to 32.0 fit.)
const GainMeanEnergyQ10 int16 = 30720
```

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tables/ -run "TestGainMA|TestGainMeanEnergy" -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/tables/gain_ma.go internal/tables/gain_tables_test.go
git commit -m "feat(tables): add MA predictor coefficients and mean energy from ITU §3.9"
```

---

## Task 5: Transcribe Pow2 LUT

**Files:**
- Create: `internal/tables/gain_pow2.go`
- Modify: `internal/tables/gain_tables_test.go`

Transcribe the pow2 interpolation LUT from §3.9 / `tabpow[]` in `tab_ld8a.c`. Typical layout: 33 entries of Q15 values approximating `2^(i/32)` for `i ∈ [0, 32]`. Used by both `log2Fixed` and `pow2Fixed` helpers in Task 7 and Task 8.

- [x] **Step 1: Write the failing shape tests**

Append to `internal/tables/gain_tables_test.go`:

```go
func TestPow2TableShape(t *testing.T) {
    if len(Pow2Table) != 33 {
        t.Fatalf("Pow2Table length = %d, want 33", len(Pow2Table))
    }
}

func TestPow2TableEndpoints(t *testing.T) {
    // 2^0 = 1.0 Q15 = 32767 (or 16384 in Q14 — check the spec's
    // chosen Q-format of the table). The first entry must equal
    // the table's "1.0" representation.
    // 2^1 = 2.0 — but since Q15 only holds values < 2, the last
    // entry represents 2^(32/32) = 2 but is typically stored as
    // "about 1.9999" = 32767 to avoid overflow. Verify.
    if Pow2Table[0] <= 0 {
        t.Errorf("Pow2Table[0] = %d, expected a positive Q15 representation of 1.0", Pow2Table[0])
    }
    if Pow2Table[32] <= Pow2Table[0] {
        t.Errorf("Pow2Table monotonic violation: last entry %d ≤ first entry %d",
            Pow2Table[32], Pow2Table[0])
    }
}

func TestPow2TableMonotonic(t *testing.T) {
    // 2^x is strictly increasing; the LUT must be too.
    for i := 1; i < len(Pow2Table); i++ {
        if Pow2Table[i] <= Pow2Table[i-1] {
            t.Errorf("Pow2Table non-monotonic at i=%d: %d not > %d", i, Pow2Table[i], Pow2Table[i-1])
        }
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tables/ -run TestPow2Table -v`
Expected: FAIL with "undefined: Pow2Table".

- [x] **Step 3: Create `gain_pow2.go`**

Header template:

```go
package tables

// Pow2Table is the fixed-point pow2 approximation LUT per ITU-T
// G.729 §3.9 / the tabpow[] initializer in tab_ld8a.c.
//
// 33 entries represent 2^(i/32) for i ∈ [0, 32], in the spec's
// chosen Q-format (typically Q15 with the last entry saturated to
// 32767 ≈ 2·(1 − 2⁻¹⁵)). Used by both log2Fixed (inverse lookup
// with linear interpolation) and pow2Fixed (forward lookup).
//
// Values transcribed from tab_ld8a.c data-array initializer under
// the merger-doctrine exception.
var Pow2Table = [33]int16{
    // TRANSCRIBE 33 values from tab_ld8a.c tabpow[].
}
```

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tables/ -run TestPow2Table -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/tables/gain_pow2.go internal/tables/gain_tables_test.go
git commit -m "feat(tables): add Pow2Table LUT for log/pow helpers from ITU §3.9"
```

---

## Task 6: Fixed codebook energy

**Files:**
- Create: `internal/gain/energy.go`
- Create: `internal/gain/energy_test.go`

Compute `E_c = Σ c[n]²` as a Word32. With `c` in Q13 each product `c[n]·c[n]` is ≤ `8192² = 2²⁶`; the sum of 40 such products stays ≤ `40·2²⁶ < 2³¹`, so no saturation is possible for canonical 4-pulse codebook vectors.

- [x] **Step 1: Write the failing energy test**

Create `internal/gain/energy_test.go`:

```go
package gain

import (
    "testing"

    "github.com/exedev/g729/internal/fixed"
)

func TestFixedCodebookEnergy_Zero(t *testing.T) {
    var c [40]int16
    if got := fixedCodebookEnergy(&c); got != 0 {
        t.Fatalf("energy(all zero) = %d, want 0", got)
    }
}

func TestFixedCodebookEnergy_SinglePulse(t *testing.T) {
    // c[5] = 8192 (Q13 = +1.0). Energy = 8192² = 67108864.
    // LMult produces 2·a·b, so fixedCodebookEnergy should return
    // the *raw* sum of a·b (not 2·a·b) — i.e. the Word32 after
    // shifting LMac's accumulation right by 1 or accumulating
    // with `acc = LAdd(acc, LMult(x,x) >> 1)`. Verify which
    // convention is used by the spec (§3.9 eq. (68)) and align
    // the test expectation.
    var c [40]int16
    c[5] = 8192
    const want fixed.Word32 = 8192 * 8192 // = 67108864 (= 2^26)
    if got := fixedCodebookEnergy(&c); got != want {
        t.Fatalf("energy(single pulse) = %d, want %d", got, want)
    }
}

func TestFixedCodebookEnergy_FourPulses(t *testing.T) {
    // All 4 pulses at magnitude 8192.
    var c [40]int16
    c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192
    const want fixed.Word32 = 4 * 8192 * 8192
    if got := fixedCodebookEnergy(&c); got != want {
        t.Fatalf("energy(4 pulses) = %d, want %d", got, want)
    }
}

func TestFixedCodebookEnergy_SquaringIsUnsigned(t *testing.T) {
    // ±pulse produces the same energy contribution.
    var cp, cn [40]int16
    cp[3] = 8192
    cn[3] = -8192
    if ep, en := fixedCodebookEnergy(&cp), fixedCodebookEnergy(&cn); ep != en {
        t.Errorf("energy(+pulse) = %d, energy(-pulse) = %d — must be equal", ep, en)
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/gain/ -run TestFixedCodebookEnergy -v`
Expected: FAIL with "undefined: fixedCodebookEnergy".

- [x] **Step 3: Implement `energy.go`**

```go
package gain

import "github.com/exedev/g729/internal/fixed"

// fixedCodebookEnergy returns Σ c[n]² as a Word32.
//
// Per ITU-T G.729 §3.9 eq. (68) the raw energy (without the
// factor-of-2 that fixed.LMult introduces) is needed. This function
// therefore accumulates the half-scaled products:
//
//   E_c = Σ (c[n] · c[n])   not   2 · Σ c[n]·c[n]
//
// For c in Q13, the Q-format of E_c is Q26 (13+13). With 4 pulses
// of ±8192 the maximum value is 4·2²⁶ = 2²⁸, well under Word32's
// positive range.
func fixedCodebookEnergy(c *[40]int16) fixed.Word32 {
    var acc fixed.Word32
    for n := 0; n < 40; n++ {
        // LMult gives 2·a·b; shift right 1 to get a·b.
        acc = fixed.LAdd(acc, fixed.LShr(fixed.LMult(c[n], c[n]), 1))
    }
    return acc
}
```

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/gain/ -run TestFixedCodebookEnergy -v`
Expected: PASS on all 4 sub-tests.

- [x] **Step 5: Commit**

```bash
git add internal/gain/energy.go internal/gain/energy_test.go
git commit -m "feat(gain): compute fixed codebook energy E_c per ITU §3.9 eq. (68)"
```

---

## Task 7: log2Fixed helper

**Files:**
- Create: `internal/gain/log2.go`
- Create: `internal/gain/log2_test.go`

Compute `log2(x) · 2^10` (Q10 signed int16) for Word32 `x > 0`. Uses `fixed.NormL` to normalize the input into a Q31 mantissa, then interpolates within `tables.Pow2Table`. Negative or zero inputs return 0 (caller's responsibility to avoid).

**Algorithm (per §3.9 / standard ITU pattern — verify against spec):**

```
1. If x ≤ 0 return 0.
2. shift = NormL(x)            ; bits to left-shift so MSB becomes the sign bit
3. norm_x = x << shift          ; now norm_x ∈ [2^30, 2^31), so log2(norm_x) = 30 + fractional_part
4. integer_log2 = 30 − shift    ; integer part of log2(original x)
5. mantissa_Q15 = (norm_x >> 15) & 0xFFFF  ; upper fractional 16 bits, then mask
   Specifically: normalize so mantissa ∈ [0.5, 1.0), then work in the top 5 bits
   as an index into Pow2Table (0..32) plus 10-bit inter-entry fraction for linear
   interpolation.
6. Look up Pow2Table[idx] and Pow2Table[idx+1]; linearly interpolate over the
   10-bit fraction; invert (table is pow2, we want log2 — the spec uses the same
   table for both by lookup arithmetic).
7. result_Q10 = (integer_log2 << 10) + interpolated_fraction_Q10
```

**The exact bit-splitting between mantissa index and interpolation fraction is spec-defined. Implement per §3.9.**

- [x] **Step 1: Write the failing tests**

Create `internal/gain/log2_test.go`:

```go
package gain

import (
    "testing"

    "github.com/exedev/g729/internal/fixed"
)

func TestLog2Fixed_PowersOfTwoAreExact(t *testing.T) {
    // log2(2^k) = k, so log2Fixed(1<<k as Word32) = k · 1024 Q10.
    cases := []struct {
        x    fixed.Word32
        want int16
    }{
        {1 << 0, 0 * 1024},     // log2(1) = 0
        {1 << 10, 10 * 1024},   // log2(1024) = 10
        {1 << 15, 15 * 1024},   // log2(32768) = 15
        {1 << 20, 20 * 1024},   // log2(2^20) = 20
        {1 << 30, 30 * 1024},   // log2(2^30) = 30
    }
    for _, c := range cases {
        got := log2Fixed(c.x)
        // Tolerance: ±1 LSB of Q10 is acceptable rounding drift
        // at table interpolation boundaries. Powers of 2 fall on
        // Pow2Table[0] exactly so should be bit-exact, but allow
        // ±1 for safety.
        if diff := got - c.want; diff > 1 || diff < -1 {
            t.Errorf("log2Fixed(%d) = %d Q10, want %d ±1", c.x, got, c.want)
        }
    }
}

func TestLog2Fixed_Log2OfThreeApprox(t *testing.T) {
    // log2(3) ≈ 1.584962500721... → Q10 ≈ 1623.
    // Tolerance wider for a non-exact point (interpolation error).
    got := log2Fixed(3)
    const want = 1623
    if diff := got - want; diff > 4 || diff < -4 {
        t.Errorf("log2Fixed(3) = %d Q10, want ≈%d (±4)", got, want)
    }
}

func TestLog2Fixed_ZeroReturnsZero(t *testing.T) {
    if got := log2Fixed(0); got != 0 {
        t.Errorf("log2Fixed(0) = %d, want 0 (guard)", got)
    }
}

func TestLog2Fixed_NegativeReturnsZero(t *testing.T) {
    if got := log2Fixed(-1); got != 0 {
        t.Errorf("log2Fixed(-1) = %d, want 0 (guard)", got)
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/gain/ -run TestLog2Fixed -v`
Expected: FAIL with "undefined: log2Fixed".

- [x] **Step 3: Implement `log2.go`**

```go
package gain

import (
    "github.com/exedev/g729/internal/fixed"
    "github.com/exedev/g729/internal/tables"
)

// log2Fixed returns log2(x) · 2^10 (Q10 signed) for x > 0.
// Non-positive inputs return 0 (caller must avoid these).
//
// Per ITU-T G.729 §3.9. Algorithm:
//
//   1. NormL(x) returns the left-shift count that puts x's MSB at
//      bit 30 (just below the sign bit). After the shift x is in
//      [2^30, 2^31), representing log2(x) = (30 − shift) + frac
//      where frac ∈ [0, 1).
//   2. The fractional part is found by interpolating in
//      tables.Pow2Table, which stores 33 samples of 2^(i/32).
//   3. Result = integer_log2 << 10 + interpolated_frac.
func log2Fixed(x fixed.Word32) int16 {
    if x <= 0 {
        return 0
    }
    shift := fixed.NormL(x)
    normX := fixed.LShl(x, shift)
    // Integer log2 part (Q10): (30 - shift) · 1024
    integerLog2 := (30 - int32(shift)) << 10

    // Extract top 5 bits of the normalized mantissa (after the
    // implicit leading 1) as Pow2Table index [0..31], plus the next
    // 10 bits as interpolation fraction.
    // normX is in [2^30, 2^31). Dropping the leading bit leaves
    // 30 bits of mantissa; use the top 15 bits: 5-bit index +
    // 10-bit fraction.
    mantissaQ15 := int32(normX>>15) & 0xFFFF // bits 15..30
    // Reinterpret mantissaQ15 ∈ [0x8000, 0xFFFF]; subtract 0x8000
    // to get the 15-bit fractional excursion above 1.0.
    frac15 := mantissaQ15 - 0x8000 // ∈ [0, 0x8000)
    idx := frac15 >> 10            // 5-bit index into Pow2Table
    interp := frac15 & 0x3FF       // 10-bit interpolation fraction

    a := int32(tables.Pow2Table[idx])
    b := int32(tables.Pow2Table[idx+1])
    // Linear interpolation in the table's pow2 values, then invert
    // to obtain log2 fraction scaled to Q10. The exact formulation
    // is spec-defined; the canonical form uses the index itself as
    // the log2 fraction (since tables.Pow2Table[i] ≈ 2^(i/32)):
    //
    //   fracLog2Q10 = (idx << 5) + (interp · (b − a)) / ((b − a) · 1024)
    //
    // which simplifies (the `(b − a)` cancels if tables are ideal)
    // to the spec's closed form: read §3.9 for the exact shift
    // counts. The form below approximates it within ±1 LSB for
    // powers of two (exact) and within ±4 LSB across the mantissa.
    fracLog2Q10 := (idx << 5) + ((interp * 32) >> 10)

    return int16(integerLog2 + fracLog2Q10)
}
```

**Note:** The derivation sketch above (`fracLog2Q10 = (idx<<5) + ((interp*32)>>10)`) is a placeholder first cut. **Read §3.9 carefully and adjust the arithmetic until `TestLog2Fixed_Log2OfThreeApprox` passes within ±4 and `TestLog2Fixed_PowersOfTwoAreExact` passes within ±1.** If the spec uses a different table arrangement (some editions split mantissa into 5+10 bits differently), follow the spec and update the test tolerances in the completion report notes only if the spec tolerates looser bounds.

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/gain/ -run TestLog2Fixed -v`
Expected: PASS on all 4 sub-tests.

- [x] **Step 5: Commit**

```bash
git add internal/gain/log2.go internal/gain/log2_test.go
git commit -m "feat(gain): log2 fixed-point helper using Pow2Table interpolation per ITU §3.9"
```

---

## Task 8: pow2Fixed helper

**Files:**
- Create: `internal/gain/pow2.go`
- Create: `internal/gain/pow2_test.go`

Inverse of `log2Fixed`: given a Q10 log2 value, return `2^x` as a Word32.

**Algorithm:**

```
1. integer_part = x >> 10  (sign-extended)
2. frac_part    = x & 0x3FF (unsigned 10 bits)
3. Interpolate in Pow2Table to get 2^(frac_part/1024) in Q15.
4. Left-shift by integer_part (clamping for saturation).
5. Return as Word32.
```

- [x] **Step 1: Write the failing tests**

Create `internal/gain/pow2_test.go`:

```go
package gain

import (
    "testing"

    "github.com/exedev/g729/internal/fixed"
)

func TestPow2Fixed_IntegerPowers(t *testing.T) {
    // pow2(k · 1024 Q10) = 2^k as a Word32. Caller interprets the
    // Q-format based on context; the function returns the raw
    // integer-domain 2^x value.
    cases := []struct {
        xQ10 int16
        want fixed.Word32
    }{
        {0 * 1024, 1 << 15},   // 2^0 = 1.0 Q15 = 32768 (or 1 in Q0 after extraction)
        {5 * 1024, 1 << 20},   // 2^5 = 32 in Q15 = 32·32768 = 1048576
        {10 * 1024, 1 << 25},  // 2^10 = 1024 in Q15
    }
    for _, c := range cases {
        got := pow2Fixed(c.xQ10)
        // Tolerance: the LUT interpolates — integer points are exact
        // (Pow2Table[0] = 32767 or similar). Accept ±1 for rounding.
        if diff := int64(got) - int64(c.want); diff > 1 || diff < -1 {
            t.Errorf("pow2Fixed(%d Q10) = %d, want %d ±1", c.xQ10, got, c.want)
        }
    }
}

func TestPow2Fixed_HalfPowerApprox(t *testing.T) {
    // pow2(0.5 · 1024 = 512 Q10) = √2 ≈ 1.41421... → Q15 ≈ 46341.
    got := pow2Fixed(512)
    const want fixed.Word32 = 46341
    if diff := int64(got) - int64(want); diff > 10 || diff < -10 {
        t.Errorf("pow2Fixed(512 Q10 = 0.5) = %d, want ≈%d (±10)", got, want)
    }
}

func TestPow2Fixed_RoundTripsThroughLog2(t *testing.T) {
    // For positive x, log2(pow2(x)) ≈ x within a few Q10 LSBs.
    cases := []int16{0, 1024, 2048, 512, 1536, 5120, 10240}
    for _, x := range cases {
        y := pow2Fixed(x)
        xBack := log2Fixed(y)
        if diff := xBack - x; diff > 4 || diff < -4 {
            t.Errorf("log2(pow2(%d)) = %d, want %d ±4", x, xBack, x)
        }
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/gain/ -run TestPow2Fixed -v`
Expected: FAIL with "undefined: pow2Fixed".

- [x] **Step 3: Implement `pow2.go`**

```go
package gain

import (
    "github.com/exedev/g729/internal/fixed"
    "github.com/exedev/g729/internal/tables"
)

// pow2Fixed returns 2^(xQ10 / 2^10) as a Word32.
//
// Per ITU-T G.729 §3.9. Algorithm:
//
//   1. Integer part = xQ10 >> 10 (sign-preserving).
//   2. Fractional part = xQ10 & 0x3FF (unsigned 10 bits).
//   3. Look up and linearly interpolate within tables.Pow2Table
//      using the upper 5 bits of the fractional part as index and
//      the lower 5 bits as the interpolation fraction.
//   4. Left-shift the interpolated Q15 value by the integer part.
//
// Negative xQ10 right-shifts the interpolated value. Saturates to
// the Word32 range.
func pow2Fixed(xQ10 int16) fixed.Word32 {
    intPart := int32(xQ10) >> 10
    fracPart := int32(xQ10) & 0x3FF

    idx := fracPart >> 5    // 0..31
    interp := fracPart & 0x1F // 0..31

    a := fixed.Word32(tables.Pow2Table[idx])   // Q15
    b := fixed.Word32(tables.Pow2Table[idx+1]) // Q15
    // Linear interpolation: y = a + (b − a) · interp / 32
    interpolated := a + ((b-a)*fixed.Word32(interp))>>5 // Q15

    // Shift by integer part. Positive shifts scale up (2^k), negative
    // shifts scale down. fixed.LShl handles positive shift saturation;
    // LShr handles negative shifts.
    if intPart >= 0 {
        return fixed.LShl(interpolated, int16(intPart))
    }
    return fixed.LShr(interpolated, int16(-intPart))
}
```

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/gain/ -run TestPow2Fixed -v`
Expected: PASS on all 3 sub-tests.

- [x] **Step 5: Commit**

```bash
git add internal/gain/pow2.go internal/gain/pow2_test.go
git commit -m "feat(gain): pow2 fixed-point helper using Pow2Table interpolation per ITU §3.9"
```

---

## Task 9: MA predictor of log gain

**Files:**
- Create: `internal/gain/predictor.go`
- Create: `internal/gain/predictor_test.go`

Compute the MA-predicted log-gain per §3.9 eq. (69):

```
Ê(m) = E̅ + Σ_{i=1..4} b_i · Û(m-i)         (Q10 dB)
```

where `b_i` are `tables.GainMAPredictor` (Q13), `Û(m-i)` are the Decoder's `pastErrors` (Q10 dB), and `E̅` is `tables.GainMeanEnergyQ10`.

The product `b_i · Û(m-i)` is Q13·Q10 = Q23; summing four then rounding to Q10 requires `>>13` with rounding, or aligning via `fixed.LMac` and `fixed.Round`.

**Q-format sequence:**

```
For each i: acc += LMult(b_i_Q13, Û_Q10)         ; Q24 Word32 (Q13 · Q10 · 2)
After 4 accumulations: Round(LShl(acc, 2))       ; Q10 Word16 (Q24 · 4 = Q26; Round → Q10)
Add E̅_Q10 via fixed.Add.
```

**The exact alignment is spec-defined. Verify against §3.9 before finalizing.** The values above are derivation sketch.

- [x] **Step 1: Write the failing tests**

Create `internal/gain/predictor_test.go`:

```go
package gain

import (
    "testing"

    "github.com/exedev/g729/internal/tables"
)

func TestPredictedLogGain_AllZeroPastErrors(t *testing.T) {
    // With pastErrors = 0, Ê(m) = E̅ = 30720 Q10.
    var d Decoder
    got := d.predictedLogGain()
    if got != tables.GainMeanEnergyQ10 {
        t.Errorf("predictedLogGain(pastErrors=0) = %d, want %d (= E̅)",
            got, tables.GainMeanEnergyQ10)
    }
}

func TestPredictedLogGain_KnownPastErrors(t *testing.T) {
    // pastErrors = {1024, 1024, 1024, 1024} Q10 = {1, 1, 1, 1} dB.
    // Σ b_i · Û = (b_1+b_2+b_3+b_4) · 1024 Q10·Q13/Q13.
    // With b = {0.68, 0.58, 0.34, 0.19}, Σ ≈ 1.79.
    // Ê = 30720 + 1.79 · 1024 ≈ 30720 + 1833 = 32553 Q10.
    d := Decoder{pastErrors: [4]int16{1024, 1024, 1024, 1024}}
    got := d.predictedLogGain()
    const want = 32553
    // Tolerance: ±4 LSB for Q13 · Q10 rounding.
    if diff := got - want; diff > 4 || diff < -4 {
        t.Errorf("predictedLogGain = %d, want ≈%d (±4)", got, want)
    }
}

func TestPredictedLogGain_OnlyFirstTapContributes(t *testing.T) {
    d := Decoder{pastErrors: [4]int16{1024, 0, 0, 0}}
    got := d.predictedLogGain()
    // E̅ + b_1 · 1024 ≈ 30720 + 0.68·1024 = 30720 + 696 = 31416 Q10.
    const want = 31416
    if diff := got - want; diff > 4 || diff < -4 {
        t.Errorf("predictedLogGain = %d, want ≈%d (±4)", got, want)
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/gain/ -run TestPredictedLogGain -v`
Expected: FAIL with "undefined: predictedLogGain".

- [x] **Step 3: Implement `predictor.go`**

```go
package gain

import (
    "github.com/exedev/g729/internal/fixed"
    "github.com/exedev/g729/internal/tables"
)

// predictedLogGain computes the MA-predicted log-gain Ê(m) per
// ITU-T G.729 §3.9 eq. (69):
//
//   Ê(m) = E̅ + Σ_{i=1..4} b_i · Û(m-i)        (Q10 dB)
//
// where b_i = tables.GainMAPredictor[i-1] (Q13), Û(m-i) =
// d.pastErrors[i-1] (Q10), and E̅ = tables.GainMeanEnergyQ10.
func (d *Decoder) predictedLogGain() int16 {
    var acc fixed.Word32
    for i := 0; i < 4; i++ {
        acc = fixed.LMac(acc, tables.GainMAPredictor[i], d.pastErrors[i])
    }
    // acc is in Q(13+10+1) = Q24. Align to Q26 (so Round → Q10) by
    // shifting left by 2.
    predicted := fixed.Round(fixed.LShl(acc, 2))
    return fixed.Add(tables.GainMeanEnergyQ10, predicted)
}
```

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/gain/ -run TestPredictedLogGain -v`
Expected: PASS on all 3 sub-tests.

- [x] **Step 5: Commit**

```bash
git add internal/gain/predictor.go internal/gain/predictor_test.go
git commit -m "feat(gain): MA-predicted log gain Ê(m) per ITU §3.9 eq. (69)"
```

---

## Task 10: Gain VQ lookup

**Files:**
- Create: `internal/gain/vq.go`
- Create: `internal/gain/vq_test.go`

Decode the two-stage VQ: `(g_p, γ̂_c) = GBK1[GA] + GBK2[GB]`.

- [x] **Step 1: Write the failing tests**

Create `internal/gain/vq_test.go`:

```go
package gain

import (
    "testing"

    "github.com/exedev/g729/internal/tables"
)

func TestDecodeVQ_SumsMatchTableEntries(t *testing.T) {
    // Exhaustive: for every (GA, GB) combination, decodeVQ must
    // equal componentwise saturating addition of GBK1[GA] + GBK2[GB].
    for ga := uint8(0); ga < 8; ga++ {
        for gb := uint8(0); gb < 16; gb++ {
            gp, gammaC := decodeVQ(Indices{GA: ga, GB: gb})
            wantGp := int32(tables.GainGBK1[ga][0]) + int32(tables.GainGBK2[gb][0])
            wantGc := int32(tables.GainGBK1[ga][1]) + int32(tables.GainGBK2[gb][1])
            // Clamp expected sums to int16 for saturation comparison.
            if wantGp > 32767 {
                wantGp = 32767
            } else if wantGp < -32768 {
                wantGp = -32768
            }
            if wantGc > 32767 {
                wantGc = 32767
            } else if wantGc < -32768 {
                wantGc = -32768
            }
            if int32(gp) != wantGp {
                t.Errorf("(GA=%d, GB=%d): g_p = %d, want %d", ga, gb, gp, wantGp)
            }
            if int32(gammaC) != wantGc {
                t.Errorf("(GA=%d, GB=%d): γ̂_c = %d, want %d", ga, gb, gammaC, wantGc)
            }
        }
    }
}

func TestDecodeVQ_GPInSpecRange(t *testing.T) {
    // Every decoded g_p must land in [0, 1.2] Q14 = [0, 19661] per §3.9.
    for ga := uint8(0); ga < 8; ga++ {
        for gb := uint8(0); gb < 16; gb++ {
            gp, _ := decodeVQ(Indices{GA: ga, GB: gb})
            if gp < 0 || gp > 19661 {
                t.Errorf("(GA=%d, GB=%d): g_p = %d out of [0, 19661]", ga, gb, gp)
            }
        }
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/gain/ -run TestDecodeVQ -v`
Expected: FAIL with "undefined: decodeVQ".

- [x] **Step 3: Implement `vq.go`**

```go
package gain

import (
    "github.com/exedev/g729/internal/fixed"
    "github.com/exedev/g729/internal/tables"
)

// decodeVQ performs the conjugate-structure two-stage codebook
// lookup per ITU-T G.729 §3.9:
//
//   g_p   = GainGBK1[GA][0] + GainGBK2[GB][0]      (Q14)
//   γ̂_c = GainGBK1[GA][1] + GainGBK2[GB][1]      (Q14)
//
// Both sums saturate at Word16.
func decodeVQ(idx Indices) (gpQ14, gammaCQ14 int16) {
    gpQ14 = fixed.Add(tables.GainGBK1[idx.GA][0], tables.GainGBK2[idx.GB][0])
    gammaCQ14 = fixed.Add(tables.GainGBK1[idx.GA][1], tables.GainGBK2[idx.GB][1])
    return
}
```

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/gain/ -run TestDecodeVQ -v`
Expected: PASS on the 128-combination exhaustive check and the range check.

- [x] **Step 5: Commit**

```bash
git add internal/gain/vq.go internal/gain/vq_test.go
git commit -m "feat(gain): conjugate-structure VQ lookup per ITU §3.9"
```

---

## Task 11: Top-level Decode + state update

**Files:**
- Create: `internal/gain/decode.go`
- Create: `internal/gain/decode_test.go`

Wire everything together:

```
1. Lazy state init on first call:
   if !d.initialized: pastErrors ← {-14336, -14336, -14336, -14336} (−14 dB Q10)
                      initialized ← true
2. E_c = fixedCodebookEnergy(c)                                   // Word32 Q26
3. ec_log2_Q10 = log2Fixed(E_c)                                   // Q10
4. E̅_c_dB_Q10 = 10·log10(2) · (ec_log2_Q10 − log2_of_40)         // ≈ 3.0103 · (ec_log2 − 5.32·1024)
   Shortcut: the spec folds constants; consult §3.9 for the exact arithmetic.
5. Ê = predictedLogGain() − E̅_c_dB_Q10                           // Q10 dB
6. log2_gc0 = Ê / (20 · log10(2))                                 // convert dB to log2
7. g_c0 = pow2Fixed(log2_gc0)                                     // Word32
8. (g_p, γ̂_c) = decodeVQ(idx)
9. g_c = γ̂_c · g_c0  (aligned to Q1)
10. Update state:
    U_current = 20·log10(γ̂_c) in Q10
    pastErrors = [U_current, pastErrors[0], pastErrors[1], pastErrors[2]]
```

**The dB↔log2 conversion, the `10·log10(E_c/40)` formulation, and the final g_c Q-format are all spec-defined. Read §3.9 + §4.1.6 and implement verbatim.** Below is a candidate implementation using the helpers from Tasks 6–10; the engineer must verify each shift and Q-format against the spec.

- [x] **Step 1: Write the failing integration test**

Create `internal/gain/decode_test.go`:

```go
package gain

import "testing"

// Smoke test: fresh Decoder, single-pulse c, known (GA, GB).
// Verify g_p and g_c are non-zero and in plausible ranges, and
// that pastErrors has been updated (the first slot is no longer
// at its default −14336).
func TestDecode_ProducesGainsAndUpdatesState(t *testing.T) {
    var d Decoder
    var c [40]int16
    c[5] = 8192 // single +PulseAmplitude
    idx := Indices{GA: 3, GB: 7}

    gp, gc := d.Decode(idx, &c)

    if gp <= 0 || gp > 20000 {
        t.Errorf("g_p = %d, out of plausible Q14 range [1, 20000]", gp)
    }
    if gc <= 0 {
        t.Errorf("g_c = %d, want positive Q1", gc)
    }
    if !d.initialized {
        t.Errorf("initialized = false after Decode, want true")
    }
    // At least one pastErrors slot should reflect the decoded γ̂_c;
    // specifically pastErrors[0] must differ from the default -14336.
    if d.pastErrors[0] == -14336 {
        t.Errorf("pastErrors[0] unchanged from default after Decode")
    }
}

func TestDecode_TwoSubframesStatePropagation(t *testing.T) {
    var d Decoder
    var c [40]int16
    c[0] = 8192

    _, _ = d.Decode(Indices{GA: 0, GB: 0}, &c)
    before := d.pastErrors
    _, _ = d.Decode(Indices{GA: 0, GB: 0}, &c)
    after := d.pastErrors

    // After two identical calls, the FIFO should have shifted:
    // after[0] == before[0] (same γ̂_c input so same log-error),
    // after[1] == before[0] (previous slot moved down),
    // after[2] == before[1] (-14336 default).
    if after[0] != before[0] {
        t.Errorf("after[0] = %d, before[0] = %d — same input should give same log-error",
            after[0], before[0])
    }
    if after[1] != before[0] {
        t.Errorf("after[1] = %d, before[0] = %d — FIFO shift broken", after[1], before[0])
    }
    if after[2] != before[1] {
        t.Errorf("after[2] = %d, before[1] = %d — FIFO shift broken", after[2], before[1])
    }
}

func TestDecode_ResetRestoresZeroValueDeterminism(t *testing.T) {
    var c [40]int16
    c[0] = 8192
    idx := Indices{GA: 2, GB: 5}

    var d1 Decoder
    gp1, gc1 := d1.Decode(idx, &c)

    var d2 Decoder
    _, _ = d2.Decode(idx, &c)
    _, _ = d2.Decode(idx, &c)
    d2.Reset()
    gp2, gc2 := d2.Decode(idx, &c)

    if gp1 != gp2 || gc1 != gc2 {
        t.Errorf("after Reset, outputs differ: (%d, %d) vs (%d, %d)", gp1, gc1, gp2, gc2)
    }
}
```

- [x] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/gain/ -run TestDecode_ -v`
Expected: FAIL with "undefined: Decode".

- [x] **Step 3: Implement `decode.go`**

```go
package gain

import (
    "github.com/exedev/g729/internal/fixed"
)

// pastErrorsDefault is the spec's initial value for each entry of
// the MA-predictor tap line (§3.9 / §4.1.6): -14 dB Q10.
const pastErrorsDefault int16 = -14336

// Decode decodes one subframe's gains from idx and the fixed
// codebook vector c. Returns (gpQ14, gcQ1) per ITU-T G.729
// §3.9 / §4.1.6. Side-effect: updates d.pastErrors (the MA
// predictor tap line) with the log of the decoded γ̂_c.
func (d *Decoder) Decode(idx Indices, c *[40]int16) (gpQ14, gcQ1 int16) {
    if !d.initialized {
        for i := range d.pastErrors {
            d.pastErrors[i] = pastErrorsDefault
        }
        d.initialized = true
    }

    // 1. Predict log-gain from past errors.
    predicted := d.predictedLogGain() // Q10 dB

    // 2. Compute E̅_c = 10·log10(E_c / 40) in Q10 dB.
    //    10·log10(x) = 3.0103 · log2(x). 3.0103 Q13 ≈ 24660.
    //    Constant 10·log10(40) ≈ 16.02 dB → Q10 = 16402.
    ecEnergy := fixedCodebookEnergy(c)          // Word32 Q26
    ecLog2Q10 := log2Fixed(ecEnergy)            // Q10 log2
    ecDbQ10 := fixed.Round(fixed.LShl(
        fixed.LMult(ecLog2Q10, 24660), 2))      // (Q10·Q13·2)·4 = Q26; Round → Q10; ·3.0103
    ecBarDbQ10 := fixed.Sub(ecDbQ10, 16402)     // subtract 10·log10(40)

    // 3. Effective log gain in dB, then convert dB → log2.
    logGainDbQ10 := fixed.Sub(predicted, ecBarDbQ10)   // Q10 dB
    // log2 = log10 · log2(10) = log10 · 3.3219 → use inverse:
    // log2(g_c0) = dB / (20·log10(2)) = dB · 0.166 ≈ dB · 5443 Q15
    log2GcQ10 := fixed.Round(fixed.LShl(
        fixed.LMult(logGainDbQ10, 5443), 1))     // (Q10·Q15·2)·2 = Q26; Round → Q10

    // 4. pow2 → g_c0, multiply by γ̂_c.
    gc0 := pow2Fixed(log2GcQ10) // Word32, scale depends on int_part
    gp, gammaC := decodeVQ(idx)
    gpQ14 = gp
    gcQ1 = fixed.Round(fixed.LShl(
        fixed.LMult(gammaC, int16(gc0>>something)), 1)) // TODO: align Q-format

    // 5. Update pastErrors FIFO:
    //    U(m) = 20·log10(γ̂_c) Q10 = log2(γ̂_c) · 6.0206 Q10.
    gammaLog2 := log2Fixed(fixed.Word32(gammaC)) // Q10 log2 of γ̂_c (Q14 value)
    // 20·log10(γ̂_c) = log2(γ̂_c) · 6.0206 = log2 · 6165 Q10
    uCurrent := fixed.Round(fixed.LShl(
        fixed.LMult(gammaLog2, 6165), 2))        // (Q10·Q10·2)·4 = Q24; Round → Q8? — adjust
    d.pastErrors[3] = d.pastErrors[2]
    d.pastErrors[2] = d.pastErrors[1]
    d.pastErrors[1] = d.pastErrors[0]
    d.pastErrors[0] = uCurrent

    return
}
```

**This implementation sketch has `TODO: align Q-format` and inline constants (5443, 6165, 24660, 16402) that are derivation guesses. The implementing engineer must:**

1. **Open §3.9 + §4.1.6** and follow the spec's exact equation chain.
2. **Compute each Q-format step** on paper before coding.
3. **Replace the derivation-guess constants** with spec-faithful values (likely from `tab_ld8a.c` if any show up as tabulated).
4. **Fix the `gc0>>something` alignment** to produce Q1 output.
5. **Verify the `20·log10(γ̂_c)` path** for the FIFO update — this must match what the encoder side would have produced so that the decoder's MA state stays synchronized with the encoder's prediction.

The integration tests deliberately use wide tolerance ranges and focus on **structural** correctness (state update, reset determinism, output ranges) rather than exact output values. **Bit-exact gain values are Phase 1g's validation territory.**

- [x] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/gain/ -run TestDecode_ -v`
Expected: PASS on all 3 sub-tests.

Also run the full package suite:

Run: `go test -race ./internal/gain/...`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/gain/decode.go internal/gain/decode_test.go
git commit -m "feat(gain): top-level Decode with MA state update per ITU §3.9 / §4.1.6"
```

---

## Task 12: Zero-allocation contract, benchmarks, and doc polish

**Files:**
- Create: `internal/gain/alloc_test.go`
- Create: `internal/gain/bench_test.go`
- Modify: `internal/gain/doc.go`

Lock in the zero-allocation hot path and add a canonical benchmark.

- [x] **Step 1: Write the failing allocation test**

Create `internal/gain/alloc_test.go`:

```go
package gain

import "testing"

func TestNoAllocationInDecode(t *testing.T) {
    var d Decoder
    var c [40]int16
    c[5] = 8192
    idx := Indices{GA: 3, GB: 7}

    allocs := testing.AllocsPerRun(128, func() {
        _, _ = d.Decode(idx, &c)
    })
    if allocs != 0 {
        t.Fatalf("Decode allocated %.2f times per call; want 0", allocs)
    }
}

func TestNoAllocationInReset(t *testing.T) {
    var d Decoder
    allocs := testing.AllocsPerRun(128, func() {
        d.Reset()
    })
    if allocs != 0 {
        t.Fatalf("Reset allocated %.2f times per call; want 0", allocs)
    }
}
```

- [x] **Step 2: Run the allocation tests**

Run: `go test ./internal/gain/ -run TestNoAllocation -v`
Expected: PASS. If failing, `go build -gcflags='-m' ./internal/gain` to locate the escape site. Typical culprits: a `[]int16` slice created inside Decode, or the pointer receiver accidentally escaping.

- [x] **Step 3: Write the benchmark**

Create `internal/gain/bench_test.go`:

```go
package gain

import "testing"

func BenchmarkDecode(b *testing.B) {
    var d Decoder
    var c [40]int16
    c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192 // 4-pulse spec example
    idx := Indices{GA: 3, GB: 7}

    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = d.Decode(idx, &c)
    }
}
```

- [x] **Step 4: Run the benchmark and confirm zero allocs**

Run: `go test -bench=. -benchmem -run=^$ ./internal/gain/`
Expected: `0 B/op   0 allocs/op`.

- [x] **Step 5: Polish the package documentation**

Replace `internal/gain/doc.go` with:

```go
// Package gain implements the G.729 + Annex A decoder's conjugate-
// structure gain VQ: from 7 bits per subframe (GA 3 bits + GB 4
// bits) and the Phase 1c fixed codebook vector c[40], decode the
// pitch gain g_p (Q14) and the fixed codebook gain g_c (Q1)
// consumed by the Phase 1e excitation sum
// u(n) = g_p·v(n) + g_c·c(n).
//
// # Public API
//
//	Indices{GA, GB uint8}
//	    Bit-field indices delivered by the bitstream unpacker
//	    (GA1/GB1 or GA2/GB2 from bitstream.Frame).
//
//	Decoder
//	    Per-instance struct holding the 4-tap MA predictor state
//	    for log-gain correction errors. Zero value is a valid
//	    initial state (pastErrors is populated lazily on the
//	    first Decode call with the spec's −14 dB Q10 default).
//
//	Decoder.Decode(idx, c) → (gpQ14, gcQ1 int16)
//	    Per ITU-T G.729 §3.9 / §4.1.6: compute E_c from c, form
//	    the MA-predicted log gain, look up the two-stage VQ,
//	    assemble g_c = γ̂_c · 10^((Ê − Ē_c)/20), and update the
//	    predictor state with the log of the decoded γ̂_c.
//
//	Decoder.Reset()
//	    Returns the Decoder to its zero-value state.
//
// # State ownership
//
// The MA predictor's tap line (past 4 log-gain correction errors)
// is the only state this package holds. It must NOT be shared
// across independent decoding sessions. Phase 1g will allocate one
// Decoder per active stream.
//
// # Numerical contract
//
//	Indices:       raw integers from the bitstream.
//	c (input):     Q13 int16 (from internal/fcb).
//	g_p (output):  Q14 int16 ∈ [0, 1.2].
//	g_c (output):  Q1 int16 (range bounded by γ̂_c and g_c0).
//	pastErrors:    Q10 dB, initialized to -14 dB (= -14336).
//	Tables:        Q14 GBK1/GBK2, Q13 MAPredictor, Q15 Pow2Table,
//	               Q10 mean energy.
//
// # Scratch-from-spec
//
// Algorithm from ITU-T G.729 §3.9 + §4.1.6 + Annex A §A.3.9
// (decoder tables are unchanged from full G.729). Four numerical
// tables (GBK1, GBK2, MA predictor + mean energy, Pow2) are
// transcribed from tab_ld8a.c data-array initializers under the
// merger-doctrine exception. No algorithmic ITU C source has been
// consulted. Every arithmetic step routes through internal/fixed.
//
// # Concurrency
//
// A Decoder is not safe for concurrent use. Each active stream
// requires its own Decoder instance. Individual methods do not
// spawn goroutines; all work is synchronous on the caller.
package gain
```

- [x] **Step 6: Run the full test + vet pass**

Run in parallel:
- `go test -race ./internal/gain/... ./internal/tables/...` → all PASS
- `go vet ./internal/gain/... ./internal/tables/...` → silent

- [x] **Step 7: Commit**

```bash
git add internal/gain/alloc_test.go internal/gain/bench_test.go internal/gain/doc.go
git commit -m "test(gain): lock zero-alloc + Decode bench; polish doc"
```

---

## Completion criteria

At the end of Task 12:

- All unit tests pass (`go test -race ./internal/gain/... ./internal/tables/...`).
- `BenchmarkDecode` reports `0 B/op, 0 allocs/op`.
- `go vet` is silent.
- 12 commits on `main`, one per task.

Write a completion report at `docs/superpowers/plans/2026-04-21-phase1d-gain-completion-report.md` summarising:

- Which spec sections were referenced for which code.
- **Q-format deviations.** The plan's candidate implementations of `log2Fixed`, `pow2Fixed`, `predictedLogGain`, and especially `Decode`'s dB↔log2 conversion use derivation-guess constants (5443, 6165, 24660, 16402) that the implementing engineer should replace with spec-faithful values. Document the actual constants used and why.
- **Pow2Table layout.** If the spec's table has a different size (65 instead of 33), different Q-format (Q14 instead of Q15), or a different index/interpolation bit split, document the layout used and adjust `log2Fixed` / `pow2Fixed` accordingly. The completion report is where to explain the choice.
- **MA predictor initial value.** The plan uses `-14 dB Q10 = -14336`; §3.9 / §4.1.6 may specify a different default for the first subframe (e.g. `-14` dB vs `-14.5` dB, or a different Q-format).
- **g_c output Q-format.** The plan nominally produces Q1 for compatibility with Phase 1e's planned excitation sum. If the spec's §4.1.6 nominates a different Q-format, document it and the Phase 1e handoff.
- Benchmark numbers for `BenchmarkDecode`.

Additionally, note that Phase 1g will validate bit-exact gain values against ITU test vectors — all tolerance bounds in Phase 1d tests are structural, not bit-exact.

---

## What comes next (not in this plan)

- **Phase 1e** — excitation sum `u(n) = g_p·v(n) + g_c·c(n)` and LP synthesis filter `1/A(z)` per subframe. Consumes `v` from Phase 1b, `c` from Phase 1c, `(g_p, g_c)` from this phase, and the LP coefficients `A(z)` from Phase 1a (LSP→LP conversion).
- **Phase 1f** — Annex A adaptive post-filter (short-term + long-term + tilt compensation + AGC).
- **Phase 1g** — top-level `Decoder`: bitstream → LSP → pitch → fcb → gain → excitation → synth → post, owning the shared past-excitation ring buffer and calling `Phase 1c.ClampPitchGainForEnhancement(g_p_prev)` to supply β to `fcb.Decode`. First ITU test-vector run here.
- **Phase 1h** — erasure concealment. **The Phase 1d MA predictor state is one of the codec's most erasure-sensitive state variables**; Phase 1h will reset or extrapolate it on bad frames.

Until Phase 1g wires all sub-blocks together, Phase 1d's correctness rests on the structural tests here (MA predictor tap line, FIFO shift, Reset determinism, VQ exhaustive combination check, log/pow round-trip) plus allocation + benchmark invariants. Bit-exact gains are measured in Phase 1g.
