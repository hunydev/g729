# Remaining Conformance Oracle Reclassification Goal

## Copyable One-Line `/goal`

```text
/goal 남은 conformance expected failure 2개(TestEncode_LSPVectorBitExact, TestPhase2fTAME1_ByteEQ)를 clean-room numeric oracle 방식으로 재분류한다. LSP는 기존 frame0/source/VQ/predictor handoff 결과를 바탕으로 LSP.BIT byte-EQ가 production patch target인지 source-divergence diagnostic인지 확정하고, 부족하면 다중 프레임 LSP decision handoff를 추가한다. TAME은 PITCH/FCB source-divergence 상속인지 gain/taming 구현 문제인지 분리하기 위해 TAME vector의 pitch/FCB/gain/taming 입력·결정 scalar handoff를 설계하고 비교한다. 최종적으로 conformance expected failure inventory를 0~2개로 정확히 갱신하고 README/release docs/test disposition을 일관되게 정리한다. Clean-room boundary를 유지하고 외부 G.729 구현 코드는 보지 않는다.
```

## Objective

남은 conformance expected failure 2개를 숫자 oracle 기반으로 재분류한다.

- `TestEncode_LSPVectorBitExact`
- `TestPhase2fTAME1_ByteEQ`

목표는 각 실패를 다음 둘 중 하나로 명확히 분류하는 것이다.

- production patch target: clean-room 구현의 실제 수정 후보
- source-divergence diagnostic: `*.BIT` byte-EQ를 strict gate로 쓰면 안 되는 차이

## Work Plan

1. LSP failure 재분류
   - 기존 LSP table/predictor/frame0 VQ/source handoff 결과를 재확인한다.
   - `LSP.BIT` transmitted tuple과 local encoder-selected tuple의 source 차이를 문서화한다.
   - frame 0만으로 부족하면 frame 1..N LSP decision handoff를 추가한다.

2. TAME failure 분해
   - TAME vector에서 pitch, FCB, gain, taming 관련 scalar를 분리한다.
   - `P1/P2`, `C/S`, `GA/GB`, `taming`, `prevTaming`, `prevGpQ14`, `pastQuaEn`, `oldExc` 요약을 비교 가능한 numeric handoff로 만든다.
   - TAME mismatch가 pitch/FCB source-divergence 상속인지, gain/taming 구현 문제인지 판정한다.

3. Verifier handoff와 strict compare
   - 모든 handoff는 `testdata/oracle/handoff/` 아래에 둔다.
   - verifier는 numeric `expected` 값만 채운다.
   - strict compare 명령은 complete/exact env gate를 제공한다.

4. Test disposition 정리
   - source-divergence로 확정된 conformance test는 실패하지 않는 diagnostic PASS로 내린다.
   - 실제 production patch target으로 남는 항목만 expected failure inventory에 유지한다.

5. Documentation update
   - `README.md`
   - `docs/releases/v0.1.0-rc1.md`
   - `docs/releases/v0.1.0-rc1-checklist.md`
   - relevant plan/audit docs

## Success Criteria

- `go test ./...` PASS
- `go build ./...` PASS
- `go test -tags=conformance ./...`의 실패 목록이 문서화된 inventory와 정확히 일치
- 남은 failure가 있다면 numeric oracle 근거로 production patch target임이 설명됨
- source-divergence로 내린 항목은 byte-EQ를 고치지 않는 이유가 문서화됨

## Current Handoff Progress

### LSP Decision Handoff

Added:

- `internal/lsp/multiframe_decision_oracle_handoff_test.go`
- `testdata/oracle/handoff/lsp_decision_got.csv`
- `testdata/oracle/handoff/lsp_decision_expected_template.csv`
- `testdata/oracle/handoff/LSP_DECISION_VERIFIER_PROMPT.md`

Scope:

- first 16 `LSP.IN` / `LSP.BIT` frames
- 1472 data rows
- key: `field,frame,tap,L0,L1,L2,L3,col`
- fields include predictor memory, omega, weights, selector targets,
  encoder-selected tuple, transmitted tuple, and tuple cost/rank

Pre-fill hashes:

- `lsp_decision_got.csv`:
  `24f43d6b517b69dc58ca420cfddfcd9f6669c27909cb6a73d6353dd3e9c9a976`
- `lsp_decision_expected_template.csv`:
  `38a4ffad4894acf412d01048d14beb1c5b9e7ea8f033629cec1569e318489c13`
- `LSP_DECISION_VERIFIER_PROMPT.md`:
  `df89b6d9b8ea76be53e9f1cbd94abe3d3ea1262c389a47f4cb85868445a08724`

Strict compare after verifier fill:

```sh
env GOCACHE=/tmp/go-build G729_COMPARE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_EXACT_LSP_DECISION_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPDecisionHandoff -v
```

### TAME Gain/Taming Handoff

Added:

- `tame_gain_taming_oracle_handoff_test.go`
- `testdata/oracle/handoff/tame_gain_taming_got.csv`
- `testdata/oracle/handoff/tame_gain_taming_expected_template.csv`
- `testdata/oracle/handoff/TAME_GAIN_TAMING_VERIFIER_PROMPT.md`

Scope:

- all 128 `TAME.IN` / `TAME.BIT` frames
- 8962 data rows
- key: `field,frame,sub,index`
- fields include local/transmitted LSP, pitch, FCB, and gain fields,
  selected integer/fraction pitch, previous gain/taming state,
  `pastQuaEn`, old-excitation energy summaries, selected quantized
  gains, taming flag, and commit-energy summaries

Pre-fill hashes:

- `tame_gain_taming_got.csv`:
  `53927db37826977a1f0e6a2421f4ccb7a0a569998bae8461c479a6260c0af298`
- `tame_gain_taming_expected_template.csv`:
  `4b31d19b4fb910e579ccfd78babe80337798c8a6c9a3b90d29c54bd06b18fc2a`
- `TAME_GAIN_TAMING_VERIFIER_PROMPT.md`:
  `7a31b94f0edc9249f1de0e6522e10f136ef42c060262fbdee8d9e6f5e588d3e0`

Strict compare after verifier fill:

```sh
env GOCACHE=/tmp/go-build G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_COMPLETE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_EXACT_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -v
```

Current local verification:

- handoff structural integrity PASS
- `go test ./...` PASS
- `go build ./...` PASS
- `env GOCACHE=/tmp/go-build G729_COMPARE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_COMPLETE_LSP_DECISION_HANDOFF=1 G729_REQUIRE_EXACT_LSP_DECISION_HANDOFF=1 go test ./internal/lsp -run TestOracleHandoff_CompareLSPDecisionHandoff -v`
  PASS: exact 1472/1472, mismatches=0, blanks=0.
- `env GOCACHE=/tmp/go-build G729_COMPARE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_COMPLETE_TAME_GAIN_TAMING_HANDOFF=1 G729_REQUIRE_EXACT_TAME_GAIN_TAMING_HANDOFF=1 go test -run TestOracleHandoff_CompareTAMEGainTamingHandoff -v`
  PASS: exact 8962/8962, mismatches=0, blanks=0.

Verifier-filled hashes:

- `lsp_decision_expected_template.csv`:
  `7e3b481a2263fe8ed0811ad38b7fdfd0d1e0c69c01e6fdca6a3f623cda7b7f20`
- `tame_gain_taming_expected_template.csv`:
  `ab66f67c7f498cf41858f15c45faafaf949e36dcd1319054e69ef14580b85dd4`

Final disposition:

- `TestEncode_LSPVectorBitExact`: source-divergence diagnostic. The
  independent numeric oracle confirms the LSP decision surface, local
  encoder-selected tuple, transmitted tuple, and tuple cost/rank rows
  for the first 16 frames; byte equality against `LSP.BIT` is not a
  production patch gate.
- `TestPhase2fTAME1_ByteEQ`: source-divergence diagnostic. PITCH/FCB
  source-divergence is inherited, and the all-frame TAME gain/taming
  scalar handoff is exact; byte equality against `TAME.BIT` is not a
  production patch gate.
- Conformance expected-failure inventory: 0.

Write-refresh protection:

- Handoff write tests now refuse to overwrite an expected template that
  already contains verifier-filled cells.
- To intentionally discard verifier output and regenerate a blank
  template, set `G729_OVERWRITE_VERIFIER_EXPECTED=1` together with the
  relevant `G729_WRITE_*_HANDOFF=1` variable.
- After verifier fill, run only strict compare commands, not
  write-refresh commands.

Completion audit:

| Requirement | Evidence | Status |
|---|---|---|
| Reclassify `TestEncode_LSPVectorBitExact` | LSP tables, predictor residual, frame0 VQ/source, and first-16-frame decision handoffs are filled and exact. | Complete: source-divergence diagnostic |
| Reclassify `TestPhase2fTAME1_ByteEQ` | PITCH/FCB source-divergence handoff is complete; TAME gain/taming handoff is filled and exact. | Complete: source-divergence diagnostic |
| Keep default suite green | `env GOCACHE=/tmp/go-build go test ./...` PASS. | Complete |
| Keep conformance inventory aligned | `env GOCACHE=/tmp/go-build go test -tags=conformance ./...` PASS after test disposition updates. | Complete |
| Update docs/test disposition | README, release docs, checklist, and conformance tests reflect 0 expected failures. | Complete |

## Clean-Room Constraint

Do not inspect ITU reference C, bcg729, FFmpeg, Sipro Lab, or any other
G.729 implementation code. Verifier output may enter the repo only as
numeric scalar oracle artifacts, deltas, controlled notes, and aggregate
histograms.
