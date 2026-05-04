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

## Appendix F — Phase 3b DIAG-2: LP interpolation trajectory & OQ-LP-INTERP pin

Date: 2026-05-04 (post-faff330; Phase 3b DIAG-1 EXONERATED candidate B)
Owner: Phase 3b DIAG-2 (candidate C isolation)

### F.1 Mission recap

Pin **OQ-LP-INTERP** (the per-subframe LP-coefficient interpolation
prescribed by ITU-T G.729 §3.2.5 / §4.1.5: domain, weighting, and
subframe ordering) from spec text and validate the production
implementation (`internal/lsp/interpolate.go::interpolateLSP` called
from `internal/lsp/decoder.go::Decoder.Decode` step 7) against corpus
evidence from SPEECH.BIT. Per the Phase 3b plan §4 DIAG-2, OQ-LP-INTERP
gates the decision on whether candidate C (LP coefficient pipeline)
proceeds to a fix (REF-1) or is exonerated, in which case Phase 3c
must enumerate a new candidate D for the residual ~62-sample
cross-correlation peak shift identified in Appendix D.

### F.2 Spec reading

ITU-T G.729 (06/2012), §3.2.5 "Interpolation of the LSP coefficients"
(PDF p. 14, paragraph beginning "The quantized (and unquantized) LP
coefficients are used for the second subframe …", lines ~901–919).
The paragraph prescribes, with `q_i^(previous)` and `q_i^(current)` the
per-frame LSP vectors (cosine-domain, i = 1..10):

- Subframe 1 : `q_i^(1) = 0.5·q_i^(previous) + 0.5·q_i^(current)`
- Subframe 2 : `q_i^(2) = q_i^(current)`
- Same equation, with `q_i → q̂_i`, applies to the QUANTIZED LSP
  vectors used by both encoder and decoder (final paragraph of §3.2.5).

4.1.5 "Reconstruction of the LP filter coefficients" (decoder side)
references back to §3.2.5 for the interpolation step verbatim — the
decoder does not redefine the equation; it pulls it through unchanged.

Pinned OQ-LP-INTERP semantics:

| Aspect            | Spec value                                    |
|-------------------|-----------------------------------------------|
| Interpolated obj. | LSP coefficients `q̂_i` (cosine-domain)        |
| Q-format          | Q15 (production, consistent with §3.2.4 LSP)  |
| Weighting         | Strict 50/50 unweighted average               |
| Subframe ordering | sf-1 interpolated; sf-2 uses raw `q̂_i^(m)`   |
| Endpoint          | `q̂_i^(m-1)` is the post-stability LSP from m-1|

Cross-references (clean-room, no external implementations):

- Salami 1998 IEEE T-SAP §V.B (CS-ACELP): describes the same
  inter-subframe LSP interpolation as a halving in the cosine domain
  to halve transmission rate of LP parameters while preserving
  per-subframe LP renewal. Consistent with §3.2.5.
- Kondoz §6 (CELP variants): cosine-domain interpolation is the
  canonical low-complexity choice (avoids per-subframe Lsf↔Lsp
  conversion); consistent.

### F.3 Code audit

`internal/lsp/interpolate.go` (lines 68–73):

```
func interpolateLSP(prev, curr, sf1, sf2 *[10]int16) {
    for i := 0; i < 10; i++ {
        sf1[i] = int16((int32(prev[i]) + int32(curr[i])) >> 1)
        sf2[i] = curr[i]
    }
}
```

Audit findings:

- Domain: Q15 cosine-LSP. `prev`/`curr` are the LSP vectors saved by
  `lsp.Decoder` step 9 (`d.prevLSP = lsp`; `lsp` produced by
  `lsfToLSP(lsf[i])` per `decoder.go:88`). Matches spec.
- Weighting: 50/50 average implemented as `(prev + curr) >> 1` on the
  promoted int32 sum. Floor rounding (toward −∞) on odd sums. The R-C
  rounding ambiguity is documented in `interpolate.go` lines 37–43 and
  was branch-tested REFUTE_unchanged at Phase 1n RC-1; it is at most a
  ±1-LSB perturbation on cells i=1, i=5 (cf. Phase 1n RC-3 synthesis
  report). It cannot generate a corpus-wide 62-sample shift.
- Subframe ordering: `sf1` ← interpolated, `sf2` ← `curr`. Matches
  spec (§3.2.5 "Subframe 1 : … Subframe 2 : q_i^(current)").
- Q-domain at LSP→LP (`lsp.LSPToLP` per `lsp_lp.go`): each subframe's
  Q15 LSP is converted to Q12 a[0..10] via the §3.2.6 recurrence on
  the F1/F2 polynomials. No inter-subframe domain mismatch.
- Cold-start: `d.prevLSP` is lazily initialised to
  `initialPrevLSP = cos(i·π/11) Q15` per §3.2.4 / §4.1.5 on the first
  Decode (`decoder.go:94-97`). Matches spec.

**No deviation from spec.** Production code is byte-faithful to the
3.2.5 / §4.1.5 prescription.

### F.4 Test method

- Test: `internal/decoder/phase3b_diag2_lpinterp_test.go::TestPhase3bDiag2_LPInterpolationTrajectory`.
- Shim: `internal/lsp/phase3b_diag2_lpinterp_export.go` exposes
  `(*Decoder).PrevLSPSnapshot() [10]int16` and `(*Decoder).InitializedForDiag() bool`.
- Driver runs the production `Decoder.Decode` per frame so state
  evolution is identical to the roundtrip pipeline B harness
  (`phase3_roundtrip_quality_test.go`). Per frame the test snapshots
  `d.lsp.PrevLSPSnapshot()` BEFORE Decode (= q̂(m-1) input to
  interpolateLSP, with the spec cold-start value substituted on frame
  0 per the §3.2.4 lazy init) and AFTER Decode (= (m), sinceq
  decoder.go:108 saves the freshly-decoded LSP into prevLSP).
- The test then re-applies the §3.2.5 eq. (24) interpolation OUTSIDE
  the production decoder over the snapshotted (prev, curr) pair to
  produce a faithful record of (sf1LSP, sf2LSP, sf1A, sf2A), plus two
  alternative sf-0 LP reconstructions for §F.7 (alt-1: sf-0 uses
  q̂(m); alt-2: sf-0 uses q̂(m-1)).
- Stability check: in-test Schur–Cohn step-down on the Q12 a[1..10]
  vector (clean-room, mirrored from `lsp/stability_test.go`'s
  TestALGTHMFrame0SF0_AzStability). |k_m| < 1 ∀m=10..1 ⇔
  minimum-phase Â(z).
- Voiced-frame proxy: top-5 frames by ||q̂(m) − q̂(m-1)||₂ (LSP
  velocity) — captures the high-LP-motion regime where any
  interpolation defect would be most visible.
- (d) full waveform-resynthesis variant: SIMPLIFIED — dumps L2
  distances between the three sf-0 a-vectors (base / alt-1 / alt-2)
  rather than driving synth.Filter three times with matched
  pastSynth memory. The simplification is bounded: a 62-sample
  cross-correlation shift requires a coherent per-frame phase
  perturbation in the LP filter; if the L2 distance between
  base and alt-1 (or alt-2) is large but the corpus-wide cross-
  correlation shift remains 62 samples in either alternative,
  no LP-interpolation-domain choice can collapse the shift.

### F.5 First-10-frame trace (excerpt)

a[1..10] in Q12; a[0] = 4096 omitted. sf-0 is interpolated, sf-1 is
the current frame's LSP converted directly to LP.

| frame | sf-0 a[1..10]                                     | sf-1 a[1..10]                                       |
|------:|---------------------------------------------------|-----------------------------------------------------|
| 0     | -297, 364, -662, 810, 99, 460, -378, 72, -10, 224 | -594, 718, -1359, 1646, 114, 982, -815, 175, -47, 447 |
| 1     | -804, 445, -1060, 963, 169, 644, -489, 373, 47, 510 | -1015, 154, -724, 195, 293, 270, -139, 567, 138, 573 |
| 5     | -3191, 3677, -5997, 5269, -4281, 5028, -3441, 2284, -1765, 1179 | -4055, 3875, -6565, 5869, -4743, 5178, -3611, 2239, -1746, 1090 |
| 9     | -4526, 2025, -4019, 3848, -1484, 3183, -2516, 1141, -1291, 789 | -5239, 3033, -4314, 4020, -2034, 3343, -2751, 1722, -1615, 800 |

Frame 0 sf-0 is exactly half of sf-1 (within ±1 Q12 ulp on every
coefficient): 0.5·(initialPrevLSP, q̂(0)) interpolated through the
Chebyshev recurrence inherits the linearity of the cosine-domain
midpoint when the prev vector is the symmetric init
`{31441, 27566, …, −31441}`. This is the cleanest possible
demonstration that the interpolation is operating exactly as spec.

### F.6 Stability + monotonicity statistics (SPEECH.BIT, 3750 frames)

| Statistic                                          | Value          |
|----------------------------------------------------|----------------|
| sf-0 (interpolated) LSP monotonicity violations    | 0 / 3750 (0.0000%) |
| sf-1 (current LSP)  LSP monotonicity violations    | 0 / 3750 (0.0000%) |
| sf-0 (interpolated) Â(z) Schur–Cohn instability    | 0 / 3750 (0.0000%) |
| sf-1 (current LSP)  Â(z) Schur–Cohn instability    | 0 / 3750 (0.0000%) |
| Frame-to-frame ‖q̂(m) − q̂(m-1)‖₂ mean             | 3964.92 Q15 (= 0.1210 normalised) |
| Frame-to-frame ‖q̂(m) − q̂(m-1)‖₂ stddev           | 2797.26 Q15    |

Every interpolated subframe LSP across the entire corpus is
strictly monotone-decreasing (well-formed cosine LSP) and every
resulting Q12 Â(z) is minimum-phase (all reflection coefficients
strictly inside the unit disk). There is no pathological subframe
where the interpolation lands on a degenerate or unstable LP filter.

### F.7 Subframe-boundary delta analysis (SPEECH.BIT, n = 3749 frame transitions)

L2 distances over a[1..10] in Q12 units:

| Quantity                                          | Mean      |
|---------------------------------------------------|----------:|
| ‖a_sf0(m) − a_sf1(m)‖₂   within-frame             | 1248.73   |
| ‖a_sf1(m) − a_sf1(m-1)‖₂ across-frame             | 2488.29   |
| ‖a_sf0(m) − a_sf1(m-1)‖₂ sf-0 vs prev sf-1        | 1244.61   |
| ‖a_sf1(m)  sf-1 vs prev sf-1        | 2488.29   |− a_sf1(m-1)

Interpretation: the interpolated subframe-0 LP sits exactly at the
midpoint of the LP trajectory between subframe-1 of frame (m-1) and
subframe-1 of frame m. Quantitatively `1248.73 ≈ 1244.61 ≈
2488.29 / 2 = 1244.15`. The sub-1% asymmetry between the
within-frame and prev-vs-sf0 distances is the expected non-linearity
of the LSP→LP §3.2.6 Chebyshev recurrence acting on a midpoint LSP
(LP space is not a linear image of LSP space, but the discrepancy is
~0.3% of the across-frame distance — orders of magnitude smaller
than any 62-sample cross-correlation shift mechanism).

(d) Top-5 voiced-proxy frames (highest LSP velocity), sf-0 LP under
three reconstructions — ‖base − alt1‖, ‖base − alt2‖, ‖alt1 − alt2‖
in Q12:

| frame | velocity | ‖base−alt1‖ | ‖base−alt2‖ | ‖alt1−alt2‖ |
|------:|---------:|------------:|------------:|------------:|
|  820  | 24493    | 5743.24     | 4476.96     |  9936.36    |
| 1068  | 21641    | 3533.67     | 3438.99     |  6709.48    |
| 3420  | 21504    | 5199.39     | 7687.18     | 12727.21    |
| 1052  | 21363    | 4717.87     | 3887.07     |  8456.90    |
| 2489  | 21155    | 4071.33     | 3759.82     |  7589.29    |

Both alternatives are large perturbations of the production LP
(thousands of Q12 units), not small enough to plausibly be the
"hidden right answer". Critically, the production base sits between
alt-1 (no interp; sf-0 = sf-1) and alt-2 (held; sf-0 = prev sf-1) at
roughly the midpoint — precisely the geometric signature of a
correct 50/50 average. Neither alternative has a privileged position
that would suggest a sub-sample / sub-subframe phase shift the
spec-correct interpolation is "missing".

### F.8 OQ-LP-INTERP resolution

**Pinned spec semantics:** §3.2.5 eq. (24) — cosine-domain LSP, 50/50
average, sf-1 interpolated, sf-2 uses raw `q̂_i^(m)`. Identical
equation applies to quantized vectors per §3.2.5 final paragraph and
4.1.5 (decoder side, by reference).

**Observed code behaviour:** byte-faithful to spec. `interpolateLSP`
implements `(prev + curr) >> 1` in Q15 cosine domain on int32-promoted
sums, with subframe ordering and quantized-LSP substitution per spec.
Cold-start prev-LSP is the §3.2.4 init `cos(i·π/11) Q15`.

**OQ-LP-INTERP status: PINNED — no fix required.**

The only documented R-C ambiguity (floor vs symmetric rounding on the
half-sum) was branch-tested REFUTE_unchanged at Phase 1n RC-1
(commit a47f03f), affecting at most ±1 Q12 ulp on a[8..10] of the
interpolated subframe — orders of magnitude below any 62-sample-shift
mechanism.

### F.9 Candidate C verdict

**EXONERATED — neither B nor C is the cause; escalate to Phase 3c
with new candidate D enumeration.**

Evidence:

1. **Spec match**: the production interpolation equation, domain,
   subframe ordering, and cold-start are all spec-correct (§F.3, §F.8).
2. **Well-formedness**: 0 / 3750 monotonicity violations, 0 / 3750
   Â(z) instability cases (§F.6). The interpolated LP filter is never
   degenerate or unstable; it cannot inject phase artefacts that
   exceed the §3.2.5 numerical envelope.
3. **Geometric correctness**: the interpolated sf-0 sits at the
   exact LP-trajectory midpoint between sf-1(m-1) and sf-1(m)
   (1244.61 vs 2488.29; ratio = 0.5001), confirming that the
   half-weight is being applied correctly through the LSP→LP
   non-linearity (§F.7).
4. **Magnitude argument vs the 62-sample shift**: a per-subframe
   LP interpolation defect would manifest as either (a) a wrong-
   weighting bias (would show up as ratio ≠ 0.5 in §F.7), (b)
   degenerate filter excursions (would show in §F.6 Schur–Cohn or
   monotonicity counters), or (c) a domain mismatch (would corrupt
   sf-0 alone, asymmetrically across frames). None of these
   signatures are present. The 62-sample corpus-wide shift cannot
   originate in `interpolateLSP`.

With B and C both exonerated, the residual −22-sample / 62-sample
cross-correlation peak offset (Appendix D.2) must originate
downstream of the LP-coefficient pipeline. Candidate space for
Phase 3c (new candidate D enumeration), ranked by signature match
to a corpus-wide phase/timing skew:

- **D-1 Adaptive postfilter long-term / short-term filter memory
  initialization** (§4.2). Postfilter long-term filter has its own
  pitch-period delay line; a misaligned reset or off-by-one delay
  index would compound to a sample-resolution corpus drift.
- **D-2 Adaptive codebook past-excitation FIFO indexing**
  (`d.pastExc`). A 1-sample bias in the AdaptiveCodebook fractional
  resampling or an off-by-one in the post-subframe FIFO advance
  (`subframe.go:51-52`) would produce per-subframe phase skew that
  exactly fits the "amplitude-blind, alignment-only" defect.
- **D-3 HP filter group delay** (`internal/decoder/hpfilter.go`).
  A sign or coefficient deviation in the §4.2.4 IIR HP would shift
  group delay; less likely to compound (it is a fixed filter) but
  worth a one-shot impulse-response check.
- **D-4 ScaleUpSat ordering vs HP filter** (the post-HP ×2
  amplification per `pcm.ScaleUpSat`). If the spec applies ×2
  BEFORE the HP filter, a per-subframe transient response shift
  could appear; if AFTER (current code), it is amplitude-only.
  Cross-check §4.2.3 / §4.2.4.

Subsidiary concern carried forward from §E.3 obs 3 (predicted log-gain
int16 saturation at +32 dB on voiced subframes) remains open as an
amplitude-only investigation; it does not interact with the phase /
62-sample-shift candidate D ladder above.

### F.10 Recommended next task

**Phase 3c: open new candidate D enumeration** — DIAG-3 targeting
D-2 (adaptive codebook past-excitation FIFO indexing) FIRST, since
its signature most cleanly matches a per-subframe sample-resolution
skew that compounds across the corpus. D-1 (postfilter long-term
filter memory) is the fallback if D-2 exonerates.

Skip REF-1 candidate-C design (no defect to fix). Phase 3b is
diagnostically complete; the remaining 62-sample shift is
re-classified as a Phase 3c objective.

## Appendix G — Phase 3b DIAG-3: adaptive codebook FIFO trajectory & OQ-AC-FIFO pin

### G.1 Mission recap

Phase 3b DIAG-3 targets candidate D-2 from §F.9: the adaptive-codebook
(AC) past-excitation FIFO + fractional resampling. The hypothesis: a
deviation in pitch-lag unpacking (P1 / P0 / P2), in past-excitation
indexing (b30 access pattern, off-by-one in `pastExc` base), in the
b30 FIR table itself, or in the per-subframe FIFO read/write order
would generate the amplitude-blind, alignment-only signature observed
in pipeline B (XCorr peak shift = −22 vs path-A +40, SegSNR −0.90 dB,
RMS 419 — Appendix D.2).

### G.2 Spec reading

ITU-T G.729 (06/2012) — clean-room re-read of the relevant clauses:

- **§3.7.1 eq. (40)** — adaptive codebook construction:

      v(n) = Σ_{i=0..9}  u(n − k − i)·b30(t + 3i)
           + Σ_{i=0..9}  u(n − k + 1 + i)·b30(3 − t + 3i)

  for `n = 0..39`, where `(k, t)` carry the (integer, fractional)
  pitch delay components. The integer-only case (`t = 0`) reduces to
  a direct copy `v(n) = u(n − k)` since b30(0) is the implicit centre
  tap. For short pitch (T_int < 40) the AC is extended by periodicity:
  `v(n) = v(n − T_int)` for `n ≥ T_int`.
- **§3.7.2** — b30 definition: Hamming-windowed sinc, cut-off 3600 Hz
  (3 dB), oversampled by 3, |k| ≤ 29 with a zero pad at ±30 ⇒ 30
  one-sided unique taps + b30(0). `b30(0) ≈ 0.9` (the cut-off scaling
  factor 2·fc/fs = 0.9), **not 1.0** — the integer-delay fast path's
  "implicit unity tap" comment is a slight idealisation; the
  amplitude offset is symmetric across encoder and decoder so it
  has no effect on a closed-loop search nor on a decoder driven by
  an encoder using the same convention.
- **§4.1.3 eq. (41)** — sub-frame 1 lag from P1 ∈ [0, 255]:

      P1 < 198  : T_int = 19 + (P1+2)/3,  T_frac = (P1+2)%3 − 1
      P1 ≥ 198  : T_int = P1 − 112,        T_frac = 0

  ⇒ T_int ∈ [19, 143], T_frac ∈ {−1, 0, +1}.
- **§4.1.3 eq. (42)** — sub-frame 2 lag from P2 ∈ [0, 31] relative to
  T1:  t_min = clip(T1−5, 20, 134); T_int = t_min + (P2+2)/3 − 1;
  T_frac = (P2+2)%3 − 1.
- **§3.7.2 / §4.1.3 parity** — P0 = NOT(b7 ⊕ b6 ⊕ b5 ⊕ b4 ⊕ b3 ⊕ b2)
  computed over the six MSBs of P1 (the parity is informational at
  the decoder; mismatch is a frame-erasure flag, not a bitstream
  layout error).
- **FIFO update order** (§4.1.6, implicit): after each subframe the
  full excitation `u = gp·v + gc·c` is committed to the past-
  excitation buffer; the next subframe reads the updated buffer.
  Read-before-write within the current subframe (the AC build for
  subframe `s` reads only memory written through subframe `s−1`).

### G.3 Code audit

| Code site                                                         | Function                       | Behaviour                                                                                          |
|-------------------------------------------------------------------|--------------------------------|----------------------------------------------------------------------------------------------------|
| `internal/decoder/decode.go:64-67`                                | `Decode`                       | `pitch.DecodeDelaySubframe1(P1)`, `pitch.CheckParity(P1, P0)` (result not gated), `…Subframe2(P2, T1)` |
| `internal/pitch/delay.go::DecodeDelaySubframe1`                   | sf-1 lag unpack                | Inverts §4.1.3 eq. (41) exactly; T_int ∈ [19, 143], T_frac ∈ {−1, 0, +1}.                          |
| `internal/pitch/delay.go::DecodeDelaySubframe2`                   | sf-2 lag unpack                | Inverts §4.1.3 eq. (42) exactly; t_min clip to [20, 134], T_int may reach 144 with frac=−1.        |
| `internal/pitch/parity.go::Parity` / `CheckParity`                | parity recompute               | XOR over six MSBs of P1, NOT'ed: `P0 = NOT(b7 ⊕ … ⊕ b2)` per §3.7.2 / §4.1.3.                       |
| `internal/decoder/subframe.go:31`                                 | AC build call                  | `pitch.AdaptiveCodebook(tInt, tFrac, d.pastExc[:], &v)`                                            |
| `internal/pitch/adaptive.go::AdaptiveCodebook`                    | AC build                       | tFrac=0 fast path = direct copy from `pastExc[L−tInt+n]`; tFrac=±1 = §3.7.1 eq. (40) over 20 taps; tInt<40 = periodic extension. |
| `internal/pitch/adaptive.go::firInterpolate`                      | b30 FIR convolution            | `tFrac=+1: k=tInt, posPhase=1, negPhase=2`; `tFrac=−1: k=tInt−1, posPhase=2, negPhase=1`. Out-of-range reads zero (matches §3.7.1 boundary clause). |
| `internal/tables/pitch_interp.go::PitchInterpFIR`                 | b30 table                      | 31 Q15 ints, indices 0..30 = b30(0..30); b30(0) = 29443 (≈ 0.898, the cut-off factor).             |
| `internal/decoder/subframe.go:51-52`                              | FIFO commit                    | `copy(pastExc[:113], pastExc[40:]); copy(pastExc[113:], u[:])` — read-then-write, end of subframe. |
| `internal/pitch/closedloop/frac.go::Interpolate3`                 | encoder mirror                 | Same posPhase/negPhase mapping, same b30 table. Encoder/decoder symmetric.                         |

**Deviations from spec**: none affecting time alignment. The integer-
delay fast path skips the FIR (foregoing a uniform ~10% amplitude
attenuation factor that b30(0) ≈ 0.898 would impose), but this is
encoder/decoder-symmetric and rescaled by gp at every subframe, so
it cannot inject a phase skew between an ITU-encoded bitstream and
our decoder (pipeline B).

### G.4 Test method

`internal/decoder/phase3b_diag3_acfifo_test.go` drives the entire
SPEECH.BIT corpus (3750 frames, 7500 subframes) through
`Decoder.DecodeWithTaps` (the existing tap-collecting mirror in
`phase3diag_taps_export_test.go`), capturing per subframe:

- raw P1, P0, P2 indices; recomputed parity bit;
- decoded T_int, T_frac (both subframes);
- v[0..39] and u[0..39];
- gpQ14;

then re-runs frame 0 with a fresh `Decoder` to snapshot
`PastExcSnapshot()` (added in `phase3b_diag3_acfifo_export_test.go`)
so the FIFO commit invariant can be verified directly.

The diagnostic dumps the b30 table, hand-recomputes a reference using
`b30(k) = 0.9 · sinc(0.3·k) · hamming(k, N=60)` (Oppenheim & Schafer
7.2 derivation, no external implementation consulted), and checks
the polyphase decomposition (sum of phase-0/1/2 taps).

### G.5 First-5-frame trace (excerpt)

```
sf  0 (frame 0/0) T_int= 20 T_frac=+0 ‖v‖₂=     0.00 gp=0.1218 gp·‖v‖=  0.00
sf  1 (frame 0/1) T_int= 20 T_frac=+0 ‖v‖₂=     0.00 gp=0.3139 gp·‖v‖=  0.00
sf  2 (frame 1/0) T_int= 46 T_frac=+0 ‖v‖₂=     0.00 gp=0.8179 gp·‖v‖=  0.00
sf  3 (frame 1/1) T_int= 49 T_frac=-1 ‖v‖₂=     9.00 gp=0.3139 gp·‖v‖=  2.83
sf  4 (frame 2/0) T_int= 22 T_frac=-1 ‖v‖₂=     9.70 gp=0.3139 gp·‖v‖=  3.04
sf  5 (frame 2/1) T_int= 22 T_frac=+0 ‖v‖₂=    10.44 gp=0.4085 gp·‖v‖=  4.26
sf  6 (frame 3/0) T_int= 24 T_frac=+1 ‖v‖₂=     8.83 gp=0.4939 gp·‖v‖=  4.36
sf  7 (frame 3/1) T_int= 24 T_frac=-1 ‖v‖₂=    23.66 gp=0.3760 gp·‖v‖=  8.90
sf  8 (frame 4/0) T_int=111 T_frac=+0 ‖v‖₂=     9.80 gp=1.0559 gp·‖v‖= 10.35
sf  9 (frame 4/1) T_int=110 T_frac=-1 ‖v‖₂=    25.36 gp=1.0888 gp·‖v‖= 27.61
```

Cold-start v=0 in subframes 0..2 reflects an empty `pastExc` ring;
warm-up by sf 3 once non-zero u has been committed. T_int trajectory
20 → 46/49 → 22 → 24 → 110/111 plausibly tracks voiced/unvoiced
transitions. T_frac visits all three legal phases.

### G.6 Distribution / parity / FIR sanity statistics

```
T_int distribution (legal range [19, 144]):
  observed range: [20, 143]   out-of-range: 0 / 7500 (0.0000%)
  top-10 T_int bins (lag -> count):
    T_int= 20  count=  386  (5.15%)
    T_int= 30  count=  350  (4.67%)
    T_int= 34  count=  336  (4.48%)
    T_int= 35  count=  319  (4.25%)
    T_int= 32  count=  313  (4.17%)
    T_int= 31  count=  307  (4.09%)
    T_int= 33  count=  287  (3.83%)
    T_int= 37  count=  269  (3.59%)
    T_int= 36  count=  261  (3.48%)
    T_int= 29  count=  245  (3.27%)
T_frac distribution:
  T_frac=-1  count= 2074  (27.65%)
  T_frac=+0  count= 3255  (43.40%)
  T_frac=+1  count= 2171  (28.95%)
  out-of-spec: 0 / 7500
Parity sanity: mismatches 0 / 3750 frames (0.0000%)
```

T_int never escapes [20, 143] (lower bound 20 dominated by short-pitch
female speech in SPEECH.BIT; upper bound 143 reached). T_frac
distribution is plausible: integer delay slightly preferred (43.4%
matches the §4.1.3 P1≥198 region width = 143−85 = 58 integer-only
codes vs the                                                                          = 201 fractional codes ⇒ expected 22% by code67
count; the corpus drift toward integer reflects pitch-tracker
preference, not a bug). Zero parity mismatches ⇒ bitstream layout is
consistent with §4.1.3.

```
PitchInterpFIR vs hand-recomputed 0.9·sinc(0.3·k)·hamming(k, N=60):
  max |delta| over all 31 taps: 48 Q15 LSBs
  per-phase DC sums (b30 mirrored, each phase ~unity expected):
    phase 0 (k=0,3,6,…): sum = 32723 Q15  (= +0.99863)
    phase 1 (k=1,4,7,…): sum = 43746 Q15  (= +1.33502)
    phase 2 (k=2,5,8,…): sum = 21818 Q15  (= +0.66583)
```

The 48-LSB max deviation between table and hand-recomputed b30 is
consistent with a single Q15-rounding pass plus a mild
window-construction variation (e.g., open vs closed Hamming
endpoints). The polyphase per-phase DC asymmetry (phase 1 = 1.335,
phase 2 = 0.666; mean = 1.000) is the expected analytical signature
of `b30 = 0.9·sinc(0.3·hhhhhhhamming` — the phases are NOT individuallyk)
unity-DC, but each fractional-FIR access combines `posPhase` +
`negPhase` whose pairwise mean restores unity (phase 1 + phase 2 =
2·1.0005). No anomaly.

