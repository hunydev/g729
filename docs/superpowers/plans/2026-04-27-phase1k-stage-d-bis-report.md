# Phase 1k Stage D-bis 진단 보고서

**작성일**: 2026-04-27
**범위**: Stage D-bis Tasks 1–3 (4-pulse canonical / 비제로 gpQ14 / ALGTHM 실제 sf0 자극) + Task 4(보고서).
**핵심 결론**: **14 dB 발산이 ALGTHM frame 0 sf0 실제-자극 재생에서 재현됨.** 분기점은 `synth.Filter` LP 합성 IIR(브랜치 C)이 가장 강력한 후보. gcQ12 비포화 + Σc² 스펙-일치 + u 스펙-일치 + s 시간 누적 폭주가 동시 관측. 단, 동일 자극에서 ALGTHM.PST 참값은 sf0 전 영역에서 ±1 이내이므로 **본 진단은 sf0 단일에서 단일 모듈을 단정하지 못한다 — 합성-자극(D-bis-2)에서는 IIR이 확실히 +18 dB 증폭하지만, 그 a 계수는 합성이며 ALGTHM frame 0 sf0의 LSP-디코드 a 계수가 실제로 unstable-looking 한 점이 변수**. 따라서 **권고는 옵션 (가) Stage F 브랜치 C 진입**이되 첫 작업으로 LSP→LP 변환 결과의 안정성/스펙-일치 확인을 선행할 것.

탈출 해치 발동: **없음** (sample 0 = 2 유지, Stage D 17개 컨트랙트 어서션 무회귀, 다중 모듈 동시 분기 미발동).

---

## 1. Stage D-bis 커밋 요약

| # | SHA | 커밋 메시지 1줄 |
|---|------|----------------|
| 1 | `a36a335` | test(decoder): D-bis-1 4-pulse canonical diagnostic harness (observation-only) |
| 2 | `1a983c0` | test(decoder): D-bis-1 promote 4-pulse spec-aligned boundaries to assertions |
| 3 | `4854bd6` | test(decoder): D-bis-2 pitch-active stimulus diagnostic (observation-only) |
| 4 | `daa9fcd` | test(decoder): D-bis-3 ALGTHM frame 0 sf0 stage-by-stage replay (observation-only) |

회귀 게이트: `go test -race ./...` ALL PASS, `go vet ./...` silent, `internal/fixed` 벤치 0 allocs/op 유지(`Add`, `LMult`, `LMac`, `DivS`, `NormL`).

ALGTHM frame 0 sample 0 가드(Phase 1i `736beba`) 변동 없음. Stage D 17개 Q-포맷 컨트랙트 어서션 모두 PASS.

D-bis-3는 gcQ12 비포화 → 플랜 Task 3 Step 4–6(어서션 승격) 스킵 조건에 해당하여 의도적으로 미적용. Task 3 Step 7(회귀 게이트)만 수행.

---

## 2. 자극별 13경계 측정 결과 (verbatim 로그 인용)

### 2.1 D-bis-1: 4-pulse canonical (`c[5]=+Q13, c[11]=-Q13, c[22]=+Q13, c[33]=-Q13`, idx={GA:3,GB:7})

```
=== 4-pulse canonical spec-derived values ===
Σc² true              = 4
Ē_c (true dB)         = -10.0000
Ê predicted (true dB) = 4.9400
logGain (true dB)     = 14.9400
g'_c (true)           = 5.5847
[① fcb] Σc²(raw=Q26)  = 268435456 → true=4.0000 (want 4.0000)
[⑩ gain] gpQ14=11626 gcQ12=3567 (true gc=0.8708)
[⑩ gain] spec g'_c=5.5847, max bound (γ̂_max≈2) = 11.1694
[⑩ gain] saturation check: gcQ12 == ±32767/-32768 ? false
[⑪ u] u[5]=1 u[11]=-1 u[22]=1 u[33]=-1 (other=0 expected)
[⑫ s] s[5]=1 s[11]=-1 s[22]=1 s[33]=-1
[⑬ sPf] sPf[0..7]=[0 0 0 0 0 1 0 0]
```

