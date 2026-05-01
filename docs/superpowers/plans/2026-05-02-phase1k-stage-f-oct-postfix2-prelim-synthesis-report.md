# Phase 1k Stage F-oct-postfix2-prelim 종합 보고서 + 다음 cycle 결정

**작성일**: 2026-05-02
**범위**: F-oct-postfix2-prelim-1/2/3/4 결합 분석 + 4 가설 (M5/M6/M1'/M3) 비교 + 결함 위치 후보 (X/Y/Z/W) 평가 + 다음 cycle 단일 결정.
**산출물**: cycle 결산 + 4 가설 비교표 + 결함 위치 후보 평가표 + 다음 cycle plan outline + 사용자 게이트 + Phase 1k 종결 평가.
**준수**: Phase 0.4 강압-적합 회피 (측정 데이터만), 사용자 G1 결정 (Annex A binary 거부, 외부 G.729 0 참조), production 변경 0 라인 (메타 task), 사전 보유 working tree 보존.
**근거 문서**: ITU-T G.729 (06/2012) PDF + READMETV.txt + 본 cycle commit (`ff5534a`, `6dc851e`, `cb9529d`, `f04ec88`).

---

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)

### 0.1 Working tree pre/post

본 task = 메타 task (보고서 only). pre / post 모두:

```
?? internal/decoder/stagef_bis_diagnostic_test.go
```

본 commit 추가 파일 = 본 보고서 1 건 (`docs/superpowers/plans/2026-05-02-phase1k-stage-f-oct-postfix2-prelim-synthesis-report.md`). production 변경 = 0 라인. test 변경 = 0 라인. 사전 보유 untracked `internal/decoder/stagef_bis_diagnostic_test.go` 는 본 cycle 5 task 어떤 commit 도 add 하지 않음 (Phase 0.5 의무 충족).

### 0.2 Escape hatch 평가 (E1–E5)

| Hatch | 평가 | 근거 |
|-------|------|------|
| **E1** (회귀 게이트 1+ FAIL) | 미발동 | 본 commit 직후 게이트 1~14 PASS, 게이트 15 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) RED 잔존 (다음 fix cycle GREEN gate). 신규 회귀 0건. `go vet ./...` clean. |
| **E2** (spec § 인용 ↔ PDF grep 불일치) | 미발동 | 본 보고서의 인용은 plan §"Spec § 인용" 의 PDF verbatim grep + Task 2/3/4 보고서의 인용 재사용. 신규 PDF 인용 0. |
| **E3** (4 가설 중 2+ 잔존) | **미발동** (대신 0 식별 trigger) | 본 cycle 4 가설 모두 반증 (§2 비교표). plan §Step 3 의 시나리오 = "4 가설 모두 반증" → spec 영역 확장 진단 cycle 권고 (단일 fix cycle 진입 금지). |
| **E4** (외부 G.729 인용/대조) | 미발동 | 본 cycle 4 task + 본 종합 모두 ITU-T G.729 (06/2012) PDF + READMETV.txt + repo 내 PST 파일만 사용. Annex A binary 실행 / black-box trace / 외부 reference 0건. 사용자 G1 결정 ("(c) Annex A binary 거부 + 후보 ③ pivot") 정합. |
| **E5** (production 변경 > 0) | 미발동 | 본 commit 변경 = 보고서 1 파일 only. production 변경 = 0 라인. test 변경 = 0 라인. |

### 0.3 강압-적합 회피 의무 (Phase 0.4) 정합성

- **§1 가설별 측정 분리**: 4 가설 평가는 §2 비교표의 측정 출처 칼럼이 모두 commit hash 로 trace — 측정 데이터만 사용, 직관적 추론 0.
- **§2 spec § 결합 해석 금지**: 본 보고서의 후보 X/Y/Z/W spec grounding (§3) 은 PDF verbatim 인용 출처만 명시 — 결합 해석 금지.
- **§3 음성 결과 (모두 반증) 인정**: 본 cycle 4 가설 모두 반증 = *유효한 측정 결과*. plan §Step 3 결정 트리에 따라 spec 영역 확장 진단 cycle 권고 (§4).
- **§5 g_l 영속화 후보 ① 제외**: 본 보고서는 후보 ① (g_l state 영속화) 와 관련된 fix scope 0 — Task 4 §4 의 "longterm `compute` 분기 spec 정합" 측정 결과로 갈음.
- **§6 G3 반증 정정**: F-oct-prelim-5-4 §3.6 row C3 의 M1 단독 채택 결정이 본 cycle 진입 premise 로 반증되었음을 §1 에 명시 (별도 정정 commit 불요 — synthesis report §5 G4 = (c) "본 plan 자체로 갈음").

