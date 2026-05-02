# Phase 1k Stage F-non-prelim 종합 보고서 + 다음 cycle 결정

**작성일**: 2026-05-03
**범위**: F-non-prelim cycle 의 Task 1 (X — excitation u[0..4] sub-항 분해), Task 2 (Y — LP a[0..10] cross-check), Task 3 (Z — postfilter chain spec 정합 재인용) 측정 결과 결합 분석. 단일 source 식별 + 4 후보 (Cα/Cβ/Cγ/Cδ) 비교평가 + 다음 cycle 단일 결정.
**산출물**: 7-section 보고서 (§0~§6). production / test 변경 0 라인 (메타 task). 외부 G.729 구현 0 인용 (G1 결정 정합). `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경 (Phase 0.5).
**Plan 출처**: `docs/superpowers/plans/2026-05-03-phase1k-stage-f-non-prelim-plan.md` (commit `658090b`) §Task F-non-prelim-4.

---

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)

### 0.1 Working tree (Task 4 진입 직전)

```
$ git log --oneline -5
dd4e21a (HEAD -> main) docs(plans): add Stage F-non-prelim-3 Z spec interpretation review
d1a4f2d test(decoder): add Stage F-non-prelim-2 Y LP a[] cross-check
f82893d test(decoder): add Stage F-non-prelim-1 X excitation sub-term decomposition
658090b docs(plans): add Phase 1k Stage F-non-prelim plan
9a5a7f6 docs(plans): F-oct-postfix2-prelim synthesis + cycle decision

$ git status (Task 4 진입 직전)
Untracked files:
  internal/decoder/stagef_bis_diagnostic_test.go
nothing added to commit (working tree clean)
```

→ Phase 0.5 (사전 보유 working tree 보존) 충족 — `stagef_bis_diagnostic_test.go` untracked 보존, 변경 0.

### 0.2 escape hatch 평가 (E1–E5)

| Hatch | 정의 | 본 Task 4 발동 여부 | 근거 |
|-------|------|---------------------|------|
| **E1** | spec § 인용 부재 / 부정합 시 plan 후속 정정 의무 | **미발동** | Task 1~3 의 §A.4.1 / §A.3.2/3 / §A.4.2.* / §4.2 / §4.3 Table 9 인용 모두 검증 완료 (Task 1 §0.2 의 §A.3.5→§4.1.5+§4.1.6 정정만 발생, 본 Task 4 추가 발동 0). |
| **E2** | 측정 도구 결함 (replication mismatch / Q-format 부정합) 시 측정 reset | **미발동** | Task 1 §2.2 replication match=true (40 sample), Q15 round 식 검증 PASS. |
| **E3** | 2+ 후보 잔존 시 추가 진단 cycle 진입 | **미발동** | §3 결과: 단일 source (fcb codebook c[0..3] 양 입력) 식별 완료. 4 후보 (Cα/Cβ/Cγ/Cδ) 중 Cα + Cβ 만 잔존하고 두 후보가 *동일 source 의 두 fix layer* 이므로 hybrid 권고로 단일 결정 (§4 참조). |
| **E4** | 외부 G.729 구현 (Annex A binary, 3rd-party Go/C 등) 인용 시 즉시 cycle 중단 | **미발동** | 본 Task 4 인용 = (a) ITU-T G.729 (06/2012) PDF, (b) READMETV.txt, (c) F-non-prelim-1/2/3 본 cycle commit (`f82893d`/`d1a4f2d`/`dd4e21a`) + 직전 cycle synthesis (`9a5a7f6`/`8907847`/`8f693b7`) + M6 (`cb9529d`). 외부 G.729 구현 0건. |
| **E5** | Phase 0.4 강압-적합 회피 위반 (측정 미보유 후보의 강압적 dismiss / 우선 가설 강요) 감지 시 보고서 reset | **미발동** | Cγ "spec 정합" 은 Task 3 의 7개 chain order + 8개 PST 비교 도메인 의문 verbatim 정합 측정으로 결론 (강압 dismiss 아님). Cδ "stimulus 의문" 은 M6 (`cb9529d`) 반증과의 모순을 §3.5 에서 명시 분석 (강압 dismiss 아님). |

→ 5 escape hatch 모두 **미발동**. 본 Task 4 진행 정합.

### 0.3 회귀 게이트 baseline (Task 4 commit 직전, 본 commit 직후 재확인)

```
$ go vet ./...
(clean)