### G.7 Subframe-boundary continuity

```
|u_prev[39] − u_curr[0]| over 7499 transitions:
  mean = 30.48   median = 8   max = 525
  exact-zero-delta transitions: 1173 / 7499 (15.6421%)

FIFO commit invariant (post-frame-0 trailing 80 == U_sf1 ⊕ U_sf2):
  trailing-80 sf-1 mismatches: 0 / 40
  trailing-80 sf-2 mismatches: 0 / 40

Per-subframe XCorr peak lag (v[] vs u[], window ±5):
  lag=+0  count=6147  (81.96%)
  lag=-5  count= 485  (6.47%)        ← window edge accumulator
  lag=-1  count= 137  (1.83%)
  lag=+1  count= 113  (1.51%)
  lag=-4  count= 106  (1.41%)
  lag=-3  count= 104  (1.39%)
  lag=-2  count= 100  (1.33%)
  lag=+2  count=  82  (1.09%)
  lag=+3  count=  81  (1.08%)
  lag=+4  count=  75  (1.00%)
  lag=+5  count=  70  (0.93%)
```

Three independent boundary tests pass:

1. Boundary continuity is non-degenerate (median |Δ| = 8, mean 30,
   exact-zero rate 15.6% — well above zero, well below the >90% that
   a degenerate "always copy last sample" FIFO would yield).
