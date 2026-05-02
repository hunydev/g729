# Phase 0c Re-entry — Want-Domain Re-interpretation Plan

**Cycle ID**: `P0c-reentry` (Phase 0c 재진입, Phase 1k 잠정 종결 직후 alternative path (a))
**작성일**: 2026-05-05
**선행 cycle**: `F-non-Cgamma-revisit` (synthesis commit `d448282`, verdict = (Cγ-refute) Phase 1k 잠정 종결)
**사용자 승인**: G-XS3 = "(a) Phase 0c (PCM/IO) 재진입 + want 도메인 재해석 cycle, 3 sub-task"
**선행 plan 양식**: `docs/superpowers/plans/2026-05-04-phase1k-stage-f-non-cgamma-revisit-plan.md` (commit `c743116`)

---

## Phase 0 — Context, Invariant, Cumulative Catalog

### 0.1 직전 cycle 정리 (F-non-Cgamma-revisit, 2026-05-04, commit `d448282`)

3 task 완수, sub-hypothesis G-1 + G-2 모두 spec 정합 폐기:

| task | commit | 측정 대상 | 결과 verdict |
|------|--------|-----------|--------------|
| F-non-Cgamma-revisit-1 | `a4120f9` | G-1 postfilter 4 sub-stage (long-term / short-term / tilt / AGC+HP) sample 5..7 | **EQ_ALL** — 4 sub-stage polarity-preserve 일치 |
| F-non-Cgamma-revisit-2 | `b30bb7a` | G-2 synth IIR memory pre/post + Y magnitude +6 perturbation 시 syn[5..7] 부호 | **EQ_ALL** — pre/post mem_syn 정합, magnitude perturbation → syn 부호 불변 |
| F-non-Cgamma-revisit-3 | `d448282` | synthesis (10 cycle 누적 + Phase 1k 종결 평가) | (Cγ-refute) **Phase 1k 잠정 종결** + alternative (a/b/c) 사용자 게이트 권고 |

**Cγ 폐기** (G-1 EQ + G-2 EQ) → 잔여 mechanism 후보 = **spec 외부** (PST 파일 자체 / want 도메인 해석 / multi-frame state 누적 / spec 재해석).
사용자 결정 G-XS3 = (a) → 본 cycle 진입.

### 0.2 10-cycle / 16-sub-hypothesis 누적 폐기 catalog (Phase 1k)

| cycle | sub-hypothesis | verdict | 폐기 sub-stage |
|-------|----------------|---------|----------------|
| F-oct-postfix-1 | M1 (LP→postfilter chain bias) | spec-측정 RED 잔존 (mechanism 외부 미식별) | — |
| F-oct-postfix-2 | M2 (postfilter coefficient quantization) | spec 정합 폐기 | M2 |
| F-oct-postfix2-prelim-1 | M1' (postfilter HP coefficient) | spec 정합 폐기 | M1' |
| F-oct-postfix2-prelim-2 | M3 (AGC gain Q-format) | spec 정합 폐기 | M3 |
| F-oct-postfix2-prelim-3 | M5 (PST byte-level 재확인 want=`ff ff ff ff ff ff`) | spec 정합 폐기 | M5 |
| F-oct-postfix2-prelim-4 | M6 (sPf/HP/×2 chain trace) | spec 정합 폐기 | M6 |
| F-oct-postfix2-prelim-5 | Cα (fcb pulse) sample 5..7 prelim | spec 정합 폐기 | Cα(prelim) |
| F-oct-postfix2-prelim-6 | Z (PST chain post-AGC+HP+×2 §A.4.2.5) | spec 정합 폐기 | Z |
| F-non-prelim-1 | Y (a[0..10] sign 11/11) | spec 정합 (sign), magnitude max\|Δ\|=6 잔존 | Y(sign) |
| F-non-prelim-2 | forced (-u) → syn(-u) = -syn(+u) linearity | spec 정합 폐기 | forced-flip |
| F-non-prelim-3 | (cumulative) | — | — |
| F-non-prelim-X-split-1 | Cα fcb pulse sample 5..7 | spec 정합 폐기 | Cα(s5..7) |
| F-non-prelim-X-split-2 | Cβ gain g_c sample 5..7 | spec 정합 폐기 | Cβ(s5..7) |
| F-non-prelim-X-split-3 | (synthesis) | — | — |
| F-non-Cgamma-revisit-1 | G-1 postfilter 4 sub-stage sample 5..7 | spec 정합 폐기 | G-1 |
| F-non-Cgamma-revisit-2 | G-2 synth IIR memory + Y magnitude small perturbation | spec 정합 폐기 | G-2 |

