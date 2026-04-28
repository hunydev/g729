# Phase 1k Stage F-quint Production Fix Cycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** F-quart-4 가 식별한 시나리오 B (다중 fix 필수) 의 본 stimulus 영향 결함 3건 — (4a) `ecDbQ10` int16 silent overflow + (4b) Q26-vs-Q0 보정 누락 + (1) §3.9.3 GainImap inverse-map 누락 — 을 *strict ordering* 으로 fix 해 ALGTHM frame 0 sample 0..7 PST/2 비트-정확 정렬을 회복하고 `TestDecode_Frame0Sample0_MatchesALGTHM` baseline FAIL (got=4 want=2) 을 PASS 로 회복한다.

**Architecture:** 2 production fix commit + 1 완료 보고서 commit. **C1 (Task F-quint-1)** = (4a)+(4b) 동시 fix (`internal/gain/decode.go:70-72`, 같은 caller, 상쇄 결함은 단독 fix 시 ÷64 dB 또는 ×8192 위험 → 동시 fix 의무). **C2 (Task F-quint-2)** = (1) GainImap inverse-map fix (`internal/gain/vq.go::decodeVQ` + 동 파일 `vq.go:14-17` docstring 의 §3.9.3 모순 문구 수정). **Task F-quint-3** = 완료 보고서 + F-sext 권고 (잔여 보류 항목 4건). 각 commit 직후 *반드시* 강압-적합 회피 게이트 (정렬도 측정 + 회귀 게이트 + spec § 재인용) 통과 — 악화 시 즉시 revert.

**Tech Stack:** Go 1.22 + ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) + 기존 F-quart-1 / F-quart-3 진단 하니스. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조**.

---

## Phase 0 — 사이클 입구 invariant + escape hatch 사전합의

### Phase 0.1 Working tree 사전 상태 (F-quint 진입 시점)

| 경로 | 상태 | F-quint 변경? |
|------|------|------|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix int64 누산 | **No** (보존, 별도 cycle 처리) |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (untracked) — F-bis/F-tris 진단 baseline | **No** (보존) |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed (F-quart-1/F-quart-3 진단 하니스) | **Yes** (assertion promotion 가능) |
| `internal/gain/decode.go` | F-quart-4 시점 그대로 | **Yes** (Task F-quint-1) |
| `internal/gain/vq.go` | F-quart-4 시점 그대로 | **Yes** (Task F-quint-2) |
| 그 외 production 파일 | 미변경 | **No** |

F-quint 신규 / 수정 파일:
- (Task F-quint-1) `internal/gain/decode.go` modify (caller side Q-format 보정 + int32 보존)
- (Task F-quint-1) `internal/decoder/stagef_quart_diagnostic_test.go` modify (cross-check 의 t.Logf → t.Fatalf assertion promotion + RED→GREEN gate)
- (Task F-quint-1) `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-1-report.md` (보고서, staged → committed)
- (Task F-quint-2) `internal/gain/vq.go` modify (decodeVQ + docstring)
- (Task F-quint-2) `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-2-report.md` (보고서, staged → committed)
- (Task F-quint-3) `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-3-report.md` (완료 보고서)

본 cycle 의 production 변경 범위는 `internal/gain/decode.go` + `internal/gain/vq.go` *2 파일*. 다른 production 패키지 (synth, postfilter, pcm, fcb, pitch, decoder/decode.go, decoder/subframe.go, lsp/) 변경 절대 금지.

### Phase 0.2 회귀 게이트 명세

각 commit (C1, C2) 직후 *반드시* 실행:

1. **Stage D 17 contract test**: `internal/synth/`, `internal/postfilter/`, `internal/pcm/`, `internal/gain/`, `internal/fcb/`, `internal/pitch/`, `internal/lsp/`, `internal/decoder/` 의 contract spec test. 본 cycle 의 어떤 fix 도 회귀 0 의무.
2. **Stage D-bis 3 contract test**: F-bis-1 P fix 검증 + LSP 합성 cross-check + 추가 contract.
3. **Phase 1i sample 0 가드** (`TestDecode_Frame0Sample0_MatchesALGTHM`): C1 직후는 *baseline FAIL 허용* (got=4 want=2 또는 다른 값), C2 직후는 **PASS 의무 (got=2 want=2)**. C2 후 FAIL 시 즉시 revert + 시나리오 C reclassification.
4. **F-quart-3 reference cross-check** (`TestDiagnostic_FquartGainReferenceCrossCheck`): C1 후 *prod = ref* 의무 (ALGTHM frame 0 sf0/sf1 의 `gp_q14` + `gc_q12` 비트-정확 일치). C2 후도 *prod = ref* 의무 (indexing 만 변경되어도 reference impl 가 동일 GA/GB 으로 호출되므로).
5. **F-quart-1 alignment harness** (`TestDiagnostic_FquartGainImap_Sf0Sample0to7`): C1 직후 + C2 직후 측정. C2 후 hpFilter sample 0..7 의 |Δ|≤1 LSB sample 수가 8/8 또는 40-sample matches가 40/40 에 근접해야 함 (강압-적합 회피 — 정렬도 *향상*만 보고, 절대값 강제 X).

### Phase 0.3 Escape hatch (E1·E2·E3·E4·E5)

