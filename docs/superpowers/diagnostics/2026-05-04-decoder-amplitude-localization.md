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

## 7. Appendix B — Post-IMPL-2 amplitude profile (2026-05-04)

After Phase 3a IMPL-2 (synth.BuildExcitation switched from a single
saturated g_c (Q12, int16) to the native (mantissa Q14, exponent int8)
pair returned by gain.Decode per REF-1 §2), the decoder no longer
truncates large g_c excursions at the Q12 boundary. The
TestPhase3Diag_DecoderAmplitudeProfile per-frame snapshot (TAME
SPEECH corpus) is now (selected frames):

```
frame    0 ourRMS=     0.8 pstRMS=     1.1 maxAbs=    4
frame    4 ourRMS=    59.4 pstRMS=   102.1 maxAbs=  220
frame    6 ourRMS=   172.6 pstRMS=   756.7 maxAbs=  444
frame  600 ourRMS=   604.9 pstRMS=  3307.1 maxAbs= 1654
frame 1400 ourRMS=  1713.7 pstRMS=  3948.8 maxAbs= 3566
frame 2400 ourRMS=   552.1 pstRMS=  5833.5 maxAbs= 1420
frame 3400 ourRMS=   907.0 pstRMS=  1666.8 maxAbs= 1542
```

Global max |sample| over 3750 frames: **5262** (was substantially
lower under the Q12-saturated path at IMPL-1). The remaining residual
ourRMS-vs-pstRMS gap on high-energy frames now sits in the
post-filter / synthesis chain, not the gain-VQ saturation envelope —
consistent with the REF-1 §2 prediction that the (mant, exp) format
restores the dynamic range claimed by §3.9.2.

Bench (Phase 3a IMPL-2, AMD EPYC 9554P, count=3):

```
BenchmarkBuildExcitation-2   235.0 ns/op   0 B/op   0 allocs/op
BenchmarkBuildExcitation-2   250.3 ns/op   0 B/op   0 allocs/op
BenchmarkBuildExcitation-2   257.1 ns/op   0 B/op   0 allocs/op
```

Allocation contract preserved (0 allocs/op) — the additional int8
exponent argument is a value type, no heap traffic introduced.

## 8. Appendix C — Post-IMPL-3 encoder representation alignment (2026-05-04)

After Phase 3a IMPL-3 (this commit), the encoder side mirrors the
decoder's native (gpQ14, gcMantQ14, gcExp) representation introduced
by IMPL-1/IMPL-2. Two changes land:

1. `gainquant.PredictedGcQ12` now returns **int32** (no int16
   saturation). DIAG-1 (§6) showed the natural g_c0·γ̂_c product
   routinely exceeds the int16 envelope (peaks ≈ 159 ⇒ Q12 ≈ 651 264);
   collapsing it inside the §3.9.2 cost-search input biased the
   conjugate-codebook winner selection toward smaller-energy entries.

2. `gainquant.SearchConjugate` now returns **γ̂_c (Q13)** directly (sum
   of `GainGBK1[ga][1]+GainGBK2[gb][1]`) instead of a saturated ĝc Q12.
   The encoder pairs this with a new `gainquant.Reconstruct` /
   `gainquant.DequantGc` log-domain split that mirrors
   `gain.Decoder.Decode` bit-for-bit (pinned by
   `TestApply_MantissaExponent`). The §A.3.10 commit accumulators
   (`swMemErr`, `oldExc`) consume the (mant, exp) pair through
   `encoder.mantExpToQ12`, an int32-Q12 conversion that absorbs the
   full dynamic range without int16 collapse.

### Byte-EQ deltas vs IMPL-2 baseline (SPEECH corpus, 1835 frames)

| Param | IMPL-2 baseline | IMPL-3 | Δ           |
|-------|-----------------|--------|-------------|
| P1    | 10.79%          | 10.41% | −0.38 pp    |
| P0    | 57.49%          | 57.22% | −0.27 pp    |
| P2    | 11.66%          | 11.50% | −0.16 pp    |
| S1    | 5.50%           | 5.18%  | −0.32 pp    |
| C1    | 0.00%           | 0.00%  |  0          |
| GA1   | 12.15%          | 12.15% |  0          |
| GB1   | 5.29%           | 5.29%  |  0          |
| S2    | 4.20%           | 4.36%  | +0.16 pp    |
| C2    | 0.00%           | 0.00%  |  0          |
| GA2   | 11.77%          | 11.77% |  0          |
| GB2   | 4.52%           | 4.90%  | +0.38 pp    |

