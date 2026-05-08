# Phase 1k Stage F-non-prelim-X-split-1 보고서 — Cα fcb c[n] sub-stage 분리

**작성일**: 2026-05-04
**범위**: 후보 Cα (`fcb.Decode` 의 c[0..3]=+8192 부호 결정) sub-stage (idx.Positions / idx.Signs / raw placement c_raw / β enhancement Δ) 분리 측정.
**산출물**: 측정 함수 1 신규 파일 (`internal/decoder/stagef_fnonprelim_xsplit_diagnostic_test.go`) + sub-stage별 raw + 부호 결정성 평가 + Cα verdict.
**준수**: production 변경 0 라인, 외부 G.729 0 참조 (Annex A binary 거부 — G1 결정 정합), F-non-prelim Task 1 (`f82893d`) 의 g_c·c 곱 측정 baseline 인계.

---

## 0. Working tree 상태 + escape hatch 평가 (E1–E5) + 사용자 G-S2/G-S3 결정 정합성

진입 시점 working tree:

```
?? internal/decoder/stagef_bis_diagnostic_test.go   (Phase 0.5 보존 의무 — 미변경)
HEAD = 49fac32 docs(plans): add Phase 1k Stage F-non-prelim-X-split plan
```

신규 파일 2건 (이 commit) — `stagef_fnonprelim_xsplit_diagnostic_test.go` + 본 보고서. 다른 working tree 변경 없음.

E1: 회귀 게이트 16 PASS + 항목 17 RED 의도 잔존 (§1) — 미발동.
E2: spec § 인용은 PDF `pdftotext -layout` verbatim grep 채택 (§2) — 미발동.
E3: Task 1 단독, Task 3 종합 시점 사항 — 본 task 평가 대상 아님.
E4: 외부 G.729 구현 인용 0건. Annex A binary 미사용. PDF + 자체 production code 만 인용 — 미발동.
E5: production 변경 0 라인 (`git diff HEAD --stat` = 0 production file) — 미발동.

사용자 G-S2 (hybrid 진단 cycle 승인) + G-S3 (X-split 진입 승인) 결정 준수.

Phase 0.4 §1 (Cα/Cβ 임의 우선 금지): 본 task 측정-only, verdict 는 raw placement / β enhancement 측정 데이터로만 도출.
Phase 0.4 §3 (음성 결과 = 유효 결과): 본 task verdict = **Cα-refute (spec 정합)** — 부정 결과를 그대로 보고.
Phase 0.4 §6 (hybrid 강요 금지): 본 task 측정으로 Cα 단독 spec 정합 도출 → Task 2 (Cβ) 측정 후 hybrid 결정 재고 (§5 항목).

---

## 1. 회귀 게이트 baseline (16 PASS + 항목 17 RED + 신규 PASS)

```
$ go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -count=1
ok  	github.com/hunydev/g729/internal/decoder	0.001s

$ go test ./internal/decoder/ -run TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput -count=1
--- FAIL: TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput
    stagef_octpostfix_regression_test.go:38: frame 0 sample 5: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 6: got=2 want=-1 (Δ=3)
    stagef_octpostfix_regression_test.go:38: frame 0 sample 7: got=2 want=-1 (Δ=3)
FAIL    (← 항목 17 의도 RED 잔존; 다음 fix cycle GREEN gate)

$ go test ./internal/decoder/ -run "TestDiagnostic_F(quart|sext|sept|octPrelim|OctPrelim5|OctPostfix2Prelim|nonPrelim)" -count=1
ok  	github.com/hunydev/g729/internal/decoder	0.010s

$ go test ./internal/postfilter/ ./internal/synth/ -count=1 -run Contract
ok  	github.com/hunydev/g729/internal/postfilter	0.001s
ok  	github.com/hunydev/g729/internal/synth	0.001s

$ go vet ./...
(clean)
```

회귀 게이트 16 PASS + 항목 17 RED 의도 잔존 (≡ F-non-prelim 종결 시점 게이트와 동일). 신규 회귀 0건.

신규 측정 harness PASS:

```
$ go test ./internal/decoder/ -run TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace -v -count=1
=== RUN   TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace
--- PASS: TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace (0.00s)
PASS
```