| 해치 | 발동 조건 | 발동 시 행동 |
|------|---------|------|
| **E1** | C1 적용 후 회귀 게이트 1+ FAIL (Stage D 17 / D-bis 3 의 임의 test) | 즉시 `git revert HEAD` + 보고서에 회귀 trace 기록 + F-quint-1 재설계 |
| **E2** | C1 적용 후 F-quart-3 cross-check 가 prod ≠ ref 잔존 | C1 fix 가 spec 식 (66)–(72) 의 *정확한* Q-format 사슬을 회복하지 못한 것 → 즉시 revert + 추가 Q-format hand-trace 의무 (보고서에 정량 trace 추가) |
| **E3** | C2 적용 후 `TestDecode_Frame0Sample0_MatchesALGTHM` FAIL 잔존 | 시나리오 C reclassification — F-quart-3 §6.4 가설 부정. 즉시 보고서에 결과 기록 + F-sext cycle 권고 (frame 1+ 진단 또는 ALGTHM cross-validation 정책 변경) |
| **E4** | 외부 G.729 구현 (ITU C / bcg729 / Sipro / FFmpeg) 1건이라도 인용/대조 흔적 발견 | 즉시 작업 중단 + 사용자 통보 + 해당 인용 제거 후 재시작 |
| **E5** | C1 또는 C2 가 본 plan 의 명시된 *2 production 파일* 외 production 파일을 변경 | 즉시 `git revert HEAD` + commit 재구성 (변경 범위 축소) |

각 보고서 (F-quint-1/2/3) §0 에 *해치 평가표* 포함 의무.

### Phase 0.4 강압-적합 (forced-fit) 회피 의무

각 fix 적용 직후 *반드시* 다음을 수행:
1. **정렬도 측정**: F-quart-1 alignment harness 재실행 → Branch A/B (= C1 후 production / C2 후 production+inverse-map) 의 sample 0..7 + 40-sample matches.
2. **spec § 재인용**: 각 fix 의 commit message 본문 + 보고서 §1 에 spec 식 verbatim 인용.
3. **악화 시 즉시 revert**: sample 0..7 의 임의 sample |Δ| 가 *증가* 하거나 40-sample matches 가 *감소* → 해당 commit 즉시 `git revert`.
4. **의도적 fit 강제 금지**: 측정값을 의도적으로 PST/2 와 더 가깝게 보이도록 fix 를 *조정* 하는 것 금지. 본 plan 의 각 step 은 spec § 식의 *직접 도출* 만 허용; spec 외 휴리스틱 0.

---

## Task F-quint-1: C1 — `ecDbQ10` int16 overflow + Q26 보정 누락 동시 fix

**Goal:** `internal/gain/decode.go:70-72` 의 caller side Q-format 사슬을 §3.9 식 (66)/(67) + `internal/gain/energy.go:18-22` docstring 의 약속에 정합하도록 수정한다. 두 결함은 상쇄적 (net +14.267 dB 오류) 이므로 *반드시 동시 fix* (단독 fix 시 ÷64 dB 또는 ×8192 의 큰 편차 도입 — F-quart-3 §6.3). C1 후 F-quart-3 cross-check 의 prod = ref 의무 통과.

**Files:**
- Modify: `internal/gain/decode.go:70-72` (caller side: `ecLog2Q10` 에 Q26 보정 + `ecDbQ10` 를 int32 보존)
- Modify: `internal/decoder/stagef_quart_diagnostic_test.go` (`TestDiagnostic_FquartGainReferenceCrossCheck` 의 t.Logf 비교를 t.Fatalf assertion 으로 promote — RED→GREEN gate)
- Create: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-1-report.md`

### Spec § 인용 (C1 fix 의 spec 근거)

ITU-T G.729 (06/2012) §3.9 (PDF p.22-23) verbatim:

식 (66): `Ē = 10·log₁₀(1/40 · Σ c(n)²)`.

`internal/gain/energy.go:18-22` docstring (production 자체 규정):

> "Callers must ALSO apply a Q-format correction to account for the Q26-vs-Q0 mismatch against the spec's log2 of a Q0 sum: see the comment in decode.go at the `ecLog2Q10 = ... - 26*1024` line."

→ caller 가 `ecLog2Q10` 에 `−26·1024` 적용 의무. 미적용 시 spec true `ec_bar_db` 대비 `−26·10·log₁₀(2) = −78.268 dB` 오차.

또한 `ecDbQ10 = (ecLog2Q10 · dbPerLog2Q13 + (1<<12)) >> 13` 의 결과는 c[]=`[8192]·4 + [1639]·4` (= ALGTHM frame 0 sf0) 입력에서 `86485 (Q10)` ≈ `84.46 dB`. int16 캐스트 시 `int16(86485) = 20949` 로 *high bits silent-discard*. 이는 식 (67) 의 `Ē_c` 입력에 +64.000 dB silent recovery 도입 → spec-위반.

두 결함 net 효과: −78.268 + 64.000 = **−14.267 dB** ec_bar_db 오차 → log10_gc0_db = +14.267 dB 오차 → gc 가 spec true 값의 `10^(14.267/20) ≈ 5.17x`. (**F-quart-3 §5.3 정량 trace 와 정확히 정합**.)

C1 fix 후 ec_bar_db 가 spec 값과 일치 → log10_gc0_db, gc0, gc 모두 spec 일치 → F-quart-3 cross-check 의 prod = ref 회복.

- [ ] **Step 1: Working tree pre-check + baseline 측정**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected:
```
M internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```
(F-quart-4 종료 시점과 동일. 다른 production 변경이 보이면 즉시 작업 중단.)

Run: `go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v`

Expected: PASS (t.Logf 만 출력, assertion 0). raw output 발췌:
```
[P] sf0  PROD: gp_q14= 13815  gc_q12=  6844
[P] sf0  REF : gp_q14= 13815  gc_q12= 32767   (gc_true=8.633396)
[P] sf0  Δgp_q14 = +0   Δgc_q12 = -25923
[S] sf0  PROD: gp_q14=  1995  gc_q12=   803
[S] sf0  REF : gp_q14=  1995  gc_q12=  4151
[S] sf0  Δgp_q14 = +0   Δgc_q12 = -3348
```

이 출력을 보고서 §3.1 의 *RED baseline* 으로 인용.

- [ ] **Step 2: assertion promotion — RED gate 작성**

`internal/decoder/stagef_quart_diagnostic_test.go::TestDiagnostic_FquartGainReferenceCrossCheck` 본문에 *Δ assertion* 추가. 기존 t.Logf 출력은 보존 (디버그용); 추가로:

```go
// After computing prodGpQ14, prodGcQ12, refGpQ14, refGcQ12 for each
// branch (P, S) and each subframe (sf0, sf1):

