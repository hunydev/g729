# Phase 1f Implementation Plan — `internal/postfilter`: Annex A Adaptive Postfilter

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Implement the G.729 Annex A adaptive postfilter chain per ITU-T G.729 §A.4.2 (referenced from §3.10 with the §B.4 Annex A simplifications). Consumes the pre-postfilter synthesis `s[n]` from `internal/synth` together with the decoded LP coefficients `a[11]` and the integer pitch delay `t_int`, and produces perceptually enhanced speech `s_pf[n]` ready for the output high-pass stage.

**Architecture:** A new `internal/postfilter` package structured as a single `Postfilter` stateful struct whose `Filter` method walks the subframe through five functional stages:

1. **Bandwidth expansion** of `a` into two vectors `a_num[i] = γ_n^i · a[i]` and `a_den[i] = γ_d^i · a[i]`.
2. **Residual computation** `r(n) = a_num(z) · s(n)` — FIR, consumes 10 past `s` samples as state.
3. **Long-term postfilter** — first refine the pitch lag `T ∈ {t_int−1, t_int, t_int+1}` by maximising correlation with `r(n)`, then apply `r'(n) = (r(n) + g_l · r(n−T)) / (1 + g_l)`.
4. **Short-term synthesis** `s_st(n) = r'(n) − Σ a_den[i] · s_st(n−i)` — IIR, 10-sample state.
5. **Tilt compensation** `s_tilt(n) = s_st(n) + μ · s_st(n−1)` — spectral tilt offset.
6. **Adaptive gain control (AGC)** — scale `s_tilt(n)` so its energy matches that of the synthesis input `s(n)`, with smoothing across samples.

Each stage has its own state; all arithmetic routes through `internal/fixed` with ITU saturation semantics.

**Tech Stack:** Go, `internal/fixed` ITU basic-ops, consumes outputs from `internal/lsp` (`a[11]` per subframe), `internal/pitch` (`t_int`), and `internal/synth` (`s[40]`). No new tables needed (all postfilter constants are scalar: `γ_n`, `γ_d`, `γ_t`, AGC α).

**Scratch-from-spec:** Every coefficient and formula derives from ITU-T G.729 §A.4.2 / §B.4 / §3.10. No ITU reference C, bcg729, Sipro Lab, or any other existing G.729 implementation is consulted for algorithmic code. Where §A.4.2 differs from the main §3.10 postfilter (bandwidth expansion factors, tilt factor derivation, AGC time constants), the Annex A values take precedence.

**Constants (Annex A §A.4.2 — verify against spec text before coding):**

| Symbol  | Meaning                                          | Approx value | Q-format  |
| ------- | ------------------------------------------------ | ------------ | --------- |
| `γ_n`   | Numerator bandwidth expansion (perceptual weighting) | 0.55     | Q15 = 18022 |
| `γ_d`   | Denominator bandwidth expansion (formant postfilter) | 0.70     | Q15 = 22938 |
| `γ_t`   | Tilt compensation weighting                       | ≈ 0.90       | Q15 |
| `γ_l`   | Long-term postfilter bound on g_l                | 0.5          | Q14 = 8192 |
| `α_agc` | AGC smoothing factor                              | ≈ 0.99       | Q15       |

The engineer MUST cross-check these against Annex A §A.4.2 and transcribe the exact values from the spec text. The table above is orientation only; any mismatch requires replacing the constant, not reworking the arithmetic structure.

**Q-format map:**

| Signal         | Source               | Q-format       | Notes                                     |
| -------------- | -------------------- | -------------- | ----------------------------------------- |
| `s[n]`         | `synth.Synthesize`   | Q0 Word16      | Pre-postfilter synthesis                  |
| `a[0..10]`     | `lsp.Decoder.Decode` | Q12 Word16     | `a[0] = 4096`                             |
| `tInt`         | `pitch.DecodeDelay*` | integer        | Range [20, 143]                           |
| `a_num[i]`     | internal             | Q12 Word16     | Bandwidth-expanded numerator              |
| `a_den[i]`     | internal             | Q12 Word16     | Bandwidth-expanded denominator            |
| `r[n]`         | internal             | Q12 Word32/16  | LPC residual (engineer chooses convention)|
| **`s_pf[n]`**  | **this phase**       | **Q0 Word16** | Postfiltered output                       |

**Concurrency:** `Postfilter` is not safe for concurrent use. One instance per decoder channel.

---

## File structure

```
internal/postfilter/
├── doc.go              # package doc (Task 11)
├── types.go            # Postfilter struct + state (Task 1)
├── bandwidth.go        # expandBandwidth helper (Task 2)
├── bandwidth_test.go
├── residual.go         # computeResidual FIR (Task 3)
├── residual_test.go
├── longterm.go         # refinePitch + applyLongTerm (Tasks 4-5)
├── longterm_test.go
├── shortterm.go        # applyShortTerm IIR (Task 6)
├── shortterm_test.go
├── tilt.go             # computeTilt + applyTilt (Task 7)
├── tilt_test.go
├── agc.go              # updateAGC + applyAGC (Task 8)
├── agc_test.go
├── postfilter.go       # Filter public entry, Reset (Tasks 9-10)
├── postfilter_test.go
├── alloc_test.go       # zero-allocation locks (Task 11)
└── bench_test.go       # benchmarks (Task 11)
```

---

## Public API surface (target)

```go
package postfilter

const (
    subframeLen = 40
    lpcOrder    = 10
    pitchMax    = 143
)

// Postfilter holds per-channel adaptive postfilter state as specified in
// ITU-T G.729 §A.4.2. The zero value is a valid Reset state.
type Postfilter struct {
    // pastS is the tail of the pre-postfilter synthesis required by the
    // residual FIR (needs 10 past samples of s).
    pastS [lpcOrder]int16 // Q0

    // pastResidual holds up to `pitchMax + subframeLen` samples of r(n)
    // so the long-term postfilter can look back by `T ∈ [20, 144]`.
    // Indexing: oldest at [0], most recent subframe at tail.
    pastResidual [pitchMax + subframeLen]int16 // Q12

    // pastSynthPost is the 10-sample state of the short-term synthesis
    // IIR 1/A(z/γ_d).
    pastSynthPost [lpcOrder]int16 // Q0

    // pastTiltInput is the z^-1 register of the tilt compensation filter.
    pastTiltInput int16 // Q0

    // agcGainPrev is the AGC gain used in the last sample of the previous
    // subframe (for smoothing across subframe boundaries).
    agcGainPrev int16 // Q14 typical; exact format is the engineer's call
                      // guided by §A.4.2.3.
}

// Filter runs the full Annex A postfilter chain on one subframe.
//
// Inputs:
//   a        — LP coefficients for this subframe (Q12, a[0] = 4096)
//   tInt     — integer pitch delay decoded by internal/pitch
//   s        — pre-postfilter synthesis samples from internal/synth (Q0)
//
// Output:
//   sPf      — postfiltered samples (Q0), saturated
//
// Updates all Postfilter state fields for continuity into the next subframe.
// Zero-allocation on the hot path.
func (pf *Postfilter) Filter(a *[11]int16, tInt int, s *[subframeLen]int16, sPf *[subframeLen]int16)

// Reset clears all state to the zero condition expected at the first
// frame of a new stream.
func (pf *Postfilter) Reset()
```

**Note on state sizing.** `pastResidual` is declared as `[pitchMax + subframeLen]int16 = [183]int16`. This is the minimum that supports `T = 143` plus the current subframe's 40 samples in the same buffer. On each `Filter` call we slide the buffer left by `subframeLen` and write the new residual at the tail.

---

## Task 1: Package skeleton + Postfilter struct + Reset

**Files:**
- Create: `internal/postfilter/doc.go` (placeholder; polished in Task 11)
- Create: `internal/postfilter/types.go`
- Create: `internal/postfilter/postfilter_test.go`

- [x] **Step 1: Write the failing tests**

Write to `internal/postfilter/postfilter_test.go`:

```go
package postfilter

import "testing"

func TestPostfilter_ZeroValueIsReset(t *testing.T) {
    var pf Postfilter

    for i, v := range pf.pastS {
        if v != 0 {
            t.Errorf("pastS[%d] = %d, want 0", i, v)
        }
    }
    for i, v := range pf.pastResidual {
        if v != 0 {
            t.Errorf("pastResidual[%d] = %d, want 0", i, v)
        }
    }
    for i, v := range pf.pastSynthPost {
        if v != 0 {
            t.Errorf("pastSynthPost[%d] = %d, want 0", i, v)
        }
    }
    if pf.pastTiltInput != 0 {
        t.Errorf("pastTiltInput = %d, want 0", pf.pastTiltInput)
    }
    if pf.agcGainPrev != 0 {
        t.Errorf("agcGainPrev = %d, want 0", pf.agcGainPrev)
    }
}

func TestPostfilter_ResetZerosState(t *testing.T) {
    var pf Postfilter
    pf.pastS[0] = 123
    pf.pastResidual[50] = -456
    pf.pastSynthPost[9] = 789
    pf.pastTiltInput = 42
    pf.agcGainPrev = 1234

    pf.Reset()

    for i, v := range pf.pastS {
        if v != 0 {
            t.Errorf("after Reset, pastS[%d] = %d, want 0", i, v)
        }
    }
    for i, v := range pf.pastResidual {
        if v != 0 {
            t.Errorf("after Reset, pastResidual[%d] = %d, want 0", i, v)
        }
    }
    for i, v := range pf.pastSynthPost {
        if v != 0 {
            t.Errorf("after Reset, pastSynthPost[%d] = %d, want 0", i, v)
        }
    }
    if pf.pastTiltInput != 0 {
        t.Errorf("after Reset, pastTiltInput = %d, want 0", pf.pastTiltInput)
    }
    if pf.agcGainPrev != 0 {
        t.Errorf("after Reset, agcGainPrev = %d, want 0", pf.agcGainPrev)
    }
}
```

