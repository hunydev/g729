# Decoder TAME Pitch-Gain Probe

Date: 2026-05-14

## Context

The decoder TAME vector still has a large late `pastExc` amplitude gap against
the clean-room numeric oracle. The latest useful filled oracle is
`decoder_tame_stage_wide_onset_expected_template.csv`, where filled
`past_exc_pre_acb_q0` rows show production history is roughly too large before
the late `117..119` checkpoint.

## Verifier Result

The requested clean-room verifier output:

```text
/home/exedev/g729-decoder-itu-stage-verifier-handoff/verifier-output/decoder_tame_gain_taming_108_137_expected.csv
```

was not a complete adaptive-gain/taming oracle:

- Rows: `210`
- Filled: `60`
- Blank: `150`
- Filled fields: `bitstream_ga` and `bitstream_gb` only
- Blank fields: `adaptive_gain_before_taming_q14`, `taming_flag_q0`,
  `adaptive_gain_after_taming_q14`, `taming_error_before_q0`,
  `taming_error_after_q0`

Reason: under the clean-room boundary, the verifier did not have independent
numeric gain VQ tables or a prior accumulated taming/error state. This artifact
is therefore not worth ingesting as a decoder oracle; it only reconfirms the
bitstream gain indices.

## Local Oracle Probe

The test-only diagnostic window scan added in commit `af458fc` compares
candidate perturbations against filled `past_exc_pre_acb_q0` oracle rows.

Command:

```sh
env GOCACHE=/tmp/go-build \
  G729_DECODER_TAME_PAST_EXC_VARIANT_WINDOW_SCAN=1 \
  G729_DECODER_TAME_PAST_EXC_VARIANT=pitch_gain_cap_0p95 \
  G729_DECODER_TAME_PAST_EXC_SCAN_START=112 \
  G729_DECODER_TAME_PAST_EXC_SCAN_END=144 \
  G729_DECODER_ITU_VECTOR_FRONTIER_TOP=8 \
  go test ./internal/decoder -run TestDecoderTAMEPastExcVariantWindowScan -count=1 -v
```

Result:

```text
production: count=918 exact=0 wantRMS=204.89 gotRMS=384.97 errRMS=232.73 meanAbs=194.62 maxAbs=447 corr=0.8622 scale=0.4589
best pitch_gain_cap_0p95 window [112,132): exact=16 errRMS=83.99 meanAbs=64.34 maxAbs=176 corr=0.9380 scale=0.8108
```

For comparison, the older fixed-gain damping probe is weaker on the same
oracle surface:

```text
best fixed_gain_half window [40,72): exact=11 errRMS=153.75 meanAbs=128.23 maxAbs=273 corr=0.9604 scale=0.5795
```

## Interpretation

`pitch_gain_cap_0p95` is a strong localization probe: limiting adaptive gain
for global subframes `112..131` makes the later TAME past-excitation history
much closer to oracle.

It is not yet a production fix. The PDF-visible decoder clause in
`docs/superpowers/specs/itu/G729E.txt` section `4.1.5` describes
reconstructing the adaptive gain from the received gain-codebook index through
equation `(73)`. It does not expose a decoder-side pitch-instability/taming
state machine. The same recommendation text lists `Taming.c` as pitch
instability control in the simulation-software inventory, but the clean-room
rule forbids inspecting that implementation source.

Therefore:

- Do not add an unconditional decoder `gp <= 0.95` clamp to production.
- Do not treat the incomplete 108..137 verifier CSV as a decoder oracle.
- Next useful work is either a PDF-visible/state-machine derivation of
  decoder pitch-instability control, or a diagnostic-only stateful taming probe
  validated against `TAME.PST`, `SPEECH.PST`, and the filled late `pastExc`
  oracle rows before any production change.

## Verification

```sh
env GOCACHE=/tmp/go-build go test ./internal/decoder -count=1
env GOCACHE=/tmp/go-build G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -count=1 -v
```

