# Phase 1n — Stage R-C-empirical Cycle Plan (sf-1 LSP interpolation rounding mode flip)

**Cycle ID**: `R-C-empirical` (Phase 1n 1번째 cycle, CE-4 ranked path **(c-R-C-empirical)** —
Phase 1m R-C inventory item 의 **empirical disposition** by branch-test).
**작성일**: 2026-05-08
**선행 commit**:
- `21894d3` — Phase 1m F-Cγ-elsewhere synthesis + ambiguous verdict close
  (R-A / R-B / R-C 3종 ambiguity 인벤토리 + (CE-ambiguous) 채택).
- `5232411` — Phase 1m CE-3 LSP/LSF init+interp diagnostic (R-C surface 식별,
  ALGTHM frame 0 sf-1 LSP i=1 / i=5 −1 LSB drift candidate cell).
- `9ab1c91` — gate 17 disposition (Phase 1l alt-path d-i): t.Skip + R-C
  empirical reactivation trigger 등재.

**선행 plan 양식**: `docs/superpowers/plans/2026-05-07-phase1m-stage-f-cgamma-elsewhere-plan.md`.

**사용자 게이트 (가정)**: G-XS6 = (c-R-C-empirical) — Phase 1m CE-4 권고 ordering rank 1.
이는 Phase 1m R-C ambiguity (`§3.2.5 eq. (24)` sf-1 rounding mode 의 spec verbatim
부재) 를 corrigendum 외부 검색 대신 **결과-driven empirical branch-test** 로
disposition 한다는 결정. 본 cycle 은 26 cycle 만에 처음 production line 을
**조건부로** touch 하는 cycle (Phase 1n 원칙 §0.3 참조).

---

## Phase 0 — Context, Invariant, E2 relaxation 선언

### 0.1 26 cycle 누적 (요약)

- 25 sub-hypothesis 폐기 (Phase 1k 16 + Phase 0c 4 + Phase 1l 2 + Phase 1m 3),
  defect = 0.
- 4 hard-spec invariant 명시 확정 (verbatim):
  - I-1: §4.2.4 AGC carryover (postfilter `agcGainPrev`, HP-1).
  - I-2: §4.3 catch-all zero-init (HP filter `hpX[2]`/`hpY[2]` + 미열거 변수, HP-2).
  - I-3: §A.4.2.5 IIR pole-pair impulse decay (1.93/-0.94, HP-2).
  - I-4: §3.2.4 LSP init formulas (`l̂_i = i·π/11` Q13 + `q_i = cos(i·π/11)` Q15,
    Phase 1m CE-3 / `5232411`).
- 3 R-blocking ambiguity 인벤토리 (`21894d3`):
  - **R-A** (§3.9.3 reorder map values verbatim 부재) — CE-1 6 UND.
  - **R-B** (§3.8.2 sign/position bit-string layout verbatim 부재) — CE-2 8 UND;
    confirmed-blocking 단 **sample 5..7 직접 영향 0** (PROD/SPEC 양쪽
    `c[5]=c[6]=c[7]=0`).
  - **R-C** (§3.2.5 eq. (24) sf-1 rounding mode 미명시) — CE-3 2 UND
    (i=1 / i=5); ALGTHM frame 0 의 sf-1 LSP odd-half-sum 2 cell 에서
    floor `>>1` vs round-half-away-from-zero 가 +1 LSB drift 발생.

### 0.2 R-C empirical 우선순위 정당화

CE-4 synthesis (`21894d3` §10):

- **R-C 만이 sample 5..7 sign mismatch 의 PLAUSIBLE-but-not-PROVABLE mechanism
  candidate live**:
  - R-A: g_p=+1995 / g_c=+4153 측정값 (F-non-prelim-X-split-2) 이 spec 정합
    범위 → Imap/GBK 값 모호가 sample 5..7 에 미치는 영향 surface 좁음.
  - R-B: ALGTHM frame 0 sf0 의 c[5]=c[6]=c[7]=0 (양쪽) → bit ordering 재해석
    으로는 sample 5..7 부호를 흔들 수 없음 (CE-2 §3.3 결정적 evidence).
  - R-C: 1-LSB sf-1 LSP drift → §3.2.6 Chebyshev 전개 → a[] LP 계수 → §3.10
    `synth.Filter` 의 early sample (n=0..filter-order≈10) 영향 — sample
    5..7 가 정확히 이 윈도우 안에 위치.