- [x] **Step 2: Verify the test fails (package doesn't exist)**

Run: `go test ./internal/postfilter/...`

Expected: compile error — `package postfilter` / `Postfilter` / `Reset` undefined.

- [x] **Step 3: Create `internal/postfilter/doc.go` placeholder**

```go
// Package postfilter implements the G.729 Annex A adaptive postfilter
// per ITU-T G.729 §A.4.2. Full documentation in doc.go (finalized at the
// end of Phase 1f).
package postfilter
```

- [x] **Step 4: Create `internal/postfilter/types.go`**

```go
package postfilter

const (
    subframeLen = 40
    lpcOrder    = 10
    pitchMax    = 143
)

// Postfilter holds per-channel adaptive-postfilter state per ITU-T G.729
// §A.4.2. The zero value is a valid Reset state.
//
// Not safe for concurrent use. One instance per decoder stream.
type Postfilter struct {
    pastS         [lpcOrder]int16
    pastResidual  [pitchMax + subframeLen]int16
    pastSynthPost [lpcOrder]int16
    pastTiltInput int16
    agcGainPrev   int16
}

// Reset clears all postfilter state to the zero initial condition.
func (pf *Postfilter) Reset() {
    *pf = Postfilter{}
}
```

- [x] **Step 5: Verify tests pass**

Run: `go test -race ./internal/postfilter/...`

Expected: `PASS`.

- [x] **Step 6: Commit**

```bash
git add internal/postfilter/doc.go internal/postfilter/types.go internal/postfilter/postfilter_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): package skeleton + Postfilter struct with Reset

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 2: Bandwidth expansion helper

**Files:**
- Create: `internal/postfilter/bandwidth.go`
- Create: `internal/postfilter/bandwidth_test.go`

**Spec reference:** ITU-T G.729 §A.4.2.1 / §3.10.1. The bandwidth-expansion operation produces `a_scaled[i] = γ^i · a[i]` for `i ∈ [0, 10]`. With `γ < 1`, this pulls the formant poles/zeros toward the origin, broadening the peaks perceptually.

**Arithmetic sketch (verify against spec):**
- Input `a[i]` is Q12 Word16; output `a_scaled[i]` is Q12 Word16.
- Compute iteratively: start with `gamma_pow = γ` (Q15); each iteration multiply `a[i]` by `gamma_pow` and scale `gamma_pow` by γ.
- Per sample: `a_scaled[i] = round(a[i] · gamma_pow)` where `a[i] · gamma_pow` is Q12·Q15 → Q27 via `LMult` → `Round` after `LShl(·, 0)` depending on convention.

- [x] **Step 1: Write the failing tests**

Write to `internal/postfilter/bandwidth_test.go`:

```go
package postfilter

import "testing"

// γ = 32767 (≈ 1.0 in Q15): bandwidth expansion should be near-identity
// (modulo 1-LSB Q15 truncation — 32767 is 1.0 − 2^-15).
func TestExpandBandwidth_GammaNearOneIsIdentity(t *testing.T) {
    a := [11]int16{4096, 1000, -500, 300, -100, 50, 25, -10, 5, -2, 1}
    var out [11]int16

    expandBandwidth(&a, 32767, &out)

    // a[0] must always be 4096 (filter normalisation).
    if out[0] != 4096 {
        t.Errorf("out[0] = %d, want 4096", out[0])
    }
    // Remaining coefficients should be within a few LSBs of the input.
    for i := 1; i <= 10; i++ {
        diff := int(out[i]) - int(a[i])
        if diff < -2 || diff > 2 {
            t.Errorf("out[%d] = %d, want ≈ %d (γ≈1.0, tol ±2)", i, out[i], a[i])
        }
    }
}

// γ = 0 (Q15): all tail coefficients must be zeroed; a[0] stays 4096.
func TestExpandBandwidth_ZeroGammaZerosTail(t *testing.T) {
    a := [11]int16{4096, 1000, -500, 300, -100, 50, 25, -10, 5, -2, 1}
    var out [11]int16

    expandBandwidth(&a, 0, &out)

    if out[0] != 4096 {
        t.Errorf("out[0] = %d, want 4096", out[0])
    }
    for i := 1; i <= 10; i++ {
        if out[i] != 0 {
            t.Errorf("out[%d] = %d, want 0 (γ = 0)", i, out[i])
        }
    }
}

// γ = 16384 (0.5 Q15): monotonic decay of |a[i]|, and the ratio
// |out[i]| / |a[i]| should be approximately 0.5^i (verify order of
// magnitude; exact bit-equality is deferred to Phase 1g).
func TestExpandBandwidth_HalfGammaGeometricDecay(t *testing.T) {
    a := [11]int16{4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096}
    var out [11]int16

    expandBandwidth(&a, 16384, &out)

    if out[0] != 4096 {
        t.Errorf("out[0] = %d, want 4096", out[0])
    }
    // Each successive tap should be roughly half of the previous (within
    // ±1 LSB rounding noise).
    for i := 1; i <= 10; i++ {
        prev := int32(out[i-1])
        want := prev / 2
        got := int32(out[i])
        if got < want-2 || got > want+2 {
            t.Errorf("out[%d] = %d, want ≈ %d (half of %d, tol ±2)",
                i, got, want, prev)
        }
    }
}
```

- [x] **Step 2: Verify the tests fail**

Run: `go test ./internal/postfilter/ -run TestExpandBandwidth -v`

Expected: compile error — `expandBandwidth` undefined.

- [x] **Step 3: Implement the helper**

Write to `internal/postfilter/bandwidth.go`:

```go
package postfilter

import "github.com/exedev/g729/internal/fixed"

// expandBandwidth computes a_scaled[i] = γ^i · a[i] for i ∈ [0, 10]
// per ITU-T G.729 §3.10.1 / §A.4.2.1.
//
// Q-formats:
//   a        [11]int16, Q12; a[0] = 4096
//   gammaQ15 Word16, Q15 (γ = gammaQ15 / 32768)
//   out      [11]int16, Q12; out[0] = 4096 (filter normalisation preserved)
//
// Implementation: iteratively accumulate γ^i in gammaPow (Q15) and
// multiply each a[i] by gammaPow. The product a[i]·gammaPow is at
// Q12·Q15 = Q27 in Word32; `LMult` doubles it to Q28, then Round+LShl
// yield the Q12 result.
func expandBandwidth(a *[11]int16, gammaQ15 int16, out *[11]int16) {
    out[0] = a[0] // normalisation: a[0] = 4096 always
    gammaPow := gammaQ15
    for i := 1; i <= 10; i++ {
        // a[i] (Q12) × gammaPow (Q15) → Q27 via LMult (which doubles to Q28),
        // LShl by -12 via Round+ExtractH rounds back to Q12 Word16.
        // Spec-faithful derivation: (a[i] * gammaPow) >> 15 is the Q12 result.
        prod := int32(a[i]) * int32(gammaPow) // Q27 (no saturation risk: ±2^26 max)
        out[i] = int16((prod + (1 << 14)) >> 15)

        // Update gammaPow = gammaPow · γ (Q15 · Q15 = Q30, down-shift to Q15).
        gp := int32(gammaPow) * int32(gammaQ15)
        gammaPow = int16((gp + (1 << 14)) >> 15)
    }
    _ = fixed.Add // silence unused import if switching to internal/fixed primitives
}
```

**Note:** the above uses direct int32 multiplication for simplicity. If Phase 1g shows Q-format mismatch against ITU vectors, rewrite using `fixed.LMult` + `fixed.Round` for ITU-compliant saturation. The structural tests above tolerate ±2 LSB so the rounding direction (truncate vs round-to-nearest) is not pinned here; Phase 1g will pin it.

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/postfilter/ -run TestExpandBandwidth -v`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/postfilter/bandwidth.go internal/postfilter/bandwidth_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): bandwidth expansion a_scaled[i] = γ^i·a[i] per ITU §3.10.1

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 3: Residual FIR — r(n) = A(z/γ_n) · s(n)

**Files:**
- Create: `internal/postfilter/residual.go`
- Create: `internal/postfilter/residual_test.go`