if prodGpQ14 != refGpQ14 {
    t.Fatalf("[%s] sf%d gp_q14 mismatch: prod=%d ref=%d (Δ=%+d)",
        branchTag, sf, prodGpQ14, refGpQ14, prodGpQ14-refGpQ14)
}
if prodGcQ12 != refGcQ12 {
    t.Fatalf("[%s] sf%d gc_q12 mismatch: prod=%d ref=%d (Δ=%+d)",
        branchTag, sf, prodGcQ12, refGcQ12, prodGcQ12-refGcQ12)
}
```

본 step 의 assertion 은 *Branch P + Branch S 의 sf0/sf1 모든 4 비교점*에 적용. C1 fix 적용 전 RED, C1 fix 적용 후 GREEN 이 되어야 한다.

**주의**: Branch P sf1 은 F-quart-3 §5.1 에서 *prod = ref = 32767 (saturated)* 로 우연 일치 했으므로 C1 fix 전부터 PASS 일 수 있다. 그러나 C1 fix 후에는 saturation 이 발생하지 않을 수 있으므로 (`ecBarDb` 가 −10 dB 정상화 → `gc_q12` 가 saturating 영역 밖일 가능성) 본 assertion 이 새 GREEN 영역에서도 통과해야 한다. C1 fix 결과 saturation 분포가 변하면 보고서 §3 에 정량 trace 명시 의무.

- [ ] **Step 3: RED 확인**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v`

Expected: **FAIL** (즉시 첫 mismatch 에서 t.Fatalf). 첫 expected 실패 위치:
```
[P] sf0 gc_q12 mismatch: prod=6844 ref=32767 (Δ=-25923)
```
또는 (assertion 순서에 따라):
```
[S] sf0 gc_q12 mismatch: prod=803 ref=4151 (Δ=-3348)
```

FAIL 확인 = RED gate 정상 작동.

**중요**: 본 step 은 *fix 적용 전*에 반드시 RED 확인. RED 미달성 (= test 가 PASS) 시 assertion 작성 결함 → step 2 재작성.

- [ ] **Step 4: C1 fix 작성 — `internal/gain/decode.go:70-72`**

현재 코드 (`internal/gain/decode.go:70-72`):
```go
ecLog2Q10 := log2Fixed(ecEnergy)
ecDbQ10 := int16((int32(ecLog2Q10)*dbPerLog2Q13 + (1 << 12)) >> 13)
ecBarDbQ10 := fixed.Sub(ecDbQ10, tenLog10_40Q10)
```

수정 코드 (예시 — 정확한 Q-format 사슬은 `energy.go:18-22` docstring + 식 (66) 직접 도출):
```go
// Q26-vs-Q0 correction: fixedCodebookEnergy returns Σ c(n)² in Q26
// (each c(n) is Q13). Spec equation (66) takes log of a Q0 sum, so
// shift the log2 result by 26 bits (= -26*1024 in Q10).
ecLog2Q10 := int32(log2Fixed(ecEnergy)) - 26*1024
// dB conversion: keep int32 to avoid silent int16 overflow on large
// energies. ALGTHM frame 0 sf0 produces ecLog2 ~28 (Q10=28730), and
// after dbPerLog2Q13 multiply the int32 result is ~84.46 dB Q10
// (= 86485), which would silently truncate if cast to int16.
ecDbQ10 := (ecLog2Q10*dbPerLog2Q13 + (1 << 12)) >> 13
ecBarDbQ10 := ecDbQ10 - int32(tenLog10_40Q10)
```

후속 사용처 (`decode.go:75`) 의 `fixed.Sub(predicted, ecBarDbQ10)` 는 두 인자 모두 Word16 을 받으므로 int32 ecBarDbQ10 을 그대로 넘기면 컴파일 오류. *반드시 saturating int16 변환* 추가:

```go
// Saturate to int16 for downstream Word16 arithmetic.
var ecBarDbQ10W16 fixed.Word16
if ecBarDbQ10 > 32767 {
    ecBarDbQ10W16 = 32767
} else if ecBarDbQ10 < -32768 {
    ecBarDbQ10W16 = -32768
} else {
    ecBarDbQ10W16 = fixed.Word16(ecBarDbQ10)
}
// 3. Effective log gain in dB → log2.
logGainDbQ10 := fixed.Sub(predicted, ecBarDbQ10W16)
```

또는 `fixed` 패키지의 saturating-cast helper 가 이미 존재하면 그것을 사용. (`fixed.SatInt16` 또는 동등) — `internal/fixed/` 의 exported API 검토 후 선택.

**중요**: int32 보존 구간을 *최소* 로 한정. saturating cast 직후 `fixed.Sub(predicted, ecBarDbQ10W16)` 는 기존 Word16 산술. C1 fix 의 변경 범위는 `decode.go:70-75` 의 4-6 라인.

본 step 은 *fix 작성 only*. 실행 / GREEN 확인은 다음 step.

- [ ] **Step 5: GREEN 확인 — F-quart-3 cross-check**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v`

Expected: **PASS**. 모든 4 비교점 (Branch P sf0/sf1, Branch S sf0/sf1) 에서 prod = ref. raw output 발췌 예시:
```
[P] sf0  PROD: gp_q14= 13815  gc_q12= 32767  ← ref 와 일치
[P] sf0  REF : gp_q14= 13815  gc_q12= 32767
[P] sf0  Δgp_q14 = +0   Δgc_q12 = +0       ← assertion PASS
[S] sf0  PROD: gp_q14=  1995  gc_q12=  4151  ← ref 와 일치
[S] sf0  REF : gp_q14=  1995  gc_q12=  4151
[S] sf0  Δgp_q14 = +0   Δgc_q12 = +0       ← assertion PASS
```

FAIL 시:
- 어느 분기·sf 의 어느 항목 mismatch 인지 raw output 으로 확인.
- Q-format 보정 라인 (`-26*1024`) 누락 또는 위치 오류 검토.
- int32 → int16 saturating cast 의 boundary 처리 검토.
- 위 검토로 fix 를 *최소* diff 로 수정. 재실행 → GREEN 확인.
- 3회 시도 후에도 GREEN 미달성 시 보고서 §의 추가 hand-trace 의무 + E2 발동 검토.

- [ ] **Step 6: F-quart-1 alignment harness 측정 (E3 risk gate)**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v`

