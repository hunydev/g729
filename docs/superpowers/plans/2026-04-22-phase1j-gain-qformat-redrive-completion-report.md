# Phase 1j 완료 리포트 — 게인 디코더 Q-포맷 재유도 (부분 성공)

**날짜**: 2026-04-22
**계획 문서**: [`2026-04-22-phase1j-gain-qformat-redrive.md`](./2026-04-22-phase1j-gain-qformat-redrive.md)
**최종 상태**: **부분 성공.** 12개 작업 중 3개(Task 1, 6, 7)만 합격. 핵심 가설이 경험적으로 반증되어 ITU 비트-정확 복원은 달성하지 못했으며, `t.Skip` 또는 위장 커밋을 만들지 않고 정직하게 정지함(프롬프트 규칙 4).

---

## 핵심 결론 — Q26 가설은 경험적으로 잘못된 것으로 판명됨

Phase 1i 완료 리포트와 Phase 1j 계획서가 제시한 핵심 가설은:

> `c[]`은 Q13(±8192 펄스), `fixedCodebookEnergy`는 Σc²을 Q26으로 반환, 그러나 `log2Fixed`는 입력을 Q0으로 취급 → +78.3 dB 오프셋 → int16 wrap → `gcQ12=32767` 포화 → ALGTHM frame 1 sample 0 = -23270.
>
> 수정안: `ecLog2Q10 := log2Fixed(ecEnergy) - 26*1024`

**이 수정은 실제로 적용해 보니 Phase 1i가 이미 비트-정확하게 맞춘 ALGTHM frame 0 sample 0(want=2)을 회귀(got=12)시켰고, sample 40 포화는 그대로였다.** 즉 가설의 방향과 진단이 모두 부정확했다.

### 진단 과정 (Diagnosis Loop)

다음 후보 수정을 모두 시도했으나 어느 것도 ALGTHM frame 0 sample 40 포화를 해소하지 못했고, 오히려 sample 0이 회귀했다:

| 시도 | 결과 |
|------|------|
| `ecLog2Q10 -= 26*1024` (계획안 핵심 수정) | sample 0: 2 → **12** (회귀); sample 40: -32768 (변화 없음) |
| 위 + `int64` 곱셈으로 int32 오버플로 차단 | sample 40: 결과 동일 (gcQ12는 어차피 Q12에서 포화됨) |
| 위 + 게인 출력 Q-포맷을 Q12 → Q9 (헤드룸 ↑) | sample 0/40 결과 동일 (인접한 buggy 음수가 흡수됨) |
| `pastErrorsDefault`를 0, -7168, -28672, +16384로 변경 | 모든 케이스에서 frame 0 80/80 mismatch |

### 실제 산식 검증 (스펙 PDF 직접 인용 기반)

ITU-T G.729 §3.9.1, eq (66)~(72) 및 Table 6 (initial value)을 PDF에서 직접 추출하여 다음을 확인:

- **Eq (66)**: $\bar{E}_c = 10 \log_{10}\left(\tfrac{1}{40} \sum c(n)^2\right)$ — c는 단위 진폭 펄스(±1)
- **Ē = 30 dB** (스펙 §3.9.1 본문 명시)
- **MA 계수 [b₁..b₄] = [0.68, 0.58, 0.34, 0.19]**, $\Sigma b_i = 1.79$
- **Û 초기값 = -14 dB** (Table 6, "Variable initial value")

ALGTHM frame 0의 GA1=5, GB1=6 → γ̂_sf1 = (9949+2966)/2¹³ = 1.577. 정통한 4-펄스 fcb (Σc² = 4 in real units):
- $\bar{E}_c = 10 \log_{10}(0.1) = -10$ dB
- 초기 $\tilde{E} = 1.79 \cdot (-14) = -25$ dB → $\hat{E}(0) = \bar{E} + \tilde{E} = 5$ dB
- $\log_{20} g'_c = \tilde{E} + \bar{E} - \bar{E}_c = -25 + 30 - (-10) = 15$ dB → $g'_c = 5.62$
- $g_c = \hat{\gamma}_c \cdot g'_c = 1.577 \cdot 5.62 = 8.86$

