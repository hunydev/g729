# Phase 1l — F-non-Hpost Cycle Plan (inter-subframe postfilter state + HP edge)

**Cycle ID**: `F-non-Hpost` (Phase 1l 1번째 cycle, NEW (P0c-inter-subframe-postfilter-state) 시나리오 구체화)
**작성일**: 2026-05-06
**선행 cycle**:
- `F-non-Cgamma-revisit` (synthesis commit `d448282`) — Phase 1k 잠정 종결.
- `P0c-reentry` (synthesis commit `8e6386c`, tasks `8ec97f5` / `aeee9e9` / `68a7df9`) — Phase 0c 재진입, NEW (P0c-inter-subframe-postfilter-state) 시나리오 확정.
**사용자 승인**: G-XS4 = "(F-non-Hpost cycle 진입, 3 sub-task: HP-1 inter-subframe postfilter state + HP-2 HP filter frame-edge + HP-3 synthesis)"
**선행 plan 양식**: `docs/superpowers/plans/2026-05-05-phase0c-reentry-want-domain-reinterpret-plan.md` (Phase 0c re-entry plan)

---

## Phase 0 — Context, Invariant, Cumulative Catalog

### 0.1 직전 cycle 정리 (Phase 1k 잠정 종결 + Phase 0c 재진입)

**Phase 1k 종결** (`F-non-Cgamma-revisit`, commit `d448282`):
- 10 cycle / 16 sub-hypothesis 누적 폐기. spec-internal mechanism 후보 = 공집합. defect = 0.
- (Cγ-refute) 결정 + 사용자 게이트 → alternative (a) Phase 0c 재진입 진입.

**Phase 0c 재진입** (`P0c-reentry`, synthesis commit `8e6386c`):

| task | commit | 측정 대상 | 결과 verdict |
|------|--------|-----------|--------------|
| P0c-1 | `8ec97f5` | ALGTHM.PST 파일 format / endianness / header / 단위 4 차원 | **EQ_ALL** — little-endian + header 없음 + frame=35 + Q0 raw int16 |
| P0c-2 | `aeee9e9` | want chain stage 식별 (4 stage × 80 sample) | S* = `postX2` (argmin Σ\|Δ\|=314), sign-match=76/80 — 현 가정 holds, 단 escape-hatch 78 미충족 |
| P0c-3 | `68a7df9` | cross-vector Δ 패턴 (ALGTHM/SPEECH/FIXED/PITCH frame 0) | **NEW (P0c-inter-subframe-postfilter-state) 시나리오 식별** — low-energy boundary-cluster + high-energy interior `[40..64]` Δ split |
| P0c-4 (synth) | `8e6386c` | 4-시나리오 결정 트리 적용 | NEW 시나리오 확정 → `F-non-Hpost` cycle 권고 + 사용자 G-XS4 |

**핵심 cross-vector evidence** (P0c-3 결과):
- low-energy (ALGTHM, SPEECH): differ-cluster `[0..21] ∪ [65..79]` (boundary-only).
- high-energy (FIXED, PITCH): differ 가 interior `[40..64]` 포함 → **sample 40 = subframe-1 → subframe-2 boundary onset** = postfilter inter-subframe state propagation 균열의 강력 signal.

### 0.2 누적 폐기 catalog (16 Phase 1k + 4 Phase 0c sub-hypothesis)

| cycle | sub-hypothesis | verdict |
|-------|----------------|---------|
| F-oct-postfix-1 | M1 (LP→postfilter chain bias) | spec-측정 RED 잔존 |
| F-oct-postfix-2 | M2 (postfilter coefficient quantization) | spec 정합 폐기 |
| F-oct-postfix2-prelim-1 | M1' (postfilter HP coefficient) | 폐기 |
| F-oct-postfix2-prelim-2 | M3 (AGC gain Q-format) | 폐기 |
| F-oct-postfix2-prelim-3 | M5 (PST byte-level want=`ff ff ff ff ff ff`) | 폐기 |
| F-oct-postfix2-prelim-4 | M6 (sPf/HP/×2 chain trace) | 폐기 |
| F-oct-postfix2-prelim-5 | Cα(prelim) (fcb pulse) sample 5..7 | 폐기 |
| F-oct-postfix2-prelim-6 | Z (PST chain post-AGC+HP+×2 §A.4.2.5) | 폐기 |
| F-non-prelim-1 | Y (a[0..10] sign 11/11) | sign 정합, magnitude max\|Δ\|=6 잔존 |
| F-non-prelim-2 | forced (-u) → syn(-u) = -syn(+u) linearity | 폐기 |
| F-non-prelim-X-split-1 | Cα fcb pulse sample 5..7 | 폐기 |
| F-non-prelim-X-split-2 | Cβ gain g_c sample 5..7 | 폐기 |
| F-non-Cgamma-revisit-1 | G-1 postfilter 4 sub-stage sample 5..7 | 폐기 |
| F-non-Cgamma-revisit-2 | G-2 synth IIR memory + Y magnitude small perturbation | 폐기 |
| **P0c-1** | PST format / endianness / header / unit | EQ_ALL (4 차원) |
| **P0c-2** | want chain stage 식별 (4 stage) | S*=postX2 (현 가정 holds) |
| **P0c-3** | cross-vector Δ 패턴 | NEW 시나리오 식별 (boundary/interior split) |
| **P0c-4** | synthesis (4-시나리오 트리) | NEW (P0c-inter-subframe-postfilter-state) 확정 |

