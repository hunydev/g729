# G.729 Gain Reconstruction PDF Audit

Date: 2026-05-10

Scope: clean-room audit of gain quantization cost scaling, fixed-codebook
gain prediction, gain reconstruction, taming, and encoder state commit.

Sources used:

- ITU-T Recommendation G.729 (03/96), sections 3.9, 3.10, and 4.1.5.
- ITU-T Recommendation G.729 Annex A (11/96), sections A.3.9 and A.3.10.

Clean-room boundary:

- Only the official PDF text was inspected.
- The ITU package `Software.zip` entries were not extracted or opened.
- No external G.729 implementation source was inspected.

## PDF Requirements

G.729 section 3.9 defines the weighted-error gain objective:

```text
E = xTx + gp^2 yTy + gc^2 zTz - 2 gp xTy - 2 gc xTz + 2 gp gc yTz
z(n) = sum c(i) h(n-i)
```

Section 3.9.1 defines:

```text
gc = gamma * gc'
E_code = 10 log10((1/40) * sum c(n)^2)
E_tilde(m) = sum b_i * U_hat(m-i), b = [0.68, 0.58, 0.34, 0.19]
gc' = 10^((E_tilde(m) + 30 - E_code) / 20)
U(m) = 20 log10(gamma)
```

Section 3.9.2 defines the conjugate gain codebooks:

```text
gp_hat = GBK1[GA][0] + GBK2[GB][0]
gc_hat = gc' * (GBK1[GA][1] + GBK2[GB][1])
```

It then preselects four first-stage candidates by fixed-gain proximity and
eight second-stage candidates by pitch-gain proximity before the final 4x8
weighted-error search.

Section 4.1.5 says the decoder reconstructs the same `gp_hat` and `gc_hat`
from the received gain-codebook indices and the predicted fixed-codebook
gain `gc'`.

## Implementation Mapping

| PDF item | Current implementation | Classification |
| --- | --- | --- |
| `z(n) = c*h` | `fcbsearch.FilterCode` consumes `c` Q13 and `h` Q12, then stores `z` Q12. | Aligned |
| Eq. 63 cost | `gainquant.SearchConjugate` builds common-scale A/B/C/D/F correlations and evaluates the same quadratic objective over the 4x8 preselected candidates. | Aligned |
| `gc'` prediction | `PredictedGcQ12Wide` computes fixed-codebook energy, MA-predicted log energy, and `2^log2(gc')` in Q12. | Aligned |
| Eq. 73 | `SearchConjugate` and `ReconstructWide` sum the physical GBK pitch-gain entries into `gpQ14`. | Aligned |
| Eq. 74 | `SearchConjugate` uses `gamma_hat * gc'` Q12 for the search cost; `ReconstructWide` folds `gamma_hat` into the decoder-shared mantissa/exponent gain representation. | Aligned |
| Section 4.1.5 decoder mirror | `gain.Decoder.Decode` and `gainquant.ReconstructWide` share the same log-domain gamma folding and mantissa/exponent split. | Aligned |
| `U(m) = 20 log10(gamma)` | `gainquant.UpdatePastQuaEn` advances the predictor FIFO from `gammaCQ13`, independent of the final mantissa/exponent representation. | Aligned |
| Eq. A.9 excitation commit | `synth.BuildExcitation` computes `gp_hat*v + gc_hat*c` with the decoder-side arithmetic. | Aligned |
| Eq. A.10 weighted-error commit | `applyGainQ14ToQ0` and `applyGcToQ12` compute `x - gp_hat*y - gc_hat*z` in the Q0 target domain. | Aligned |
| Taming | `gainquant.Tame` is a bounded predicted-overflow guard, but it did not fire on the current problem sample. | Not current cause |

## Q-Format Notes

The fixed-codebook vector is stored as Q13. Therefore:

- `sum c(n)^2` is accumulated as a Q26 energy before the log correction.
- `z = c*h` is stored as Q12 because `c` Q13 times `h` Q12 is shifted back
  by 13.