**누적 결함 0건** (10 cycle 16 sub-hypothesis 폐기). spec-internal mechanism 후보 = **공집합**.

### 0.3 ALGTHM frame 0 sf0 sample 5..7 측정-state table (carry from §0.3 직전 plan)

| 항목 | 값 | 출처 cycle |
|------|------|-----------|
| g_p (Q14) | +1995 | F-non-prelim-X-split-2 |
| g_c (Q1) | +4153 | F-non-prelim-X-split-2 |
| v[0..4] (adaptive cb) | 0 | F-non-prelim-1 |
| c[0..3] (fcb pulse, Q13) | +8192 each | F-non-prelim-X-split-1 |
| u[0..3] (excitation) | +1 each | F-non-prelim-1 |
| u[4..7] | 0 | F-non-prelim-1 |
| syn[5..7] (synthesis output) | `[+1, +1, +1]` | F-non-prelim-1 |
| sPf[5..7] (postfiltered) | `[+1, +1, +1]` | F-oct-postfix2-prelim-4 |
| post-HP (PST) | `[+2, +2, +2]` | F-oct-postfix2-prelim-4 |
| **want (ALGTHM.PST byte-level)** | `[−1, −1, −1]` (raw bytes `ff ff ff ff ff ff`) | F-oct-postfix-1 / M5 byte-verify cb9529d |
| a[0..10] sign vs reference | 11/11 일치 | F-non-prelim-1 |
| a[1..10] magnitude max\|Δ\| | 6 | F-sept-2 |
| forced (-u) → syn 부호 | 완전 반전 (linearity 입증) | F-non-prelim-2 |
| Z (PST chain) | post-AGC + post-HP + post-×2 (§4.2 + §A.4.2.5) | F-oct-postfix2-prelim-6 |
| Δ (production − want) sample 5..7 | uniform **+3** (`+2 − (−1) = +3`) | 본 cycle 도출 |

**Δ=+3 sample-uniform** 패턴 → 후보: (i) post-processing 후 상수 합 / 감산 / shift, (ii) want 도메인 단위 mismatch (Q15 normalized vs Q0 raw), (iii) frame 0 specific bias (multi-frame state 누적), (iv) PST 파일이 우리가 가정한 chain 단계와 다른 단계 출력.

### 0.4 Invariant E1-E5 재확인 (carry, 강압-적합 회피)

- **E1**: 외부 G.729 구현 0건 참조. ITU-T G.729 (06/2012) PDF + READMETV.txt 만 spec source. **Annex A binary 사용 금지** (G1 결정).
- **E2**: production 변경 0 라인 (측정 only). 본 cycle 3 task 모두 진단 test 추가만. 결함 식별 시 **별도 fix cycle** 으로 분리.
- **E3**: F-oct-postfix-1 RED (gate 17) 영구 잔존 — fix 시점 = mechanism 식별 후 별도 cycle.
- **E4**: 측정값과 spec 비교 시 **PDF/README verbatim 인용 의무**. cherry-pick 금지. 모든 verdict = `EQ` / `NE` 이진.
- **E5**: 자동 promotion 0 — 측정-only test 는 회귀 게이트 자동 등재 금지. synthesis 결정 후 **명시 사용자 게이트** 통해 promotion.

**강압-적합 회피 절차** (Phase 0.4 재확인):
1. 측정 결과가 README/PDF 와 mismatch 일 때 = production bug 후보. mismatch 가 spec scope 밖일 때 = sub-hypothesis 폐기.
2. **금지**: "거의 정합" / "범위 내 변동" / "byte 수 일치하니 format 정합". byte 수 일치 + endianness 검증 + 단위 검증 = 3 차원 모두 EQ 필수.
3. **금지**: README 모호 paragraph (예: "16 bit sampled data" 의 sign / Q-format 미명시) 를 우리 구현 정당화로 사용. 모호 지점 = 별도 sub-hypothesis 로 분리.

