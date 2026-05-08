# Phase 1k Stage F-oct-prelim-5-4 종합 보고서 + F-oct 권고

**작성일**: 2026-05-01
**범위**: F-oct-prelim-5-1 (P-SRC × B-CMP) / 5-2 (Q-FMT × H-INIT × H-RESP) /
        5-3 (M1/M2/M3/M4 4-tuple) 결과 결합 분석. 가설 G3 (Annex A vs main
        spec 분기 거동) 의 분기 위치 단일 식별 또는 G3 폐기. F-oct cycle
        권고 단일 결정 (a production fix / b plan-end / c 추가 진단). 본
        task 는 *meta task* — production / 테스트 변경 0, 보고서 1건만.
**산출물**: 회귀 게이트 14 항목 + go vet 결과, 결합 분석 표, §A.4.2
            conditional 분기 cover 점검 raw dump, G3 최종 평가, F-oct 권고,
            잔여 보류 항목 갱신, Phase 1k Stage 종결 제안.
**준수**: F-oct-prelim-5-1 (`445c72d`) + 5-2 (`9f27f74`) + 5-3 (`9a749b0`)
        + F-oct-prelim-4 + F-sept-4 + F-sext-1 + ITU-T G.729 (06/2012) PDF
        §A.4.2 / §3.10 / §4.2.2 / §4.3 만 인용. 외부 G.729 구현 0건 참조.
**production 변경**: 0 라인. **테스트 변경**: 0 라인.

---

## 0. Working tree 상태 + escape hatch 종합 평가 (E1–E5)

### 0.1 Working tree pre (Step 1 직전)

```
?? internal/decoder/stagef_bis_diagnostic_test.go
HEAD = 9a749b0 test(decoder): add Stage F-oct-prelim-5-3 silence negative mechanism trace
```

- `internal/decoder/stagef_bis_diagnostic_test.go` (사전 보유 untracked) **미변경** 보존 ✅
- 5-3 commit (`9a749b0`) tip 그대로, production diff 없음.

### 0.2 Working tree post (Step 6 직전, 본 보고서 commit 직전)

```
?? internal/decoder/stagef_bis_diagnostic_test.go
?? docs/superpowers/plans/2026-05-01-phase1k-stage-f-oct-prelim-5-4-report.md
```

- 추가된 untracked 1건 = 본 보고서. 사전 보유 1건 = `stagef_bis_diagnostic_test.go`.
- `git diff -- internal/` = empty (production / 테스트 변경 0 라인) ✅
- `git diff -- docs/` = empty (기존 plan 문서 변경 0).

### 0.3 Escape hatch 종합 평가 (cycle 5-1/5-2/5-3/5-4 누적)

| Hatch | 정의 | cycle 누적 발동 여부 | 근거 |
|-------|------|---------------------|------|
| E1 | 회귀 게이트 신규 FAIL → revert | ❌ 미발동 (4 task 전부) | 14 게이트 PASS 유지. 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) 만 plan-허용 baseline FAIL — cycle 진입 전·후 동일. |
| E2 | spec § 인용 부재 / Q-format 결함 | ❌ 미발동 (4 task 전부) | 5-2 Q-FMT-1 5계수 \|Δ\|≤0.0002 정합. 본 task 인용 모두 ITU PDF + 사내 보고서. |
| E3 | 가설 2+ 잔존 → 추가 진단 / helper 보류 | ❌ 미발동 (5-3 시점 단일 잔존 후보 = M1; 본 task 의 §A.4.2 cover 점검으로 단일 확정) | 본 §3.5 dump 참조. 2+ 잔존 부재. |
| E4 | 외부 G.729 구현 인용 | ❌ 미발동 (4 task 전부) | Annex A `READMETV.txt` (5-1) + ITU-T G.729 (06/2012) PDF + 사내 보고서만. 외부 reference C 구현 0건 참조. |
| E5 | production 1+ 라인 변경 | ❌ 미발동 (4 task 전부) | 본 cycle commit 3건 (5-1/5-2/5-3) 전부 `*_test.go` 한정. 본 task = `docs/` 1건. `internal/**/*.go` 의 non-test 파일 변경 누적 0 라인. |