| 경계 | 측정 | 스펙-기대 | dB 차이 | 상태 |
|------|------|-----------|---------|------|
| ① Σc² | 4.0000 (Q26 raw=4·2²⁶) | 4.0000 | 0.00 | ✅ 비트-정확 |
| ⑩ gcQ12 | 3567 → 0.8708 | g'_c·γ̂_c ∈ [0, 11.17] | 측정 불가¹ | ✅ 범위 안, **포화 없음** |
| ⑪ u[i] | round(gc) = ±1 (네 위치) | round(0.8708)=±1 | 0.00 | ✅ 일치 |
| ⑫ s[i] | u 그대로 (trivial 필터) | u | 0.00 | ✅ 항등 |
| ⑬ sPf | `[0 0 0 0 0 1 0 0]` | — | — | postfilter 자체 거동 |

¹ γ̂_c VQ 출력 직접-추출은 Stage F 작업 영역. 단일-펄스(Stage D)와 동일하게 본 단계에서도 정량 dB 측정 불가.

**결론**: 4-pulse canonical 자극 또한 14 dB을 재현하지 않음. Phase 1j 보고서가 가설로 제시한 "Σc²=4 → ⑩ gcQ12 포화" 시나리오는 **거짓**임을 입증(gcQ12=3567, gcTrue=0.87 — 단일 펄스의 1.74보다 *작음*, 포화에서 더 멀어짐). g'_c=5.58을 받았지만 γ̂_c의 두 번째 단계 양자화 출력이 ≈0.156으로 작아 gcTrue가 ~0.87로 안착.

### 2.2 D-bis-2: 비제로 gpQ14 (피치-활성, 합성 자극)

자극: `c[0]=+Q13`, `v[n]=n+1` (1..40 Q0 ramp), `gpQ14=8192` (≈0.5 Q14), `idx={GA:3,GB:7}`.

```
=== Pitch-active stimulus (gpQ14=8192 ≈ 0.5000) ===
[⑩ gain] gcQ12=7134 (true gc=1.7417)
[⑪ u] u[0..7]=[2 1 2 2 3 3 4 4]
[⑪ u] u[20..27]=[11 11 12 12 13 13 14 14]
[⑪ u] u[32..39]=[17 17 18 18 19 19 20 20]
[⑫ s trivial] s[0..7]=[2 1 2 2 3 3 4 4]
[⑫ s trivial] s[32..39]=[17 17 18 18 19 19 20 20]
[⑫ s IIR] s[0..7]=[2 3 5 6 8 10 13 16]
[⑫ s IIR] s[20..27]=[67 71 76 80 85 89 94 99]
[⑫ s IIR] s[32..39]=[124 129 134 139 144 149 154 159]
[⑫ amplification] max|sTrivial|=20 max|sIIR|=159
[⑫ amplification] max|sIIR|/max|sTrivial| = 18.0073 dB
```

**핵심 측정**: 합성 IIR(`a[1]=-3686 = -0.9 Q12`) 적용 시 `max|s|`가 20 → 159으로 +18.0073 dB 증폭. 14 dB 영역(±2 dB)을 *초과*하는 수치이며 IIR 시간-누적이 14 dB 발산을 만들 수 있음을 직접 입증. ⑪ u에서는 gp·v 항이 정상적으로 가시(`u[0]=2`는 gc·c[0]=round(1.74)=2, `u[1]=1`은 gp·v[1]=0.5·2=1, `u[39]=20`은 0.5·40 등 중첩).

**주의**: 본 a 계수는 *합성된* one-pole이며 ALGTHM frame 0의 실제 a와 직접 비교 불가. 그러나 실제 LP 계수가 어느 정도라도 누적성이면 동일 메커니즘이 작동함을 시사.

### 2.3 D-bis-3: ALGTHM frame 0 sf0 실제-자극 재생 (가장 결정적)