**누적 결함 식별 0건** (16 Phase 1k + 4 Phase 0c). spec-internal mechanism 후보 = NEW (P0c-inter-subframe-postfilter-state).

### 0.3 ALGTHM frame 0 sf0 + cross-vector 측정-state table (Phase 0c synthesis carry)

| 항목 | 값 | 출처 cycle |
|------|------|-----------|
| g_p (Q14) | +1995 | F-non-prelim-X-split-2 |
| g_c (Q1) | +4153 | F-non-prelim-X-split-2 |
| v[0..4] (adaptive cb) | 0 | F-non-prelim-1 |
| c[0..3] (fcb pulse, Q13) | +8192 each | F-non-prelim-X-split-1 |
| u[0..3] (excitation) | +1 each | F-non-prelim-1 |
| u[4..7] | 0 | F-non-prelim-1 |
| syn[5..7] | `[+1, +1, +1]` | F-non-prelim-1 |
| sPf[5..7] | `[+1, +1, +1]` | F-oct-postfix2-prelim-4 |
| post-HP (PST) | `[+2, +2, +2]` | F-oct-postfix2-prelim-4 |
| **want (ALGTHM.PST byte-level)** sample 5..7 | `[−1, −1, −1]` | F-oct-postfix-1 / M5 byte-verify `cb9529d` |
| Δ (production − want) sample 5..7 | uniform **+3** | P0c-2 |
| ALGTHM differ-cluster (frame 0) | `[0..21] ∪ [65..79]` | P0c-3 |
| SPEECH differ-cluster (frame 0) | `[0..21] ∪ [65..79]` (low-energy boundary-only) | P0c-3 |
| FIXED differ-cluster (frame 0) | interior `[40..64]` 포함 | P0c-3 |
| PITCH differ-cluster (frame 0) | interior `[40..64]` 포함 | P0c-3 |
| Z (PST chain) | post-AGC + post-HP + post-×2 (§4.2 + §A.4.2.5) | F-oct-postfix2-prelim-6 |
| S* (argmin chain stage) | `postX2` (P0c-2) | P0c-2 |

**핵심 패턴**: 
- low-energy → boundary-only differ → frame edge (HP filter frame init) 후보.
- high-energy → interior differ from i=40 → subframe boundary (postfilter sub-state carryover) 후보.
- sample 40 = sf-1 (`[0..39]`) → sf-2 (`[40..79]`) 경계.

### 0.4 Invariant E1-E5 재확인 (carry, 강압-적합 회피)

- **E1**: 외부 G.729 구현 0건 참조. spec source = `docs/superpowers/specs/itu/G729E.pdf` + `testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt` + 교과서 (Kondoz, Spanias) only. **금지**: ITU reference C, bcg729, Sipro, FFmpeg G.729, Annex A binary 일체.
- **E2**: production 변경 0 라인 (측정 only). 본 cycle 3 task 모두 진단 test 추가만. 결함 식별 시 **별도 fix cycle**.
- **E3**: F-oct-postfix-1 RED (gate 17) 영구 잔존 — fix 시점 = mechanism 식별 후 별도 cycle.
- **E4**: 측정값과 spec 비교 시 **PDF/README verbatim 인용 의무**. cherry-pick 금지. 모든 verdict = `EQ` / `NE` 이진. UNDETERMINED 는 spec 진정 모호 시에만.
- **E5**: 자동 promotion 0 — 측정-only test 는 회귀 게이트 자동 등재 금지. Task 3 synthesis 결정 후 **명시 사용자 게이트** 통해 promotion.

