# Decoder amplitude-collapse localization (Phase 3 entry diagnostic)

Date: 2026-05-04
Scope: pipeline B failure surfaced by `phase3_roundtrip_quality_test.go`
       — ourDecoder driven by ITU `SPEECH.BIT` produces RMS = 65,
       max |sample| = 850 over 3 750 frames, vs ITU PST reference
       RMS ≈ 2 095. SegSNR of pipeline B is −0.46 dB (essentially noise).

This document records the **localization** result; it does not prescribe
production fixes. A separate dispatch is responsible for any production
code change.

I1 status: clean-room. No external G.729 implementation was consulted.
References used were the ITU-T G.729 / Annex A spec PDFs already in
`docs/itu/` plus the in-repo source.

## 1. Method

Three diagnostic Go test files were added (informational `t.Logf` only,
no `t.Errorf`):

| # | File                                                                   | Package         | Purpose                                                                       |
|---|------------------------------------------------------------------------|-----------------|-------------------------------------------------------------------------------|
| 1 | `phase3diag_01_unpacked_indices_test.go`                               | `g729` (root)   | Dump first 10 frames of `SPEECH.BIT` and corpus-wide range stats per field    |
| 2 | `internal/decoder/phase3diag_02_excitation_rms_test.go`                | `decoder`       | Per-subframe RMS of v[], c[], u[]; gp/gc distributions; T_int histogram       |
| 3 | `internal/decoder/phase3diag_03_synthesis_bypass_test.go`              | `decoder`       | Per-subframe RMS at u → s → sPf → hp → out, plus bypass-postfilter comparison |

A test-only helper file exposes a tap-collecting `DecodeWithTaps` mirror
of `Decode` so the per-stage signals can be observed without touching
production decode/subframe code:

| File | Package | Purpose |
|---|---|---|
| `internal/decoder/phase3diag_taps_export_test.go` | `decoder` | Test-only `Decoder.DecodeWithTaps` returning `Phase3DiagFrameTaps` (v, c, u, s, sPf, hp, gp, gc, T per subframe; 80-sample post-`ScaleUpSat` output) |

All four files run as part of `go test -run TestPhase3Diag_ -v ./...`.

## 2. Verified-healthy sub-systems

### 2.1 G.192 → packed → bitstream.Unpack

Diagnostic 01 confirms every field of `bitstream.Frame` covers its full
permitted range across all 3 750 SPEECH.BIT frames:

```
L0  observed [0, 1]      permitted [0, 1]
L1  observed [0, 127]    permitted [0, 127]
L2  observed [0, 31]     permitted [0, 31]
L3  observed [0, 31]     permitted [0, 31]
P1  observed [1, 255]    permitted [0, 255]
P0  observed [0, 1]      permitted [0, 1]
C1  observed [0, 8191]   permitted [0, 8191]
S1  observed [0, 15]     permitted [0, 15]
GA1 observed [0, 7]      permitted [0, 7]
GB1 observed [0, 15]     permitted [0, 15]
P2  observed [1, 30]     permitted [0, 31]
C2  observed [0, 8191]   permitted [0, 8191]
S2  observed [0, 15]     permitted [0, 15]
GA2 observed [0, 7]      permitted [0, 7]
GB2 observed [0, 15]     permitted [0, 15]
```

The G.729 §4 / Annex A Table 8 transmission order is preserved. No bit-
slip / no swapped-field defect upstream of the decoder.

### 2.2 Pitch lag T

Diagnostic 02 shows a plausible voiced-speech distribution: the top T_int
bin is 20 (only 386 occurrences out of 7 500 subframes, ~5 %), with a
broad mode in the 28–38 range. No degenerate "always 20" or "always
143" pattern.

### 2.3 Adaptive codebook output v[]

`rms(v) avg = 5.18` (Q0) over 7 500 subframes. Combined with healthy
gp the AC contribution to the excitation is small, as expected at
codec start (past-excitation FIFO is still warming up); v will grow
once u stops being small (it is currently small for the reason
identified in §3, so v rms is consistent with — not the cause of —
the collapse).

### 2.4 Fixed-codebook vector c[]

`rms(c) avg = 2682.41` in **Q13** ⇒ unitless RMS ≈ 0.327. This matches
the algebraic-codebook nominal energy: 4 unit pulses, RMS = √(4/40) ≈
0.316. Q-format and pulse construction are correct.

### 2.5 Adaptive-codebook gain g_p

```
gpQ14: min=827  max=22215  mean=10981.7   (Q14, unity = 16384)
       ≈ min 0.05, max 1.36, mean 0.67
```

Ranges and mean match ITU-T G.729 §3.9 / §4.1.6 (g_p ∈ [0.0, 1.2] with
some saturation room) and Salami 1998 voiced-speech reports. Healthy.