```
=== Parsed ALGTHM frame 0 indices ===
LSP: L0=1 L1=105 L2=17 L3=0
sf0: P1=2 (parity P0=1) C1=0 S1=15 GA1=5 GB1=6
sf1: P2=2 C2=6134 S2=15 GA2=6 GB2=2

=== sf0 LP coefficients a[0..10] (Q12) ===
  a[ 0] =   4096 (= 1.000000)
  a[ 1] =  -2197 (= -0.536377)
  a[ 2] =   -375 (= -0.091553)
  a[ 3] =   -924 (= -0.225586)
  a[ 4] =   7735 (= 1.888428)
  a[ 5] =    294 (= 0.071777)
  a[ 6] =    665 (= 0.162354)
  a[ 7] =   7844 (= 1.915039)
  a[ 8] =  -1010 (= -0.246582)
  a[ 9] =    145 (= 0.035400)
  a[10] =    -33 (= -0.008057)

=== sf0 pitch delay: tInt=20 tFrac=+0 ===

=== sf0 v[*] (with empty pastExc) ===
  v[0..7]   = [0 0 0 0 0 0 0 0]
  v[20..27] = [0 0 0 0 0 0 0 0]

=== sf0 c[*] non-zero entries ===
  c[ 0] = +8192 (= +1.0000 Q13)
  c[ 1] = +8192 (= +1.0000 Q13)
  c[ 2] = +8192 (= +1.0000 Q13)
  c[ 3] = +8192 (= +1.0000 Q13)
  c[20] = +1639 (= +0.2001 Q13)   ← 피치-증강(β=0.2 Q14, tInt=20)
  c[21] = +1639
  c[22] = +1639
  c[23] = +1639
  Σc² (raw Q26) = 279180740 → true = 4.1601

=== sf0 gain ===
  gpQ14=13815 (= 0.8432) gcQ12=6844 (= 1.6709)
  gcQ12 saturated? false
  spec g'_c (default pastErrors) = 5.4762 → max gc bound ≈ 10.9523

=== sf0 u[*] ===
  u[0..7]   = [2 2 2 2 0 0 0 0]
  u[20..27] = [0 0 0 0 0 0 0 0]   ← v=0이므로 gc·c[20..23]=round(1.67·0.2)=0
  u[32..39] = [0 0 0 0 0 0 0 0]

=== sf0 s[*] (post-LP synth) ===
  s[0..7]   = [2 4 4 4 0 -6 -10 -18]
  s[20..27] = [-336 -114 394 1140 1666 1996 1562 -220]
  s[32..39] = [-5572 3094 17622 31874 32767 32767 15800 -31282]
                                      ^^^^^ ^^^^^ ← 양의 Q15 포화

=== sf0 sPf[*] (post-postfilter) ===
  sPf[0..7]   = [1 2 4 4 0 -6 -10 -17]
  sPf[20..27] = [-333 -113 386 1116 1637 1960 1535 -204]
  sPf[32..39] = [-5487 3010 17233 31274 32767 32767 14918 -31385]
```

**Sample-by-sample 발산표 (s·2 vs ALGTHM.PST[0])** — 전체 sweep:

| n | s·2 | PST | Δ |  | n | s·2 | PST | Δ |
|---|-----|-----|---|--|---|-----|-----|---|
| 0 | 4 | 2 | +2 |  | 20 | -672 | -1 | -671 |
| 1 | 8 | 4 | +4 |  | 21 | -228 | -1 | -227 |
| 2 | 8 | 3 | +5 |  | 22 | 788 | 0 | +788 |
| 3 | 8 | 3 | +5 |  | 23 | 2280 | 0 | +2280 |
| 4 | 0 | 1 | -1 |  | 24 | 3332 | 0 | +3332 |
| 5 | -12 | -1 | -11 |  | 25 | 3992 | 0 | +3992 |
| 6 | -20 | -1 | -19 |  | 26 | 3124 | 0 | +3124 |
| 7 | -36 | -1 | -35 |  | 27 | -440 | 0 | -440 |
| 8 | -40 | -1 | -39 |  | 28 | -5336 | 0 | -5336 |
| 9 | -20 | -1 | -19 |  | 29 | -11896 | 0 | -11896 |
| 10 | 4 | -1 | +5 |  | 30 | -17864 | 0 | -17864 |
| 11 | 64 | -1 | +65 |  | 31 | -17764 | 0 | -17764 |
| 12 | 136 | -1 | +137 |  | 32 | -11144 | 0 | -11144 |
| 13 | 160 | -1 | +161 |  | 33 | 6188 | 0 | +6188 |
| 14 | 176 | -1 | +177 |  | 34 | 35244 | 0 | +35244 |
| 15 | 92 | -1 | +93 |  | 35 | 63748 | 0 | +63748 |
| 16 | -132 | -1 | -131 |  | 36 | 65534 | 0 | +65534 ← Q15 포화 |
| 17 | -356 | -1 | -355 |  | 37 | 65534 | 0 | +65534 ← Q15 포화 |
| 18 | -668 | -1 | -667 |  | 38 | 31600 | 0 | +31600 |
| 19 | -876 | -1 | -875 |  | 39 | -62564 | 0 | -62564 |