종합: cycle 전체 hatch 발동 0회. 가드 invariant 100% 준수.

---

## 1. F-oct-prelim-5 cycle commit 요약

| Task | Commit | 추가 파일 | 핵심 결정 |
|------|--------|-----------|-----------|
| 5-1 | `445c72d` | `stagef_octprelim5_diagnostic_test.go` (TestDiagnostic_FoctPrelim5PSTSourceVerbatim + TestDiagnostic_FoctPrelim5BitVectorCompare) | (P-SRC-2, B-CMP-2) — Annex A vs main 0/6 byte-equal. ALGTHM/PITCH/FIXED frame 0 BIT 10-byte 3-way 8/10 bit 동일 → stimulus 분기. |
| 5-2 | `9f27f74` | 위 파일에 TestDiagnostic_FoctPrelim5HpFilterInitState append | (Q-FMT-1, H-INIT-1, H-RESP-1) — hpFilter 모든 차원 §4.2.2 정합. **M2 폐기**, F-sext-2/3 종결. |
| 5-3 | `9a749b0` | 위 파일에 TestDiagnostic_FoctPrelim5SilenceNegativeMechanism append | 4-tuple = (M1 잔존, M2 폐기, M3 폐기, M4 잔존-보류). M1 cover 측정은 5-4 위임. |
| 5-4 | (본 보고서) | `2026-05-01-phase1k-stage-f-oct-prelim-5-4-report.md` | §A.4.2 cover 점검 후 단일 결정. |

---

## 2. 회귀 게이트 종합 결과 (14 항목 + go vet)

Step 1 명령어 + 결과:

| # | gate | 결과 |
|---|------|------|
| 1 | `TestDiagnostic_FoctPrelim5PSTSourceVerbatim` | PASS |
| 2 | `TestDiagnostic_FoctPrelim5BitVectorCompare` | PASS |
| 3 | `TestDiagnostic_FoctPrelim5HpFilterInitState` | PASS |
| 4 | `TestDiagnostic_FoctPrelim5SilenceNegativeMechanism` | PASS |
| 5 | `TestDecode_Frame0Sample0_MatchesALGTHM` | PASS |
| 6 | `TestDiagnostic_FquartGainReferenceCrossCheck` | PASS |
| 7 | `TestDiagnostic_FquartGainImap_Sf0Sample0to7` | PASS |
| 8 | `TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7` | PASS |
| 9 | `TestDiagnostic_FseptExcitationDecomposition_Sf0Sample5` | PASS |
| 10 | `TestDiagnostic_FseptLPReferenceCrossCheck` | PASS |
| 11 | `TestDiagnostic_FseptSynthIIRTrace_Sf0Sample0to7` | PASS |
| 12 | `TestDiagnostic_FoctPrelimPSTFormat` | PASS |
| 13 | `TestDiagnostic_FoctPrelimFrameAlignment` | PASS |
| 14 | `TestDiagnostic_FoctPrelimMultiVectorScan` | PASS |

전체 `go test ./internal/...` 출력 (요약):

```
ok    github.com/hunydev/g729/internal/bitstream  (cached)
FAIL  github.com/hunydev/g729/internal/decoder    0.014s
       --- FAIL: TestDiagnostic_SinglePulseChain (plan-허용 baseline)
ok    github.com/hunydev/g729/internal/fcb        (cached)
ok    github.com/hunydev/g729/internal/fixed      (cached)
FAIL  github.com/hunydev/g729/internal/gain       0.002s
       --- FAIL: TestDecode_LowEnergyCodebookIsSmooth (plan-허용)
       --- FAIL: TestDecode_SucceedsAcrossAllGainIndices (plan-허용)
ok    github.com/hunydev/g729/internal/lsp        (cached)
ok    github.com/hunydev/g729/internal/pcm        (cached)
ok    github.com/hunydev/g729/internal/pitch      (cached)
ok    github.com/hunydev/g729/internal/postfilter (cached)
ok    github.com/hunydev/g729/internal/synth      (cached)
ok    github.com/hunydev/g729/internal/tables     (cached)
```

`go vet ./...` → 출력 없음 (clean).

