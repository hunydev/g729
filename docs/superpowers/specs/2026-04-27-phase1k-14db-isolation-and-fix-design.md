# Phase 1k 설계 명세 — 14 dB 오차 격리 및 수정 (single-pulse harness + Q-format contracts)

**작성일**: 2026-04-27
**전제 phase**: Phase 1j (부분 성공, [완료 보고서](../plans/2026-04-22-phase1j-gain-qformat-redrive-completion-report.md))
**다음 phase**: Phase 1l (SPEECH/FIXED/LSP/PITCH/TAME/TEST 6개 ITU 벡터 재활성)

---

## 1. 목표

단일-펄스 격리 하네스와 모듈 전반의 Q-포맷 계약 테스트를 결합하여 ALGTHM frame 0 sample 40 포화의 원인인 **14 dB 오차의 정확한 위치**를 관측적으로 특정하고, 수정 적용 후 ALGTHM frame 0 (80 샘플) 비트-정확을 달성한다.

## 2. 배경 — Phase 1j가 남긴 사실

Phase 1j 완료 보고서에서 두 가지가 확정되었다.

1. **Q26 가설은 경험적으로 반증되었다.** `ecLog2Q10 -= 26*1024` 단독 수정은 Phase 1i가 비트-정확하게 잠근 ALGTHM frame 0 sample 0(=2)을 12로 회귀시킨다.
2. **스펙-유도 산식 자체는 검증되었다.** §3.9.1 eq (66)~(72), Table 6에서 직접 추출한 산식으로 4-pulse canonical fcb의 sf1 g_c = 8.86 ≈ Q12 max(8.0)을 초과한다.

이는 두 개 이상의 14 dB 오차가 서로 상쇄하며 sample 0만 우연히 비트-정확이 된 상태일 가능성을 시사한다. 게인 모듈 단독 수정으로는 풀리지 않는다.

## 3. 접근 — 3-Stage 진단/수정/검증 파이프라인

```
[Stage D — Diagnose]
  D1 frame 0 sf1 회귀 가드 확장
  D2~D5 모듈별 Q-포맷 계약 테스트 추가 (fcb, gain, synth, postfilter)
  D6 단일-펄스 격리 하네스 (observation-only)
  D7 하네스 어서션 채택 (스펙 참값과 일치하는 경계에 한해)
        ↓
[Stage F — Fix]
  F1 진단이 지목한 위치에 최소 수정 + ALGTHM sf2 어서션 동일 커밋
        ↓
[Stage V — Validate]
  V1 frame 0 회귀 가드를 80 샘플로 확장
  V2 ALGTHM frame 0 t.Skip 제거
  V3 병리적 테스트 A+B 혼합 재인증
  V4 전체 회귀 패스 (go test -race / go vet / BenchmarkDecode 0 allocs)
        ↓
완료 보고서
```

## 4. 스코프 울타리

**포함**:
- Stage D 5개 신규 테스트 파일 + 1개 기존 확장 (총 7 commits, 하네스는 observation/assertion 2-stage)
- Stage F 한 파일(가능하면 한 줄) 수정, 두 파일 동시 수정은 진단 증거가 명시된 경우에만 허용
- ALGTHM frame 0 (80 샘플) 비트-정확 활성화

**제외 (다음 phase로)**:
- SPEECH/FIXED/LSP/PITCH/TAME/TEST 6개 ITU 벡터 재활성 → Phase 1l
- OVERFLOW.BIT 비트스트림 파서 버그 → Phase 1m+
- 프레임 erasure / parity / 공개 API / 인코더 → 별도 phase

## 5. 컴포넌트

### 5.1 Stage D — 신규 5개 + 기존 1개 확장