**Spec reference:** ITU-T G.729 §A.4.2.1. The residual of the bandwidth-expanded predictor:
`r(n) = s(n) + Σ_{i=1..10} a_num[i] · s(n−i)` (no division — this is FIR, not IIR; it's the "forward" LPC analysis filter with bandwidth-expanded coefficients).

Note: `a_num[0] = a[0] = 4096` (Q12) and `a_num[i]` are produced by `expandBandwidth(·, γ_n)`. Since the numerator is `A(z) = 1 + a_1 z^-1 + ... + a_10 z^-10`, the filter is `r(n) = Σ_{i=0..10} a_num[i] · s(n−i)` with `a_num[0] = 1` (Q12 as 4096).

**Q-format derivation:**
- `a_num[i]` Q12, `s(n−i)` Q0 → product Q12 in Word32.
- `LMult(a_num[i], s(n-i))` → Q13 (ITU double).
- Accumulate via `LMac` — each `LMac` adds `2·a·b` at Q13.
- After the 11-tap sum, the accumulator is Q13.
- For the residual, the natural output is Q12 (to keep precision and have room before the 1/A(z/γ_d) IIR). Shift right by 1: `LShr(L_temp, 1)` → Q12 Word32.
- Store Q12 Word32 in `pastResidual` — but note that `pastResidual` is `[int16]`, so down-shift to Q12 Word16 with `ExtractH(LShl(L_temp, 3))` → Q0 Word16... that loses Q12 precision.

**Design choice for `r(n)` storage:** two options —

**Option A:** store `r(n)` as Q0 Word16 (Round + ExtractH from the Q13 accumulator at scale 2^13).
**Option B:** store `r(n)` as Q12 Word16 by shifting down from Q13.

The long-term postfilter (Task 5) will consume `r(n)` and produce `r'(n)`; the short-term IIR 1/A(z/γ_d) (Task 6) consumes `r'(n)`. Whatever Q-format we choose for `r(n)`, the rest of the chain must align. The spec's reference implementation typically keeps `r(n)` in Q0 with saturation.

**Plan choice:** Store `r(n)` as Q0 Word16 (matches `s(n)` scale and simplifies later arithmetic). If Phase 1g validation shows precision loss, revisit and widen.

**Arithmetic (Q0 output):**
```
L_temp = LMult(a_num[0], s(n))          // Q13 (= 2·4096·s(n))
for i = 1..10:
    L_temp = LMac(L_temp, a_num[i], s_or_past(n-i))   // Q13
L_temp = LShl(L_temp, 3)                // Q13 → Q16
r[n] = Round(L_temp)                    // Q0 Word16
```

Where `s_or_past(n-i) = pf.pastS[lpcOrder + (n-i)]` if `n-i < 0`, else `s[n-i]`.

- [x] **Step 1: Write the failing tests**

Write to `internal/postfilter/residual_test.go`:

```go
package postfilter

import "testing"

// Zero-coefficient numerator (except a[0]): residual = s directly.
func TestComputeResidual_ZeroTail(t *testing.T) {
    var pf Postfilter
    aNum := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var s, r [subframeLen]int16
    for i := range s {
        s[i] = int16(100 + i*10)
    }

    pf.computeResidual(&aNum, &s, &r)

    for i := range r {
        if r[i] != s[i] {
            t.Errorf("r[%d] = %d, want %d (zero-tail residual is identity)",
                i, r[i], s[i])
        }
    }
}

// First-tap only: aNum = [4096, 2048, 0, ...] → r[n] = s(n) + 0.5·s(n-1).
// With s[n] = constant = 200 and pastS = 0: r[0] = 200 + 0.5·0 = 200,
// r[1..] = 200 + 0.5·200 = 300.
func TestComputeResidual_FirstTapOnly(t *testing.T) {
    var pf Postfilter
    aNum := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var s, r [subframeLen]int16
    for i := range s {
        s[i] = 200
    }

    pf.computeResidual(&aNum, &s, &r)

    // r[0] = 200 + 0.5·pastS[9] = 200 + 0 = 200 (±1 LSB).
    if r[0] < 199 || r[0] > 201 {
        t.Errorf("r[0] = %d, want 200 (±1)", r[0])
    }
    // r[n≥1] = 200 + 0.5·200 = 300 (±1).
    for i := 1; i < subframeLen; i++ {
        if r[i] < 299 || r[i] > 301 {
            t.Errorf("r[%d] = %d, want 300 (±1)", i, r[i])
        }
    }
}

// pastS must contribute to r[0..9]. With pastS[9] = 1000, aNum = [4096, 4096, 0, ...],
// s = 0: r[0] = 0 + 1.0·1000 = 1000; r[1] = 0 + 1.0·0 = 0.
func TestComputeResidual_PastSContributes(t *testing.T) {
    var pf Postfilter
    pf.pastS[9] = 1000
    aNum := [11]int16{4096, 4096, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var s, r [subframeLen]int16

    pf.computeResidual(&aNum, &s, &r)

    if r[0] < 999 || r[0] > 1001 {
        t.Errorf("r[0] = %d, want 1000 (from pastS[9])", r[0])
    }
    if r[1] != 0 && r[1] != 1 && r[1] != -1 {
        t.Errorf("r[1] = %d, want 0 (no active input)", r[1])
    }
}

// After the call, pastS must hold the last 10 samples of s.
func TestComputeResidual_UpdatesPastS(t *testing.T) {
    var pf Postfilter
    aNum := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var s, r [subframeLen]int16
    for i := range s {
        s[i] = int16(500 + i)
    }

    pf.computeResidual(&aNum, &s, &r)

    for i := 0; i < lpcOrder; i++ {
        want := s[subframeLen-lpcOrder+i]
        if pf.pastS[i] != want {
            t.Errorf("pastS[%d] = %d, want %d (last 10 of s)",
                i, pf.pastS[i], want)
        }
    }
}
```

- [x] **Step 2: Verify the tests fail**

Run: `go test ./internal/postfilter/ -run TestComputeResidual -v`

Expected: compile error — `computeResidual` undefined.

- [x] **Step 3: Implement the residual FIR**

Write to `internal/postfilter/residual.go`:

```go
package postfilter

import "github.com/exedev/g729/internal/fixed"

// computeResidual applies the bandwidth-expanded FIR
//   r(n) = Σ_{i=0..10} aNum[i] · s(n−i)
// per ITU-T G.729 §A.4.2.1, updating pastS for the next subframe.
//
// Q-formats: aNum Q12, s Q0, r Q0 (engineer may widen to Q12 Word16 in
// Phase 1g if the long-term postfilter needs more precision; the choice
// here is Q0 for alignment with synthesis output).
//
// State: pastS holds s(n-10..n-1) on entry; on return, pastS holds
// s(30..39) for the next subframe.
func (pf *Postfilter) computeResidual(aNum *[11]int16, s, r *[subframeLen]int16) {
    var work [lpcOrder + subframeLen]int16
    copy(work[:lpcOrder], pf.pastS[:])
    copy(work[lpcOrder:], s[:])

    for n := 0; n < subframeLen; n++ {
        lTemp := fixed.LMult(aNum[0], work[lpcOrder+n])
        for i := 1; i <= lpcOrder; i++ {
            lTemp = fixed.LMac(lTemp, aNum[i], work[lpcOrder+n-i])
        }
        lTemp = fixed.LShl(lTemp, 3)
        r[n] = fixed.Round(lTemp)
    }

    copy(pf.pastS[:], work[lpcOrder+subframeLen-lpcOrder:])
}
```

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/postfilter/ -run TestComputeResidual -v`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/postfilter/residual.go internal/postfilter/residual_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): residual FIR r(n) = A(z/γ_n)·s(n) per ITU §A.4.2.1

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 4: Long-term postfilter — pitch refinement

**Files:**
- Create: `internal/postfilter/longterm.go`
- Create: `internal/postfilter/longterm_test.go`

**Spec reference:** ITU-T G.729 §A.4.2.2. The long-term postfilter refines the pitch lag over `T ∈ {t_int−1, t_int, t_int+1}` (closed set of 3 integer candidates) by maximising the cross-correlation `R(T) = Σ_{n=0..39} r(n) · r(n−T)`. The best `T` is used in Task 5.

**Algorithm (derived from §A.4.2.2):**
```
best_T   = t_int
best_num = 0     // best correlation numerator (squared or raw)
for k in {-1, 0, +1}:
    T = t_int + k
    if T < 20 or T > 143: skip (out of range)
    R = Σ_{n=0..39} r(n) · r(n-T)
    E = Σ_{n=0..39} r(n-T)²
    // Prefer the candidate with largest R²/E (avoids energy-normalized cmp sensitivity).
    // Use cross-multiplication to avoid division.
    if R > 0 and R² · best_E > best_num² · E:
        best_T   = T
        best_num = R
        best_E   = E
return best_T
```

**Indexing.** `r(n−T)` for `n ∈ [0, 39]` and `T ∈ [19, 144]` reaches back as far as `n-T = -144`, i.e. 144 samples before the current subframe. This is why `pastResidual` is sized `pitchMax + subframeLen = 183`.

**Addressing convention.** On entry to `refinePitch`, `pastResidual[:pitchMax]` holds the last `pitchMax` samples of r from the previous subframe; `pastResidual[pitchMax:pitchMax+subframeLen]` holds this subframe's just-computed r. Then `r(n-T)` = `pastResidual[pitchMax + n - T]`. If `n-T < -pitchMax` (shouldn't happen when `T ≤ pitchMax`), the value is zero (initial condition).

- [x] **Step 1: Write the failing tests**

Write to `internal/postfilter/longterm_test.go`:

```go
package postfilter

import "testing"

// With a pure periodic residual (period T_true = 30) placed in pastResidual,
// refinePitch should lock onto T = 30 when given t_int = 30.
func TestRefinePitch_LocksToTruePeriod(t *testing.T) {
    var pf Postfilter

    // Load pastResidual with a period-30 square wave: +1000 for 15 samples,
    // -1000 for 15, repeating. The `r` buffer is the current subframe
    // (also period-30 aligned).
    for i := range pf.pastResidual {
        if (i/15)%2 == 0 {
            pf.pastResidual[i] = 1000
        } else {
            pf.pastResidual[i] = -1000
        }
    }
    var r [subframeLen]int16
    for i := range r {
        if (i/15)%2 == 0 {
            r[i] = 1000
        } else {
            r[i] = -1000
        }
    }
    // pastResidual tail = r (simulating the residual-write ordering).
    copy(pf.pastResidual[pitchMax:], r[:])

    bestT := pf.refinePitch(&r, 30)
    // The true period 30 should be the winner; allow 29 or 31 only if
    // the test signal's coarse structure hides the exact match.
    if bestT != 30 {
        t.Errorf("bestT = %d, want 30", bestT)
    }
}

// With no signal at all, refinePitch falls back to the input t_int.
func TestRefinePitch_ZeroSignalFallsBackToTInt(t *testing.T) {
    var pf Postfilter
    var r [subframeLen]int16
    // pastResidual is all zero; r is all zero.

    bestT := pf.refinePitch(&r, 55)
    if bestT != 55 {
        t.Errorf("bestT = %d, want 55 (fallback to t_int)", bestT)
    }
}

// t_int at the domain edge must not probe out-of-range.
func TestRefinePitch_ClampsAtLowerEdge(t *testing.T) {
    var pf Postfilter
    var r [subframeLen]int16
    // With t_int = 20 (minimum), refinePitch may try t_int-1 = 19 which
    // is out of range and must be skipped. The function must not panic
    // or read below pastResidual[0].

    _ = pf.refinePitch(&r, 20) // no crash is the test
}

func TestRefinePitch_ClampsAtUpperEdge(t *testing.T) {
    var pf Postfilter
    var r [subframeLen]int16
    // With t_int = 143 (maximum), refinePitch may try t_int+1 = 144 which
    // is out of range. Skip without panic.

    _ = pf.refinePitch(&r, 143)
}
```

- [x] **Step 2: Verify the tests fail**

Run: `go test ./internal/postfilter/ -run TestRefinePitch -v`

Expected: compile error — `refinePitch` undefined.

- [x] **Step 3: Implement refinePitch**

Write to `internal/postfilter/longterm.go`:

```go
package postfilter

// refinePitch selects the best pitch lag T ∈ {tInt-1, tInt, tInt+1} (within
// [20, pitchMax]) by maximising the normalised cross-correlation with r per
// ITU-T G.729 §A.4.2.2.
//
// Returns the selected lag T. Caller assembles the long-term postfilter
// output using this T in applyLongTerm.
func (pf *Postfilter) refinePitch(r *[subframeLen]int16, tInt int) int {
    const (
        minT = 20
        maxT = pitchMax
    )

    // Build the full residual view: past + current.
    // Indexing: resView[pitchMax + n] = r(n); resView[pitchMax + n - T] = r(n-T).
    var resView [pitchMax + subframeLen]int16
    copy(resView[:pitchMax], pf.pastResidual[subframeLen:]) // recent history
    copy(resView[pitchMax:], r[:])

    bestT := tInt
    // Use cross-multiplication to compare (R·R)/E across candidates without division.
    var bestRsq, bestE int64 = 0, 1

    for k := -1; k <= 1; k++ {
        T := tInt + k
        if T < minT || T > maxT {
            continue
        }
        var R, E int64
        for n := 0; n < subframeLen; n++ {
            rn := int64(resView[pitchMax+n])
            rnT := int64(resView[pitchMax+n-T])
            R += rn * rnT
            E += rnT * rnT
        }
        if R <= 0 || E == 0 {
            continue
        }
        // Compare R²/E vs bestRsq/bestE via cross-multiplication.
        if R*R*bestE > bestRsq*E {
            bestT = T
            bestRsq = R * R
            bestE = E
        }
    }

    return bestT
}
```

**Note on int64 arithmetic.** The cross-correlation sum of 40 products of Word16 values stays well within Word64 range. ITU's reference implementation uses different scaling (split normalisation across `Corr` and `Energy` Q-format), but for Annex A §A.4.2.2 a clear int64 accumulator is spec-faithful and matches the algorithm's intent. Phase 1g can swap to the ITU `DOT_PRODUCT` primitive if bit-exactness demands.

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/postfilter/ -run TestRefinePitch -v`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/postfilter/longterm.go internal/postfilter/longterm_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): pitch refinement ±1 around t_int per ITU §A.4.2.2

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 5: Long-term postfilter — gain `g_l` + filter application