$ go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
--- PASS: TestDecode_Frame0Sample0_MatchesALGTHM (0.00s)

$ go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -v
    stagef_octpostfix_regression_test.go:38: frame 0 sample 5: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 6: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 7: got=2 want=-1 (Δ=3)
--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput  ← item 16 RED 잔존 (F-oct-postfix-1 명세대로)
```

→ vet clean + sample0 PASS + sample5..7 RED 잔존. 본 Task 4 의 production / test 변경 0 라인 의무에 의해 commit 직후 동일 결과 보장.

---

## 1. F-non-prelim cycle commit 요약 + cycle premise (G-N1 = X 우선)

### 1.1 cycle commit 표

| commit | task | 분류 | 주요 측정 / 결론 |
|--------|------|------|------------------|
| `658090b` | Plan | docs | F-non-prelim 4-task plan 등록 (X / Y / Z / synthesis). G-N1 = "(a) X 우선" 채택 명시. |
| `f82893d` | Task 1 (X) | test | 신규 `TestDiagnostic_FnonPrelimXExcitationSubterms`. **verdict X-fcb**: u[0..3]=+1 = `g_c·c` 단독, v[0..4]≡0 (zero past_exc), c[0..3]=+8192 (Q13 = +1.0), g_p=+1995 (Q14), g_c=+4153 (Q12). replication match = true. |
| `d1a4f2d` | Task 2 (Y) | test | 신규 `TestDiagnostic_FnonPrelimYLpACheck`. **verdict Y-magnitude**: a[0..10] sign-equal=11/11 vs §3.2.6 reference, forced a[1..10] sign-flip → syn[5..7] magnitude만 변화 (+1 → 0), 부호 flip 0/3. F-sept-2 L3 magnitude gap (max\|Δ\|=6) 잔존하나 *부호 source 와 직교* — 본 cycle 외 처리 의무 인계. |
| `dd4e21a` | Task 3 (Z) | docs | spec § verbatim catalog (§A.4.2.* + §4.2 + §4.2.5/§A.4.2.5 + §4.3 Table 9 + READMETV.txt) + production cross-ref. **verdict Z-confirm**: chain order 7항 + PST 비교 도메인 8항 모두 spec verbatim 정합, 차이 0건. PST = post-AGC + post-HP + post-×2 도메인 입증. |

### 1.2 cycle premise (G-N1)

F-oct-postfix2-prelim 종합 (`9a5a7f6`) §3 의 후보 분류:

- **X (excitation u 부호 source)** — 우선 진입 (G-N1 = "(a) X 우선 정합")
- **Y (LP a[] cross-check)** — 2순위
- **Z (postfilter chain spec 해석)** — LOW-cost 검토 (보고서 only)
- **W (PST 출처 / test vector 자체)** — *모두 반증* 시에만 진입 (Phase 0.4 §6 의무)

본 cycle 의 task 정의 = "u[0..4] 가 syn[5..7] self-feedback source" 라는 합성 IIR 추론 (M1' / M3 모두 REFUTED — `f04ec88`) 이후 *u[0..4] 의 부호가 어디서 오는가* 라는 한 단계 상류 질문 (Task 1 §4 cross-check).

### 1.3 측정 정량 요약

| 측정 항목 | 값 | 출처 commit |
|-----------|----|--------------|
| g_p (Q14) | +1995 (≈ +0.122) | `f82893d` §2 |
| g_c (Q12) | +4153 (≈ +1.014) | `f82893d` §2 |
| v[0..4] (pitch codebook, Q0) | `[0,0,0,0,0]` | `f82893d` §2 |
| c[0..4] (fcb codebook, Q13) | `[+8192,+8192,+8192,+8192,0]` | `f82893d` §2 |
| g_p·v[0..4] (Q15 pre-Round) | `[0,0,0,0,0]` | `f82893d` §2 |
| g_c·c[0..4] (Q15 pre-Round) | `[+33224,+33224,+33224,+33224,0]` | `f82893d` §2 |
| u[0..4] (Q0, composite) | `[+1,+1,+1,+1,0]` | `f82893d` §2 |
| Round 식 검증 | `(33224·2 + 32768) >> 16 = 1` ✓ | `f82893d` §2.1 |
| replication match (40 sample) | true | `f82893d` §2.2 |
| a[0..10] (Q12) | `[+4096,−2197,−375,−4,−144,−68,+303,−36,−90,+145,−33]` | `d1a4f2d` §2 |
| sign-equal vs §3.2.6 ref | 11/11 | `d1a4f2d` §3.1 |
| max\|Δ\| vs §3.2.6 ref | 6 (a[1]: −2197 vs −2203) | `d1a4f2d` §3.1 |
| forced a-sign-flip → syn[5..7] flip | 0/3 (부호 보존, magnitude만 변화) | `d1a4f2d` §3.1 |
| Z chain order 정합 (§A.4.2.* 7항) | 7/7 production 정합 | `dd4e21a` §3 |
| Z PST 비교 도메인 (8 의문) | 8/8 spec verbatim 정합 | `dd4e21a` §4 |
| 합성 결함 (item 16 RED) | got=`[+2,+2,+2]`, want=`[−1,−1,−1]`, Δ=3 | `56caa72` (F-oct-postfix-1) |

---

## 2. X / Y / Z 비교표 (단일 표, 측정 데이터만 — Phase 0.4 §1)

| 후보 | 측정 출처 | 핵심 측정값 | verdict | spec 인용 |
|------|-----------|-------------|---------|------------|
| **X** (excitation u[0..4] sub-항 부호 source) | Task 1 (`f82893d`) §2 + §3 | g_p·v[0..4] = `[0,0,0,0,0]`; g_c·c[0..3] = `+33224 (Q15)` 단독; u[0..3] = `+1`; 가법 분해 + replication match=true | **X-fcb (단일 식별)** — `g_c·c[n]` 단독으로 sample 0..3 양 입력 결정 | §4.1.5 + §4.1.6 eq.(75) + §A.4.1 (plan §A.3.5 인용은 PDF grep 결과 부정합 — Task 1 §0.2 정정) |
| **Y** (LP a[0..10] cross-check, sample 5..7 영역 한정) | Task 2 (`d1a4f2d`) §2 + §3 | sign-equal=11/11; forced a[1..10] sign-flip → syn[5..7] 부호 flip 0/3, magnitude만 변화 (+1 → 0) | **Y-magnitude (X-fcb 정합)** — a[] sign 은 §3.2.6 정합, 부호 source 아님; magnitude gap (max\|Δ\|=6) 은 부호 source 와 직교, 본 cycle 외 처리 | §A.3.2/3 + §A.4.1 + §3.2.6 (LSP→LP 변환) |
| **Z** (postfilter chain "정합" 정의 spec 해석) | Task 3 (`dd4e21a`) §3 + §4 + §5 | chain order 7항 (§A.4.2 cascade + §4.2 parent + §4.2.5 HP + §4.3 Table 9 init) production 정합 7/7; PST 비교 도메인 8 의문 spec verbatim 정합 8/8 | **Z-confirm (반증)** — spec 해석 / chain 종점 / 비교 도메인 결함 0; PST = post-AGC + post-HP + post-×2 입증 | §A.4.2.* (line 2226–2293) + §4.2 (line 1553–1564) + §4.2.5 (line 1687–1693) + §4.3 Table 9 (line 1695–1708) + §2 (line 384–395) + READMETV.txt (line 8–17, 21) |

→ X 단독 "유력" + Y "X-fcb 정합 보강" + Z "반증" → **단일 source 식별** 조건 충족 (plan §Task 4 Step 3 결정 트리 첫 행).

---

## 3. 단일 source 식별 + 4 후보 (Cα/Cβ/Cγ/Cδ) 평가표

### 3.1 단일 source 식별

X-fcb (단일) + Y-magnitude (X-fcb 정합) + Z-confirm (반증) 누적 = **단일 source = fcb codebook c[0..3] 양 입력**.

수식 chain (§4.1.6 eq.(75) + §A.4.1):

```
u[n] = Round( (g_p · v[n]) Q15 + (g_c · c[n]) Q15 )      n = 0..39
                       │                  │
                       │                  └─ c[0..3] = +8192 (Q13 = +1.0), g_c = +4153 (Q12 = +1.014) → +33224 (Q15) → Round → +1
                       └─ v[0..4] = 0 (zero past_exc, tInt=20, 첫 frame artefact) → 기여 0