| 파일 | 유형 | 책임 |
|------|------|------|
| `internal/decoder/diagnostic_singlepulse_test.go` | 신규 | 단일-펄스 격리 하네스. gain → excitation → synth → postfilter를 모듈 public API로 수동 조립, 13개 경계의 실측 vs 스펙-유도 참값 비교. 프로덕션 코드에 훅 추가 없음. |
| `internal/fcb/qformat_contract_test.go` | 신규 | `PulseAmplitude == 8192` Q13 불변, 피치 강화 후 `\|c[n]\|` 상한, Σc²=N·2²⁶ 식별식. |
| `internal/gain/qformat_contract_test.go` | 신규 (Phase 1j Task 1 확장) | `fixedCodebookEnergy(c_Q13)` Q26 주장, `log2Fixed` 입력을 Q0로 해석한다는 계약, `pow2Fixed` 출력 Q0, 로그-도메인 상수 (dbPerLog2Q13, tenLog10_40Q10, invDbScaleQ15, dbPerLog2Q10) 컴파일타임 정체성 검증. |
| `internal/synth/qformat_contract_test.go` | 신규 | `BuildExcitation`: `LMult(gpQ14, v_Q0) → Q15`, `LMult(gcQ12, c_Q13) → Q26`, `LShr 11 → Q15`. `filterSubframe`: `a[0]==4096` Q12, LP 누산기 Q26, LShl 3 + Round 경로. |
| `internal/postfilter/qformat_contract_test.go` | 신규 | shortterm / longterm / tilt / agc 각 sub-stage I/O Q-포맷. AGC 게인 Q24, tilt 계수 Q15. |
| `internal/decoder/frame0_sample0_test.go` | 기존 확장 | sample 0 단일 → frame 0 sf1 전체 40 샘플로 확장. Phase 1i 회귀 가드. |

### 5.2 Stage F — 위치 미정 (진단 결과 의존)

진단 결과별 예상 수정 위치:

| 어느 경계가 처음 14 dB로 튀나 | 수정 후보 위치 |
|------|------|
| ④~⑤ ecDbQ10, ecBar | `internal/gain/decode.go` 또는 `log2.go` |
| ⑥~⑦ Ê, logGainDb | `internal/gain/decode.go` (predictedLogGain, Ē 상수) |
| ⑧~⑩ log2Gc, gc0, gcQ12 | `internal/gain/decode.go` 또는 `pow2.go` |
| ⑪ u_Q0 | `internal/synth/excitation.go` (LShr 카운트) |
| ⑫ s_Q0 | `internal/synth/filter.go` |
| ⑬ sf_Q0 | `internal/postfilter/*.go` |

수정 1줄당 주석: ITU §ref + 무엇이 틀렸는지 한 줄.

### 5.3 Stage V — 기존 파일 변경

| 파일 | 변경 |
|------|------|
| `internal/decoder/frame0_sample0_test.go` | sf2까지 확장 → 80 샘플 풀커버리지 |
| `internal/decoder/algthm_test.go` | frame 0 `t.Skip` 제거. 나머지 34 프레임은 Phase 1l까지 skip 유지. |
| `internal/gain/pathological_test.go` | A+B 혼합 재인증 (5.5절). |

## 6. 데이터 흐름 — 단일-펄스 하네스

### 6.1 입력

```
c_Q13           = [+8192, 0, 0, …]               (N=1 pulse, +1.0 true)
v_Q0            = [0, 0, …]                      (adaptive codebook 비활성)
gpQ14           = 0                              (pitch gain 영향 0)
pastErrors_Q10  = [-14336, -14336, -14336, -14336]  (§3.9 Table 6 default)
idx.GA, idx.GB  = γ̂_c ∈ {0.25 또는 0.5 근방} 산출 쌍 (구현 단계에서 결정)
```

### 6.2 경계별 스펙-유도 참값

