# Pitch Closed-Loop Search Verifier Prompt

You are an isolated clean-room verifier. Do not return or describe
implementation code. Fill only numeric `expected` cells in:

```text
testdata/oracle/handoff/pitch_closedloop_search_expected_template.csv
```

Use the scalar keys exactly as provided:

```csv
field,frame,sub,index,lag,frac
```

Rules:

- Preserve the header, row count, row order, and key columns.
- Fill every `expected` cell with a signed decimal integer.
- Do not add source names, code snippets, branch descriptions,
  provenance notes, comments, or explanatory text inside the CSV.
- Treat `-1` in a key column as "not applicable" for that scalar.
- The file covers the first four PITCH-vector frames and both
  subframes. Values are scalar snapshots of the closed-loop pitch
  search inputs and score surface: `a_hat`, residual, target `x`,
  impulse response `h`, backward target `xb`, `old_exc`, residual
  extension, search window, reference pitch fields, local selected
  pitch fields, and integer/fraction RN scores.

After filling the template, the implementation-side compare command is:

```sh
G729_COMPARE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 G729_REQUIRE_COMPLETE_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 G729_REQUIRE_EXACT_PITCH_CLOSEDLOOP_SEARCH_HANDOFF=1 go test -run TestOracleHandoff_ComparePitchClosedLoopSearchInputHandoff -v
```
