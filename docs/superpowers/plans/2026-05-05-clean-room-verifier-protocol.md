# Clean-room Verifier Protocol

**Date:** 2026-05-05
**Status:** ACTIVE
**Scope:** G.729 clean-room validation through numeric oracle artifacts.

## 1. Purpose

The Go implementation remains clean-room. A separate verifier may compare behavior against an external oracle, but the implementation side may receive only numeric mismatch artifacts. The protocol is designed to let us diagnose conformance gaps without exposing implementation-derived code, names, branches, constants, or control flow.

## 2. Roles

**Implementation worker**
- May read this repository, the allowed ITU specification text/PDF already stored under `docs/superpowers/specs/itu/`, local plans, and public non-implementation explanatory material approved by the task.
- Must not read ITU reference C, bcg729, FFmpeg, Sipro Lab, or any other G.729 implementation.
- May consume only oracle artifacts that pass `TestOracleArtifacts_ValidateOptionalFiles`.

**Verifier**
- May run an external oracle in a separate environment.
- Must provide only numeric artifacts using `testdata/oracle/README.md`.
- Must not provide code fragments, function names, variable names, line references, branch descriptions, or magic-number provenance.

**Reviewer**
- Checks that artifacts contain only approved schema fields and controlled notes.
- Rejects artifacts that mention implementation names, implementation source locations, or algorithmic hints beyond scalar mismatches.

## 3. Allowed Artifact Information

Per row:
- `vector`: vector name, for example `PITCH`
- `frame`: zero-based frame index
- `subframe`: `-1`, `0`, or `1`
- `field`: scalar field name, for example `P1`, `P2`, `top_open_loop`
- `expected`: verifier oracle scalar
- `got`: this implementation's scalar
- `delta`: `got - expected`
- `notes`: one controlled value: `mismatch`, `out_of_window`, `range_ok`, `range_fail`, or `unknown`

Allowed aggregate summaries:
- per-field exact rate
- delta histogram
- first N mismatches by frame/subframe/field
- window rates such as exact, `±1`, `±2`, `±5`, and `±10`

## 4. Forbidden Artifact Information

Artifacts must not contain:
- code snippets
- implementation function names
- implementation variable names
- source file paths or line references
- branch names or algorithm-step labels copied from another implementation
- explanations for magic-number origins
- names or URLs of known G.729 implementations

The validator intentionally scans raw artifact text for high-risk words and implementation names. The scan is conservative; if a legitimate artifact is rejected, rename the field or note using the controlled schema rather than weakening the clean-room boundary.

## 5. Review Procedure

1. Verifier produces CSV or JSONL under `testdata/oracle/`.
2. Implementation worker runs:

   ```sh
   go test -run TestOracleArtifacts_ValidateOptionalFiles -v
   ```

3. If validation fails, the artifact is removed or redacted before any diagnostic use.
4. If validation passes, optional diagnostic consumers may summarize numeric mismatches.
5. Production code changes must cite specification text or local clean-room measurements, not external implementation details.

## 6. H-CENTER Example

The previous H-CENTER proxy compared this implementation's open-loop `T_op` with `PITCH.BIT` P1-derived closed-loop `T1`, which is only a consistency proxy. A future verifier may provide raw oracle open-loop pitch as:

```csv
vector,frame,subframe,field,expected,got,delta,notes
PITCH,0,-1,top_open_loop,74,74,0,range_ok
```

The implementation side may then compute exact and windowed rates over `top_open_loop` without learning how the oracle produced the value.

## 7. Governance

Required gates for protocol changes:

```sh
go test ./...
go build ./...
go test -race ./...
```

The repository-level `AGENTS.md` records the numeric-oracle-only rule so future sessions inherit the clean-room boundary.
