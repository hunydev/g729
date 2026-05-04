# Phase 3a REF-1 — g_c representation design note

Date: 2026-05-04
Plan: `2026-05-04-phase3a-gc-qformat-fix-plan.md`
Inputs:
- DIAG-1 commit `b4f6b05` and report appendix §6 in
  `docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md`
- Spec: ITU-T G.729 (06/2012) §3.9.2 eq. (74)/(75)
- Textbook: Salami 1998 IEEE T-SAP §V.B; Kondoz §6 (CS-ACELP gain quantization)

I1 (clean-room) HARD: spec PDF + textbooks only. No external G.729
implementation consulted. If unsure → STOP.

## 1. Pin

**OQ-GCREP = (a) mantissa Q14 + exponent int8.**

Justification (DIAG-1 corpus over SPEECH.BIT, 7500 subframes):

| tap                    | min     | max      | mean      | wraps int16 |
|------------------------|---------|----------|-----------|-------------|
| `gc0Q14_unsat` (int32) | 75 244  | 652 416  | 525 722.7 | 100.00 %    |
| `prodQ12_unsat` (int32)| 3 627   | 652 396  | 156 258.6 | 77.48 %     |

g_c0 in unscaled units: max ≈ 39.8, mean ≈ 32.1. Final g_c (after γ̂_c)
peaks ≈ 159 — far beyond the int16 Q12 envelope (max ≈ 7.999).

Per G.729 §3.9.2 eq. (75), `g_c0 = 2^x` where `x` is the predicted
log-gain in Q10 dB / converted to log2. Our existing `pow2Fixed` already
splits `x` into:

```
intPart = x >> 10            // exponent of 2
fracQ14 = lookup(...)        // Q14 mantissa in [16384, 32767)
result  = fracQ14 << (intPart - 14)   (or >> if intPart < 14)
```

That is *natively* a (mantissa Q14, exponent int8) representation;
collapsing it back into a single Q0 / Q12 int16 is what destroys the
range. Option (a) keeps the spec's own decomposition and confines the
ripple to one site (BuildExcitation).

## 2. New API

### `internal/gain`

```go
// Decode now returns g_c as (mantissa Q14, exponent int8) so the full
// spec dynamic range survives into BuildExcitation.
//
//   g_c (linear) = gcMantQ14 · 2^(gcExp - 14)
//
// Invariants:
//   gcMantQ14 ∈ [16384, 32767]   (Q14 [1.0, 2.0))
//   gcExp     ∈ [-15, +9]        (covers DIAG-1 corpus + margin)
//   gcMantQ14 = 0 ⇒ g_c = 0     (zero-energy guard path)
func (d *Decoder) Decode(idx Indices, c *[40]int16) (gpQ14 int16, gcMantQ14 int16, gcExp int8)
```

The legacy `gcQ12` Q12 single-int16 return is removed. Test-only
adapters convert (mantissa, exp) → Q12 int16 with saturation for
asserts that need the legacy view.

### `internal/synth`

```go
// BuildExcitation composes
//   u(n) = g_p · v(n) + g_c · c(n), saturated to Q0 Word16.
// where g_c = gcMantQ14 · 2^(gcExp - 14).
//
// Q-format alignment per sample:
//   lPitch = LMult(gpQ14, v[n])            // Q15 in Word32
//   prod32 = LMult(gcMantQ14, c[n])         // Q28 in Word32  (Q14 · Q13 · 2)
//   shift  = 13 - gcExp                     // align to Q15
//   if shift >= 0: lCode = LShr(prod32, shift)
//   else:          lCode = LShlSat(prod32, -shift)   // saturate before sum
//   lSum   = LAdd(lPitch, lCode)            // Q15
//   u[n]   = Round(LShl(lSum, 1))           // Q15 → Q16 → Q0 with sat
func BuildExcitation(gpQ14 int16, gcMantQ14 int16, gcExp int8, v, c *[40]int16, u *[40]int16)
```

Edge case: `gcMantQ14 == 0` short-circuits the code half (lCode = 0)
to avoid touching `gcExp` arithmetic.

