# Decoder ITU Vector Validation

Date: 2026-05-12
Last updated: 2026-05-15

Scope: decoder credibility gate based on fixed ITU Annex A test-vector
bitstreams and companion reference PCM outputs.

Clean-room boundary:

- No ITU reference C, bcg729, FFmpeg, Sipro, or other G.729 implementation
  source is inspected.
- This gate consumes only ITU test-vector data files already present under
  `testdata/itu/G729_Release3/g729AnnexA/test_vectors`.
- The comparison target is numeric PCM output, not implementation code.

## Why This Supersedes PESQ For Decoder Validation

For decoder validation, the input bitstream is fixed. The strongest practical
test is therefore:

```text
ITU .BIT -> local decoder -> PCM
ITU .PST reference PCM
```

and then sample-level comparison. If output is bit-exact, or if any accepted
delta is explicitly justified by a documented fixed/floating-point tolerance,
the decoder has much stronger evidence than an objective listening-quality
score.

PESQ NB can remain a legacy VoIP quality diagnostic for end-to-end encoder
work, but it is not the primary decoder conformance gate. Public decoder
credibility should be based on vector PCM equality first.

## Harness

The new opt-in test is:

```sh
G729_DECODER_ITU_VECTOR_VALIDATION=1 \
go test ./internal/decoder -run TestDecoderITUVectorValidation -count=1 -v
```

Hard-gate mode:

```sh
G729_DECODER_ITU_VECTOR_VALIDATION=1 \
G729_REQUIRE_DECODER_ITU_VECTOR_EXACT=1 \
go test ./internal/decoder -run TestDecoderITUVectorValidation -count=1 -v
```

Stage trace:

```sh
G729_DECODER_ITU_VECTOR_TRACE=1 \
G729_DECODER_ITU_VECTOR_TRACE_VECTOR=ALGTHM \
G729_DECODER_ITU_VECTOR_TRACE_MODE=first-diff \
go test ./internal/decoder -run TestDecoderITUVectorFirstDiffTrace -count=1 -v
```

Trace modes:

- `first-diff`: first non-exact frame/sample.
- `worst-frame`: frame with the largest squared-error sum.
- `max-sample`: frame containing the largest single absolute sample delta.

Default scope:

```text
G729_DECODER_ITU_VECTOR_SCOPE=annexa-good
```

This includes:

- `ALGTHM`
- `SPEECH`
- `FIXED`
- `LSP`
- `PITCH`
- `TAME`
- `TEST`
- `OVERFLOW`

`G729_DECODER_ITU_VECTOR_SCOPE=all` also includes `ERASURE` and `PARITY`.
Those belong to the separate bad-frame concealment/parity behavior surface and
should not block the ordinary-good-frame decoder gate.

## Current Result

Command:

```sh
G729_DECODER_ITU_VECTOR_VALIDATION=1 \
go test ./internal/decoder -run TestDecoderITUVectorValidation -count=1 -v
```

Result:

| Vector | Frames | Bad frames | Exact frames | Exact samples | First diff | Max abs delta | Mean abs delta | RMS delta |
| --- | ---: | ---: | ---: | ---: | --- | ---: | ---: | ---: |
| `ALGTHM` | 35 | 0 | 0.00% | 3.82% | 0:2 | 5247 | 140.63 | 411.49 |
| `SPEECH` | 3750 | 0 | 4.45% | 16.11% | 0:0 | 5601 | 19.62 | 59.50 |
| `FIXED` | 120 | 0 | 0.00% | 21.20% | 0:3 | 499 | 5.95 | 18.82 |
| `LSP` | 2232 | 0 | 0.27% | 2.85% | 5:40 | 799 | 30.74 | 56.04 |
| `PITCH` | 1835 | 0 | 0.00% | 2.72% | 0:2 | 6526 | 50.60 | 133.47 |
| `TAME` | 128 | 0 | 0.00% | 0.54% | 0:41 | 15798 | 4847.13 | 6243.47 |
| `TEST` | 176 | 0 | 0.57% | 8.76% | 1:40 | 1869 | 18.72 | 58.69 |
| `OVERFLOW` | 384 | 0 | 0.00% | 0.22% | 0:41 | 65535 | 6449.57 | 9710.74 |
| `TOTAL` | 8660 | 0 | 2.01% | 8.79% | 0:0 | 65535 | 419.73 | 2282.48 |

Interpretation:

- The decoder is not yet ITU-vector bit-exact.
- This is a stronger blocker than any PESQ score for decoder credibility.
- Existing FFmpeg black-box and listening-quality gates remain useful
  interoperability/quality checks, but they do not replace vector equality.

Update after the synthesis overflow recovery fix:

- `internal/synth` now follows the §3.10 divide-by-4 / multiply-by-4 recovery
  path instead of the previous divide-by-2 / multiply-by-2 fallback.
- SPEECH and Asterisk/FFmpeg black-box localization metrics were unchanged in
  the ordinary speech path.
- At that checkpoint, the stress-vector surface improved: `OVERFLOW` max abs
  delta dropped from `65535` to `36943`, and total RMS delta dropped from
  `4770.37` to `3651.96`.
- The decoder is still not ITU-vector exact; this is a blocker reduction, not
  conformance completion.

Update after the LP polynomial recurrence fix:

- `LSPToLP` now follows the verifier-observed Q24 reduced-polynomial path and
  only promotes the post-transform sums to Q28.
- `decoder_tame_lp_polynomial_step_expected.csv` is exact `2640/2640`.
- `decoder_tame_lsp_pipeline_expected.csv` remains exact `1044/1044`.
- `decoder_tame_lp_full_expected.csv` improved from `2793/2816` to
  `2816/2816` exact; the prior repeated 1-LSB `lp_a_q12[4]` mismatch is gone.
- Ordinary-good vector RMS improved overall, especially SPEECH and PITCH.
  TAME and OVERFLOW still have large late-frame drift, so the next decoder
  conformance work should target gain/excitation history rather than LP.

Update after the fixed-gain Q1 quantization fix:

- Strict decoder gain reconstruction now quantizes the final fixed-codebook
  gain to Q1 before the native `(gcMantQ14, gcExp)` split. The encoder
  `gainquant` local reconstruction mirrors the same commit path.
- `decoder_tame_gain_internals_expected.csv` improved to exact `1394/4864`;
  `fixed_gain_q14` improved to exact `192/256` while `bitstream_ga`,
  `bitstream_gb`, and `gamma_q13` remain exact `256/256`.
- `decoder_tame_full_stage_expected.csv` improved to exact `24317/122112`;
  `fixed_gain_q14` is exact `192/256` and `fixed_contrib_q0` is exact
  `9938/10240`.
- Final reference-PCM comparison is exact `67417/740800` (`9.10%`) across all
  ten Annex A vectors. The ordinary-good vector gate is still not exact, but
  total RMS is down to `2282.48` after this fix.

## PST Output Failure Frontier

When stage-level verifier rows cannot be independently completed, the next
clean-room fallback is to localize against only final ITU output:

```text
ITU .BIT -> local decoder -> PCM
ITU .PST reference PCM
```

The opt-in frontier harness is:

```sh
G729_DECODER_ITU_VECTOR_FRONTIER=1 \
go test ./internal/decoder -run TestDecoderITUVectorFailureFrontier -count=1 -v
```

Optional controls:

- `G729_DECODER_ITU_VECTOR_SCOPE`, default `annexa-good`.
- `G729_DECODER_ITU_VECTOR_FRONTIER_TOP`, default `3`, capped at `20`.

The report keeps the first tiny mismatch visible, but also reports the first
sample crossing material absolute-delta thresholds and the worst frames by
frame RMS. This prevents the debug loop from over-focusing on frame-0
rounding/HP-domain differences when larger state drift appears later.

Current frontier summary:

