# Phase 1k Stage F-bis Task 2 (F-bis-2) 분석 보고서

**상태**: F-bis-2 분석 완료. **Hard escape hatch 발동** — `pcm.ScaleUpSat`의 현 production 거동(`<<1`, ×2)이 ITU-T G.729 §4.2.5 / §A.4.2.5 의 "multiplied by a factor 2" 규정과 *완전히 일치*. 따라서 F-bis 플랜의 §3.10 인용("finally divided by 2")은 *플랜 작성 오류*이며, F-bis-1이 식별한 단계 식별은 위치적으로 옳지만 *결함 방향이 정반대*. F-bis-3 진입 (가)/(나) 권고 보류 — 사용자 결정 대기.

**작성일**: 2026-04-27

**준수**: ITU 참조 C, bcg729, FFmpeg G.729 등 외부 구현 미참조. 본 보고서 인용은 ITU-T G.729 (06/2012) 단일 PDF (`docs/superpowers/specs/itu/G729E.pdf`, 본 PDF는 표지 "Recommendation ITU-T G.729" 본문 + Annex A/B/C/D/E 통합본) §3.1.1, §3.10, §4.2, §4.2.4, §4.2.5, §A.4.2, §A.4.2.4, §A.4.2.5 만 참조.

---

## 0. Working tree 상태 확인 (본 태스크 분석-only)

### 0.1 `git status` (본 보고서 커밋 직전)

```
On branch main
Your branch is ahead of 'origin/main' by 100 commits.

Changes not staged for commit:
	modified:   internal/lsp/lsp_lp.go

Untracked files:
	internal/decoder/stagef_bis_diagnostic_test.go

no changes added to commit
```

| 경로 | 상태 | 본 태스크 변경? |
|------|------|------|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix | **아니오** (F-bis-1에서 적용; F-bis-2는 *touch* 없음) |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (uncommitted) — F-bis-1 진단 하니스 | **아니오** (F-bis-1에서 추가; F-bis-2는 *touch* 없음) |
| 그 외 production 파일 | 변경 없음 | **아니오** |

본 보고서 커밋 후에도 working tree는 위 두 파일의 미커밋 상태를 그대로 유지한다.

---

## 1. §1 `pcm.ScaleUpSat` spec-인용 분석

### 1.1 ITU-T G.729 §4.2.5 verbatim (출력 ×2 단계의 *유일한* 규정)

PDF p.29, §4.2.5 "High-pass filtering and upscaling":

> "A high-pass filter with a cut-off frequency of 100 Hz is applied to the reconstructed postfiltered speech sf'(n). The filter is given by:
>
>   H_h2(z) = (0.93980581 − 1.8795834·z⁻¹ + 0.93980581·z⁻²) / (1 − 1.9330735·z⁻¹ + 0.93589199·z⁻²)        (91)
>
> **The filtered signal is multiplied by a factor 2 to restore the input signal level.**"

키 인용은 굵게 표기한 마지막 문장. 이 문장이 본 단계의 *유일하고 권위 있는* spec 규정이다.

### 1.2 §3.10 verbatim — 본 §은 출력 ×2와 *무관*

PDF p.24, §3.10 "Memory update":

> "An update of the states of the synthesis and weighting filters is needed to compute the target signal in the next subframe. After the two gains are quantized, the excitation signal, u(n), in the present subframe is obtained using:
>
>   u(n) = ĝ_p · v(n) + ĝ_c · c(n)   n = 0,...,39    (75)
>
> ... (이하 §3.10는 합성/가중 필터의 *메모리 갱신* 절차 — 출력 진폭 스케일링과는 무관) ..."

**플랜 §3.10 인용은 spec과 불일치**: F-bis 플랜 (`docs/superpowers/plans/2026-04-27-phase1k-stage-f-bis-plan.md` line 49-55, 65-71) 은 §3.10에 *"The output speech is finally divided by 2 with saturation control"* 라는 인용을 두 번 게재하나, 실제 §3.10에는 해당 문장이 없다. 출력 스케일링 규정은 §4.2.5 (또는 G.729A 동일 조항 §A.4.2.5) 에만 존재하며, 그 규정은 *÷2가 아닌 ×2*이다. **플랜의 §3.10 → ÷2 인용은 작성 오류**.