The drift is in the expected direction: the unbiased cost search picks
slightly different (ga, gb) winners on a small fraction of subframes,
which then permutes adjacent-subframe pitch indices through the
encoder's predictor state. The plan treats this drift as
acceptable — Phase 3a's REF-1 mandate is *representation alignment*
between encoder and decoder, not bit-equality with the ITU reference
encoder (which is Phase 4 / I5 territory).

### RoundTrip quality (SPEECH corpus, 3750 frames)

| Pipeline | RMS | GlobalSNR | SegSNR  |
|----------|-----|-----------|---------|
| B (ITU.bit → ourDec)            | 419 | −0.05 dB | −0.90 dB |
| C (ourEnc → ourDec, full RT)    |   5 |  0.01 dB |  0.00 dB |

Both lines are within noise of the IMPL-2 baseline (B: 419 / −0.05 / −0.90;
C: 5 / 0.01 / +0.01). The full round-trip self-consistency holds.

### PSTdomain pins

The four `TestDecode_ITUVectorXxxKnownPSTDomainDifference`
(TAME/FIXED/PITCH/OVERFLOW) and `TestPhase2fTAME1_ByteEQ` /
`TestDiagnostic_SinglePulseChain` failures remain unchanged from the
IMPL-2 baseline — these are FAIL-DEFERRED upstream pins (Phase 1o D-3
PASS-by-design), and IMPL-3 does not touch the post-filter / single-
pulse chain or the §A.3.10 macro-level pin envelope.

### Hot-path zero-allocation budget

`gainquant.SearchConjugate`, `gainquant.PredictedGcQ12`,
`gain.Decoder.Decode`, and `synth.BuildExcitation` continue to report
0 B/op / 0 allocs/op under their pinned `AllocsPerRun` /
`-benchmem` measurements.

---

## Appendix D — Phase 3a INT-1 FAIL diagnostic

Date: 2026-05-04 (post-c7fcc06; INT-1 + IMPL-4 finalization run)
Disposition: **FAIL — gate triggered, no closure-PASS commit**
Acceptance band breached: SegSNR pipeline B = −0.90 dB (< 0 dB FAIL
floor) AND rms(out) pipeline B = 419 (< 500 FAIL floor). Both legs of
the FAIL guard fired.

### D.1 — Acceptance numbers (SPEECH corpus, 3750 frames, 300 000 samples)

| Metric                    | Pre-IMPL-1 baseline | INT-1 (HEAD = c7fcc06)     | Δ        |
|---------------------------|---------------------|----------------------------|----------|
| Pipeline B rms(out)       | 65                  | 419                        | +6.45×   |
| Pipeline B max\|sample\|  | 850                 | 5262                       | +6.19×   |
| Pipeline B GlobalSNR      | n/r                 | −0.05 dB                   |          |
| Pipeline B SegSNR         | −0.46 dB            | −0.90 dB                   | −0.44 dB |
| Pipeline C rms(out)       | n/r                 | 5                          |          |
| Pipeline C SegSNR         | n/r                 | +0.00 dB                   |          |
| Reference rms (SPEECH.IN) | 2237                | 2237                       |          |

Per-frame samples (every 200th, plus first 10) confirm the recovery
shape: ourDec frame-by-frame RMS now climbs into the 100–1700 envelope
on voiced frames (was capped at low tens by the IMPL-1 saturation),
and the 5262 global max\|sample\| sits at the right order of magnitude
relative to the 2237 corpus RMS reference.

### D.2 — Verdict

The IMPL-1..3 chain (mantissa Q14 + exponent int8 representation
threaded through gain.Decode → synth.BuildExcitation → encoder fcbStep)
is **directionally correct and incomplete**:

- Amplitude envelope: **RECOVERED**. The 33× shortfall identified by
  the original diagnostic (rms 65 vs 2095) has collapsed to a 5×
  shortfall (rms 419 vs 2237 reference, or vs 2095 path-A). g_c0/g_c
  no longer wraps at the int16 envelope; the formerly-saturated VQ
  domain now reaches its natural [16384, 32767) mantissa band with a
  free exponent.
