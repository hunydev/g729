# Clean-room Oracle Artifacts

This directory is for optional verifier artifacts. The main test suite must pass when no artifact files are present.

Artifacts may be CSV (`*.csv`) or JSONL (`*.jsonl`). They must contain only numeric scalar comparisons and controlled notes.

Files under `testdata/oracle/handoff/` are verifier handoff material, not oracle artifacts. They are ignored by the optional artifact validator until a verifier produces a completed `.csv` or `.jsonl` file directly under `testdata/oracle/`.

## CSV Schema

Required header:

```csv
vector,frame,subframe,field,expected,got,delta,notes
```

Example:

```csv
vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,74,0,range_ok
PITCH,1,-1,top_open_loop,82,85,3,mismatch
```

## JSONL Schema

Each line is one object with the same fields:

```json
{"vector":"PITCH","frame":0,"subframe":-1,"field":"top_open_loop","expected":74,"got":74,"delta":0,"notes":"range_ok"}
```

## Field Rules

- `vector`: non-empty short vector identifier.
- `frame`: zero-based frame index.
- `subframe`: `-1` for frame-level fields, otherwise `0` or `1`.
- `field`: non-empty scalar field identifier such as `P1`, `P2`, or `top_open_loop`.
- `expected`: verifier oracle scalar.
- `got`: this implementation's scalar.
- `delta`: `got - expected`.
- `notes`: one of `mismatch`, `out_of_window`, `range_ok`, `range_fail`, `unknown`.

## Forbidden Content

Artifacts must not include implementation code, implementation-derived names, source locations, branch descriptions, magic-number explanations, or names/URLs of external G.729 implementations. The Go validator rejects artifact files containing high-risk tokens.

## H-CENTER Raw `T_op`

For future raw open-loop pitch verification, use:

```csv
vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,74,0,range_ok
```

`expected` is the verifier-provided raw open-loop pitch. `got` is this implementation's `T_op`. The optional H-CENTER diagnostic reports exact, `±1`, `±2`, `±5`, and `±10` rates plus a delta histogram.

To refresh the handoff files for an external verifier:

```sh
G729_WRITE_ORACLE_HANDOFF=1 go test -run TestOracleHCenter_WriteTopOpenLoopHandoff -v
```