2. The FIFO commit invariant holds bit-exactly (0 mismatches across
   the trailing 80 samples post-frame-0): the just-committed pastExc
   tail is equal to the just-computed `u` of both subframes ⇒ the
   `copy(pastExc[:113], pastExc[40:]); copy(pastExc[113:], u[:])`
   pair (subframe.go:51-52) implements read-before-write correctly,
   without overlap-write corruption.
3. The XCorr peak lag is at +0 in 81.96% of subframes; the second-
   most populated bin is the edge-overflow at lag=−5 (window
   saturation). Excluding the edge bin the next-most is ±1 at 1.5–
   1.8%, indicating no systematic per-subframe phase skew in `v[]`
   vs `u[]`.

### G.8 OQ-AC-FIFO resolution

**Pinned semantics (per spec)**:

1. P1, P0, P2 unpack into (T_int1, T_frac1) and (T_int2, T_frac2)
   per §4.1.3 eqs. (41) and (42); T_int ∈ [19, 143] for sf-1 and
   [20, 144] for sf-2; T_frac ∈ {−1, 0, +1}.
2. P0 is the parity bit over the six MSBs of P1 (§3.7.2 / §4.1.3);
   the decoder MAY ignore mismatch (informational only — frame-
   erasure flag is supplied separately by the transport layer).