**강압-적합 회피 절차** (재확인):
1. 측정 결과가 PDF/README mismatch = production bug 후보. mismatch 가 spec scope 밖 = sub-hypothesis 폐기.
2. **금지**: "거의 정합" / "범위 내 변동" / "carryover policy 가 자연스러우니 spec 정합". 모든 sub-state 는 PDF verbatim 인용된 carryover/reset 정책과 EQ/NE 이진 비교.
3. **금지**: PDF §4.2.x 모호 paragraph (예: "the postfilter state is updated" 의 carryover 명시 부재) 를 우리 구현 정당화로 사용. 모호 지점 = 별도 sub-hypothesis 분리.

### 0.5 누적 contract test gate (19건)

| # | gate | 상태 | 출처 |
|---|------|------|------|
| 1..16 | (Phase 1a~1j 누적) | PASS | 누적 |
| 17 | F-oct-postfix-1 ALGTHM.PST sample 5..7 부호 일치 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`, `internal/decoder/stagef_octpostfix_regression_test.go`, commit `56caa72`) | **RED 잔존** | F-oct-postfix-1 |
| 18 | F-non-prelim-X-split measurement bundle (Cα fcb + Cβ gain) | PASS | F-non-prelim-X-split (`aa9dcf9`) |
| 19 | P0c-reentry measurement bundle (P0c-1/2/3) | **pending** (E5 사용자 게이트 미수행) | P0c-reentry |

회귀 게이트 commit 직후 검증:
- `go vet ./...` clean.
- 누적 18 gate PASS/FAIL dump (1..16 + 18 PASS, 17 RED 잔존).
- 19번 = pending (E5 게이트 대기, P0c-reentry bundle).
- 본 cycle 신규 gate 0건 (측정-only, E5 자동 promotion 금지 → HP-1/2 합쳐 잠정 gate 20번 = pending, Task 3 synthesis 결정 후 promotion 여부 사용자 합의).

### 0.6 Working tree 보존 명시

- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) **변경 금지** — Phase 1k 부터 보존된 진단 test, 본 cycle scope 외.
- 본 cycle commit 시 `git status` 에 해당 파일 untracked 상태 그대로 유지 확인.

### 0.7 NEW: 본 cycle hypothesis 진술

**Primary hypothesis** (HP-1):
> Postfilter chain (long-term `Hp` memory + short-term `Hf` memory + tilt γ_t state + AGC gain state) 의 subframe-1 → subframe-2 경계 (sample 39 → sample 40) 에서 **state carryover / reset 정책 ≠ §4.2.x verbatim 정의**. 균열은 high-energy vector (FIXED/PITCH) interior `[40..64]` Δ 의 직접 mechanism.

**Secondary hypothesis** (HP-2):
> §A.4.2.5 HP filter frame 경계 state init (frame 0 첫 호출 시 x[i-1], x[i-2], y[i-1], y[i-2] 초기값) 정책 ≠ verbatim 정의. 균열은 low-energy vector (ALGTHM/SPEECH) `[0..21]` boundary-cluster Δ 의 직접 mechanism. (`[65..79]` late-transient 도 동일 mechanism 가능.)

**핵심 production state 식별** (spec verbatim 인용 시 사용할 field 이름, `internal/postfilter/types.go` + `internal/decoder/types.go` 확인):

| sub-state | spec 명칭 | production field | 위치 |
|-----------|-----------|------------------|------|
| long-term postfilter residual memory | `Hp` (§4.2.1, A.4.2.1) | `Postfilter.pastResidual [pitchMax+subframeLen]int16` | `internal/postfilter/types.go:15` |
| short-term postfilter input memory | `Hf` (§4.2.2, A.4.2.2) | `Postfilter.pastS [lpcOrder]int16` | `internal/postfilter/types.go:14` |
| short-term postfilter output memory | `Hf` (synth side) | `Postfilter.pastSynthPost [lpcOrder]int16` | `internal/postfilter/types.go:16` |
| tilt filter state | γ_t / past tilt input (§4.2.3, A.4.2.3) | `Postfilter.pastTiltInput int16` | `internal/postfilter/types.go:17` |
| AGC gain state | g(n-1) (§4.2.4, A.4.2.4) | `Postfilter.agcGainPrev int32` (Q24) + `initialized bool` | `internal/postfilter/types.go:22-23` |
| HP filter x-state | x[n-1], x[n-2] (§A.4.2.5) | `Decoder.hpX [2]int16` | `internal/decoder/types.go:31` |
| HP filter y-state | y[n-1], y[n-2] (§A.4.2.5) | `Decoder.hpY [2]int32` (Q12) | `internal/decoder/types.go:32` |

---

## Phase 1 — Hypothesis Tree (inter-subframe postfilter state + HP edge)

```
F-non-Hpost (NEW (P0c-inter-subframe-postfilter-state) 구체화, 사용자 G-XS4)
├── HP-1 (subframe boundary postfilter state, sf-1 → sf-2 at i=39 → i=40)
│   ├── pastResidual (Hp)        end-sf-1 vs start-sf-2 → carryover/reset?
│   ├── pastS / pastSynthPost (Hf) end-sf-1 vs start-sf-2 → carryover/reset?
│   ├── pastTiltInput (γ_t state)  end-sf-1 vs start-sf-2 → carryover/reset?
│   └── agcGainPrev (AGC g(n-1))  end-sf-1 vs start-sf-2 → carryover/reset?
├── HP-2 (HP filter §A.4.2.5 frame edge state)
│   ├── hpX[0], hpX[1] (x-state) frame 0 init → spec mandated value vs production zero-init?
│   ├── hpY[0], hpY[1] (y-state) frame 0 init → spec mandated value vs production zero-init?
│   └── early-transient `[0..21]` + late-transient `[65..79]` correlation
└── HP-3 (synthesis, 3-시나리오 결정 트리)
    ├── (Hpost-state-defect)  HP-1 ≥1 NE  → fix cycle on inter-subframe postfilter state
    ├── (HP-edge-defect)      HP-1 EQ + HP-2 NE → fix cycle on §A.4.2.5 HP filter init
    └── (Hpost-refute)        HP-1 + HP-2 모두 EQ → alternative path (Phase 1g multi-frame state, 또는 Cγ chain elsewhere)
