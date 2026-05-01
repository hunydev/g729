# Phase 1k Stage F-oct-postfix-2 보고서 — γ_t 분기 production fix (E3 발동: GREEN 미입증)

**작성일**: 2026-05-01
**범위**: internal/postfilter/tilt.go γ_t 선택 분기 fix 시도 + E3 escape hatch 발동.
**산출물**: 본 보고서 1건. production 변경 0 라인 (적용 후 revert).
**준수**: spec §A.4.2.3 strict reading + Phase 0.4 강압-적합 회피 + plan §69 E3 protocol.
**결과**: **E3 — fix 후에도 RED 잔존**. tilt.go 되돌림. Task 3/4 skip. Task 5 (다음 cycle 권고 갱신) 진입 권고.

---

## 0. Working tree 상태 + escape hatch 평가

진입 시점:

```
?? internal/decoder/stagef_bis_diagnostic_test.go
56caa72 (HEAD -> main) test(decoder): add Stage F-oct-postfix-1 ALGTHM sf0 sample 5..7 regression (RED)
```

진입 invariant 충족 (Phase 0.1).

E3 발동 사유: plan §69 — *"Task F-oct-postfix-2 의 fix 후에도 항목 14 가 RED 잔존 — 즉 γ_t 선택 fix 가 sample 5..7 부호를 spec want = `-1` 로 전환시키지 못함"*. 본 보고서 §3 raw 출력이 발동 근거.

E3 protocol (plan Step 3):
- Step 4 진입 금지 ✓
- `git checkout -- internal/postfilter/tilt.go` 로 fix 되돌리기 ✓
- 본 보고서 §3 에 fix 적용 후 sample 5..7 raw 출력 정량 기록 ✓
- Task F-oct-postfix-5 의 다음 cycle 권고 갱신 (본 보고서 §5) ✓
- E3 발동 시 Task 3/4 skip + Task 5 만 실행 ✓ (본 cycle 종결)

---

## 1. 변경 diff verbatim (E3 발동 후 revert 됨)

적용 시도된 diff (plan §"fix 후 코드 (제안)" 정합):

```diff
--- a/internal/postfilter/tilt.go
+++ b/internal/postfilter/tilt.go
@@ -20,9 +20,14 @@ const gammaTiltInactiveQ14 int16 = 3277 // round(0.2·2^14)
 //     k_1'  = -r_h(1) / r_h(0)
 //     μ     = γ_t · k_1'
 //
-// γ_t selection follows Annex A's voicing-dependent rule; for Phase 1g
-// we consult pf.agcGainPrev as a proxy for "long-term active" (non-zero)
-// vs "inactive" (zero).
+// γ_t selection follows ITU-T G.729 (06/2012) §A.4.2.3 (Annex A
+// p.43) and §4.2.3 (main p.29): γ_t depends on the sign of k1'.
+// If k1' < 0, the postfilter is "active" (γ_t = 0.9 in our Q14
+// constants, matching main §4.2.3); if k1' ≥ 0, γ_t = 0.2
+// (inactive, matching main §4.2.3). The Annex A spec §A.4.2.3
+// uses 0.8 / 0 for the active/inactive constants; the difference
+// vs. our 0.9 / 0.2 is tracked as a follow-up cycle (see Stage
+// F-oct-postfix-5 §3 잔여 보류 항목).
 func (pf *Postfilter) computeTiltMu(aNum, aDen *[lpcOrder + 1]int16) int16 {
@@ -63,7 +68,7 @@ func (pf *Postfilter) computeTiltMu(aNum, aDen *[lpcOrder + 1]int16) int16 {
        }
 
        gammaTQ14 := gammaTiltActiveQ14
-       if pf.agcGainPrev == 0 {
+       if k1 >= 0 {
                gammaTQ14 = gammaTiltInactiveQ14
        }
```

변경 라인 수 (적용 시점): docstring 6 + 분기 조건 1 = 7 라인. signature/import 변경 0. **현 working tree 에는 적용되어 있지 않음** (E3 revert 결과).

---

## 2. spec § 인용 ↔ 변경 라인 ↔ ALGTHM sample 5..7 부호 변화 3-way mapping

| spec § 인용 | 변경 라인 (revert 됨) | sample 5..7 부호 변화 |
|---|---|---|
| §A.4.2.3 "γt = 0.8 if k1' < 0, else 0" (PDF p.43) | `tilt.go:67` `if pf.agcGainPrev == 0` → `if k1 >= 0` | got=`[+2,+2,+2]` (pre-fix) → got=`[+2,+2,+2]` (post-fix). **부호 무변화**. want=`[-1,-1,-1]`. |

**plan §518 표의 예측치 (`got=[+1,+1,+1]` pre-fix → `[-1,-1,-1]` post-fix) 와의 괴리**:
- pre-fix 실측치 = `[+2,+2,+2]` (plan 표 +1 과 1 차이) — plan 작성 시점 측정 오차 또는 후속 cycle 영향.
- post-fix 실측치 = `[+2,+2,+2]` — 분기 flip 이 본 sample 의 부호/크기 모두 무변화. 즉 본 cycle 의 fix scope (γ_t 선택) 가 sample 5..7 에 전혀 영향을 주지 못함.

