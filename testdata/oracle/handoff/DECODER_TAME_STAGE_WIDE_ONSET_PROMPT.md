# Decoder TAME Stage-Wide Onset Verifier Request

Clean-room boundary: do not inspect ITU reference C, bcg729, FFmpeg, Sipro Lab, or any other G.729 implementation source. Use only the ITU test-vector numeric artifacts, the G.729 recommendation text, and independently derived arithmetic.

## Input

- Template: `testdata/oracle/handoff/decoder_tame_stage_wide_onset_expected_template.csv`
- Local reference shape only: `testdata/oracle/handoff/decoder_tame_stage_wide_onset_got.csv`
- ITU vector inputs: `testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.BIT` and `TAME.PST`

## Task

Fill `expected` cells in the template with independently derived signed decimal integer values for the listed TAME frames and subframes. Preserve the exact header, row order, and column order.

The requested windows are chosen to localize the known TAME excitation-history growth:

- frames `0..5`: cold-start and first large `pastExc` jump
- frames `22..33`: stable pre-onset into early drift
- frames `49..60`: first active `output/PST >= 1.25`
- frames `68..79`: first active `output/PST >= 1.50`
- frames `112..127`: late checkpoint and existing oracle neighborhood

Requested fields:

- `past_exc_pre_acb_q0[0..152]`
- `lp_a_q12[0..10]`
- `adaptive_gain_q14`
- `fixed_gain_q14`
- `adaptive_v_q0[0..39]`
- `fixed_c_q13[0..39]`
- `pitch_contrib_q0[0..39]`
- `fixed_contrib_q0[0..39]`
- `excitation_u_q0[0..39]`
- `synth_s_q0[0..39]`

If a value cannot be independently derived under the clean-room boundary, leave that cell blank. Do not copy local `got` values into `expected`.

## Output

Write the completed CSV to:

`/home/exedev/g729-decoder-itu-stage-verifier-handoff/verifier-output/decoder_tame_stage_wide_onset_expected_template.csv`

Report:

- row count and column count
- number of filled numeric cells
- number of blank cells
- schema/order validation result against the template
- any fields or frame ranges left blank and why