본 harness 는 t.Fatalf assertion 가 Branch A sanity check 1건 (`synth.Filter[0..7] == [2 3 4 4 3 2 1 1]`) 만 가지므로 *반드시 PASS* 여야 한다. C1 fix 후 Branch A 의 synth.Filter sample 0..7 이 변경되면 sanity check FAIL → C1 fix 가 baseline 정렬을 *변경* 했음을 의미.

**예상 동작 (F-quart-3 §5.1 의 ref 값 기반)**: C1 fix 후 Branch A 의 gc_q12 가 6844 → 32767 (saturating) → 합성 출력이 더 큰 진폭. 기존 sanity check `synth.Filter[0..7] == [2 3 4 4 3 2 1 1]` 가 FAIL 가능 → sanity check 자체를 갱신하거나 *제거* 해야 한다.

C1 fix 후 새 baseline 측정 → 그 값을 보고서 §3.2 에 정확히 기록. sanity check 의 갱신은:

```go
// Pre-C1: synth.Filter[0..7] = [2 3 4 4 3 2 1 1]
// Post-C1: synth.Filter[0..7] = [측정값]
// 본 sanity check 는 C1 의 정렬도 변화를 포착하기 위해 *경고만* 출력
// (t.Fatalf → t.Logf 로 강제 변경). C2 후 다시 assertion 으로 promote 검토.
```

또는 sanity check 를 *유연한 비교* 로 갱신 — 본 step 은 production 코드 수정이 아니라 test 수정이므로 자유도 높음. 단 test 변경도 *최소* 로 한정 (불필요한 갱신 금지).

**E3 risk 측정**: Branch A hpFilter sample 0..7 + 40-sample matches 를 *기록*. C1 fix 후 다음 4 가지 시나리오 중 1개로 분류:

| 시나리오 | hpFilter 변화 | 40-sample matches | 의미 |
|---------|--------------|-----------------|------|
| (i) 향상 | 일부 |Δ| 감소 | 증가 | C1 단독으로도 정렬 향상 — 기대치 낮으나 가능 |
| (ii) 변화 적음 | 거의 동일 | ±1~2 변동 | C1 의 영향이 indexing 결함에 가려짐 — 기대 |
| (iii) 악화 | 일부 |Δ| 증가 | 감소 | E3 risk 발현 — but 본 plan §3.3 표 예측 (saturation 영역으로 진입) 과 정합. C2 후 정렬 회복 가설 유지. |
| (iv) 심각 악화 | 모든 sample 큰 |Δ| | 큰 감소 | C1 fix 가 spec 식 도출에서 벗어남 — 즉시 revert + 재설계 |

(i)–(iii) 은 C1 보고서 §의 *기록* 만, C2 진행. (iv) 발현 시 즉시 revert + E2 발동.

- [ ] **Step 7: 회귀 게이트 통과 확인**

Run (전체 회귀 게이트 — 각 명령 *반드시* 실행 + 결과 보고서 §4 에 기록):

```bash
# Stage D 17 contract test (path가 정확하지 않을 수 있음 — make / project test runner 사용 권고)
go test ./internal/...
```

Expected:
- `TestDecode_Frame0Sample0_MatchesALGTHM` 는 *baseline FAIL 허용* (got 값이 변할 수 있으나 want=2 와 일치하지 않아도 OK — C2 까지 보류).
- 그 외 모든 test PASS.

**FAIL 분류**:
- Stage D 17 / D-bis 3 contract test 의 임의 1건 FAIL → **E1 발동, 즉시 revert + 재설계**.
- F-quart-3 cross-check FAIL → **E2 발동** (Step 5 에서 이미 검출).
- F-quart-1 alignment harness sanity check FAIL → Step 6 의 처리 분기.
- `TestDecode_Frame0Sample0_MatchesALGTHM` FAIL with got != 4 → 정렬도 변화 발생, *허용* (C2 까지 보류).

- [ ] **Step 8: F-quint-1 보고서 작성**

`docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-1-report.md`:

```markdown
# Phase 1k Stage F-quint-1 보고서 — C1 ec 체인 동시 fix

**작성일**: 2026-04-28
**범위**: F-quart-3 §6.1 의 (1)+(2) 결함 (`ecDbQ10` int16 silent overflow + Q26 보정 누락) 동시 fix.
**산출물**: F-quart-3 cross-check 의 prod = ref 회복 + F-quart-1 alignment 측정 + 회귀 게이트 결과.
**준수**: ITU-T G.729 (06/2012) PDF §3.9 / §4.1.6 만 인용. 외부 구현 0건 참조.

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)
## 1. §3.9 식 (66) + energy.go docstring 인용
## 2. C1 fix diff (`internal/gain/decode.go:70-75`)
## 3. RED→GREEN trace
   3.1 RED baseline (Step 1 출력)
   3.2 C1 fix 적용 후 cross-check 출력 (Step 5)
   3.3 F-quart-1 alignment harness 출력 (Step 6, 시나리오 i/ii/iii/iv)
## 4. 회귀 게이트 결과 (Step 7)
## 5. C2 진입 권고
```