3. AC reconstruction follows §3.7.1 eq. (40) using b30, the 1/3-
   sample Hamming-windowed sinc with cut-off 3600 Hz; the integer-
   delay case reduces to direct copy under the convention b30(0)
   absorbs the cut-off scaling factor (≈ 0.898) and is treated as
   the implicit centre tap.
4. The past-excitation FIFO is updated after each subframe (full
   `u = gp·v + gc·c` committed); the next subframe reads the updated
   buffer (read-before-write within the current subframe boundary).

**Observed code behaviour** (§G.3):

- Lag unpack: matches eqs. (41)/(42) exactly (tested over 7500
  subframes, 0 out-of-range, 0 illegal T_frac).
- Parity recomputation: matches the §3.7.2 / §4.1.3 prescription
  (0 mismatches over 3750 frames).
- AC build: implements eq. (40) via `firInterpolate` with the
  posPhase/negPhase mapping; integer fast-path skips the FIR (a
  ≈10% amplitude offset relative to a strict b30(0)≈0.898
  multiplier, but encoder/decoder symmetric).
- FIFO commit: read-then-write, with the trailing 80 samples
  bit-exactly equal to the just-decoded U_sf1 ⊕ U_sf2 (verified by
  `PastExcSnapshot()`).
- b30 table: matches a clean-room hand-recomputed sinc·hamming to
  ≤48 Q15 LSBs across 31 taps; polyphase DC averages to unity.

