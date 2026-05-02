# Phase 1k Stage F-non-prelim-X-split-2 보고서 — Cβ gain g_c sub-stage 분리

**작성일**: 2026-05-04
**범위**: 후보 Cβ (`gain.Decoder.Decode` 의 g_c=+4153 부호 결정) sub-stage (GA/GB VQ table entry / MA predictor / γ̂ 결합 / sign 처리) 분리 측정.
**산출물**: 측정 함수 1 추가 (`TestDiagnostic_FnonPrelimXSplit2GainGcTrace`) + sub-stage별 raw + ROM cross-ref + Cβ verdict.
**준수**: F-non-prelim Task 1 의 g_c=+4153 raw 측정 baseline 인계, F-non-prelim-X-split-1 (`fd0b381`, Cα-refute) verdict 와의 cross-check.

## 0. escape hatch 평가 + spec § PDF verbatim 인용

E1–E5 모두 false. production 변경 0 라인 (E5). 외부 G.729 implementation 0 참조 (E4) — Annex A binary 거부. 본 cycle 의 사전 보유 working tree (`internal/decoder/stagef_bis_diagnostic_test.go` untracked) 는 미변경 (Phase 0.5).

PDF `pdftotext -layout docs/superpowers/specs/itu/G729E.pdf` verbatim grep 결과:

- §3.9 eq. (65): `g_c = γ · g_c'` (g_c' = predicted gain, γ = correction factor).
- §3.9.1 eq. (69): `Ẽ(m) = Σ_{i=1..4} b_i · Û(m−i)` with `[b1 b2 b3 b4] = [0.68 0.58 0.34 0.19]`.
- §3.9.1 eq. (71): `g_c' = 10^((Ẽ(m) + Ē − E)/20)`, `Ē = 30 dB`.
- §3.9.2 eq. (73): `ĝ_p = GA1(GA) + GB1(GB)` (Q14, conjugate-structure two-stage VQ).
- §3.9.2 eq. (74): `ĝ_c = g_c' γ̂ = g_c' (GA2(GA) + GB2(GB))` (γ̂ Q13).
- §A.3.9 verbatim: "Same as described in clause 3.9." (decoder-side 동일).
- §4.3 Table 9: "All static encoder and decoder variables should be initialized to zero, except the variables listed in Table 9." past_err[i] (gain MA predictor history) 는 zero 초기화이나 production `gain.pastErrorsDefault = -14336 (Q10) = MIN_GAIN_PRED_DB ≈ -14 dB` 로 first-call seed (zero state 등가성 확보 위함; spec 의 `Û(0)=0` 가산 효과 회피용 디폴트, ITU reference 와 합치). 본 측정에서 verbatim 그대로 사용.

GBK 수치 ROM 은 PDF text 에 미인쇄 (dimension 만 명시: GA = 8×2, GB = 16×2). `internal/tables/gain_gbk1.go` / `gain_gbk2.go` header (merger-doctrine exception) 에 따라 ITU reference data array 를 bit-exact 전사. 본 측정의 ROM cross-ref 는 dimension + sign 일치 + 합산 결과 검증으로 한정.

## 1. 회귀 게이트 (16 PASS + 항목 17 RED + 신규 Cα+Cβ PASS)

명령:
```
go test ./internal/decoder/ -run "<plan §0.3 의 16 게이트 + Cα + Cβ>" -count=1
go test ./internal/decoder/ -run "TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput" -count=1
go vet ./...
```

결과:
- 16 contract gate (#1–#16) + Cα (`TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace`) + Cβ (`TestDiagnostic_FnonPrelimXSplit2GainGcTrace`) → **PASS**.
- 항목 17 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`) → **RED 유지** (frame 0 sample 5/6/7: got=2 want=−1 Δ=3, F-non-prelim baseline 변동 없음).
- `go vet ./...` clean.
- 비-contract diagnostic 3건 (`TestDiagnostic_SinglePulseChain`, `TestDecode_LowEnergyCodebookIsSmooth`, `TestDecode_SucceedsAcrossAllGainIndices`) FAIL 유지 (production 변경 0 — 자동 보장).

## 2. Cβ 측정 raw 출력 (verbatim t.Logf)

```
indices: P1=2  C1=0x0000  S1=0xf  GA1=5  GB1=6
[Cβ idx.GA1, GB1]    GA1=5 (3 bits)  GB1=6 (4 bits)
[Cβ Imap]            GainImap1[5]=0  GainImap2[6]=1   (§3.9.3 inverse permutation)
[Cβ GA[0] entry]     GainGBK1[0] = (g_p_ga=+1   Q14, γ̂_ga=+1516 Q13)
[Cβ GB[1] entry]     GainGBK2[1] = (g_p_gb=+1994 Q14, γ̂_gb=+0    Q13)
[Cβ γ̂ correction]    eq.(74) γ̂ = +1516 + +0 = +1516 (Q13)
[Cβ ĝ_p compose]     eq.(73) ĝ_p = +1 + +1994 = +1995 (Q14)   gain.Decode return ĝ_p=+1995  match=true
[Cβ MA predictor]    past_err init=[-14336 -14336 -14336 -14336] (Q10, MIN_GAIN_PRED_DB)
                     b=[+5571 +4751 +2785 +1556] (Q13; spec [0.68 0.58 0.34 0.19])
                     LMac acc(Q24)=-420417536  Round(LShl(acc,2))=-25660 (Q10)
                     Ê(m) = Ē + Σb·Û = +30720 + -25660 = +5060 (Q10 dB)
[Cβ fcb energy]      Σc² (Q26) = 279180740
[Cβ g_c (Q12)]       gain.Decode → (ĝ_p=+1995 Q14, ĝ_c=+4153 Q12)   X-fcb verdict +4153 match=true
[Cβ ROM cross-ref]   GainGBK1 dim=8×2  GainGBK2 dim=16×2  γ̂_ga sign=+  γ̂_gb sign=0  γ̂ sum sign=+
[Cβ 결정]            sign-determining sub-stage = VQ-table-γ̂ (γ̂ = +1516 > 0; predictor finite; γ̂·g_c' 부호 보존)
[Cβ verdict]         Cβ-refute (g_c=+4153 양 부호 = §3.9 spec-canonical)
```

## 3. Cβ 후보 평가

| sub-stage | 측정 | spec 정합 |
|-----------|------|-----------|
| (a) GA1=5 / GB1=6 codeword | bitstream verbatim | 정합 |
| (b) GainImap1[5]=0 / GainImap2[6]=1 | §3.9.3 역치환 일치 | 정합 |
| (c) GainGBK1[0] = (+1 Q14, +1516 Q13) | dimension §3.9.2 일치, γ̂_ga=+1516 > 0 | 정합 |
| (d) GainGBK2[1] = (+1994 Q14, +0 Q13) | dimension §3.9.2 일치, γ̂_gb=0 (sign loss 없음) | 정합 |
| (e) γ̂ = +1516 (Q13) > 0 | eq.(74) 합산 부호 양 | 정합 |
| (f) MA predictor Ê(m) = +5060 (Q10) | frame 0 zero-state seed (past_err = MIN_GAIN_PRED_DB) | 정합 |
| (g) g_c = γ̂·g_c' = +4153 (Q12) | eq.(65) 부호 보존 (γ̂ > 0 → g_c > 0) | 정합 |
| (h) X-fcb verdict cross-ref | F-non-prelim Task 1 §2 의 g_c=+4153 raw 일치 | 정합 |

**Cβ verdict: Cβ-refute (spec 정합).** sign 결정 sub-source = **단일 = VQ-table γ̂**. MA predictor 기여 없음 (predictor 는 magnitude 만 결정, 부호 영향 0). sign-mask 또는 별도 sign 처리 로직 0건. composition (eq.(65) 곱) 부호 보존 정합.

## 4. F-non-prelim Task 1 §2 raw cross-check + ROM verbatim cross-ref

- F-non-prelim Task 1 (`d1a4f2d` 직전 `db…`) §2: `g_c · c[n]` Q15 pre-Round = +33224. 본 측정 g_c=+4153 (Q12) × c[n]=+8192 (Q13) >> 13 = +4153 × +1 = +4153 (Q12). Round / shift 후 합치.
- F-non-prelim-X-split-1 (`fd0b381`) Cα verdict = **Cα-refute** (c[0..3]=+8192 spec-canonical).
- 본 task Cβ verdict = **Cβ-refute** (g_c=+4153 spec-canonical).
- ROM verbatim cross-ref: PDF §3.9.2 는 GBK 수치 ROM 미인쇄 (dimension 만). production `tables.GainGBK1[0] = {1, 1516}` / `tables.GainGBK2[1] = {1994, 0}` 는 ITU reference data array 의 merger-doctrine exception 전사 — 본 cycle 의 검증은 dimension + sign + 합산 일치로 한정.

## 5. Task 3 (synthesis) 진입 의무

Phase 0.4 §6: hybrid 결정 강요 금지. 본 cycle 측정 결과:
- **Cα-refute + Cβ-refute** → "둘 다 spec 정합" (hybrid 반증).
- Phase 0.4 §3 / Task 3 §3 결정 트리: production fix scope 후보 0 (fcb / gain 모두 결함 없음).
- 권고: **Cγ 재진입** (synthesis §3.4 의 Cγ LOW priority dismiss 의 재고) 또는 **Y magnitude follow-up cycle** (max|Δ|=6, `d1a4f2d` §3.1).
- Cδ 재진입 절대 금지 (Phase 0.4 §7) — 본 verdict 가 Cα/Cβ 정합이어도 Cδ 트리거 금지.

Task 3 (F-non-prelim-X-split-3) 진입 시 Cα/Cβ 비교표 + 단일 결정 (Cγ 재진입 또는 Y follow-up) 작성 의무.