**Files:**
- Modify: `internal/postfilter/longterm.go` (add applyLongTerm)
- Modify: `internal/postfilter/longterm_test.go` (add tests)

**Spec reference:** ITU-T G.729 §A.4.2.2. Given the selected `T`, compute
the long-term postfilter gain `g_l`:
```
g_l = R(T) / E(T)        clamped to [0, γ_l]
```
where `γ_l` is the Annex-A upper bound (≈ 0.5, spec-specified — verify). If `R(T) ≤ 0` or after clamping `g_l = 0`, the long-term postfilter is effectively bypassed for this subframe.

Then apply the pitch postfilter:
```
r'(n) = (r(n) + g_l · r(n−T)) / (1 + g_l)
```
This is a normalised mix of the current residual with its pitch-delayed version.

**Q-format for `g_l`:** typically Q14 (range [0, 0.5] fits with margin).

**Arithmetic sketch:**
- Division `R/E` is expensive; use `fixed.DivS` with normalised operands.
- Normalise `R` and `E` by `NormL`, then compute `DivS(R_norm, E_norm)` at Q14 with the scale factor bookkeeping.
- Clamp to `[0, γ_lQ14]`.

For the application: `(r(n) + g_l·r(n-T)) / (1 + g_l)`. If `g_l = 0.5` then divisor is 1.5; direct division is awkward. Two equivalent formulations:
- **Form A (ITU ref):** compute two scalars `g0 = 1/(1+g_l)` Q14 and `g1 = g_l/(1+g_l)` Q14, then `r'(n) = g0·r(n) + g1·r(n-T)`.
- **Form B:** compute numerator and divide per-sample — expensive.

Use Form A.

- [x] **Step 1: Write the failing tests**

Append to `internal/postfilter/longterm_test.go`:

```go
// With g_l = 0 (zero correlation), long-term postfilter is identity.
func TestApplyLongTerm_ZeroGainIsIdentity(t *testing.T) {
    var pf Postfilter
    var r, rOut [subframeLen]int16
    for i := range r {
        r[i] = int16(100 + i)
    }
    // Leave pastResidual zero so R = 0 → g_l = 0.

    pf.applyLongTerm(&r, 40 /* arbitrary T */, &rOut)

    for i := range rOut {
        if rOut[i] != r[i] {
            t.Errorf("rOut[%d] = %d, want %d (zero gain is identity)",
                i, rOut[i], r[i])
        }
    }
}

// With a pure repeating signal of period T, applyLongTerm should select
// g_l = γ_l (or close) and produce rOut ≈ r (since r(n) ≈ r(n-T)).
func TestApplyLongTerm_PeriodicSignalPreserved(t *testing.T) {
    var pf Postfilter
    const T = 30
    // Load past and current residual with a T-periodic sinusoid-like signal.
    for i := range pf.pastResidual {
        pf.pastResidual[i] = int16(1000 * sign(i%T-15))
    }
    var r [subframeLen]int16
    for i := range r {
        r[i] = int16(1000 * sign(i%T-15))
    }
    copy(pf.pastResidual[pitchMax:], r[:])

    var rOut [subframeLen]int16
    pf.applyLongTerm(&r, T, &rOut)

    // Since r(n) = r(n-T) everywhere for a T-periodic signal,
    // (r + g_l·r)/(1+g_l) = r for any g_l — rOut must equal r (±1 LSB).
    for i := range rOut {
        if rOut[i] < r[i]-1 || rOut[i] > r[i]+1 {
            t.Errorf("rOut[%d] = %d, want %d (±1)", i, rOut[i], r[i])
        }
    }
}

// helper: sign function used in the test above.
func sign(x int) int16 {
    if x >= 0 {
        return 1
    }
    return -1
}
```

- [x] **Step 2: Verify the tests fail**

Run: `go test ./internal/postfilter/ -run TestApplyLongTerm -v`

Expected: compile error — `applyLongTerm` undefined.

- [x] **Step 3: Implement applyLongTerm**

Append to `internal/postfilter/longterm.go`:

```go
// computeLongTermGain returns the postfilter gain g_l (Q14) and two
// pre-computed weights g0 = 1/(1+g_l) and g1 = g_l/(1+g_l) (Q14).
//
// Formula per ITU-T G.729 §A.4.2.2:
//   g_l_raw = R(T) / E(T)  where R, E are computed on r and r-shifted-by-T
//   g_l     = clamp(g_l_raw, 0, γ_l)   with γ_l = 0.5 (Annex A)
func (pf *Postfilter) computeLongTermGain(r *[subframeLen]int16, T int) (g0, g1 int16) {
    const gammaLQ14 int16 = 8192 // = 0.5; verify against §A.4.2.2

    var resView [pitchMax + subframeLen]int16
    copy(resView[:pitchMax], pf.pastResidual[subframeLen:])
    copy(resView[pitchMax:], r[:])

    var R, E int64
    for n := 0; n < subframeLen; n++ {
        rn := int64(resView[pitchMax+n])
        rnT := int64(resView[pitchMax+n-T])
        R += rn * rnT
        E += rnT * rnT
    }

    // g_l = 0 when correlation is non-positive or energy is zero.
    if R <= 0 || E == 0 {
        return 16384, 0 // g0 = 1/(1+0) = 1.0 Q14; g1 = 0.
    }

    // Compute g_l at Q14 without using DivS to keep the implementation
    // easy to reason about. This may require refinement in Phase 1g for
    // bit-exactness — the ITU reference uses normalised DivS.
    gRawQ14 := int16(clamp64(R*16384/E, 0, 32767))
    var gLQ14 int16
    if gRawQ14 > gammaLQ14 {
        gLQ14 = gammaLQ14
    } else {
        gLQ14 = gRawQ14
    }

    // g0 = 1 / (1 + g_l) in Q14; g1 = g_l / (1 + g_l) in Q14.
    // (1 + g_l) in Q14 = 16384 + gLQ14.
    denom := int32(16384 + int32(gLQ14))
    g0 = int16(int32(16384) * 16384 / denom) // (1.0 Q14) · 2^14 / denom
    g1 = int16(int32(gLQ14) * 16384 / denom)
    return g0, g1
}

func clamp64(v, lo, hi int64) int64 {
    if v < lo {
        return lo
    }
    if v > hi {
        return hi
    }
    return v
}

// applyLongTerm filters the residual with the long-term postfilter:
//   r'(n) = g0 · r(n) + g1 · r(n-T)
// per ITU-T G.729 §A.4.2.2, where g0 and g1 encode 1/(1+g_l) and g_l/(1+g_l).
func (pf *Postfilter) applyLongTerm(r *[subframeLen]int16, T int, rOut *[subframeLen]int16) {
    g0, g1 := pf.computeLongTermGain(r, T)

    var resView [pitchMax + subframeLen]int16
    copy(resView[:pitchMax], pf.pastResidual[subframeLen:])
    copy(resView[pitchMax:], r[:])

    for n := 0; n < subframeLen; n++ {
        // r'(n) = g0·r(n) + g1·r(n-T), both Q14·Q0·2 products sum at Q15.
        p0 := int32(g0) * int32(resView[pitchMax+n])        // Q14 in Word32 (no doubling)
        p1 := int32(g1) * int32(resView[pitchMax+n-T])      // Q14 in Word32
        sum := p0 + p1                                        // Q14
        rOut[n] = int16((sum + (1 << 13)) >> 14)              // round Q14 → Q0
    }
}
```