### 0.5 누적 contract test gate (19건)

| # | gate | 상태 | 출처 |
|---|------|------|------|
| 1..16 | (Phase 1a~1j 누적 16건) | PASS | 누적 |
| 17 | F-oct-postfix-1 ALGTHM.PST sample 5..7 부호 일치 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`, `internal/decoder/stagef_octpostfix_regression_test.go`, commit `56caa72`) | **RED 잔존** | F-oct-postfix-1 |
| 18 | F-non-prelim-X-split measurement bundle (Cα fcb + Cβ gain) | PASS | F-non-prelim-X-split (commit `aa9dcf9`) |
| 19 | F-non-Cgamma-revisit measurement bundle (G-1 + G-2 EQ_ALL) | **pending** (E5 사용자 게이트 미수행) | F-non-Cgamma-revisit |

회귀 게이트 commit 직후 검증:
- `go vet ./...` clean.
- 누적 18 gate PASS/FAIL dump (1..16 + 18 PASS, 17 RED 잔존).
- 19번 = pending (E5 게이트 대기).
- 본 cycle 신규 게이트 0건 (측정-only, 자동 promotion 금지 → P0c-1/2/3 합쳐 잠정 gate 20번 = pending, Task 4 synthesis 결정 후 promotion 여부 사용자 합의).

### 0.6 Working tree 보존 명시

- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) **변경 금지** — 직전 cycle 부터 보존된 진단 test, 본 cycle scope 외.
- 본 cycle commit 시 `git status` 에 해당 파일 untracked 상태 그대로 유지 확인.

---

## Phase 1 — Hypothesis Tree (want-domain 재해석)

```
P0c (want-domain 재해석 후보, 사용자 G-XS3 = (a))
├── P0c-1 (PST 파일 format / endianness / header / 단위)
│   ├── byte-order (Intel little-endian int16 — README 인용)
│   ├── header presence (없음 가설 — 5600 bytes / 2 / 80 = 35 frames 정수)
│   ├── sample count (frame count 정합)
│   └── sample unit (Q0 int16 raw vs Q15 normalized vs ×2 scaled)
├── P0c-2 (want chain stage 식별)
│   ├── syn (synthesis IIR 직후) sample 0..79 vs want
│   ├── sPf (postfilter 직후) sample 0..79 vs want
│   ├── post-HP (HP filter 직후) sample 0..79 vs want
│   └── post-×2 (AGC+HP+×2 직후, 현재 production PST) sample 0..79 vs want
├── P0c-3 (cross-vector Δ 패턴)
│   ├── ALGTHM.PST frame 0 sample 0..15 (baseline)
│   ├── SPEECH.PST frame 0 sample 0..15
│   ├── FIXED.PST frame 0 sample 0..15
│   └── PITCH.PST frame 0 sample 0..15
└── Synthesis (Task 4, 본 plan 외 ad-hoc) — 3-시나리오 결정 트리
    ├── (P0c-format-defect) Task 1 NE → fix cycle (PST 파일 format 해석 변경)
    ├── (P0c-want-stage-defect) Task 2 S* ≠ post-×2 → spec re-interpretation cycle
    └── (P0c-refute) Task 1 + Task 2 EQ + Task 3 cross-vector consistent → alternative (b) Phase 1g multi-frame state 진입