```

**기대 entropy** (사전):
- (Hpost-state-defect) ≈ 50% — high-energy interior `[40..64]` Δ 의 직접 mechanism, P0c-3 strong signal.
- (HP-edge-defect) ≈ 30% — low-energy boundary `[0..21]` ∪ `[65..79]` 의 직접 mechanism, P0c-3 보조 signal.
- (Hpost-refute) ≈ 20% — 16+4 cycle 누적 결함 0건 base rate (Phase 1k Cγ-refute pattern).

---

## Phase 2 — Task 분해 (3 task, TDD 측정-only)

### Task 1: HP-1 — Subframe boundary postfilter state carryover/reset trace (sf-1 → sf-2 at i=40)

**목적**: postfilter 4 sub-state (Hp, Hf, γ_t, AGC) 가 subframe-1 종료 (sample 39 처리 직후) 와 subframe-2 시작 (sample 40 처리 직전) 사이에 **carryover (default per ITU continuous-state postfilter)** 인지 **reset to zero** 인지 production 측정 + spec verbatim 비교.

**선행 측정 부재**: 직전 16+4 sub-hypothesis 모두 sf-1 single-subframe 또는 sample 5..7 한정 측정. subframe 경계 state propagation cross-section 측정 0건.

**PDF verbatim 인용 의무** (Task 1 첫 단계, `docs/superpowers/specs/itu/G729E.pdf`):
- §4.2.1 (long-term postfilter Hp memory) — Hp 의 subframe 경계 state 정책 verbatim 인용.
- §4.2.2 (short-term postfilter Hf memory) — Hf 의 subframe 경계 state 정책 verbatim 인용.
- §4.2.3 (tilt γ_t state) — past tilt input / γ_t gating 의 subframe 경계 정책 verbatim 인용.
- §4.2.4 (AGC gain state) — g(n-1) 의 subframe 경계 정책 verbatim 인용.
- (Annex A 변형) §A.4.2.1~§A.4.2.4 — Annex A simplification 가 carryover policy 를 변경하는지 verbatim 확인.

**TDD 절차**:
1. **RED**: `internal/postfilter/phase1l_hp_subframe_boundary_diagnostic_test.go` 신규 — `TestDiagnostic_Phase1lHp1SubframeBoundaryTrace`.
   - sub-test 3건 (ALGTHM / FIXED / PITCH) — low-energy + 양 high-energy.
   - 각 vector frame 0 에 대해:
     - sf-1 (sample 0..39) decode 직후 4 sub-state snapshot:
       - `Hp_end_sf1`  = `pf.pastResidual[pitchMax-39 .. pitchMax+40]` 영역 (sf-1 직후 실제 채워진 마지막 슬롯) 또는 `pf.pastResidual[:]` 전체.
       - `Hf_end_sf1` = `pf.pastS[:]` + `pf.pastSynthPost[:]`.
       - `γt_end_sf1` = `pf.pastTiltInput`.
       - `AGC_end_sf1` = `pf.agcGainPrev` (Q24) + `pf.initialized`.
     - sf-2 (sample 40..79) decode 직전 (즉 sf-1 직후 시점) 동일 4 sub-state snapshot:
       - 동일 field, 동일 vector dump.
     - sf-2 decode 직후 동일 4 sub-state snapshot (carryover 후 갱신 검증용).
   - delta dump (`Δ = state_start_sf2 - state_end_sf1`):
     - Δ = 0 (모든 entry) → **carryover** verdict.
     - state_start_sf2 = zero vector → **reset** verdict.
     - 둘 다 아님 → **partial / unspecified** verdict.
   - spec verbatim 비교 (`expectedPolicy`):
     - PDF §4.2.x 인용에서 명시한 carryover/reset 정책 추출.
     - production verdict vs expectedPolicy 4 sub-state EQ/NE 4-tuple.
   - classifier `classifyHp1State(vector, substate) (productionPolicy, specPolicy, verdict)`.
   - hard assertion: `if len(produced) != 80 { t.Fatalf(...) }` 만. sub-state 값 강요 금지.
2. **GREEN**: production 변경 0 (E2). test = 측정 only + log.
3. **dump 확인**: 3 vector × 4 sub-state × {carryover EQ, reset NE, partial NE} verdict matrix.
4. **commit**:
   ```
   test(postfilter): add Phase 1l HP-1 subframe boundary state diagnostic

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