### 0.4 사용자 G1 결정 정합성

사용자 G1 결정 = "(c) Annex A binary 거부 + 후보 ③ (M1 외 후보 재진입) pivot". 본 cycle 은 후보 ③ 의 4 가설 (M1'/M3/M5/M6) 분리 측정 — 정합. 4 가설 모두 반증 결과는 후보 ③ 자체의 *spec scope 한계* 를 지시 (4 가설이 모두 postfilter chain + PST/synth 의 직접 측정 가능 영역에 한정되었으므로). §3 후보 X/Y/Z/W 는 후보 ③ 의 spec scope 외 (excitation 입력 부호 자체 / LP 변환 / spec 해석 / PST 출처 의문 재발) 로 확장 — G1 결정의 필연적 후속.

---

## 1. F-oct-postfix2-prelim cycle commit 요약 + cycle premise

### 1.1 본 cycle commit hash 누적

```
<this commit> docs(plans): F-oct-postfix2-prelim synthesis + cycle decision
f04ec88       test(postfilter,synth): add Stage F-oct-postfix2-prelim-4 M1' + M3 measurement
cb9529d       test(decoder): add Stage F-oct-postfix2-prelim-3 M6 PST want sign verification
6dc851e       test(decoder): add Stage F-oct-postfix2-prelim-2 M5 excitation sign trace
ff5534a       test(decoder): add Stage F-oct-postfix2-prelim-1 chain dump baseline
118446e       docs(plans): add Phase 1k Stage F-oct-postfix2-prelim plan
8907847       docs(plans): F-oct-postfix synthesis + cycle decision (E3)
```

### 1.2 직전 cycle premise (G3 반증 후 4 가설 재산출)

F-oct-postfix synthesis (`8907847`) §2.3-§2.4 누적:

- F-oct-postfix-2 revert 측정에서 γ_t 분기 fix (k1 ≥ 0 strict reading) 적용 후 sample 5..7 출력 Δ=0 → **G3 (γ_t 분기 단독 결함) 반증**.
- F-oct-prelim-5-4 §3.6 의 "M1 단독 채택" 결정의 *암묵적 가정* (γ_t 분기 fix 가 sample 5..7 부호 결함의 충분 조건) 도 동시 반증.
- 후보군 갱신 = M1' (postfilter 외 분기) / M3 (synth IIR 재진입) / M5 (excitation 부호) / M6 (PST want 부호) 4 가설.

### 1.3 본 cycle 측정 정량 누적

| 측정 단계 (sample 5..7 한정) | 값 | 부호 | 출처 |
|------------------------------|----|----|------|
| excitation u[5..7] | [+0, +0, +0] | [0 0 0] | Task 2 (`6dc851e`) |
| excitation u[0..4] | [+1, +1, +1, +1, +0] | [+ + + + 0] | Task 4 (`f04ec88`) §3 |
| pastSynth (codec-start) | [0;10] | — | Task 4 §3 (§4.3 Table 9 invariant) |
| synth IIR syn[5..7] | [+1, +1, +1] | [+ + +] | Task 1 / 2 / 4 cross-check |
| synth IIR syn[0..4] | [+1, +2, +2, +2, +1] | [+ + + + +] | Task 4 §3 |
| postfilter sPf[5..7] (longterm + shortterm + tilt + AGC) | [+1, +1, +1] | [+ + +] | Task 4 §2 |
| post-hpfilter out[5..7] (PST 도메인 ×2 scale 전) | [+1, +1, +1] | [+ + +] | F-sext cross-ref |
| post-hpfilter out[5..7] (PST 도메인) | [+2, +2, +2] | [+ + +] | Task 1 (`ff5534a`) baseline |
| PST want sample 5..7 | [-1, -1, -1] | [− − −] | Task 3 (`cb9529d`) byte-level |
| Δ = got − want | [+3, +3, +3] | sample-uniform | Task 1 baseline |

### 1.4 핵심 측정 발견 (cycle 누적)

1. **부호 *생성* 단계 = synth IIR (excitation→syn)**: u[5..7]=0 → syn[5..7]=+1 (Task 2 §3). 출처 = u[0..4]=[+1,+1,+1,+1,+0] 양 입력의 자기-피드백 (Task 4 §5 confirmed).
2. **synth IIR 선형 invariant**: `syn(-u) == -syn(+u)` for sample 0..7 (Pass-1, fixed.Overflow=false). 즉 syn[5..7] 양수 = u[0..4] 양수 입력의 *spec-거동* (§3.10 IIR linear) — IIR 자체 결함 0 (Task 4 §5).
3. **postfilter 6 stage 부호 보존**: input s[5..7] = [+ + +] → longterm rOut → shortterm sSt → tilt sTilt → AGC sPf 모두 [+ + +] (Task 4 §2). 모든 분기 = codec-start 의도 경로 (longterm `compute`, tilt `inactive`, AGC `init-seed`) — cover 결손 0.
4. **PST want byte-level 정합**: ALGTHM.PST byte 10..15 = `ff ff ff ff ff ff` = int16 LE [-1, -1, -1] (Task 3 §2). 9 vector 분포에서 `[-,-,-]` 3 vector + `[0,0,0]` 5 vector + `[-,0,0]` 1 vector + `[+,+,+]` *0 vector* (Task 3 §3). production 출력 `[+2,+2,+2]` 와 정합하는 PST 분포 0 — PST 데이터 결함 0.
5. **Δ=+3 sample-uniform**: sample 5/6/7 모두 Δ 동일 → 결함 항이 sample-별 변동이 아니라 *전역 상수 (gain / sign flip / 입력 부호)* (Task 1 §3).

---

## 2. 4 가설 비교표 (단일 표, 측정 데이터만)

plan §Step 2 의 4 가설 비교표 (Phase 0.4 §1 — 측정 데이터만):

| 가설 | 측정 출처 (commit) | 핵심 결과 | 평가 | spec § 인용 |
|------|--------------------|----------|------|--------------|
| **M5** (excitation pre-postfilter 부호) | Task 2 §2-§3 (`6dc851e`) | u[5..7] = [+0, +0, +0] (zero excitation). syn[5..7]=+1 의 부호 *생성* 단계 = synth IIR (excitation 외부). plan §Step 4 표 의 "excitation 부호 = `[+,+,+]` (PST want 와 반전) → M5 유력" 조건 미해당. | **REFUTED** | §A.4.1 (= §4.1.5 gain decoding), §A.4.2 cascade |
| **M6** (PST want 데이터 부호) | Task 3 §2-§4 (`cb9529d`) | ALGTHM.PST byte 10..15 = `ff ff ff ff ff ff` → int16 LE = [-1,-1,-1] 정합. 9 vector 분포 `[+,+,+]` = 0 — production `[+2,+2,+2]` 와 정합 vector 0건. P-SRC-2 (Annex A vs main BYTE-EQUAL) sample 5..7 영역 연장. | **REFUTED** | READMETV.txt verbatim |
| **M1'** (postfilter 외 분기 cover 결손) | Task 4 §2 + §4 (`f04ec88`) | postfilter 6 stage sign-chain 모두 [+ + +] (input s → residual r → longterm rOut → shortterm sSt → tilt sTilt → AGC sPf). 모든 분기 (longterm `compute` R=20>0 E=22≠0, tilt `inactive` γ_t=3277 codec-start, AGC `init-seed` initialized=false) = 의도 경로. cover 결손 0. replicated chain == production Postfilter.Filter ✓. | **REFUTED** | §A.4.2.1 (Hp), §A.4.2.2 (Hf), §A.4.2.3 (Ht), §A.4.2.4 (AGC) |
| **M3** (synth IIR memory propagation) | Task 4 §3 + §5 (`f04ec88`) | (a) pastSynth codec-start = [0;10] (§4.3 Table 9). (b) memory pre-5 [1 2 2 2 1 0 0 0 0 0] → post-7 [1 1 1 1 2 2 2 1 0 0] sign change 0/10 (모두 비음수). (c) `syn(-u) == -syn(+u)` for sample 0..7 (Pass-1 linear invariant). syn[5..7]=+1 = u[0..4]=[+1,+1,+1,+1,+0] 자기-피드백의 *spec-거동*. | **REFUTED** | §A.4.1, §3.10 IIR linear |

**4 가설 단일 식별 결과 = 0 식별 (전부 반증).** plan §Step 3 결정 트리의 시나리오 "4 가설 모두 반증" → spec 영역 확장 진단 cycle 권고 (단일 fix cycle 진입 금지).

본 0 식별 결과는 *유효한 측정 결과* (Phase 0.4 §3) — 가장 그럴듯한 가설을 임의 채택하지 *않는다*. 다음 cycle 의 결함 위치 후보는 §3 의 X/Y/Z/W 4 후보 평가표로 분리 진단.

---

## 3. 결함 위치 후보 (X/Y/Z/W) 비교 평가표

본 cycle 4 가설 측정으로 chain 내부 (excitation u[5..7] / synth IIR / longterm / shortterm / tilt / AGC / hpfilter / PST want) 의 결함이 모두 반증. 결함 위치는 chain *외부* (또는 spec 해석 자체) 의 4 후보로 분리:

| 후보 | 정의 | 측정 baseline | priority | risk | spec grounding | cost | 비고 |
|------|------|---------------|----------|------|----------------|------|------|
| **X** | excitation u[0..4] 의 *부호* 자체 (현재 [+1,+1,+1,+1,+0] — 양 입력으로 syn 양수 자기-피드백 유발) | Task 4 §3 (u[0..4]=[+1,+1,+1,+1,+0]). 만약 spec want = u[0..4] 음수 → syn[5..7] 음수 → PST want [-1,-1,-1] 정합. | **HIGH** | MEDIUM (excitation 생성 path = §A.3.* / §4.1.5 — fix scope 가 fcb / pitch / gain 분기까지 확장 가능, 회귀 위험 존재) | §A.3.* (Excitation reconstruction), §4.1.5 (Decoding of the adaptive and fixed-codebook gains) — PDF verbatim 인용 가능 | MEDIUM (single subframe excitation 측정 + spec verbatim 추가 필요) | Task 2 가 excitation u[5..7]=0 만 측정. u[0..4] 의 *spec want* (다른 vector 의 PST 부호 → 역추론) 측정이 다음 cycle 필수 |
| **Y** | LP a[] 계수 변환 (synth IIR 입력측 LP 계수 — §A.3.* / §A.4.1) | Task 4 §3 (a[0..10] = [4096, -2197, -375, -4, -144, -68, 303, -36, -90, 145, -33] Q12). 만약 a[] 계수의 부호 또는 변환이 spec 와 어긋나면 동일 u[] 입력에서 syn 부호 flip 가능. | MEDIUM | LOW-MEDIUM (LP 계수 path = §A.3.2/3 LSP→a[] 변환 — Phase 1 누적 LSP test 다수 cover, 회귀 risk 제한) | §A.3.2 (LSP decoding), §A.3.3 (LSP→a[] conversion), §A.4.1 | MEDIUM (LP a[] 변환 reference cross-check 측정 추가, F-sept-2 LPReferenceCrossCheck 와 cross-check) | F-sept-2 (`TestDiagnostic_FseptLPReferenceCrossCheck`) PASS 가 baseline — 동일 frame 0 sf0 의 a[] 가 F-sept reference 와 정합한다면 우선순위 하향 가능 |
| **Z** | spec 해석 자체 — postfilter chain 의 "정합" 정의 재검토 (예: PST want 의 *비교 도메인* 해석, 또는 sample 5..7 의 frame-edge 정의) | Task 1 §2 baseline (chain dump out[5..7]=[+2,+2,+2], post-hpfilter PST 도메인 ×2 scale 후). PST want 도메인은 동일 PST 도메인 (Task 3 byte-level 검증). 도메인 불일치 0. 그러나 *비교 단위* 자체 (예: PST want 가 다른 sub-band / down-sample / time-shift 표현?) 가 미검증. | LOW-MEDIUM | LOW (spec 해석 측정은 production 0 변경 + spec PDF 추가 인용만) | §A.4.2 cascade + §4.2 (post-processing parent), §4.3 Table 9 (state init) | LOW (spec PDF re-grep + frame-edge 정의 인용 재발췌만) | F-oct-prelim-1/2 가 PST format / frame 정렬을 cover. *재검토* 의 형식: PDF §4.2 / §A.4.2 의 "input/output sample alignment" 절을 verbatim 추가 인용 |
| **W** | PST 출처 자체 의문 재발 (M6 반증과 모순 — re-investigate) | Task 3 §2-§3 (M6 REFUTED, byte-level + 9 vector 분포). M6 반증 데이터와 W 후보는 *명시적 모순* — Phase 0.4 §6 강압-적합 회피 의무 (W 강압적 dismiss 금지) 와 동시에 W 강압적 채택 금지. | **LOW** (M6 반증과 직접 모순) | HIGH (M6 반증 데이터 폐기 cost + spec invariant 재정의 risk) | READMETV.txt verbatim, ITU 원본 vector 자체 의문 시 ITU-T G.729 (06/2012) Annex A test vector 명세 (PDF 별첨) 추가 인용 필요 | HIGH (M6 cycle 자체 재실행 + ITU 외부 별첨 verbatim grep + cross-tree byte 비교 재실행) | **명시적 모순 분석**: Task 3 §2 의 ALGTHM.PST byte 10..15 = `ff ff ff ff ff ff` 는 little-endian int16 = [-1,-1,-1] 의 *유일 해석* (sign-extension/endianness 결함 0). 9 vector 분포에서 `[+,+,+]` = 0 vector 는 *production 출력 `[+2,+2,+2]` 와 정합 PST 가 부재* 를 입증. W 채택 시 이 두 측정을 *모두 무효* 처리해야 하나, 두 측정은 byte-level + cross-vector 로 독립 검증 — W 강압적 채택의 *측정 정당화 0*. → W 우선순위 = LOW (재투자 비용 대비 식별 확률 최저) |

### 3.1 우선순위 결정 근거 (측정 기반 only)

- **X (HIGH)**: 측정 발견 §1.4 (1) "syn[5..7]=+1 의 출처 = u[0..4]=[+1,+1,+1,+1,+0] 자기-피드백" 이 *직접* 지시. spec want want = [-1,-1,-1] 와 부호 정합하려면 u[0..4] 또는 그 이전 부호 결정 단계 (gain, fcb, pitch) 가 음수여야 함. 다른 vector (PITCH/FIXED PST 도 `[-,-,-]`) 의 *spec want* 분포 = [−,−,−] 3 vector 가 가장 다수 — excitation 부호 결함이 vector 다수에 일관 나타나야 spec-정합. **본 cycle 에서 측정 *부재* 한 영역** (Task 2 는 sample 5..7 한정 — sample 0..4 는 미측정) — 다음 cycle 의 직접 측정 대상.
- **Y (MEDIUM)**: 측정 baseline (a[0..10] dump) 는 있으나 *spec reference 와의 cross-check* 가 sample 5..7 한정 미수행. F-sept-2 LP cross-check 가 frame 0 전반 PASS — sample 5..7 영역의 a[] 변환 정합성을 추가 측정해야 우선순위 확정. X 와 결합 시 hybrid 가능성 (X 가 직접 부호 결정, Y 가 a[] 부호 보조).
- **Z (LOW-MEDIUM)**: 측정 비용이 가장 낮으나 (PDF re-grep only), 결함 식별 확률은 sample-uniform Δ=+3 패턴이 *frame-edge* 또는 *비교 단위* 로 설명 가능한지 spec verbatim 검토 필요. F-oct-prelim-1/2 의 frame alignment PASS 가 baseline — Z 단독 식별 확률 LOW.
- **W (LOW)**: §3 표 비고 칸 명시 — M6 반증 데이터와 명시적 모순. Phase 0.4 §6 의무로 *dismiss 도 채택도 금지* — 다음 cycle 에서 W 진입 시 M6 cycle 재실행 (Task 3 byte-level + 9 vector 분포 재측정) 필수.

### 3.2 hybrid 가능성

X + Y hybrid: excitation u[0..4] 부호 결함 (X) + LP a[] 부호 결함 (Y) 가 *같은 root cause* (예: gain decoding 의 부호 항) 의 *두 분기* 일 수 있음. 본 cycle 측정 부재 — 다음 cycle 의 분리 측정 후 결합 분석 필요.

---

## 4. 다음 cycle 권고 (단일 결정 + task 분해 outline)

### 4.1 단일 결정

**다음 cycle = F-non-prelim (가칭, 추가 진단 cycle) — 후보 X 우선 측정 + Y 보조 측정.**

근거 (측정 기반 1문장): 본 cycle 의 측정 발견 §1.4 (1)+(2) (syn[5..7]=+1 의 출처가 u[0..4] 양 입력의 자기-피드백 + IIR 자체는 spec 정합 선형) 가 부호 결정 항을 *excitation u[0..4] 입력 부호* 로 직접 지시 — 후보 X 가 측정 데이터에 의해 가장 강하게 지지.

plan §Step 4 표 매핑: "모두 반증" → "F-non (가칭, 추가 진단)" — production fix cycle 진입 금지 (Phase 0.4 §1 + plan §Step 3 결정 트리 의무).

### 4.2 다음 cycle task 분해 outline

| Task | 명 | 핵심 입증 의무 | spec § |
|------|------|----------------|--------|
| **F-non-prelim-1** | excitation u[0..4] sign measurement (sample 0..4 한정) | (a) frame 0 sf0 sample 0..4 의 u[n] dump (현재 [+1,+1,+1,+1,+0]). (b) 동일 sample 의 다른 PST vector (PITCH/FIXED/SPEECH) 의 *spec want* sample 0..4 부호 cross-vector 분포. (c) gain decoding (g_p, g_c) + fcb code + pitch contribution 각 sub-항 분리 dump (총합이 u[n] = pitch_contrib + fcb_contrib). 식별: 어느 sub-항이 부호 결정. | §A.3.* + §4.1.5 + §A.3.5 (impulse response) |
| **F-non-prelim-2** | LP a[] cross-check (sample 5..7 영역, Y 보조) | (a) frame 0 sf0 의 a[0..10] (Task 4 §3 dump) 의 *spec reference* (F-sept-2 reference 또는 PDF §A.3.3 LSP→a[] 변환 verbatim) cross-check. (b) a[] 부호 flip 시 syn[5..7] 의 부호 변화 측정 (forced stimulus). 식별: a[] 가 spec 정합인지, 또는 부호 결함이 LP path 에 존재하는지. | §A.3.2 + §A.3.3 + §A.4.1 |
| **F-non-prelim-3** | spec 해석 재검토 (Z 보조, 비용 LOW) | PDF §4.2 / §A.4.2 의 "input/output sample alignment" / "post-processing input domain" 절 verbatim grep + frame-edge 정의 재발췌. PST want 의 *비교 단위* (sample-by-sample / domain) 명시적 확인. | §4.2 + §A.4.2 + §4.3 Table 9 |
| **F-non-prelim-4** | 종합 + 다음 cycle 결정 (X/Y/Z/W 잔여 후보 평가표) | Task 1~3 측정 종합. X 단독 식별 시 production fix cycle (F-non-fix) 진입 outline. X 반증 + Y/Z 잔존 시 추가 진단 cycle. 모두 반증 시 W 진입 (M6 cycle 재실행 의무). | — (메타 task) |

각 task 의무: production 변경 0 라인, 외부 G.729 0 참조, helper 신규 0 (기존 decoder/synth/postfilter package helper 재사용), spec § 인용 = PDF verbatim grep 만, 사전 보유 working tree (`?? stagef_bis_diagnostic_test.go`) 보존, 회귀 게이트 15 (RED) + 본 cycle 신규 4 측정 (PASS) 유지.

### 4.3 진입 게이트

- 본 보고서 commit 직후 `go vet ./...` clean ✓.
- `go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v` PASS ✓.
- `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` RED 잔존 ✓.
- 사전 보유 `internal/decoder/stagef_bis_diagnostic_test.go` 보존 ✓.

---

## 5. 사용자 게이트 (다음 cycle 진입 승인)

본 cycle 종결 + 다음 cycle (F-non-prelim, 후보 X 우선 + Y 보조) 진입 직전 **사용자 승인 의무 항목**:

| # | 항목 | 의사결정 | 비고 |
|---|------|----------|------|
| **G-N1** | 본 cycle 결과 = 4 가설 모두 반증 + 후보 X (excitation u[0..4] 부호) 우선순위 채택 정합 여부 | (a) X 우선 정합 → F-non-prelim plan dispatch / (b) X 반려 → Y 또는 Z 우선 / (c) W 강제 진입 (M6 cycle 재실행) | 측정 기반 권고 = (a). Phase 0.4 §1 의무 정합. |
| **G-N2** | F-non-prelim cycle scope (excitation gain/fcb/pitch sub-항 분리 측정) 진입 승인 — production 변경 0 / 외부 G.729 0 invariant 유지 | (a) 승인 / (b) scope 축소 (excitation 합산만 측정, sub-항 분리 폐기) / (c) 보류 | (b) 선택 시 sub-항 결함 식별 불가 — 진단 효율 저하. |
| **G-N3** | F-non-prelim 의 spec § 인용 범위 확장 승인 (§A.3.* 전체 + §4.1.5 — 본 cycle §A.4.* 한정에서 확장) | (a) 승인 / (b) 한정 유지 (§A.4.* + §A.3.5 only) | spec 영역 확장이 후보 X 측정의 필수 전제. |
| **G-N4** | 사전 보유 working tree 보존 의무 유지 (`?? internal/decoder/stagef_bis_diagnostic_test.go`) | (a) 유지 / (b) 본 cycle 종결과 함께 add 또는 delete | (a) 권고 — F-bis baseline 잔존이 Phase 1 누적 진단의 future audit 자료. |
| **G-N5** | 본 cycle 결과의 F-oct-postfix synthesis (`8907847`) §5 G1 결정 ("후보 ③ pivot") 의 *스코프 한계* 인정 — chain 내부 4 가설 모두 반증으로 후보 ③ 자체가 *상류 (excitation/LP) 또는 spec 해석* 으로 확장 필요 | (a) 인정 + 다음 cycle 진입 / (b) G1 결정 재검토 (후보 ① g_l 영속화 또는 ② Annex A binary 재검토) | (a) 권고. (b) 선택 시 사용자 G1 ((c) Annex A binary 거부) 를 (b) 또는 (a) 로 변경 필요 — 별도 결정 이력. |

### 5.1 잔여 보류 항목 갱신 (plan §Step 5)

| # | 항목 | 본 cycle 갱신 |
|---|------|---------------|
| 1 | F-oct-postfix2-prelim cycle 자체 | **본 cycle 종결** (본 commit) |
| 2 | 4 가설 단일 식별 | **0 식별** (전부 반증) — §2 비교표 |
| 3 | `stagef_bis_diagnostic_test.go` untracked | **보존 유지** (변경 0) — Phase 0.5 충족 |
| 4 | F-oct-prelim-5-4 §3.6 M1 단독 결정 정정 | 본 plan §0.4 §6 + 본 보고서 §1.2 로 갈음 (별도 commit 불요) |
| 5 | 다음 fix cycle 의 RED gate | 항목 15 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) 승계 — 다음 cycle 도 측정-only (RED 유지 의무) |
| 6 | **신규**: 후보 X (excitation u[0..4] 부호) 측정 의무 | 다음 cycle F-non-prelim-1 으로 인계 |
| 7 | **신규**: 후보 W (PST 출처 의문) 잠재 재진입 | M6 반증 데이터 폐기 비용 + 명시적 모순 분석 의무 (Phase 0.4 §6) — 다음 cycle 에서 후보 X/Y/Z 모두 반증 시에만 진입 |