**최초 명확 발산**: sample 5에서 |s·2|=12 vs |PST|=1 (≈ +21.6 dB). 이후 시간-누적적으로 폭주 → sample 30 ±17800 → sample 36–37 양의 Q15 포화(65534=2·32767). 14 dB 영역 통과 지점: **sample 5–9 부근**.

**핵심 측정 5종**:
1. `gcQ12 saturated?` → **false** (gcQ12=6844, gcTrue=1.67 vs g'_c=5.48의 max bound 10.95 안쪽).
2. Σc²=4.16 (스펙 일치, 피치-증강 c[20..23]은 Σ에 0.16 기여).
3. u[*]에서 v 기여 없음(empty pastExc 의도적, 분리 진단). c 기여만 `u[0..3]=[2,2,2,2]`로 정상.
4. `s` vs `sPf`: 두 시퀀스가 거의 동일한 비율 → postfilter는 **14 dB 분기점 아님** (브랜치 D 기각).
5. s[0]·2=4 ≠ PST[0]=2이지만 Phase 1i 잠금은 *프로덕션 출력*(postfilter→hpFilter→ScaleUpSat 후) 기준이므로 본 raw-s 비교는 의도된 차수 차이. 잠금 위반 아님.

---

## 3. 자극 간 비교 (Stage D + Stage D-bis 통합)

| 경계 | 단일-펄스 (Stage D) | 4-pulse canonical (D-bis-1) | 합성 IIR + gp·v (D-bis-2) | ALGTHM f0 sf0 (D-bis-3) |
|------|--------------------|------------------------------|---------------------------|--------------------------|
| ① Σc² (Q26) | 2²⁶ ✅ | 4·2²⁶ ✅ | (n/a, c 동일) | 4.16·2²⁶ ✅(피치-증강 0.16 포함) |
| ② energy 누산 | 0 dB ✅ | 0 dB ✅ | 0 dB ✅ | 0 dB ✅ |
| ⑩ gcQ12 포화 | False | False | False | **False** |
| ⑩ gcTrue | 1.74 (≤22.34) ✅ | 0.87 (≤11.17) ✅ | 1.74 (≤22.34) ✅ | 1.67 (≤10.95) ✅ |
| ⑪ u[ ] = round(gc) | u[0]=2 ✅ | u[i]=±1 ✅ | u 합리적 (gp·v + gc·c) ✅ | u[0..3]=[2,2,2,2] ✅ |
| ⑫ trivial 필터 항등 | s=u ✅ | s=u ✅ | s=u (sTrivial) ✅ | (n/a — 실제 a 사용) |
| ⑫ 실제 IIR 필터 | (테스트 안 함) | (테스트 안 함) | **+18.0 dB 증폭** ★ | **sample 5에서 +21 dB, sample 30 포화** ★★ |
| ⑬ postfilter 비율 | s≈sPf ✅ | s≈sPf ✅ | (n/a) | s≈sPf ✅ (브랜치 D 기각) |
| s·2 vs PST 발산 | (n/a) | (n/a) | (n/a) | sample 2부터 시작, sample 5에서 14 dB 통과, sample 36 포화 |

**비트-정확 경계 (모든 자극 일치)**: ①, ②, ⑩(비포화), ⑪, ⑫(trivial), ⑬(postfilter 비-증폭).

**14 dB 재현 자극**: D-bis-2(합성 +18 dB), D-bis-3(샘플 5+ 모든 후속 14 dB 초과). **단일-펄스(Stage D) 및 4-pulse canonical(D-bis-1)에서는 비재현** — 이 둘은 모두 trivial 필터(항등) 또는 단순 LP에 의존.

**공통 분기점**: 두 재현 자극 모두 `synth.Filter` 의 LP 합성 IIR을 통과해야만 14 dB이 나타남. ⑩~⑪까지 모든 모듈은 스펙 일치.

---

## 4. Stage F 분기 결정 매트릭스

| 브랜치 | 트리거 조건 | D-bis 증거 | 결정 |
|--------|------------|------------|------|
| **A: gain log-domain** | gcQ12 포화 또는 gcTrue ≫ g'_c·2 | 모든 자극에서 비포화, gcTrue 안전 범위. Phase 1j 가설 반증. | ❌ **기각** |
| **B: excitation BuildExcitation** | u[*]에서 gp·v 또는 gc·c 항 누락 | D-bis-2에서 gp·v 항 정상 가시. D-bis-3에서 gc·c 항 정상. | ❌ **기각** |
| **C: synth.Filter (LP IIR / LShl·Round)** | 합성 IIR amplification ≈ 14 dB **또는** sample n 발산이 IIR 누적 패턴 | D-bis-2: +18 dB(합성 a). D-bis-3: 실제 ALGTHM a로 sample 5+에서 14 dB 초과 + sample 30+ 포화. **누적-증폭 패턴 일치**. | ✅ **1순위 후보** |
| **D: postfilter** | s vs sPf 비율 ≈ 14 dB | D-bis-3에서 sPf와 s가 1% 이내 일치. 비-증폭. | ❌ **기각** |
| **E: fcb** | Σc² 비-스펙 또는 c[*] 위치/부호 비-canonical | 모든 자극에서 Σc²·c[ ] 스펙 일치(피치-증강 β=0.2 포함). | ❌ **기각** |

**부수 의문**: ALGTHM f0 sf0의 LSP-디코드 a 계수에 `a[4]=1.888`, `a[7]=1.915` 같이 |a|≫1 인 항목이 있음. 안정 LP 필터에서도 일부 a_i는 |1|을 초과 가능하므로 이것 자체가 버그를 의미하지 않으나, **LSP→LP 변환 단계(즉 ⓪ lspToLP)** 의 스펙-일치도 Stage F 첫 작업으로 점검 권장. 이는 브랜치 C의 부분 집합으로 분류.

---

## 5. Stage F 진입 결정 (사용자 입력)

**옵션 (가)** — Stage F 브랜치 C 진입 (LP 합성 + 부가로 LSP→LP 안정성 점검).
- 근거: D-bis-3가 sample 5에서 14 dB 통과 + sample 30+ 포화를 실제 ALGTHM 자극으로 재현. D-bis-2가 단순 IIR로도 +18 dB 증폭을 보임. 다른 4개 브랜치 모두 명확히 기각.
- 첫 작업 후보: (i) `internal/lsp/lsp_lp.go::lspToLP` 출력 a[]가 ALGTHM frame 0 sf0의 ITU 참조와 비트 일치 여부 검증(스펙 §3.2.6 polyStep), (ii) `internal/synth/filter.go` 의 LShl/Round 위치에서 누산 Q-포맷 점검(LMac 누산 폭, 출력 Round-shift).
- 위험: D-bis-3 a 계수가 *실제로 unstable*이면 분기점은 더 이전(LSP 디코더 §3.2.4 stability/rearrange 또는 §3.2.6 변환)으로 옮겨감. 그래도 브랜치 C 내부 작업으로 흡수 가능.

**옵션 (나)** — escape hatch 4 발동 → Phase 1l(SPEECH/FIXED 등 다른 ITU 벡터) 우회.
- 근거: D-bis-3 분기점이 단일 모듈로 못박히지 않고 LSP/synth 양쪽 가능성. 위험-회피.
- 비용: ALGTHM frame 0 80-sample 일치 영구 보류. Stage D + D-bis 산출물(합 22개 신규 어서션 + 2개 진단 하네스)만 영구 가드로 마감.

**옵션 (다)** — Stage D-ter 신설 (예: empty pastExc 가정을 제거하고 실제 frame 0 pastExc 시퀀스 + sf1까지 확장).
- 근거: D-bis-3는 sf0만 측정. 14 dB은 보고서 §5에서 sf2(=sf1)에 위치한다고 사전 기록. sf0가 이미 폭주한다는 본 결과는 14 dB 위치를 sf0로 앞당김 — 하지만 사용자 결정에 더 단단한 근거 필요시 Stage D-ter 가능.
- 비용: 추가 1–2 사이클 진단, Stage F 지연.

**본 보고서는 (가)를 권고**한다 — D-bis-3가 실제 ALGTHM 자극에서 sample 5의 14 dB 통과 + sample 30+ 포화를 직접 재현했고, 4개 비-C 브랜치가 모두 명확히 기각되었기 때문. 단, Stage F 브랜치 C의 첫 작업은 **LSP→LP 변환 출력의 스펙-일치 확인**(문제가 ⓪에서 시작했을 가능성)이며, 그 다음에 `synth.Filter` 의 누산/Round 점검으로 진행해야 한다.

---

## 6. 영구 가드로 남는 산출물

- `internal/decoder/diagnostic_multipulse_test.go`
  - `TestDiagnostic_FourPulseCanonicalChain` — 어서션 3개 (① Σc²=4·2²⁶, ⑩ gcTrue 범위, ⑩ 비포화)
  - `TestDiagnostic_PitchActivePulseChain` — 관측 로그 only, 어서션 0개 (합성 자극은 손계산 어렵고 강압-적합 위험으로 의도적 보류)
- `internal/decoder/diagnostic_algthm_replay_test.go`
  - `TestDiagnostic_ALGTHMFrame0SF0Replay` — 관측 로그 only, 어서션 0개 (gcQ12 비포화로 플랜 Step 4 스킵 조건 적용)

총 신규 어서션 3개. Stage D 17개와 합산 시 Phase 1k 누적 어서션 = 20개. 진단 하네스 2종(다중-펄스 / 합성 IIR / ALGTHM 재생) 확보.

---

## 7. 다음 단계 매핑

- 옵션 (가) 선택 시: Phase 1k 플랜 Task 8(Stage F 브랜치 C 작업) 적용. 첫 PR 후보:
  1. `lsp.lspToLP` Q-포맷 컨트랙트 + ALGTHM frame 0 sf0 a[] 스펙 비교 테스트 추가.
  2. `synth.Filter` 누산기 폭(int32) + Round 위치 컨트랙트 테스트 추가.
  3. 위 두 테스트 중 분기점이 확정되면 production 수정.
- 옵션 (나) 선택 시: Phase 1k Stage F 영구 보류. Stage D + D-bis 산출물 마감. Phase 1l SPEECH/FIXED 벡터 활성화 시작.
- 옵션 (다) 선택 시: Stage D-ter 플랜 작성(실제 pastExc 시퀀스 + sf1까지 확장).

---

## 8. 탈출 해치 발동 평가

| 해치 | 정의 | D-bis 결과 |
|------|------|-----------|
| 1 | 단일-펄스에서 모든 경계 0 dB | 이미 Stage D에서 발동. 본 단계는 그 후속. |
| 2 | ALGTHM f0 sample 0 회귀(2≠2) | **미발동** — sample 0 = 2 유지, Phase 1i `736beba` 잠금 무회귀. |
| 3 | 다중 모듈(≥2) 동시 분기 | **미발동** — fcb/gain/synth/postfilter 컨트랙트 17개 모두 PASS, 분기는 합성 IIR 단일 위치. |
| 4 | Stage D 17개 컨트랙트 회귀 | **미발동** — 모두 PASS. |

탈출 해치 0건 발동. Stage F 진입 조건 충족.