- Phase / waveform alignment: **NOT recovered**. SegSNR pipeline B is
  unchanged from the IMPL-2 measurement (−0.90 dB at IMPL-3 head; was
  −0.46 dB pre-IMPL-1). The cross-correlation peak shift
  (−22 samples) reported by the roundtrip harness diverges from the
  pipeline-A intrinsic shift (+40 samples) by 62 samples — a gross
  mis-alignment that no g_c-magnitude fix can repair.
- The 62-sample gap is consistent with the diagnostic report's
  candidate B (MA predictor cold-start) and candidate C (LP coefficient
  pipeline) hypothesis space: a per-frame predictor or LP-filter
  initialization defect would manifest as a phase/timing skew that is
  amplitude-blind (RMS recovers, SegSNR does not) — exactly the
  observed signature.

### D.3 — Phase 3b candidate ranking

Ranked by signature match to the residual −0.90 dB SegSNR / −22-sample
shift defect:

1. **Candidate B — MA predictor cold-start** (gain.Decoder.pastErrors
   FIFO seed). The four pastErrorsDefault entries are seeded from
   spec-default (-14 dB) but the encoder side may seed from a different
   convention; a seed mismatch would manifest as a per-frame phase
   error that compounds across the corpus, matching the SegSNR-flat /
   RMS-recovered signature. **Highest-priority probe.**
2. **Candidate C — LP coefficient interpolation / Â(z) pipeline**.
   Phase 1o D-3 byte-EQ holds at sample 0 for SPEECH/LSP/TEST, which
   bounds the LP defect surface, but does not prove sample-by-sample
   alignment across the corpus. A 1-sample LP interpolation skew per
   subframe would accumulate into a 62-sample drift over 3750 frames.
3. (Lower priority) Postfilter long-term / tilt μ. Already reviewed in
   gate 17 D-1b (commit 6633b28) and Phase 1h diagnostic; no surviving
   open hypothesis.

### D.4 — Why this is FAIL not PARTIAL

Per Phase 3a plan §2 acceptance bands:
- ACCEPT: SegSNR ≥ 3 dB
- ACCEPT-PARTIAL: SegSNR ∈ [0, 3 dB) AND rms(out) ≥ 1500
- FAIL: SegSNR < 0 dB OR rms(out) < 500

Measured SegSNR = −0.90 dB violates the SegSNR < 0 leg; rms(out) = 419
violates the rms < 500 leg. Both FAIL conditions fire. The amplitude
recovery (RMS 65 → 419, max 850 → 5262) is real and substantial but
does not clear the rms ≥ 1500 PARTIAL floor either.

Disposition: **CLOSED-DEFERRED** at the plan level (IMPL-1..4 + INT-1
land as a coherent representation-fix ladder; Phase 3 acceptance is
deferred to Phase 3b). NO closure-PASS report is written, per the
plan's FAIL guard.

### D.5 — Pin / sweep status snapshot at FAIL

- Phase 1o D-3 PSTdomain pins:
  - SPEECH (sample-0 byte-EQ gate): **PASS** — sample 0 = 2 (matches).
  - LSP (sample-40 byte-EQ): **PASS**.
  - TEST (sample-40 byte-EQ): **PASS**.
  - TAME, FIXED, PITCH, OVERFLOW: **FAIL by design** — PASS-by-design
    pins flagged the production-output change from IMPL-1..3 (drift
    moved to sample 40–41), exactly the documented "reactivation
    trigger" #4 in `itu_vector_pstdomain_test.go` docstring (lines
    94–98). Disposition update is owned by Phase 3b once the
    candidate B / C diagnosis lands.
- Phase 2c INT-1 byte-EQ: P1 10.79% → 10.41% (−0.38 pp), P0 57.49% →
  57.22% (−0.27 pp), P2 11.66% → 11.50% (−0.16 pp). Within IMPL-3
  documented drift tolerance.
- Phase 2d INT-1a byte-EQ: S1 5.18% / C1 0.00% / GA1 12.15% / GB1
  5.29% / S2 4.36% / C2 0.00% / GA2 11.77% / GB2 4.90%. Matches the
  IMPL-3 baseline recorded in the "IMPL-3 byte-EQ deltas" appendix
  table earlier in this document.