---

## 6. Phase 1k 종결 평가

### 6.1 본 cycle 후 Phase 1k 종결 가능 여부

**평가 = 종결 *불가* (다음 cycle 진입 의무).**

근거:
1. **항목 15 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) RED 잔존**: F-oct-postfix-1 cycle 의 RED gate 가 본 cycle 종결 시점에도 잔존 (의도된 RED — 다음 fix cycle GREEN 의무). production 출력 `[+2,+2,+2]` ↔ PST want `[-1,-1,-1]` (Δ=3) mismatch 가 미해결.
2. **본 cycle 4 가설 모두 반증 + 결함 위치 미식별**: chain 내부 (excitation chain 후단 / synth IIR / postfilter 6 stage / PST want) 모두 spec 정합 측정. 결함 위치는 chain *상류* (후보 X excitation 입력 부호 / Y LP 변환) 또는 spec 해석 (Z) 또는 PST 출처 재의문 (W) — 4 후보 중 1+ 의 측정 cycle 필요.
3. **사용자 G1 결정 (후보 ③ pivot) 의 spec scope 한계 도달**: 본 cycle 후보 ③ 의 4 가설 측정이 모두 반증 → G1 결정 자체의 spec 영역 확장 (§A.3.* + §4.1.5) 가 다음 cycle 의 필연적 후속 (사용자 G-N5 의무).