```

**기대 entropy** (사전):
- (P0c-format-defect) ≈ 15% — M5/M6 byte-verify 로 sample 5..7 byte EQ 확인됨 (단 frame 0 전체 / 단위는 미측정).
- (P0c-want-stage-defect) ≈ 30% — Z (post-AGC+HP+×2) 는 §A.4.2.5 + README 로 입증되었으나, README "decoder file.bit file.pst" 표현은 chain 종착점만 명시 (×2 여부 모호).
- (P0c-refute) ≈ 55% — 10 cycle 누적 결함 0건 base rate + Δ=+3 sample-uniform 패턴 의 spec-내부 설명 부재.

---

## Phase 2 — Task 분해 (3 task, TDD 측정-only)

### Task 1: P0c-1 — ALGTHM.PST 파일 format / endianness / header / 단위 재검증

**목적**: `internal/decoder/testdata_helpers_test.go` 의 PST loader 가정 (header 없음 + Intel little-endian int16 + 80 sample/frame + Q0 raw int16) 4 차원 모두 README 와 EQ 인지 재검증.

**선행 측정 보완**: M5 (commit `cb9529d`) 에서 sample 5..7 6 byte 만 byte-verify (`ff ff ff ff ff ff`). frame 0 전체 (160 byte) + 마지막 frame (frame 34) + 파일 길이 도출 frame count 미측정. 단위 (Q0 vs Q15) 명시적 분류 부재.

**README verbatim 인용 의무** (Task 1 첫 단계):
- `testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt` 인용:
  > "Format: all files contain 16 bit sampled data using the Intel (PC) format."
  > "*.out  - output files"
  > "decoder file.bit file.pst"
  > "5600  algthm.pst"
- 주의: README 는 *.out 와 *.pst 의 관계 / sample unit (Q0 raw int16 vs scaled) / sign convention 명시 없음. → 모호 지점 별도 분류 (E4).
- PDF §A.4.2.5 (post-processing) 인용 의무 — sample unit + ×2 multiplier verbatim.

**TDD 절차**:
1. **RED**: `internal/decoder/phase0c_pst_format_diagnostic_test.go` 신규 — `TestDiagnostic_Phase0c1PstFormatTrace`.
   - raw byte read (`os.ReadFile("ALGTHM.PST")`) → 5600 byte.
   - dump 의무:
     - first 32 byte (header 후보 영역, frame 0 sample 0..15) hex.
     - last 8 byte (frame 34 sample 76..79) hex.
     - 파일 길이 → frame count = `len/160` (정수 분해 검증).
     - little-endian int16 해석 → first 16 sample value list.
     - big-endian 가설 검증 (대조군) → 동일 16 sample big-endian 해석값 list.
     - Q0 (raw int16) 가설 vs Q15 (normalized [-1,+1)) 가설 vs ×2 scaled 가설 → 3 단위 가설별 sample 5..7 해석값 dump.
   - classifier `classifyPstFormat()` — 4 차원 (byte-order / header / frame-count / unit) 별 EQ/NE.
   - reference: README 인용 verbatim + PDF §A.4.2.5 verbatim.
2. **GREEN**: production 변경 0 (E2). test = 측정 only.
3. **dump 확인**: 4 차원 verdict 4-tuple (예: `[byte-order=EQ, header=EQ, frame-count=EQ, unit=NE]`).
4. **commit**:
   ```
   test(decoder): add Phase 0c-1 ALGTHM.PST format diagnostic

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

**측정 의무** (1줄): READMETV.txt + PDF §A.4.2.5 인용 하 ALGTHM.PST 4 차원 (byte-order / header / frame-count / unit) 별 EQ/NE 4-tuple verdict.

**polarity expectation**:
- byte-order = little-endian (README "Intel (PC) format") → EQ 기대.
- header = 없음 (5600 = 160×35 정수 분해) → EQ 기대.
- frame-count = 35 → EQ 기대.
- unit = Q0 raw int16 (PDF §A.4.2.5 verbatim 후 결정) → 미정. NE 시 mechanism 후보 식별 (예: want 가 Q15 normalized 인데 우리 production 이 Q0 비교).

**escape hatch**: 4 차원 모두 EQ → P0c-format-defect 폐기 → Task 2 진행.
NE 시나리오 (특히 unit NE) → 즉시 Task 2 보류, P0c-format-fix cycle dispatch 사용자 게이트.

---

### Task 2: P0c-2 — want chain stage 식별 (어느 chain stage 가 PST 와 정합?)

**목적**: 우리 production 이 가정한 "PST = post-AGC+HP+×2 출력" (Z verdict, F-oct-postfix2-prelim-6 commit) 이 frame 0 전체 80 sample 에 대해 EQ 인지 재검증. EQ 가 아니면 어느 chain stage S* 가 want 에 가장 정합 (max sign-correlation 또는 min Σ|stage[i]−want[i]|) 인지 분류.

**선행 측정 보완**: F-oct-postfix2-prelim-4 (commit `f04ec88`) 에서 sample 5..7 한정 4 stage 출력 측정. 본 task = frame 0 전체 80 sample 4 stage × want 비교.

