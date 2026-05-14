# Decoder ITU Vector Validation

Date: 2026-05-12

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
the decoder has much stronger evidence than a MOS-style quality score.

PESQ/POLQA remain useful as end-to-end listening-quality diagnostics, but they
are not the primary decoder conformance gate:

- ITU's P.862 pages state that P.862/P.862.1/P.862.2/P.862.3 were deleted on
  2024-01-05 and point users to P.863/P.863.1/P.863.2.
- ITU-T P.863.1 documents comparison guidance between older P.862/PESQ results
  and P.863 narrowband mode, including standard-codec average-score context.
- Therefore PESQ NB can remain a legacy VoIP quality diagnostic, but public
  decoder credibility should be based on vector PCM equality first.

References:

- <https://www.itu.int/rec/t-rec-p.862>
- <https://www.itu.int/rec/T-REC-P.863/>
- <https://www.itu.int/rec/dologin_pub.asp?id=T-REC-P.863.1-201305-S%21%21PDF-E&lang=e&type=items>

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
| `ALGTHM` | 35 | 0 | 0.00% | 2.61% | 0:2 | 13862 | 834.89 | 2095.61 |
| `SPEECH` | 3750 | 0 | 0.00% | 13.89% | 0:0 | 5801 | 52.20 | 154.58 |
| `FIXED` | 120 | 0 | 0.00% | 16.91% | 0:1 | 491 | 10.87 | 30.77 |
| `LSP` | 2232 | 0 | 0.00% | 2.54% | 0:20 | 1202 | 34.28 | 65.97 |
| `PITCH` | 1835 | 0 | 0.00% | 1.26% | 0:2 | 9432 | 420.26 | 894.11 |
| `TAME` | 128 | 0 | 0.00% | 0.36% | 0:1 | 14039 | 3780.52 | 5091.07 |
| `TEST` | 176 | 0 | 0.00% | 6.21% | 0:20 | 2223 | 39.80 | 107.31 |
| `OVERFLOW` | 384 | 0 | 0.00% | 0.14% | 0:1 | 65535 | 6742.60 | 10403.43 |
| `TOTAL` | 8660 | 0 | 0.00% | 7.32% | 0:0 | 65535 | 511.89 | 2406.89 |

Interpretation:

- The decoder is not yet ITU-vector bit-exact.
- This is a stronger blocker than any PESQ/POLQA score for decoder credibility.
- Existing FFmpeg black-box and listening-quality gates remain useful
  interoperability/quality checks, but they do not replace vector equality.

Update after the synthesis overflow recovery fix:

- `internal/synth` now follows the §3.10 divide-by-4 / multiply-by-4 recovery
  path instead of the previous divide-by-2 / multiply-by-2 fallback.
- SPEECH and Asterisk/FFmpeg black-box localization metrics were unchanged in
  the ordinary speech path.
- The stress-vector surface improved: `OVERFLOW` max abs delta dropped from
  `65535` to `36943`, and total RMS delta dropped from `4770.37` to `3651.96`.
- The decoder is still not ITU-vector exact; this is a blocker reduction, not
  conformance completion.

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
| `ALGTHM` | `0:2 d=1` | `12:1 d=2248` | `12:40 d=4622` | `15:6684.31` | `14:13862` |
| `SPEECH` | `0:0 d=2` | `43:23 d=1164` | `2732:48 d=5801` | `1841:1364.48` | `1841:3886` |
| `FIXED` | `0:1 d=2` | `-` | `-` | `118:113.05` | `97:491` |
| `LSP` | `0:20 d=2` | `564:41 d=1121` | `-` | `1705:596.88` | `564:1202` |
| `PITCH` | `0:2 d=1` | `2:41 d=1785` | `21:50 d=4481` | `282:2707.98` | `282:8594` |
| `TAME` | `0:1 d=2` | `1:42 d=1068` | `2:34 d=4273` | `123:10069.57` | `126:14039` |
| `TEST` | `0:20 d=2` | `78:60 d=1230` | `-` | `79:611.53` | `79:2223` |
| `OVERFLOW` | `0:1 d=2` | `2:10 d=1105` | `19:6 d=6716` | `236:38434.16` | `237:62595` |

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
| `123` | `10069.57` | `723.78` | `13949` | `1487` |
| `127` | `9883.27` | `520.09` | `14016` | `962` |
| `126` | `9824.08` | `1086.53` | `14039` | `1784` |
| `122` | `9587.16` | `683.40` | `13686` | `1301` |
| `125` | `9565.75` | `1178.91` | `13619` | `1849` |