### 6.2 Phase 1k 종결의 필요 조건 (잔여)

| 조건 | 현재 상태 | 잔여 작업 |
|------|----------|----------|
| 항목 15 RED → GREEN 전환 | RED 잔존 | 다음 fix cycle (F-non-fix 가칭) 의 production 1~3 라인 fix |
| chain 결함 위치 단일 식별 | 0 식별 (4 가설 반증) | F-non-prelim-1 (X 측정) → 식별 시 fix cycle 진입 |
| 회귀 게이트 1~14 PASS 유지 | PASS | 다음 cycle 모든 task commit 직후 재확인 |
| 비-contract 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) FAIL | FAIL 유지 (production 0 변경 의무로 자동) | fix cycle 진입 시 cross-check 의무 |
| 사전 보유 working tree (`stagef_bis_diagnostic_test.go`) 처리 | 보존 (untracked) | Phase 1k 종결 commit 직전 사용자 G-N4 결정 |

### 6.3 종결 시점 전망

- **최단 경로** (X 단독 식별): F-non-prelim-1 (X 측정) → X 단독 유력 → F-non-fix (production 1~3 라인 fix) → Phase 1k 종결. 2 cycle 추가.
- **중간 경로** (X+Y hybrid): F-non-prelim-1/2 → hybrid 식별 → F-non-fix-A + F-non-fix-B 2 fix cycle → Phase 1k 종결. 3 cycle 추가.
- **장기 경로** (X/Y/Z 모두 반증, W 진입): F-non-prelim-1/2/3 → 모두 반증 → W (M6 cycle 재실행) → 식별 → fix. 4+ cycle 추가.

본 cycle 측정 §1.4 (1) 가 X 를 강하게 지지 — *최단 경로 가능성 높음* (측정 기반 추정, 강압-적합 아님).

---

## Appendix — Spec § 인용 (PDF verbatim, 본 cycle 인용 누적)

(plan §"Spec § 인용" + Task 2 §0.1 + Task 4 §0 인용 동상 — 중복 등재 생략. 본 보고서의 신규 PDF 인용 0.)

본 보고서는 다음 commit 의 PDF verbatim 인용을 ground-truth 로 사용:
- ITU-T G.729 (06/2012) §A.3.* (Decoder), §A.4.1 (= §4.1 Parameter decoding), §A.4.2.1/2/3/4 (Postfilter cascade), §3.10 (LP synthesis IIR linearity), §4.3 Table 9 (codec-start state init).
- READMETV.txt verbatim (PST format = Intel PC = 16-bit signed little-endian).

외부 G.729 구현 참조 0건 (E4 미발동 — 사용자 G1 결정 정합).