Phase 0.3 §94 정합 — 자동 promotion 하지 않음 (Task 3 §3 잔여 보류 항목).

---

## 2. sub-stage raw 출력 (sample 0..39, focus 0..3)

PDF `pdftotext -layout docs/superpowers/specs/itu/G729E.pdf` verbatim 인용 (행 번호는 layout 출력):

- §3.8 eq.(45): `c(n) = s0 δ(n − m0) + s1 δ(n − m1) + s2 δ(n − m2) + s3 δ(n − m3)` (line 1244).
- §3.8 eq.(47): `β = ĝ_p^(m−1)   bounded by 0.2 ≤ β ≤ 0.8` (line 1257).
- §3.8 eq.(48): `c(n) = c(n)              n = 0..T−1` / `c(n) = c(n) + β c(n − T)   n = T..39` (line 1264-1267).
- §3.8.2 eq.(61): `S = s0 + 2 s1 + 4 s2 + 8 s3` (s=1 → +1, s=0 → −1) (line 1320+).
- §A.3.8.2 (line 2189): "Same as described in clause 3.8.2." — 디코더측 동일.
- §4.3 Table 9 (line 1699): past_exc = 0, past gain state init.

ALGTHM frame 0 sf0 측정 결과 (`TestDiagnostic_FnonPrelimXSplit1FcbPulseTrace` 출력 verbatim):

```
indices: P1=2  C1=0x0000  S1=0xf  GA1=5  GB1=6
pitch delay: tInt=20  tFrac=0   beta_q14_prod=3277 (clamp(g_p_prev=0) → 0.2·2^14)

[Cα idx.Positions]   raw=0x0000  decoded fields i0=0 i1=0 i2=0 jx=0 i3=0  →  m=[0,1,2,3]
[Cα idx.Signs]       raw=0xf     decoded s=[+1,+1,+1,+1]
[Cα c_raw[0..39]]    nonzero @ n=[0 1 2 3]
   c_raw[0] = +8192  (track 0, sign +)
   c_raw[1] = +8192  (track 1, sign +)
   c_raw[2] = +8192  (track 2, sign +)
   c_raw[3] = +8192  (track 3, sign +)
[Cα c_raw[0..3]]     = [+8192 +8192 +8192 +8192]
[Cα c_prod[0..3]]    = [+8192 +8192 +8192 +8192]
[Cα Δ[0..3]]         = [0 0 0 0]                    (β·c[n−T] off — n<T=20 정합)
[Cα Δ[0..39] nonzero indices] = [20 21 22 23]       (= β·c_raw[0..3] enhancement, n≥T 정합)
[Cα c[0..3] vs X-fcb verdict (+8192,+8192,+8192,+8192)]  match=true
```

추가 cross-check: production `fcb.Decode` 호출 (clamp(0)=Q14 3277, T=20) 의 enhancement Δ 가 `n=20..23` 에서 비-0 으로 처음 발생 — `Δ[20] = round(2·β·c_raw[0])` 가 `c_raw[0..3]=+8192` 의 *결과* (양 부호) 로 누적되는 것은 §3.8.2 eq.(48) n≥T 분기 정합. sample 0..3 영역에는 무관.

---

## 3. Cα sub-stage 부호 결정성 평가

| sub-stage | 측정 값 | spec § 정합 |
|-----------|--------|-------------|
| (a) idx.Positions = 0x0000 → m=[0,1,2,3] | 4-pulse 위치 모두 sample 0..3 영역 (track 0/1/2/3) | §3.8 Table 7 (track i 각 8 positions) — m=0,1,2,3 모두 Table 7 valid entry (i_k=0). 정합. |
| (b) idx.Signs = 0xf → s=[+1,+1,+1,+1] | 4-bit mask 모두 1 → §3.8.2 eq.(61) s_i=1 ↔ +1 | spec verbatim 정합. |
| (c) c_raw[0..3] = [+8192,+8192,+8192,+8192] | §3.8 eq.(45) `c(n) = Σ s_k δ(n−m_k)` 직접 적용 결과 | spec 정합. |
| (d) Δ[0..3] = [0,0,0,0]; Δ[20..23] ≠ 0 | §3.8.2 eq.(48) n<T branch (T=tInt=20) → enhancement off; n≥T branch → enhancement on | spec verbatim 정합. |
| (e) c_prod[0..3] = [+8192,+8192,+8192,+8192] | (c) + (d) 합 | X-fcb verdict (`f82893d`) 와 일치. |

