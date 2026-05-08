# Phase 1k Stage F-bis Task 1 (F-bis-1) 진단 보고서

**상태**: F-bis-1 진단 완료. **Working tree는 의도적으로 RED 상태(미커밋)** — Phase 1i sample-0 잠금(`TestDecode_Frame0Sample0_MatchesALGTHM`)이 후보 P fix 단독 적용 시 got=4 want=2로 깨진 채 유지됨. F-bis-2 진입은 사용자 승인 대기.

**작성일**: 2026-04-27

**준수**: ITU 참조 C, bcg729, FFmpeg G.729 등 외부 구현 미참조. ITU-T G.729 §3.2.6 / §3.10 + LSP→LP 표준 이론(Schur–Cohn step-down)만 사용. 진단은 ITU 벡터(.BIT/.PST) 블랙-박스 I/O 비교.

---

## 1. Working tree 상태 (의도적 RED, 미커밋)

### 1.1 `git status` 출력 (verbatim)

```
On branch main
Your branch is ahead of 'origin/main' by 99 commits.
  (use "git push" to publish your local commits)

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
	modified:   internal/lsp/lsp_lp.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	internal/decoder/stagef_bis_diagnostic_test.go

no changes added to commit (use "git add" and/or "git commit -a")
```

### 1.2 변경 요약

| 경로 | 상태 | 역할 |
|------|------|------|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) | 후보 P fix: `f1, f2 [11]int64` exact 누산기 + `polyStepExact` (saturation 제거). 마지막 Q28→Q12 단계만 Word16 saturation 유지. |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (uncommitted) | F-bis-1 진단 하니스. ALGTHM frame 0 sf0 sample 0의 4개 stage 경계 측정. t.Errorf 미사용(진단-only). |

HEAD는 본 보고서 커밋 직전 시점에서 F-bis 플랜 commit(`f2988dd`) — production 변경/하니스는 모두 미커밋.

---

## 2. F-bis-1 Step 2: P fix working tree에서 `TestALGTHMFrame0SF0_AzStability` 재확인 (PASS)

```
$ go test -v -run "TestALGTHMFrame0SF0_AzStability|TestLSPToLPLeadingCoefficient|TestLSPToLPAllZeroLSPProducesSymmetric" ./internal/lsp/
=== RUN   TestLSPToLPLeadingCoefficient
--- PASS: TestLSPToLPLeadingCoefficient (0.00s)
=== RUN   TestLSPToLPAllZeroLSPProducesSymmetric
--- PASS: TestLSPToLPAllZeroLSPProducesSymmetric (0.00s)
=== RUN   TestALGTHMFrame0SF0_AzStability
    stability_test.go:78: a[] (int16 Q12) = [4096 -2197 -375 -4 -144 -68 303 -36 -90 145 -33]
    stability_test.go:79: a[] (float, Q12-normalized) = [1 -0.536376953125 -0.091552734375 -0.0009765625 -0.03515625 -0.0166015625 0.073974609375 -0.0087890625 -0.02197265625 0.035400390625 -0.008056640625]
    stability_test.go:103: step m=10: k=-0.008057
    stability_test.go:103: step m=9: k=0.031081
    stability_test.go:103: step m=8: k=-0.006054
    stability_test.go:103: step m=7: k=-0.009197
    stability_test.go:103: step m=6: k=0.068325
    stability_test.go:103: step m=5: k=0.020135
    stability_test.go:103: step m=4: k=-0.017413
    stability_test.go:103: step m=3: k=-0.011058
    stability_test.go:103: step m=2: k=-0.096825
    stability_test.go:103: step m=1: k=-0.595293
    stability_test.go:105: A(z) minimum-phase confirmed; reflection coefficients all |k|<1. Stage F branch = synth.Filter (LP synthesis IIR primitives).
--- PASS: TestALGTHMFrame0SF0_AzStability (0.00s)
PASS
ok  	github.com/hunydev/g729/internal/lsp	0.002s
```

**판정**: 모든 |k_m| < 1 → §3.2.6 minimum-phase 회복. Stage F partial §2.3에서 측정된 값(`k_10 = −0.008057`, `k_1 = −0.595293` 등)과 정확히 일치. **P fix 정상 적용 확인.**

---

## 3. F-bis-1 Step 4: Stage-by-stage sample 0 진단 (verbatim t.Logf)