| Vector | First diff | First >=1024 | First >=4096 | Worst RMS frame | Worst max frame |
| --- | --- | --- | --- | --- | --- |
| `ALGTHM` | `0:2 d=1` | `13:17 d=1152` | `15:6 d=4377` | `15:1517.94` | `15:5247` |
| `SPEECH` | `0:0 d=2` | `43:23 d=1230` | `2732:48 d=5555` | `2732:1106.62` | `2732:5555` |
| `FIXED` | `0:3 d=1` | `-` | `-` | `1:95.37` | `49:497` |
| `LSP` | `0:40 d=2` | `-` | `-` | `1018:428.58` | `1018:813` |
| `PITCH` | `0:2 d=1` | `21:40 d=2530` | `561:35 d=4615` | `680:1378.79` | `680:6532` |
| `TAME` | `0:41 d=30` | `11:42 d=1041` | `31:58 d=4178` | `123:11025.91` | `127:15672` |
| `TEST` | `0:40 d=2` | `79:34 d=1644` | `-` | `79:432.09` | `79:1859` |
| `OVERFLOW` | `0:41 d=30` | `19:1 d=1474` | `19:6 d=5848` | `237:36074.13` | `237:62595` |

Interpretation:

- `FIXED`, `LSP`, and `TEST` remain useful because their worst-frame deltas are
  bounded compared with the severe vectors.
- `TAME`, `OVERFLOW`, and `PITCH` are the strongest local debug targets because
  large deltas appear early and dominate RMS/max-error surfaces.
- `SPEECH` still matters as the ordinary-path credibility gate, but its first
  `>=1024` delta appears much later than the synthetic stress vectors.
- This frontier test is not an oracle replacement. It is a clean-room triage
  tool for choosing the next decoder fix when independent stage oracles are
  unavailable.

## PST Variant-Audit Reactivation

After the stream-start fixed-codebook pitch-enhancement fix, several opt-in
variant audits had stale production mirrors: their local test pipeline still
computed the pitch-enhancement beta directly from `prevGpQ14`, so the first
subframe used the lower clamp instead of the production stream-start upper
value. The diagnostic mirrors now call the production beta helper and update
the previous-gain state through the production helper.

The ITU-vector variant probes also now use the same lenient G.192 loading
policy as the vector-validation gate, so `OVERFLOW.BIT` frame `19` with
`0x0000` softbit words can be audited consistently.

Current PST-referenced upstream variant result:

| Vector | Production gSNR | Best simple variant | Variant gSNR | Interpretation |
| --- | ---: | --- | ---: | --- |
| `SPEECH` | `23.29` | production | `23.29` | Simple stage removal/scale/reset is disqualified. |
| `TAME` | `6.50` | `fixed_gain_half` | `8.71` | TAME pressure is fixed-gain / gain-history shaped, not ACB-shaped. |
| `PITCH` | `14.14` | production | `14.14` | Simple ACB and gain-scale variants are disqualified. |
| `OVERFLOW` | `-1.58` | `pitch_gain_cap_0p95` | `0.50` | Stress surface is pitch-gain/overflow shaped, but decoder-side re-taming is not a spec-safe production fix. |

Current PST-referenced ACB-specific result:

| Vector | Production gSNR | Best ACB variant | Variant gSNR | Interpretation |
| --- | ---: | --- | ---: | --- |
| `SPEECH` | `23.29` | production | `23.29` | Production ACB is best. |
| `TAME` | `6.50` | `acb_frac_sign_flip` | `6.76` | Small movement only; not material enough for an ACB fix. |
| `PITCH` | `14.14` | production | `14.14` | Production ACB is best even on the pitch vector. |
| `OVERFLOW` | `-1.58` | `acb_short_no_periodic` | `-0.03` | Useful stress diagnostic, but conflicts with the already verified short-pitch interpolation relation. |

Next local fix target:

- Do not apply decoder-side gain re-taming as a production fix unless a
  Recommendation-backed decoder clause is found; §4.1.5 reconstructs gains
  directly from the received gain-codebook index.
- Focus the next clean-room local diagnostic on TAME fixed-gain magnitude:
  gain predictor state, fixed-codebook energy domain, and `gcMant/gcExp`
  reconstruction around the high-error TAME frontier frames.

The focused gain-frontier command is:

```sh
G729_DECODER_FIXED_GAIN_FRONTIER=1 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=5 \
go test ./internal/decoder -run TestDecoderITUFixedGainFrontier -count=1 -v
```

Current `TAME` result:

| Frame | Production RMS | Fixed-gain-half RMS | Production max | Half max |
| ---: | ---: | ---: | ---: | ---: |
| `123` | `11086.67` | `633.60` | `15440` | `1543` |
| `127` | `11070.30` | `389.01` | `15798` | `988` |
| `126` | `10806.60` | `1026.26` | `15611` | `1693` |
| `122` | `10742.33` | `470.62` | `15320` | `1165` |
| `125` | `10705.31` | `1101.71` | `15371` | `1662` |

The per-subframe logs show `fixedRMS` is much smaller than `pitchRMS` at
those already-bad frames. The fixed-gain-half improvement is therefore mostly
stateful: reducing earlier fixed contribution changes the subsequent
past-excitation/adaptive vector trajectory, rather than merely subtracting a
large direct fixed contribution in the listed frame.

## External Reference-Execution Numeric Oracle

Date: 2026-05-14

A separate private verifier workspace may execute the ITU reference decoder and
export only numeric CSV artifacts. The repository must not import reference C
source, implementation names, branch descriptions, or magic-number provenance.
The current external output directory is:

```text
/home/exedev/g729_untracked/verifier-output
```

Generated files:

| File | Rows | Filled expected | Blanks | SHA256 |
| --- | ---: | ---: | ---: | --- |
| `decoder_final_pcm_expected.csv` | `740800` | `740800` | `0` | `70efc4c2b17722815f7aefeaa7f5aeaa473874077f01b7b57fe3a70d8e5af4d8` |
| `decoder_final_pcm_pst_compare.csv` | `10` | `0` | `0` | `8f4eb6edf4697c2f2d15ba86e4dfe7ce0526abebee13babcb384aaa236e387ba` |
| `decoder_tame_full_stage_expected.csv` | `122112` | `122112` | `0` | `2e7bb08284f6526d04d05fcf0813559c548b6d19eb170aa6233e3f4d2a9b6bfc` |
| `decoder_tame_gain_internals_expected.csv` | `4864` | `4864` | `0` | `71d1348a344609f4dd37935d132850fec2f21e2424ceeb74c89d875f82a351fa` |

The verifier reported that final PCM produced by the reference decoder matched
the official Annex A `.PST` files exactly for all 10 sources:

```text
ALGTHM ERASURE FIXED LSP OVERFLOW PARITY PITCH SPEECH TAME TEST
mismatches=0, max_abs_delta=0
```

This means the final-PCM oracle is equivalent to the official `.PST` files, but
the stage oracle adds a stronger TAME localization surface for fields that are
not present in `.PST`.

Opt-in gates:

```sh
G729_COMPARE_DECODER_REFERENCE_FINAL_PCM=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceFinalPCM -count=1 -v
```

```sh
G729_COMPARE_DECODER_REFERENCE_TAME_STAGE=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceTAMEFullStage -count=1 -v
```

```sh
G729_COMPARE_DECODER_REFERENCE_TAME_GAIN_INTERNALS=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderReferenceTAMEGainInternals -count=1 -v
```

Strict modes:

```text
G729_REQUIRE_EXACT_DECODER_REFERENCE_FINAL_PCM=1
G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_STAGE=1
G729_REQUIRE_EXACT_DECODER_REFERENCE_TAME_GAIN_INTERNALS=1
```

Path override:

```text
G729_DECODER_REFERENCE_ORACLE_DIR=/path/to/verifier-output
```