| 단계 | 실측 대상 | 참값 식 | 참값 (N=1 pulse) |
|------|----------|--------|---------|
| ① | fcb 출력 `c_true`, `max\|c\|` | `cQ13 / 2¹³` | `1.0`, `1.0` |
| ② | `fixedCodebookEnergy(c)` | `Σc_true² · 2²⁶` | `2²⁶ = 67,108,864` |
| ③ | `log2Fixed` 출력 (Q10) | `log2(Σc_true²·2²⁶) · 1024 = 26·1024` | `26624` |
| ④ | `ecDbQ10` (Q10) | `10·log10(Σc²)` | 스펙: `0`, buggy: ~78 dB (int16 wrap) |
| ⑤ | `ecBarDbQ10` (Q10) | `10·log10(Σc_true²/40) · 1024` | `-16·1024 = -16405` |
| ⑥ | `predicted` Ê (Q10) | `(Ē + Σbᵢ·U(m-i)) · 1024 = (30 + 1.79·(-14)) · 1024` | `≈ 5060 ≈ +5 dB` |
| ⑦ | `logGainDbQ10` (Q10) | `Ê - Ē_c` | `5060 - (-16405) = 21465` |
| ⑧ | `log2GcQ10` (Q10) | `logGainDb / (20·log10(2))` | `≈ 3566 ≈ log2(11.16)` |
| ⑨ | `gc0Q14` (Q14) | `2^log2GcQ10 · 2¹⁴` | `≈ 182,846` |
| ⑩ | `gcQ12` (Q12, γ̂_c=0.5 가정) | `γ̂_c · g'_c · 2¹²` | `≈ 22,857` |
| ⑪ | `u_Q0` (n=0) | `g_c · c_true (no pitch)` 정수 반올림 | `≈ 6` |
| ⑫ | `s_Q0` (n=0) | LP 상태 0 가정 시 `≈ u[0]` | `≈ 6` |
| ⑬ | `sf_Q0` (n=0) | postfilter §A.4.2.4 산식 | 구현 단계 산출 |

실제 수치는 구현 플랜의 책임. 명세는 비교식의 **형태**만 확정.

### 6.3 진단 비교 식

각 경계 b에 대해:
```
actual_raw    := 모듈 출력에서 직접 읽기
actual_true   := actual_raw / scale(모듈이 주장하는 Q-포맷)
expected_true := 6.2의 스펙 유도값
divergence_dB := abs(10·log10(actual_true / expected_true))
```

`divergence_dB > 0.5`인 첫 경계 = 14 dB 범인 위치 후보.

### 6.4 진단 표 → 수정 위치 매핑

5.2의 표 그대로 사용. ③에서 26·1024 등이 보이는 것은 **계약**이지 버그 아님 (현재 코드의 Q-포맷 규약을 그대로 표현). 버그는 ⑤ 이후 첫 14 dB 분기점에서 발견되어야 정상.

## 7. 오류 처리 및 탈출 해치

### 7.1 탈출 해치 1 — 진단이 단일 모듈을 지목하지 않을 때

**증상**: 6.3의 `divergence_dB > 0.5`가 ⑤~⑬에서 여러 경계에 걸쳐 산재하거나, 어느 경계도 14 dB ± 2 dB 안쪽으로 명확히 들어오지 않음.

**대응**:
1. Stage F 진입 **금지**.
2. 진단 결과를 그대로 완료 보고서에 기록.
3. Phase 1l 전 단계에서 재-브레인스토밍.
4. Stage D 산출물(하네스 + 계약 테스트 6개)만 영구 가드로 커밋, 부분 성공으로 마감.

### 7.2 탈출 해치 2 — 수정이 frame 0 sample 0을 회귀시킬 때

**증상**: Stage F 후보 적용 시 5.1의 frame0_sample0_test.go (sf1 확장본)가 깨짐.

**대응**:
- 회귀 우선 — Phase 1i 비트-정확 결과는 양보 불가.
- 회귀 발생 = "수정 위치가 14 dB 상쇄 짝의 한쪽"이라는 증거.
- 진단 표에서 **두 곳의 14 dB 짝**을 식별, 동시 수정.
- 두 곳 동시 수정 후보가 명확하지 않으면 → 탈출 해치 1로 다운그레이드.

구현: Stage F 수정 커밋은 한 commit 안에서 **두 파일 변경 허용**, 단 두 파일 모두 진단 표에 명시적 dB 차이 증거가 있어야 함.

### 7.3 탈출 해치 3 — 14 dB이 정확히 14 dB이 아닐 때

