# Decoder Pitch-Instability Decision Verifier Prompt

You are an isolated clean-room verifier. Do not return or describe
implementation code. Fill only numeric `expected` cells in:

```text
testdata/oracle/handoff/decoder_pitch_instability_decision_expected_template.csv
```

Use the scalar keys exactly as provided:

```csv
source,frame,sub,field,index
```

Rules:

- Preserve the header, row count, row order, and key columns.
- Fill every `expected` cell you can independently derive with a signed
  decimal integer.
- Leave `expected` blank if the value cannot be independently derived without
  using local implementation-derived values.
- Do not run `G729_WRITE_DECODER_PITCH_INSTABILITY_DECISION_HANDOFF=1` after
  filling; it regenerates a blank expected template.
- Do not add source names beyond the existing `source` key values, code
  snippets, branch descriptions, provenance notes, comments, or explanatory
  text inside the CSV.
- Treat `index=-1` as "not applicable" for scalar rows.

Scope:

- Sources: `TAME`, `SPEECH`, `PITCH`, and `OVERFLOW` ITU Annex A decoder test
  bitstreams.
- Rows target subframes where the local diagnostic trigger
  `gp>16000 && pastRMS>=220 && fixedRMS<=40` fired. This trigger is not a
  proposed normative rule; it only identifies stress subframes for verification.
- The verification question is whether a Recommendation-backed or
  independently observable decoder pitch-instability/taming decision exists on
  these good-frame subframes.

Fields:

- `bitstream_ga`, `bitstream_gb`: received gain index parts for the subframe.
- `pitch_t_int`, `pitch_t_frac`: decoded pitch lag components.
- `adaptive_gain_before_q14`: decoded adaptive-codebook gain before any
  pitch-instability limiting.
- `pitch_instability_flag_q0`: `1` if a decoder-side pitch-instability/taming
  limiter applies on this good-frame subframe, otherwise `0`.
- `adaptive_gain_after_pitch_instability_q14`: adaptive gain after that
  decision. If no decoder-side limiter exists, this should equal
  `adaptive_gain_before_q14`.
- `fixed_gain_q14`: reconstructed fixed-codebook gain in Q14 scalar form.
- `past_exc_rms_x100`, `past_tail_rms_x100`, `adaptive_v_rms_x100`,
  `pitch_contrib_rms_x100`, `fixed_contrib_rms_x100`,
  `excitation_u_rms_x100`: RMS scalars multiplied by 100 and rounded to the
  nearest integer.
- `pitch_to_fixed_ratio_x100`, `past_to_fixed_ratio_x100`: ratio scalars
  multiplied by 100 and rounded to the nearest integer.

Important boundary:

- If Recommendation text confirms that good-frame decoder gain reconstruction
  is only equations `(73)`, `(74)`, and `(75)` with no decoder-side
  pitch-instability limiter, fill `pitch_instability_flag_q0=0` and
  `adaptive_gain_after_pitch_instability_q14=adaptive_gain_before_q14` where
  the before-gain is independently known.
- If a limiter exists only in encoder gain quantization or in erased-frame
  concealment, do not mark good-frame decoder rows as limited.
- If support tables or prior decoder state are unavailable under your
  clean-room constraints, leave the dependent rows blank rather than guessing.

After filling the template, the implementation-side compare command is:

```sh
env GOCACHE=/tmp/go-build \
  G729_COMPARE_DECODER_PITCH_INSTABILITY_DECISION=1 \
  G729_REQUIRE_COMPLETE_DECODER_PITCH_INSTABILITY_DECISION=1 \
  G729_REQUIRE_EXACT_DECODER_PITCH_INSTABILITY_DECISION=1 \
  go test ./internal/decoder -run TestOracleHandoff_CompareDecoderPitchInstabilityDecision -count=1 -v
```