The per-subframe logs show `fixedRMS` is much smaller than `pitchRMS` at
those already-bad frames. The fixed-gain-half improvement is therefore mostly
stateful: reducing earlier fixed contribution changes the subsequent
past-excitation/adaptive vector trajectory, rather than merely subtracting a
large direct fixed contribution in the listed frame.

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

For full-file candidates on `TAME`, `gain_ec_q25` improves aggregate RMS from
`5081.86` to `2534.89`, while `gain_gamma_q14` improves it to `3938.36`. Both
reduce the severe late frames `122`, `123`, `125`, `126`, and `127`, but both
also regress early frames such as `3`, `5`, `6`, and `7`.

The cutover probe is:

```sh
G729_DECODER_GAIN_CANDIDATE_CUTOVER=1 \
G729_DECODER_GAIN_CANDIDATE_VECTOR=TAME \
G729_DECODER_GAIN_CANDIDATE=gain_gamma_q14 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=8 \
go test ./internal/decoder -run TestDecoderITUGainCandidateCutover -count=1 -v
```

Current cutover result:

| Candidate | Best cutover frame | Aggregate RMS | Production RMS |
| --- | ---: | ---: | ---: |
| `gain_ec_q25` | `10` | `2160.31` | `5081.86` |
| `gain_gamma_q14` | `26` | `1189.85` | `5081.86` |

Interpretation:

- These are not safe production changes because SPEECH/PITCH reject the same
  gain variants.