**증상**: 진단 결과가 12 ~ 16 dB 범위지만 정확히 14 dB이 아님.

**대응**:
- 14 dB은 사전 가설. 실제 측정값을 그대로 받아들이고 진단 표에 기록.
- 가능한 물리적 의미 식별:
  - 12 dB ≈ log10(4)·10 → 4배 어딘가 (LMult 결과 두 배 + Round 등)
  - 6 dB ≈ log10(2)·10 → 2배 어딘가 (LShr/LShl 한 단계 차이)
  - 78 dB ≈ 26·log10(2)·10 → Q26 vs Q0 미스매치
- 식별된 실제 dB 값으로 Stage F 진행.

### 7.4 탈출 해치 4 — 병리적 재인증 미완성

**증상**: Stage V V3에서 A 전략 어서션이 스펙-유도 상한과 어긋나거나 산식이 한 줄에 맞지 않음.

**대응**:
- A 실패 케이스는 즉시 B로 강등, 강등 사실을 완료 보고서에 명시.
- 4개 모두 A에서 살아남지 못하면 → 14 dB 픽스 자체가 의심됨 → 탈출 해치 1 발동 검토.

### 7.5 금지 사항

- ✗ `t.Skip` 신규 추가
- ✗ 위장 placeholder 커밋 (의도와 다른 어서션을 통과시키기 위해 작성)
- ✗ 회귀를 무시하고 "전반적으로 더 나아졌다"고 주장
- ✗ ITU C 참조 / bcg729 / Sipro Lab / FFmpeg 코드 열람
- ✗ 가설을 강행하기 위한 임시 오버라이드

### 7.6 완료 보고서 의무 기재 항목

1. Stage D 진단 결과 표 (각 경계의 실측 vs 참값, dB 차이)
2. 식별된 14 dB 위치(들)와 증거
3. Stage F 수정 diff 요약
4. ALGTHM frame 0 80 샘플 비트-정확 여부 (실패 시 어느 샘플이 어느 정도)
5. 병리적 테스트 A/B 분류 결과
6. 탈출 해치 발동 여부 및 사유

## 8. 테스트 전략

### 8.1 Stage D — 표준 TDD + 관측-우선 변형

D1~D5 (계약 테스트, 표준 TDD):
1. 모듈이 주장하는 Q-포맷을 어서션으로 명시 (예: `TestLog2Fixed_TreatsInputAsQ0`)
2. 어서션 식 자체는 스펙 또는 모듈 doc에서 직접 인용 (한 줄 §ref 주석)
3. 실행 → 통과면 모듈은 주장과 일치, 실패면 거짓말 중
4. **t.Skip 0개**, 통과/실패 무관하게 결과 그대로 commit (실패도 영구 진단 증거)

D6 (단일-펄스 하네스, 관측-우선 변형):
1. 첫 commit: 13개 경계 관측값을 t.Logf로 출력 (어서션 0개)
2. 두 번째 commit: 스펙 참값과 일치하는 경계에만 어서션 추가
3. 진단 그 자체가 영구 회귀 가드가 됨

### 8.2 Stage F — 회귀-가드 우선 TDD

F1 (수정):
1. frame 0 sf1 (40 샘플) 회귀 가드 통과 중 확인
2. ALGTHM frame 0 sample 40+ 어서션을 새 실패 테스트로 추가 (red)
3. 진단이 지목한 위치에 최소 수정 적용
4. red 테스트 통과(green) + 회귀 가드 유지 확인
5. 두 조건 동시 충족만 commit. 한쪽이라도 깨지면 탈출 해치 2.

### 8.3 Stage V — 정합성 검증

V1 ~ V2: Stage F 단계에서 frame 0 80 샘플은 이미 통과 — 추가 어서션 없음, ALGTHM `t.Skip` 제거만.

V3 (병리적 재인증, 혼합 전략):
- AllZero / LowEnergy: A 전략 → 스펙-유도 상하한, 한 줄 산식이어야 채택
- HighEnergy / SucceedsAcrossAllGainIndices: B 전략 → 실측 어서션 + 스펙 경계 한 줄 주석
- 4개 모두 한 commit으로 묶어 단위화