The large CSVs are intentionally not committed. They are verifier-produced
numeric artifacts and should remain outside the MIT repository unless there is
an explicit release decision to publish only the numeric vectors.

Current non-strict compare against the external reference oracle:

| Gate | Exact | Mismatches | First mismatch | Max abs | Interpretation |
| --- | ---: | ---: | --- | ---: | --- |
| Final PCM, all 10 vectors | `67417/740800` (`9.10%`) | `673383` | `0:0` | `65535` | Same failure surface as the official `.PST` vector gate; total RMS is lower after the fixed-gain Q1 commit path. |
| TAME full stage | `24317/122112` (`19.91%`) | `97795` | frame `0` sub `0` | `15798` | Stage oracle is complete enough for direct localization. |
| TAME gain internals | `1394/4864` (`28.66%`) | `3470` | frame `0` sub `0` | `32768` | Gain VQ indices and γ̂ are exact; remaining differences are log/gain rounding and downstream state. |

TAME stage field summary:

| Field | Exact | Mismatches | Max abs | Note |
| --- | ---: | ---: | ---: | --- |
| `pitch_t_int` | `256/256` | `0` | `0` | Pitch integer decode matches. |
| `pitch_t_frac` | `256/256` | `0` | `0` | Pitch fraction decode matches. |
| `adaptive_gain_q14` | `256/256` | `0` | `0` | Adaptive gain decode matches. |
| `fixed_c_q13` | `9904/10240` | `336` | `45` | Improved by canonical `+8191/-8192` pulse endpoints, lower stream-start beta, and truncating pitch-sharpening arithmetic. |
| `fixed_gain_q14` | `192/256` | `64` | `32768` | Final Q1 commit-path quantization fixed the dominant scalar mismatch. |
| `fixed_contrib_q0` | `9938/10240` | `302` | `5` | Fixed contribution is now mostly reduced to small rounding differences. |
| `adaptive_v_q0` | `426/10240` | `9814` | `306` | Drifts after prior excitation diverges. |
| `pitch_contrib_q0` | `438/10240` | `9802` | `302` | Drifts with adaptive-vector history. |
| `excitation_u_q0` | `415/10240` | `9825` | `302` | Drifts with fixed contribution and past-excitation feedback. |
| `synth_s_q0` | `151/10240` | `10089` | `8104` | Downstream synthesis result. |
| `postfilter_s_q0` | `71/10240` | `10169` | `8133` | Downstream postfilter result. |
| `pcm_q0` | `55/10240` | `10185` | `15798` | Final output for TAME. |

TAME gain-internals field summary:

| Field | Exact | Mismatches | Max abs | Note |
| --- | ---: | ---: | ---: | --- |
| `bitstream_ga` | `256/256` | `0` | `0` | Transmitted gain stage-1 indices match. |
| `bitstream_gb` | `256/256` | `0` | `0` | Transmitted gain stage-2 indices match. |
| `gamma_q13` | `256/256` | `0` | `0` | Fixed-codebook correction γ̂ now uses the non-saturating joint sum. |
| `log2_gc_q10` | `65/256` | `191` | `5` | dB-to-log2 conversion is close but not bit-exact. |
| `gc0_q14` | `15/256` | `241` | `93` | Q15 Pow2 fraction preserves more precision than the previous Q10 path. |
| `fixed_gain_q14` | `192/256` | `64` | `32768` | Final Q1 quantization is mirrored, but 64 rows still diverge. |
| `predicted_energy_q10` | `17/256` | `239` | `7` | Follows the MA predictor FIFO and `U(m)` rounding. |
| `u_current_q10` | `21/256` | `235` | `5` | γ̂ log update is close but not exact. |
| `ec_bar_q10` | `0/256` | `256` | `22` | Fixed-codebook energy log term has small fixed-point rounding differences. |
| `fixed_codebook_energy_q26` | `109/256` | `147` | `2500335` | Later rows also include upstream `fixed_c_q13` drift. |

Immediate decoder-exact target:

- The verifier has removed the previous blank-oracle blocker.
- Since pitch parameters and adaptive gain already match, and the first FCB
  endpoint/sharpening defects are fixed, the next strict local target is
  fixed-gain reconstruction at frame 0/subframe 0.
- Once those first-frame fixed-path scalar/vector mismatches are resolved,
  re-run the full TAME stage oracle to see whether later `past_exc` and ACB
  drift collapses.

Additional gain-reconstruction candidate probes:

```sh
G729_DECODER_GAIN_AUDIT=1 \
G729_DECODER_GAIN_VECTOR=TAME \
go test ./internal/decoder -run TestPhase3jGainVariantAudit_SPEECH -count=1 -v
```

The gain audit now uses selectable ITU vectors, the lenient G.192 reader, and
the same pitch-enhancement state helpers as production. Current result:

| Vector | Production gSNR | Best tested gain candidate | Candidate gSNR | Interpretation |
| --- | ---: | --- | ---: | --- |
| `SPEECH` | `23.29` | production | `23.29` | Global gain Q-format changes are disqualified. |
| `TAME` | `6.50` | `gain_ec_q25` | `12.54` | Strong TAME-only gain/excitation-history clue. |
| `PITCH` | `14.14` | production | `14.14` | Global gain Q-format changes are disqualified. |
| `OVERFLOW` | `-1.58` | `gain_ec_q13` | `0.04` | Small stress-vector movement with worse correlation; not a production fix. |

The focused gain candidate frontier is:

```sh
G729_DECODER_GAIN_CANDIDATE_FRONTIER=1 \
G729_DECODER_GAIN_CANDIDATE_VECTOR=TAME \
G729_DECODER_GAIN_CANDIDATE=gain_ec_q25 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=5 \
go test ./internal/decoder -run TestDecoderITUGainCandidateFrontier -count=1 -v
```

After the fixed-gain Q1 change, the diagnostic gain mirror was updated to use
the same Q1 split path and the same non-saturating `gamma_q13` sum as
production. With that current mirror, full-file `gain_ec_q25` on `TAME`
improves aggregate RMS from `6226.68` to `2375.67`. It strongly reduces the
severe late frames `122`, `123`, `125`, `126`, and `127`, but it also regresses
early frames such as `3`, `4`, `5`, `6`, and `7`.

The cutover probe is:

```sh
G729_DECODER_GAIN_CANDIDATE_CUTOVER=1 \
G729_DECODER_GAIN_CANDIDATE_VECTOR=TAME \
G729_DECODER_GAIN_CANDIDATE=gain_ec_q25 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=8 \
go test ./internal/decoder -run TestDecoderITUGainCandidateCutover -count=1 -v
```

Current cutover result:

| Candidate | Best cutover frame | Aggregate RMS | Production RMS |
| --- | ---: | ---: | ---: |
| `gain_ec_q25` | `1` | `2364.65` | `6226.68` |

Interpretation:

- These are not safe production changes because SPEECH/PITCH reject the same
  gain variants.
- The TAME error is strongly state-history dependent: applying the candidate
  from near the stream start beats production and full-file candidate
  application, while still badly regressing early frames.
- The next clean-room numeric target should focus on TAME frames `20..30` and
  `122..127`: gain predictor FIFO, decoded gain taps, fixed contribution,
  excitation history, adaptive vector, synthesis output, and final PST PCM.

Follow-up verifier result for that full numeric oracle request:

- Independently derivable from `TAME.BIT` / `TAME.PST`: final `pst_pcm_q0`
  rows and transmitted bitstream indices such as `GA`/`GB`.
- Not independently derivable under the current clean-room inputs: gain
  predictor FIFO, decoded `gp/gamma/gc`, `ecBar/logGain/log2Gc/uCurrent`,
  adaptive vector, fixed contribution, excitation, and synthesis rows.
- Reason: those internal stages require a full forward decode from frame `0`,
  including support tables that are not fully available from Recommendation
  text/math alone. The verifier must not fill them from local implementation
  values or other implementation table sources.