**부호 결정 sub-stage**: **raw-placement** — c[0..3] 의 양 부호는 (a)+(b)+(c) 만으로 결정. (d) 의 기여 = 0 (n<T 영역 enhancement off, §3.8.2 eq.(48) verbatim 정합).

**spec 정합성**: 4 sub-stage (a~d) 모두 ITU-T G.729 (06/2012) §3.8 + §3.8.2 + §A.3.8.2 verbatim 정합. fcb.Decode 결함 0건.

---

## 4. F-non-prelim Task 1 (`f82893d`) §2 의 c[0..4] dump 와의 cross-check

| 측정 항목 | F-non-prelim-1 (`f82893d`) | 본 task | 일치 |
|-----------|----------------------------|---------|-------|
| c[0..3] (Q13) | `[+8192,+8192,+8192,+8192]` | `[+8192,+8192,+8192,+8192]` | ✓ |
| tInt | 20 | 20 | ✓ |
| C1 codeword | (인용 없음, 본 task 신규) | `0x0000` | — |
| S1 codeword | (인용 없음, 본 task 신규) | `0xf` | — |
| pulse positions m | (인용 없음, 본 task 신규) | `[0,1,2,3]` | — |
| pulse signs s | (인용 없음, 본 task 신규) | `[+1,+1,+1,+1]` | — |
| β·c[n−T] enhancement on n=0..3 | (측정 미수행) | `Δ[0..3]=0` (off, n<T) | — |

본 task 의 sub-stage 분리 측정으로 F-non-prelim-1 X-fcb verdict 의 c[0..3]=+8192 origin 이 **raw 4-pulse placement** 임을 식별. β enhancement 는 sample 0..3 에 영향 0, sample 20..23 에 후속 영향 (별도 cycle scope 외).

---

## 5. Cα 가설 평가 + Task 2 (Cβ) 진입 의무 항목

### Cα 가설 평가

후보 (plan §Task 1 Step 5 결정 트리):

- **Cα-raw**: raw placement 단독 결정 + spec 정합 미확인 — 미해당 (sub-stage 모두 spec verbatim 정합 확인됨)
- **Cα-enh**: β-enhancement 가 c[0..3] 부호 결정 — 미해당 (Δ[0..3]=0)
- **Cα-refute**: 모든 sub-stage spec 정합 + c[0..3]=+8192 = spec-canonical 출력 — **채택**
- **Cα-inconclusive**: 측정 데이터로 식별 불가 — 미해당 (식별 완료)

**최종 verdict: Cα-refute**. c[0..3] 의 양 부호는 ITU-T G.729 (06/2012) §3.8 + §3.8.2 + §A.3.8 verbatim 정합. `fcb.Decode` 결함 없음. Cα 단독 fix scope 영구 폐기.

### Task 2 (Cβ) 진입 의무 항목

1. F-non-prelim-X-split-2 진입: `gain.Decoder.Decode` g_c (Q12) = +4153 의 sub-source 분리 측정 (GA[5] + GB[6] VQ table entry / MA predictor frame 0 zero-state / 합산 g_c 부호 결정) — plan §Task 2 따름.
2. 본 task 측정 결과 (Cα-refute) 가 Task 3 §3 결정 트리의 분기 변수 — Cβ 측정 결과에 따라:
   - Cβ-refute → 둘 다 spec 정합 → Cγ 재진입 또는 Y magnitude follow-up (Phase 0.4 §3 + §7)
   - Cβ-단독 결함 → F-non-fix-gain 단독 fix cycle 권고
3. Phase 0.4 §6 (hybrid 강요 금지): 본 task verdict 가 *선험적으로* Cβ 단독 결정으로 강요 금지 — Task 2 측정 데이터로 결정.