- 비용 최소: branch-test 1 cell (interpolation rounding expression 1 line) +
  knob 도입/제거 cycle-내 closure → 1 task 분량 (CE-1/2/3 3 task 대비 1/3).

### 0.3 **E2' relaxation 선언 (본 cycle 한정)**

**E2 원본** (Phase 1k~1m 모든 cycle 적용): production code line 변경 = 0.

**E2'** (Phase 1n 본 cycle 한정 relaxation):

> Phase 1n R-C-empirical cycle 동안 production code 의 단일 표현식 (`internal/lsp/interpolate.go` 의 `interpolateLSP` 내 `(int32(prev)+int32(curr))>>1`) 을 **test-only knob 뒤에** 위치시킨다. knob 의 default 값은 현재 production 동작 (floor `>>1`) 과 byte-EQ — 즉 default 호출 경로의 동작은 변동 0. test 전용 setter 가 knob 을 symmetric round (round-half-away-from-zero) 로 toggle 하여 ALGTHM frame 0 의 sample 5..7 변화를 측정한다. 본 knob 은 **동일 cycle 내 RC-3 단계에서 반드시 제거** 된다 (verdict 에 따라 (a) refute → knob 제거 + 표현식 원복 = 결과적 production 동작 0 변경, 또는 (b) defect-confirmed → 사용자 게이트 통과 후 knob 제거 + 표현식을 symmetric round 로 교체 = production 동작 변경, 단 본 plan 은 cycle-end 변경에 대해 **사용자 게이트 G-XS7 명시 통과 의무** 를 요건으로 함).

**E2' 가 보장하는 것**:

1. 모든 non-Phase-1n test (gate 1..16 PASS, gate 17 SKIP, gate 18 PASS, gate
   19 pending, Phase 1m CE-1/2/3 측정 test) 는 본 cycle 진행 중 byte-EQ
   동작 유지 (knob default = 현재 production = floor).
2. 측정 test 만이 명시적으로 knob 을 toggle.
3. cycle 종결 시 knob 자체는 retire (test 와 production 양쪽에서 제거).
4. 결함 미확인 (refute) 시 production 동작 = cycle 진입 시점과 byte-EQ.

**E1 / E3 / E4 / E5 는 본 cycle 에서도 원본 강도 유지** — 외부 G.729 구현 0
참조, gate 17 reactivation 은 synthesis + 사용자 게이트 후, spec 모호
cherry-pick 금지, 자동 promotion 금지.

### 0.4 누적 contract test gate (19건 + 본 cycle 측정 1~2건)

| # | gate | 상태 | 비고 |
|---|------|------|------|
| 1..16 | (Phase 1a~1j 누적) | PASS | knob default = floor → 변동 0 |
| 17 | `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` | **SKIP** | reactivation 결정은 RC-3 synthesis + G-XS7 |
| 18 | F-non-prelim-X-split bundle | PASS | knob 영향 없음 |
| 19 | P0c-reentry bundle | pending | Phase 1m carry, 본 cycle scope 외 |
| 20 | Phase 1m CE-1/2/3 measurement bundle | pending (G-XS6 요건) | knob default → byte-EQ |
| 21 (잠정) | Phase 1n RC-1 sf-1 rounding branch-test | 측정-only / E5 자동 등재 금지 | RC-3 synthesis + G-XS7 후 결정 |

### 0.5 Working tree 보존 명시

- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) **변경 금지** —
  Phase 1k 부터 보존된 진단 test, 본 cycle scope 외. 본 cycle commit 시
  `git status` 에 untracked 상태 그대로 유지. **stage / modify / delete
  금지**.

---

## Phase 1 — Hypothesis 진술 (R-C-empirical)

**Refined hypothesis (R-C-empirical)**:

> ITU-T G.729 §3.2.5 eq. (24) (sf-1 LSP interpolation: `q_i^{(1)} = 0.5·q_i^{(prev)} + 0.5·q_i^{(curr)}`) 의 rounding mode spec verbatim 부재 ambiguity 가 결함이라면, production 의 floor `(int32(prev)+int32(curr)) >> 1` 을 symmetric round (round-half-away-from-zero) 로 교체한 branch 에서 ALGTHM frame 0 sf0 sample 5..7 부호가 floor branch 대비 변경되며, 변경 방향이 want `[−1, −1, −1]` 에 일치한다 (현 production = `[+1, +1, +1]`, Δ = +3 uniform per F-oct-postfix-1).

**Defect mechanism trace (CE-3 finding 재확인)**:

```
sf-1 LSP i=1, i=5 의 odd-half-sum cell
  └→ floor: 31954, 7351   /   sym round: 31955, 7352   (+1 LSB drift on 2/10 cells)
       └→ §3.2.6 Chebyshev expansion → a[] LP filter coefficients (Q12)
            └→ §3.10 synth.Filter 의 IIR direct-form recursion
                 └→ early sample n=0..~10 출력에 영향 (sample 5..7 ∈ window)
                      └→ sample 5..7 부호가 want 와 일치하면 → DEFECT 확정
```

**Pass / Fail / Partial 정의** (RC-1 측정 → RC-3 synthesis):

| outcome | 조건 (knob = symmetric round 시 sample 5..7 측정값) | 분류 | 다음 단계 |
|---------|-----------------------------------------------------|------|-----------|
| **Defect-confirmed** | 3/3 sample 부호 = want `[−1, −1, −1]` (또는 ≥ 2/3 + 명시 rationale) | DEFECT | RC-3 synthesis → G-XS7 사용자 게이트 → (승인 시) production 을 symmetric round 로 교체 + knob 제거 + gate 17 reactivation. |
| **Refute** | sample 5..7 부호 무변동 또는 want 반대 방향 (`+1` → 더 큰 양수 / 부호 동일 + magnitude drift only) | REFUTE | RC-3 → knob 제거 + 표현식 원복 (= 본 cycle 진입 시점과 byte-EQ) → R-C surface live 해제 → (c) corrigendum 또는 (a) multi-frame 으로 escalate. |
| **Partial** | 1/3 sample 부호 변경 + 2/3 unchanged (또는 sign 변경이 want 와 부분 일치) | PARTIAL | RC-3 → knob 제거 + 표현식 원복 → (c) corrigendum search 권고 (R-C 만으로는 mechanism 불충분, 보조 mechanism 필요) → 사용자 게이트 G-XS7-partial. |

**Anti-cherry-pick 규칙** (E4 강도 유지):

- "거의 정합" / "방향만 일치" / "±1 LSB 범위 내 부합" 분류 **금지**.
- `+1 → 0` 는 "부호 변경" 으로 분류하지 않음 (want 가 `−1` 이므로 0 도 unmatched).
- 측정값 = exact int16, classifier = strict equality.

---

## Phase 2 — Pre-cycle exploration 결과

본 절은 plan 작성 직전 수행한 production code 직접 read 결과.

### 2.1 sf-1 LSP interpolation — `internal/lsp/interpolate.go`

**file**: `internal/lsp/interpolate.go` (전체 19 라인).

**target function**: `interpolateLSP(prev, curr, sf1, sf2 *[10]int16)` — 패키지
unexported, 한 파일 내 단일 호출자 (`(*Decoder).Decode`).

**target expression** (line 15):

```go
sf1[i] = int16((int32(prev[i]) + int32(curr[i])) >> 1)
```

이 한 줄이 R-C ambiguity 의 production 표현식. sf2 (line 16) 는 `curr[i]` 직접
복사 — eq. (24) sf-2 = `q_i^{(curr)}` 와 명시적 EQ, 본 cycle scope 외.

**call site**: `internal/lsp/decoder.go` line 100 부근 — `interpolateLSP(&d.prevLSP, &lsp, &lspSF1, &lspSF2)` (Decode step 7). 직후 `lspToLP(&lspSF1, &sf1A)` / `lspToLP(&lspSF2, &sf2A)` 가 §3.2.6 Chebyshev 전개 수행 → return `(sf1A, sf2A [11]int16)` Q12.

### 2.2 측정 scaffolding — `internal/lsp/phase1m_ce3_init_interp_diagnostic_test.go`