- Therefore no additional verifier request should be made for the same full
  internal TAME oracle unless the allowed clean-room input set changes. The
  next decoder work should proceed with PST-only frontier/cutover diagnostics
  plus direct spec/Q-format audit of local code.

Frame-window scan update:

```sh
G729_DECODER_GAIN_CANDIDATE_WINDOW=1 \
G729_DECODER_GAIN_CANDIDATE_VECTOR=TAME \
G729_DECODER_GAIN_CANDIDATE=gain_ec_q25 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=5 \
go test ./internal/decoder -run TestDecoderITUGainCandidateWindow -count=1 -v
```

The best finite `gain_ec_q25` window is `[1,127)`, with aggregate RMS
`2364.42`. It slightly beats the `[1,128)` cutover (`2364.65`). The window
still regresses early affected frames such as `3..7`, but it dramatically
reduces the late high-energy frames `122..127`. This reinforces the current
interpretation: the candidate is a long-state damping probe, not a valid
replacement gain formula.

Pre-final-scale output audit:

```sh
G729_DECODER_ITU_OUTPUT_DOMAIN_AUDIT=1 \
go test ./internal/decoder -run TestDecoderITUOutputDomainAudit -count=1 -v
```

Current result:

| Vector | Output RMS delta | HP raw RMS delta | Best domain |
| --- | ---: | ---: | --- |
| `ALGTHM` | `403.55` | `2739.21` | output |
| `FIXED` | `16.71` | `69.58` | output |
| `LSP` | `55.23` | `334.84` | output |
| `OVERFLOW` | `9700.14` | `8963.60` | `hp_raw` |
| `PITCH` | `131.64` | `2251.16` | output |
| `SPEECH` | `54.50` | `1044.31` | output |
| `TAME` | `6226.68` | `3077.27` | `hp_raw` |
| `TEST` | `56.06` | `708.71` | output |

Pre-scale ratio audit:

```sh
G729_DECODER_ITU_PRESCALE_RATIO_AUDIT=1 \
go test ./internal/decoder -run TestDecoderITUPreScaleRatioAudit -count=1 -v
```

Current active-frame result (`.PST` frame RMS >= `500`):

| Vector | Active frames | HP raw median / PST | Output median / PST | HP near 0.5x | HP near 1.0x | Output > 1.5x |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `ALGTHM` | `29` | `0.509` | `1.014` | `29` | `0` | `0` |
| `LSP` | `555` | `0.501` | `1.003` | `550` | `0` | `0` |
| `OVERFLOW` | `383` | `0.521` | `1.042` | `169` | `24` | `60` |
| `PITCH` | `1522` | `0.503` | `1.005` | `1522` | `0` | `0` |
| `SPEECH` | `1819` | `0.502` | `1.004` | `1792` | `0` | `0` |
| `TAME` | `127` | `0.769` | `1.538` | `25` | `62` | `67` |
| `TEST` | `78` | `0.503` | `1.007` | `78` | `0` | `0` |

Interpretation:

- A global final-output scale change is disqualified: ordinary vectors
  (`SPEECH`, `LSP`, `TEST`, and most active `ALGTHM`/`PITCH` frames) show the
  expected pattern where pre-final-scale `hp_raw` is near half of `.PST` and
  final output is near `.PST`.
- TAME and OVERFLOW are stress-vector exceptions where local `hp_raw` is already
  too large before the final `ScaleUpSat` step. TAME has `62/127` active frames
  with `hp_raw` near `.PST` amplitude and `67/127` active frames where final
  output exceeds `1.5x` `.PST`.
- The TAME worst-frame trace logs `hp_raw` directly against the PST final
  domain. On frame `123`, `hp_raw/PST=1.006` while `output/PST=2.013`.
  Because ordinary vectors confirm the final x2 path,
  this means TAME's local pre-scale chain has already grown toward final-output
  amplitude in that region.
- This is not a production fix. It narrows the next question to the upstream
  source of stress-vector pre-scale over-amplification: gain history,
  excitation history, LP/synthesis envelope, or postfilter input state before
  the final scale.

Stage-timeline audit:

```sh
G729_DECODER_ITU_PRESCALE_STAGE_TIMELINE=1 \
go test ./internal/decoder -run TestDecoderITUPreScaleStageTimelineAudit -count=1 -v
```

Current TAME summary:

- Active frames: `127`
- First `hp_raw / PST >= 0.8`: frame `66`
- First `output / PST >= 1.5`: frame `61`
- Worst output-ratio frame: frame `127`
  (`s/PST=1.035`, `spf/PST=1.036`, `hp/PST=1.007`,
  `output/PST=2.014`)
- The top late frames `117..127` have `spf/s≈1.0`, `hp/spf≈0.972`, and
  `out/hp=2.0`, so postfilter, HP, and final scaling are acting like a stable
  transfer on an already oversized pre-scale signal.

SPEECH contrast run:

```sh
G729_DECODER_ITU_PRESCALE_STAGE_TIMELINE=1 \
G729_DECODER_ITU_PRESCALE_STAGE_VECTOR=SPEECH \
G729_DECODER_ITU_PRESCALE_STAGE_TOP=8 \
go test ./internal/decoder -run TestDecoderITUPreScaleStageTimelineAudit -count=1 -v
```

SPEECH has `1819` active frames with no `hp_raw / PST >= 0.8` frame and no
`output / PST >= 1.5` frame. Its highest output-ratio rows remain around
`s/PST≈0.5` and `output/PST≈1.05`. This confirms the TAME failure is not a
global output-scale problem.

Updated localization:

- TAME over-amplification is already visible at synthesis output `S`, before
  postfilter/HP/final scaling.
- Because TAME `uRMS` remains small while `sRMS` approaches final-domain PST
  amplitude, the next local target is the LP synthesis state and the
  excitation/gain history feeding it, not a postfilter or output writer fix.

Synth overflow recovery check:

```sh
G729_DECODER_SYNTH_OVERFLOW_AUDIT=1 \
G729_DECODER_SYNTH_OVERFLOW_VECTOR=TAME \
go test ./internal/decoder -run TestPhase3fSynthOverflowRecoveryAudit_SPEECH -count=1 -v
```

Current result:

| Variant | gSNR | Corr | Pass-1 overflow | Pass-2 overflow | Diff samples |
| --- | ---: | ---: | ---: | ---: | ---: |
| `production_quarter_x4` | `6.50` | `0.972` | `0` | `0` | `0` |
| `legacy_half_x2` | `6.50` | `0.972` | `0` | `0` | `0` |
| `no_recovery_pass1_sat` | `6.50` | `0.972` | `0` | `0` | `0` |

The synthesis overflow-recovery branch is inactive on TAME and SPEECH, so the
TAME over-amplification is not a recovery-scale bug.

Verifier-numeric oracle injection probes:

```sh
G729_DECODER_TAME_ORACLE_LP_PROBE=1 \
go test ./internal/decoder -run TestDecoderTAMEOracleLPCoeffProbe -count=1 -v

G729_DECODER_TAME_ORACLE_ACB_PROBE=1 \
go test ./internal/decoder -run TestDecoderTAMEOracleACBVectorProbe -count=1 -v
```

Current result:

| Probe | Overridden rows | gSNR | Corr | Delta gSNR | Diff samples |
| --- | ---: | ---: | ---: | ---: | ---: |
| `oracle_lp` | `6` subframes | `6.49` | `0.972` | `-0.00` | `788` |
| `oracle_acb` | `6` subframes | `7.95` | `0.974` | `+1.45` | `880` |

Interpretation:

- The verifier-provided `lp_a_q12` rows for TAME frames `117..119` do not move
  final PST agreement, so the small LP coefficient drift in that artifact is
  not the dominant late-envelope cause.
