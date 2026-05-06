# Remaining Conformance Verifier Prompt

You are an isolated clean-room verifier. Do not inspect or describe any
G.729 implementation source code. Fill only numeric `expected` cells in
the two CSV templates listed below.

Do not run any `G729_WRITE_*_HANDOFF=1` command after filling these
files. Those commands regenerate blank templates. If a refresh is
absolutely required, do it before filling, or only with explicit intent
to discard verifier output.

## Files To Fill

1. `testdata/oracle/handoff/lsp_decision_expected_template.csv`
   - Rows: 1472 data rows plus header
   - Header:
     ```csv
     field,frame,tap,L0,L1,L2,L3,col,expected
     ```
   - Key:
     ```csv
     field,frame,tap,L0,L1,L2,L3,col
     ```
   - Detailed task prompt:
     `testdata/oracle/handoff/LSP_DECISION_VERIFIER_PROMPT.md`

2. `testdata/oracle/handoff/tame_gain_taming_expected_template.csv`
   - Rows: 8962 data rows plus header
   - Header:
     ```csv
     field,frame,sub,index,expected
     ```
   - Key:
     ```csv
     field,frame,sub,index
     ```
   - Detailed task prompt:
     `testdata/oracle/handoff/TAME_GAIN_TAMING_VERIFIER_PROMPT.md`

## Rules

- Preserve each header exactly.
- Preserve row count, row order, and key columns exactly.
- Fill every `expected` cell with a signed decimal integer.
- Do not run write-refresh commands after filling; run only the strict
  compare commands below.
- Do not add source names, code snippets, branch descriptions,
  provenance notes, comments, or explanatory text inside the CSV files.
- Treat `-1` in a key column as "not applicable" for that scalar.
- Return only the filled CSV files, or a short completion note outside
  the CSV files.

## Local Strict Compare Commands

After `lsp_decision_expected_template.csv` is filled:

```sh
env GOCACHE=/tmp/go-build G729_COMPARE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_EXACT_LSP_DECISION_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPDecisionHandoff -v
```

After `tame_gain_taming_expected_template.csv` is filled:

```sh
env GOCACHE=/tmp/go-build G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_COMPLETE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_EXACT_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -v
```

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg, Sipro Lab, or any other
G.729 implementation code. Verifier output may enter this repository
only as numeric scalar oracle artifacts, deltas, controlled notes, and
aggregate histograms.