- The TAME error is strongly state-history dependent: a late cutover beats both
  production and full-file candidate application.
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
G729_DECODER_GAIN_CANDIDATE=gain_gamma_q14 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=5 \
go test ./internal/decoder -run TestDecoderITUGainCandidateWindow -count=1 -v
```

The best finite `gain_gamma_q14` window is `[26,120)`, with aggregate RMS
`1187.84`. It slightly beats the `[26,128)` cutover (`1189.85`). The window
still regresses early affected frames such as `27..34`, but it dramatically
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
| `ALGTHM` | `2068.11` | `3785.84` | output |
| `FIXED` | `28.05` | `81.35` | output |
| `LSP` | `65.13` | `337.96` | output |
| `OVERFLOW` | `10396.15` | `9629.71` | `hp_raw` |
| `PITCH` | `888.43` | `2652.20` | output |
| `SPEECH` | `143.44` | `1074.60` | output |
| `TAME` | `5081.86` | `3943.11` | `hp_raw` |
| `TEST` | `103.93` | `732.28` | output |

Pre-scale ratio audit:

```sh
G729_DECODER_ITU_PRESCALE_RATIO_AUDIT=1 \
go test ./internal/decoder -run TestDecoderITUPreScaleRatioAudit -count=1 -v
```

Current active-frame result (`.PST` frame RMS >= `500`):

| Vector | Active frames | HP raw median / PST | Output median / PST | HP near 0.5x | HP near 1.0x | Output > 1.5x |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `ALGTHM` | `29` | `0.439` | `0.878` | `17` | `0` | `0` |
| `LSP` | `555` | `0.502` | `1.004` | `544` | `0` | `0` |
| `OVERFLOW` | `383` | `0.523` | `1.047` | `171` | `23` | `64` |
| `PITCH` | `1522` | `0.417` | `0.835` | `904` | `0` | `0` |
| `SPEECH` | `1819` | `0.502` | `1.004` | `1792` | `0` | `0` |
| `TAME` | `127` | `0.673` | `1.345` | `39` | `32` | `55` |
| `TEST` | `78` | `0.502` | `1.003` | `78` | `0` | `0` |

Interpretation:

- A global final-output scale change is disqualified: ordinary vectors
  (`SPEECH`, `LSP`, `TEST`, and most active `ALGTHM`/`PITCH` frames) show the
  expected pattern where pre-final-scale `hp_raw` is near half of `.PST` and
  final output is near `.PST`.
- TAME and OVERFLOW are stress-vector exceptions where local `hp_raw` is already
  too large before the final `ScaleUpSat` step. TAME has `32/127` active frames
  with `hp_raw` near `.PST` amplitude and `55/127` active frames where final
  output exceeds `1.5x` `.PST`.
- The TAME worst-frame trace logs `hp_raw` directly against the PST final
  domain. On frame `123`, `hp_raw` RMS delta is only `712.97` while `output`
  RMS delta is `10069.57`. Because ordinary vectors confirm the final x2 path,
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
- First `hp_raw / PST >= 0.8`: frame `88`
- First `output / PST >= 1.5`: frame `72`
- Worst output-ratio frame: frame `123`
  (`s/PST=0.987`, `spf/PST=0.989`, `hp/PST=0.961`,
  `output/PST=1.921`)
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
| `26` | `120` | `94` | `1187.84` | best |
| `26` | `123` | `97` | `1187.95` | near-best |
| `26` | `128` | `102` | `1189.85` | near-best |

The best window's largest improvements are frames `120..127`, even though the
candidate is disabled at frame `120` in the best `[26,120)` run. This is strong
evidence that the damaging state is accumulated during frames `26..119` and
then expressed through the adaptive-codebook/past-excitation history in the
late TAME frames. Early frames `26..34` regress under the same diagnostic, so
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
| `52` | `239` | `187` | `26` | `120` | `1185.18` |
| `52` | `249` | `197` | `26` | `125` | `1186.90` |
| `52` | `240` | `188` | `26` | `120` | `1187.84` |

The best boundary is frame `26` subframe `0` through frame `119` subframe `0`
inclusive (`[52,239)` in global subframe numbering). Excluding frame `119`
subframe `1` slightly improves the frame-window best. This keeps the target on
subframe-wise gain/excitation history rather than a frame-output artifact.

History timeline:

```sh
G729_DECODER_TAME_HISTORY_TIMELINE=1 \
G729_DECODER_UPSTREAM_WINDOW_CANDIDATE=fixed_gain_half \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=8 \
go test ./internal/decoder -run TestDecoderTAMEHistoryTimeline -count=1 -v
```

Current summary:

- Window: `[52,239)` global subframes (`26/0` through `119/0`).
- Output RMS: production `5081.86`, candidate `1185.18`.
- At window start, direct fixed contribution is exactly halved, but total
  excitation barely changes: frame `26/0` has `fixed c/p=0.500`, `u c/p=0.990`,
  and `s c/p=0.997`.
- By late TAME frames, the accumulated FIFO/ACB effect dominates: frame `118/1`
  has `past c/p=0.762`, `v c/p=0.753`, `u c/p=0.745`, and `s c/p=0.606`.
- After the window is disabled, direct fixed contribution returns to `1.000x`,
  but the reduced past-excitation/adaptive vector persists into frames
  `120..127`, keeping `v/u` around `0.77..0.82x` and `s` around `0.60..0.62x`.

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
| production | `203.36` | `383.97` | `231.30` | `101.59` | `0.8663` | `0.4588` |
| `[52,239)` fixed-gain-half | `203.36` | `291.74` | `184.49` | `127.52` | `0.7790` | `0.5430` |

Interpretation:

- Production's ACB vector shape is closer to the oracle than the damping
  candidate (`corr 0.8663` vs `0.7790`), but its amplitude is almost `1.9x`
  high (`got RMS 383.97` vs `ref RMS 203.36`).
- The damping candidate improves raw ACB error by reducing amplitude, but it
  worsens shape after scale normalization (`scaled err 127.52` vs `101.59`).
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

- First sample with frame max abs delta `>=4096`: frame `2`.
- First active frame with local output RMS / PST RMS `>=1.25`: frame `53`.
- First active frame with local output RMS / PST RMS `>=1.50`: frame `72`.
- First 4-frame persistent local output RMS / PST RMS `>=1.50`: frame `76`.
- At the fixed-gain diagnostic window start (`26/0`), output ratio is still
  near unity (`frame 26 out/PST=1.003`) and direct fixed contribution is small
  relative to pitch/excitation.
- By frame `53`, the ratio has grown to `1.279`, with `pastRMS≈267`,
  `vRMS≈263`, and `uRMS≈255`.
- By frame `72`, the ratio has grown to `1.514`, with `pastRMS≈331`,
  `vRMS≈331`, and `uRMS≈331`.
- In the verifier checkpoint zone (`117..127`), the ratio is already
  `1.787..1.921`, and local pre-ACB history / ACB / excitation RMS are all in
  the `~385..430` range.

Interpretation:

- Frame `117` is not the onset. It is where the verifier has enough numeric
  checkpoint rows to observe the already-accumulated failure.
- The local amplitude drift starts as a slow history accumulation after the
  frame-`26` diagnostic window boundary, becomes material around frame `53`,
  and becomes severe/persistent around frames `72..76`.
- This further argues against fixing late TAME by changing ACB interpolation,
  current-subframe gain summing, postfilter, HP, or final scaling. The next
  useful production audit should target the state transition that begins the
  `pastExc` growth between frames `26` and `53`.

TAME gain-energy audit:

```sh
G729_DECODER_TAME_GAIN_ENERGY_AUDIT=1 \
G729_DECODER_ITU_VECTOR_FRONTIER_TOP=6 \
go test ./internal/decoder -run TestDecoderTAMEGainEnergyAudit -count=1 -v
```

Current result:

- Fixed-codebook energy saturation: `0/256` subframes.
- In the onset window (`26..53`), non-saturating int64 energy and the local
  `gain.FixedCodebookEnergy` `Word32` result are identical.
- Direct fixed contribution is much smaller than adaptive/pitch contribution
  through the onset region. Example: frame `53/0` has `pitchRMS=258.2`,
  `fixedRMS=44.7`, and `uRMS=254.7`.
- The legacy diagnostic `GcQ12Final` is frequently saturated, but the active
  excitation path uses the mantissa/exponent pair (`GcMantQ14`, `GcExp`), not
  the clamped Q12 diagnostic value.

Interpretation:

- The TAME drift is not caused by fixed-codebook energy accumulator saturation.
- The onset still looks ACB/past-excitation feedback dominated: small direct
  fixed contribution changes can perturb the recurrent `pastExc` path, but the
  immediate sample energy is mostly adaptive contribution.
- The next verifier-independent local check should therefore focus on
  pitch/ACB feedback and the `pastExc` FIFO update over frames `26..53`, not on
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
| `cold-start` | `8` | `7` | `301.05` | `37.63` | `0.0` | `301.1` |
| `pre-onset` (`26..53`) | `54` | `26` | `45.60` | `0.84` | `221.6` | `267.2` |
| `first-1.25x` (`53..72`) | `38` | `23` | `60.60` | `1.59` | `267.2` | `327.7` |
| `severe-rise` (`72..117`) | `90` | `51` | `72.07` | `0.80` | `327.7` | `399.8` |
| `checkpoint` (`117..127`) | `22` | `12` | `17.69` | `0.80` | `399.8` | `417.5` |

Interpretation:

- The FIFO copy/update operation is not behaving like a discrete bug. Each
  growth step is explained by incoming `U` RMS being larger than the 40 samples
  shifted out.
- The short-pitch feedback path is required; disabling it globally makes TAME
  much worse. This aligns with the separate oracle replay result where local
  ACB interpolation is exact when supplied with oracle `pastExc`.
- The local failure is therefore a long recurrent energy-balance problem:
  `pastExc` is already high after cold start, then continues to drift upward
  through `26..117`. A production fix still needs a spec-backed reason why the
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