**README/PDF verbatim 인용 의무** (Task 2 첫 단계):
- READMETV.txt 인용 (위와 동일):
  > "decoder file.bit file.pst"
  → README 는 PST 가 "decoder 출력" 이라고만 명시. postfilter / HP / ×2 명시 없음.
- PDF §4.2 verbatim: postfilter 정의 (long-term + short-term + tilt + AGC).
- PDF §A.4.2.5 verbatim: post-processing (HP filter + ×2 multiplier) — Annex A simplification 명시.
- PDF §4.1.6 verbatim: synthesis IIR (sPf 입력 = syn 출력 정의).

**TDD 절차**:
1. **RED**: `internal/decoder/phase0c_want_stage_diagnostic_test.go` 신규 — `TestDiagnostic_Phase0c2WantStageTrace`.
   - frame 0 80 sample, 4 chain stage 출력 dump:
     - (a) `syn[0..79]` (synthesis IIR 직후, §4.1.6).
     - (b) `sPf[0..79]` (postfilter 직후, §4.2).
     - (c) `postHP[0..79]` (HP filter 직후, §A.4.2.5 step 1).
     - (d) `postX2[0..79]` (×2 직후, §A.4.2.5 step 2 = 현재 production PST).
   - want = `readPSTFrames("ALGTHM.PST")[0][0..79]`.
   - 각 stage 대해:
     - sign-match count `sum_{i=0..79} (sign(stage[i]) == sign(want[i]))`.
     - Σ|stage[i] − want[i]|.
     - Δ pattern (sample-uniform 상수 / sign-uniform / random) 분류.
   - classifier `classifyWantStage()` → S* = argmin Σ|stage[i]−want[i]|.
   - verdict:
     - S* = postX2 → spec 정합 (현 가정 유지) → P0c-want-stage-defect 폐기.
     - S* ≠ postX2 → spec 재해석 후보 식별 (예: S* = postHP → ×2 가 want 에 적용 안됨).
