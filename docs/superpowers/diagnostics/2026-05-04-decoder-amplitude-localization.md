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

## 6. Appendix A — Phase 3a DIAG-1 unsaturated tap distribution (date 2026-05-04)

Source: `internal/decoder/phase3a_diag1_gc_taps_test.go`
Driver:  `(*gain.Decoder).DecodeWithFullTaps` shim
         (`internal/gain/phase3a_diag1_export.go`, test-only diagnostic
         helper — mirror of `Decode` returning the unsaturated 32-bit
         intermediates; production `decode.go` / `pow2.go` unchanged.)
Corpus:  `testdata/itu/G729_Release3/g729AnnexA/test_vectors/SPEECH.BIT`
         3750 frames × 2 subframes = 7500 subframes
Date:    2026-05-04

### 6.1 Aggregate distribution

| tap                  | unit       | min     | max      | mean       |
|----------------------|------------|---------|----------|------------|
| `predicted`          | Q10 dB     | 3855    | 32767    | 25049.8    |
| `ecBarDbQ10`         | Q10 dB     | -11462  | -7434    | -9963.8    |
| `log2GcQ10`          | Q10        | 2252    | 5443     | 4905.9     |
| `gc0Q14_unsat`       | int32 @Q14 | 75244   | 652416   | 525722.7   |
| `prodQ12_unsat`      | int32 @Q12 | 3627    | 652396   | 156258.6   |

Wrap counts (subframes whose unsaturated tap exceeds the int16
envelope `[-32768, 32767]`):

| tap                  | wrap count    | wrap % |
|----------------------|---------------|--------|
| `gc0Q14_unsat`       | 7500 / 7500   | 100.00 |
| `prodQ12_unsat`      | 5811 / 7500   |  77.48 |
| zero-energy guard    | 0 / 7500      |   0.00 |

### 6.2 First 10 subframes (raw dump)

```
frame  sf     pred    ecBar   log2Gc    gc0Q14_un   prodQ12_un    gpQ14    gcQ12   gammaC
    0   1     5060   -10068     2513        89784         4153     1995     4153     1516
    0   2     4602   -10240     2465        86908         5590     5143     5590     2108
    1   1     6206   -10240     2732       104128        19759    13400    19759     6218
    1   2    14220   -10240     4063       256352        16491     5143    16491     2108
    2   1    14126   -10135     4030       250688        16127     5143    16127     2108
    2   2    12374    -9319     3603       187768        17288     6693    17288     3017
    3   1    13101   -10240     3877       226032        43988     8092    32767     6377
    3   2    17648    -9731     4548       355968        42486     6161    32767     3911
    4   1    19635   -10240     4962       471120        48552    17300    32767     3377
    4   2    19093   -10240     4872       443280       443266    17839    32767    32767
```

(`gpQ14` / `gcQ12` are the post-saturation values currently returned
by `Decode`; `gcQ12` already pegs to 32767 by frame 3.)

### 6.3 Spec-grounded interpretation

ITU-T G.729 §3.9.2 expresses the fixed-codebook gain as

  g_c   = γ̂_c · g_c0                                   (eq. 74)
  g_c0  = 10^( (Ê(m) − E̅_c)/20 ) = 2^( (Ê(m) − E̅_c)/(20·log10 2) )   (eq. 75)

with γ̂_c ∈ ~[0, ~2] in Q13 and g_c0 the predicted-energy gain target.
Salami 1998 §V.B (pp. 137–140) describes the conjugate-structure
mantissa-+-correction reconstruction, and Kondoz §6 (gain VQ) confirms
that the predicted-gain magnitude can substantially exceed unity for
voiced/transition subframes, so its representation is not bounded by
the codeword range alone.

The corpus numbers concur:

* `gc0Q14_unsat` is **100 %** outside int16 at Q14. Its true value
  ranges roughly 4.6 ≤ g_c0 ≤ 39.8 (75244 / 2^14 to 652416 / 2^14),
  with mean ≈ 32.1. No int16-Q14 packing can hold this tap.
* The product `γ̂_c · g_c0` at Q12 wraps int16 in **77.5 %** of all
  subframes, with peaks at 652396 (≈ 159.2 in unscaled units) — the
  full-pipeline `gcQ12` is therefore *systematically* clipped, which
  is exactly the amplitude-collapse symptom this diagnostic chain is
  tracking (cross-ref §3, where total excitation `u` underwhelms the
  spec by ≈ 6×).
* All taps fit comfortably within int32 (max ≈ 6.5·10⁵, well below
  2³¹ ≈ 2.1·10⁹).

### 6.4 OQ-GCREP pin

**Pin: (a) mantissa Q14 + exponent int8** (the plan's strong default).

Justification (corpus + spec):

The natural decomposition produced by eq. (75) and implemented in
`pow2Fixed` already splits log2 g_c0 into an integer part (the
exponent of the binary point shift) and a fractional part interpolated
through `tables.Pow2Table` to give a Q14 mantissa in [1.0, 2.0). The
DIAG-1 distribution shows g_c0 magnitudes of ~4–40 (i.e. exponents 2–6
above unity) and the post-multiply g_c spans ~1–160 — both safely
representable as `(mantQ14 int16) · 2^(exp int8)` with `exp` ∈ [-15,
+8]. This keeps every existing Q14 hot-path stage (LMul / LAdd)
operating on a 16-bit mantissa (no Q-format ripple beyond the
gain→excitation handoff), while the int8 exponent absorbs the dynamic
range that today causes the int16 saturation visible in §6.1's wrap
table. Options (b) Word32-Q16 and (c) two-level (gc0 + γ̂_c kept
separate) both work numerically but force either a wider arithmetic
type or an extra multiply at every consumer of `g_c`; the spec's own
2^x reconstruction makes (a) the cheapest match.

OQ-BWIDTH (the dynamic-range bracket): the corpus puts
|g_c0|·2^14 ≤ 6.6·10⁵ and |g_c|·2^12 ≤ 6.6·10⁵, so an int8 exponent
range of [-15, +8] (worst-case g_c0 ≈ 256, g_c ≈ 256) is sufficient
with margin; the encoder side (DIAG-2) must confirm the same bracket
holds on PITCH/ALGTHM corpora before REF-1 freezes the API.

### 6.5 Spec citations (clean-room, no external implementations)

* ITU-T G.729 (06/2012) §3.9, §3.9.1 eq. (69)–(73) — MA log-gain
  predictor.
* ITU-T G.729 (06/2012) §3.9.2 eq. (74)–(75) — g_c = γ̂_c · g_c0,
  g_c0 = 2^((Ê(m)−E̅_c)/(20·log10 2)).
* ITU-T G.729 (06/2012) §4.1.6 — decoder excitation reconstruction.
* Salami, R. et al., "Design and Description of CS-ACELP: A Toll
  Quality 8 kb/s Speech Coder," IEEE Trans. Speech Audio Proc., vol.
  6 no. 2 (March 1998), §V.B pp. 137–140 — gain quantization
  mantissa-correction structure.
* Kondoz, A. M., *Digital Speech: Coding for Low Bit Rate
  Communication Systems*, 2nd ed., Wiley 2004, §6 — CS-ACELP gain VQ
  representation.

No external G.729 implementation (ITU C reference, bcg729, Sipro
Lab, FFmpeg, …) was consulted in producing this section.