- [ ] **Step 9: Working tree 검증 + commit**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected:
```
M internal/lsp/lsp_lp.go
M internal/gain/decode.go         ← Task F-quint-1 변경
M internal/decoder/stagef_quart_diagnostic_test.go  ← assertion promotion
?? internal/decoder/stagef_bis_diagnostic_test.go
```

**E5 검증**: `internal/gain/decode.go` 외 production 파일 변경 0 라인. `git diff -- internal/synth/ internal/postfilter/ internal/pcm/ internal/fcb/ internal/pitch/ internal/lsp/ internal/gain/vq.go internal/gain/energy.go internal/decoder/decode.go internal/decoder/subframe.go` → 모두 empty diff 의무.

```bash
git add internal/gain/decode.go \
        internal/decoder/stagef_quart_diagnostic_test.go \
        docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-1-report.md
git commit -m "$(cat <<'EOF'
fix(gain): apply Q26-vs-Q0 correction and preserve int32 in ec dB chain

Per ITU-T G.729 (06/2012) §3.9 equation (66) and the production
docstring contract in internal/gain/energy.go:18-22, the caller of
fixedCodebookEnergy MUST apply a Q26→Q0 correction (-26·1024 in Q10)
before converting log2 to dB. The previous code at decode.go:70-72
omitted this correction (-78.268 dB lost) and additionally truncated
ecDbQ10 to int16, silently discarding +64 dB of high bits for typical
ALGTHM frame 0 sf0 inputs (ec_int ~2.79e8, log2 ~28, dB ~84.46).
Net effect: gc was 5.17x smaller than spec value.

This fix applies both corrections atomically (each correction in
isolation produces -64 dB or -78 dB extreme errors that would mask
the other). Cross-check against a §3.9 reference impl confirms
prod=ref for ALGTHM frame 0 sf0/sf1 under both production and
§3.9.3 inverse-mapped GA/GB indexing branches.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-quint-2: C2 — §3.9.3 GainImap inverse-map fix

**Goal:** `internal/gain/vq.go::decodeVQ` 가 비트스트림 GA/GB 에 대해 §3.9.3 의 인버스 매핑 (`tables.GainImap1[GA]` / `tables.GainImap2[GB]`) 을 적용해 GBK 의 *물리 entry index* 를 회복하도록 수정. 동시에 `vq.go:14-17` docstring 의 §3.9.3 모순 문구 ("play no role at the decoder") 를 spec-correct 문구로 교체. C2 후 `TestDecode_Frame0Sample0_MatchesALGTHM` PASS 회복 의무.

**Files:**
- Modify: `internal/gain/vq.go::decodeVQ` (인버스 매핑 적용, 2 줄)
- Modify: `internal/gain/vq.go:14-17` (docstring spec 정합)
- Create: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-2-report.md`

### Spec § 인용 (C2 fix 의 spec 근거)

ITU-T G.729 (06/2012) §3.9.3 verbatim (PDF p.22):

> "To reduce the impact of single bit errors, the GA and GB indices are reordered before transmission. The mapping tables are given in Annex C/D."

`internal/tables/gain_gbk1.go:36-44` GainImap1 docstring (production self-citing):

> "GainImap1 is the inverse of GainMap1: given the transmitted GA bit pattern, the decoder looks up `entry = GainGBK1[GainImap1[GA]]`."

→ 디코더 측 inverse map 의무 명시. `decodeVQ` 가 미적용 → spec-위반.

식 (73): `ĝ_p = GA_1(GA) + GB_1(GB)` (GA/GB 는 *물리 entry index*; reorder 적용된 비트스트림 비트가 아님 — §3.9.3 가 reorder 를 *전송 비트 순서* 에만 적용한다고 명시).

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected (F-quint-1 commit 후):
```
M internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```
(F-quint-1 의 변경은 commit 되어 있으므로 working tree 는 F-quart-4 직후와 같은 상태 + 추가 commit 1개.)

- [ ] **Step 2: RED gate — Phase 1i sample 0 가드 baseline FAIL 확인**

Run: `go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v`

Expected: **FAIL**. 본 test 는 Phase 1i 도입 시점부터 baseline FAIL (got=4 want=2). C1 fix 후 got 값이 변경되었을 가능성 — 보고서 §3.1 에 *F-quint-1 commit 후의 got 값* 정확히 기록.

본 step 은 *RED 확인 only* — fix 적용 전.

- [ ] **Step 3: C2 fix 작성 — `internal/gain/vq.go::decodeVQ`**

현재 코드 (`internal/gain/vq.go:18-22`):
```go
func decodeVQ(idx Indices) (gpQ14, gammaCQ13 int16) {
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[idx.GA][0]), fixed.Word16(tables.GainGBK2[idx.GB][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[idx.GA][1]), fixed.Word16(tables.GainGBK2[idx.GB][1])))
	return
}
```

수정 코드:
```go
func decodeVQ(idx Indices) (gpQ14, gammaCQ13 int16) {
	gaEntry := tables.GainImap1[idx.GA]
	gbEntry := tables.GainImap2[idx.GB]
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][0]), fixed.Word16(tables.GainGBK2[gbEntry][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][1]), fixed.Word16(tables.GainGBK2[gbEntry][1])))
	return
}
```

변경: `idx.GA` → `tables.GainImap1[idx.GA]`, `idx.GB` → `tables.GainImap2[idx.GB]` (GBK1/GBK2 인덱싱 양 라인). 2 임시 변수 도입 (가독성 + 4번 인덱싱의 caching).

배열 경계 검증: `GainImap1` 은 `[8]uint8` (= 3-bit input 정확 매핑), `GainImap2` 는 `[16]uint8` (= 4-bit input). `idx.GA` 는 3-bit (`internal/gain/types.go` 의 Indices 타입 검증), `idx.GB` 는 4-bit → 인덱싱 OOB 불가.

- [ ] **Step 4: docstring 수정 — `internal/gain/vq.go:14-17`**