신규 FAIL 0건. 비-contract 3건 plan-허용 FAIL 외 모두 PASS. cycle 5-1/5-2/5-3 commit 직후 게이트 baseline 보존.

---

## 3. 시나리오 결합 분석 (Task 5-1 × 5-2 × 5-3)

### 3.1 측정 결과 요약 (3 task 통합)

| 차원 | 측정 결과 | 출처 |
|------|-----------|------|
| P-SRC (PST 출처 binary) | **P-SRC-2** — Annex A `READMETV.txt` 가 ANSI-C reference (G.729A) 산출물임을 verbatim 확인. main G.729 ≠ Annex A. | 5-1 §1, §3 |
| B-CMP (BIT 3-vector 비교) | **B-CMP-2** — ALGTHM vs PITCH vs FIXED frame 0 BIT 10-byte 3-way 8/10 동일, 2/10 stimulus 분기. | 5-1 §3 |
| Q-FMT (hpFilter 5계수 Q-format) | **Q-FMT-1** — 5계수 모두 \|Δ\|≤0.0002, spec real value 정합. | 5-2 §3.1 |
| H-INIT (hpFilter init state) | **H-INIT-1** — `hpX[0..1] = 0`, `hpY[0..1] = 0` zero-init, §4.3 정합. | 5-2 §3.2 |
| H-RESP (hpFilter step response) | **H-RESP-1** — zero-input → 0, impulse → 식 (151/152) 산술 정합. | 5-2 §3.3 |
| M1 (postfilter conditional 음수 감쇠) | **잔존** (cover 미측정 — 본 task §3.5 에서 점검) | 5-3 §3.4 |
| M2 (hpFilter 음수 감쇠) | **폐기** (Q-FMT-1+H-INIT-1+H-RESP-1) | 5-3 §3.4 |
| M3 (synthesis memory 비-0 init) | **폐기** (D 17 contract PASS + zero dump) | 5-3 §3.4 |
| M4 (PST 자체 결함 부재 = G3 폐기) | **잔존-보류** (M1 폐기 시 단일 채택) | 5-3 §3.4 |

### 3.2 결합 매핑 표 (plan §Step 2)

| Task 5-1 | Task 5-2 | Task 5-3 | F-oct 권고 |
|----------|----------|----------|------------|
| (P-SRC-2, B-CMP-2) | (Q-FMT-1, H-INIT-1, H-RESP-1) | (M1 잔존, M2/M3 폐기, M4 잔존-보류) | **(a) 또는 (b) 분기** — M1 §A.4.2 conditional cover 점검 결과에 의존. |

P-SRC-2 (PST 출처 = Annex A ANSI-C reference) 는 우리 chain 외부 ground-truth 정합성을 약화하지 않음 — Annex A는 ITU 공식 reference. cycle 5-1 의 6/6 byte-equal 부재는 *Annex A vs main 분기* 의 직접 증거이지만, 본 cycle 의 ground-truth 는 일관되게 Annex A `algthm.pst` 등 (= F-quart/F-sept/F-sext 와 동일 vector) — 즉 *PST 자체* 는 우리 측정과 일관. 따라서 분기 유효 위치는 *우리 chain 내부 § 인용 정합성*으로 환원.

### 3.3 5-3 단일 잔존 후보 = M1 — 분기 cover 점검 필요성

5-3 §3.4 는 M1 의 spec 인용 conditional 분기 (long-term postfilter `g_l`, tilt comp `γ_t` voicing 의존, AGC) 이 본질적으로 cover 측정 외 라고 명시. 본 task 가 plan 지시대로 grep 으로 spec 인용 + 분기 점검을 수행 (production 변경 0).

### 3.4 cover 점검 절차 (plan task 본문 §A.4.2 cover 점검)

명령어:

```
grep -rn "A.4.2\|Hp(z)\|Ht(z)\|long-term post\|tilt comp\|AGC" internal/postfilter/
```

대상: `internal/postfilter/*.go` 중 production (`.go` non-test).

### 3.5 cover 점검 raw dump (production-only)