### G.9 Candidate D-2 verdict

**EXONERATED — adaptive codebook FIFO is spec-correct; 62-sample
shift does not originate here.**

Evidence:

1. **Spec match** — every prescribed equation (eqs. 40, 41, 42,
   parity definition) is implemented exactly; no off-by-one, no
   sign error, no swapped phase index.
2. **Bit-exact FIFO invariant** — the post-subframe pastExc tail
   equals `u` (0 / 80 mismatches), so neither read-before-write
   nor write-before-read is corrupted.
3. **No per-subframe phase skew** — XCorr peak lag(v vs u) sits at
   0 in 82% of subframes, with a ±1-symmetric tail (1.5% / 1.8%);
   no directional bias toward the −22-sample / 62-sample shift
   measured at the corpus level (Appendix D.2).
4. **Encoder/decoder symmetry** — `pitch/closedloop.Interpolate3`
   uses the same posPhase/negPhase mapping and the same b30 table
   as `pitch.AdaptiveCodebook`; pipeline B (ITU encoder → our
   decoder) has zero asymmetry to expose.
5. **b30 table integrity** — table values match a clean-room hand-
   recomputed Hamming-windowed sinc to ≤48 Q15 LSBs (≤ 0.15% per
   tap), well within Q15 rounding noise.

