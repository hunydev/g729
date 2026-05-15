# Decoder TAME Postfilter Micro Oracle Request

Run this in `/home/exedev/g729_untracked`, not in `/home/exedev/g729`.

Clean-room boundary:

- You may execute the licensed/reference decoder only in this untracked verifier workspace.
- Do not copy reference source, implementation names, branch descriptions, formulas, table provenance, or code snippets into `/home/exedev/g729`.
- The only deliverable that may be consumed by `/home/exedev/g729` is numeric oracle data: scalar/vector values, frame/sub/index keys, controlled notes, aggregate validation counts, and SHA256.

Goal:

Generate a numeric postfilter micro-stage oracle for the TAME decoder vector so `/home/exedev/g729` can compare its local postfilter internals against reference execution. The current first full-stage mismatch is `TAME frame=0 sub=1 field=postfilter_s_q0 index=1`, so include at least frame 0 subframes 0 and 1. Prefer all TAME frames `0..127` if practical.

Output file:

`/home/exedev/g729_untracked/verifier-output/decoder_tame_postfilter_micro_expected.csv`

CSV schema:

```csv
source,frame,sub,field,index,expected,note
```

Rows:

- `source` must be `TAME`.
- `frame` should be `0..127` if practical.
- `sub` must be `0` or `1`.
- `index` is vector index, or `0` for scalar fields.
- `expected` must be a signed decimal integer.
- `note` must be `reference_decoder_execution`.
- No blank expected cells.

Required fields:

- `lp_a_q12[0..10]`
- `postfilter_a_num_q12[0..10]`
- `postfilter_a_den_q12[0..10]`
- `postfilter_past_s_before_q0[0..9]`
- `postfilter_past_s_after_q0[0..9]`
- `postfilter_past_residual_before_q0[0..182]`
- `postfilter_past_residual_after_q0[0..182]`
- `postfilter_past_synth_post_before_q0[0..9]`
- `postfilter_past_synth_post_after_q0[0..9]`
- `postfilter_past_tilt_input_before_q0`
- `postfilter_past_tilt_input_after_q0`
- `postfilter_agc_gain_before_q24`
- `postfilter_agc_gain_after_q24`
- `postfilter_initialized_before_q0` as `0` or `1`
- `postfilter_initialized_after_q0` as `0` or `1`
- `postfilter_residual_q0[0..39]`
- `postfilter_refined_t_q0`
- `postfilter_longterm_g0_q14`
- `postfilter_longterm_g1_q14`
- `postfilter_longterm_q0[0..39]`
- `postfilter_shortterm_q0[0..39]`
- `postfilter_tilt_mu_q15`
- `postfilter_tilt_q0[0..39]`
- `postfilter_agc_target_q14`
- `postfilter_agc_gain_q24[0..39]`
- `postfilter_s_q0[0..39]`

Validation to report:

- Output directory path.
- Row count.
- Filled expected count.
- Blank expected count, must be `0`.
- Non-integer expected count, must be `0`.
- Note values used, expected only `reference_decoder_execution`.
- Frame/sub coverage.
- TAME final PCM vs official `TAME.PST`, expected exact match.
- SHA256 of `decoder_tame_postfilter_micro_expected.csv`.
- Blockers, if any.