- Injecting verifier-provided `adaptive_v_q0` rows for the same six subframes
  gives a material improvement. This makes past-excitation/adaptive-codebook
  history the strongest current local target.
- Because the verifier could not independently derive frame `0..116`
  excitation history under the current clean-room inputs, the next local work
  should use PST-only and numeric-oracle injection probes to narrow which
  earlier gain/fixed contribution changes first corrupt the past-excitation
  FIFO.

Upstream fixed-gain window scan:

```sh
G729_DECODER_UPSTREAM_VARIANT_WINDOW=1 \
G729_DECODER_UPSTREAM_WINDOW_CANDIDATE=fixed_gain_half \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=8 \
go test ./internal/decoder -run TestDecoderITUUpstreamVariantWindow -count=1 -v
```

Current best windows:

| Start | End | Length | Candidate RMS | Note |
| ---: | ---: | ---: | ---: | --- |
| `3` | `120` | `117` | `1158.46` | best |
| `3` | `122` | `119` | `1161.95` | near-best |
| `3` | `127` | `124` | `1165.21` | near-best |

The best window's largest improvements are frames `120..127`, even though the
candidate is disabled at frame `120` in the best `[3,120)` run. This is strong
evidence that the damaging state is accumulated during frames `3..119` and
then expressed through the adaptive-codebook/past-excitation history in the
late TAME frames. Early frames `3..10` regress under the same diagnostic, so
`fixed_gain_half` is still not a production formula; it is a localization
probe.

Subframe-resolution window scan:

```sh
G729_DECODER_UPSTREAM_VARIANT_SUBFRAME_WINDOW=1 \
G729_DECODER_UPSTREAM_WINDOW_CANDIDATE=fixed_gain_half \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=10 \
go test ./internal/decoder -run TestDecoderITUUpstreamVariantSubframeWindow -count=1 -v
```

Current best windows:

| Subframe start | Subframe end | Subframes | Frame start | Frame end | Candidate RMS |
| ---: | ---: | ---: | ---: | ---: | ---: |
| `6` | `240` | `234` | `3` | `120` | `1158.46` |
| `6` | `244` | `238` | `3` | `122` | `1161.95` |
| `6` | `245` | `239` | `3` | `123` | `1162.91` |

The best boundary is frame `3` subframe `0` through frame `119` subframe `1`
inclusive (`[6,240)` in global subframe numbering). This keeps the target on
subframe-wise gain/excitation history rather than a frame-output artifact.

History timeline:

```sh
G729_DECODER_TAME_HISTORY_TIMELINE=1 \
G729_DECODER_UPSTREAM_WINDOW_CANDIDATE=fixed_gain_half \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=8 \
go test ./internal/decoder -run TestDecoderTAMEHistoryTimeline -count=1 -v
```

Current summary:

- Window: `[6,240)` global subframes (`3/0` through `119/1`).
- Output RMS: production `6226.68`, candidate `1158.46`.
- At window start, direct fixed contribution is exactly halved, but total
  excitation barely changes: frame `3/0` has `fixed c/p=0.500`, `u c/p=0.997`,
  and `s c/p=1.006`.
- By late TAME frames, the accumulated FIFO/ACB effect dominates: frame `118/1`
  has `past c/p=0.903`, `v c/p=0.914`, `u c/p=0.921`, and `s c/p=0.581`.
- After the window is disabled, direct fixed contribution returns to `1.000x`,
  but the reduced past-excitation/adaptive vector persists into frames
  `120..122`, while final synthesis amplitude remains around `0.58x`.

Interpretation:

- The late TAME improvement is not caused by halving the direct fixed
  contribution in those late frames; it persists after the candidate is off.
- The candidate works by slowly reducing the excitation FIFO and therefore the
  adaptive-codebook vector seen by later subframes.
- This keeps the next debug target on gain/fixed contribution accumulation into
  `pastExc`, especially why the spec-shaped production gain trajectory lets
  TAME's FIFO climb about `25%` higher than the damping probe.

ACB oracle shape audit:

```sh
G729_DECODER_TAME_ACB_ORACLE_SHAPE=1 \
G729_DECODER_UPSTREAM_WINDOW_CANDIDATE=fixed_gain_half \
go test ./internal/decoder -run TestDecoderTAMEACBOracleShape -count=1 -v
```

Aggregate result against the numeric `adaptive_v_q0` rows in
`decoder_tame_stage_wide_expected.csv`:

| Variant | ref RMS | got RMS | err RMS | scaled err | corr | scale |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| production | `203.36` | `366.30` | `170.91` | `38.26` | `0.9821` | `0.5453` |
| `[6,240)` fixed-gain-half | `203.36` | `333.86` | `248.43` | `150.80` | `0.6709` | `0.4087` |

Interpretation:

- Production's ACB vector shape is much closer to the oracle than the damping
  candidate (`corr 0.9821` vs `0.6709`), but its amplitude is still too high
  (`got RMS 366.30` vs `ref RMS 203.36`).
- The damping candidate improves final PST RMS by lowering long-state
  excitation energy, but it worsens the local ACB shape against the oracle.
- This argues against changing the ACB interpolation formula as the next
  production fix. The sharper target is an earlier excitation-history amplitude
  accumulation issue, preferably a shape-preserving one rather than a broad
  fixed-gain damping rule.

ACB checkpoint handoff:

```sh
G729_COMPARE_DECODER_TAME_ACB_CHECKPOINT=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEACBCheckpoint -count=1 -v
```

Current result for
`testdata/oracle/handoff/decoder_tame_acb_checkpoint_expected.csv`:

| Field | Exact | Filled | Blanks | Max abs |
| --- | ---: | ---: | ---: | ---: |
| `adaptive_gain_q14` | `6` | `6` | `182` | `0` |
| `fixed_gain_q14` | `0` | `6` | `182` | `18432` |
| `adaptive_v_q0` | `0` | `240` | `7280` | `441` |
| `excitation_u_q0` | `0` | `240` | `7280` | `447` |
| `past_exc_pre_acb_q0` | `0` | `546` | `28218` | `442` |

Interpretation:

- The checkpoint did not solve onset localization: frames `26..116` remain
  blank because the verifier could not independently reconstruct the forward
  excitation/history under the current clean-room inputs.
- It does confirm that at frames `117..119`, local adaptive gain is exact while
  fixed gain is close but not exact, and the larger failure is already present
  in pre-ACB history, ACB vector, and final excitation.
- The next useful oracle would need either earlier independent excitation
  history or enough clean-room support-table data to forward-decode the missing
  `26..116` range. Without that, a production damping change remains
  under-justified.

Past-excitation age map:

```sh
G729_DECODER_TAME_PAST_EXC_AGE_MAP=1 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=16 \
go test ./internal/decoder -run TestDecoderTAMEPastExcAgeMap -count=1 -v
```

Current aggregate:

- Filled `past_exc_pre_acb_q0` rows: `546`.
- Exact rows: `0`.
- Oracle RMS: `204.01`; local RMS: `386.68`.
- Error RMS: `232.02`; correlation: `0.8703`; best linear scale from local to
  oracle: `0.4592`.

Interpretation:

- The mismatch is not isolated to a single FIFO index band. All filled age bands
  have broadly similar RMS/scale behavior, with local history about `2.18x`
  larger than the oracle.
- The verifier-filled history maps mostly to source subframes `117/0..119/0`;
  `117/0` source excitation is already over-amplified (`oracle RMS 211.81`,
  local RMS `401.34`), so the causal origin is earlier than `117/0`.

ACB replay from oracle history:

```sh
G729_DECODER_TAME_ACB_ORACLE_REPLAY=1 \
go test ./internal/decoder -run TestDecoderTAMEACBOracleReplay -count=1 -v
```

Current result:

| Frame/sub | Pitch delay | Exact | Error RMS |
| --- | ---: | ---: | ---: |
| `119/0` | `32.+0` | `true` | `0.00` |
| `119/1` | `32.+0` | `true` | `0.00` |

Interpretation:

- Where the verifier supplied a complete `past_exc_pre_acb_q0[0..152]`, feeding
  it into the local ACB interpolation reproduces `adaptive_v_q0` exactly.
- This strongly argues against the ACB interpolation formula as the remaining
  TAME root cause. The remaining target is the upstream generation of
  excitation history before `117/0`.

Excitation replay from oracle ACB:

```sh
G729_DECODER_TAME_EXCITATION_ORACLE_REPLAY=1 \
go test ./internal/decoder -run TestDecoderTAMEExcitationOracleReplay -count=1 -v
```

Current aggregate:

| Path | ref RMS | got RMS | err RMS | scaled err | corr | scale | max abs |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| local `u` vs oracle `u` | `205.62` | `386.08` | `232.78` | `103.58` | `0.8639` | `0.4601` | `447` |
| oracle `v` replay through local gain/fixed | `205.62` | `205.67` | `0.33` | `0.32` | `1.0000` | `0.9997` | `2` |
| implied fixed contribution vs local fixed | `58.54` | `58.81` | `0.34` | `0.21` | `1.0000` | `0.9954` | `2` |

Interpretation:

- Once `adaptive_v_q0` is replaced by the oracle, local gain decode, fixed
  codebook contribution, and `BuildExcitation` reproduce oracle
  `excitation_u_q0` within rounding noise.
- This rules out the current subframe's gain/fixed/excitation summing path as
  the material TAME mismatch. The remaining failure is inherited from the
  earlier `pastExc` history that feeds ACB.
- The next production-relevant target is therefore not `BuildExcitation` or
  fixed-codebook scaling. It is the earlier mechanism that made `pastExc`
  approximately `2x` too large before `117/0`.

TAME history onset audit:

```sh
G729_DECODER_TAME_HISTORY_ONSET=1 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=12 \
go test ./internal/decoder -run TestDecoderTAMEHistoryOnsetAudit -count=1 -v
```

Current result:

- First sample with frame max abs delta `>=4096`: frame `31`.
- First active frame with local output RMS / PST RMS `>=1.25`: frame `31`.
- First active frame with local output RMS / PST RMS `>=1.50`: frame `61`.
- First 4-frame persistent local output RMS / PST RMS `>=1.50`: frame `61`.
- Around the fixed-gain diagnostic window start (`3/0`), output ratio is still
  near unity (`frame 3 out/PST=1.005`) even though local pre-ACB history is
  already large from the start-up state.
- By frame `31`, the ratio has grown to `1.256`, with `pastRMS≈266`,
  `vRMS≈271`, and `uRMS≈271`.
- By frame `61`, the ratio has grown to `1.527`, with `pastRMS≈308`,
  `vRMS≈312`, and `uRMS≈312`.
- In the verifier checkpoint zone (`117..127`), the ratio is already
  `1.890..2.014`, and local pre-ACB history / ACB / excitation RMS are all in
  the `~360..418` range.

Interpretation:

- Frame `117` is not the onset. It is where the verifier has enough numeric
  checkpoint rows to observe the already-accumulated failure.
- The local amplitude drift starts as a slow history accumulation after the
  frame-`3` diagnostic window boundary, becomes material around frame `31`,
  and becomes severe/persistent around frame `61`.
- This further argues against fixing late TAME by changing ACB interpolation,
  current-subframe gain summing, postfilter, HP, or final scaling. The next
  useful production audit should target the state transition that begins the
  `pastExc` growth between frames `3` and `31`.

TAME gain-energy audit:

```sh
G729_DECODER_TAME_GAIN_ENERGY_AUDIT=1 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=6 \
go test ./internal/decoder -run TestDecoderTAMEGainEnergyAudit -count=1 -v
```

Current result:

- Fixed-codebook energy saturation: `0/256` subframes.
- In the onset window (`3..31`), non-saturating int64 energy and the local
  `gain.FixedCodebookEnergy` `Word32` result are identical.
- Direct fixed contribution is much smaller than adaptive/pitch contribution
  through the onset region. Example: frame `31/0` has `pitchRMS=272.3`,
  `fixedRMS=28.1`, and `uRMS=271.1`.
- The legacy diagnostic `GcQ12Final` is frequently saturated, but the active
  excitation path uses the mantissa/exponent pair (`GcMantQ14`, `GcExp`), not
  the clamped Q12 diagnostic value.

Interpretation:

- The TAME drift is not caused by fixed-codebook energy accumulator saturation.
- The onset still looks ACB/past-excitation feedback dominated: small direct
  fixed contribution changes can perturb the recurrent `pastExc` path, but the
  immediate sample energy is mostly adaptive contribution.
- The next verifier-independent local check should therefore focus on
  pitch/ACB feedback and the `pastExc` FIFO update over frames `3..31`, not on
  widening `fixedCodebookEnergy`.

TAME ACB/FIFO feedback audit:

```sh
G729_DECODER_PITCH_ACB_AUDIT=1 \
G729_DECODER_PITCH_ACB_VECTOR=TAME \
go test ./internal/decoder -run TestPhase3hPitchACBVariantAudit_SPEECH -count=1 -v

G729_DECODER_TAME_FIFO_BALANCE=1 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=8 \
go test ./internal/decoder -run TestDecoderTAMEPastExcFIFOBalanceAudit -count=1 -v
```

Current ACB variant result:

- TAME pitch subframes: `256` total; `255` have `T_frac=0`; `152` have
  `T_int<40`.
- Production gSNR is `6.50 dB`.
- Best simple ACB variant is only `acb_frac_sign_flip` at `6.76 dB`, and this
  is not material.
- Removing short-pitch periodic/current-subframe feedback
  (`acb_short_no_periodic`) collapses agreement to `0.39 dB`.

Current FIFO balance result:

| Window | Subframes | Grow rows | Sum dPost | Avg dPost | First preRMS | Last postRMS |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `cold-start` | `8` | `7` | `424.60` | `53.07` | `0.0` | `424.6` |
| `pre-1.25x` (`3..31`) | `56` | `28` | `-175.36` | `-3.13` | `434.8` | `259.5` |
| `first-1.25x` (`31..61`) | `60` | `34` | `43.30` | `0.72` | `259.5` | `302.8` |
| `severe-rise` (`61..117`) | `112` | `61` | `78.36` | `0.70` | `302.8` | `381.1` |
| `checkpoint` (`117..127`) | `22` | `13` | `24.09` | `1.10` | `381.1` | `405.2` |

Interpretation:

- The FIFO copy/update operation is not behaving like a discrete bug. Each
  growth step is explained by incoming `U` RMS being larger than the 40 samples
  shifted out.
- The short-pitch feedback path is required; disabling it globally makes TAME
  much worse. This aligns with the separate oracle replay result where local
  ACB interpolation is exact when supplied with oracle `pastExc`.
- The local failure is therefore a long recurrent energy-balance problem:
  `pastExc` is already high after cold start, then continues to drift upward
  through `31..117`. A production fix still needs a spec-backed reason why the
  local incoming `U` trajectory differs from the reference, not an arbitrary
  FIFO or ACB damping rule.

## TAME FFmpeg/PST Localization Update

Command:

```sh
G729_DECODER_FFMPEG_ENVELOPE_AUDIT=1 \
G729_DECODER_FFMPEG_ENVELOPE_VECTOR=TAME \
go test ./internal/decoder -run TestPhase3oFFmpegEnvelopeAudit_SPEECH -count=1 -v
```

Key result:

- `TAME.BIT -> FFmpeg` matches `TAME.PST` strongly:
  `gSNR=29.13`, `seg=29.09`, `corr=0.999`.