→ **결론**: ALGTHM frame 0 sf0 sample 5..7 의 spec want 와의 mismatch 는 γ_t 선택 분기 *조건* 결함이 아니다. 잔여 결함 위치는 별도 cycle 에서 식별 필요.

---

## 3. RED 잔존 raw 출력 (plan Step 3 의무 인용)

**Pre-fix** (`56caa72` HEAD, fix 미적용):

```
=== RUN   TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput
    stagef_octpostfix_regression_test.go:38: frame 0 sample 5: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 6: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 7: got=2 want=-1 (Δ=3)
--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput (0.00s)
```

**Post-fix** (`pf.agcGainPrev == 0` → `k1 >= 0` 적용 + `go clean -testcache`):

```
=== RUN   TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput
    stagef_octpostfix_regression_test.go:38: frame 0 sample 5: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 6: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 7: got=2 want=-1 (Δ=3)
--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput (0.00s)
```

**Δ = 0** (got 무변화). RED 잔존 → E3 발동.

**Post-revert** (`git checkout -- internal/postfilter/tilt.go`):

```
=== RUN   TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput
    stagef_octpostfix_regression_test.go:38: frame 0 sample 5: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 6: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 7: got=2 want=-1 (Δ=3)
--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput (0.00s)
```

→ Working tree = `56caa72` HEAD 동상 (untracked `stagef_bis_diagnostic_test.go` 만 잔존).

---

## 4. 14 회귀 게이트 — Step 4 미진입 (E3 protocol)

plan Step 3 E3 protocol 의 "Step 4 진입 금지" 의무 준수. 단, working tree 가 `git checkout` 로 HEAD `56caa72` 와 정확히 일치 (production 변경 0) 이므로 회귀 0 자명. 사후 sanity check 로 contract 부분집합 PASS 확인:

```
$ go test ./internal/postfilter/ ./internal/synth/ ./internal/decoder/ \
    -run "TestDecode_Frame0Sample0_MatchesALGTHM|TestDiagnostic_F(quart|sext|sept|octPrelim|OctPrelim5)"
ok    github.com/exedev/g729/internal/postfilter
ok    github.com/exedev/g729/internal/synth
ok    github.com/exedev/g729/internal/decoder
```

---

## 5. (Annex A 0.8/0 vs main 0.9/0.2 constant 차이) 잔여 보류 항목 + 다음 cycle 권고

**잔여 보류 항목** (plan §80 의무 명시):
- A. Annex A §A.4.2.3 strict 정합 위해서는 constant `gammaTiltActiveQ14 = round(0.8·2^14)`, `gammaTiltInactiveQ14 = 0` 으로 교체 필요. 본 cycle scope 외.
- B. **신규**: γ_t 선택 분기 fix 가 sample 5..7 부호 결함을 해소하지 못함. 잔여 결함 위치는 본 stage scope 외.

**Task F-oct-postfix-5 다음 cycle 권고 갱신** (plan Step 3 E3 의무):

> **γ_t 분기 fix 가 ALGTHM frame 0 sf0 sample 5..7 결함 해소 미입증** (got=`[+2,+2,+2]` 분기 flip 전후 무변화). 결함 진원지는 γ_t 선택 *외* 의 영역으로 추정. 다음 cycle 후보:
>
> 1. **pitch synthesis IIR memory 추적** (plan §146 의 보조 옵션과 정합) — `computeLongTermGain` g_l 영속화 + `computeTiltMu` 의 g_l-based 분기 pivot. signature 변경 없이 state 1 field 추가.
> 2. **Annex A binary 행동 추적 cycle** — ITU-T 참조 binary (Annex A C source) 의 frame 0 sf0 sample 5..7 중간 단계 (μ, γ_t, k1', s_st, s_tilt) trace 측정. plan §69 E3 phrasing 의 "Phase 1l 또는 Annex A binary 행동 추적 cycle" 정합.
> 3. **Stage F-sept/F-oct-prelim 회귀 재진단** — F-oct-prelim-5-4 §6 결정 (a) 의 전제 ("γ_t 분기 fix 가 sample 5..7 부호 결함을 해소") 가 본 cycle 로 반증됨. F-sept/F-oct-prelim 의 잔여 측정 데이터 재해석으로 결함 후보 재산출 권고.

**본 cycle 결과 정정 의무**: F-oct-prelim-5-4 §6 결정 (a) 의 spec 해석은 *분기 조건* fix 의 ALGTHM sample 5..7 영향 가설을 포함했으나, 본 cycle 측정으로 영향 = 0 입증. 차기 plan 에서 정정 사실 명시 의무.

---

## 6. commit 산출물

본 cycle 은 E3 발동으로 **production fix commit 미생성**. 본 보고서는 untracked 산출물로 남기며, Task F-oct-postfix-5 의 종합 보고 commit 시점에 함께 commit 권고.

**git status (보고서 작성 후)**:

```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-postfix-2-report.md
```

production 변경 0 라인. tilt.go = `56caa72` HEAD 동상.