```

→ 양 입력 origin 은 (a) `fcb.Decode` 의 c[n] 부호 결정, 또는 (b) `gain.Decoder.Decode` 의 g_c 부호 결정. 두 layer 모두 c[n] · g_c 의 *곱 부호* 에 영향.

### 3.2 4 후보 평가표

| 후보 | 정의 | priority | risk | spec-grounding | cost (예상 fix 라인) | 측정 정합 |
|------|------|----------|------|----------------|----------------------|-----------|
| **Cα** `fcb.Decode` (c[n] 부호 결정 결함) | `internal/fcb/decode.go:20` `Decode` — 4 pulse position + 4 sign + β·c[n−t] pitch enhancement. c[0..3]=+8192 자체가 결함이면 fcb 패키지 결함. | **HIGH** | MID — fcb 단독 unit-level fix 가능; 기존 fcb test (`internal/fcb/`) 영향 검토 필요. β·c[n−t] enhancement 가 sample 0..3 에 +8192 누적시킨 것인지, raw pulse 가 +8192 으로 placement 된 것인지 분리 미상 (Task 1 §3.1 의 한계 — "확정은 fcb/decode.go 검토 필요" 명시). | §3.8 / §4.1.5 (4-pulse ACELP positions/signs) + §3.8 (pitch enhancement β·c[n−t]) | 1~3 (sign 처리 또는 β enhancement 부호) | Task 1 §3.1 의 한계 단서 — c[n] origin 의 sub-decomposition 미수행 |
| **Cβ** `gain.Decoder.Decode` (g_c 부호 결정 결함) | `internal/gain/decode.go:38` `Decode` — gain VQ (GA1=5, GB1=6 → g_p, g_c). g_c=+4153 (Q12) > 0 자체가 결함이면 gain VQ 결함. | **HIGH** | MID — gain VQ 단독 unit fix 가능; 기존 gain test (`TestGainVQ_SampleEntries_MatchSpec` 등) 가 ROM-table 일치 검증 중이므로 결함 가능성은 *table lookup* 보다 *부호 처리 / predictor* 측에 가능성 높음. | §3.9 (gain VQ) + §A.3.9 (GA/GB indices) + §4.1.6 (g_p, g_c 적용) | 1~3 (sign 처리 또는 predictor) | Task 1 §2 의 g_c=+4153 raw 측정 제공; 하지만 `+4153 자체가 spec-canonical 값인지` ROM cross-check 미수행 (한계) |
| **Cγ** spec 정합 (production 자체는 결함 아님 — sample 5..7 한정 추가 mechanism 필요) | u[0..3]=+1 자체는 정합이고 syn[5..7]=+1 (got) ≠ −1 (want) 는 LP synthesis 1/Â(z) IIR 의 a[] coefficient × past synth memory 와 +1 입력의 합성 결과인데, 합성 결과 부호가 음수가 되어야 함을 *추가 mechanism* 으로 spec 이 명시한다는 가설. | **LOW** | HIGH — Task 2 Y-magnitude (forced a-flip → syn[5..7] magnitude만 변화, 부호 보존) + Task 3 Z-confirm (chain order 7/7 + PST 도메인 8/8 spec 정합) 으로 spec 측 추가 mechanism 0 입증. Task 3 §5 결론 verbatim: "**우리 현 가정과의 모순: 0건**". | §4.1.6 + §A.4.1 (LP synthesis filter) — 단, 추가 sign-flip mechanism 의 spec § 인용 측정 결과 = **0건** | 0~10 (가상 — spec § 식별 시) | Task 3 §5 verbatim 정합 + Task 2 forced-flip 결과 |
| **Cδ** stimulus (test vector / PST want) 자체 의문 | ALGTHM.PST byte 10..15 의 want=`[−1,−1,−1]` 또는 `algthm.bit` input encoding 자체 결함 가설. | **DISMISSED** | — | M6 (`cb9529d`) verdict 와의 **모순**: M6 §2 = ALGTHM.PST byte 10..15 = `0xFFFF 0xFFFF 0xFFFF` = int16 little-endian `[−1,−1,−1]` byte parsing 결함 0. M6 §3 multi-vector 분포 = `[−,−,−]` 다수 (ALGTHM/PITCH/FIXED 3건). M6 §4 verdict = "**M6 REFUTED** — PST want 부호 자체 정상, byte parsing/endianness/sign-extension 결함 없음. mismatch origin = production 출력측". | (해당 없음 — M6 반증 인용) | 0 (반증) | M6 (`cb9529d`) §2 + §3 + §4 |

### 3.3 Cδ 의 M6 반증과의 모순 분석 (Phase 0.4 §6 의무)

Cδ 를 본 cycle 에서 강압적 dismiss 하는 것이 Phase 0.4 §6 위반이 되지 않는 이유:

1. **M6 measurement 직접 반증** (`cb9529d` §2 + §3): byte-level raw hex `0xFFFF 0xFFFF 0xFFFF` + multi-vector 분포 `[−,−,−]` 다수. test 인프라 (byte parsing / endianness / sign-extension) 결함 0 입증.
2. **Cδ 는 *측정 보유* 후보 가 아니라 *반증된* 후보** — Phase 0.4 §6 의 "측정 미보유 후보 강압 dismiss 금지" 의무는 *측정 부재* 후보에 적용; Cδ 는 M6 라는 직접 측정이 이미 반증.
3. **본 cycle Task 1~3 도 Cδ 재반증 측정 0** — 본 cycle 의 측정 (X u[]/Y a[]/Z spec) 은 Cα/Cβ/Cγ 분리에 집중. Cδ 의 *재반증* 의무는 M6 가 충족.

→ Cδ DISMISSED 결정은 M6 측정 인용 + 본 cycle Cα/Cβ/Cγ 측정의 production-측 mismatch origin 입증의 *결합* 으로 정당화. 강압 dismiss 0.

### 3.4 Cγ 의 강압 dismiss 회피 분석 (Phase 0.4 §1)

Cγ "spec 정합 — sample 5..7 한정 추가 mechanism" 가설을 강압적 dismiss 하지 않은 측정 근거:

- Task 3 §3 (production cross-ref): 7 chain stage (Hp / Hf / Ht / AGC / HP / ×2 / Table 9 init) 모두 spec verbatim 정합 — 차이 **0건**.
- Task 3 §4 (PST 비교 도메인): 8 의문 (sample-rate / frame size / file format / chain 종점 / 비교 단위 / decimation / pre-HP / pre-×2) 모두 spec verbatim 정합 — 모순 **0건**, spec 명시 부재 **0건**.
- Task 2 §3 (forced a-sign-flip): a[1..10] 전체 sign flip 시도에도 syn[5..7] 부호 보존 (0/3 flip, 3/3 magnitude 변화) — *a[] 가 부호 source 가 아님* 입증.

→ Cγ priority = **LOW** 는 측정-driven 결론. Phase 0.4 §1 정합.

---

## 4. 권고 단일 결정 + 다음 cycle task outline

### 4.1 권고 단일 결정 (hybrid)

**권고: Cα + Cβ hybrid 진단 cycle (= F-non-prelim-X-split 가칭, plan §Task 4 Step 4 의 "X (hybrid g_p+g_c) — F-non-prelim-X-split — 추가 진단" 행 변형)**.

근거:

- 단일 source = fcb codebook c[0..3] 의 양 입력 (= `g_c · c[n]` 곱) 으로 식별 (§3.1) 됐으나, 곱 부호 source 가 (a) `fcb.Decode` 의 c[n] 부호 결정 (Cα) 인지, (b) `gain.Decoder.Decode` 의 g_c 부호 결정 (Cβ) 인지 분리 측정 미수행.
- Task 1 §3.4 한계 verbatim: "fix 후보 = (a) `fcb.Decode` 의 pulse sign 처리, (b) `gain.Decoder.Decode` 의 g_c 부호."
- Plan §Task 4 Step 4 의 단일 결정 강제 시 **Phase 0.4 §1 의 "단일 결정 강요 시 hybrid 권고 가능"** 조항 발동 → hybrid 진입 정합.
- Cα 와 Cβ 는 둘 다 priority HIGH, risk MID, spec-grounding 보유. 측정 데이터만으로 단일 결정 불가 — 추가 진단 1 cycle 후 production fix cycle 진입.

### 4.2 다음 cycle (F-non-prelim-X-split) task outline

**cycle 명**: F-non-prelim-X-split (가칭, 추가 진단 cycle — production fix 0 라인)
**목적**: `g_c · c[n]` 곱 부호의 sub-source (Cα = c[n] 부호 vs Cβ = g_c 부호) 분리 식별.
**예상 task 수**: **3 task** (Task 1 c[n] 분리 측정, Task 2 g_c VQ 분리 측정, Task 3 종합 + fix cycle 진입 결정).

| Task | 명 | 핵심 입증 의무 | spec § 인용 | 산출 |
|------|----|-----------------|--------------|------|
| **1** | C-fcb-pulse-trace | `fcb.Decode` 내 sub-stage 분리 dump: (a) 4 pulse positions (idx.Positions=0x0000), (b) 4 signs (idx.Signs=0xf), (c) raw pulse placement c_raw[0..39] = ±PulseAmplitude (Q13=±8192) 양/음 분포, (d) β·c[n−t] pitch enhancement 적용 후 c[0..39] 차분. → c[0..3]=+8192 의 origin 이 *raw placement* (sign 0xf → 4-pulse 모두 +) 인지 *β enhancement* (β=Q14:3277, t=20: 효과 **없음** since n<20 일 때 enhancement off — sample 0..3 영역) 인지 정량 분리. | §3.8 + §4.1.5 (4-pulse ACELP) + §3.8.2 (pitch enhancement) | 측정 보고서 + verdict (Cα 단독 / Cα 부분 / Cα 반증) |
| **2** | C-gain-gc-trace | `gain.Decoder.Decode` 내 sub-stage 분리 dump: (a) GA1=5 → GA[5] entry, GB1=6 → GB[6] entry, (b) 합산 g_c 부호 결정 산출, (c) MA predictor (past_err) 영향 (frame 0 = zero predictor state, GAIN-INIT §4.3 Table 9). → g_c=+4153 의 origin 이 *VQ table entry 자체* (ROM cross-check) 인지 *predictor / sign-mask* 인지 정량 분리. | §3.9 (gain VQ) + §3.9.2 (MA predictor) + §A.3.9 + §4.3 Table 9 | 측정 보고서 + verdict (Cβ 단독 / Cβ 부분 / Cβ 반증) |
| **3** | synthesis + production fix cycle 진입 결정 | Task 1 + Task 2 verdict 결합. (i) Cα 단독 → F-non-fix-fcb cycle (production fix in `internal/fcb`, 1~3 라인). (ii) Cβ 단독 → F-non-fix-gain cycle (production fix in `internal/gain`, 1~3 라인). (iii) 둘 다 spec 정합 (반증) → Cγ 재진입 또는 Y magnitude gap (max\|Δ\|=6) follow-up cycle. | (Task 1 + Task 2 인용 catalog 합산) | 종합 보고서 + 다음 cycle plan 또는 fix cycle 진입 |

### 4.3 production / test 변경 라인 예측

| cycle | production | test | 보고서 |
|-------|-----------|------|--------|
| F-non-prelim-X-split (다음) | **0** (진단 only) | +1 신규 진단 함수 (assertion 0) | +3 보고서 (~150 라인 each) |
| F-non-fix-{fcb,gain} (그 후) | 1~3 | 0 (item 16 RED → GREEN 전환) | +1 fix cycle 보고서 |

---

## 5. 사용자 게이트

F-oct-postfix2-prelim synthesis (`9a5a7f6`) §5 G-N1~G-N5 + 본 cycle 신규 항목:

| # | 항목 | 본 cycle 결정 / 갱신 | 사용자 응답 의무 |
|---|------|---------------------|------------------|
| **G-S1** | F-non-prelim cycle 자체 | 본 cycle 종결 (Task 1~4 commit 4건). | acknowledge |
| **G-S2** | 4 후보 (Cα/Cβ/Cγ/Cδ) 단일 식별 | 단일 source = fcb codebook c[0..3] 양 입력. **Cα + Cβ hybrid (분리 미상)** + Cγ LOW priority + Cδ DISMISSED (M6 반증). | hybrid 결정 승인 / 또는 Cα 단독·Cβ 단독 강제 선택 / 또는 E3 단독 발동 (다른 진단) |
| **G-S3** | 다음 cycle 권고 = F-non-prelim-X-split (3-task, production 0) | hybrid 진입 정합 (Phase 0.4 §1 hybrid 권고 조항). | 진입 승인 / 다른 cycle 명 제안 |
| **G-S4** | `stagef_bis_diagnostic_test.go` (untracked) 보존 | 본 cycle 변경 0 — Phase 0.5 충족. | acknowledge / 다음 cycle 시 처리 결정 |
| **G-S5** | 항목 16 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) RED 잔존 | 본 cycle 보존 — 다음 fix cycle 의 GREEN 전환 gate 로 승계. | acknowledge |
| **G-S6** | F-sept-2 L3 magnitude gap (max\|Δ\|=6) follow-up | 본 cycle 부호 source 식별과 *직교* — 본 cycle 외 처리 항목 인계. | 별도 cycle 우선순위 결정 (g_c·c 부호 fix 후 / 병행) |
| **G-S7** | Cγ "추가 spec mechanism" 잔여 가능성 | priority LOW (Task 3 §5 verbatim "모순 0건"). spec § 추가 검토 의무 = 다음 fix cycle 의 spec § 인용 catalog 가 본 cycle Z 의 7+8 항목으로 충족. | acknowledge / Cγ 우선 재진입 강제 시 명시 |
| **G-S8** | W 후보 (PST 출처) 진입 조건 | Phase 0.4 §6: *모두 반증* 조건 미충족 (Cα/Cβ 잔존 + Cδ M6 반증 별개). W 미진입. | acknowledge |

---

## 6. Phase 1k 종결 평가

### 6.1 현 상태

- **Phase 1k Stage F 누적 cycle**: F-oct-prelim (5 task) + F-oct-postfix (2 task) + F-oct-postfix2-prelim (4 task) + F-non-prelim (4 task) = 4 cycle (총 15 task + synthesis 4건).
- **누적 RED 항목**: 1 (item 16 = `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`, frame 0 sf0 sample 5..7 Δ=3).
- **단일 source 식별 도달**: 본 cycle Task 4 에서 = `g_c · c[n]` 곱 부호. Cα/Cβ 분리 미상.

### 6.2 종결 가능성 평가

| 시나리오 | 가능 / 불가 | 추정 잔여 cycle 수 |
|----------|------------|-------------------|
| **Best case** (Cα 또는 Cβ 단독 식별 + 1~3 라인 fix → item 16 GREEN) | **가능** | F-non-prelim-X-split (1 진단) + F-non-fix-{fcb,gain} (1 fix) = **2 cycle** |
| **Mid case** (Cα + Cβ 둘 다 부분 결함 → 양쪽 fix) | 가능 | F-non-prelim-X-split (1) + F-non-fix-fcb (1) + F-non-fix-gain (1) = **3 cycle** |
| **Worst case** (Cα/Cβ 둘 다 spec 정합 반증 → Cγ 재진입 또는 W 진입) | 가능 (장기) | +2~5 cycle (Cγ spec 추가 mechanism 발견 또는 W 진단) |
| **F-sept-2 L3 magnitude gap follow-up 통합** | 가능 (병행) | +1~2 cycle |

→ **Phase 1k 종결 가능 — 추정 best 2 cycle / mid 3 cycle / worst +5 cycle**. 본 cycle 의 단일 source 식별 (= `g_c · c[n]` 곱) 은 fix scope 를 `internal/fcb` ∪ `internal/gain` 2 패키지로 자동 협소화 — 종결 경로 명확.

### 6.3 잔여 우려

- **F-sept-2 L3 magnitude gap (max\|Δ\|=6)**: 부호 source 와 *직교* 하나 LP a[] coefficient 정확도 결함. item 16 GREEN 후에도 별도 cycle 필요 가능 (Y-magnitude verdict 의 본 cycle 외 처리 인계).
- **Cα 재정의 위험**: `fcb.Decode` 의 β·c[n−t] pitch enhancement (β_q14=3277, t=20) 가 sample 0..3 영역 (n<t) 에서는 *적용되지 않음* — `placePulses` 단계의 raw c[n] = `±8192 (Q13)` 가 직접 sample 0..3 결정. 따라서 Cα 후보의 핵심 sub-source = idx.Signs (=0xf) 처리. F-non-prelim-X-split Task 1 의 측정으로 이를 verbatim 분리 입증 의무.
- **production 변경 0 라인 의무 유지**: 본 cycle 종결 시점까지 production untouched (Phase 0.4 + plan invariant 정합).

---

**보고서 종료.** 본 commit = F-non-prelim cycle 4번째 (final) commit. 다음 cycle = F-non-prelim-X-split (사용자 G-S3 승인 시).