**측정 의무** (1줄): PDF §4.2.1~§4.2.4 (+ §A.4.2.1~§A.4.2.4) verbatim 인용 하 ALGTHM + FIXED + PITCH frame 0 sf-1→sf-2 경계 4 sub-state (Hp/Hf/γ_t/AGC) carryover-vs-spec EQ/NE 12-cell verdict matrix.

**polarity expectation**:
- ITU continuous-state postfilter 관행상 4 sub-state 모두 carryover 기대 (default).
- spec 가 carryover 명시 시 → production carryover 확인 시 EQ.
- spec 가 carryover 명시 + production 이 reset 또는 partial → NE → (Hpost-state-defect) 식별.
- spec 가 명시 없음 (모호) → E4 분리 sub-hypothesis (carryover 가 합리적이지만 verbatim 부재 → spec ambiguity 분류).

**escape hatch**: 12-cell 모두 EQ → HP-1 폐기 → Task 2 진행. ≥1 NE → HP-1 evidence 확정 + Task 2 보조 측정 + Task 3 (Hpost-state-defect) 시나리오 dispatch.

---

### Task 2: HP-2 — HP filter (§A.4.2.5) frame-edge state trace

**목적**: §A.4.2.5 HP filter 의 frame 경계 (frame 0 첫 호출 시) state init 정책이 production zero-init 과 EQ/NE 인지, 그리고 frame 0 의 early-transient `[0..21]` 와 late-transient `[65..79]` 영역의 sample-level Δ 가 HP filter state 초기화 / 누적 효과와 correlate 하는지 측정.

**선행 측정 부재**: 직전 측정에서 HP filter sample 5..7 한정 byte/value EQ 검증만 수행. frame 0 80 sample 전체에 대한 HP filter input/output/state 시계열 dump 0건. 특히 frame 0 첫 호출 시 `hpX[0]=hpX[1]=0`, `hpY[0]=hpY[1]=0` 이 §A.4.2.5 verbatim 명시인지 미확인.

**PDF verbatim 인용 의무** (Task 2 첫 단계):
- §A.4.2.5 verbatim 인용:
  - HP filter transfer function (b0/b1/b2/a1/a2 verbatim).
  - HP filter init paragraph: "what does the spec say about state x[n-1], x[n-2], y[n-1], y[n-2] at frame 0 / first-call?"
  - ×2 multiplier 적용 단계 (post-HP scaling).
- §4.2.2 (output HP filter, frame 단위 호출 cadence) verbatim — 본 production 의 §A.4.2.5 적용이 frame-rate 인지 subframe-rate 인지 확인.