### 2.6 LP synthesis filter 1/Â(z)

Diagnostic 03 measures `rms(u) → rms(s)` amplification:

```
rms(u) =  5.29
rms(s) = 32.85    ⇒ +6.21× amplification
```

That is exactly the gain expected from a typical voiced-speech LP
filter (10 dB ≈ 3.16× to 18 dB ≈ 8× formant boost; on average ~6×
across voiced+unvoiced is unremarkable). The synthesis filter is
working — it just has a very small input.

### 2.7 Annex A postfilter and HP filter

```
rms(s)   = 32.85
rms(sPf) = 33.42   (postfilter Δ ≈ +1.7 %)
rms(hp)  = 32.51   (HP-filter Δ ≈ −2.7 %)
```

Negligible amplitude effect, as expected from a unit-AGC postfilter
followed by a 100 Hz HP. Neither stage is destroying amplitude.

### 2.8 Final ×2 scale (`pcm.ScaleUpSat`)

```
rms(out, ×2)     = 65.03   final
rms(2·s, bypass) = 65.71   ScaleUpSat(s) directly, postfilter+HP bypassed
```

The factor-of-two output scaling is correctly applied (32.51 × 2 ≈
65.02). The bypass test also confirms the postfilter+HP combined
contribution to amplitude is near zero (65.03 vs 65.71). The amplitude
collapse is therefore **upstream of `s`**, i.e. it lives in `u`, the
total excitation.

## 3. Suspected sub-system: gain-VQ decoder (g_c reconstruction)

### 3.1 Numerical evidence — corpus aggregate

```
rms(u)            =  5.29   excitation (Q0)
rms(s)            = 32.85   post 1/Â(z)
gpQ14 mean       = 10981.7  ≈ 0.67
gcQ12 min/max/mean = -32768 / +32767 / 1800.1   (Q12; unity = 4096)
```

Two independent red flags in the gc distribution:

1. **gcQ12 saturates int16 at both extrema** — i.e. the Q-format chosen
   for the decoded `g_c` (Q12, max representable ≈ +7.999) cannot hold
   the actual gain magnitude for many voiced-speech subframes.
2. **gcQ12 takes negative values** — a magnitude-only quantity (the
   fixed-codebook gain is non-negative by spec, ITU-T G.729 §3.9.2 /
   §4.1.6) reaching −32 768 indicates the int16 result is the wrap-
   around of a positive overflow somewhere in the Q14 intermediate
   `gc0Q14` chain (`gain.decode.go` builds `gc0Q14 = pow2Fixed(...)`,
   then `gcQ12 = (γ̂_c · gc0Q14) >> 15`; if `gc0Q14` itself wraps
   above +32 767 it lands at a negative int16, propagating sign into
   `gcQ12`).

### 3.2 Per-subframe corroboration (frames 3 … 7, both subframes)

```
frame sf  Tint Tfrac gpQ14  gcQ12   vRMS    cRMS    uRMS
  3    1    24    +1   8092  32767   1.40  2590.54   2.59
  3    2    24    -1   6161  32767   2.69  2743.97   2.85
  4    1   111     0  17300  32767   1.55  2590.54   2.53
  4    2   110    -1  17839  32767   2.98  2590.54   3.97
  5    1    34    +1   3915  32767   3.47  2790.06   2.54
  5    2    37    -1   7711 -32768   2.50  2590.54   2.83
  6    1    33    +1  18403 -32768   2.66  2661.28   3.51
  6    2    35    -1   7711 -32768   3.19  2790.06   2.67
  7    1    35    +1  20524  32767   3.06  2590.54   4.61
  7    2    36    -1  17943  32767   3.70  2790.12   4.95
```

`gcQ12` saturates positive or negative on every single subframe in this
window, in voiced speech where the *actual* g_c should be a moderate
positive number. The result: `synth.BuildExcitation` shifts a value
that, after the deliberate `>>11` re-scaling in
`internal/synth/excitation.go`, contributes only O(1) to u — not the
O(100) it should. The total excitation is therefore dominated by
`gp · v`, which itself is small at codec start because of the same gc
collapse (stale, near-silent past-excitation FIFO).

### 3.3 Amplitude budget reconciliation

Working forward from observed numbers:

```
rms(u)   ≈ 5.3           (corpus average)
rms(s)   ≈ 5.3 × 6.2 ≈ 33   (LP filter amplification, ×6.2 measured)
rms(out) ≈ 33   × 2.0 ≈ 65   (ScaleUpSat ×2)
```

This matches the symptom (RMS = 65, max 850) **exactly** to the
arithmetic precision of the corpus average. The amplitude shortfall is
~33× (target 2 095, actual 65). All of that shortfall is concentrated
in the `u` stage; downstream stages have unit gain (within ±5 %).