2. **GREEN**: production 변경 0 (E2).
3. **dump 확인**: 4 stage × 80 sample 매트릭스 + 4 sign-match count + 4 Σ|Δ| + S* + Δ pattern 분류.
4. **commit**:
   ```
   test(decoder): add Phase 0c-2 want-stage interpretation diagnostic

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

**측정 의무** (1줄): frame 0 80 sample 4 chain stage (syn / sPf / postHP / postX2) 각각의 sign-match count + Σ|Δ| + S* (argmin) verdict + Δ pattern 분류.

**polarity expectation**:
- S* = postX2 (현 가정) → sign-match=80, Σ|Δ|=0 기대. **그러나 gate 17 RED 존재 → 일부 sample NE 확정**. 따라서 sign-match < 80, Σ|Δ| > 0.
- 만약 S* = postHP (×2 미적용 stage) → want 가 ×2 미적용 → §A.4.2.5 인용 재해석 필요.
- 만약 S* = sPf (HP 미적용 stage) → want 가 §4.2 종착 → §A.4.2.5 step 전체 미적용.
- 만약 4 stage 모두 sign-match < 80 → spec 외부 mechanism (Task 3 cross-vector 확인).

**escape hatch**: S* = postX2 + sign-match ≥ 78 (sample 5..7 만 NE) → 현 가정 유지 + Task 3 진행. S* ≠ postX2 → P0c-want-stage-defect cycle 사용자 게이트.

---

### Task 3: P0c-3 — cross-vector Δ 패턴 검증

**목적**: ALGTHM.PST frame 0 sample 5..7 의 Δ=+3 패턴이 ALGTHM 단독 현상인지, 또는 다른 ITU PST vector 에서도 systemic Δ 패턴이 있는지 cross-vector 측정.

**선행 측정 부재**: 직전 10 cycle 모두 ALGTHM.PST 단독 측정. 다른 vector (SPEECH, FIXED, PITCH 등) frame 0 의 sign / magnitude 비교 0건.

**대상 vector (Annex A test_vectors 디렉토리, READMETV.txt 인용)**:
- 본 cycle 사용 가능 vector (file size + .bit pair 존재):
  - `ALGTHM.PST` (5600 byte, 35 frame) — baseline (현재 RED).
  - `SPEECH.PST` (600000 byte, 3750 frame) — 일반 음성 (algorithm 종합 coverage).
  - `FIXED.PST` (19200 byte, 120 frame) — fixed codebook 집중.
  - `PITCH.PST` (293600 byte, 1835 frame) — pitch search 집중.
  - (보조 후보) `TAME.PST` (20480 byte, 128 frame), `PARITY.PST` (48000 byte, 300 frame), `LSP.PST` (357120 byte, 2232 frame).
- **선택 3 vector** (Task 3 측정 대상): SPEECH, FIXED, PITCH (+ baseline ALGTHM).
- **사용자 spec 의 SPEED.PST / SINE.PST 는 Annex A 디렉토리에 부재** (실제 존재 vector 로 대체 — 이는 spec 보강 사항, 별도 cherry-pick 아님).

**TDD 절차**:
1. **RED**: `internal/decoder/phase0c_cross_vector_diagnostic_test.go` 신규 — `TestDiagnostic_Phase0c3CrossVectorPattern`.
   - sub-test 4건 (ALGTHM / SPEECH / FIXED / PITCH).
   - 각 vector 대해:
     - `.bit` decode → production PST 출력 frame 0 sample 0..15.
     - `.pst` read → want frame 0 sample 0..15.
     - Δ[i] = production[i] − want[i] for i ∈ [0..15].
     - Δ 패턴 분류 (`classifyDeltaPattern()`):
       - (i) **sample-uniform constant**: Δ[i] 가 i 무관 상수 → post-processing 상수 합 후보.
       - (ii) **sign-uniform ±1 jitter**: |Δ[i]| ≤ 1, Δ 부호 일정 → LSB rounding 후보.
       - (iii) **random bias**: Δ 부호 / magnitude 무작위 → chain-internal Δa 전파 후보.
       - (iv) **zero**: Δ[i] = 0 ∀ i → vector 정합 (frame 0 specific 문제 아님).
   - vector 부재 시 `tb.Skip()` + 어떤 vector 사용했는지 log.
   - classifier verdict matrix:

     | vector | Δ pattern | sample 5..7 Δ | sign-match (16) |
     |--------|-----------|----------------|-----------------|
     | ALGTHM | (예: i) +3 | (+3,+3,+3) | (예: 13/16) |
     | SPEECH | ? | ? | ? |
     | FIXED | ? | ? | ? |
     | PITCH | ? | ? | ? |

2. **GREEN**: production 변경 0 (E2).
3. **dump 확인**: 4-vector × Δ pattern matrix + 16-sample Δ list per vector.
4. **commit**:
   ```
   test(decoder): add Phase 0c-3 cross-vector want pattern diagnostic

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

**측정 의무** (1줄): 4 vector (ALGTHM + SPEECH + FIXED + PITCH) frame 0 sample 0..15 production vs want Δ pattern 4-row matrix + (i)~(iv) 분류 verdict.

**polarity expectation**:
- (i) sample-uniform Δ 가 4 vector 공통 → post-processing 상수 mechanism (예: ×2 후 +N offset 누락).
- (iv) ALGTHM 만 NE + 다른 3 vector 모두 zero → ALGTHM frame 0 specific (multi-frame state init 문제 가능성, alternative (b) Phase 1g 후보).
- (ii)/(iii) → Δ 의 chain-internal 전파 (현재 sub-hypothesis catalog 폐기 결과와 모순 → 재측정 필요 sub-hypothesis 식별).

**escape hatch**: 모든 vector zero → 본 cycle scope 외 (ALGTHM 이 우연히 NE — 가능성 ≈ 0%, gate 17 RED 와 모순). vector 부재 시 skip + 사용 가능 vector 만으로 verdict 도출 + 사용자 게이트로 추가 vector 확보 권고.

---

## Phase 3 — 회귀 게이트 (각 commit 직후)