**TDD 절차**:
1. **RED**: `internal/decoder/phase1l_hp_edge_diagnostic_test.go` 신규 — `TestDiagnostic_Phase1lHp2EdgeTrace`.
   - sub-test 2건 (ALGTHM + SPEECH) — low-energy boundary-cluster vector.
   - 각 vector frame 0 에 대해:
     - HP filter input dump: `sPf[0..79]` (postfilter chain 종료, HP 입력).
     - HP filter state 시계열 dump (sample-level):
       - sample 0..21 영역: HP filter call 직전 `hpX[0], hpX[1], hpY[0], hpY[1]` + call 직후 동일 4 field + output `postHP[i]`.
       - sample 65..79 영역: 동일 4 field + output dump.
     - HP output postHP[0..79] 전체 dump.
     - want 비교: `want[i] = readPSTFrames("ALGTHM.PST")[0][i]` (또는 SPEECH).
     - production PST = `postHP[i] × 2` (§A.4.2.5 step 2).
     - Δ[i] = production[i] − want[i] for i ∈ [0..21] ∪ [65..79].
   - early/late transient correlation:
     - early-transient `[0..21]`: HP state init 영향 dominant region (state 초기 0 → impulse-response transient).
     - late-transient `[65..79]`: subframe-2 후반, HP state 누적 효과.
     - correlation hypothesis: Δ[0..21] sign / magnitude 패턴이 spec-mandated nonzero init 과의 차이로 설명 가능한지.
   - spec verbatim vs production:
     - PDF §A.4.2.5 init paragraph 인용 → "frame 0 첫 호출 시 state = ?" verbatim 명시 추출.
     - production = zero-init (`Decoder` 의 zero value). 
     - verdict EQ (spec 도 zero-init 명시) / NE (spec 가 nonzero init 명시) / spec 모호 → E4 분리.
   - classifier `classifyHp2EdgeState(vector, region) (productionInit, specInit, transientPattern, verdict)`.
     - region = early `[0..21]` / late `[65..79]`.
   - hard assertion: `len(produced) != 80` 만. state 값 강요 금지.
2. **GREEN**: production 변경 0 (E2).
3. **dump 확인**: 2 vector × 2 region (early/late) × {init EQ/NE, transient correlate Y/N} verdict matrix.
4. **commit**:
   ```
   test(decoder): add Phase 1l HP-2 HP filter frame-edge diagnostic

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

**측정 의무** (1줄): PDF §A.4.2.5 verbatim 인용 하 ALGTHM + SPEECH frame 0 HP filter input/state/output 시계열 dump + early `[0..21]` + late `[65..79]` Δ correlation EQ/NE 4-cell verdict matrix.

**polarity expectation**:
- §A.4.2.5 이 zero-init 명시 → production EQ → HP-2 폐기.
- §A.4.2.5 이 nonzero init 명시 (예: prev-frame y 값 carryover) + production zero-init → NE → (HP-edge-defect) 식별.
- transient correlate Y + state init NE → HP-edge-defect 강력 후보.
- transient correlate Y + state init EQ → HP filter 외부 mechanism (예: postfilter chain 의 boundary 효과가 HP 입력에 누적).

**escape hatch**: 4-cell 모두 EQ → HP-2 폐기 → Task 3 (Hpost-refute) 또는 (Hpost-state-defect) verdict. ≥1 NE → (HP-edge-defect) evidence 확정.

---

### Task 3: HP-3 — Synthesis + 3-scenario decision tree

**목적**: HP-1 + HP-2 verdict 결합 → mechanism 식별 또는 후보 고갈 결정 + 사용자 게이트 권고.

**선행 의무**: HP-1 + HP-2 commit 완료 후 dispatch (ad-hoc, 본 plan 내 Task 3 으로 명시 또는 별도 dispatch — 본 cycle 에서는 plan 내 명시).

**TDD 절차**: synthesis = report-only, test 추가 없음.
1. report 작성: `docs/superpowers/plans/2026-05-06-phase1l-stage-f-non-hpost-synthesis-report.md`.
   - HP-1 verdict matrix 12 cell 요약.
   - HP-2 verdict matrix 4 cell 요약.
   - 3-시나리오 결정 트리 적용.
   - 차기 cycle 권고 + 사용자 게이트 G-XS5 양식.
2. **commit**:
   ```
   docs(plans): F-non-Hpost synthesis + Phase 1l decision

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

**3-시나리오 결정 트리**:

| 시나리오 | 조건 | 다음 cycle | 누적 cycle 추정 |
|---------|------|-----------|-----------------|
| **(Hpost-state-defect)** | HP-1 12-cell ≥1 NE | postfilter sub-state fix cycle (별도, 1~2 cy) | 본 cycle + fix = 2~3 cy 종결 |
| **(HP-edge-defect)** | HP-1 EQ_ALL + HP-2 ≥1 NE | §A.4.2.5 HP filter init fix cycle (별도, 1 cy) | 본 cycle + fix = 2 cy 종결 |
| **(Hpost-refute)** | HP-1 + HP-2 모두 EQ | alternative path (b) Phase 1g multi-frame state 진입, 또는 (c) Cγ chain elsewhere 재방문 | 본 cycle 종결 + 별도 plan |