```
internal/postfilter/agc.go:3:        // computeAGCTargetGain returns the AGC target gain g_target (Q14) per
internal/postfilter/agc.go:4:        // ITU-T G.729 §A.4.2.4 as √(E(s) / E(sTilt)).
internal/postfilter/agc.go:47:       // applyAGC smooths g_target into agcGainPrev (one-pole lowpass, α ≈ 0.99)
internal/postfilter/agc.go:48:       // and scales sTilt to produce sPf per ITU-T G.729 §A.4.2.4.
internal/postfilter/agc.go:50:       const alphaQ15 int64 = 32440 // ≈ 0.99; ITU-T G.729 §A.4.2.4
internal/postfilter/bandwidth.go:4:  // per ITU-T G.729 §3.10.1 / §A.4.2.1.
internal/postfilter/doc.go:2:        // chain per ITU-T G.729 §A.4.2. ...
internal/postfilter/doc.go:9:        // Per ITU-T G.729 §A.4.2.1 / §A.4.2.2 / §A.4.2.3 / §A.4.2.4: ...
internal/postfilter/doc.go:20:       // value is a valid reset state per §A.4.2 first-frame initialisation.
internal/postfilter/doc.go:33:       // All coefficients and formulas derive from ITU-T G.729 §A.4.2 directly.
internal/postfilter/longterm.go:5:   // residual per ITU-T G.729 §A.4.2.2.
internal/postfilter/longterm.go:49:  // g1 = g_l/(1+g_l) (Q14) for the long-term postfilter, where
internal/postfilter/longterm.go:53:  // per ITU-T G.729 §A.4.2.2 with γ_l = 0.5 (Annex A bound).
internal/postfilter/longterm.go:59:  const gammaLQ14 int16 = 8192 // = 0.5; ITU-T G.729 §A.4.2.2
internal/postfilter/longterm.go:99:  // applyLongTerm filters the residual with the long-term postfilter
internal/postfilter/longterm.go:103: // per ITU-T G.729 §A.4.2.2.
internal/postfilter/postfilter.go:3: // Annex A postfilter constants per ITU-T G.729 §A.4.2.
internal/postfilter/postfilter.go:10:// ITU-T G.729 §A.4.2.
internal/postfilter/residual.go:9:   // per ITU-T G.729 §A.4.2.1, updating pastS for the next subframe.
internal/postfilter/shortterm.go:9:  // per ITU-T G.729 §A.4.2.1, using pastSynthPost as the 10-tap IIR memory.
internal/postfilter/tilt.go:4:       // §A.4.2.3 — "the impulse response … is truncated after 22 samples".
internal/postfilter/tilt.go:16:      // cascade A(z/γ_n)/A(z/γ_d) per ITU-T G.729 §A.4.2.3:
internal/postfilter/tilt.go:83:      // per ITU-T G.729 §A.4.2.3.
internal/postfilter/types.go:10:     // §A.4.2. The zero value is a valid Reset state.
internal/postfilter/types.go:18:     // agcGainPrev is the AGC gain used in the last sample of the previous
internal/postfilter/types.go:21:     // seeded to g_target per ITU-T G.729 §A.4.2.4 initialization.
```

### 3.6 conditional 분기 별 cover 점검 (단일 결론)

각 §A.4.2 conditional 분기 vs 우리 production 코드의 정합성:

| # | spec 분기 (§A.4.2) | 우리 production 위치 | 정합 | 비고 |
|---|---------------------|----------------------|------|------|
| C1 | `g_l = clamp(R(T)/E(T), 0, γ_l)` with `γ_l = 0.5` (long-term postfilter gain, §A.4.2.2) | `longterm.go:71-81` (`R<=0\|\|E==0 → return 16384,0`; `gRawQ14>γ_l → clamp γ_l`) | ✅ 정합 | spec 인용 line 53/59 + γ_l = 8192 (Q14 = 0.5). |
| C2 | k_1 = -r_h(1)/r_h(0) (tilt comp 부호, §A.4.2.3) | `tilt.go:55-63` (`rh0==0 → 0`; saturate ±32768) | ✅ 정합 | k_1 부호는 r_h(1) 부호의 반전 — 결정 분기 없음, 산술 직접. |
| C3 | **γ_t = 0.9 if long-term postfilter active (g_l > 0), else 0.2** (tilt comp voicing-dependent gain, §A.4.2.3) | `tilt.go:65-68` — `gammaTQ14 = gammaTiltActiveQ14 (0.9); if pf.agcGainPrev == 0 { gammaTQ14 = gammaTiltInactiveQ14 (0.2) }` | ❌ **결손** | docstring `tilt.go:23-25` *명시적*: "for Phase 1g we consult pf.agcGainPrev as a *proxy* for 'long-term active' (non-zero) vs 'inactive' (zero)". spec 의 voicing 판정 = *현 subframe* `g_l > 0`; 우리 구현 = *전 subframe* AGC 출력 gain. 두 quantity 는 동일하지 않으며, 특히 frame 0 sf0 (cold start, agcGainPrev = 0 zero-value) 에서 *항상* γ_t = 0.2 (inactive) branch 진입 — `g_l` 값과 독립. |
| C4 | AGC α-smoothing + first-call seed (§A.4.2.4) | `agc.go:47-50, 73-76` + `types.go:18-21` (initialized flag → seed agcGainPrev = g_target Q24) | ✅ 정합 | Phase 1i §A.4.2.4 init fix 인용. contract test (qformat_contract_test.go) PASS. |

### 3.7 cover 결손 단일 식별

C3 = γ_t 분기 cover **결손 확정**. proxy 사용은 production code 의 docstring 이 자체 인정 — hidden divergence 가 아니라 *문서화된* spec 미준수. frame 0 sf0 (cold start) 에서 spec 의 `g_l > 0` 판정과 우리 구현의 `agcGainPrev == 0` 판정이 시점 차이로 결과가 다를 수 있는 *유일한* §A.4.2 conditional 분기.

→ **M1 잔존 확정** (cover 결손).
→ M4 단일 채택 불가.
→ F-oct = (a) postfilter production fix cycle 권고.

---

## 4. 가설 G3 최종 평가 (단일 식별 / 폐기 / 모순)

가설 G3: "Annex A vs main spec 분기 거동" — 우리 chain 내 분기 위치를 단일 식별.

| 가설 | 5-3 분류 | 본 task 결정 |
|------|----------|--------------|
| M1 (postfilter conditional 분기) | 잔존 | **단일 채택** — §3.6 C3 (tilt γ_t 분기 proxy) cover 결손 확정. |
| M2 (hpFilter) | 폐기 (5-2) | 그대로 |
| M3 (synth memory init) | 폐기 (5-3 §3.3 zero dump) | 그대로 |
| M4 (PST 자체 결함 부재 = G3 폐기) | 잔존-보류 | **반증** — M1 단일 채택으로 G3 폐기 가설 채택 불가. |

**G3 최종 평가 = 분기 위치 단일 식별** — `internal/postfilter/tilt.go:65-68` 의 γ_t 선택 분기. spec §A.4.2.3 의 voicing 판정 (`g_l > 0` 현 subframe) ≠ 우리 proxy (`agcGainPrev == 0` 직전 subframe AGC 출력).

E3 (2+ 잔존 모순) 미발동. 강압-적합 회피 의무 (Phase 0.4) 준수: M1 채택 근거는 production code self-documented divergence — 측정 데이터의 우회 적합 아님.

---

## 5. 잔여 보류 항목 갱신

F-oct-prelim-4 §5 의 9 항목 본 cycle 갱신:

| # | 항목 | 직전 상태 | 본 cycle 갱신 |
|---|------|-----------|--------------|
| 1 | F-oct (production fix / plan-end / 추가 진단) | F-oct-prelim-5 권고 | **(a) postfilter production fix 권고** — §6 단일 결정 |
| 2 | filterSubframe ÷4/×4 | F-quint-3 §4.1 동상 | 미갱신 |
| 3 | β init = 0.2 | F-quint-3 §4.2 동상 | 미갱신 |
| 4 | frame 1+ 잔여 | F-oct-prelim-2 frame 1..3 alignment 정합 | 미갱신 (F-oct cycle 후 재평가) |
| 5 | 회귀 가드 promotion | F-oct-prelim-3 §5 promotion 금지 강화 | 미갱신 |
| 6 | 비-contract diagnostic 3건 | F-quint-3 §4.6 동상 | 미갱신 (Phase 1k Stage 외) |
| 7 | F-sext-2 / F-sext-3 reactivate | F-oct-prelim-4 §5 reactivate 검토 | **종결** — Task 5-2 의 H-RESP-1 으로 흡수 (F-sext-2/3 폐기) |
| 8 | `lsp_lp.go` uncommitted | F-sept-2 시점 정식화 (`02bf785`) 완료 | 미갱신 |
| 9 | `stagef_bis_diagnostic_test.go` untracked | 보존 유지 | 미갱신 (F-bis cycle 종결 시 별도 commit 검토) |

신규 보류 항목 (본 task 산출):

| # | 항목 | 비고 |
|---|------|------|
| 10 | `tilt.go:65-68` γ_t 분기 proxy → spec 정합 (현 subframe `g_l > 0`) production fix | F-oct (production fix cycle) 의 단일 fix point. 식별 근거 = 본 §3.6 C3. |

---

## 6. F-oct 권고 단일 결정 (a / b / c)

**결정: (a) postfilter production fix cycle**

근거:
1. §3.6 cover 점검에서 C3 (tilt γ_t voicing 판정) 가 spec §A.4.2.3 와 *명시적* divergence — production docstring `tilt.go:23-25` 가 proxy 사용 자체 인정.
2. 5-3 4-tuple 의 M1 잔존 → cover 측정 결손 확정 → M1 단일 채택.
3. 단일 fix point 식별 (보류 항목 #10) → fix cycle 진입 가능.
4. M4 (G3 폐기) 채택 불가 — (b) plan-end declared 배제.
5. 2+ 잔존 부재 — (c) 추가 진단 cycle 배제.

(b) / (c) 배제 근거가 명확하므로 강압-적합 회피 의무 (Phase 0.4) 위반 없음.

---

## 7. 결론 — Phase 1k Stage F-oct-prelim-5 closure

### 7.1 cycle 결산

- F-oct-prelim-5 (4 task) 종결. production 변경 0 라인 누적, 외부 G.729 0 참조, hatch 발동 0회.
- 가설 G3 분기 위치 단일 식별: `internal/postfilter/tilt.go:65-68` γ_t 선택.
- F-oct 권고: (a) postfilter production fix cycle.

### 7.2 다음 cycle 권고

**Phase 1k Stage F-oct-postfix** (가칭) 진입 권고:

- **목표**: `tilt.go` γ_t 선택 분기를 spec §A.4.2.3 정합으로 production fix.
  spec 의 voicing 판정 = 현 subframe long-term postfilter active 여부 = `g_l > 0` (computeLongTermGain 출력 g1 의 0 여부 또는 `R/E > 0` 결정).
- **scope 한정**: 단일 함수 (`computeTiltMu`) 시그니처 확장 또는 `Postfilter` state 에 현 subframe `g_l` (또는 boolean active flag) 추가. `applyTiltWithMu` / `Filter` 호출 chain 내 한 line 수정.
- **회귀 게이트**: 본 cycle 14 게이트 + postfilter package contract test + ALGTHM frame 0 sample 5..7 부호 (got `+2` → spec want `-1` 방향 변화) 측정. fix 후 spec want 정합 또는 |Δ| 감소 확인.
- **중단 조건**: fix 후 ALGTHM frame 0 sample 5..7 양수 잔존 또는 회귀 신규 FAIL → revert + 추가 진단 cycle 환원.

### 7.3 Phase 1k 종결 가능성

본 task 시점 Phase 1k 종결은 *권고하지 않음*. F-oct-postfix cycle 완료 후
ALGTHM frame 0 BIT-EQUAL 또는 sample 부호 정합 확인 시점에 Phase 1k 종결
재평가. fix 가 결함 해소를 입증하지 못할 경우 잔여 §A.4.2 분기 재점검 또는
Annex A binary 행동 추적 cycle (F-oct-prelim-5-4 결정 트리의 maps row
P-SRC-2 추가 진단) 으로 분기.

---

**End of report.**
