# Decoder ITU FCB Position Clarification Prompt

You are an isolated clean-room verifier. Do not inspect, return, or describe
implementation code. Return only numeric scalar answers and short controlled
notes.

The local decoder ITU stage partial oracle has one unresolved surface: three
13-bit fixed-codebook indices produce fourth-pulse position disagreements in
`fixed_c_q13`. Fill only the blank cells in:

```text
testdata/oracle/handoff/decoder_itu_fcb_position_clarification_expected_template.csv
```

Please independently decompose these `C` values using the G.729 fixed-codebook
pulse-position equation:

```text
C = i0 + 8*i1 + 64*i2 + 512*(2*i3 + jx)
m0 = 5*i0
m1 = 5*i1 + 1
m2 = 5*i2 + 2
m3 = 5*i3 + 3 + jx
```

Return a CSV with exactly this header:

```csv
C,i0,i1,i2,i3,jx,m0,m1,m2,m3,note
```

Template rows:

```csv
C,i0,i1,i2,i3,jx,m0,m1,m2,m3,note
4099,,,,,,,,,,
3587,,,,,,,,,,
4183,,,,,,,,,,
```

Rules:

- Preserve the row order.
- Fill every numeric cell with a signed decimal integer.
- `note` must be one of: `formula_ok`, `formula_conflict`, or `unknown`.
- Return the filled file as
  `decoder_itu_fcb_position_clarification_expected.csv`.
- Do not add implementation-derived names, source locations, snippets, branch
  descriptions, or provenance details.
- If any row conflicts with the equation above, set `note=formula_conflict`
  and still return the numerically derived decomposition you believe follows
  the G.729 specification.

After copying the filled file into `testdata/oracle/handoff`, the implementation
side compare command is:

```sh
G729_COMPARE_DECODER_ITU_FCB_POSITION_CLARIFICATION=1 \
G729_REQUIRE_COMPLETE_DECODER_ITU_FCB_POSITION_CLARIFICATION=1 \
G729_REQUIRE_EXACT_DECODER_ITU_FCB_POSITION_CLARIFICATION=1 \
go test ./internal/fcb -run TestOracleHandoff_CompareDecoderITUFCBPositionClarification -count=1 -v
```