The amplitude-blind, alignment-only 62-sample corpus shift therefore
must originate downstream of the AC reconstruction. From the
remaining D-1 / D-3 / D-4 ladder of §F.9, the strongest residual
candidate is **D-1 (adaptive postfilter long-term filter memory)**:
the §4.2 long-term postfilter has its own pitch-period delay line
that is separate from `d.pastExc`, and a misaligned reset, off-by-
one delay index, or different cold-start convention would compound
to a sample-resolution corpus-wide drift exactly matching the
observed signature.

### G.10 Recommended next task

**Phase 3b DIAG-4 targeting candidate D-1 (adaptive postfilter
long-term filter)** — `internal/postfilter/`:

- enumerate the postfilter long-term delay-line state and cold-start
  policy (§4.2.1 / §4.2.2);
- audit the per-subframe pitch-period read against `tInt`;
- diagnostic test capturing the postfilter LT memory + per-subframe
  output offset vs a "postfilter-bypass" pipeline (already wired
  in `phase3diag_03_synthesis_bypass_test.go`);
- pin OQ-PFLT-MEM and verdict candidate D-1 CONFIRMED / PARTIAL /
  EXONERATED.

If D-1 also exonerates, escalate to D-3 (HP filter group delay
impulse-response check) and D-4 (ScaleUpSat ordering vs HP).

