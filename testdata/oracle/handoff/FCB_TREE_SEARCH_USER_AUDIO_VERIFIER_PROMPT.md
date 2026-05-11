# FCB Tree-Search User-Audio Verifier Prompt

You are an isolated clean-room verifier. Do not inspect, return, or
describe G.729 implementation source code. Fill only numeric `expected`
cells in:

```text
testdata/oracle/handoff/fcb_tree_search_user_audio_expected_template.csv
```

Use this file as the numeric input surface and local-result reference:

```text
testdata/oracle/handoff/fcb_tree_search_user_audio_got.csv
```

Use the scalar keys exactly as provided:

```csv
field,frame,sub,index
```

Scope:

- The rows cover the converted user problem sample
  `testdata/external/user_quality_audio.m4a`.
- The implementation-side writer and comparer are pinned to that path;
  do not substitute another sample when filling this template.
- The pinned conversion is 8 kHz mono signed little-endian 16-bit PCM with
  `118701` samples (`237402` bytes) and SHA-256
  `e8d783af34de25d8d7d16a84dfe92238c647e4079a07d8dffd4e715a804ca5fa`.
- Frame numbers use 10 ms G.729 frame indexing after conversion to
  8 kHz mono signed 16-bit PCM.
- The included frame range is `292..294`, covering the reported
  2.9 second clipping region.

Rules:

- Preserve the header, row count, row order, and key columns.
- Fill every `expected` cell with a signed decimal integer.
- Match rows between the template and the `got` file only by
  `field,frame,sub,index`.
- Do not add source names, code snippets, branch descriptions,
  provenance notes, comments, or explanatory text inside the CSV.
- Treat `index=-1` as "scalar field, no element index".
- The `got` file provides numeric fixed-codebook search surfaces:
  `d_abs`, `sign`, and sign-folded `phi`.
- The `got` file also provides local diagnostic rows: pitch/search scalars,
  threshold statistics, accepted-prefix counts, current focused-search
  `selected_*` rows, and exhaustive local `full_*` rows.
- For input/copy-through rows, fill `expected` with the matching numeric
  `got` value after verifying the row was imported correctly.
- For `selected_*` rows, fill `expected` with the verifier oracle's exact
  reduced-complexity Annex A fixed-codebook tree-search result on the
  provided numeric `d_abs`/`phi` search surface.
- For `full_*` rows, fill `expected` with the verifier's independent
  exhaustive `C^2/E` result on the same numeric search surface.
- External G.729 implementations may be used only as black-box
  executables, never as source material.
- The verifier may use a black-box external oracle privately, but must
  return only this numeric CSV.

After filling the template, the implementation-side compare command is:

```sh
G729_COMPARE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_COMPLETE_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
G729_REQUIRE_EXACT_FCB_TREE_SEARCH_USER_AUDIO_HANDOFF=1 \
go test -run TestOracleHandoff_CompareFCBTreeSearchUserAudioHandoff -count=1 -v
```

## Clean-Room Boundary

Do not inspect ITU reference C, bcg729, FFmpeg source, Sipro Lab, or any
other G.729 implementation code. External tools may be used only as
black-box executables when needed. Verifier output may enter this
repository only as numeric scalar oracle artifacts, deltas, controlled
notes, and aggregate histograms.