- `gamma_hat * gc'` is represented as Q12 in the gain-search objective.
- The committed fixed gain is not forced into a saturated Word16 Q12 value.
  It is reconstructed as `(gcMantQ14, gcExp)` and applied to `z` Q12 or `c`
  Q13 using the matching shift derivations.

This avoids the older failure mode where large fixed-codebook gains collapsed
when stored as a single int16 Q12 value.

## Problem-Sample Diagnostics

Problem sample:

```text
testdata/external/user_quality_audio.m4a
```

Gain cost-model audit:

```text
G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_audio.m4a \
G729_EXTERNAL_SAMPLE_GAIN_COST_MODEL_AUDIT=1 \
  go test -run TestExternalSampleGainCostModelAuditDiagnostic -count=1 -v
```

Results:

```text
bounded: subframes=2968 fullCost==linear 99.97% preCost==linear 100.00% core==preCost 100.00% fullLinearInPreselect 27.43%
wide:    subframes=2968 fullCost==linear 97.54% preCost==linear 99.43% core==preCost 100.00% fullLinearInPreselect 47.04%
```

Interpretation:

- The integer Eq. 63 preselected-candidate ordering matches a direct float
  residual almost exactly on the problem sample.
- Core's `SearchConjugate` output matches the preselected Eq. 63 winner.
- The major remaining difference is still whether the full-search optimum is
  inside the Annex-A-style preselected set, not a broken Q-format cost formula.

Taming audit:

```text
G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_audio.m4a \
G729_EXTERNAL_SAMPLE_TAMING=1 \
  go test -run TestExternalSampleTamingDiagnostic -count=1 -v
```

Results:

```text
core-production    GlobalSNR 5.21 SegSNR 4.37 Peak 32768 NearClip 2 subfr 2968 tame 0 maxRaw 22215 maxDelta 0
quality-production GlobalSNR 5.94 SegSNR 4.77 Peak 30732 NearClip 0 subfr 2968 tame 0 maxRaw 22215 maxDelta 0
core-helper-tame   GlobalSNR 4.77 SegSNR 4.26 Peak 30746 NearClip 0
core-helper-no-tame GlobalSNR 4.77 SegSNR 4.26 Peak 30746 NearClip 0
```

Interpretation:

- Taming does not fire on the current problem sample in either Core or
  Quality.
- The remaining Core near-clips are not explained by a taming ceiling,
  stale predicted-overflow state, or a mismatch between raw and committed
  pitch gain.

Focused 2.92s-2.95s gain trace:

```text
G729_EXTERNAL_SAMPLE_QUALITY=testdata/external/user_quality_audio.m4a \
G729_EXTERNAL_SAMPLE_GAIN_TRACE=1 \
G729_EXTERNAL_GAIN_TRACE_FRAMES=292:294 \
  go test -run TestExternalSampleGainTraceDiagnostic -count=1 -v
```

Selected observations:

- Core uses `T=40` in the clip window, so fixed-codebook harmonic enhancement
  is bypassed for those subframes.
- Core's `gcOptQ12` is zero in these rows, but the standard preselected
  conjugate codebook can still select large reconstructed fixed gains because
  GB preselection is driven by pitch-gain proximity.
- Quality avoids the decoded near-clips through its standard-payload native
  gain search and decoder-in-loop repair path, not through a different gain
  reconstruction formula.

## Finding

No Core production mismatch was found in gain reconstruction, Q-format
application, predictor FIFO update, or taming for the current problem sample.

The remaining Core quality gap is best characterized as a gain-codebook
selection/preselection limitation under the standard Annex-A-style search,
not a broken implementation of `gc = gamma * gc'` or the decoder-side gain
reconstruction path.

No production code change is recommended from this audit. The useful next
clean-room surfaces are:

- a numeric oracle for the exact Annex A reduced-complexity fixed-codebook
  tree subset, if exact Core alignment is required;
- a bounded Quality-profile heuristic review, if the goal is product audio
  rather than stricter Core behavior.