## 4. Top root-cause candidates

### Candidate A (highest confidence): wrong Q-format envelope for the decoded fixed-codebook gain

`internal/gain/decode.go` returns `gcQ12` as a single Q12 `int16`. Q12
saturates at ±7.999 — but the spec's `g_c` covers a much wider dynamic
range (typically up to ~10 000 in linear amplitude for voiced
fortissimo, and down to ~10⁻³ for low-energy unvoiced). A single
fixed-Q `int16` cannot represent both ends of that range. The spec
representation is a mantissa+exponent (or equivalently a separate
Q-shift returned alongside the mantissa) so that the
`synth.BuildExcitation` step can apply the appropriate per-subframe
shift before saturating. Our decoder collapses the entire range into
one Q12 word, which:

- **wraps at the top end** (positive overflow → negative int16) — visible
  as gcQ12 = −32768 on voiced subframes;
- **truncates at the bottom end** — silent unvoiced subframes get gcQ12
  rounded to small magnitudes, which after the `>>11` in
  BuildExcitation contribute essentially nothing.

The interplay with the >>11 in `BuildExcitation` (`LShr(LMult(gcQ12,
c[n]), 11)`) is consistent with a Q-format that should be *narrower*
than Q12 (e.g. Q3 with an explicit shift), or with `g_c` returned as
a (mantissa Q14, exponent) pair. Either of those would close the
33× amplitude gap.

**Verification plan A:**

1. Extend the diagnostic to also dump `predicted` (Ê) and `gc0Q14` from
   `gain.Decoder.Decode` (test-only export hook).
2. Confirm `gc0Q14` itself wraps to negative on the same subframes
   where `gcQ12` saturates.
3. Cross-check against ITU-T G.729 §3.9.2 eq. (74)/(75) on the
   declared dynamic range of `g_c` and the spec's mantissa/exponent
   representation; pin the verified Q-format expectation in a clean-
   room textbook citation (Salami 1998 §V.B; Kondoz §6 on CS-ACELP
   gain quantization).
4. Build a corrected gc-reconstruction prototype in a *test-only*
   shadow function and re-run the full-corpus test 03 to confirm
   `rms(u)` rises to O(150–300) and `rms(out)` reaches O(2000).

### Candidate B (medium confidence): MA gain predictor cold-start producing wrong predicted log-gain

`gain.Decoder` initialises `pastErrors` to `pastErrorsDefault = −14336`
(−14 dB Q10). The combination with the (suspected wrong) Q-format
in candidate A produces a feedback loop: a tiny gc means the
log-correction error U(m) inserted into the FIFO is a large negative
value, which biases future predictions toward smaller gc, which
biases future U(m) further negative, etc. This is a symptom not a
cause, but if candidate A is fixed and the problem persists this
should be the next pin.

**Verification plan B:** dump `pastErrors` evolution per subframe; if
trajectories quickly settle to `[−14336, −14336, −14336, −14336]` after
a fix to candidate A then this is fully a downstream effect.

### Candidate C (lower confidence): LP-coefficient Q-format mismatch into 1/Â(z)

The synthesis filter shows healthy ≈6× amplification, which suggests
the LP coefficient pipeline (lsp.Decoder → sf*A) is reasonably correct.
However, a wrong-magnitude Â could also produce a small `s`. We expect
this not to be the issue because:

- Pipeline A (`SPEECH.PST`) achieves SegSNR 4.35 dB so the codec is
  intelligible (i.e. the signal model is right).
- Phase 1o D-3 already pinned that `lsp.LSPToLP`, lsp interpolation,
  and the synthesis filter individually pass byte-EQ on the test
  vectors used (see `internal/decoder/itu_vector_pstdomain_test.go`).

**Verification plan C** (only if A and B do not close the gap): inject
synthetic excitations of known RMS and confirm `s` RMS scales linearly
with input — already implicitly tested by diag 03 since the 6× ratio
is itself consistent across thousands of subframes.

## 5. List of added diagnostic test files

```
phase3diag_01_unpacked_indices_test.go                     (root g729)
internal/decoder/phase3diag_taps_export_test.go            (test-only export shim)
internal/decoder/phase3diag_02_excitation_rms_test.go      (decoder)
internal/decoder/phase3diag_03_synthesis_bypass_test.go    (decoder)
docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md
```

No production code touched. `go vet ./...` and `go build ./...` clean.

The two pre-existing untracked phase 3 entry tests
(`phase3_roundtrip_quality_test.go`,
`phase3_roundtrip_quality_diag_test.go`) are committed unchanged to
preserve the diagnostic baseline these new tests build upon.