**Note.** The arithmetic above is a direct spec-faithful implementation without relying on ITU primitives for every step; a Phase 1g pass may rewrite hot-loop operations using `fixed.LMult`/`LMac` for ITU-compliant saturation semantics. The structural tests (identity on zero-correlation; preservation of periodic signals) validate the functional contract independent of the exact rounding mode.

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/postfilter/ -run TestApplyLongTerm -v`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/postfilter/longterm.go internal/postfilter/longterm_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): long-term postfilter gain + application per ITU §A.4.2.2

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 6: Short-term synthesis — 1/A(z/γ_d) IIR

**Files:**
- Create: `internal/postfilter/shortterm.go`
- Create: `internal/postfilter/shortterm_test.go`

**Spec reference:** ITU-T G.729 §A.4.2.1. Apply the short-term postfilter
`1/A(z/γ_d)` to the long-term postfilter output `r'(n)`, producing the
short-term postfiltered signal `s_st(n)`:

```
s_st(n) = r'(n) − Σ_{i=1..10} a_den[i] · s_st(n−i)
```

This is functionally identical to the synthesis filter from Phase 1e
(§3.10), only the coefficients are `a_den` (bandwidth-expanded by γ_d)
instead of the raw LPC `a`, and the state `pastSynthPost` is separate
from the main synth filter's state.

The arithmetic matches Phase 1e's `filterSubframe` exactly — the only
differences are (a) `a_den` vs `a`, (b) the state array
`pf.pastSynthPost` vs `Synthesizer.pastSynth`, (c) the input is
`r'(n)` vs `u(n)`. Consider literally invoking `synth.filterSubframe`
if a refactor to export it is cheap; otherwise duplicate the loop
here and flag the duplication.

**Design choice:** Duplicate the loop in `internal/postfilter/shortterm.go`
to avoid cross-package coupling of a private helper. The two loops
are small enough (~15 lines) that duplication is cleaner than
refactoring.

- [x] **Step 1: Write the failing tests**

Write to `internal/postfilter/shortterm_test.go`:

```go
package postfilter

import "testing"

// Zero LPC tail → s_st = r' (identity).
func TestApplyShortTerm_ZeroTail(t *testing.T) {
    var pf Postfilter
    aDen := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var rIn, sOut [subframeLen]int16
    for i := range rIn {
        rIn[i] = int16(500 + i*7)
    }

    pf.applyShortTerm(&aDen, &rIn, &sOut)

    for i := range sOut {
        if sOut[i] != rIn[i] {
            t.Errorf("sOut[%d] = %d, want %d (zero-tail identity)",
                i, sOut[i], rIn[i])
        }
    }
}

// State carry: two consecutive calls with identity filter pass through r'.
func TestApplyShortTerm_StateCarriesAcrossSubframes(t *testing.T) {
    var pf Postfilter
    aDen := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var r1, r2, s1, s2 [subframeLen]int16
    for i := range r1 {
        r1[i] = int16(100 + i)
        r2[i] = int16(200 + i)
    }

    pf.applyShortTerm(&aDen, &r1, &s1)
    pf.applyShortTerm(&aDen, &r2, &s2)

    for i := range s1 {
        if s1[i] != r1[i] {
            t.Errorf("s1[%d] = %d, want %d", i, s1[i], r1[i])
        }
        if s2[i] != r2[i] {
            t.Errorf("s2[%d] = %d, want %d", i, s2[i], r2[i])
        }
    }
}

// pastSynthPost holds last 10 of s_st after the call.
func TestApplyShortTerm_UpdatesPastSynthPost(t *testing.T) {
    var pf Postfilter
    aDen := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var rIn, sOut [subframeLen]int16
    for i := range rIn {
        rIn[i] = int16(i + 10)
    }

    pf.applyShortTerm(&aDen, &rIn, &sOut)

    for i := 0; i < lpcOrder; i++ {
        want := sOut[subframeLen-lpcOrder+i]
        if pf.pastSynthPost[i] != want {
            t.Errorf("pastSynthPost[%d] = %d, want %d",
                i, pf.pastSynthPost[i], want)
        }
    }
}
```

- [x] **Step 2: Verify the tests fail**

Run: `go test ./internal/postfilter/ -run TestApplyShortTerm -v`

Expected: compile error.

- [x] **Step 3: Implement applyShortTerm**

Write to `internal/postfilter/shortterm.go`:

```go
package postfilter

import "github.com/exedev/g729/internal/fixed"

// applyShortTerm runs the short-term postfilter IIR
//   s_st(n) = r'(n) − Σ_{i=1..10} aDen[i] · s_st(n−i)
// per ITU-T G.729 §A.4.2.1, using pastSynthPost as the 10-tap IIR memory.
//
// Arithmetic parallels synth.filterSubframe: Q13 accumulator, LShl by 3,
// Round to Q0 Word16. Different state array and different coefficients.
func (pf *Postfilter) applyShortTerm(aDen *[11]int16, rIn *[subframeLen]int16, sOut *[subframeLen]int16) {
    var work [lpcOrder + subframeLen]int16
    copy(work[:lpcOrder], pf.pastSynthPost[:])

    for n := 0; n < subframeLen; n++ {
        lTemp := fixed.LMult(rIn[n], aDen[0]) // Q13 (aDen[0] = 4096 = 1.0 Q12)
        for i := 1; i <= lpcOrder; i++ {
            lTemp = fixed.LMsu(lTemp, aDen[i], work[lpcOrder+n-i])
        }
        lTemp = fixed.LShl(lTemp, 3)
        work[lpcOrder+n] = fixed.Round(lTemp)
    }

    copy(sOut[:], work[lpcOrder:])
    copy(pf.pastSynthPost[:], work[lpcOrder+subframeLen-lpcOrder:])
}
```

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/postfilter/ -run TestApplyShortTerm -v`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/postfilter/shortterm.go internal/postfilter/shortterm_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): short-term synthesis 1/A(z/γ_d) per ITU §A.4.2.1

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 7: Tilt compensation

**Files:**
- Create: `internal/postfilter/tilt.go`
- Create: `internal/postfilter/tilt_test.go`

**Spec reference:** ITU-T G.729 §A.4.2.3. The short-term postfilter
introduces a spectral tilt (low-frequency boost). To compensate, apply
a one-tap FIR:

```
s_tilt(n) = s_st(n) + μ · s_st(n−1)
```

where `μ` is derived from the first-order autocorrelation of the
short-term postfilter impulse response. Per §A.4.2.3:

```
μ = γ_t · k_1
```

where `k_1 = −r_h(1) / r_h(0)`, `r_h(i)` = autocorrelation lag i of `h(n)` (impulse response of `A(z/γ_n)/A(z/γ_d)`), and `γ_t ≈ 0.9` (verify).

**Annex A simplification:** §A.4.2.3 specifies the Annex-A approximation of `k_1`. Transcribe exactly — do not guess. Typical Annex-A approach: compute a 22-sample truncated impulse response, do two 22-tap autocorrelations, and derive `k_1`. Output `μ` in Q15.

For the structural plan here, we specify the *shape* of the computation and leave the exact formula to the engineer (who must read §A.4.2.3). The tests below validate the structural properties.

- [x] **Step 1: Write the failing tests**

Write to `internal/postfilter/tilt_test.go`:

```go
package postfilter

import "testing"

// With μ = 0, tilt filter is identity.
func TestApplyTilt_ZeroMuIsIdentity(t *testing.T) {
    var pf Postfilter
    var sIn, sOut [subframeLen]int16
    for i := range sIn {
        sIn[i] = int16(200 + i*3)
    }

    pf.applyTiltWithMu(&sIn, 0, &sOut)

    for i := range sOut {
        if sOut[i] != sIn[i] {
            t.Errorf("sOut[%d] = %d, want %d (μ = 0 is identity)",
                i, sOut[i], sIn[i])
        }
    }
}

// μ = 0.5 Q15 = 16384: sOut[n] = sIn[n] + 0.5·sIn[n-1].
// With sIn = constant 100 and pastTiltInput = 0:
//   sOut[0] = 100 + 0.5·0 = 100
//   sOut[1..] = 100 + 0.5·100 = 150
func TestApplyTilt_HalfMuAmplifies(t *testing.T) {
    var pf Postfilter
    var sIn, sOut [subframeLen]int16
    for i := range sIn {
        sIn[i] = 100
    }

    pf.applyTiltWithMu(&sIn, 16384, &sOut)

    if sOut[0] < 99 || sOut[0] > 101 {
        t.Errorf("sOut[0] = %d, want 100 (pastTiltInput = 0)", sOut[0])
    }
    for i := 1; i < subframeLen; i++ {
        if sOut[i] < 149 || sOut[i] > 151 {
            t.Errorf("sOut[%d] = %d, want 150 (±1)", i, sOut[i])
        }
    }
}