`LShlSat` is the saturating left shift already provided by
`internal/fixed` (`fixed.LShl` saturates per ITU spec — verify the
existing primitive's semantics in IMPL-2 before relying on it; otherwise
add an explicit saturation).

### `internal/gainquant`

Encoder `Apply` returns the same triple. `Quantize` is unchanged
(operates on real-valued energies before format collapse). The
closed-loop gain candidate evaluation must consume the triple when it
needs the linear g_c; intermediate energies stay in Word32.

### Call sites

- `internal/decoder/subframe.go` (or equivalent): receives triple,
  forwards to `BuildExcitation`.
- `encoder.go fcbStep` / `closedloopStep`: receives triple from
  `gainquant.Apply`, forwards.

## 3. Rejected alternatives

- **(b) Word32 Q16 in int32** — uniform but wastes 16 bits of mantissa
  precision in the typical range and forces a 16-bit downshift in
  BuildExcitation that loses the same precision option (a) preserves.
- **(c) keep gc0 + γ̂_c separate until BuildExcitation** — pushes the
  log-domain → linear conversion downstream, ballooning BuildExcitation
  arithmetic and duplicating the pow2 lookup. Breaks the existing clean
  separation between `gain.Decode` (log-domain reconstruction) and
  `synth.BuildExcitation` (linear-domain combine).

## 4. Q-format alignment validation (REF-1 reasoning, not code)

Worst-case `prod32 = LMult(gcMantQ14, c[n])`:
- `gcMantQ14` ≤ 32767 (Q14 < 2.0)
- `c[n]` ≤ 8192 (Q13, ±1.0 unit pulse)
- `LMult` returns `2 · a · b` in int32 ⇒ `prod32` ≤ 2 · 32767 · 8192
  ≈ 5.37 × 10⁸, well within int32 (max 2.15 × 10⁹).

Worst-case `gcExp` from DIAG-1 corpus:
- `g_c` max ≈ 159, ≈ 2^7.31 → integer exponent ≤ 8
- `g_c` min ≈ 0.001 (zero-energy guard handles 0; Salami §V.B notes
  unvoiced lower bound around −60 dB ≈ 2^-10 of unit RMS) → integer
  exponent ≥ -15

⇒ `gcExp ∈ [-15, +9]` is sufficient with margin.

`shift = 13 - gcExp`:
- `gcExp = +9`  ⇒ shift = +4  (right shift, fine)
- `gcExp = -15` ⇒ shift = +28 (right shift, lCode → 0, correct)
- `gcExp = +15` (defensive)  ⇒ shift = -2 (left shift; saturate)

The left-shift case happens only outside the corpus range; saturation
at that bound is acceptable and rare. IMPL-2 must include a unit test
for the left-shift saturation path to lock the contract.

## 5. Spec citations

- ITU-T G.729 (06/2012) §3.9.2 eq. (74) — predicted log-gain Ê(m)
  from MA-predicted past errors.
- Eq. (75) — `g_c = γ̂_c · g_c0`, `g_c0 = 2^Ê(m) / 10·log10(40) ⋯`
  decomposition.
- Salami 1998 IEEE T-SAP "Design and Description of CS-ACELP …" §V.B
  paragraphs on conjugate-structure VQ and quantized-gain dynamic
  range.
- Kondoz 2nd ed. ch. 6 (CS-ACELP) on the mantissa+exponent
  reconstruction of g_c.

(No external G.729 implementation referenced.)

## 6. Disposition

OQ-GCREP: **PINNED → (a) mantissa Q14 + exponent int8.**
OQ-EXC-Q: **PINNED → u stays Q0 Word16, BuildExcitation aligns via the
shift derived from `gcExp` per §2.**
OQ-BWIDTH: **PROVISIONAL → gcExp ∈ [-15, +9]** (SPEECH.BIT corpus).
DIAG-2 (encoder) must confirm on PITCH/ALGTHM corpora; if exceeded,
widen to int8 max [-128, +127] which the type already permits at no
storage cost.

REF-1 closed. IMPL-1 may proceed.