**Q12 포맷의 최댓값은 32767 = 8.0**. 즉 **스펙-정확한 sf1 g_c = 8.86은 Q12에 들어가지 않는다.** 그리고 sf2는 더 심해서 spec g_c ≈ 41.9 (predicted_sf2 ≈ 17 dB → g'_c ≈ 22.4, γ̂_sf2 ≈ 1.87).

### 진짜 근본 원인의 위치

Phase 1i는 `ecBar`가 +4 dB(스펙 -10 dB와 14 dB 차이)인 상태에서 sample 0 = 2를 비트-정확 달성했다. 이는 **gain 계산의 14 dB 누적 오차가 다른 곳의 정반대 14 dB 오차와 우연히 상쇄되어 sample 0이 맞은 것**일 수 있음. Q26 보정만 하면 상쇄가 깨져서 sample 0이 회귀한다.

남아 있는 14 dB 오차의 후보 위치:
1. **합성필터 (`internal/synth`)**: LP 계수의 Q-포맷이나 누적 합산의 스케일링
2. **포스트필터 (`internal/postfilter`)**: §A.4.2.4 AGC 게인 적용 시 Q-포맷
3. **여기 신호 (`internal/synth/excitation.go`)**: `BuildExcitation`에서 `LShr`, `LMult`의 시프트 카운트
4. **펄스 진폭(`internal/fcb`)**: c가 진짜 Q13인지 vs 다른 포맷
5. **MA 예측기**: `predictedLogGain`이 Ẽ + Ē가 아니라 다른 식을 계산하고 있을 가능성

이 진단은 Phase 1k 또는 별도 phase로 분리할 필요가 있음 — 게인 모듈 내부 단독 수정으로는 해결 불가.

---

## 작업별 결과 요약

| Task | 제목 | 상태 | 비고 |
|------|------|------|------|
| 1 | Q-format chain documentation + contract invariant tests | ✅ 완료 | `energy.go` / `log2.go` / `pow2.go`에 Q-포맷 계약 명시. 향후 디버깅의 기반. |
| 2 | Failing regression test — `ecBar` magnitude | ❌ 미완 | 가설 자체가 틀려 `want = 18429` 자체가 무의미. 커밋하지 않음. |
| 3 | Failing test — `Frame0Sample40_MatchesALGTHM` | ❌ 미완 | Sample 40 포화는 게인 모듈 단독으로 해결 불가. |
| 4 | Core fix — `ecLog2Q10 -= 26*1024` | ❌ 적용 시 회귀 | 경험적으로 가설 방향이 잘못. |
| 5 | Pathological tests update | ❌ 미완 | Task 4가 미적용이므로 기존 어서션이 여전히 유효. |
| 6 | Gain VQ codebook sample-check | ✅ 완료 (계획 변경) | 스펙 PDF에 수치 테이블이 없어, 구조적 편향 속성(§3.9.2)으로 검증. |
| 7 | MA predictor audit | ✅ 완료 | `pastErrorsDefault = -14336` 컴파일타임 가드 + FIFO 시프트 검증. |
| 8 | Full frame-0 boundary test | ❌ 미완 | sample 0/40 동시 정확이 게인 모듈 단독으로 불가능. |
| 9–11 | ITU bit-exact 7개 벡터 재활성 | ❌ 미완 | Tasks 8 미해결로 차단됨. |
| 12 | 회귀 테스트 + Phase 보고 | ❌ 미완 | 본 보고서로 대체. |

---

## 계획에서의 일탈 — 사유 명시

1. **Task 4 미적용 (가장 큰 일탈)**
   - 사유: 핵심 가설이 경험적으로 반증됨. 적용 시 Phase 1i가 보장한 ALGTHM frame 0 sample 0 비트-정확이 회귀(2 → 12). 프롬프트 규칙 "DO NOT regress these"에 따라 적용 거부.
   - 대안 진단 4가지 모두 실패 (위 표 참조).

2. **Task 6 — 구조적 검증으로 변경**
   - 사유: ITU-T G.729 스펙 PDF는 GBK1/GBK2의 수치 테이블 항목을 게재하지 않음(§3.9.2의 *구조적* 편향만 기술; 항목 8/16, Q14/Q13 분리, 1단/2단 편향 방향). 수치값은 머저 독트린 하의 C 참조 `tab_ld8a.c`에만 존재하며 계획서가 명시적으로 금지함.
   - 가능한 가장 강한 스펙-기반 가드: `(g_p, γ̂)` 쌍 순서가 단계별 편향 방향을 따른다는 §3.9.2 본문 진술의 ≥75% 만족.

3. **Task 5 — 본 phase에서 적용하지 않음**
   - 사유: Task 4 미적용으로 기존 (GA=3, GB=7) 어서션이 여전히 spec-realistic. 변경 불필요.

4. **Tasks 8–12 — 미시도 commit 없음**
   - 사유: 프롬프트 규칙 4에 따라 "documenting open issue placeholder" 금지. `t.Skip` 추가도 금지.

---

## 커밋 목록 (`git log --oneline 736beba..HEAD`)

```
514175c docs(plans): restore Phase 1j plan with completed-task checkboxes
f99a1e1 test(gain): lock MA-predictor init value + FIFO-shift semantics per §3.9.1
356ac5d test(gain): structural spec-check of GainGBK1/GBK2 codebooks per §3.9.2
2507437 docs(gain): annotate Q-format contract of energy/log2/pow2 chain
```

총 4개 신규 commit. Phase 1i tip(`736beba`)에서 분기.

---

## 검증 결과

### `go test -race ./...`

```
ok  	github.com/hunydev/g729/internal/bitstream
ok  	github.com/hunydev/g729/internal/decoder         1.024s
ok  	github.com/hunydev/g729/internal/fcb
ok  	github.com/hunydev/g729/internal/fixed
ok  	github.com/hunydev/g729/internal/gain            1.008s
ok  	github.com/hunydev/g729/internal/lsp
ok  	github.com/hunydev/g729/internal/pcm
ok  	github.com/hunydev/g729/internal/pitch
ok  	github.com/hunydev/g729/internal/postfilter
ok  	github.com/hunydev/g729/internal/synth
ok  	github.com/hunydev/g729/internal/tables
```

전체 PASS — **단, ITU 7개 벡터 비트-정확 테스트는 Phase 1h에서 추가된 `t.Skip`이 그대로 유지됨** (회귀 없음, 추가도 없음).

### `go vet ./...`

빈 출력 (silent).

### `BenchmarkDecode -benchmem`

```
BenchmarkDecode-2   123024     9738 ns/op     0 B/op     0 allocs/op
```

✅ 0 allocs/op 유지.

### ITU 검증 매트릭스 — 비트-정확 미달성

| 벡터 | 프레임 수 | 상태 | 비고 |
|------|----------|------|------|
| ALGTHM | 35 | ❌ Phase 1h `t.Skip` 유지 | sample 0 ✓ (Phase 1i에서 잠금), sample 40 ✗ (-32768 포화) |
| SPEECH | 3750 | ❌ Phase 1h `t.Skip` 유지 | 동일 근본 원인 (ALGTHM과) |
| FIXED | ? | ❌ Phase 1h `t.Skip` 유지 | 동일 |
| LSP | ? | ❌ Phase 1h `t.Skip` 유지 | 동일 |
| PITCH | ? | ❌ Phase 1h `t.Skip` 유지 | 동일 |
| TAME | ? | ❌ Phase 1h `t.Skip` 유지 | 동일 |
| TEST | ? | ❌ Phase 1h `t.Skip` 유지 | 동일 |
| OVERFLOW | ? | ❌ 별도 비트스트림 파서 버그 (Phase 1k+) | scope 외 |

---

## Phase 1k 권고 사항

1. **합성필터/포스트필터/여기신호의 Q-포맷 종단간 재검증** — 게인 모듈만 수정해서는 ALGTHM이 풀리지 않음을 증명함. 다음 후보:
   - `internal/synth/excitation.go`의 `LShr(LMult(gcQ12, c), 11)` 시프트 카운트가 실제로 14 dB 차이를 일으키는지
   - `internal/synth/Filter`의 LP 계수 누적이 Q-포맷 일관성을 유지하는지
   - `internal/postfilter`의 AGC 게인 누적기 Q-포맷
2. **단순화된 단일-펄스 fcb 합성 회로 단독 테스트** — 4-펄스 ALGTHM 대신 1-펄스 합성 입력으로 합성/포스트필터 단독 검증 회로를 만들어 14 dB 오차의 정확한 위치를 격리
3. **전체 합성 체인 종이 위 재유도** — 게인 모듈처럼 매 모듈마다 Q-포맷 계약을 코드 주석으로 박아 두어, 다음 진단 시 동일한 시간 낭비를 막음 (Phase 1j Task 1이 게인에 했던 작업을 다른 모듈로 확장)

---

## 위반 여부 자가 점검

- ✅ ITU C 참조 / bcg729 / Sipro Lab 코드 미열람
- ✅ ITU 테스트 벡터의 내부 바이트 레이아웃 미검사 (G.192 / 원시 PCM 인터페이스만 사용)
- ✅ 변수/함수 명은 스펙 수학 기호에서만 유래
- ✅ 커밋 메시지에 금지어 ("포팅", "porting", "bcg729", "ITU C", "reference implementation") 없음
- ✅ `t.Skip` 신규 추가 없음
- ✅ 위장 placeholder 커밋 없음
- ✅ Phase 1i 비트-정확 (frame 0 sample 0 = 2) 회귀 없음
- ✅ `BenchmarkDecode` 0 allocs/op 유지
- ✅ `go vet` silent

본 phase는 부분 성공이지만, 다음 phase가 올바른 방향으로 진단을 시작할 수 있도록 **잘못된 가설을 명확히 제거**했다는 점에서 의미가 있다. 게인 모듈에 Q-포맷 계약 문서를 박아 둔 것 또한 향후 작업의 기반이 된다.