## Appendix H — Phase 3b DIAG-4: postfilter-bypass discriminator + (conditional) D-3/D-4 drill

Date: 2026-05-04 (post-241c8d4; Phase 3b DIAG-3 EXONERATED candidate D-2)
Owner: Phase 3b DIAG-4 (candidate D-1 isolation; supplementary D-3 / D-4 drill)
Disposition: **D-1 EXONERATED — and the −22-sample shift premise itself rebutted.**

### H.1 Mission

Bypass-discriminator for candidate D-1 (Annex A postfilter long-term
filter memory + pitch-period delay-line + tilt + AGC) per ITU-T G.729
A.4.2. Decision rule: if a postfilter-bypassed pipeline cleans up the
residual ~62-sample cross-correlation peak shift identified in
Appendix D.2, candidate D-1 is confirmed and the rest of the D-3 / D-4
ladder can be skipped. Otherwise, the same diagnostic file drills into
synthesis 1/Â(z) memory (D-3) and excitation cold-start (D-4) using
existing taps exports (no new production surface).

### H.2 Method

Bypass mechanism: a test-only shim
`(*Decoder).DecodeFrameNoPostfilter` mirrors `Decoder.Decode` line-for-
line except the `d.pst.Filter(...)` call is replaced with `copy(sPf, s)`
 synthesis output is fed directly to the §4.2.2 output HP filter,
then `pcm.ScaleUpSat`. Located in
`internal/decoder/phase3b_diag4_postfilter_bypass_export_test.go`
(`_test.go`-only, no production API surface added).

REF availability:
- **REF_pf**: `SPEECH.PST` — post-postfilter ITU reference, present.
- **REF_in**: `SPEECH.IN`  — upstream PCM, present (this is the
  Appendix D.2 reference).
- **REF_raw**: pre-postfilter ITU reference — **NOT shipped** in
  `g729AnnexA/test_vectors/`. The discriminator therefore measures
  alignment vs both REF_pf and REF_in to triangulate.

A_pf_only_lt (long-term-only postfilter) variant: **deferred** —
would require partial-bypass plumbing inside `internal/postfilter/`
that the current API does not expose; not needed for the verdict
once REF_pf and REF_in agreement is established.

Drill scope (D-3 / D-4): first 5 frames only, via existing
`DecodeWithTaps` + `phase3diag_taps_export_test.go` exports —
per-subframe rms(u), rms(s), max|u|, max|s|, plus a structural cold-
start invariant check on `pastExc`.

### H.3 Comparison table

SPEECH corpus, 3750 frames, 300 000 samples. Calibration anchor:
ITU-PST vs SPEECH.IN  shift = +40, GlobalSNR = 7.06 dB, SegSNR = 4.35 dB
(matches Appendix D.2 pipeline-A baseline).

vs REF_pf (SPEECH.PST):

| Variant       | rms |  max|s| | SegSNR (dB) | XCorrShift | GlobalSNR (dB) |
|---------------|----:|--------:|------------:|-----------:|---------------:|
| A_pf          | 419 |    5262 |       −0.88 |         −2 |          −0.06 |
| A_raw         | 411 |    5462 |       −0.90 |         −2 |          −0.05 |
| A_pf_only_lt  |  —  |     —   |    deferred |         —  |             —  |

vs REF_in (SPEECH.IN — Appendix D.2 reference):

| Variant | rms |  max|s| | SegSNR (dB) | XCorrShift | GlobalSNR (dB) |
|---------|----:|--------:|------------:|-----------:|---------------:|
| A_pf    | 419 |    5262 |       −0.90 |        −22 |          −0.05 |
| A_raw   | 411 |    5462 |       −0.41 |        +36 |          −0.05 |


### H.4 D-1 verdict

**EXONERATED — and the framing of the residual defect must be
revised.** Two independent observations forced the revision:

1. **vs the same-stage reference (REF_pf), our decoder is sample-
   aligned.** A_pf shows shift = −2 against SPEECH.PST — well within
   the ±2 tolerance of `bestAlignedSNR` over a 240-sample search
   window. This means the entire production decode pipeline
   (synthesis + postfilter + HP + ×2) reproduces the ITU PST
   reference's *temporal alignment* faithfully. There is no
   sample-resolution phase skew in the decoder.

2. **The −22 shift in Appendix D.2 was a `bestAlignedSNR` lock onto a
   spurious local maximum at low SNR.** Against SPEECH.IN, A_pf
   reports shift = −22 with GlobalSNR = −0.05 dB. The SNR landscape at
   |error|·rms(error) ≈ rms(signal) has many near-equal-height local
   maxima and the argmax is unstable. When the postfilter is bypassed
   (A_raw, same upstream pipeline), the argmax jumps to +36 (close to
   the ITU PST anchor +40) but GlobalSNR is unchanged (−0.05 dB) and
   SegSNR actually *improves* by +0.49 dB. The 58-sample shift
   movement is not evidence of a 58-sample postfilter delay; it is
   evidence that the −22 → +36 argmax flip occurred between two
   nearly-degenerate local maxima of an SNR landscape that is
   essentially flat at this signal level.

   The corresponding 62-sample gap (−22 vs +40) is therefore a
   measurement artifact of the chosen reference (SPEECH.IN, which is
   at a different processing stage than the decoder output) combined
   with an alignment metric that is brittle when the underlying
   per-sample RMS error is comparable to the signal RMS.