- 기존 helper `ce3VectorPath("ALGTHM.BIT")` + `bitstream.ReadG192File` + `bitstream.Unpack` 사용 가능. 본 패키지 (lsp) 에서 ALGTHM frame 0 의
  `(L0, L1, L2, L3)` 추출 + `Decoder.Decode(idx)` 호출까지의 chain 검증 완료.
- 단 RC-1 은 sample 5..7 까지 도달해야 하므로 **decoder 패키지** 에서 full
  `Decoder.Decode(packed, bad, out)` 호출 필요. → RC-1 test 위치 = `internal/decoder/`.
- decoder 패키지 helper 사용: `vectorPath`, `ensureTestdataPresent`,
  `readG192Frames`, `readPSTFrames` (gate 17 test 와 동일 helper 재사용).

### 2.3 main decode loop — `internal/decoder/decode.go`

`Decoder.Decode(packed, bad, out)` (line 18~50):

1. `bitstream.Unpack` → frame fields.
2. `d.lsp.Decode(...)` → `(sf1A, sf2A)` — **여기서 `interpolateLSP` 호출 발생**.
3. `pitch.DecodeDelaySubframe1(P1)` → `(tInt1, tFrac1)` — RC-2 의 T1 추출원.
4. `d.decodeSubframe(&sf1A, ..., out[:40])` — sf1A → `synth.Filter` → s[0..39] →
   postfilter → HP → out[0..39].
5. `pcm.ScaleUpSat(out, out)` — ×2 saturation.

→ sample 5..7 의 path = `sf1A → synth.Filter → s[5..7] → pst.Filter → hpFilter → ×2`. sf1A 변동이 sample 5..7 에 영향을 미친다는 가설이 RC-1 의 직접 검증 대상.

### 2.4 synth filter entry — `internal/synth/filter.go`

`(*Synthesizer).filterSubframe(a, u, s)` (line 19) — `a *[11]int16` Q12 LP coefficient 입력. Direct-form 1/A(z) 40-sample loop. Past state `pastSynth[10]` 유지. sample 5..7 = `work[15..17]` 위치 (work[0..9] = pastSynth, work[10..49] = 출력).

**sample 5..7 의존 윈도우**: direct-form recursion `s[n] = u[n] − Σ_{k=1..10} a[k]·s[n-k]`. sample 5 는 `s[0..4] + pastSynth[5..9]` 의존; sample 7 은 `s[0..6] + pastSynth[7..9]` 의존. 즉 sample 5..7 은 a[1..10] 전체에 영향받는다 (frame 0 이므로 pastSynth = 0).

### 2.5 pitch pre-emphasis (RC-2 fold-in 후보) — `internal/fcb/enhance.go`

`applyPitchEnhancement(c *[40]int16, t int, betaQ14 int16)` (line 40~54):

- guard: `t < 1 || t >= 40 → return`. β = 0 → return.
- loop: `for n := t; n < 40; n++ { c[n] += round(β·c[n-t]·2^{-14}) }` (Q13, in-place).

**관측 evidence (Phase 1m CE-2)**: ALGTHM frame 0 sf0 의 PROD/SPEC c[] 양쪽
에서 `c[5]=c[6]=c[7]=0`. FCB pulse positions = {0, 1, 2, 3} (모두 Q13 ±8192).
이 evidence 로부터 사후추론:

- 만일 `T ∈ {5, 6, 7}` 이고 β > 0 이면 c[T] = β·c[0] ≠ 0 가 필연 (c[0] = ±8192).
  관측 c[5..7] = 0 → 따라서 **T ∉ {5, 6, 7} 또는 β = 0** (둘 중 하나는
  반드시 참).
- → RC-2 의 직접적 surface 는 c[] 단계에서 이미 닫힘 (CE-2 결정적 측정).

**RC-2 의 잔여 가치**: T1 raw 값 추출 (1 line, `pitch.DecodeDelaySubframe1(f.P1)`)
+ β1 raw 값 추출 (1 line, `fcb.ClampPitchGainForEnhancement(0)` — frame 0 sf0
의 prevGp = 0 → β = `betaLowerQ14 = 3277`) + assertion `T1 ∉ {5,6,7} ∨ β1 = 0`
명시 기록. → ≤ 25 LOC test, ≤ 1 file 신규 (decoder 패키지).