### 1.3 §A.4.2.5 — G.729A 의 §4.2.5 상속 확인

PDF p.43, §A.4.2.5 "High-pass filtering and upscaling":

> "Same as described in clause 4.2.5."

→ G.729A의 출력 단계는 §4.2.5을 단어 그대로 상속. 본 codec 구현(G.729A) 에서도 *output ×2 (multiplied by a factor 2)* 가 spec-mandated 거동임이 확정.

### 1.4 §3.1.1 — 인코더 입력 ÷2 와 §4.2.5 ×2 의 대칭성 확인 (정합성 검증)

PDF p.5, §3.1.1 "Preprocessing":

> "The scaling consists of dividing the input by a factor 2 to reduce the possibility of overflows in the fixed-point implementation. The high-pass filter serves as a precaution against undesired low-frequency components. ... Both the scaling and high-pass filtering are combined by dividing the coefficients at the numerator of this filter by 2."

→ **인코더는 입력을 ÷2** (수치적으로는 H_h1(z)의 b 계수에 ÷2 융합). **디코더는 출력을 ×2** (§4.2.5). 두 단계가 정확히 상쇄하여 입출력 진폭이 보존된다(spec 본문의 "to restore the input signal level"). 본 codec의 `internal/pcm/coeffs.go::B0=3798 (Q13)` 가 0.46363718 = 0.92727436/2 임을 검증 (line 19-45). 인코더 측 ÷2 는 코드에 정상 반영됨.

### 1.5 production `internal/pcm/scale.go:17-25` verbatim

```go
// ScaleUpSat multiplies each sample in in by 2 with int16 saturation
// and writes the result to out. This is the decoder-side inverse of
// the 1/2 amplitude scaling applied in PreProcessor: the decoder
// synthesizes samples in the halved-amplitude domain, and this final
// step restores the original amplitude, clipping to +/-Max16 where
// the doubling would overflow.
//
// in and out may alias (in-place is safe). Processes
// min(len(in), len(out)) samples. Allocates nothing.
//
// See ITU-T G.729 section 4.2 for the decoder output-scaling
// requirement.
func ScaleUpSat(in, out []int16) {
	n := len(in)
	if len(out) < n {
		n = len(out)
	}
	for i := 0; i < n; i++ {
		out[i] = fixed.Shl(in[i], 1)
	}
}
```

### 1.6 Line-by-line spec ↔ production 대조

| 항목 | spec §4.2.5 / §A.4.2.5 | production (`scale.go:17-25`) | 일치? |
|------|------|------|---|
| 단계 위치 | HP filter 후 마지막 단계 | `decode.go:47` `pcm.ScaleUpSat(out…)` ← `hpFilter` 직후 | ✓ |
| 연산 | "multiplied by a factor 2" | `fixed.Shl(in[i], 1)` (= ×2 with Word16 saturation) | **✓ (정확히 일치)** |
| Saturation | "to restore the input signal level" — int16 도메인 출력이면 saturation 묵시 | `fixed.Shl(…, 1)` 가 ±Max16 saturation 수행 | ✓ |
| 함수 docstring 의미 | 인코더 ÷2 의 복원 | "decoder-side inverse of the 1/2 amplitude scaling applied in PreProcessor" | ✓ (정확히 §3.1.1↔§4.2.5 대칭 기술) |

**결론 — `pcm.ScaleUpSat` 의 모든 라인은 §4.2.5 / §A.4.2.5 와 *완전히 일치*한다.** docstring(line 5-16) 도 §3.1.1 ↔ §4.2.5 의 대칭 관계를 정확히 기술. 어떠한 부호/Q-포맷/시프트/상수/saturation 위치 어긋남도 발견되지 않음.

### 1.7 F-bis-1 데이터 재해석 (escape hatch 트리거)

F-bis-1 §3 측정값:

| 단계 | sample 0 | sample 1 | sample 2 | sample 3 |
|------|---:|---:|---:|---:|
| `synth.Filter` 직후 | 2 | 3 | 4 | 4 |
| `postfilter.Filter` 직후 | 2 | 2 | 3 | 4 |
| `hpFilter` 직후 | 2 | 2 | 3 | 3 |
| `pcm.ScaleUpSat` 직후 | 4 | 4 | 6 | 6 |
| **PST want** | **2** | **4** | **3** | **3** |

만약 §4.2.5 의 ×2 가 옳다면, *spec-correct 한 pre-ScaleUpSat 값*은 PST 의 절반: `[1, 2, 1.5, 1.5]`. production의 hpFilter 출력 `[2, 2, 3, 3]` 과 비교:

| sample | spec-기대 pre-×2 (= PST/2) | production hpFilter | 차이 (production / spec-기대) |
|---:|---:|---:|---:|
| 0 | 1 | 2 | **2× too high (upstream)** |
| 1 | 2 | 2 | **1× — spec과 일치 (upstream OK)** |
| 2 | 1.5 | 3 | **2× too high (upstream)** |
| 3 | 1.5 | 3 | **2× too high (upstream)** |

→ **F-bis-1 데이터의 진정한 의미**: 결함은 `pcm.ScaleUpSat` 가 *아니라*, 그 *상류* (synth/postfilter/hpFilter 중 한 단계 또는 그 조합) 가 sample 0/2/3 에서 spec 의 절반-amplitude 도메인 대비 *2× 과대 진폭*을 산출. ScaleUpSat 는 spec § 4.2.5 의 ×2 를 충실히 수행하지만, 입력이 이미 2× 부풀려져 있어 출력이 PST 대비 2× 과대.

sample 1 만 production hpFilter 출력이 spec-기대(=2) 와 일치 — 이는 F-bis-1 §3.3 에서 "sample 1 sub-issue 후보" 로 기록한 것의 *반대 해석*: sample 1 만 정상이고 0/2/3 이 비정상. (F-bis-1 §3.3 은 ×2 누락 가설을 전제로 sample 1 을 anomaly 로 기술했지만, ×2 가 spec-correct 임이 본 §1 에서 확정되었으므로 정상/이상의 라벨이 뒤집힌다.)

### 1.8 Phase 1i sample-0 잠금의 우연한 통과 메커니즘

- Phase 1i 시점: P 결함 (lspToLP Q28 saturation) 이 활성 → a[] 진폭이 약 1/2 로 축소 → synth.Filter 출력 진폭이 약 1/2 로 축소 → pre-ScaleUpSat = 1 → ScaleUpSat ×2 = 2 = PST want ✓.
- F-bis-1 P fix 후: a[] 진폭 복원 → synth.Filter 출력이 spec 대비 *2× 과대* (위 §1.7 sample 0/2/3 표) → pre-ScaleUpSat = 2 → ScaleUpSat ×2 = 4 ≠ PST want 2.

**즉, Phase 1i 잠금은 두 개의 결함 (P 결함이 진폭을 2× 축소 + 상류 어딘가에 진폭을 2× 부풀리는 결함) 이 우연히 상쇄하여 통과**. P fix 가 첫 번째 결함만 제거하자 두 번째 결함 (상류 2× 과대) 이 노출. ScaleUpSat 는 두 결함 어느 쪽과도 무관하다.

### 1.9 §1 결론 (escape hatch trigger)

- `internal/pcm/scale.go::ScaleUpSat` 는 spec §4.2.5 / §A.4.2.5 와 *완전히 일치*. **수정 대상 아님**.
- F-bis 플랜의 *§3.10 ÷2* 인용은 *작성 오류* (실제 §3.10 는 메모리 갱신 절차). 출력 스케일링 규정은 §4.2.5 의 ×2 만 존재.
- F-bis-1 의 단계 식별 ("ratio = 2.000 진입") 은 *위치적으로 정확*하지만, 그 위치를 *결함 위치*로 라벨링한 것은 잘못. 실제 결함은 그 *상류*.
- **사용자 명시 escape hatch 조건 일치**: "If your analysis of `pcm.ScaleUpSat` finds production already matches spec (impossibly — but if), report that conflict to the user and STOP — do not proceed with any bundle."

---

## 2. §2 sample 1 sub-fault 분석 (보조 — escape hatch 발동으로 우선순위 하향)