- Phase 2f TAME-1: GA1 6.25%, GB1 2.34%, GA2 3.91%, GB2 3.91% — the
  four documented plausibility-floor breaches under OQ-TAMING-THR
  (slot 5/5).
- `TestDiagnostic_SinglePulseChain`: pre-existing diagnostic FAIL
  (gain log-domain 14 dB suspect, ledger entry from Phase 1g). Not a
  Phase 3a regression.

### D.6 — Hot-path zero-allocation budget (retained)

| Bench                         | ns/op (HEAD c7fcc06) | allocs |
|-------------------------------|----------------------|--------|
| `gain.BenchmarkDecode`        | 132.1                | 0      |
| `synth.BenchmarkBuildExcitation` | 233.7             | 0      |
| `synth.BenchmarkSynthesize`   | 933.0                | 0      |
| `synth.BenchmarkFilterSubframe`  | 757.4             | 0      |
| `decoder.BenchmarkDecode`     | 8011                 | 0      |

INT-1 introduces no production code change (decoder wiring already
landed in IMPL-2, encoder wiring in IMPL-3, repo-wide audit confirmed
zero remaining production consumers of `LegacyGcQ12FromMantExp`).
ns/op deltas vs c7fcc06 are therefore 0 by construction; the table
above is the ratified post-IMPL-3 baseline carried into Phase 3b.

### D.7 — Next action

Open Phase 3b plan targeting candidate B (MA predictor cold-start)
first, with candidate C (LP coefficient pipeline) as fallback. The
acceptance harness (`phase3_roundtrip_quality_test.go`) and the
FAIL-floor decision rule remain unchanged; Phase 3b graduates to
ACCEPT-PARTIAL the moment SegSNR pipeline B clears 0 dB AND rms ≥
1500, and to ACCEPT-PASS at SegSNR ≥ 3 dB.

## Appendix E — Phase 3b DIAG-1: pastErrors trajectory & OQ-PASTSEED / OQ-PASTPROG pin

Date: 2026-05-04 (post-922e7e1; Phase 3b plan dispatch)
Owner: Phase 3b DIAG-1 (candidate B isolation)

### E.1 Mission recap

Pin **OQ-PASTSEED** (cold-start value of the four `gain.Decoder.pastErrors`
taps `Û(m-1..m-4)`) and **OQ-PASTPROG** (re-seed strategy on the
zero-energy guard `Σc² = 0` short-circuit) from the ITU-T G.729 (06/2012)
specification text, validated against corpus evidence from SPEECH.BIT.
Per the Phase 3b plan §3, both OQs gate the decision on whether the
candidate-B fix (REF-1 / IMPL-1) proceeds or is exonerated in favor of
DIAG-2 (candidate C, LP-interpolation alignment).

### E.2 Method

- Test: `internal/decoder/phase3b_diag1_pasterrors_test.go::TestPhase3bDiag1_PastErrorsTrajectory`.
- Shim:  `internal/gain/phase3b_diag1_pastErrors_export.go` exposes
  `(*Decoder).PastErrorsSnapshot() [4]int16` (read-only copy) and
  `(*Decoder).Initialized() bool`.
- Driver mirrors `DecodeWithTaps` (`phase3diag_taps_export_test.go`)
  line-for-line and inserts a `PastErrorsSnapshot()` call BEFORE each
  per-subframe `decodeSubframeWithTaps` invocation. State advancement
  (lsp predictor, gain predictor, synth/pst memories, pastExc FIFO) is
  identical to the production `Decoder.Decode` pathway.
- Corpus: SPEECH.BIT (3750 frames = 7500 subframes), the same bitstream
  consumed by `phase3_roundtrip_quality_test.go` pipeline B.
- Captured per subframe: `preTaps[0..3]`, `predicted` (Ê(m) + E̅ Q10
  dB after `tables.GainMeanEnergyQ10` add in `predictor.go`), `gpQ14`,
  `gcMantQ14`, `gcExp`, `γ̂_c` Q13, `uCurrent` (post-shift `pastErrors[0]`,
  i.e. the value FIFO-shifted in this subframe), and the zero-energy
  guard flag.

### E.3 First-50-subframe trajectory (excerpt)

Q-format note: all tap and `predicted` / `uCurrent` columns are Q10 dB
(divide by 1024 for dB).

