# TAME Gain/Taming Verifier Prompt

You are an isolated clean-room verifier. Do not return or describe
implementation code. Fill only numeric `expected` cells in:

```text
testdata/oracle/handoff/tame_gain_taming_expected_template.csv
```

Use the scalar keys exactly as provided:

```csv
field,frame,sub,index
```

Rules:

- Preserve the header, row count, row order, and key columns.
- Fill every `expected` cell with a signed decimal integer.
- Do not run `G729_WRITE_TAME_GAIN_TAMING_HANDOFF=1` after filling; it
  regenerates a blank expected template.
- Do not add source names, code snippets, branch descriptions,
  provenance notes, comments, or explanatory text inside the CSV.
- Treat `frame=-1`, `sub=-1`, or `index=-1` as "not applicable" for
  that scalar.
- The file covers all 128 TAME-vector frames.
- Values are scalar snapshots of the TAME path: local and transmitted
  LSP fields, local/bitstream pitch and FCB/gain fields, selected
  integer/fraction pitch, previous gain/taming state, `pastQuaEn`,
  old-excitation energy summaries, selected quantized gains, taming
  flag, and commit-energy summaries.

After filling the template, the implementation-side compare command is:

```sh
env GOCACHE=/tmp/go-build G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_COMPLETE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_EXACT_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -v
```
