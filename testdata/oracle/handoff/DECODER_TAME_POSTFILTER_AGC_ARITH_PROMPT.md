# Decoder TAME Postfilter AGC Arithmetic Oracle Request

Run this in `/home/exedev/g729_untracked`, not in `/home/exedev/g729`.

Clean-room boundary:

- You may execute the licensed/reference decoder only in this untracked verifier workspace.
- Do not copy reference source, implementation names, branch descriptions, formulas, table provenance, or code snippets into `/home/exedev/g729`.
- The only deliverable that may be consumed by `/home/exedev/g729` is numeric oracle data: scalar/vector values, frame/sub/index keys, controlled notes, aggregate validation counts, and SHA256.

Context:

`decoder_tame_postfilter_micro_expected.csv` identified the remaining early postfilter mismatch around AGC arithmetic. Local output now matches TAME frame 0 subframe 1 except a few +/-1 samples. The unresolved question is the exact AGC target scaling and per-sample recurrence arithmetic.

Output file:

`/home/exedev/g729_untracked/verifier-output/decoder_tame_postfilter_agc_arith_expected.csv`

CSV schema:

```csv
source,frame,sub,field,index,expected,note
```

Coverage:

- Required: TAME frame `0`, subframes `0` and `1`.
- Prefer also frames `1..7`, subframes `0` and `1`.
- `source` must be `TAME`.
- `index` is vector index, or `0` for scalar fields.
- `expected` must be a signed decimal integer.
- `note` must be `reference_decoder_execution`.
- No blank expected cells.

Required scalar fields per covered subframe:

- `agc_energy_input_raw_q0`
- `agc_energy_postfilter_raw_q0`
- `agc_abs_input_raw_q0`
- `agc_abs_postfilter_raw_q0`
- `agc_target_internal_q14`
- `agc_target_internal_q24`
- `agc_gain_before_q24`
- `agc_gain_after_q24`

Required vector fields per covered subframe, index `0..39`:

- `agc_input_s_q0`
- `agc_postfilter_pre_agc_q0`
- `agc_gain_before_update_q24`
- `agc_update_mul_prev_q0`
- `agc_update_mul_target_q0`
- `agc_update_acc_q0`
- `agc_gain_after_update_q24`
- `agc_output_product_q24`
- `agc_output_q0`

Validation to report:

- Output directory path.
- Row count.
- Filled expected count.
- Blank expected count, must be `0`.
- Non-integer expected count, must be `0`.
- Note values used, expected only `reference_decoder_execution`.
- Frame/sub coverage.
- TAME final PCM vs official `TAME.PST`, expected exact match.
- SHA256 of `decoder_tame_postfilter_agc_arith_expected.csv`.
- Blockers, if any.