### 2.6 Pre-exploration 종합 — RC-1 target line 확정

| 항목 | 값 |
|------|----|
| target file | `internal/lsp/interpolate.go` |
| target function | `interpolateLSP(prev, curr, sf1, sf2 *[10]int16)` |
| current rounding expression | `int16((int32(prev[i]) + int32(curr[i])) >> 1)` |
| line number | **15** |
| call sites | `internal/lsp/decoder.go` step 7 (단일) |

---

## Phase 3 — Task 분해 (RC-1 mandatory + RC-2 fold-in + RC-3 synthesis)

### Task RC-1: sf-1 LSP interpolation rounding mode branch-test (mandatory)

**Sub-hypothesis**: §1 refined hypothesis 본문 그대로.

**Scope**:

- **In**: `internal/lsp/interpolate.go` 의 단일 표현식을 knob 뒤에 위치시키되
  default = floor (현재 동작 byte-EQ); test (decoder 패키지) 에서 knob 을
  toggle 하여 ALGTHM frame 0 sample 5..7 측정.
- **Out**: production default 동작 변경 (cycle 종결 시까지 보장); R-A / R-B
  관련 변경; gate 17 직접 reactivation (RC-3 synthesis 후).

**E2' 적용 design (test-only knob)**:

1. `internal/lsp/interpolate.go` 수정:

   ```go
   // lspInterpRoundMode controls the sf-1 interpolation rounding mode.
   // Phase 1n R-C-empirical: knob default = arithmetic floor (`>>1`),
   // matching the cycle-entry production behaviour byte-for-byte.
   // The knob is removed at Phase 1n cycle close (RC-3) — either
   // retired with the floor default kept (refute) or retired with
   // the expression replaced by symmetric round (defect-confirmed,
   // post G-XS7 user gate).
   //
   // Values: 0 = floor (`(prev+curr) >> 1`)        ← DEFAULT
   //         1 = round-half-away-from-zero
   var lspInterpRoundMode int

   func interpolateLSP(prev, curr, sf1, sf2 *[10]int16) {
       for i := 0; i < 10; i++ {
           sum := int32(prev[i]) + int32(curr[i])
           switch lspInterpRoundMode {
           case 1:
               // round-half-away-from-zero
               if sum >= 0 {
                   sf1[i] = int16((sum + 1) >> 1)
               } else {
                   sf1[i] = int16(-((-sum + 1) >> 1))
               }
           default:
               sf1[i] = int16(sum >> 1)
           }
           sf2[i] = curr[i]
       }
   }
   ```

   default branch 는 `sum >> 1` 로 현재 production 과 byte-EQ (Go 의 `>>` 는
   signed int32 에서 arithmetic shift = floor toward −∞).

2. `internal/lsp/interp_testhook.go` 신규 (build tag 없이 export — test-only
   접근만 의도; 본 패키지 외 호출자는 RC-3 에서 grep 검증 + 제거 보장):

   ```go
   package lsp

   // SetLSPInterpRoundModeForTest is a Phase 1n R-C-empirical test-only
   // knob installer. It returns a restore func; callers MUST defer it.
   // Removed at RC-3 cycle close.
   func SetLSPInterpRoundModeForTest(mode int) (restore func()) {
       prev := lspInterpRoundMode
       lspInterpRoundMode = mode
       return func() { lspInterpRoundMode = prev }
   }
   ```

3. RC-1 test: `internal/decoder/phase1n_rc1_lspinterp_branch_diagnostic_test.go`
   신규 — `TestDiagnostic_Phase1nRc1LSPInterpBranchALGTHM`:

   - sub-test "floor (default)": baseline. knob 미설정. ALGTHM frame 0 full
     decode. 측정 sample 5..7 = `[+1, +1, +1]` 와 EQ assertion (현 production
     동작 baseline).
   - sub-test "symmetric round": `defer lsp.SetLSPInterpRoundModeForTest(1)()`.
     ALGTHM frame 0 full decode (재생성 — 새 `Decoder` instance). 측정 sample
     5..7 raw 값 t.Logf + classifier `classifyRc1Branch(got, want)` 호출 →
     verdict ∈ {DEFECT_3of3, DEFECT_2of3, REFUTE_unchanged, REFUTE_diverge,
     PARTIAL_1of3}.
   - want: `wantFrames[0][5..7]` = `[−1, −1, −1]` (per ALGTHM.PST).
   - **t.Errorf 사용 지점** (강제 실패): "floor (default)" sub-test 가 sample
     5..7 ≠ `[+1, +1, +1]` 이면 default branch 가 production 과 byte-EQ 가
     아닌 증거 → E2' 위반 → 테스트 실패. 그 외 verdict 는 t.Logf only (E5).
   - sub-test 종료 시 `restore()` 호출되어 knob 복원 (default = 0).