**(Hpost-refute) 시 권고 alternative path**:
- (b) Phase 1g (decoder integration) multi-frame state 진단 — frame 0 specific 누적 state 측정 (특히 prev_frame 초기값 / multi-frame propagation).
- (c) Cγ chain elsewhere — F-non-Cgamma-revisit 의 G-1/G-2 폐기 결과 재검토 (특히 SF-1 tilt γ_t gating side finding 과 결합 측정).

**측정 bundle promotion** (시나리오 별):
- (Hpost-state-defect) / (HP-edge-defect): 측정 bundle = mechanism 식별 evidence → fix cycle 후 promotion (gate 20 PASS 등재).
- (Hpost-refute): 측정 bundle = NEW 시나리오 폐기 evidence → promotion 검토 (E5 자동 promotion 금지 → 사용자 G-XS5 합의 후).

---

## Phase 3 — 회귀 게이트 (각 commit 직후)

각 task commit 직후 실행:
1. `go vet ./...` — clean 필수.
2. 누적 19 gate dump:
   - 1..16 PASS (변동 없음).
   - 17 **RED 잔존** (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`).
   - 18 PASS (F-non-prelim-X-split bundle).
   - 19 pending (P0c-reentry bundle, E5 게이트 미수행).
3. 본 cycle 신규 측정 test 2건 = 모두 측정-only, 회귀 게이트 자동 등재 금지 (E5).
4. test 실행 명령:
   - Task 1: `go test ./internal/postfilter/ -run Phase1lHp1 -v`
   - Task 2: `go test ./internal/decoder/ -run Phase1lHp2 -v`
   - 누적: `go test ./...` (RED 17 잔존 확인, 본 cycle 신규 test PASS 확인).

---

## Phase 4 — Escape hatch E1-E5

| code | 발동 조건 | 행동 |
|------|----------|------|
| E1 | 외부 G.729 구현 참조 유혹 (ITU reference C, Annex A binary, bcg729, Sipro, FFmpeg) — 특히 carryover policy 모호 시 외부 구현 대조 욕구 | 즉시 차단. spec source = PDF + READMETV.txt + textbooks (Kondoz, Spanias) only. 모호 시 = 별도 sub-hypothesis (E4). |
| E2 | production 변경 유혹 (HP-1 NE 발견 시 postfilter state reset 추가, HP-2 NE 발견 시 hpX/hpY init 변경) | 즉시 차단. fix = 별도 cycle (Hpost-state-fix / HP-edge-fix). |
| E3 | gate 17 RED 잔존을 task failure 로 간주하려는 유혹 | 차단. 17 = mechanism 식별 후 별도 fix cycle (Task 3 synthesis 시 결정). |
| E4 | PDF §4.2.x carryover policy 가 명시 부재 시 "ITU 관행상 carryover 가 자연스러우니 production EQ" 로 cherry-pick 하려는 유혹 | 차단. 모호 = 명시적 모호 분류 + verbatim 인용 + 별도 sub-hypothesis 분리. |
| E5 | 측정-only test 자동 promotion 유혹 (HP-1/HP-2 commit 시 회귀 게이트 등재) | 차단. promotion = Task 3 synthesis 결정 후 사용자 G-XS5 명시 게이트. |

---

## Phase 5 — Self-review (작성자)

- ✅ Phase 0 (Phase 1k 종결 + Phase 0c 재진입 정리 + 16+4 누적 sub-hypothesis catalog) 포함.
- ✅ Phase 0.3 measured-state table (cross-vector boundary/interior split 추가).
- ✅ Phase 0.4 강압-적합 회피 절차 (carryover policy 모호 cherry-pick 차단).
- ✅ Phase 0.5 19 gate 상태 (1..16 PASS, 17 RED, 18 PASS, 19 pending).
- ✅ Phase 0.6 untracked file (`stagef_bis_diagnostic_test.go`) 보존 명시.
- ✅ Phase 0.7 hypothesis 진술 (primary inter-subframe postfilter state + secondary HP edge transient) + production state field 식별표.
- ✅ Task 3개 분해 (HP-1 sub-state carryover / HP-2 HP edge / HP-3 synthesis).
- ✅ 각 task TDD (RED→GREEN→commit, production 0 = E2).
- ✅ 각 task commit 메시지 양식 + Co-authored-by trailer 명시 (verbatim).
- ✅ 회귀 게이트 19건 + 본 cycle 신규 게이트 자동 등재 금지 (E5).
- ✅ Escape hatch E1-E5 (Hpost 특수 trigger 명시).
- ✅ 외부 G.729 구현 0건 참조 (E1).
- ✅ production 변경 0 라인 (E2).

**위험 요소**:
- (R-A) §4.2.1~§4.2.4 carryover policy paragraph 가 명시 부재 (PDF 가 단지 "the postfilter state is updated for the next subframe" 식 일반 언급) → E4 별도 sub-hypothesis 분리. 본 cycle 에서 spec ambiguity 발견 시 자체적으로 (Hpost-state-defect) 도 (Hpost-refute) 도 아닌 verdict (UNDETERMINED) 가능 — 단 E4 의 "genuine spec ambiguity" 기준 충족 시에만.
- (R-B) HP-1 측정에서 `pf.pastResidual` 의 sf-1 직후 vs sf-2 직전 snapshot 이 동일 (carryover verdict) 인 것은 production code 의 자연스러운 상태이지만, `applyLongTerm` 호출 직후 `slide` 가 발생하면 snapshot timing 이 ambiguous → snapshot timing 명시 의무 (test 작성 시 inline doc).
- (R-C) HP-2 측정에서 §A.4.2.5 init paragraph 가 verbatim "the filter state is initialized to zero" 명시면 production EQ → HP-2 폐기. 만약 그런 명시 부재 시 spec ambiguity (zero-init 가 default 가정인지 아닌지 확인 의무).
- (R-D) FIXED/PITCH vector frame 0 decode 시 첫 frame parameter 가 silence frame 일 가능성 → high-energy 라는 P0c-3 분류가 frame 0 한정 valid 인지 재확인 의무 (HP-1 first 단계).

---

## Phase 6 — Execution Handoff

**다음 dispatch**: `HP-1` (Task 1, subframe boundary postfilter state carryover trace).

**선행 의무 (dispatch 직전)**:
1. Phase 0.5 19 gate baseline 확인 (`go test ./...` + 17 RED 잔존 + 본 cycle 신규 test 0건).
2. `internal/postfilter/postfilter.go` `Filter()` 호출 표면 + `Postfilter` field 접근 권한 (package-internal test 위치) 확인.
3. PDF §4.2.1~§4.2.4 + §A.4.2.1~§A.4.2.4 carryover policy paragraph 위치 사전 확인 (verbatim 인용 준비).
4. `testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt` 의 FIXED.PST / PITCH.PST 설명 재독 (high-energy frame 0 가정 검증).

**완료 trigger**: HP-2 commit 직후 → HP-3 synthesis dispatch (본 plan 내 Task 3 또는 ad-hoc) → 3-시나리오 verdict + 사용자 게이트 G-XS5.

---

## Phase 7 — 종료 + 3-scenario 결정 트리 (Task 3 reference)

**Task 3 (HP-3 synthesis)**: HP-1 + HP-2 verdict 결합 → 3-시나리오 결정 트리 적용 → 차기 cycle 권고 + 사용자 G-XS5.

**3-시나리오 결정 트리** (Task 3 시점 적용):

| 시나리오 | 조건 | 다음 cycle |
|---------|------|-----------|
| **(Hpost-state-defect)** | HP-1 ≥1 NE | postfilter inter-subframe state fix cycle |
| **(HP-edge-defect)** | HP-1 EQ_ALL + HP-2 ≥1 NE | §A.4.2.5 HP filter init fix cycle |
| **(Hpost-refute)** | HP-1 + HP-2 모두 EQ | alternative path (b Phase 1g multi-frame / c Cγ chain elsewhere 재방문) |

**Task 4 reference**: 본 plan 종료 = HP-3 synthesis commit (`docs(plans): F-non-Hpost synthesis + Phase 1l decision`). Task 3 = HP-3 synthesis 자체 (별도 Task 4 없음 — 본 cycle 의 synthesis 가 Task 3).

---

**Plan 종료.** 본 commit = `F-non-Hpost` cycle 0번째 (plan-only) commit. 다음 commit = HP-1 (`test(postfilter): add Phase 1l HP-1 subframe boundary state diagnostic`).

---

## Task 진행 status

- [x] Task 1 — HP-1 (subframe boundary postfilter state carryover/reset, 3 vector × 4 sub-state) — completed (`076b6de`).
- [x] Task 2 — HP-2 (§A.4.2.5 HP filter frame-edge state, 2 vector × 2 region) — completed (`2ee0009`).
- [x] Task 3 — HP-3 (synthesis, 3-시나리오 결정 트리) — completed; **(Hpost-refute)** 확정 → Phase 1l 잠정 종결. 보고서: `2026-05-06-phase1l-stage-f-non-hpost-synthesis-report.md`.