§1 의 결론에 따라 "ScaleUpSat alone-fix" 가설 자체가 부정되므로, 본 §의 sample 1 분석은 *F-bis-3 진입을 위한 후보 식별* 이 아닌 *상류 결함 후보 좁히기* 의 출발점으로 기록한다. 깊은 hand-calc 는 새 플랜(F-tris) 에서 수행 권고.

### 2.1 후보 spec § 인용

- **§4.2.4 / §A.4.2.4 AGC initial state** (가장 강한 후보):
  > "The initial value of g(−1) = 1.0 is used. Then for each new subframe, g(−1) is set equal to g(39) of the previous subframe." (§4.2.4 마지막 문장; §A.4.2.4 는 α=0.9 만 변경, 초기값은 동일)

  Q14 표현으로 g(−1) = 1.0 = 16384.

- §3.10 / §A.3.10 synthesis filter saturation recovery (덜 가능, P fix 후 |Δ|≤3 stimulus 에서는 trigger되지 않음).
- §4.2.5 / §A.4.2.5 HP filter 초기 상태 (`hpX[0]=hpX[1]=hpY[0]=hpY[1]=0`) — sample 0/2/3 은 ×2 패턴이라 단순 transient 가 아님. 약한 후보.

### 2.2 production 코드 발췌 — `internal/postfilter/agc.go:49-56`

```go
func (pf *Postfilter) applyAGC(sTilt *[subframeLen]int16, gTargetQ14 int16, sPf *[subframeLen]int16) {
	const alphaQ15 int64 = 32440 // ≈ 0.99; ITU-T G.729 §A.4.2.4

	gTargetQ24 := int64(gTargetQ14) << 10
	if !pf.initialized {
		pf.agcGainPrev = int32(gTargetQ24)
		pf.initialized = true
	}
```

→ 첫 호출 시 `agcGainPrev = g_target` (Q24). spec §4.2.4 / §A.4.2.4 는 g(−1) = 1.0 (= 16384 Q14, = 16777216 Q24) 명시. **production 의 seed (= g_target) ≠ spec seed (= 1.0)** — Phase 1i `f24add7` 시점의 변경(F-bis-1 보고서 §5 5번 항목 참조) 이 §4.2.4 마지막 문장과 어긋남. 이 시드 차이는 sf0 첫 몇 샘플의 AGC 평활화 출력 진폭에 직접 영향. 단, 본 단일 문제로 sample 0/2/3 에 *정확히 ×2* 과대 + sample 1 만 정상 인 패턴이 나오는지는 §2.3 hand-calc 가 부정.

### 2.3 hand-calc — "ScaleUpSat 단독 fix (가상 ÷2)" 가설의 충분/불충분

가상 시나리오 A: ScaleUpSat 를 pass-through 로 변경 (ScaleUpSat = identity):
- sample 0: 2 → 2 (= PST want 2) ✓
- sample 1: 2 → 2 (≠ PST want 4) ✗
- sample 2: 3 → 3 (= PST want 3) ✓
- sample 3: 3 → 3 (= PST want 3) ✓
- 결과: 3/4 일치, 1/4 (sample 1) 어긋남 → 단독 fix 불충분.

가상 시나리오 B: ScaleUpSat 를 ÷2 로 변경:
- sample 0: 2 → 1 (≠ PST want 2) ✗
- sample 1: 2 → 1 (≠ PST want 4) ✗
- sample 2: 3 → 2 (≠ PST want 3) ✗
- sample 3: 3 → 2 (≠ PST want 3) ✗
- 결과: 0/4 일치 → 가장 나쁜 선택.

가상 시나리오 C (현 production = spec ×2 유지):
- sample 0: 2 → 4 (≠ 2) ✗
- sample 1: 2 → 4 (= 4) ✓
- sample 2: 3 → 6 (≠ 3) ✗
- sample 3: 3 → 6 (≠ 3) ✗
- 결과: 1/4 일치.

