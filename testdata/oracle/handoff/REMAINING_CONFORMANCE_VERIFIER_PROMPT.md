# Remaining Conformance Verifier Prompt

You are an isolated clean-room verifier. Do not inspect or describe any
G.729 implementation source code. Fill only numeric `expected` cells in
the CSV templates listed below.

Do not run any `G729_WRITE_*_HANDOFF=1` command after filling these
files. Those commands regenerate blank templates. If a refresh is
absolutely required, do it before filling, or only with explicit intent
to discard verifier output.

## Current Blank Templates

The currently relevant unfilled conformance handoff is:

1. `testdata/oracle/handoff/encoder_closedloop_stage_expected_template.csv`
   - Rows: 100848 data rows plus header
   - Header:
     ```csv
     field,frame,sub,index,lag,frac,expected
     ```
   - Key:
     ```csv
     field,frame,sub,index,lag,frac
     ```
   - Required row-key/local-observation file:
     `testdata/oracle/handoff/encoder_closedloop_stage_got.csv`
   - Detailed task prompt:
     `testdata/oracle/handoff/ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md`
   - Status: broad closed-loop stage conformance surface; optional follow-up
     if a broader independent oracle is required after the focused FCB
     surfaces exact-passed.

Older handoff templates may still exist in this directory for historical
or H-CENTER workflows. This consolidated prompt covers the current
unfilled conformance templates that matter for the active Core-quality
alignment work.

## Completed Focused FCB Templates

These templates have already been filled by an isolated verifier, validated
locally, and strict-compared with zero mismatches:

- `testdata/oracle/handoff/fcb_tree_search_expected_template.csv`
- `testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv`

The focused FCB outputs remain in this handoff directory as verifier-filled
numeric oracle artifacts. Do not overwrite them with write-refresh commands
unless explicitly discarding verifier output.

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

## Important Distinction

The completed focused FCB handoffs and the remaining broad handoff use their
`got` files differently:

- For the FCB tree-search handoff, the verifier must use
  `fcb_tree_search_got.csv` as the numeric `d_abs`/`sign`/`phi` search
  surface and fill the expected template with numeric oracle results.
  Copy-through input rows should match the numeric `got` values after
  import verification.
- For the user-audio FCB tree-search handoff, the verifier must use
  `fcb_tree_search_user_audio_got.csv` the same way, but the surface comes
  from the converted user problem sample rather than `SPEECH.IN`.
- For the encoder closed-loop stage handoff, the verifier must not copy
  `encoder_closedloop_stage_got.csv` values into `expected`. That file is
  only for row keys and later local comparison after independent numeric
  calculation.

## Local Strict Compare Commands

Completed focused FCB strict compare:

```sh
G729_COMPARE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_HANDOFF=1 \
go test -run TestOracleHandoff_CompareFCBTreeSearchHandoff -count=1 -v
```

Completed user-audio focused FCB strict compare:

```sh
G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v
```

After `encoder_closedloop_stage_expected_template.csv` is filled:

```sh
G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
G729_REQUIRE_COMPLETE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
G729_REQUIRE_EXACT_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 \
go test -run TestOracleHandoff_CompareEncoderClosedLoopStageHandoff -count=1 -v
```

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg source, Sipro Lab, or any
other G.729 implementation code. External tools may be used only as
black-box executables when a detailed task prompt permits it. Verifier
output may enter this repository only as numeric scalar oracle artifacts,
deltas, controlled notes, and aggregate histograms.