각 task commit 직후 실행:
1. `go vet ./...` — clean 필수.
2. 누적 19 gate dump:
   - 1..16 PASS (변동 없음).
   - 17 **RED 잔존** (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` `internal/decoder/stagef_octpostfix_regression_test.go`).
   - 18 PASS (F-non-prelim-X-split bundle).
   - 19 pending (E5 게이트 미수행).
3. 본 cycle 신규 측정 test 3건 = 모두 측정-only, 회귀 게이트 자동 등재 금지 (E5).
4. test 실행 명령:
   - Task 1: `go test ./internal/decoder/ -run Phase0c1 -v`
   - Task 2: `go test ./internal/decoder/ -run Phase0c2 -v`
   - Task 3: `go test ./internal/decoder/ -run Phase0c3 -v`
   - 누적: `go test ./...` (RED 17 잔존 확인, 본 cycle 신규 test PASS 확인).

---

## Phase 4 — Escape hatch E1-E5

| code | 발동 조건 | 행동 |
|------|----------|------|
| E1 | 외부 G.729 구현 참조 유혹 (ITU reference C, Annex A binary, 3rd party fork, bcg729, Sipro, FFmpeg) | 즉시 차단. spec source = PDF + READMETV.txt only. 모호 지점 = 별도 sub-hypothesis 분리. |
| E2 | production 변경 유혹 (측정 중 fix 욕구; 특히 Task 1 unit NE 발견 시 PST loader 수정 욕구, Task 2 S* ≠ postX2 발견 시 chain 수정 욕구) | 즉시 차단. fix = 별도 cycle (P0c-format-fix / P0c-want-stage-fix). |
| E3 | gate 17 RED 잔존을 task failure 로 간주하려는 유혹 | 차단. 17 = mechanism 식별 후 별도 fix cycle (본 cycle 의 Task 4 synthesis 시 결정). |
| E4 | README "16 bit sampled data" 모호 해석 (Q-format 미명시) 을 우리 구현 (Q0 raw int16) 정당화로 cherry-pick 하려는 유혹 | 차단. README 모호 → 명시적 모호 분류 + PDF §A.4.2.5 cross-reference + 모호 지점 별도 sub-hypothesis 분리. |
| E5 | 측정-only test 자동 promotion 유혹 (Task 1/2/3 commit 시 회귀 게이트 등재) | 차단. promotion = Task 4 synthesis 결정 후 사용자 명시 게이트 G-XS4. |

---

## Phase 5 — Self-review (작성자)

- ✅ Phase 0 (직전 cycle 정리 + invariant + 10 cycle 16 sub-hypothesis 누적 catalog) 포함.
- ✅ Phase 0.3 ALGTHM frame 0 sf0 measured-state table (Δ=+3 sample-uniform 추가).
- ✅ Phase 0.4 강압-적합 회피 절차 (README + PDF verbatim + cherry-pick 금지).
- ✅ Phase 0.5 19 gate 상태 (1..16 PASS, 17 RED, 18 PASS, 19 pending).
- ✅ Phase 0.6 untracked file (`stagef_bis_diagnostic_test.go`) 보존 명시.
- ✅ Task 3개 분해 (P0c-1 format / P0c-2 want-stage / P0c-3 cross-vector).
- ✅ 각 task TDD (RED→GREEN→commit, production 0 = E2).
- ✅ 각 task commit 메시지 양식 + Co-authored-by trailer 명시 (verbatim).
- ✅ 회귀 게이트 19건 + 본 cycle 신규 게이트 자동 등재 금지 (E5).
- ✅ Escape hatch E1-E5 (P0c 특수 trigger 명시).
- ✅ 외부 G.729 구현 0건 참조 (E1).
- ✅ production 변경 0 라인 (E2).

**위험 요소**:
- (R-A) 4 vector 모두 sample-uniform Δ 패턴 → post-processing 상수 mechanism 이지만 §A.4.2.5 verbatim 인용 시 그런 상수 명시 부재 → spec 모호 영역 발견 → 별도 sub-hypothesis 분리 (P0c-postproc-spec-ambiguity).
- (R-B) Task 1 unit NE 발견 시 즉시 fix 충동 — E2 강력 차단. 측정 verdict 기록 후 사용자 게이트.
- (R-C) cross-vector test 의 production decode pipeline 호출 표면 (frame 0 PST 출력 추출) 부재 시 Task 3 미실행 risk → `decode.go` `Decode()` API 의 PST 출력 buffer 직접 호출 가능 여부 사전 검증 의무 (Task 3 첫 단계).
- (R-D) 사용자 spec 의 SPEED.PST / SINE.PST 부재 → 실제 존재 vector (SPEECH/FIXED/PITCH) 로 대체. 본 결정 = vector 가용성 evidence 기반 (cherry-pick 아님).

---

## Phase 6 — Execution Handoff

**다음 dispatch**: `P0c-1` (Task 1, ALGTHM.PST 파일 format / endianness / header / 단위 재검증).

**선행 의무 (dispatch 직전)**:
1. Phase 0.5 19 gate baseline 확인 (`go test ./...` + 17 RED 잔존 확인 + 본 cycle 신규 test 0건 확인).
2. `internal/decoder/testdata_helpers_test.go` 의 `readPSTFrames` + `vectorPath` 호출 표면 점검.
3. `testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt` 전문 재독 + Phase 1 hypothesis tree + Phase 2 Task 1 측정 의무 1줄 재확인.

**완료 trigger**: Task 3 commit 직후 → ad-hoc Task 4 synthesis dispatch (별도 plan / report 파일) → 3-시나리오 verdict + 사용자 게이트 G-XS4.

---

## Phase 7 — Synthesis 트리거 + 3-시나리오 결정 트리

**Task 4 (ad-hoc synthesis)**: 본 plan 외 별도 dispatch (3 task 완료 후). 본 plan 종료 시점 = Task 3 commit.

**3-시나리오 결정 트리** (Task 4 시점 적용):

| 시나리오 | 조건 | 다음 cycle | 누적 cycle 추정 |
|---------|------|-----------|-----------------|
| **(P0c-format-defect)** | Task 1 4 차원 중 ≥1 NE | P0c-format-fix (1 fix cycle) | 본 cycle + fix = 2 cy 종결 |
| **(P0c-want-stage-defect)** | Task 1 EQ + Task 2 S* ≠ postX2 | P0c-want-stage-fix (spec 재해석 cycle) | 본 cycle + 재해석 + fix = 3 cy 종결 |
| **(P0c-refute)** | Task 1 EQ_ALL + Task 2 S* = postX2 + Task 3 cross-vector consistent (sample-uniform 또는 zero) | alternative (b) Phase 1g multi-frame state 진입 | 본 cycle 종결 + Phase 1g 별도 plan |

**(P0c-refute) 시 권고 alternative path**:
- (b) Phase 1g (decoder integration) 재진입 + multi-frame state 진단 — frame 0 specific 누적 state 측정 (특히 prev_frame init 0 가정 vs ITU 가정).
- (P0c-refute) 강화 evidence: Task 3 cross-vector 에서 다른 vector frame 0 도 sample-uniform Δ 동일 패턴 → post-processing 상수 mechanism 강력 후보 (Phase 1g 보다 §A.4.2.5 재해석 우선).

**측정 bundle promotion** (시나리오 별):
- (P0c-format-defect) / (P0c-want-stage-defect): 측정 bundle = mechanism 식별 evidence → fix cycle 후 promotion (gate 20 PASS 등재).
- (P0c-refute): 측정 bundle = spec 외부 mechanism 후보 폐기 evidence → promotion 검토 (E5 자동 promotion 금지 → 사용자 G-XS4 합의 후).

---

**Plan 종료.** 본 commit = P0c-reentry cycle 0번째 (plan-only) commit. 다음 commit = Task 1 (`test(decoder): add Phase 0c-1 ALGTHM.PST format diagnostic`).

---

## Task 진행 status

- [x] Task 1 — P0c-1 (ALGTHM.PST format / endianness / header / 단위 재검증) — done.
- [x] Task 2 — P0c-2 (want chain stage 식별, 4 stage × frame 0 80 sample) — done. S* = postX2 (argmin sumAbsDiff = 314), signMatch = 76/80 (< 78 escape-hatch). Verdict = NE (sample 5..7 + 1 추가 sample sign mismatch); spec assumption (PST = post-AGC+HP+×2) chain-stage 식별 측면에서는 holds (S* = postX2) but escape-hatch threshold 미충족. 사용자 게이트 권장.
- [x] Task 3 — P0c-3 (cross-vector Δ pattern, ALGTHM + SPEECH + FIXED + PITCH) — pending.
- [ ] Task 4 — synthesis (ad-hoc, 본 plan 외 dispatch) — pending.