```
$ go test -v -run TestDiagnostic_FbisStageBoundaries_Sample0Trace ./internal/decoder/
=== RUN   TestDiagnostic_FbisStageBoundaries_Sample0Trace
    stagef_bis_diagnostic_test.go:63: u[0..3] = [2 2 2 2] (BuildExcitation output)
    stagef_bis_diagnostic_test.go:64: a[] (Q12) = [4096 -2197 -375 -4 -144 -68 303 -36 -90 145 -33]
    stagef_bis_diagnostic_test.go:69: BOUNDARY synth.Filter:      sample 0 = 2  (sRaw[0..3] = [2 3 4 4])
    stagef_bis_diagnostic_test.go:74: BOUNDARY postfilter.Filter: sample 0 = 2  (sPf[0..3] = [2 2 3 4])
    stagef_bis_diagnostic_test.go:79: BOUNDARY hpFilter:          sample 0 = 2  (hpOut[0..3] = [2 2 3 3])
    stagef_bis_diagnostic_test.go:84: BOUNDARY pcm.ScaleUpSat:    sample 0 = 4  (scaled[0..3] = [4 4 6 6])
    stagef_bis_diagnostic_test.go:87: PST want sample 0..3 = [2 4 3 3]
    stagef_bis_diagnostic_test.go:89: Δ at each boundary vs PST sample 0 (want=2):
    stagef_bis_diagnostic_test.go:90:   synth.Filter:      Δ=0  ratio_to_want=1.000
    stagef_bis_diagnostic_test.go:91:   postfilter.Filter: Δ=0  ratio_to_want=1.000
    stagef_bis_diagnostic_test.go:92:   hpFilter:          Δ=0  ratio_to_want=1.000
    stagef_bis_diagnostic_test.go:93:   pcm.ScaleUpSat:    Δ=2  ratio_to_want=2.000
    stagef_bis_diagnostic_test.go:95: Inter-stage ratios (sample 0):
    stagef_bis_diagnostic_test.go:96:   synth → postfilter:   1.000
    stagef_bis_diagnostic_test.go:97:   postfilter → hpFilter: 1.000
    stagef_bis_diagnostic_test.go:98:   hpFilter → ScaleUpSat: 2.000
--- PASS: TestDiagnostic_FbisStageBoundaries_Sample0Trace (0.00s)
PASS
ok  	github.com/hunydev/g729/internal/decoder	0.003s
```

### 3.1 Stage-by-stage Δ 표 (sample 0; PST want = 2)

| 경계 | sample 0 측정 | Δ (vs want=2) | ratio (vs want) | 인접 stage 변화 |
|------|---:|---:|---:|---|
| `synth.Filter` 직후 | **2** | 0 | 1.000 | — (시작점) |
| `postfilter.Filter` 직후 | **2** | 0 | 1.000 | ratio = 1.000 (변화 없음) |
| `hpFilter` 직후 | **2** | 0 | 1.000 | ratio = 1.000 (변화 없음) |
| `pcm.ScaleUpSat` 직후 | **4** | **+2** | **2.000** | **ratio = 2.000 ← 정확히 ×2 진입** |

### 3.2 판정 — 플랜 Step 4 outcome **(a)**

플랜 §F-bis-1 Step 4 결정 가이드:
> 1. 한 인접 경계 쌍에서 정확히 2x (또는 1/2x) 변화 발견 → **결함 위치 확정**.

**결과**: `hpFilter` → `pcm.ScaleUpSat` 경계에서 정확히 ratio = **2.000** 진입. 그 이전 3개 stage(synth.Filter, postfilter.Filter, hpFilter) 출력은 **PST want 와 sample 0 비트-정확 일치(Δ=0)**. 즉:

- **상류 3개 stage(synth/postfilter/hpFilter)는 P fix 후 PST 기준 sample 0 비트-정확.**
- **`pcm.ScaleUpSat`만이 sample 0의 want=2를 got=4로 만든다 — 정확히 ×2 결함.**

이는 플랜 §3.10 인용 "The output speech is finally divided by 2 with saturation control" 와 정합한다. 현 `internal/pcm/scale.go::ScaleUpSat`는 그 이름이 시사하듯 **×2(`fixed.Shl(in[i], 1)`)** 를 수행 — spec이 요구하는 ÷2의 정반대 방향.

### 3.3 sample 1..3 보조 관찰 (참고 — F-bis-2 분석 단서)

PST want 0..3 = `[2 4 3 3]`, hpFilter 0..3 = `[2 2 3 3]`, ScaleUpSat 0..3 = `[4 4 6 6]`.

| sample | want | hpFilter | ScaleUpSat | hpFilter 정확? | ScaleUpSat 정확? |
|---:|---:|---:|---:|---:|---:|
| 0 | 2 | 2 | 4 | ✓ | ✗ (got 4) |
| 1 | 4 | 2 | 4 | ✗ | ✓ (×2 우연 일치) |
| 2 | 3 | 3 | 6 | ✓ | ✗ (got 6) |
| 3 | 3 | 3 | 6 | ✓ | ✗ (got 6) |

→ `hpFilter` 출력이 거의 모든 샘플에서 PST와 일치(sample 1만 어긋남)하고, `pcm.ScaleUpSat`가 일관되게 ×2를 더 곱한다. **PST 그라운드 트루스는 ScaleUpSat 미적용(또는 ÷2)이 옳다.** sample 1의 단일 어긋남(want=4 vs hpFilter=2)은 별개 sub-issue 후보(상류 합성 단계의 ±1~2 LSB 변동)지만, **단일-경로 ×2 결함의 주범은 명백히 `pcm.ScaleUpSat`**.

---

## 4. 회귀 게이트 (`go test -race ./...`) — 예상 RED 1건만 발생

### 4.1 verbatim 결과 발췌