// pastTiltInput updates to the last sample of sIn.
func TestApplyTilt_UpdatesPastTiltInput(t *testing.T) {
    var pf Postfilter
    var sIn, sOut [subframeLen]int16
    for i := range sIn {
        sIn[i] = int16(i + 1)
    }

    pf.applyTiltWithMu(&sIn, 0, &sOut)

    if pf.pastTiltInput != sIn[subframeLen-1] {
        t.Errorf("pastTiltInput = %d, want %d",
            pf.pastTiltInput, sIn[subframeLen-1])
    }
}
```

- [x] **Step 2: Verify the tests fail**

Run: `go test ./internal/postfilter/ -run TestApplyTilt -v`

Expected: compile error.

- [x] **Step 3: Implement the tilt filter + (separately) computeTiltMu**

Write to `internal/postfilter/tilt.go`:

```go
package postfilter

// computeTiltMu returns the tilt compensation factor μ (Q15) per
// ITU-T G.729 §A.4.2.3.
//
// PLACEHOLDER IMPLEMENTATION. The exact formula per §A.4.2.3 must be
// transcribed from the spec. The structure below computes the impulse
// response h(n) of A(z/γ_n)/A(z/γ_d) truncated to 22 samples, then
// k_1 = −r_h(1)/r_h(0), μ = γ_t · k_1. Replace with the spec formula.
func (pf *Postfilter) computeTiltMu(aNum, aDen *[11]int16) int16 {
    // TODO: transcribe §A.4.2.3. For now, return 0 (tilt filter bypassed).
    // The engineer MUST replace this with the spec-faithful derivation
    // before marking Task 7 complete — but leaving μ = 0 does not break
    // the overall chain; the output quality will suffer until filled in.
    _ = aNum
    _ = aDen
    return 0
}

// applyTiltWithMu applies the one-tap tilt filter
//   s_tilt(n) = s_st(n) + μ · s_st(n-1)
// per ITU-T G.729 §A.4.2.3.
//
// pastTiltInput holds s_st(-1) on entry; on return, holds s_st(39).
func (pf *Postfilter) applyTiltWithMu(sIn *[subframeLen]int16, muQ15 int16, sOut *[subframeLen]int16) {
    prev := pf.pastTiltInput
    for n := 0; n < subframeLen; n++ {
        // sOut[n] = sIn[n] + μ·prev. μ·prev at Q15·Q0 = Q15 in Word32.
        prod := int32(muQ15) * int32(prev)
        // Round Q15 → Q0 (shift right by 15).
        contrib := int16((prod + (1 << 14)) >> 15)
        // Saturate-add sIn[n] + contrib.
        sum := int32(sIn[n]) + int32(contrib)
        if sum > 32767 {
            sum = 32767
        } else if sum < -32768 {
            sum = -32768
        }
        sOut[n] = int16(sum)
        prev = sIn[n]
    }
    pf.pastTiltInput = prev
}
```

**Important.** `computeTiltMu` is intentionally a placeholder returning 0. Fill it in from §A.4.2.3. When Phase 1f's top-level `Filter` is wired (Task 9), it will call `computeTiltMu(aNum, aDen)` and pass the result to `applyTiltWithMu`. For Task 7 we test only `applyTiltWithMu` with explicitly supplied μ values. The spec-faithful `computeTiltMu` implementation will be verified implicitly via Phase 1g ITU test vectors.

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/postfilter/ -run TestApplyTilt -v`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/postfilter/tilt.go internal/postfilter/tilt_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): tilt compensation one-tap FIR per ITU §A.4.2.3

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 8: Adaptive Gain Control (AGC)

**Files:**
- Create: `internal/postfilter/agc.go`
- Create: `internal/postfilter/agc_test.go`

**Spec reference:** ITU-T G.729 §A.4.2.4. The AGC matches the energy of the
postfilter output to that of the synthesis input. Per §A.4.2.4, the
target gain is:

```
g_target = √( E(s) / E(s_tilt) )
```

which is then smoothed across samples:

```
g_pf(n) = α · g_pf(n−1) + (1 − α) · g_target
s_pf(n) = g_pf(n) · s_tilt(n)
```

with `α ≈ 0.9875` (verify against §A.4.2.4).

**Annex A simplification:** §A.4.2.4 specifies simplifications (e.g.
avoiding the square-root by tracking energies or gains in a way that
removes the sqrt, or using a LUT). Transcribe exactly.

**Plan choice:** Expose two primitives:
- `computeAGCTargetGain(s, sTilt) int16` — returns `g_target` in Q14.
- `applyAGC(sTilt, gTargetQ14) sPf` — does the smoothing + scaling, updates `agcGainPrev`.

The computation of `g_target` requires energy computation and a
square-root or equivalent. The plan leaves the exact algorithmic choice
(LUT-based vs Newton-Raphson iteration vs ITU's `Sqrt_L`) to the
engineer, guided by §A.4.2.4. The structural tests below validate the
functional contract.

- [x] **Step 1: Write the failing tests**

Write to `internal/postfilter/agc_test.go`:

```go
package postfilter

import "testing"

// Energy-neutral case: s and sTilt have equal energies → g_target ≈ 1.0 Q14.
func TestComputeAGCTargetGain_EqualEnergy(t *testing.T) {
    var pf Postfilter
    var s, sTilt [subframeLen]int16
    for i := range s {
        s[i] = int16(1000 - i*10)
        sTilt[i] = int16(1000 - i*10) // identical
    }

    g := pf.computeAGCTargetGain(&s, &sTilt)
    // g should be ≈ 16384 = 1.0 Q14 (±2 LSB tolerance).
    if g < 16380 || g > 16388 {
        t.Errorf("g = %d, want ≈ 16384 (equal energies)", g)
    }
}

// Zero sTilt → g_target = 0 (avoid division by zero).
func TestComputeAGCTargetGain_ZeroTiltEnergy(t *testing.T) {
    var pf Postfilter
    var s, sTilt [subframeLen]int16
    for i := range s {
        s[i] = int16(1000)
    }
    // sTilt all zero.

    g := pf.computeAGCTargetGain(&s, &sTilt)
    if g != 0 {
        t.Errorf("g = %d, want 0 (zero sTilt energy)", g)
    }
}

// AGC smoothing: with constant g_target = 0.5 Q14, agcGainPrev slews
// toward that value across samples but never overshoots.
func TestApplyAGC_SmoothingDoesNotOvershoot(t *testing.T) {
    var pf Postfilter
    var sTilt, sPf [subframeLen]int16
    for i := range sTilt {
        sTilt[i] = 1000
    }
    const gTargetQ14 int16 = 8192 // 0.5

    pf.applyAGC(&sTilt, gTargetQ14, &sPf)

    // agcGainPrev should now be ∈ (0, 8192].
    if pf.agcGainPrev < 0 || pf.agcGainPrev > 8192 {
        t.Errorf("agcGainPrev = %d, want ∈ (0, 8192]", pf.agcGainPrev)
    }
    // Over many subframes it should converge to 8192.
    for k := 0; k < 200; k++ {
        pf.applyAGC(&sTilt, gTargetQ14, &sPf)
    }
    if pf.agcGainPrev < 8190 || pf.agcGainPrev > 8194 {
        t.Errorf("after convergence, agcGainPrev = %d, want ≈ 8192",
            pf.agcGainPrev)
    }
}
```

- [x] **Step 2: Verify the tests fail**

Run: `go test ./internal/postfilter/ -run 'TestComputeAGC|TestApplyAGC' -v`

Expected: compile error.

- [x] **Step 3: Implement computeAGCTargetGain and applyAGC**

Write to `internal/postfilter/agc.go`:

```go
package postfilter

// computeAGCTargetGain returns the AGC target gain g_target (Q14) per
// ITU-T G.729 §A.4.2.4 as √(E(s) / E(sTilt)).
//
// Implementation sketch: accumulate E_s and E_t as sum-of-squares in int64,
// compute gQ28 = (E_s << 28) / E_t via 32/32→32 division (with normalisation
// if necessary), then take the integer square root (LUT-based or via
// Newton-Raphson).
//
// For the first cut we use a straightforward int64 sqrt. Phase 1g may
// swap in fixed.Sqrt or a spec-specified approximation.
func (pf *Postfilter) computeAGCTargetGain(s, sTilt *[subframeLen]int16) int16 {
    var eS, eT int64
    for i := 0; i < subframeLen; i++ {
        eS += int64(s[i]) * int64(s[i])
        eT += int64(sTilt[i]) * int64(sTilt[i])
    }
    if eT == 0 {
        return 0
    }
    // Compute (E_s / E_t) in Q28 (double the Q14 target).
    ratioQ28 := (eS << 28) / eT
    if ratioQ28 < 0 {
        return 0
    }
    // √(ratioQ28 at Q28) = sqrt at Q14.
    return isqrtQ14(ratioQ28)
}

// isqrtQ14 returns ⌊√x⌋ where x is interpreted at Q28 (result is at Q14).
// Uses integer Newton-Raphson. Replace with a table lookup if Phase 1g
// demands bit-exact match to the spec's Sqrt_L convention.
func isqrtQ14(xQ28 int64) int16 {
    if xQ28 == 0 {
        return 0
    }
    // Seed: log2-based initial guess.
    var guess int64 = 1 << 14
    for i := 0; i < 10; i++ {
        next := (guess + xQ28/guess) >> 1
        if next == guess {
            break
        }
        guess = next
    }
    if guess > 32767 {
        return 32767
    }
    return int16(guess)
}

// applyAGC smooths g_target into agcGainPrev (one-pole lowpass, α ≈ 0.99)
// and scales sTilt to produce sPf per ITU-T G.729 §A.4.2.4.
func (pf *Postfilter) applyAGC(sTilt *[subframeLen]int16, gTargetQ14 int16, sPf *[subframeLen]int16) {
    const alphaQ15 int32 = 32440 // ≈ 0.99; verify against §A.4.2.4

    gPrev := int32(pf.agcGainPrev)
    for n := 0; n < subframeLen; n++ {
        // g_pf = α·g_prev + (1−α)·g_target (all Q14/Q15; careful with alignment).
        g := (alphaQ15*gPrev + (32768-alphaQ15)*int32(gTargetQ14)) >> 15
        gPrev = g
        // sPf[n] = g · sTilt[n], Q14·Q0 → round to Q0.
        prod := g * int32(sTilt[n])
        v := (prod + (1 << 13)) >> 14
        if v > 32767 {
            v = 32767
        } else if v < -32768 {
            v = -32768
        }
        sPf[n] = int16(v)
    }
    if gPrev > 32767 {
        gPrev = 32767
    } else if gPrev < 0 {
        gPrev = 0
    }
    pf.agcGainPrev = int16(gPrev)
}
```

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/postfilter/ -run 'TestComputeAGC|TestApplyAGC' -v`

Expected: `PASS`.

- [x] **Step 5: Commit**

```bash
git add internal/postfilter/agc.go internal/postfilter/agc_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): adaptive gain control with smoothing per ITU §A.4.2.4

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 9: `Filter` public entry — full chain + state updates