- `TAME.BIT -> local` does not:
  `gSNR=6.50`, `seg=11.04`, `corr=0.972`.
- Local-vs-FFmpeg active-frame envelope ratio median is `2.609`, with
  `100/127` active frames above `1.5x` in the earlier pre-fix run. Current
  production after decoder fixes is milder but still over-amplified:
  ratio median `1.362`, mean `1.348`, and `56/127` active frames above `1.5x`.
- Current stage ratios remain synthesis/final-output shaped:
  `pitch/u=0.99`, `fixed/u=0.16`, `s/u=25.22`, `spf/s=1.00`,
  `hp/spf=0.97`, `out/hp=2.00`.
- Current worst-frame detail is frame `123`. It is mostly adaptive-loop driven
  (`uRMS≈386..396`, direct fixed contribution much smaller), and its raw HP
  output is already close to final-domain PST amplitude before final
  `ScaleUpSat`. Since ordinary vectors validate the final x2 shape, this points
  to upstream pre-scale over-amplification rather than a `.PST` domain flip.

The companion upstream perturbation command:

```sh
G729_DECODER_FFMPEG_UPSTREAM_AUDIT=1 \
G729_DECODER_FFMPEG_ENVELOPE_VECTOR=TAME \
go test ./internal/decoder -run TestPhase3sFFmpegUpstreamVariantAudit_SPEECH -count=1 -v
```

shows `reset_gain_each_frame` as the best diagnostic variant
(`gSNR=-3.20 -> 3.27`), but it is not a production fix: on `SPEECH.BIT` it
destroys ordinary-path agreement. The current interpretation is that TAME's
large error is a gain-predictor / excitation-history / synthesis-envelope
coupling issue, not an FFmpeg oracle mismatch and not a postfilter/HP-only
defect.

Follow-up localization added three narrower probes:

- `G729_DECODER_LSP_ROUTE_AUDIT=1 G729_DECODER_FFMPEG_ENVELOPE_VECTOR=TAME`
  found `lsp_l2_l3_swap` improves TAME global SNR from `-3.11` to `4.37 dB`,
  but the same variant destroys SPEECH (`22.54 -> 1.62 dB`). This is not a
  production fix; it indicates a TAME-specific LSP/LP-envelope sensitivity.
- `pitch_gain_cap_0p95` improves TAME under the FFmpeg audit (`-3.20 -> 2.02
  dB`), but destroys SPEECH (`20.06 -> 6.65 dB`). The energy-gated taming
  variant is byte-identical to production on both TAME and SPEECH, so the
  ordinary encoder-side taming energy condition is not the decoder fix.
- Direct LP coefficient scaling is also TAME-only: `lp_three_quarter` improves
  TAME (`-3.11 -> 1.49 dB`) but destroys SPEECH (`22.54 -> -7.45 dB`).

The combined interpretation is that TAME's decoder error is a narrow
gain-history / LSP-to-LP / synthesis-envelope interaction. Simple global
attenuation, fixed LSP field swapping, and unconditional gain caps are
disqualified by SPEECH. The next clean-room escalation should request numeric
oracle rows for TAME frames around `98`, `118`, `119`, and `123` at minimum:
LSP indices, reconstructed LSF/LSP, LP `a[]`, gain predictor FIFO, decoded
`gp/gc`, excitation `u[]`, and synth `s[]`.

## Decoder ITU Stage Handoff

The clean-room numeric handoff for that escalation is now explicit:

```sh
G729_DUMP_DECODER_ITU_STAGE_HANDOFF=1 \
go test ./internal/decoder -run TestDecoderITUStageHandoffTemplate -count=1 -v
```

Generated files:

- `testdata/oracle/handoff/decoder_itu_stage_expected_template.csv`
- `testdata/oracle/handoff/decoder_itu_stage_got.csv`

The template contains blank `expected` values and must not be treated as an
oracle. It covers the current high-value localization frames:

- `ALGTHM`: `0`, `14`, `15`
- `TAME`: `0`, `98`, `117`, `118`, `119`, `123`
- `OVERFLOW`: `0`, `106`, `107`, `108`

The expected artifact may contain only numeric scalar values. It must not
contain ITU reference C, bcg729, FFmpeg, Sipro, or other implementation source,
implementation-derived branch descriptions, or magic-number provenance. A
completed verifier artifact should be saved as:

```text
testdata/oracle/handoff/decoder_itu_stage_expected.csv
```

## Frame-0 HP-Input Inverse Update

The verifier-filled frame-0 inverse artifact is:

```text
testdata/oracle/handoff/decoder_itu_frame0_hp_input_inverse_expected_template.csv
```

Validation status:

- Header: `source,frame,sub,field,index,expected`
- Rows: `480`
- Filled expected cells: `480/480`
- Post-fill SHA-256:
  `5dbfcd17059df81a630a4c0391094937c99bb1373a04af12e5b05362f31578cf`

Local compare:

```sh
G729_COMPARE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
G729_REQUIRE_COMPLETE_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_HP_INPUT_INVERSE=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderITUFrame0HPInputInverse -count=1 -v
```

Result:

| Source | In range | Total |
| --- | ---: | ---: |
| `ALGTHM` | 71 | 80 |
| `TAME` | 49 | 80 |
| `OVERFLOW` | 50 | 80 |
| `TOTAL` | 170 | 240 |

Interpretation:

- The artifact is complete and numeric.
- Local `postfilter_s_q0` is outside the PST-derived HP-input range for
  `70/240` samples.
- This makes a final scale-only or HP-output-only explanation unlikely. The
  remaining frame-0 mismatch is already present before the output HP filter
  input, so follow-up should target postfilter input/output and upstream
  synthesis/excitation state.

Partial verifier artifact status:

- `decoder_itu_stage_expected.csv` currently has `1562` filled numeric cells
  and `8929` blank cells.
- Bitstream parameter fields, selected pitch delays, and selected adaptive
  gains match exactly on filled rows.
- Filled `fixed_c_q13` rows match `297/303`; the remaining six disagreements
  all move the fourth fixed-codebook pulse for `C=4099`, `C=3587`, or `C=4183`.
- The separate three-row fixed-codebook clarification returned `30/30` exact
  against the local §3.8.2-style decomposition:
  `4099 -> m3=23`, `3587 -> m3=19`, and `4183 -> m3=23`.
- Therefore those six `fixed_c_q13` disagreements should be treated as a
  partial-stage-artifact issue, not as a production decoder fixed-codebook
  defect.
- The clarification artifact is:
  `testdata/oracle/handoff/decoder_itu_fcb_position_clarification_expected.csv`.
  Compare it with:

  ```sh
  G729_COMPARE_DECODER_ITU_FCB_POSITION_CLARIFICATION=1 \
  G729_REQUIRE_COMPLETE_DECODER_ITU_FCB_POSITION_CLARIFICATION=1 \
  G729_REQUIRE_EXACT_DECODER_ITU_FCB_POSITION_CLARIFICATION=1 \
  go test ./internal/fcb -run TestOracleHandoff_CompareDecoderITUFCBPositionClarification -count=1 -v
  ```

- Filled final `pcm_q0` rows are still far from exact, so the partial artifact
  is useful for localization but is not a completed oracle gate.

Frame-0 chain follow-up:

- `decoder_itu_stage_frame0_chain_expected.csv` is a verifier-filled numeric
  artifact for ALGTHM, TAME, and OVERFLOW frame `0`.
- The artifact confirmed the frame-0 subframe-0 `fixed_c_q13` rows require the
  stream-start pitch-sharpening beta to use the upper value before any previous
  decoded pitch gain exists.
- After that production fix, local frame-0 subframe-0 `fixed_c_q13` exact
  matches the verifier artifact: `120/120`.
