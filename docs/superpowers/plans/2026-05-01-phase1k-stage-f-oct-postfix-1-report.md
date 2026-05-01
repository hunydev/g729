# Phase 1k Stage F-oct-postfix-1 보고서 — RED 확인

**작성일**: 2026-05-01
**범위**: failing regression test 작성 + RED 의무 입증.
**산출물**: `internal/decoder/stagef_octpostfix_regression_test.go` 신규 1 파일 + RED 출력 verbatim 인용.
**준수**: F-oct-prelim-5-4 §6 결정 (a) production fix cycle 진입.
**production 변경**: 0 라인. **테스트 변경**: 1 신규 파일 (~43 라인).

## 0. Working tree 상태 + escape hatch 평가 (E1–E5)

진입 시점 `git status --porcelain`:

```
?? internal/decoder/stagef_bis_diagnostic_test.go
```

`git log -1 --oneline` = `b921b2d docs(plans): add Phase 1k Stage F-oct-postfix plan` — plan §Phase 0.1 와 일치.

| 해치 | 상태 | 평가 |
|------|------|------|
| E1 | 미발동 | 본 commit 후 신규 회귀 0건 (Phase 0.2 의 비-contract 3건 FAIL 유지). |
| E2 | 미발동 | PDF §A.4.2.3 verbatim 인용 = plan §"Spec § 인용" 인용 1 과 일치. |
| E3 | N/A | Task 2 영역. |
| E4 | 미발동 | 외부 G.729 구현 0 참조. |
| E5 | 미발동 | production 변경 0 라인. |

## 1. 회귀 게이트 baseline (13 항목 PASS)

`go vet ./...` clean. package-level `go test`:

```
ok  	github.com/exedev/g729/internal/postfilter
ok  	github.com/exedev/g729/internal/synth
ok  	github.com/exedev/g729/internal/pcm
ok  	github.com/exedev/g729/internal/fcb
ok  	github.com/exedev/g729/internal/pitch
ok  	github.com/exedev/g729/internal/lsp
```

`internal/decoder/` 패키지의 신규 RED 외 FAIL 1건 (`TestDiagnostic_SinglePulseChain`), `internal/gain/` 의 FAIL 2건 (`TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 은 plan §Phase 0.2 의 비-contract diagnostic 3건 FAIL 유지 — 본 commit 으로 인한 신규 회귀 0건.

## 2. PDF §A.4.2.3 raw 인용 + strict reading 채택 근거

ITU-T G.729 (06/2012) PDF p.43 §A.4.2.3 verbatim:

> The value of γt = 0.8 is used if k1′ < 0 and γt is set to zero if k1′ ≥ 0.

분기 조건의 ground-truth = **k1' 의 부호** (g_l 값 또는 voicing 상태와의 명시적 결합 *부재*).

## 3. RED 출력 verbatim (sample 5..7)

```
=== RUN   TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput
    stagef_octpostfix_regression_test.go:38: frame 0 sample 5: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 6: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 7: got=2 want=-1 (Δ=3)
--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput (0.00s)
```

3 sample 모두 부호 반전 (got=+2, want=-1, Δ=3). plan 본문의 "got=+1" 가정과 비교 시 부호는 동상 (양수), magnitude 만 +1 차이 — F-oct-prelim-5-4 §3.2 측정 시점 이후 어떤 후속 commit (Phase 0.1 표 외 미식별) 이 magnitude 를 미세 변동시켰을 가능성. 본 cycle 의 fix 입증 대상은 **부호** 이므로 cycle premise 유지.

## 4. F-oct-prelim-5-4 §3.6 spec 해석과 PDF 원문의 불일치 정량 기록

F-oct-prelim-5-4 §3.6 표는 spec 인용을 "γ_t = 0.9 if long-term postfilter active (g_l > 0), else 0.2 (§A.4.2.3)" 로 표기. PDF §A.4.2.3 원문은 분기 변수를 **k1' 의 부호** 로 명시 — `g_l` 또는 long-term postfilter active 상태와의 명시적 결합 *없음*. 본 cycle 은 strict reading (k1' 부호) 을 ground-truth 로 채택 (plan §"핵심 spec 해석" 결정 동상).

## 5. Task F-oct-postfix-2 진입 의무 항목 (γ_t 선택 분기 fix scope)

- `internal/postfilter/tilt.go:65-68` 의 분기 조건 1 라인: `pf.agcGainPrev == 0` → `k1 >= 0`.
- 같은 파일 line 23-25 의 docstring 갱신 (Phase 1g proxy 자기-인정 → §A.4.2.3 정합 명시).
- signature 변경 0 — `postfilter.go:44` 호출부 변경 0 라인.
- 본 RED test 가 GREEN 으로 전환됨이 Task 2 의 단일 입증 의무.
