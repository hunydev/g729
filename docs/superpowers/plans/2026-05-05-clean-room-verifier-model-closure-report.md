# Clean-room Verifier Model Closure Report

**Date:** 2026-05-05
**Status:** CLOSED
**Scope:** independent black-box verifier workflow for the pure Go G.729 implementation.

## 1. Objective

Build a clean-room validation model that separates implementation work from external oracle verification. The implementation side may consume only structured numeric mismatch artifacts; no implementation code or implementation-derived hints are allowed into the repo.

## 2. Delivered Artifacts

| Requirement | Artifact |
| --- | --- |
| Clean-room verifier protocol | `docs/superpowers/plans/2026-05-05-clean-room-verifier-protocol.md` |
| Repository-level numeric-oracle rule | `AGENTS.md` |
| Oracle schema README | `testdata/oracle/README.md` |
| Artifact validator test | `oracle_artifact_test.go` / `TestOracleArtifacts_ValidateOptionalFiles` |
| Parser fixture tests | `TestOracleArtifacts_ParserAndSummaryFixtures` |
| Unsafe artifact rejection tests | `TestOracleArtifacts_RejectUnsafeFixtures` |
| Optional diagnostic consumer | `logOracleSummary` + `TestOracleArtifacts_ValidateOptionalFiles` |
| H-CENTER raw `T_op` integration hook | `TestOracleHCenter_TopOpenLoopOptionalDiagnostic` |

No production encoder/decoder code was changed.

## 3. Protocol Summary

The protocol defines three roles:

- implementation worker: reads only this repo, allowed specs, and approved explanatory material
- verifier: may run an external oracle elsewhere, but may provide only numeric artifacts
- reviewer: rejects artifacts containing implementation-derived information

Allowed artifact fields are:

`vector,frame,subframe,field,expected,got,delta,notes`

`notes` is restricted to:

`mismatch`, `out_of_window`, `range_ok`, `range_fail`, `unknown`

`delta` is defined as `got - expected`.

## 4. Validator and Consumer Behavior

`oracle_artifact_test.go` supports optional `.csv` and `.jsonl` files under `testdata/oracle/`.

Validation:
- exact schema/header for CSV
- JSONL object fields
- non-empty `vector` and `field`
- `frame >= 0`
- `subframe ∈ {-1,0,1}`
- `delta == got - expected`
- controlled `notes`
- raw-text scan for high-risk tokens and known implementation names

Diagnostic summary:
- total exact rate
- per-field exact rate
- `±1`, `±2`, `±5`, `±10` window rates
- delta histogram
- first 8 mismatches
- H-CENTER `top_open_loop` mismatch clusters by expected lag range

If no optional oracle artifacts are present, the diagnostic tests skip. Parser and rejection fixture tests still run in the default suite.

## 5. H-CENTER Workflow

The previous H-CENTER proxy CSV remains at:

`testdata/phase2b/hcenter_top_vs_t1.csv`

That file compares this implementation's open-loop `T_op` against P1-derived closed-loop `T1`, so it remains a proxy. The new oracle path is prepared for raw verifier open-loop rows:

```csv
vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,74,0,range_ok
```

When such rows are present, `TestOracleHCenter_TopOpenLoopOptionalDiagnostic` reports exact/window rates and range clusters. This lets a verifier provide raw `T_op` numbers without exposing how those numbers were produced.

## 6. Verification Gates

Focused oracle gate:

```sh
env GOCACHE=/tmp/go-build go test -run 'TestOracleArtifacts|TestOracleHCenter' -v
```

Result: PASS, with optional artifact tests skipped because no `.csv` or `.jsonl` oracle artifact is currently present.

Full gates:

```sh
env GOCACHE=/tmp/go-build go test ./...
env GOCACHE=/tmp/go-build go build ./...
env GOCACHE=/tmp/go-build go test -race ./...
```

Result: all passed.

Forbidden file-name/source drop scan:

```sh
rg --files | rg -i '(bcg729|ffmpeg|sipro|g729a|ld8|cod_ld8|dec_ld8|tab_ld8|pst\.c|taming\.c|\.(c|h)$)'
```

Result: no matches. This scan checks for prohibited implementation source files or implementation-named drops. Existing documentation contains clean-room attestation text naming prohibited implementations; that is not treated as source contamination.

## 7. Clean-room Attestation

No external G.729 implementation code was opened or copied. No new dependency was added. No decoder-side files, LSP codebook tables, release notes, tags, or README known-limitation sections were modified.

The new test surface is diagnostic-only and optional. Production behavior and hot-path allocation behavior are unchanged.

## 8. Remaining Manual Workflow

To use the verifier model:

1. Run the external verifier outside this implementation workspace.
2. Export only numeric rows matching `testdata/oracle/README.md`.
3. Place the artifact under `testdata/oracle/`.
4. Run `go test -run TestOracleArtifacts_ValidateOptionalFiles -v`.
5. If validation passes, run the relevant optional diagnostic, for example `TestOracleHCenter_TopOpenLoopOptionalDiagnostic`.
6. Use only the numeric summary to form new clean-room hypotheses.

— end of clean-room verifier model closure report —