```
?   	github.com/hunydev/g729	[no test files]
ok  	github.com/hunydev/g729/internal/bitstream	(cached)
--- FAIL: TestDecode_Frame0Sample0_MatchesALGTHM (0.00s)
    frame0_regression_test.go:23: frame 0 sample 0: got=4 want=2 (Δ=2)
FAIL
FAIL	github.com/hunydev/g729/internal/decoder	0.023s
ok  	github.com/hunydev/g729/internal/fcb	(cached)
ok  	github.com/hunydev/g729/internal/fixed	(cached)
ok  	github.com/hunydev/g729/internal/gain	(cached)
ok  	github.com/hunydev/g729/internal/lsp	1.009s
ok  	github.com/hunydev/g729/internal/pcm	(cached)
ok  	github.com/hunydev/g729/internal/pitch	(cached)
ok  	github.com/hunydev/g729/internal/postfilter	(cached)
ok  	github.com/hunydev/g729/internal/synth	(cached)
ok  	github.com/hunydev/g729/internal/tables	(cached)
FAIL
```

### 4.2 판정

- **유일한 FAIL**: `TestDecode_Frame0Sample0_MatchesALGTHM` (frame 0 sample 0, got=4 want=2). **Stage F partial §2.4와 정확히 동일 — 의도된 RED, escape hatch 1 시나리오대로.**
- **Stage D 17 contracts (postfilter)**: PASS (`internal/postfilter` ok, cached).
- **Stage D-bis 3 contracts (postfilter)**: PASS (`internal/postfilter` ok).
- **synth Q-format contracts (Stage F-prep-2)**: PASS (`internal/synth` ok).
- **lsp ALGTHM stability + 기타 unit tests**: PASS (`internal/lsp` ok, 본 fix가 P 결함 해소).
- **Phase 1i 다른 잠금**: 본 RED 1건 외 회귀 없음.

회귀 게이트 통과 (예상 RED 외 GREEN).

---

## 5. F-bis-2 권고: 의심 stage = `pcm.ScaleUpSat`

**Outcome (a) 단일 stage 식별 결과: `internal/pcm/scale.go::ScaleUpSat`.**

F-bis-2 Step 1(가/나/다/라 분기 중 **(라) `pcm.ScaleUpSat` 단계**)에 진입 시 분석 항목:

1. **§3.10 인용**: "The output speech is finally divided by 2 with saturation control"
2. **§A.4.2 (G.729A 단순화) 인용 확인**: G.729A가 §3.10의 final ÷2 단계를 G.729와 동일하게 상속하는지 명시 인용 필요.
3. **production 비교 위치**: `internal/pcm/scale.go:17-25` (`ScaleUpSat`).
   - 현 구현: `out[i] = fixed.Shl(in[i], 1)` (×2 with saturation).
   - 함수 doc(`scale.go:5-15`)은 "decoder-side inverse of the 1/2 amplitude scaling applied in PreProcessor" 라고 설명 — 즉 **encoder PreProcessor의 ÷2를 가정한 decoder ×2**. 그러나 ITU PST 그라운드 트루스 비교는 본 가정을 부정한다(PST는 ÷2 후 amplitude). encoder PreProcessor의 ÷2가 ITU encoder 출력을 *모방*한다면, decoder는 ÷2(또는 무처리)가 옳다.
4. **encoder 측 대응 수정 필요성 검토**: `internal/pcm/preprocessor.go`의 ÷2 가정과 짝을 이루므로 encoder 측 (현 Phase에서 사용 안 됨이라도) 정합성 점검 항목.
5. **이전 Phase 1i `736beba` 잠금 시점의 잠재 보정 경로**: P fix 부재 시 `lspToLP` Q28 포화로 a[] 진폭이 비정상 축소되어, decoder ×2가 우연히 PST에 근사했음. P fix가 a[] 진폭을 복원하면서 ScaleUpSat 의 ×2가 폭로됨 — 본 보고서 §3 데이터가 이 가설을 직접 입증.
6. **F-bis-3 단일 커밋 묶음 후보**:
   - `internal/lsp/lsp_lp.go` (P fix, 본 working tree 변경)
   - `internal/pcm/scale.go::ScaleUpSat` (×2 → ÷2 또는 통과; spec § 인용 후 결정)
   - 필요 시 `internal/pcm/preprocessor.go` 정합성
   - sample 40 가드, 진단 하니스 보존/제거 결정 (Stage F 플랜 Task 3 Step 4 동일)

**escape hatch (c) 발동 조건 미충족**: 단일-경로 ×2 가설이 정확히 outcome (a)로 입증됨 → 가설 부정 → 롤백 권고는 해당 없음.

---

## 6. 다음 단계 — 사용자 승인 필요

- **본 단계(F-bis-1) 종료.** working tree는 의도된 RED + 미커밋 상태로 보존.
- **F-bis-2 진입 = 사용자 명시 승인 대기.** F-bis-2는 §3.10 + §A.4.2 인용으로 `pcm.ScaleUpSat`의 spec 위반을 line-by-line 확정 (분석-only, 코드 수정 없음).
- 본 보고서만 단일 커밋으로 랜딩(production 코드/하니스 미커밋 유지).