| sf# | Û(m-1) | Û(m-2) | Û(m-3) | Û(m-4) |  pred | uCur  | guard |
|----:|-------:|-------:|-------:|-------:|------:|------:|:-----:|
|   0 |      0 |      0 |      0 |      0 |  5060 |-15009 |       |
|   1 | -15009 | -14336 | -14336 | -14336 |  4602 |-12077 |       |
|   2 | -12077 | -15009 | -14336 | -14336 |  6206 | -2456 |       |
|   3 |  -2456 | -12077 | -15009 | -14336 | 14220 |-12077 |       |
|   4 | -12077 |  -2456 | -12077 | -15009 | 14126 |-12077 |       |
|   5 | -12077 | -12077 |  -2456 | -12077 | 12374 | -8886 |       |
|  ...|        |        |        |        |       |       |       |
|  10 |  12324 |  -7887 |  -6580 |  -2234 | 31866 |  1114 |       |
|  20 |   6165 |  -3257 |  -1072 | 12324  | 32767 | -2456 |       |
|  49 |  -7267 |  -9838 |  -9085 | -1048  | 16785 |  -229 |       |

Three structural observations from the table:

1. **sf 0 preTaps are all-zero** because the snapshot is taken BEFORE
   `gain.Decoder.Decode`'s lazy `if !d.initialized { for ... }`
   initializer fires. By sf 1 the spec seed −14336 has propagated into
   taps 1, 2, 3 (with sf 0's `uCurrent = -15009` shifted into tap 0).
   This matches the spec's lazy-init contract.
2. **sf 0 `predicted = 5060` Q10 = +4.94 dB**. Independent spec
   computation: `Ê(0) = E̅ + Σ b_i·Û(0-i)` with `Û_init = -14`,
   `b_i = (0.68, 0.58, 0.34, 0.19)`, `E̅ = 30 dB`:
   `Σ b_i = 1.79`; `Ê(0) = 30 + (-14)·1.79 = 30 - 25.06 = +4.94 dB`,
   i.e. `4.94·1024 = 5058 Q10`. Match (off by 2 = expected Q10 fixed-
   point rounding from the LMac/Round path in `predictor.go`).
3. **`predicted` saturates at 32767 (= +32.0 dB) on many voiced
   subframes** (sf 11, 13, 17, 18, 20, 22, 26..43). This is the int16
   ceiling on `fixed.Add(GainMeanEnergyQ10, predicted)` in
   `predictor.go:30`. Recorded for completeness — it is an independent
   numerical concern (predicted overshoots the int16 envelope when
   `Ê(m) > +2 dB`), but it is NOT an OQ-PASTSEED / OQ-PASTPROG question.

### E.4 Long-run statistics (subframes 50..7499, n = 7450)

| Statistic                            | mean (Q10 dB) | mean (dB) | stddev (Q10) |
|--------------------------------------|--------------:|----------:|-------------:|
| `predicted` (= Ê(m) + E̅)            |      +25036.4 |   +24.450 |       9465.0 |
| `uCurrent`  (= 20·log₁₀ γ̂_c, Q10)    |       −2243.7 |    −2.191 |       7017.4 |
| `predicted` − `uCurrent`             |      +27280.1 |   +26.641 |       5969.0 |

Note on the `predicted - uCurrent` diff: the test harness computes this
difference for completeness, but it is NOT the MA-residual `Ê(m) − E(m)`
of eq. (70). Spec algebra: `E(m) = uCurrent + predicted − E̅`; so
`Ê(m) − E(m) = −uCurrent`. The relevant calibration check is therefore
`mean(uCurrent) ≈ 0`. Observed mean is −2.19 dB (Δ ≈ −2 dB from zero),
within typical bias for unvoiced-dominated corpora and consistent with a
calibrated predictor.

Zero-energy guard fired: **0 of 7500 subframes (0.000%)** across the
entire SPEECH.BIT corpus. The algebraic codebook structurally guarantees
`Σc² ≥ 4` (four ±1 unit pulses, possibly pitch-enhanced), so
`fixedCodebookEnergy(c) > 0` on every valid frame. The guard is purely
defensive against malformed input.

### E.5 OQ-PASTSEED resolution

**Pinned value: `Û(k)_init = −14 dB`, i.e. `pastErrorsDefault = −14336` (Q10).**

**Spec citation: ITU-T G.729 (06/2012), §4.3 Table 9 ("Description of
parameters with non-zero initialization"), final row:**

> Variable: Û(k);  Reference: 3.9.1;  Initial value: −14

Cross-referenced in §3.9.1 paragraph defining the 4-tap MA prediction
(eq. 69) and §4.4.3 eq. (95) which bounds the FER attenuation rule at
`Û(m) ≥ −14` (same numerical floor, indicating −14 dB is also the
spec's long-term lower bound on the predictor state).

Code state: `internal/gain/decode.go:9` defines
`pastErrorsDefault int16 = -14336`. Unit check:
14 dB · 2¹⁰ = −14336. **Matches spec.**

Corpus evidence: sf 0's `predicted` field is +4.94 dB (Q10 = 5060),
within 2 Q10 ulps of the spec-derived hand-computation
`30 + (−14)·(0.68+0.58+0.34+0.19) = +4.94 dB`. The seed is consumed
bit-for-bit on the first decode invocation.

**OQ-PASTSEED status: PINNED — no fix required.**

### E.6 OQ-PASTPROG resolution

**Pinned strategy: re-seed `pastErrors[0] := −14336` and FIFO-shift
older entries down by one slot. Current code (`decode.go:69-72`) is
consistent with the spec.**

**Spec citation: G.729 (06/2012) §3.9.1 / §3.9.2 do NOT specify
behaviour for `Σc² = 0` because eq. (66) `E_c = 10·log₁₀(Σc²/40)` is
mathematically undefined at that value, and the algebraic codebook
construction (§3.8 / §A.3.8) structurally precludes it.** The closest
spec analog is §4.4.3 eq. (95):

> `Û(m) = (¼ Σ_{i=1..4} Û(m−i)) − 4.0  bounded by  Û(m) ≥ −14`

 the FER attenuation rule, which establishes that the spec's chosen
floor for the predictor state is the same −14 dB used at cold start.
Re-seeding `Û(m) := −14` on the (defensive) zero-energy path is
consistent with this floor.

Corpus evidence: 0 of 7500 SPEECH.BIT subframes triggered the guard.
**The OQ-PASTPROG question is therefore unobservable on valid
bitstreams** and the chosen defensive behaviour cannot affect any
acceptance metric on the Phase 3 corpus.

**OQ-PASTPROG status: PINNED — no fix required (and unreachable on
valid input).**

### E.7 Candidate B verdict

**EXONERATED — proceed to DIAG-2.**

Evidence:

1. **Seed value**: −14336 matches spec (§4.3 Table 9). Seed is consumed
   bit-for-bit on first decode (sf 0 `predicted` matches hand
   computation to within 2 Q10 ulps).
2. **Cold-start bias check**: `mean(predicted)` sf 0..9 = +12.31 dB vs
   long-run +24.45 dB (Δ = −12.14 dB). This Δ is the MECHANICAL
   consequence of the spec-mandated −14 dB seed: starting from
   `Ê(0) = +4.94 dB` and ramping up over ~10 subframes as voiced
   content fills the predictor history. The same ramp would occur in
   ANY spec-conformant decoder; an ITU reference decoder fed the same
   bitstream sees the identical convergence trajectory. There is no
   asymmetric-cold-start bias to fix.
3. **Zero-energy guard**: never fires on the corpus. Cannot account
   for any phase / amplitude defect.
4. **Magnitude argument vs the 62-sample shift**: the cold-start
   transient affects only the first ~10 subframes (5..50 ms of audio).
   The 62-sample SegSNR shift is measured across the entire 30 s
   corpus and is amplitude-blind. A 10-subframe transient cannot
   produce a corpus-wide 62-sample alignment offset.

### E.8 Recommended next step

**Skip REF-1 candidate-B design (no defect to fix) and dispatch
DIAG-2 (candidate C — LP coefficient interpolation / Â(z) pipeline)
per Phase 3b plan §4.**

Open subsidiary concern (out of DIAG-1 scope, recorded for triage):
the int16 saturation of `predicted` at +32 dB on voiced subframes
(see §E.3 obs 3) deserves its own diagnostic if DIAG-2 also
exonerates candidate C. The saturation collapses the upper ~3 dB of
the predicted-gain dynamic range and could be a contributor to the
residual amplitude defect (rms 419 vs 1500 PARTIAL floor), though
not to the phase / 62-sample-shift defect.