현재 docstring (`vq.go:14-17`):
```go
// The stages are summed component-wise with Word16 saturation.  The
// codebooks are indexed directly by the received bits (GA, GB); the
// optional reorder tables (Map/Imap) live in tables for the encoder
// search routine and play no role at the decoder.
```

수정 docstring (spec-correct):
```go
// The stages are summed component-wise with Word16 saturation.  Per
// ITU-T G.729 §3.9.3, the encoder reorders GA/GB indices before
// transmission to reduce the impact of single bit errors, so the
// decoder MUST apply the inverse map (GainImap1/GainImap2) to recover
// the physical GBK entry index from the received bits.
```

본 docstring 은 §3.9.3 verbatim 인용을 자체 정당화로 사용. "play no role at the decoder" 모순 문구 *완전 제거*.

- [ ] **Step 5: GREEN 확인 — Phase 1i 가드 PASS**

Run: `go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v`

Expected: **PASS** (got=2 want=2).

**FAIL 처리**:
- got != 2 인 경우 = **E3 발동**: F-quart-3 §6.4 가설 (C1+C2 가 sample 1 잔존 완전 설명) 부정.
- 즉시 보고서 §3.2 에 got 값 기록 + F-quart-1 alignment harness 재실행 (Step 6) 결과 정량 분석.
- E3 발동 시 C2 commit 보류 (revert 가 아닌 *commit 미실행*; working tree 변경은 보고서 §의 분석 자료로 보존). 사용자 결정 요청 = F-sext cycle 권고.

- [ ] **Step 6: F-quart-1 alignment harness 재측정**

Run: `go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v`

C2 후 Branch A (production verbatim — 이제 spec-fix 적용된 것과 동치) 와 Branch B (Branch A 와 동일 결과 예상 — Branch B 는 test 가 추가로 inverse map 한 인덱스로 호출하므로 inverse-map-of-inverse-map = 원본).

**중요**: C2 fix 후 Branch A 가 spec-fix 동치 → Branch B 는 *추가로 한 번 더* inverse map 한 결과. 즉 Branch B 의 의미가 변경. 본 step 은 Branch A 의 sample 0..7 정렬도 측정만 의미 있다.

Expected (F-quart-3 §6.4 가설):
```
Branch A     synth.Filter       [측정값]                         ?/40
Branch A     postfilter.Filter  [측정값]                         ?/40
Branch A     hpFilter           [1 2 1 1 0 -1 -1 -1] 또는 ±1 LSB   40/40 또는 그에 근접
Branch A     pcm.ScaleUpSat     [2 4 3 3 1 -1 -1 -1] 또는 ±2 LSB   (PST 도메인)
```

PST/2 = `[1 2 1 1 0 -1 -1 -1]` 와의 |Δ|≤1 LSB 일치 sample 수 + 40-sample matches 측정. 8/8 + 40/40 의무 *아님* (강압-적합 회피) — 측정값을 그대로 기록.

- [ ] **Step 7: 회귀 게이트 통과**

Run: `go test ./internal/...`

Expected: 전체 PASS. 특히:
- `TestDecode_Frame0Sample0_MatchesALGTHM` PASS (Step 5).
- F-quart-3 cross-check 의 assertion (F-quint-1 Step 2 에서 추가) PASS — C2 fix 가 reference impl 에 대해서도 동치.
- Stage D 17 + D-bis 3 contract test 회귀 0.

**FAIL 분류**:
- Stage D 17 / D-bis 3 회귀 → **E1 발동, 즉시 revert + 재설계**.
- F-quart-3 cross-check FAIL → **E2 발동**: C2 fix 가 reference impl 의 `referenceDecode` 와 다른 indexing 사용 가능성. reference impl 도 *test 코드 자체에서 inverse map 적용*했는지 확인 (F-quart-3 §4.2 의 알고리즘 step 1 검토). 만약 reference impl 가 inverse map 적용 안된 채로 작성 되었다면 reference 도 spec-violation 동치 → reference 코드 수정 의무.

- [ ] **Step 8: F-quint-2 보고서 작성**

`docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-2-report.md`:

```markdown
# Phase 1k Stage F-quint-2 보고서 — C2 §3.9.3 GainImap inverse-map fix

**작성일**: 2026-04-28
**범위**: F-quart-1 §1 의 §3.9.3 디코더 측 inverse map 누락 fix + docstring 정합.
**산출물**: Phase 1i sample 0 가드 PASS + F-quart-1 alignment 8/8 (또는 그에 근접).
**준수**: ITU-T G.729 (06/2012) PDF §3.9.3 만 인용. 외부 구현 0건 참조.

## 0. Working tree 상태 + escape hatch 평가
## 1. §3.9.3 + GainImap1/GainImap2 docstring 인용
## 2. C2 fix diff (decodeVQ + 동 파일 docstring)
## 3. RED→GREEN trace
   3.1 RED baseline (Step 2 의 Phase 1i 가드 FAIL 출력)
   3.2 C2 fix 적용 후 PASS 출력 (Step 5)
   3.3 F-quart-1 alignment 재측정 (Step 6) — Branch A 의 sample 0..7 + 40-sample matches
## 4. 회귀 게이트 결과 (Step 7)
## 5. F-quint-3 (완료 보고서) 진입 + 잔여 보류 항목
```

- [ ] **Step 9: Working tree 검증 + commit**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected:
```
M internal/lsp/lsp_lp.go
M internal/gain/vq.go         ← Task F-quint-2 변경
?? internal/decoder/stagef_bis_diagnostic_test.go
```

**E5 검증**: `internal/gain/vq.go` 외 production 파일 변경 0 라인. `git diff -- internal/synth/ internal/postfilter/ internal/pcm/ internal/fcb/ internal/pitch/ internal/lsp/ internal/gain/decode.go internal/gain/energy.go internal/decoder/decode.go internal/decoder/subframe.go` → 모두 empty diff (단, decode.go 는 F-quint-1 commit 에 포함됨).