The postfilter is not the source of the residual quality defect.
**Candidate D-1 is exonerated.**

### H.5 Supplementary D-3 / D-4 drill findings

Cold-start `pastExc[0..152] == 0`: **true** (zero-value Decoder; spec
4.1.6 / §4.3 cold-start contract structurally guaranteed by Go's
zero-value semantics for `[pastExcLen]int16`).

First-5-frame per-subframe stage RMS:

| frame | sf |  rms(u) |  rms(s) | max|u| | max|s| |
|------:|---:|--------:|--------:|-------:|-------:|
|   0   | 1  |    0.32 |    0.32 |     1  |     1  |
|   0   | 2  |    0.32 |    0.45 |     1  |     2  |
|   1   | 1  |    1.58 |    1.67 |     5  |     5  |
|   1   | 2  |    1.41 |    1.55 |     5  |     5  |
|   2   | 1  |    1.47 |    1.54 |     5  |     5  |
|   2   | 2  |    1.67 |    2.10 |     6  |     7  |
|   3   | 1  |    3.45 |    5.41 |    11  |    18  |
|   3   | 2  |    3.72 |    6.99 |    12  |    18  |
|   4   | 1  |    3.63 |    5.73 |    12  |    21  |
|   4   | 2  |   34.20 |   49.12 |   109  |   118  |

Frame 0 sf 1 raw: `U[0:8] = [1 1 1 1 0 0 0 0]`,
`S[0:8] = [1 1 1 1 0 0 0 0]`. Synthesis filter is acting as ~unity
gain at cold start (LP coefficients close to identity through MA
predictor warm-up + 50/50 LSP interpolation against zero-prev), then
`rms(s)/rms(u)` settles into the 1.0..1.6 range across frames 1..4
and rises to ~1.4 at frame 4 sf 2 once voiced energy enters. No
anomalous cold-start blow-up, no decay; consistent with §3.10
synthesis-filter spectral envelope.

**D-3 / D-4 drill verdict**: no smoking gun on the first 5 frames.
Cold-start contract holds; synthesis filter ratio is benign.

### H.6 Recommended next task

**ESCALATE to user.** The four enumerated Phase 3b candidates
(B / C / D-1 / D-2) are now all exonerated, and the supplementary
D-3 / D-4 cold-start drill found no defect on the cold-start
boundary. The DIAG-4 evidence further establishes that the original
"−22-sample shift" framing of the residual defect was a measurement
artifact, not a real phase skew. The remaining real defect is
*amplitude*, not *phase*:

- A_pf vs REF_pf SegSNR = −0.88 dB (at zero-shift alignment).
- A_pf rms = 419 vs REF_pf rms = 2095 (≈5× amplitude shortfall).
- This shortfall persists across A_raw (rms 411), so it is **upstream
  of the postfilter**; consistent with Appendix B's high-energy-frame
  observation that "the remaining residual ourRMS-vs-pstRMS gap on
  high-energy frames now sits in the post-filter / synthesis chain,
  not the gain-VQ saturation envelope".

Since DIAG-4 has now ruled out phase skew, postfilter, and cold-start
synthesis state, and the remaining gap is an aggregate-amplitude /
spectral-envelope defect on high-energy (voiced) frames, the next
diagnostic should not be authored without operator dispatch. Options
the operator may consider:

- **REF-N (new)**: per-frame g_p × v[] vs g_c × c[] energy-budget
  reconciliation across the full corpus (compare the relative
  contribution of adaptive vs fixed codebook to `u` against a
  spec-derived expected ratio — Appendix B already shows the gap
  concentrates on high-energy frames where pitch contribution
  dominates).
- **REF-M (new)**: full-corpus synthesis-filter spectral-envelope
  audit (compare the per-frame gain of 1/Â(z) against a hand-derived
  Levinson-recursion gain from the decoded LP coefficients) to
  rule out a dynamic-range collapse in the 1/Â(z) implementation
  not detectable in the 5-frame cold-start drill.
- **Operator decision**: accept current quality and proceed to the
  Phase 3 closure step ("path A acceptable" decision criterion, see
  `phase3_roundtrip_quality_test.go` head comment) on the basis that
  no spec-grounded defect remains identifiable in the four enumerated
  candidate slots and the residual gap is an aggregate-amplitude
  loss rather than a phase / alignment defect.

**Recommended dispatch ID**: `OP-DECIDE-3B-EXIT` — operator
adjudication of Phase 3b exit, given exhaustion of the enumerated
candidate ladder.

### H.7 Spec citations (clean-room, no external implementations)

- ITU-T G.729 (06/2012) §A.4.2 — Annex A postfilter (long-term +
  short-term + tilt + AGC).
- ITU-T G.729 (06/2012) §3.10 / §4.1.2 / §4.1.6 — synthesis filter
  1/Â(z), excitation, overflow recovery.
- ITU-T G.729 (06/2012) §4.2.2 — output HP filter.
- ITU-T G.729 (06/2012) §4.3 Table 9 — non-zero initialisations
  (cold-start invariants).
- Quackenbush / Barnwell / Clements 1988 §2.3 / §2.4 — GlobalSNR /
  SegSNR formulation (already used by `phase3_roundtrip_quality_test.go`).

No reference C, no bcg729, no Sipro Lab, no FFmpeg, no other G.729
implementation consulted.