→ **세 시나리오 어느 것도 4/4 일치를 만들지 못한다.** 즉, ScaleUpSat 를 어떤 방향으로 바꾸더라도 sf0 첫 4 샘플을 동시에 PST 와 비트-정확 일치시킬 수 없다. 이는 §1.7 의 "상류에서 sample 0/2/3 만 2× 부풀려지고 sample 1 은 정상" 패턴과 정합. **결함은 상류의 *sample-의존적* 이상**(예: filter 메모리 / past state 가 첫 호출 시 spec-위반 초기값을 가짐) 이며, 단일 출력-스케일 상수 변경으로는 닫히지 않는다.

### 2.4 추가 sub-fault 좁히기 — 권고 조사 항목 (F-tris 후보)

본 §은 sub-fault *식별*이 아닌 *좁히기*에 멈춘다 (escape hatch 발동 → 후속 플랜 책임). 권고 조사 순서:

1. **`postfilter.applyAGC` seed (`agc.go:53-56`) ↔ §4.2.4 마지막 문장**: seed 를 1.0 (Q14=16384, Q24=16777216) 로 변경 시 sample 0/2/3 의 ×2 패턴이 사라지는지 측정.
2. **`synth.filterSubframe` saturation recovery (`synth/filter.go:31-52`)** 가 P fix 후 stimulus 에서 *trigger되는지* 진단 로그 (Stage F partial §7 §3.10 인용 위반 지점). trigger 시 ÷2/×2 가 sample 별로 비대칭 결과를 산출할 수 있음.
3. **synth.Filter 의 Q-포맷 재검증** (`synth/filter.go:60-69`): a[0]=4096 Q12 입력에 대해 LMult→LShl(3)→Round 체인의 net Q-증폭이 spec 의 Q0 (또는 명시 Q-format) 와 일치하는지 line-by-line.

이 항목들은 본 보고서의 분석 범위 외 — 사용자 결정 후 새 플랜에서 다룰 것.

---

## 3. §3 F-bis-3 bundle 권고

**둘 다 권고 불가 — Hard escape hatch 발동.**

| 옵션 | 평가 |
|------|------|
| (가) ScaleUpSat fix + sample 1 sub-fault fix 단일 커밋 | **불가**. ScaleUpSat 는 spec-correct → "fix" 자체가 spec 위반. |
| (나) ScaleUpSat fix only (sample 1 deferred) | **불가**. 상동 — ScaleUpSat 는 변경 대상 아님. |

**대신 권고**: F-bis-3 진입을 *보류*하고 F-bis 플랜을 갱신. 구체적으로:

1. F-bis 플랜 §"§3.10 인용" 의 *÷2* 를 §4.2.5 *×2* 로 정정 (line 49-71 의 두 인용 블록).
2. F-bis-1 §3.2 의 "결함 위치 = pcm.ScaleUpSat" 결론을 "결함 *경계* = ScaleUpSat 직전, 결함 *방향* = 상류 진폭 2× 과대" 로 정정.
3. 새 태스크 (F-tris 또는 F-bis-2.5): §2.4 의 후보 3개를 stage-by-stage 진단 하니스 확장으로 좁혀, *상류* 결함 단계 단일 식별. 진단 비교 기준은 PST 의 절반 (= spec-mandated pre-×2 도메인) 으로 수정 (현 진단은 PST 와 직접 비교했음).
4. F-bis-3 (또는 F-tris-3) 단일 커밋: P fix + 상류 결함 fix + sample 40 가드. **ScaleUpSat 는 그대로 유지**.

---

## 4. §4 hand-calc 기대값 (F-bis-3 또는 후속 commit message 용)

**escape hatch 발동으로 본 §의 "결합 fix 후 기대값" 산출은 보류**. 상류 결함 식별 전까지 기대값을 위조-적합 하지 않음 (강압-적합 금지 원칙).

ScaleUpSat 가 spec-correct 임이 확정되었으므로, 후속 fix 의 기대값은 다음 *제약식*을 만족해야:

```
(상류 spec-correct 단계) × 2 (ScaleUpSat) = PST want sample n   (n = 0..39)
∴ 상류 spec-correct 단계 = PST want sample n / 2
```

ALGTHM frame 0 sf0 첫 4 샘플 기준:

| sample | PST want | spec-mandated pre-ScaleUpSat (= PST/2, ½-LSB rounding 허용) |
|---:|---:|---:|
| 0 | 2 | 1 |
| 1 | 4 | 2 |
| 2 | 3 | 1 (또는 2) |
| 3 | 3 | 1 (또는 2) |

상류 결함이 식별되면, 그 단계 fix 후 위 표의 pre-ScaleUpSat 열을 hpFilter 출력에서 비트-정확 측정 가능해야 한다.

---

## 5. §5 sf0 40-sample 비트-정확 도달 가능성 평가

**평가**: escape hatch 발동 상태에서 단정 불가. 그러나 §1.7 / §2.3 의 데이터로부터 다음을 추론 가능:

- 상류 결함이 *단일 진폭 2× 과대* + *sample-1 anomaly* 의 조합이라면, 두 결함을 동시 해소하는 fix 가 동일 root-cause (예: AGC seed) 일 가능성이 있음. 그 경우 sf0 40 샘플 중 다수가 비트-정확에 가까워질 가능성 있음.
- 그러나 §2.3 hand-calc 는 sample 1 의 anomaly 가 *진폭 2× 과대 패턴과 동일 메커니즘* 임을 보장하지 않는다. 별개의 sub-fault 인 경우, sf0 sample 40 까지 비트-정확이 sf0 잠금 가능 여부는 *상류 결함 식별 후* 측정으로만 판정.
- 따라서 **본 보고서 단계에서는 sf0 sample 40 가드 expectation 을 hand-calc로 산정하지 않는다.** F-tris 단계 진단 데이터 확보 후 결정 권고.

---

## 6. §6 Escape hatch 발동 정리

본 분석 중 **사용자 명시 hard escape hatch 조건이 충족**:

> "If your analysis of `pcm.ScaleUpSat` finds production already matches spec (impossibly — but if), report that conflict to the user and STOP — do not proceed with any bundle."

발동 근거:

1. §4.2.5: "The filtered signal is **multiplied by a factor 2** to restore the input signal level." (인용은 §1.1)
2. §A.4.2.5: "Same as described in clause 4.2.5." (인용은 §1.3)
3. production `scale.go:23`: `out[i] = fixed.Shl(in[i], 1)` = ×2 with int16 saturation. → spec 과 line-by-line 일치 (§1.6 표).
4. 플랜의 §3.10 *÷2* 인용 (`f-bis-plan.md:51-55, 65-71`) 은 *작성 오류* — 실제 §3.10 (인용은 §1.2) 는 메모리 갱신 절차로 출력 스케일링과 무관.

추가 발견 (escape hatch 와 동등 무게 — F-tris 트리거):

- F-bis-1 의 "단일-경로 ×2 결함" 가설은 stage-by-stage ratio 측정으로 *위치*는 식별했으나 *결함 방향*을 정반대로 추론. 데이터 자체는 유효 (재해석은 §1.7).
- 실제 결함은 `pcm.ScaleUpSat` 의 *상류*에서 sample-의존적 ×2 과대 진폭. 후보 1순위는 `postfilter.applyAGC` seed (§4.2.4 위반, §2.2 발췌).
- Phase 1i `TestDecode_Frame0Sample0_MatchesALGTHM` 통과는 *2개 결함의 우연한 상쇄*. P fix 가 첫 결함만 제거하여 두 번째 결함이 노출 (§1.8).

---

## 7. 다음 단계 — 사용자 결정 대기

- 본 보고서 커밋 후 working tree 는 *동일 상태* 유지 (P fix 미커밋 + 진단 하니스 미커밋).
- **F-bis-3 진입 금지**: bundle (가)/(나) 모두 ScaleUpSat 변경을 포함하나, 본 §은 변경 대상 아님이 입증됨.
- 사용자 결정 사항:
  1. F-bis 플랜의 §3.10 인용 정정 + F-bis-1 결론 재라벨링 (본 §3 1-2번).
  2. 새 진단 사이클 (F-tris) 진입 — *상류* 결함 단계 식별 (본 §2.4 후보 3개).
  3. 진단 비교 기준을 PST → PST/2 (spec-mandated pre-×2 도메인) 로 변경.
- 사용자 명시 승인 전까지 코드/하니스/플랜 수정 없음.