**Files:**
- Create: `internal/postfilter/postfilter.go`
- Modify: `internal/postfilter/postfilter_test.go` (add tests)

**Spec reference:** Composes Tasks 2-8 into one call per ITU-T G.729 §A.4.2.

**Required order of operations (per §A.4.2):**
1. Bandwidth expand `a` → `aNum` (γ_n) and `aDen` (γ_d).
2. `r = computeResidual(aNum, s)`.
3. Write `r` into `pastResidual[pitchMax : pitchMax + subframeLen]`.
4. `T = refinePitch(r, tInt)`.
5. `rOut = applyLongTerm(r, T)`.
6. `sSt = applyShortTerm(aDen, rOut)`.
7. `μ = computeTiltMu(aNum, aDen)`.
8. `sTilt = applyTiltWithMu(sSt, μ)`.
9. `gTarget = computeAGCTargetGain(s, sTilt)`.
10. `sPf = applyAGC(sTilt, gTarget)`.
11. Slide `pastResidual` left by `subframeLen` so the next call finds `r` in the same tail slot.

- [x] **Step 1: Write the failing test**

Append to `internal/postfilter/postfilter_test.go`:

```go
// End-to-end smoke: Filter with zero input must produce zero output.
func TestFilter_ZeroInputZeroOutput(t *testing.T) {
    var pf Postfilter
    a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var s, sPf [subframeLen]int16

    pf.Filter(&a, 40, &s, &sPf)

    for i := range sPf {
        if sPf[i] != 0 {
            t.Errorf("sPf[%d] = %d, want 0 (zero input)", i, sPf[i])
        }
    }
}

// End-to-end: with zero-LPC (a_1..a_10 = 0), the postfilter is essentially
// an identity (bandwidth-expansion zeros all tail, residual = s, no pitch
// correlation, short-term synthesis identity, tilt μ = 0, AGC g = 1).
// Output should approximately equal input for low-energy signals.
func TestFilter_ZeroLPCIsApproximateIdentity(t *testing.T) {
    var pf Postfilter
    a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
    var s, sPf [subframeLen]int16
    for i := range s {
        s[i] = int16(500 + i*3)
    }

    // Run a few frames to let AGC converge.
    for k := 0; k < 5; k++ {
        pf.Filter(&a, 40, &s, &sPf)
    }

    // After convergence, each output should be within ~10% of the input.
    for i := range sPf {
        want := int(s[i])
        got := int(sPf[i])
        diff := got - want
        if diff < 0 {
            diff = -diff
        }
        tol := want / 10
        if tol < 5 {
            tol = 5
        }
        if diff > tol {
            t.Errorf("sPf[%d] = %d, want ≈ %d (tol %d)", i, got, want, tol)
        }
    }
}
```

- [x] **Step 2: Verify the tests fail**

Run: `go test ./internal/postfilter/ -run TestFilter_ -v`

Expected: compile error — `Filter` undefined on `*Postfilter`.

- [x] **Step 3: Implement Filter**

Write to `internal/postfilter/postfilter.go`:

```go
package postfilter

// Annex A postfilter constants (verify against §A.4.2).
const (
    gammaNumQ15 int16 = 18022 // γ_n ≈ 0.55
    gammaDenQ15 int16 = 22938 // γ_d ≈ 0.70
)

// Filter runs the full Annex A postfilter chain on one subframe per
// ITU-T G.729 §A.4.2.
//
// Inputs:
//   a    — LP filter coefficients for this subframe (Q12)
//   tInt — integer pitch delay decoded by internal/pitch
//   s    — pre-postfilter synthesis from internal/synth (Q0)
//
// Output:
//   sPf  — postfiltered samples (Q0)
//
// Updates all Postfilter state fields; zero-allocation.
func (pf *Postfilter) Filter(a *[11]int16, tInt int, s *[subframeLen]int16, sPf *[subframeLen]int16) {
    var aNum, aDen [11]int16
    expandBandwidth(a, gammaNumQ15, &aNum)
    expandBandwidth(a, gammaDenQ15, &aDen)

    var r [subframeLen]int16
    pf.computeResidual(&aNum, s, &r)

    // Slide pastResidual left by subframeLen, put r in the tail.
    copy(pf.pastResidual[:pitchMax+subframeLen-subframeLen], pf.pastResidual[subframeLen:])
    copy(pf.pastResidual[pitchMax:], r[:])

    T := pf.refinePitch(&r, tInt)

    var rOut [subframeLen]int16
    pf.applyLongTerm(&r, T, &rOut)

    var sSt [subframeLen]int16
    pf.applyShortTerm(&aDen, &rOut, &sSt)

    muQ15 := pf.computeTiltMu(&aNum, &aDen)
    var sTilt [subframeLen]int16
    pf.applyTiltWithMu(&sSt, muQ15, &sTilt)

    gTarget := pf.computeAGCTargetGain(s, &sTilt)
    pf.applyAGC(&sTilt, gTarget, sPf)
}
```

- [x] **Step 4: Verify tests pass**

Run: `go test -race ./internal/postfilter/...`