```bash
git add internal/gain/vq.go \
        docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-2-report.md
git commit -m "$(cat <<'EOF'
fix(gain): apply §3.9.3 inverse map to decode GA/GB indices

Per ITU-T G.729 (06/2012) §3.9.3, the encoder reorders the GA and
GB indices before transmission to reduce the impact of single bit
errors. The decoder MUST apply the inverse map (GainImap1/GainImap2)
to recover the physical GBK1/GBK2 entry index from the received
bits, as documented by the GainImap1/GainImap2 self-citing docstrings
in internal/tables/gain_gbk{1,2}.go.

The previous decodeVQ indexed GBK1/GBK2 directly by received GA/GB
bits, omitting the inverse map and selecting wrong codebook entries.
For ALGTHM frame 0 sf0 (GA=5, GB=6) the previous code selected
(g_p=13815, γ̂_c=12915) instead of the §3.9.3-correct (1995, 1516).

Combined with the C1 ec-chain fix, this restores TestDecode_Frame0
Sample0_MatchesALGTHM to PASS (got=2 want=2) and brings ALGTHM frame
0 sample 0..7 PST/2 alignment to 8/8 |Δ|≤1 LSB.

The vq.go decodeVQ docstring previously asserted that "the optional
reorder tables (Map/Imap) live in tables for the encoder search
routine and play no role at the decoder" — this contradicted §3.9.3
and is replaced with a spec-correct rationale.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task F-quint-3: 완료 보고서 + F-sext 권고

**Goal:** F-quint cycle 의 두 fix (C1, C2) 가 시나리오 B 가설 (sample 1 |Δ|=2 잔존 완전 설명) 을 검증했는지 종합 보고. 잔여 보류 항목 (filterSubframe ÷4/×4, β init Phase 1g, frame 1+ 잔여, ALGTHM cross-validation 정책) 을 F-sext cycle 권고로 정리.

**Files:**
- Create: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-3-report.md`
- **Modify: 없음** (production 변경 0)

- [ ] **Step 1: Working tree pre-check**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected: F-quint-2 commit 후 상태 (lsp_lp.go modified + stagef_bis_diagnostic_test.go untracked + 그 외 production 미변경; F-quint-1/F-quint-2 변경은 commit 됨).

- [ ] **Step 2: 종합 측정값 수집**

Run (각 명령 출력을 보고서 §1 에 인용):

```bash
git log --oneline -10
go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
go test ./internal/...
```

각 명령 PASS 의무 (Phase 1i 가드 + cross-check + alignment harness + 전체 회귀).

- [ ] **Step 3: F-quart-3 §6.4 가설 검증 결과 분석**

가설 (F-quart-3 §6.4): "C1+C2 동시 적용 후 sample 1 |Δ|=2 잔존이 *완전 설명* 됨 → Phase 1i 가드 PASS + alignment 8/8."

검증:
- Step 2 의 alignment harness 출력에서 Branch A hpFilter sample 0..7 |Δ|≤1 LSB sample 수 = ? / 8.
- 40-sample matches = ? / 40.
- 기대값 8/8 + 40/40 또는 그에 근접 (강압-적합 회피 — 절대값 강제 X).

가설 검증 결과 분류:
- (검증 성공) sample 0..7 = 8/8 + 40-sample 40/40 또는 매우 근접 (39/40, 38/40 정도) → 가설 강력 지지. F-sext cycle 권고는 잔여 보류 항목 (frame 1+ 등) 만.
- (부분 검증) sample 0..7 = 8/8 이지만 40-sample 의 일부 sample 미정렬 → frame 0 sf0 한정 검증 성공, sf1/frame 1+ 추가 결함 가능성 — F-sext cycle 의 *frame 1+ 진단 우선*.
- (검증 실패) sample 0..7 < 8/8 + Phase 1i 가드 PASS → test logic 검토 (가드는 sample 0 만 검증, 다른 sample 잔여 가능). 실패가 아니라 가드 정의 자체 한계.

- [ ] **Step 4: 잔여 보류 항목 정리**

F-quart-4 §5 의 5 항목을 본 cycle 결과로 갱신:

1. **filterSubframe ÷2/×2 (§3.10/§A.3.10 ÷4/×4)**: 본 stimulus 미-trigger 유지. F-sext-1 (가칭) 에서 trigger 가능 stimulus 인공 구성 후 fix.
2. **β init = 0.2 컨벤션**: Phase 1g 비트-스트림 reference 측정 후 결정. F-sext 범위 외.
3. **frame 1+ 잔여 결함 가능성**: F-quint cycle 은 frame 0 sf0 한정. frame 1+ 의 voiced sf 에서 b30 FIR 정확성 (`tables.PitchInterpFIR`), pastSynth/pastExc 의존 분기 등이 검증 미수행 — F-sext-2 (가칭) 진단 cycle 권고.
4. **ALGTHM cross-validation 정책**: 본 cycle 에서 변경 없음. 시나리오 C reclassification 시에만 재검토.
5. **Phase 1i 가드 외 추가 회귀 가드 도입**: F-quint cycle 의 sample 0..7 정렬을 영구 회귀 게이트로 promote 검토 (예: `TestDecode_Frame0Sample0to7_MatchesALGTHM`). F-sext-3 (가칭).

- [ ] **Step 5: F-sext cycle 권고서 작성**

`docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-3-report.md`:

```markdown
# Phase 1k Stage F-quint-3 완료 보고서 + F-sext 권고

**작성일**: 2026-04-28
**범위**: F-quint cycle 의 종합 검증 + 잔여 보류 항목 → F-sext cycle 권고.
**산출물**: F-quart-3 §6.4 가설 검증 결과 + F-sext task 후보 ranking.
**준수**: 본 보고서는 F-quart-1/2/3/4 + F-quint-1/2 보고서만 인용. spec § 인용 0 (이미 선행 보고서 단일 출처).

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5 종합)
## 1. F-quint cycle commit 요약 (git log 발췌)
## 2. 종합 회귀 게이트 결과
   2.1 Phase 1i sample 0 가드 (PASS)
   2.2 F-quart-3 cross-check (모든 4 비교점 prod=ref)
   2.3 F-quart-1 alignment harness (Branch A 의 sample 0..7 + 40-sample matches)
   2.4 Stage D 17 + D-bis 3 contract test (회귀 0)
## 3. F-quart-3 §6.4 가설 검증 결과 (성공/부분/실패 분류)
## 4. 잔여 보류 항목 + F-sext cycle 권고 (5 항목)
## 5. 결론 — Phase 1k Stage F closure
```

- [ ] **Step 6: Working tree 검증 + commit**

Run: `git status --porcelain && git diff --stat -- internal/`

Expected:
```
M internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-3-report.md
```

```bash
git add docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-3-report.md
git commit -m "$(cat <<'EOF'
docs(plans): add Stage F-quint completion report + F-sext recommendation

Stage F-quint cycle (C1 ec-chain + C2 §3.9.3 inverse-map) restores
ALGTHM frame 0 sample 0..7 PST/2 alignment to 8/8 |Δ|≤1 LSB and
brings TestDecode_Frame0Sample0_MatchesALGTHM to PASS, validating
the F-quart-3 §6.4 hypothesis that the sample 1 |Δ|=2 residual was
fully explained by the (4a)+(4b) ec-chain offset cancellation.

Pending items deferred to F-sext cycle: filterSubframe ÷4/×4 spec
recovery (frame 0 sf0 untriggerable), β init convention (Phase 1g
bitstream reference), frame 1+ residual diagnosis, ALGTHM cross-
validation policy, and sample 0..7 regression gate promotion.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Self-Review

**1. Spec coverage**:
- ✓ §3.9 식 (66) (= F-quint-1 C1 fix): caller side Q26 보정 + int32 보존.
- ✓ §3.9 식 (67) (간접): ecBarDb 정정으로 식 (67) `E(m) = 20·log(g_c) + Ē_c − Ē` 가 spec-correct.
- ✓ §3.9.3 (= F-quint-2 C2 fix): inverse map 적용.
- ✓ §3.9 식 (73)/(74): C2 fix 후 GBK 인덱싱이 *물리 entry* 를 회복.
- ✓ Phase 1i `TestDecode_Frame0Sample0_MatchesALGTHM` PASS 회복.
- ✓ Production 변경 범위 = `gain/decode.go` + `gain/vq.go` 2 파일 (E5 invariant).
- ✓ Escape hatch E1/E2/E3/E4/E5: Phase 0.3 + 모든 task §0.

**2. Placeholder scan**:
- F-quint-1 Step 4 의 saturating cast 코드 예시는 *fix 작성 의무* 명시 (정확한 helper API 는 `internal/fixed/` 검토 후 결정). placeholder 가 아닌 *구현 결정 자유도*.
- F-quint-1 Step 6 의 (i)/(ii)/(iii)/(iv) 시나리오는 측정 분류 표 — placeholder 아닌 분류 골격.
- F-quint-2 Step 6 의 "또는 ±1 LSB" 표현은 강압-적합 회피 의도된 자유도 (실측값 그대로 기록 의무).

**3. Type consistency**:
- `gpQ14` (int16, Q14): F-quint-2 일관.
- `gammaCQ13` (int16, Q13): F-quint-2 일관.
- `ecLog2Q10`: F-quint-1 Step 4 에서 int32 보존 (16 → 32 bit promotion 명시).
- `ecDbQ10`: F-quint-1 Step 4 에서 int32 보존, saturating cast 직후 `fixed.Word16` 으로 변환.
- `tables.GainImap1` (`[8]uint8`) / `tables.GainImap2` (`[16]uint8`): F-quint-2 Step 3 일관.

**4. 외부 구현 참조 0**: 모든 spec 인용 = ITU-T G.729 (06/2012) PDF + production self-citing docstring. 외부 G.729 구현 (참조 C, bcg729, Sipro, FFmpeg) 0 인용. ✓

**5. TDD 준수**:
- F-quint-1 Step 2-3 = RED gate 작성 + 확인.
- F-quint-1 Step 4 = 최소 fix.
- F-quint-1 Step 5 = GREEN 확인.
- F-quint-1 Step 7 = 회귀 게이트.
- F-quint-2 Step 2 = RED 확인 (Phase 1i 가드).
- F-quint-2 Step 3-4 = 최소 fix (코드 + docstring).
- F-quint-2 Step 5 = GREEN 확인.
- F-quint-2 Step 7 = 회귀 게이트.
- F-quint-3 = 완료 보고서 (test 추가/변경 없음, 메타 task).

**6. 강압-적합 회피**:
- F-quint-1 Step 6 의 (iv) 시나리오 = 즉시 revert + E2 발동.
- F-quint-2 Step 5 의 FAIL = E3 발동, commit 보류 + 시나리오 C reclassification.
- F-quint-1 Step 4 / F-quint-2 Step 3 모두 *spec § 직접 도출* 만 허용; 휴리스틱 0.
- F-quint-2 Step 6 = 측정값 그대로 기록, 강제 PASS 금지.

**7. Commit 정책**:
- C1 (F-quint-1) = 1 commit (production 1 파일 + test 1 파일 + 보고서 1 파일).
- C2 (F-quint-2) = 1 commit (production 1 파일 + 보고서 1 파일).
- F-quint-3 = 1 commit (보고서 1 파일).
- 총 3 commit. 분리 commit → 각 fix 의 정렬도 영향 독립 측정 가능 (강압-적합 회피).

**8. Co-author trailer**: 3 commit 모두 `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` 포함.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-plan.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration. Per-task gates (Phase 0.2 / 0.3) catch regressions early.

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints.

**Which approach?**
