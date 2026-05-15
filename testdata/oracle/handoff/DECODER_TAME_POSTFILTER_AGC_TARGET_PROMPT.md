# Decoder TAME Postfilter AGC Target Oracle Request

Run this in `/home/exedev/g729_untracked`, not in `/home/exedev/g729`.

Clean-room boundary:

- You may execute the licensed/reference decoder only in this untracked verifier workspace.
- Do not copy reference source, implementation names, branch descriptions, formulas, table provenance, or code snippets into `/home/exedev/g729`.
- The only deliverable that may be consumed by `/home/exedev/g729` is numeric oracle data: scalar/vector values, frame/sub/index keys, controlled notes, aggregate validation counts, and SHA256.

Context:

`decoder_tame_postfilter_agc_arith_expected.csv` confirmed the AGC recurrence shape:

- `agc_gain_after_update_q24 = (agc_update_acc_q0 << 12)`
- `agc_update_acc_q0 = agc_update_mul_prev_q0 + agc_update_mul_target_q0`
- `agc_update_mul_target_q0 = agc_target_internal_q14 >> 2`

The remaining early mismatch is target generation itself. For TAME frame 0 subframe 1:

- expected `agc_target_internal_q14 = 1352`
- local currently computes `1372`

The printed Q0 vectors alone are not enough to reproduce the reference target exactly, so please emit numeric micro-stages for the AGC target calculation.

Output file:

`/home/exedev/g729_untracked/verifier-output/decoder_tame_postfilter_agc_target_expected.csv`

CSV schema:

```csv
source,frame,sub,field,index,expected,note
```

Coverage:

- Required: TAME frames `0..7`, subframes `0` and `1`.
- Prefer TAME frames `0..127` if practical.
- `source` must be `TAME`.
- `index` is vector index, or `0` for scalar fields.
- `expected` must be a signed decimal integer.
- `note` must be `reference_decoder_execution`.
- No blank expected cells.

Required vector fields per covered subframe, index `0..39`:

- `agc_target_input_s_q0`
- `agc_target_pre_agc_q0`
- `agc_target_input_square_contrib_q0`
- `agc_target_pre_agc_square_contrib_q0`
- `agc_target_input_acc_after_q0`
- `agc_target_pre_agc_acc_after_q0`

Required scalar fields per covered subframe:

- `agc_target_input_energy_raw_q0`
- `agc_target_pre_agc_energy_raw_q0`
- `agc_target_input_abs_sum_q0`
- `agc_target_pre_agc_abs_sum_q0`
- `agc_target_ratio_input_q0`
- `agc_target_ratio_den_q0`
- `agc_target_ratio_q28`
- `agc_target_sqrt_input_q28`
- `agc_target_sqrt_output_q14`
- `agc_target_alpha_complement_q15`
- `agc_target_mul_sqrt_alpha_q0`
- `agc_target_internal_q14`
- `agc_target_internal_q12`

If the reference execution uses normalization, shifts, exponents, lookup-table interpolation, or inverse-square-root steps internally, also emit those numeric values with neutral field names prefixed by `agc_target_`, for example:

- `agc_target_norm_input_shift_q0`
- `agc_target_norm_pre_agc_shift_q0`
- `agc_target_sqrt_table_index_q0`
- `agc_target_sqrt_table_frac_q0`
- `agc_target_sqrt_table_base_q0`
- `agc_target_sqrt_table_slope_q0`
- `agc_target_sqrt_acc_q0`

Validation to report:

- Output directory path.
- Row count.
- Filled expected count.
- Blank expected count, must be `0`.
- Non-integer expected count, must be `0`.
- Note values used, expected only `reference_decoder_execution`.
- Frame/sub coverage.
- TAME final PCM vs official `TAME.PST`, expected exact match.
- SHA256 of `decoder_tame_postfilter_agc_target_expected.csv`.
- Blockers, if any.
