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

Output-domain audit:

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

Interpretation:

- A global final-output scale change is disqualified: ordinary vectors
  (`SPEECH`, `PITCH`, `FIXED`, `LSP`, `TEST`, `ALGTHM`) prefer the current
  final output domain.
- TAME/OVERFLOW are stress-vector exceptions where `hp_raw` is closer to the
  `.PST` file than the final `ScaleUpSat` output.
- The TAME worst-frame trace now logs `hp_raw` directly against the PST final
  domain. On frame `123`, `hp_raw` RMS delta is only `712.97` while `output`
  RMS delta is `10069.57`, so much of that specific late-frame error is output
  domain/amplitude shaped.
- This is still not a production fix. It narrows the next question to why the
  stress-vector `.PST` agreement flips domain while ordinary vectors do not.

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
  output is close to PST while final `ScaleUpSat` doubles it away from PST.

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
