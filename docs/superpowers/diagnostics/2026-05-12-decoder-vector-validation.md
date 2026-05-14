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
  `gSNR=-3.11`, `seg=0.71`, `corr=0.922`.
- Local-vs-FFmpeg active-frame envelope ratio median is `2.609`, with
  `100/127` active frames above `1.5x`.
- Stage ratios show the over-amplification is upstream of postfilter/HP:
  `pitch/u=1.00`, `fixed/u=0.05`, `s/u=20.00`, `spf/s=1.00`,
  `hp/spf=0.97`, `out/hp=1.41`.
- Worst-frame detail at frame `118` has `61/80` clipped output samples.
  Both subframes are mostly adaptive-loop driven:
  `gp≈0.984/0.987`, direct fixed contribution remains small, while synthesis
  output is already near saturation.

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
  for TAME frame `0..116` decoded `excitation_u_q0`. If filled, it directly
  supplies the frame 117 pre-ACB FIFO and reveals the earliest excitation
  divergence before frame 117.

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