- The final PCM gate remains non-exact: the same artifact's inverse HP
  candidate check has local `hp_q0` within the PST-derived final-output range
  for `158/240` rows. Remaining work is therefore downstream of fixed-codebook
  pulse reconstruction: gain, excitation/synthesis, postfilter, HP, or PST
  rounding/domain reconciliation.
- The artifact is now covered by an opt-in regression/localization harness:

  ```sh
  G729_COMPARE_DECODER_ITU_FRAME0_CHAIN=1 \
  G729_REQUIRE_EXACT_DECODER_ITU_FRAME0_CHAIN_FIXED_C=1 \
  go test ./internal/decoder -run TestOracleHandoff_CompareDecoderITUFrame0Chain -count=1 -v
  ```

  Current result: `fixed_c_q13` exact `120/120`, final `pst_pcm_q0` exact
  `116/240`, and local `hp_q0` within the PST-derived inverse range
  `158/240`. Per-vector HP range counts are ALGTHM `68/80`, TAME `45/80`,
  and OVERFLOW `45/80`.

TAME wide-stage artifact status:

- `decoder_tame_stage_wide_expected.csv` is a verifier-filled numeric artifact
  for TAME frames `117`, `118`, and `119`, subframes `0` and `1`.
- It covers LP coefficients, adaptive/fixed gains, adaptive vector,
  fixed codebook, pitch/fixed contribution, excitation, and synthesis cells.
- Current comparison reports `444/1518` exact cells with no missing local rows.
- `adaptive_gain_q14` is exact `6/6`, so the decoded adaptive gain and taming
  output at these subframes are not the first failing stage.
- `adaptive_v_q0`, `pitch_contrib_q0`, `excitation_u_q0`, and `synth_s_q0`
  are `0/240` exact. The first substantial divergence is therefore the
  adaptive-codebook vector / past-excitation history entering TAME frame 117.
- `fixed_c_q13` and `fixed_contrib_q0` are mostly exact (`214/240`) and the
  fixed contribution max absolute delta is only `2`, so these rows are
  localization evidence but not the dominant current decoder error.
- The follow-up verifier relation artifact
  `tame_short_pitch_relation.csv` reported the spec-required short-pitch
  `T_frac=0` relation as phase-0 FIR/interpolation, not direct periodic
  repetition.
- After applying that production fix, the TAME-wide comparison still has
  `444/1518` exact cells, but the dominant adaptive-codebook error shrank:
  `adaptive_v_q0` max absolute delta `2994 -> 441`,
  `pitch_contrib_q0` / `excitation_u_q0` max absolute delta `3102 -> 447`.
  Remaining synthesis mismatch is now more tied to LP/gain/history deltas than
  to the original short-pitch direct-repeat bug.
- The first follow-up handoff,
  `decoder_tame_pre_acb_history_expected_template.csv`, asks for 153 numeric
  rows for the TAME frame 117 subframe 0 past-excitation FIFO immediately
  before adaptive-codebook reconstruction. The verifier reported that this
  cannot be independently derived from only `decoder_tame_stage_wide_expected`,
  `tame_short_pitch_relation`, and the pre-ACB template: downstream
  `adaptive_v_q0` rows do not uniquely determine the source FIFO.
- The replacement forward-trace handoff is
  `decoder_tame_excitation_history_expected_template.csv`: 9360 numeric rows
  for TAME frame `0..116` decoded `excitation_u_q0`. The verifier reported
  that this also cannot be independently derived from the provided inputs
  because full forward decode requires support tables that were not yet part
  of the handoff.
- The current replacement gate is
  `decoder_support_tables_expected_template.csv`: 264 numeric rows for the
  small LSP cosine, pitch interpolation, gain VQ/map, log/pow, and gain
  predictor tables needed before asking for broad decoder forward traces. The
  verifier reported that this cannot be fully completed under the current
  clean-room boundary because some gain VQ/map values are available only as
  simulation-software numeric tables, not Recommendation text/math.

Comparison command:

```sh
G729_COMPARE_DECODER_ITU_STAGE_HANDOFF=1 \
G729_REQUIRE_COMPLETE_DECODER_ITU_STAGE_HANDOFF=1 \
G729_REQUIRE_EXACT_DECODER_ITU_STAGE_HANDOFF=1 \
G729_REJECT_DECODER_ITU_STAGE_SELF_ORACLE=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderITUStageHandoff -count=1 -v
```

TAME wide comparison command:

```sh
G729_COMPARE_DECODER_TAME_STAGE_WIDE=1 \
G729_REQUIRE_EXACT_DECODER_TAME_STAGE_WIDE=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEStageWide -count=1 -v
```

TAME pre-ACB history comparison command:

```sh
G729_COMPARE_DECODER_TAME_PRE_ACB_HISTORY=1 \
G729_REQUIRE_COMPLETE_DECODER_TAME_PRE_ACB_HISTORY=1 \
G729_REQUIRE_EXACT_DECODER_TAME_PRE_ACB_HISTORY=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEPreACBHistory -count=1 -v
```

TAME excitation-history comparison command:

```sh
G729_COMPARE_DECODER_TAME_EXCITATION_HISTORY=1 \
G729_REQUIRE_COMPLETE_DECODER_TAME_EXCITATION_HISTORY=1 \
G729_REQUIRE_EXACT_DECODER_TAME_EXCITATION_HISTORY=1 \
go test ./internal/decoder -run TestOracleHandoff_CompareDecoderTAMEExcitationHistory -count=1 -v
```

Decoder support-table comparison command:

```sh
G729_COMPARE_DECODER_SUPPORT_TABLES=1 \
G729_REQUIRE_COMPLETE_DECODER_SUPPORT_TABLES=1 \
G729_REQUIRE_EXACT_DECODER_SUPPORT_TABLES=1 \
go test -run TestOracleHandoff_CompareDecoderSupportTables -count=1 -v
```

Before the independent expected values exist, the template-only comparison is
allowed to pass without the `REQUIRE_*` flags and reports all rows as blank
expected values. That is a contract check, not decoder evidence.

## First Trace Result

ALGTHM `first-diff` mode currently points to frame 0 sample 2:

```text
first diff sample=2 got=4 want=3 delta=+1
frame exact=61.25% maxAbs=4 meanAbs=1.55 rms=1.83
hp_x2 exact=61.25% near1=90.00%
hp_raw vs PST>>1 exact=62.50% near1=92.50%
```

Interpretation: ALGTHM frame 0 is not the highest-value first fix target.
The first surface is mostly final-domain rounding / scaling around the HP
output, not a large upstream synthesis failure.

ALGTHM `worst-frame` mode currently points to frame 15:

```text
first diff sample=0 got=10242 want=18565 delta=-8323
frame exact=0.00% maxAbs=13820 meanAbs=5300.49 rms=6682.73
```

ALGTHM `max-sample` mode currently points to frame 14:

```text
first diff sample=0 got=3730 want=7886 delta=-4156
frame exact=0.00% maxAbs=13858 meanAbs=5876.21 rms=6633.74
```

Interpretation: after the validation gate exists, the next engineering work
should target the material mid-vector state drift rather than spending the
first fix cycle on the frame-0 ±1-style boundary differences.

## Next Engineering Goal

Promote this matrix from opt-in diagnostic to release gate only after
`annexa-good` reaches one of these states:

- preferred: `100.00%` exact frames and `100.00%` exact samples;
- fallback only with strong evidence: all remaining deltas are bounded by a
  documented fixed/floating-point tolerance and supported by independent numeric
  oracle artifacts.

Recommended order:

1. Start with the smallest ordinary-good vector whose first diff is early and
   low-complexity.
2. Diagnose the first divergent frame at stage level against the `.PST`
   output, using numeric traces only.
3. Fix one spec-aligned decoder issue at a time.
4. Re-run the full `annexa-good` matrix after every fix.
5. Leave `ERASURE` and `PARITY` for a separate robustness goal after ordinary
   good-frame decode reaches the gate.