**TDD 절차**:

1. RED: production knob hook 부재 상태에서 test 컴파일 실패 (예상).
2. GREEN: `interpolate.go` knob 도입 + `interp_testhook.go` 신규 + test 통과
   (default sub-test = baseline EQ; symmetric sub-test = log only).
3. measurement dump: floor sample 5..7 / sym sample 5..7 / Δ per-sample / verdict.
4. **commit (RC-1)**:

   ```
   test(decoder): add Phase 1n RC-1 sf-1 LSP interp rounding branch-test

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

   commit body: E2' 명시 인용 + knob 도입 정당화 + cycle-end 제거 commitment.

---

### Task RC-2: pitch pre-emphasis residue-0 sub-measurement (optional fold-in)

**Sub-hypothesis (RC-2)**:

> ALGTHM frame 0 sf0 의 pitch pre-emphasis `c'(n) = c(n) + β·c'(n−T)` 는 sample 5..7 에 직접 기여하지 않는다 (CE-2 c[5..7]=0 evidence 의 mechanistic confirmation): T1 ∉ {5, 6, 7} 또는 β1 = 0 중 하나가 참.

**Inclusion 결정**: **포함** (cheap fold-in).

**Inclusion rationale**:

- 비용 ≤ 25 LOC test, 1 file 신규, production touch 0.
- CE-2 (`0d58ca6`) 의 c[5..7]=0 측정값을 mechanistic 으로 닫는다 (T1, β1 raw
  값 explicit 기록 → 이후 cycle 에서 "T1 가 sample 5..7 에 영향?" 재방문
  방지).
- RC-1 결과가 REFUTE 일 경우 RC-3 synthesis 의 alternative-mechanism 후보
  검토 시 본 측정값이 "pitch pre-emphasis 도 surface 가 아니다" 결정의 직접
  evidence.
- E1/E2/E4/E5 모두 강도 유지 (production 0 변경, 측정-only).

**측정 design**:

- ALGTHM frame 0 unpack → P1 추출 → `pitch.DecodeDelaySubframe1(uint8(f.P1))`
  → `(tInt1, tFrac1)`.
- prevGp = 0 (frame 0 sf0 진입 시점 zero-init) → `fcb.ClampPitchGainForEnhancement(0)`
  → `betaQ14` 추출.
- assertion: `tInt1 < 5 || tInt1 > 7` OR `betaQ14 == 0`. 둘 중 하나라도 참 →
  EQ (CE-2 c[5..7]=0 mechanistic confirmation). 둘 다 거짓 → NE (CE-2 측정
  evidence 와 contradict, 즉시 escalate).
- t.Logf: `T1=%d  β1Q14=%d  CE2-confirm=%s` 명시 기록.

**TDD 절차**:

1. RED: test 신규.
2. GREEN: production 0 변경, helper 호출만.
3. **commit (RC-2)**:

   ```
   test(decoder): add Phase 1n RC-2 pitch pre-emphasis residue-0 sub-measurement

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

---

### Task RC-3: synthesis + knob removal + gate 17 reactivation 결정

**선행 의무**: RC-1 (필수) + RC-2 (포함 시) commit 완료.

**TDD 절차**: synthesis = report-only.

**행동 sequence**:

1. RC-1 verdict 분류 (Defect-confirmed / Refute / Partial) 명시 결정.
2. RC-2 verdict 기록 (T1, β1 raw 값 + CE-2-confirm).
3. **knob 제거 의무 (E2' cycle-end commitment)**:
   - **Refute / Partial**: `internal/lsp/interpolate.go` 의 knob 변수 +
     switch 분기 제거 → 표현식을 cycle-진입 시점과 동일한 단일 line 으로
     원복 (`int16((int32(prev[i]) + int32(curr[i])) >> 1)`).
     `internal/lsp/interp_testhook.go` 파일 삭제. RC-1 test 의 symmetric
     sub-test 는 t.Skip 으로 archival (knob 부재 시 컴파일 실패 회피)
     또는 file 삭제. → **결과적 production 동작 변경 = 0**.
   - **Defect-confirmed**: G-XS7 사용자 게이트 양식 (synthesis report 에
     명시) 권고 → 사용자 승인 후 별도 fix cycle 에서 (a) knob + switch
     제거, (b) 표현식을 symmetric round 단일 line 으로 교체, (c) RC-1
     test 의 symmetric sub-test 를 회귀 baseline 으로 promote, (d) gate
     17 reactivation (`stagef_octpostfix_regression_test.go` 의 `t.Skip`
     제거). 본 cycle (RC-3) 자체는 fix 미수행, **G-XS7 게이트만 발의**.
4. synthesis report 작성:
   `docs/superpowers/plans/2026-05-08-phase1n-stage-r-c-empirical-synthesis-report.md`.
   - RC-1 verdict + sample 5..7 raw 값 (floor / sym / Δ) 표.
   - RC-2 verdict + T1, β1 표.
   - 3-시나리오 결정 트리 적용 결과.
   - knob 제거 diff 요약 (Refute/Partial 시 자동 / Defect 시 G-XS7-pending).
   - gate 17 reactivation 권고 (Defect 시 G-XS7-bound).
   - 차기 cycle 권고: Refute → (c) corrigendum search 또는 (a) multi-frame
     state propagation; Defect → fix cycle dispatch + G-XS7; Partial →
     (c) corrigendum + R-C secondary mechanism 검색.
5. **commit (RC-3)**:

   ```
   docs(plans): Phase 1n R-C-empirical synthesis + knob removal

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

   commit body 는 verdict + knob 제거 diff 요약 + G-XS7 양식 (필요 시).

---

## Phase 4 — 회귀 게이트 (각 commit 직후)

각 task commit 직후:

1. `go vet ./...` — clean 필수.
2. `go test ./...` — gate 1..16 PASS, gate 17 SKIP, gate 18 PASS, gate 20
   pending 모두 RC-1 commit 시점에도 byte-EQ (knob default = floor 보장).
3. RC-3 commit 후: knob 제거 후 다시 `go vet ./...` + `go test ./...` —
   회귀 0 확인.
4. RC-1 / RC-2 측정 test = E5 자동 promotion 금지.

---

## Phase 5 — Anti-goals (명시 금지 list)

본 cycle 에서 절대 수행 금지:

1. **외부 G.729 구현 참조 (E1)** — ITU reference C / bcg729 / Sipro / FFmpeg /
   Annex A binary 인용 0건. symmetric round 의 정의는 PDF §3.2.5 eq. (24)
   verbatim + 일반 수치해석 정의 (round-half-away-from-zero) 만 인용.
2. **production default 동작 변경 (E2')** — knob default = floor 고정.
   변경은 cycle-end 의 RC-3 단계에서만, 그것도 Refute/Partial 시 0 변경 보장,
   Defect 시 G-XS7 게이트 통과 후 별도 단계.
3. **knob retention (E2' cycle-end commitment)** — 본 cycle 종결 시 knob
   변수 + setter + switch 분기 모두 제거. 별도 cycle 로 이연 금지.
4. **gate 17 즉시 reactivation 강행 (E3)** — Defect 시에도 G-XS7 사용자 게이트
   통과 전까지 t.Skip 유지.
5. **spec ambiguity cherry-pick (E4)** — symmetric round 가 want 와 일부
   일치하더라도 "방향 정합" 으로 선언 금지. classifier = strict 3/3 또는
   2/3 + 명시 rationale.
6. **자동 promotion (E5)** — RC-1 / RC-2 측정 test 의 회귀 게이트 자동 등재
   금지. RC-3 + G-XS7 통과 후에만 promotion.
7. **bitstream / pitch / fcb / gain / synth / postfilter / HP 의 production
   변경** — 본 cycle scope = `internal/lsp/interpolate.go` 단일 표현식 +
   `internal/lsp/interp_testhook.go` 신규 파일만. 그 외 production touch 0.
8. **R-A / R-B 재방문** — 본 cycle scope 외. R-A 는 (c) corrigendum 의
   주된 surface, R-B 는 sample 5..7 영향 0 으로 확정 (CE-2).
9. **OVERFLOW.BIT bit-stream framing 별도 issue** — 무관, 재방문 금지.
10. **untracked 진단 파일 변동** — `internal/decoder/stagef_bis_diagnostic_test.go`
    본 cycle 동안 stage / commit / move / delete 금지.

---

## Phase 6 — Escape hatch E1-E5 (R-C-empirical 특수 trigger)

| code | 발동 조건 | 행동 |
|------|----------|------|
| E1 | symmetric round 가 "ITU reference C 의 default" 라는 사후추론 유혹 | 차단. 정의 = 일반 수치해석 round-half-away-from-zero verbatim only. |
| E2' | knob default = floor 외 다른 값 commit, 또는 cycle-end knob retention | 차단. RC-3 의 knob 제거는 cycle-end 의 hard requirement. |
| E3 | RC-1 Defect 발견 즉시 gate 17 t.Skip 제거 욕구 | 차단. G-XS7 사용자 게이트 통과 전까지 SKIP 유지. |
| E4 | "2/3 일치 + 1/3 unchanged" 를 임의로 Defect 분류 | 차단. 2/3 분류는 plan 명시 rationale (e.g. sample 7 의 추가 mechanism 인정 + 명시 sub-hypothesis 분리) 필요. |
| E5 | RC-1 / RC-2 commit 시 회귀 게이트 자동 등재 | 차단. promotion = G-XS7 후. |

---

## Phase 7 — Cycle-end commitments (요약)

| 결과 | knob 제거 | production 표현식 | gate 17 | 차기 cycle |
|------|-----------|-------------------|---------|-----------|
| **Defect-confirmed** | RC-3 + G-XS7 후 | symmetric round 로 교체 | reactivation 권고 (G-XS7-bound) | fix cycle (G-XS7 통과 후) |
| **Refute** | RC-3 자동 | floor 원복 (cycle-진입 시점 byte-EQ) | SKIP 유지 | (c) corrigendum 또는 (a) multi-frame |
| **Partial** | RC-3 자동 | floor 원복 (cycle-진입 시점 byte-EQ) | SKIP 유지 | (c) corrigendum + R-C secondary mechanism |

**모든 시나리오 공통**:

- knob 변수 + setter + switch 분기 제거 (cycle-end 의 hard requirement).
- E1 / E2' / E3 / E4 / E5 위반 0 확인.
- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) 미변경 확인.
- `go vet ./...` clean + `go test ./...` 회귀 0.

---

## Phase 8 — 사용자 게이트 (G-XS7) 양식 — RC-3 Defect 시에만 발의

```
Q (G-XS7): Phase 1n R-C-empirical RC-1 verdict = Defect-confirmed.
sf-1 LSP interpolation rounding mode 를 floor `>>1` 에서 round-half-
away-from-zero 로 production 교체 + gate 17 reactivation 을 승인하시겠습니까?

EVIDENCE:
- ALGTHM frame 0 sf0 sample 5..7: floor=[+1,+1,+1] sym=[<v5>,<v6>,<v7>] want=[-1,-1,-1]
- Δ per-sample (sym - floor): [<d5>,<d6>,<d7>]
- match score: <N>/3
- prior 25 sub-hypothesis refute + 4 hard-spec invariant: 본 결과로 mechanism 식별 = SKIP 정당화 소멸.

OPTIONS:
(A) 승인 → fix cycle dispatch (1 commit, ≤ 5 LOC production diff + gate 17 t.Skip 제거).
(B) 보류 → corrigendum cross-check 추가 후 재발의.
(C) 거부 → R-C 를 spec ambiguity 로 영구 동결, 다른 mechanism 검색.
```

(Refute / Partial 시 본 양식 미발의, RC-3 synthesis report 만 commit.)