### 8.4 커밋 단위 규약

```
Stage D — 1 task = 1 commit (총 6~7개)
  D1: test(decoder): expand frame0 regression guard to sf1 40 samples
  D2: test(fcb): Q-format contract — PulseAmplitude=8192, Σc²=N·2^26
  D3: test(gain): Q-format contract — extend Phase 1j contract scope
  D4: test(synth): Q-format contract — excitation+filter Q-format chain
  D5: test(postfilter): Q-format contract — sub-stage Q-format I/O
  D6: test(decoder): single-pulse diagnostic harness (observation-only)
  D7: test(decoder): single-pulse harness assertions for spec-aligned boundaries

Stage F — 1 commit (또는 두 곳 동시 수정 시 2 commits)
  F1: fix(<module>): correct 14 dB scale at boundary <X> per §<spec ref>
       (failing ALGTHM frame 0 sf2 test in same commit)

Stage V — 3 commits
  V1: test(decoder): expand frame0 regression guard to full 80 samples
  V2: test(decoder): enable ALGTHM frame 0 bit-exact assertion
  V3: test(gain): re-certify pathological tests (A spec-derived + B empirical)

문서 마감 — 1 commit
  docs(plans): add Phase 1k completion report
```

총 11~13 commits 예상 (탈출 해치 발동 시 더 적을 수 있음).

### 8.5 커밋 메시지 규칙

- Co-author trailer: `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`
- 금지어 부재 자가 점검: "포팅", "porting", "bcg729", "ITU C", "reference implementation", "Sipro"
- 수정 commit (F1): ITU §ref 한 줄 인용 + 무엇이 틀렸는지 한 줄

### 8.6 전체 회귀 게이트 (각 Stage 종료 시)

```bash
go test -race ./...   # 모든 패키지 PASS, skip 카운트는 V2 후 ALGTHM frame 0만 -1
go vet ./...          # silent
go test -bench=BenchmarkDecode -benchmem ./internal/decoder
                       # 0 allocs/op 유지
```

### 8.7 TDD 위반 가드

- Stage F commit: 실패하는 새 테스트가 같은 commit에 포함되어야 함 (분리 commit 금지)
- Stage D commit들: 모두 t.Skip 0개
- 각 commit 후: BenchmarkDecode 0 allocs 자동 확인

## 9. 성공 기준

본 phase는 다음을 모두 충족할 때만 "완전 성공":

1. ✅ Stage D 6개 신규 테스트 + 1개 확장 모두 commit, t.Skip 신규 0개
2. ✅ 진단 표에서 14 dB 위치 1곳(또는 명시적 증거 하의 2곳) 특정
3. ✅ Stage F 수정 후 frame 0 sample 0 = 2 회귀 없음 + sample 40 ITU 일치
4. ✅ ALGTHM frame 0 (80 샘플) 비트-정확 통과 (`internal/decoder/algthm_test.go`)
5. ✅ 병리적 테스트 4개 재인증 (A/B 분류 명시)
6. ✅ go test -race / go vet silent / BenchmarkDecode 0 allocs 유지
7. ✅ 완료 보고서 7.6 항목 6개 모두 기재

7.1~7.4 탈출 해치가 발동되면 "부분 성공" — Stage D 산출물만 영구 가드로 남기고 다음 phase에서 재시도.

## 10. 참고

- ITU-T G.729 사양: §3.8 (fcb), §3.9 (gain), §3.10 (synth recovery), §A.4 (Annex A 단순화), Table 6 (gain decoder initial values)
- Phase 1j 완료 보고서: 14 dB 가설의 경험적 반증, 스펙-유도 g_c=8.86 계산
- Phase 1i 완료 보고서: AGC seed 픽스, prevLSP spec init, §3.10 /2 scale, tenLog10_40Q10=16405
- 스크래치-from-스펙 + 머저 독트린: 알고리즘 코드는 스펙만 참조, 데이터 테이블만 tab_ld8a.c에서 전사 가능