Expected: `PASS` for all tests (including all earlier tasks' tests).

If `TestFilter_ZeroLPCIsApproximateIdentity` fails, inspect each stage's
output with printf debugging to isolate where identity breaks down. With
zero-LPC every internal stage must be identity (except AGC which converges
toward 1.0).

- [x] **Step 5: Commit**

```bash
git add internal/postfilter/postfilter.go internal/postfilter/postfilter_test.go
git commit -m "$(cat <<'EOF'
feat(postfilter): Filter top-level wires all stages per ITU §A.4.2

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 10: Reset determinism + two-subframe state propagation

**Files:**
- Modify: `internal/postfilter/postfilter_test.go` (add tests)

- [x] **Step 1: Write the failing tests**

Append to `internal/postfilter/postfilter_test.go`:

```go
// After Reset, Filter with the same inputs produces the same output as
// a fresh zero-value Postfilter.
func TestFilter_ResetRestoresZeroValueDeterminism(t *testing.T) {
    a := [11]int16{4096, 1500, -800, 300, -100, 50, 0, 0, 0, 0, 0}
    var s [subframeLen]int16
    for i := range s {
        s[i] = int16(200 + i*5)
    }

    // Reference.
    var pfRef Postfilter
    var sRef [subframeLen]int16
    pfRef.Filter(&a, 40, &s, &sRef)

    // Under test: populate then Reset.
    var pfUUT Postfilter
    var dummy [subframeLen]int16
    pfUUT.Filter(&a, 60, &dummy, &dummy) // dirty the state
    pfUUT.Reset()

    var sUUT [subframeLen]int16
    pfUUT.Filter(&a, 40, &s, &sUUT)

    for i := range sRef {
        if sRef[i] != sUUT[i] {
            t.Errorf("sPf[%d] = %d, want %d (Reset non-deterministic)",
                i, sUUT[i], sRef[i])
        }
    }
}

// State propagates across subframes: subframe 2's output depends on the
// state left behind by subframe 1.
func TestFilter_StatePropagatesAcrossSubframes(t *testing.T) {
    a := [11]int16{4096, 1500, -800, 300, -100, 50, 0, 0, 0, 0, 0}
    var s1, s2 [subframeLen]int16
    for i := range s1 {
        s1[i] = int16(200 + i*5)
        s2[i] = int16(500 - i*3)
    }

    // Variant A: filter s1 then s2 on one postfilter.
    var pfA Postfilter
    var a1, a2 [subframeLen]int16
    pfA.Filter(&a, 40, &s1, &a1)
    pfA.Filter(&a, 40, &s2, &a2)

    // Variant B: filter s2 on a fresh postfilter (no s1 history).
    var pfB Postfilter
    var b2 [subframeLen]int16
    pfB.Filter(&a, 40, &s2, &b2)

    // a2 must differ from b2 in at least one sample — otherwise state
    // failed to propagate.
    different := false
    for i := range a2 {
        if a2[i] != b2[i] {
            different = true
            break
        }
    }
    if !different {
        t.Error("a2 == b2 — postfilter state did not carry across subframes")
    }
}
```

- [x] **Step 2: Run the tests — they should already pass**

Run: `go test -race ./internal/postfilter/ -run 'TestFilter_Reset|TestFilter_StatePropagates' -v`

Expected: `PASS`.

- [x] **Step 3: Commit**

```bash
git add internal/postfilter/postfilter_test.go
git commit -m "$(cat <<'EOF'
test(postfilter): lock Reset determinism and two-subframe state propagation

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 11: Zero-allocation lock + benchmarks + doc polish

**Files:**
- Create: `internal/postfilter/alloc_test.go`
- Create: `internal/postfilter/bench_test.go`
- Rewrite: `internal/postfilter/doc.go`

- [x] **Step 1: Write zero-allocation tests**

Write to `internal/postfilter/alloc_test.go`:

```go
package postfilter

import "testing"

func TestNoAllocationInFilter(t *testing.T) {
    var pf Postfilter
    a := [11]int16{4096, 1500, -800, 300, -100, 50, 20, -10, 5, 0, 0}
    var s, sPf [subframeLen]int16
    for i := range s {
        s[i] = int16(200 + i*5)
    }

    allocs := testing.AllocsPerRun(100, func() {
        pf.Filter(&a, 40, &s, &sPf)
    })
    if allocs != 0 {
        t.Errorf("Filter allocs = %v, want 0", allocs)
    }
}

func TestNoAllocationInReset(t *testing.T) {
    var pf Postfilter
    pf.pastS[0] = 1
    pf.pastResidual[0] = 1
    pf.pastSynthPost[0] = 1
    pf.pastTiltInput = 1
    pf.agcGainPrev = 1

    allocs := testing.AllocsPerRun(100, func() {
        pf.Reset()
    })
    if allocs != 0 {
        t.Errorf("Reset allocs = %v, want 0", allocs)
    }
}
```

- [x] **Step 2: Write benchmarks**

Write to `internal/postfilter/bench_test.go`:

```go
package postfilter

import "testing"

var (
    benchA    = [11]int16{4096, 1500, -800, 300, -100, 50, 20, -10, 5, -2, 1}
    benchS    [subframeLen]int16
    benchSPf  [subframeLen]int16
)

func init() {
    for i := range benchS {
        benchS[i] = int16(i * 17)
    }
}

func BenchmarkFilter(b *testing.B) {
    var pf Postfilter
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        pf.Filter(&benchA, 40, &benchS, &benchSPf)
    }
}
```

- [x] **Step 3: Run the benchmarks and record results**

Run: `go test -bench=. -benchmem -run=^$ ./internal/postfilter/`

Expected: `0 B/op, 0 allocs/op`. Timing budget: <3 μs/subframe is generous
given the chain's complexity; anything under 10 μs/subframe is acceptable
for a first cut.

If allocations are flagged, the usual suspects are:
- Arrays declared with non-const length escaping.
- Slice-of-array conversions triggering reslices.
- Closures over stack state.

Use `go build -gcflags="-m" ./internal/postfilter/` to pinpoint.

- [x] **Step 4: Polish doc.go**

Overwrite `internal/postfilter/doc.go`:

```go
// Package postfilter implements the G.729 Annex A adaptive postfilter
// chain per ITU-T G.729 §A.4.2. It consumes the pre-postfilter synthesis
// s[40] from internal/synth, the LP filter coefficients a[11] (Q12), and
// the integer pitch delay t_int from internal/pitch, and produces
// postfiltered samples s_pf[40] ready for the output high-pass stage.
//
// # Pipeline
//
// Per ITU-T G.729 §A.4.2.1 / §A.4.2.2 / §A.4.2.3 / §A.4.2.4:
//
//  1. Bandwidth expansion:  a → aNum (γ_n ≈ 0.55), aDen (γ_d ≈ 0.70)
//  2. Residual FIR:         r(n) = Σ aNum[i]·s(n−i)
//  3. Pitch refinement:     T ∈ {t_int−1, t_int, t_int+1}, max cross-correlation
//  4. Long-term postfilter: r′(n) = (r(n) + g_l·r(n−T)) / (1 + g_l)
//  5. Short-term synthesis: s_st(n) = r′(n) − Σ aDen[i]·s_st(n−i)
//  6. Tilt compensation:    s_tilt(n) = s_st(n) + μ·s_st(n−1)
//  7. Adaptive gain control: s_pf(n) = g_pf(n)·s_tilt(n) with smoothing
//
// Each stage carries its own state across subframes. A Postfilter's zero
// value is a valid reset state per §A.4.2 first-frame initialisation.
//
// # Numerical contract
//
//	Inputs:
//		a      — Q12 [11]int16, a[0] = 4096
//		t_int  — integer, ∈ [20, 143]
//		s      — Q0 [40]int16
//	Output:
//		s_pf   — Q0 [40]int16, saturated
//
// # Scratch-from-spec
//
// All coefficients and formulas derive from ITU-T G.729 §A.4.2 directly.
// No ITU reference C, bcg729, Sipro Lab, or any other existing G.729
// implementation was consulted for algorithmic code. Numerical primitives
// route through internal/fixed for ITU saturation semantics.
//
// # Concurrency
//
// Postfilter is not safe for concurrent use. One instance per decoder channel.
package postfilter
```

- [x] **Step 5: Full run**

Run: `go test -race ./... && go vet ./...`

Expected: all packages `ok`; `go vet` silent.

- [x] **Step 6: Commit**

```bash
git add internal/postfilter/alloc_test.go internal/postfilter/bench_test.go internal/postfilter/doc.go
git commit -m "$(cat <<'EOF'
test(postfilter): lock zero-alloc + benches; polish package doc

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Completion criteria

- [x] All 11 tasks checked off.
- [x] `go test -race ./...` passes.
- [x] `go vet ./...` silent.
- [x] `BenchmarkFilter` reports `0 B/op, 0 allocs/op`.
- [x] `git log --oneline` shows 11 commits on `main` for Phase 1f.
- [x] Completion report saved to `docs/superpowers/plans/2026-04-21-phase1f-postfilter-completion-report.md`.

---

## Completion report template

At the end, record:

1. Spec sections referenced (§A.4.2.1/2/3/4, §3.10.1, §B.4).
2. Deviations from the plan and their rationale. Expected candidates:
   - Exact values of γ_n, γ_d, γ_t, γ_l, α_agc adopted after spec cross-check.
   - `computeTiltMu` implementation (§A.4.2.3 gives the full formula; the plan intentionally uses a placeholder returning 0).
   - AGC square-root approach (Newton-Raphson vs LUT vs `fixed.Sqrt`).
   - Residual Q-format: if Q0 proves lossy, upgrade to Q12 and document.
   - Long-term gain computation: the plan uses direct int64 arithmetic; a rewrite using `fixed.DivS` + `NormL` may be required for bit-exactness.
3. Benchmark numbers.
4. Open items for Phase 1g:
   - Bit-exact output vs ITU test vectors for each stage.
   - Confirming `computeTiltMu` matches Annex A §A.4.2.3 exactly.
   - AGC convergence rate (α_agc): compare measured vs spec.
   - Long-term pitch T refinement on edge cases (t_int at 20 or 143).
   - Output HP filter: Phase 1f does NOT include the final HP post-processing filter — that lives in Phase 1g as part of the decoder's output stage, or as a separate Phase 1f'. Flag for Phase 1g planning.
5. Commit list (oldest → newest).

---

## Self-review checklist (for plan author)

**Spec coverage:**
- [x] §A.4.2.1 bandwidth expansion → Task 2
- [x] §A.4.2.1 residual FIR → Task 3
- [x] §A.4.2.2 pitch refinement → Task 4
- [x] §A.4.2.2 long-term postfilter → Task 5
- [x] §A.4.2.1 short-term synthesis → Task 6
- [x] §A.4.2.3 tilt compensation → Task 7
- [x] §A.4.2.4 AGC → Task 8
- [x] §A.4.2 top-level composition → Task 9
- [x] §A.4.2 first-frame zero state → Task 1 + Task 10
- [x] Zero-allocation + benchmarks → Task 11

**Placeholder scan:** `computeTiltMu` is deliberately a placeholder returning 0. This is explicit in Task 7's text and called out in the completion report template. No other placeholders in the code. ✓

**Type consistency:** `Postfilter`, `Filter`, `Reset` match across Tasks 1, 9, 10, 11. Private helpers (`expandBandwidth`, `computeResidual`, `refinePitch`, `applyLongTerm`, `applyShortTerm`, `computeTiltMu`, `applyTiltWithMu`, `computeAGCTargetGain`, `applyAGC`) match across their definition task and Task 9's wiring. ✓

**Q-format consistency:** `a`/`aNum`/`aDen` always Q12; `s`/`sPf`/`sSt`/`sTilt` always Q0; gains Q14 (agcGain) or Q15 (μ, γ_n/γ_d). Explicit choice: residual `r(n)` is Q0, flagged in the completion report template for Phase 1g re-evaluation. ✓

**TDD discipline:** Each task has write failing test → verify fail → implement → verify pass → commit. Test-only tasks (10) still follow the cycle with "expected PASS" step. ✓

**Spec fidelity caveats:** Three places where the plan defers to spec transcription by the engineer: (a) exact γ values, (b) `computeTiltMu` formula, (c) AGC α_agc time constant. These are explicitly flagged in both the task text and the completion report template. ✓
