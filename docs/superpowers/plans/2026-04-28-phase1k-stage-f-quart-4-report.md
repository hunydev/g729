# Phase 1k Stage F-quart-4 종합 노트 + F-quint plan 권고

**작성일**: 2026-04-28
**범위**: F-quart-1 / F-quart-2 / F-quart-3 보고서의 모든 측정값과 spec § 인용을 통합 → 단일 fix vs 다중 fix 결정 근거 ranking 표 산출 + F-quint plan 권고서 (적용 순서, 강압-적합 회피 obligation, 사용자 결정 항목).
**산출물**: 시나리오 분류 (A / B / C 중 1) + 다중 fix 적용 순서 + F-quint plan 권고서.
**준수**: 본 보고서는 F-quart-1 / F-quart-2 / F-quart-3 보고서 (+ F-tris-2 §3.4 부산물 1건) 만 인용한다. spec § 인용은 각 선행 보고서가 단일 출처 — 본 보고서는 §/식 번호만 재참조하고 verbatim 인용은 추가하지 않는다. 외부 G.729 구현 (참조 C, bcg729, Sipro Lab, FFmpeg) **0 건 참조**.

---

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4 최종)

### 0.1 Working tree 상태 (task 시작·종료 동일)

| 경로 | 상태 | F-quart-4 변경? |
|------|------|---|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix int64 누산 보존 | No |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (untracked) — F-bis-1/F-tris-1 진단 하니스 보존 | No |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed (694e9c2 + 44eb15b) | No |
| `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-4-report.md` | **new (committed by 본 task)** | Yes (신규) |

`git diff --stat -- internal/` 출력 (task 시작·종료 양쪽):

```
internal/lsp/lsp_lp.go | 108 ++++++++++++++++++++++++++++++++++++++++++++---------------------------------------------
1 file changed, 54 insertions(+), 54 deletions(-)
```

**Production 코드 (`internal/lsp/lsp_lp.go` 의 F-bis-1 P fix 외) 0 라인 변경. 신규 test 0 건.** 본 commit 은 본 보고서 1 파일.

### 0.2 Escape hatch 종합 (선행 3 보고서 + 본 통합 task)

| 해치 | 발동 조건 | F-quart-1 | F-quart-2 | F-quart-3 | **F-quart-4 종합** |
|------|---------|-----------|-----------|-----------|------|
| **E1** | 모든 후보 spec 일치 (= 결함 0 건) | No (spec-위반 1: GainImap 누락) | No (3순위 5단계 spec 일치 — 다른 곳에 결함 존재) | No (비선형 체인 결함 2 건 확정) | **No** — spec-위반 확정 3 건 누적, 결함 ≥ 1 확정. |
| **E2** | 단일 fix 로 sample 0..7 |Δ|≤1 LSB 정렬 *전부* 회복 | Partial (7/8 sample 회복, sample 1 |Δ|=2 잔존) | n/a (측정-only) | n/a (측정-only) | **Partial** — 단일-fix (S1) 부정. |
| **E3** | 단일 fix 가 sample 0..7 정렬 *악화* | No (Branch B 39/40 > Branch A 34/40 향상) | n/a | n/a | **No** — 정렬 악화 가설 부정. |
| **E4** | 외부 G.729 구현 1 건이라도 인용 | No (PDF 단일 출처) | No (PDF 단일 출처) | No (PDF 단일 출처) | **No** — 본 보고서도 외부 구현 0 인용 (선행 3 보고서만 인용). |

**종합 결과**: E1 / E3 / E4 모두 무발동, E2 부분-발동. → F-quint plan 진입 자격 충족 + **시나리오 B (다중 fix 필수)** 으로 분류 (§2 참조).

---

## 1. F-quart-1/2/3 핵심 발견 통합표

플랜 §594-605 표를 F-quart-1/2/3 측정값으로 채운 결과:

| # | 후보 | 출처 보고서 | spec 위반? | 본 stimulus 영향? | 단독-fix 정렬 시나리오 |
|---|------|-----------|-----------|------------------|------------------|
| 1 | **§3.9.3 GainImap inverse-map 누락** (`gain/vq.go::decodeVQ`) | F-quart-1 §1.2 + §3-§4 | **✓ 확정** (§3.9.3 PDF p.22) | **✓ frame 0 sf0 Branch A 34/40 → Branch B 39/40 (+5 sample)** | **S2 부분조건** — sample 0..7 의 7/8 회복 (sample 1 |Δ|=2 잔존, 40-sample 39/40) |
| 2 | **§3.9 / §4.1.6 `ecDbQ10` int16 silent overflow** (`gain/decode.go:71`) | F-quart-3 §5.3 + §6.2 (ranking #1) | **✓ 확정** (§3.9 식 (66)–(67)) | **✓ frame 0 sf0 ec_int=2.79e8 → 86485 → int16 → 20949, +64 dB silent recovery** | (단독 측정 미수행) — 단독 시 gc 가 spec 의 약 0.193x 추정 (F-quart-3 §6.2). |
| 3 | **§3.9 Q26-vs-Q0 보정 누락** (`gain/decode.go:70-72` caller) | F-quart-3 §5.3 + §6.2 (ranking #2) | **✓ 확정** (§3.9 식 (66) + `gain/energy.go:18-22` docstring 자체 규정) | **✓ frame 0 sf0 −78.268 dB 손실, #2 와 부분 상쇄해 net +14.267 dB 오류** | (단독 측정 미수행) — 단독 시 gc 가 spec 의 1/2¹³ 추정 (F-quart-3 §6.2). |
| 4 | `pitch.AdaptiveCodebook` (§3.7 / §3.7.1 / §A.3.7) | F-quart-2 §1.4 | **✗ spec 일치** | ✗ (frame 0 sf0: tInt=20, tFrac=0, pastExc=0 ⇒ v=0 trivial) | n/a |
| 5 | `fcb.decodePositions` / `placePulses` / `applyPitchEnhancement` (§3.8 / §4.1.5) | F-quart-2 §2.4 | **✗ spec 일치** (5 단계 모두 line-by-line) | △ c[]=`[+1,+1,+1,+1,0..., +0.2,+0.2,+0.2,+0.2,...]` 정상 입력 | n/a |
| 6 | Pitch enhancement β clamp (§3.8 식 47) | F-quart-2 §3.3 | **✗ spec 일치** (clamp [0.2, 0.8] 정확) | △ frame 0 sf0 prevGpQ14=0 → β=0.2 (lower-bound) — spec-implied 동작, spec § 자체로는 일치 | 약함 (Phase 1g 비트-스트림 reference 로만 결정 가능) |
| 7 | `synth.filterSubframe` ÷2/×2 saturation recovery (§3.10/§A.3.10 ÷4/×4) | F-tris-2 §3.4 / §3.5 | **✓ 확정** (§3.10/§A.3.10 위반) | **✗ frame 0 sf0 미-trigger** (overflow 0, Pass 2 미진입) | n/a (본 stimulus 영향 없음) |

### 1.1 표 핵심 정리

- **확정 spec-위반 4 건** (#1, #2, #3, #7).
- **본 stimulus 영향 있는 spec-위반 3 건** (#1, #2, #3) — 모두 `internal/gain/` 모듈 안.
- **본 stimulus 영향 없는 spec-위반 1 건** (#7) — frame 0 sf0 미-trigger, F-quint 범위 외 (별도 스택에 보관).
- **spec 일치 + 본 stimulus 영향 약함 1 건** (#6) — fix 후보 아님, Phase 1g 측정 의무.
- **spec 일치 + 본 stimulus 영향 0 인 부정 후보 2 건** (#4, #5) — 결함 위치 부정 closure.

---

## 2. 단일/다중 fix 시나리오 분류 (A/B/C)

### 2.1 분류 결과: **시나리오 B — 다중 fix 필수**

**근거** (플랜 §609-615):

1. **시나리오 A (단일 fix 충분조건) 부정**:
   - F-quart-1 §5.3 결과 = **(S2) 부분조건** (sample 1 |Δ|=2 잔존, 40-sample 39/40).
   - GainImap fix 단독으로 sample 0..7 *전부* |Δ|≤1 + 40/40 일치를 달성하지 못함.
   - → 시나리오 A 의 충분조건 (S1 + 잔여 spec-위반 0) 불충족.

2. **시나리오 C (단일 fix 부정) 부정**:
   - F-quart-1 §0.2 / §5.3 = E3 무발동 (Branch B 39/40 > Branch A 34/40, 정렬도 *향상*).
   - GainImap fix 가 정렬을 악화시키지 않음 → 근본 재진단 cycle 불필요.

3. **시나리오 B (다중 fix 필수) 충족**:
   - F-quart-3 §6.1 (4)-(6) = `internal/gain/decode.go:70-72` 에서 **추가 spec-위반 2 건 확정** (int16 overflow + Q26 보정 누락).
   - 두 결함이 *부분 상쇄* 하여 net +14.267 dB gc 오차 → spec true gc 의 0.193x 산출.
   - F-quart-3 §6.1 (6) = "F-quart-1 의 §3.9.3 GainImap fix 만으로는 sample 1 |Δ|=2 잔존을 설명한다 — spec-fix branch 에서도 gc 는 spec 의 0.193x → synth.Filter 출력이 spec 보다 작아 근접 0 으로 수렴 → 일부 sample 만 우연히 matching".
   - → F-quart-1 의 S2 잔여 결함이 F-quart-3 의 두 결함으로 **완전 설명** (F-quart-3 §6.4).
   - → 시나리오 B 확정.

### 2.2 시나리오 B 진입 의미

- F-quint plan 은 **복합-commit fix** 또는 **순차 단일-commit fix sequence** 중 하나로 작성.
- 각 fix 적용 직후 **sample 0..7 정렬도 측정 의무** (§4.3 강압-적합 회피).
- 모든 fix 적용 후에도 잔여 |Δ|>1 LSB 발생 시 → 시나리오 C 로 reclassification 후 frame 1+ 진단 cycle (사용자 결정 항목 (c) 참조).

---

## 3. 다중 fix 적용 순서 + 각 단계 예상 정렬도

### 3.1 Dependency graph (플랜 §621-625 + F-quart-3 새 결함 통합)

플랜 §621-625 의 (1)~(5) 후보를 F-quart-3 의 두 새 결함으로 *확장* (= 본 task §1 의 #1, #2, #3, #6, #7 통합):

| 라벨 | 결함 | 카테고리 | 영향 경로 | 다른 fix 와의 의존성 |
|------|------|---------|---------|------------------|
| **(1)** | GainImap inverse-map 누락 | gain.decodeVQ (= 플랜 §621 의 (1)) | idx → g_p / γ̂_c → gain.Decode → gc → u → s | (2)/(3) 와 *coupled* — γ̂_c 가 (4) gain.Decode 비선형 체인의 입력. (6) 와 *독립*. |
| **(4a)** | `ecDbQ10` int16 silent overflow | gain.Decode 비선형 체인 (= 플랜 §624 의 (4) 안) | ecLog2Q10 → ecDbQ10 → ecBarDbQ10 → predictedLogGain → log2GcQ10 → pow2 → gc0Q14 → gc | (4b) 와 *반드시 동시 적용* (단독 fix 시 ÷64 dB 큰 편차 도입, F-quart-3 §6.3). (1) 과 *coupled* (양 분기 모두 영향). |
| **(4b)** | Q26-vs-Q0 보정 누락 | gain.Decode 비선형 체인 (= 플랜 §624 의 (4) 안) | 동상 (4a) | (4a) 와 *반드시 동시 적용* (단독 fix 시 ×8192 큰 편차). |
| (2) | pitch enhancement β clamp | (플랜 §622) — F-quart-2 §3.3 결과 spec 일치 | c → u → s | **fix 대상 아님** (spec 일치 closure). |
| (3) | fcb.Decode 위치/sign | (플랜 §623) — F-quart-2 §2.4 결과 spec 일치 | c | **fix 대상 아님** (spec 일치 closure). |
| (5) | filterSubframe ÷2/×2 (= F-tris-2 #7) | (플랜 §625) | s | **F-quint 범위 외** (frame 0 sf0 미-trigger). 별도 stage 에서 처리 (잔여 백로그). |
| (6) | β init = 0.2 (lower-bound) | F-quart-2 §3.3 약함 후보 | c | **fix 대상 아님** (spec § 자체로는 일치). Phase 1g 측정 의무. |

**플랜 §627 의 순서 (3) → (2) → (1) → (4)** 의 적용:

- (3), (2) = F-quart-2 spec 일치 closure → fix 대상 아님 → 적용 순서에서 제외.
- 남는 fix 는 **(1) GainImap** + **(4a)+(4b) gain.Decode 비선형 체인 두 결함**.

### 3.2 단일 vs 분리 fix 권고 — (4a)+(4b)

F-quart-3 §6.3 명시:

> 순위 (1)+(2) 는 동시에 fix 해야 한다 — 각각 단독 fix 시 ÷64 dB 또는 ×8192 의 큰 편차 도입.

(여기서 F-quart-3 의 "(1)+(2)" 는 본 보고서 라벨 **(4a)+(4b)** 와 동일.)

**권고**: **(4a) 와 (4b) 는 단일 commit 으로 묶어서 적용**. 이유:
- 두 결함이 *상쇄* 관계 → 한 쪽만 fix 시 net 오차가 *증가* (악화 방향) → E3 발동 위험.
- 같은 caller (`internal/gain/decode.go:70-72`) 안의 인접 라인 → diff 응집도 높음.
- F-quart-3 §6.3 가 *동시 fix 후* 의 saturate 재검증 의무를 명시 → 두 결함의 상호작용을 단일 단위로 검증해야 함.

**권고**: **(1) 은 (4a)+(4b) 와 *별도 commit* 으로 적용** (= 분리 fix). 이유:
- (1) 은 *데이터 매핑 정책* 결함 (`decodeVQ` 의 indexing), (4a)+(4b) 는 *Q-format 산술* 결함 → 코드 변경 위치 다름 (`gain/vq.go` vs `gain/decode.go`).
- 분리 시 각 commit 단위의 정렬도 변화를 독립 측정 가능 → 강압-적합 회피 obligation 충족 용이.
- F-quart-3 §6.3 명시: "순위 (3) [= 본 보고서 (1)] 의 GainImap fix 는 (1)+(2) 와 직교 — 별도 fix."

### 3.3 적용 순서 + 각 단계 예상 정렬도

| 단계 | 적용 fix | 누적 commit | sample 0..7 예상 정렬도 (frame 0 sf0) | 근거 |
|------|---------|-----------|----------------------------------|------|
| **0 (baseline)** | none (현재 working tree) | — | sample 0..7: `[2 2 3 3 2 1 0 1]`, vs PST/2 `[1 2 1 1 0 -1 -1 -1]`, 40-sample 34/40 (F-quart-1 §3 hpFilter 행) | F-quart-1 측정 |
| **1** | **(4a)+(4b) 동시 fix** (gain/decode.go ec 체인) | C1 (+1 commit) | (단독 측정 미수행 — 측정 의무 §4.3) — gc 가 spec true 값으로 회복하지만 indexing 은 여전히 GA1=5/GB1=6 → γ̂_c entry 가 spec 과 다름 → 부분 향상만 예상 | F-quart-3 §5.1 (Branch P 의 ref gc_q12=32767 saturating) — 큰 gc 가 큰 합성 출력 ⇒ |Δ| 증가 가능 → **E3 risk 1순위 단계** |
| **2** | **(1) GainImap fix** (gain/vq.go) + docstring 수정 | C2 (+1 commit) | F-quart-3 §5.2 (Branch S ref gc_q12=4151) — spec-correct 0.193x 의 *반대* (gc 가 spec true 값과 일치) → sample 0..7 정렬도 *대폭 향상* 예상, **목표 = 8/8 |Δ|≤1 LSB + 40/40 matches** | F-quart-3 §5.2 + §6.4 (S2 잔여 결함 완전 설명 가설 검증) |
| **3 (검증)** | (no-op, 회귀 게이트 + 정렬 측정 only) | C2 동일 | TestDecode_Frame0Sample0_MatchesALGTHM 결과 = PASS 기대 (got=2 want=2) | 본 task 작성 시점 baseline FAIL got=4 want=2 의 회복 측정 |

**중요**: 위 순서는 플랜 §627 의 "(3) → (2) → (1) → (4)" 와 다르다. 이유:
- 플랜 §627 의 순서는 *5 후보 모두 fix 대상* 이라는 가정 하에 작성됨.
- F-quart-2 가 (2)/(3) 을 spec 일치로 closure → fix 대상에서 제외.
- 남은 (1) 과 (4) 의 순서는 F-quart-3 §6.3 가 *(4) → (1) 분리* 로 명시 (= 본 표의 "단계 1 → 단계 2").
- 또한 (4) 를 먼저 적용하면 Branch P (production indexing) 의 gc 가 ref 값과 일치 → 정렬도 측정의 *baseline 신뢰도* 가 높아짐. 이후 (1) 적용 시 Branch S (spec indexing) 의 영향만 분리 측정 가능.

**대안 순서 (1) → (4)** 도 수학적으로 동일한 최종 정렬도 도출 가능. 단 단계 1 (= (1) 단독) 은 F-quart-1 §5 가 이미 측정 (S2 = 7/8 회복) → 정보 이득 적음. 권고는 **(4) → (1)** 순서.

---

## 4. F-quint plan 권고서

### 4.1 사이클 입구 invariant

F-quint plan 진입 시 working tree 가 다음 상태여야 한다:

| 항목 | 요구 상태 | 검증 명령 |
|------|---------|---------|
| `internal/lsp/lsp_lp.go` | F-bis-1 P fix int64 누산만 변경 (uncommitted 또는 commit 1, 본 fix 그대로 보존) | `git diff -- internal/lsp/lsp_lp.go` (54 +/-) |
| `internal/decoder/stagef_bis_diagnostic_test.go` | untracked 보존 (F-bis/F-tris 진단 baseline) | `git status --porcelain internal/decoder/` |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed (694e9c2 + 44eb15b) 보존 | `git log --oneline internal/decoder/stagef_quart_diagnostic_test.go` |
| `internal/gain/decode.go`, `internal/gain/vq.go` | F-quart-4 시점 그대로 (변경 0) | `git diff -- internal/gain/` |
| F-quint commit C1 직전 회귀 게이트 PASS | Stage D 17 contract test + Stage D-bis 3 contract test + Phase 1i sample 0 가드 | `go test ./internal/...` (전체 PASS, 단 `TestDecode_Frame0Sample0_MatchesALGTHM` 만 baseline FAIL got=4 want=2 허용) |

회귀 게이트 명세 요약 (F-quint 사이클 *각 commit 직후* 재검증 의무):

- **Stage D 17 contract test**: `internal/synth/`, `internal/postfilter/`, `internal/pcm/`, `internal/gain/`, `internal/fcb/`, `internal/pitch/`, `internal/lsp/`, `internal/decoder/` 의 contract spec 일치 test (Stage D 도입). F-quint 의 어떤 fix 도 이 17 개를 회귀시키지 않아야 한다.
- **Stage D-bis 3 contract test**: F-bis-1 P fix 검증 + LSP 합성 cross-check + 추가 contract (Stage D-bis 도입). 본 cycle 의 lsp_lp.go P fix 가 commit 으로 승격되면 D-bis test 가 새 baseline 이 된다.
- **Phase 1i sample 0 가드**: `TestDecode_Frame0Sample0_MatchesALGTHM` (현재 FAIL got=4 want=2). F-quint 단계 2 (= GainImap fix 후) 에서 PASS 회복 기대. 회복 미달 시 시나리오 B → C 로 reclassification.

### 4.2 Task ranking + 사용자 결정 요청 항목

**Task ranking** (본 stimulus 영향 있는 fix 만):

| 순위 | Task | 변경 위치 | commit 단위 | 기대 효과 |
|------|------|---------|-----------|---------|
| **1** | (4a)+(4b) 동시 fix — `ecDbQ10` 의 int16 overflow 회피 (int32 보존) + `ecLog2Q10` 에 `-26*1024` Q26 보정 적용 | `internal/gain/decode.go:70-72` (caller side) + `internal/gain/energy.go` 의 docstring 검증 | **단일 commit C1** | 비선형 체인의 net +14.267 dB 오차 제거; gc 가 spec true 값과 일치. 단 indexing 미정정 → sample 0..7 부분 향상만 예상 (E3 risk 1순위). |
| **2** | (1) GainImap inverse-map fix — `decodeVQ` 가 `GBK1[GainImap1[GA]]` / `GBK2[GainImap2[GB]]` 사용 + `vq.go:14-17` docstring 의 §3.9.3 모순 문구 수정 | `internal/gain/vq.go` + 동 docstring | **단일 commit C2** | F-quart-3 §6.4 가설 검증 — sample 0..7 의 8/8 |Δ|≤1 LSB + 40/40 matches 회복 기대. `TestDecode_Frame0Sample0_MatchesALGTHM` PASS 기대. |
| (보류) | (5) `filterSubframe` ÷2/×2 → ÷4/×4 (§3.10/§A.3.10 정합) | `internal/synth/filter.go:31-52` | **F-quint 범위 외** (frame 0 sf0 미-trigger) | 별도 stage 에서 처리. 본 stimulus 영향 0. |
| (불요) | (2)/(3) — pitch / fcb 단계 fix | n/a | n/a | F-quart-2 §4.1 spec 일치 closure → fix 대상 아님. |
| (보류) | (6) β init 변경 | n/a | n/a | spec § 자체로는 일치 → Phase 1g 비트-스트림 reference 측정 후에만 결정. |

**권장 commit 정책**: **2 개의 분리 commit (C1 + C2)** + 각 commit 직후 정렬도 측정 + 회귀 게이트 통과 확인.

복합 commit (C1+C2 단일) 비-권고 이유:
- 두 결함은 코드 변경 위치 다름 (`decode.go` vs `vq.go`).
- 분리 시 각 commit 의 정렬도 영향이 *독립* 측정 가능 → 강압-적합 (forced-fit) 가설 검증 용이.
- C1 단독으로 E3 발동 시 (= 정렬 악화) 즉시 revert 후 fix 재설계 가능. 복합 commit 은 어느 쪽 결함 책임인지 분리 불가.

**사용자 결정 요청 항목** (플랜 §652-657):

- **(a) F-quint plan 진입 (시나리오 B)** ← **본 보고서 권고**.
  - 의미: 위 Task 1 (C1) → Task 2 (C2) 순으로 단일-commit fix sequence 진행.
  - 각 commit 직후 sample 0..7 정렬도 측정 + Stage D/D-bis contract 회귀 0 확인.
  - C2 후 `TestDecode_Frame0Sample0_MatchesALGTHM` PASS 미달 시 시나리오 C 로 reclassification.

- **(b) Stage F 부정 + 외부 정합 모델 진단 cycle 진입 (E1 발동 시)** — **본 case 미해당** (E1 무발동, spec-위반 ≥ 1 확정).

- **(c) 본 사이클 일시 정지 + frame 1+ 진단 cycle 우선** — **본 case 미해당** (시나리오 C 가설 미진입, 본 stimulus 한정 결함 가설 = (1)+(4a)+(4b) 가 frame 0 sf0 잔존을 *완전 설명* 하는 가설로 강력 지지됨).

### 4.3 강압-적합 (forced-fit) 회피 obligation

F-quint 사이클의 각 commit 직후 *반드시* 다음을 수행한다:

1. **정렬도 측정** (각 fix 직후, 본 task §3.3 표의 예상 정렬도와 비교):
   - `go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v` (F-quart-1 진단 하니스 재실행 → Branch A/B sample 0..7 표 갱신).
   - `go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v` (F-quart-3 cross-check 재실행 → prod = ref 일치 확인).
   - `go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v` (Phase 1i sample 0 가드).

2. **spec § 재인용** (각 fix 의 commit message 본문에):
   - C1: §3.9 식 (66)–(67) + `internal/gain/energy.go:18-22` docstring (Q26 보정 명시) verbatim 재인용.
   - C2: §3.9.3 verbatim 재인용 ("the GA and GB indices are reordered before transmission") + `internal/tables/gain_gbk1.go:36-44` 의 GainImap1 docstring (디코더 의무 명시) 재인용.

3. **악화 시 즉시 revert**:
   - C1 직후 sample 0..7 의 임의 sample 의 |Δ| 가 *증가* 하거나 40-sample matches 가 *감소* → C1 즉시 `git revert` + 재진단.
   - C2 직후 동일 측정 → 악화 시 C2 즉시 revert + 재진단.
   - 회귀 게이트 (Stage D 17 + D-bis 3 + Phase 1i 가드) 의 임의 test FAIL → 즉시 revert.

4. **외부 G.729 구현 0 인용 유지**: F-quint 사이클의 모든 commit message / 보고서 / 코드 주석에서 외부 구현 (참조 C, bcg729, Sipro, FFmpeg) 인용 0 건 (E4 무발동 유지).

5. **production 0-수정 약속의 단계적 해제**: F-quint 는 본 task 와 달리 production fix cycle → `internal/gain/decode.go` + `internal/gain/vq.go` 의 *최소* diff 만 허용. F-bis-1 P fix (lsp_lp.go) 와 별개로, 본 cycle 에서 추가 production 변경은 위 2 commit 만.

---

## 5. 미해결 항목 (F-quint 범위 외)

본 task 의 종합 분석으로도 결정 불가한 항목:

1. **frame 1+ 잔여 결함 가능성**: 본 cycle 의 모든 측정은 frame 0 sf0 (한정). frame 1+ 에서는 `pastSynth` / `pastExc` / `pastErrors` FIFO 가 비-zero → 다른 spec-위반 후보가 활성화될 수 있다 (예: F-quart-2 §3.3 약함 후보 (6) β init 의 영향). → F-quint 후속 cycle (예: F-sext) 에서 frame 1+ 진단 하니스 추가.

2. **codec state-machine 외부 결함**: `decoder.Decoder.hpFilter` (§4.2.2) / `pcm.ScaleUpSat` (PST 도메인 ×2) 의 spec § line-by-line 검증은 F-quart-1 §6.3 가 권고했으나 본 cycle 에서 실측 미수행. F-quint 후 sample 1 잔존 시 후속 진단.

3. **ALGTHM 자체 오류 가능성**: `docs/superpowers/specs/itu/ALGTHM.PST` 자체의 frame 0 sample 0..7 reference value 정확성. 외부 G.729 구현 0 인용 정책 하에서는 검증 불가. 시나리오 C reclassification 시에만 ITU 비트-스트림 cross-validation 도입 검토 (E4 발동 = 정책 변경 = 사용자 결정 항목).

4. **(5) `filterSubframe` ÷2/×2 vs §3.10/§A.3.10 ÷4/×4**: 본 stimulus 미-trigger. 별도 stage (예: F-sext-1) 에서 *trigger 가능한 stimulus* 을 인공적으로 구성하여 측정 후 fix.

5. **(6) β init 컨벤션**: spec § 자체로는 일치. Phase 1g 비트-스트림 reference 측정 후에만 결정.

---

## 6. 결론

- **시나리오**: **B (다중 fix 필수)**.
- **fix 후보 (본 stimulus 영향)**: **3 건** — (1) GainImap inverse-map, (4a) int16 overflow, (4b) Q26 보정 누락.
- **commit 정책**: **2 commit** — C1 = (4a)+(4b) 동시 (단일 caller, 상쇄 결함 동시 fix 의무), C2 = (1) (분리, 직교 변경).
- **적용 순서**: **C1 → C2** (F-quart-3 §6.3 의 (4) → (1) 권고 + 단계 1 baseline 신뢰도 향상 근거).
- **각 commit 직후 의무**: 정렬도 측정 + spec § 재인용 + 회귀 게이트 (Stage D 17 + D-bis 3 + Phase 1i 가드) 통과 + 악화 시 즉시 revert.
- **사용자 결정 요청**: **(a) F-quint plan 진입** 권고 ((b)/(c) 미해당).
- **F-quint 후 잔존 결함 처리**: 시나리오 C reclassification → frame 1+ 진단 cycle 또는 ALGTHM cross-validation 정책 변경 검토.

→ **F-quint plan 작성 권고**. F-quint 는 본 보고서 §3.3 의 단계 1 (C1) + 단계 2 (C2) 를 두 task 로 분할하여 진단-only 가 아닌 *production fix cycle* 로 진행한다.
