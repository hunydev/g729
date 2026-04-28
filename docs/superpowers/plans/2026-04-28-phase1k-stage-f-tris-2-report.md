# Phase 1k Stage F-tris-2 분석 노트

**작성일**: 2026-04-28
**범위**: F-tris-1 시나리오 2(synth-stage 결함 — sample 0..7 contiguous block) 식별에 따른 ITU-T G.729 (06/2012) §-인용 + production 코드 line-by-line 대조 + hand-calc.
**산출물**: F-tris-3 fix 대상 후보 확정 또는 escape hatch 발동 결정.
**결론 요약**: 1순위 a/b (`synth.BuildExcitation`, `synth.Filter::onePass`) production 코드는 §3.9 / §4.2.4 / §A.4.2.4 와 *line-by-line 일치* — 두 단계 모두 결함 위치 아님 (escape hatch E1 부분-발동). 2순위 `gain.Decode` 에서 *spec-위반 후보* 1건 발견 (`decodeVQ` 가 GainImap1/GainImap2 매핑을 적용하지 않음 — 본 보고서 §4.2). 그러나 hand-calc 만으로는 본 후보 적용 시 sample 0..2 의 PST/2 정렬을 *예측 불가* (pow2/log2 비선형 체인 + decodeVQ → Decode → BuildExcitation → Filter 다단 의존). **Escape hatch E2 발동 — F-tris-3 직진 보류, 사용자 결정 대기 (F-quart 후보)**.

**준수**: ITU 참조 C, bcg729, Sipro Lab, FFmpeg G.729 등 외부 구현 미참조. 본 보고서 인용은 ITU-T G.729 (06/2012) 단일 PDF (`docs/superpowers/specs/itu/G729E.pdf`) §3.7 / §3.8 / §3.9 / §3.10 / §4.1.5 / §4.1.6 / §4.2.4 / §A.3.10 / §A.4.2 / §A.4.2.4 만 직접 인용.

---

## 0. Working tree 상태 (본 태스크 분석-only)

### 0.1 `git status` (본 보고서 커밋 직전)

```
On branch main
Your branch is ahead of 'origin/main' by 102 commits.

Changes not staged for commit:
	modified:   internal/lsp/lsp_lp.go

Untracked files:
	internal/decoder/stagef_bis_diagnostic_test.go

no changes added to commit
```

| 경로 | 상태 | 본 태스크 변경? |
|------|------|------|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix (int64 누산) | **아니오** |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (uncommitted) — F-bis-1/F-tris-1 진단 하니스 | **아니오** |
| 그 외 production 파일 | 변경 없음 | **아니오** |

본 보고서 커밋 후에도 working tree 는 위 두 파일의 미커밋 상태를 그대로 유지한다. 본 §의 단일 신규 파일 (`docs/superpowers/plans/2026-04-28-phase1k-stage-f-tris-2-report.md`) 만 staged → committed.

### 0.2 진단 데이터 재현 (F-tris-1 결과 재확인)

`go test ./internal/decoder/ -run TestDiagnostic_FtrisStageBoundaries_Sf0FullTrace -v` 출력 (sample 0..7 발췌, 본 보고서 작성 시점):

```
u[0..7] = [2 2 2 2 0 0 0 0]
a[] (Q12) = [4096 -2197 -375 -4 -144 -68 303 -36 -90 145 -33]

PST want sf0      [ 0.. 7]     2     4     3     3     1    -1    -1    -1
PST/2 spec-target [ 0.. 7]     1     2     1     1     0    -1    -1    -1
synth.Filter      [ 0.. 7]     2     3     4     4     3     2     1     1
postfilter.Filter [ 0.. 7]     2     2     3     4     3     2     1     2
hpFilter          [ 0.. 7]     2     2     3     3     2     1     0     1

Match count vs PST/2 (|Δ|≤1 LSB):
  synth.Filter:      33 / 40
  postfilter.Filter: 32 / 40
  hpFilter:          34 / 40
```

F-tris-1 보고와 일치 확인.

---

## 1. ALGTHM.PST frame 0 sample 0..7 원본 + PST/2 재계산 (typo 검증)

`readPSTFrames` 가 반환한 `wantFrames[0][0..7]` 원본 값 (위 §0.2 발췌) 을 직접 인용해 *F-tris-1 의 PST/2 표시에 누적 오기가 없음* 을 확인.

| n | PST want (직접 인용) | PST/2 = `int16(want >> 1)` (Go arithmetic shift) | F-tris-1 production hpFilter | F-tris-1 production synth.Filter |
|---|---:|---:|---:|---:|
| 0 |  2 |  1 |  2 | 2 |
| 1 |  4 |  2 |  2 | 3 |
| 2 |  3 |  1 |  3 | 4 |
| 3 |  3 |  1 |  3 | 4 |
| 4 |  1 |  0 |  2 | 3 |
| 5 | −1 | −1 |  1 | 2 |
| 6 | −1 | −1 |  0 | 1 |
| 7 | −1 | −1 |  1 | 1 |

검증 메모:
- Go `int16` 산술 우-시프트 의 sign 보존 → `(-1) >> 1 = -1`. PST/2[5..7] = −1 정확.
- 양수 truncation: `3 >> 1 = 1` (round-toward-zero 가 아니라 floor). `1 >> 1 = 0`.
- F-tris-1 보고문의 PST/2 표 수치와 본 표 일치 → **typo 없음**.

**관찰 1 — 단일 ×2 가설의 부정 (F-bis-2 §1.7 해석의 정밀화)**:

| n | PST want (=A) | PST/2 (=B) | synth.Filter (=S) | S/B 비율 | S/A 비율 |
|---|---:|---:|---:|---:|---:|
| 0 |  2 |  1 |  2 | 2.00 | 1.00 |
| 1 |  4 |  2 |  3 | 1.50 | 0.75 |
| 2 |  3 |  1 |  4 | 4.00 | 1.33 |
| 3 |  3 |  1 |  4 | 4.00 | 1.33 |
| 4 |  1 |  0 |  3 |  ∞   | 3.00 |
| 5 | −1 | −1 |  2 | -2.00 | -2.00 |

→ synth.Filter sf0 의 sample 0..7 은 *PST 도메인도 PST/2 도메인도 일정-비율로 매핑되지 않음*. F-bis-2 §1.7 의 "synth 단계가 ‘대략 PST 도메인’" 표현은 sample 0 한 점만 관찰한 단순 해석. 실제로는 sample 별 비율이 1.00, 0.75, 1.33, 1.33, 3.00, −2.00 로 *부호 반전과 비선형 진폭 모두 발생*. 결함은 단순 1-bit shift 가 아니다.

**관찰 2 — 33/40 ~ 34/40 already 일치**:

샘플 11..21 의 hpFilter 출력 = −1 (PST/2 와 일치, 11 샘플 연속). 샘플 29..39 의 모든 stage 출력 = 0 (PST/2 = 0 과 일치, 11 샘플). 결함은 *transient 시작 구간 sample 0..10* 에 집중 (그 외 ~28 샘플은 이미 PST/2 와 |Δ|≤1 LSB).

---

## 2. 1순위 a — `synth.BuildExcitation` 분석

### 2.1 §3.9 / §4.1.6 spec 인용

PDF p.21, §3.9 식 (75):

> "u(n) = ĝ_p · v(n) + ĝ_c · c(n)        n = 0,...,39    (75)"

PDF p.27, §4.1.6 동일 식 재인용 (디코더 측 — 인코더 §3.9 와 동일 수식).

§3.9 Q-format 명시는 spec 본문에 *없음* (spec 은 알고리즘만 규정, fixed-point Q-format 분배는 구현 선택). 단, §A 와 §3.7 / §3.8 가 입력 v / c 의 정수 표현 폭만 묵시:
- v(n): adaptive codebook output, integer Word16 (Q0 — past excitation 이 Q0 이므로 LP-domain 단위).
- c(n): algebraic codebook impulses, ±1.0 with chosen Q-format. spec §4.1.5 는 "amplitude ±1" 만 명시; 본 codebase 는 Q13 선택 (`internal/fcb/types.go:14` const PulseAmplitude = 8192 — `// expressed in Q13 (+1.0 exactly).`).

### 2.2 Production line-by-line

`internal/synth/excitation.go:27-34`:

```go
func BuildExcitation(gpQ14, gcQ12 int16, v, c *[40]int16, u *[40]int16) {
	for n := 0; n < 40; n++ {
		lPitch := fixed.LMult(fixed.Word16(gpQ14), fixed.Word16(v[n]))
		lCode := fixed.LShr(fixed.LMult(fixed.Word16(gcQ12), fixed.Word16(c[n])), 11)
		lSum := fixed.LAdd(lPitch, lCode)
		u[n] = int16(fixed.Round(fixed.LShl(lSum, 1)))
	}
}
```

### 2.3 Q-format 사슬 line-by-line 검증 표

LMult 정의 (G.729 표준 fixed primitive): `LMult(a,b) = (a*b) << 1` (= 2·a·b in Q(qa+qb+1)).
LShr / LShl: 산술 우/좌 시프트.
Round: `(x + 0x8000) >> 16` 후 Word16 saturation.

| line | 연산 | 입력 Q | 출력 Q (실제) | 출력 Q (spec equivalent) | 일치 |
|------|------|--------|---------------|--------------------------|---|
| 29 | `LMult(gpQ14, v)` | gp Q14 × v Q0 | Q14+0+1 = **Q15** | ĝ_p·v(n) Q15 | ✓ |
| 30 | `LMult(gcQ12, c)` | gc Q12 × c Q13 | Q12+13+1 = **Q26** | ĝ_c·c(n) Q26 | ✓ |
| 30 | `LShr(…, 11)` | Q26 → Q26-11 | **Q15** | Q15 (lPitch 와 정합) | ✓ |
| 31 | `LAdd(lPitch, lCode)` | Q15 + Q15 | **Q15** | u(n) in Q15 (sum) | ✓ |
| 32 | `LShl(lSum, 1)` | Q15 → Q16 | **Q16** | (round-prep) | ✓ |
| 32 | `Round(…)` | Q16 → Q0 (`+0x8000)>>16`) | **Q0** Word16 | u(n) Q0 (= 정수 진폭) | ✓ |

**Hand-trace (frame 0 sf0, n=0)**: gpQ14=13815, gcQ12=6844, v[0]=0 (frame 0 sf0 → empty pastExc 로부터 t=20 lookup → 0), c[0]=8192 (가정 — sf0 의 한 pulse 위치 추정. 실제는 `placePulses` 가 결정; n=0 이 pulse 위치인지 무관하게 본 식의 Q-format 검증은 동일).

ĝ_p · v(0) = 13815 × 0 / 16384 = 0
ĝ_c · c(0) = 6844 × 8192 / 4096 / 8192 = 6844/4096 = 1.671 (real)
u(0) = round(0 + 1.671) = 2 ✓ (`u[0..7] = [2 2 2 2 0 0 0 0]` 과 일치)

Production 에서:
- lPitch = LMult(13815, 0) = 0
- lCode = LShr(LMult(6844, 8192), 11) = LShr(2·6844·8192, 11) = LShr(112,164,864, 11) = 54,768
- lSum = 0 + 54,768 = 54,768
- LShl(54,768, 1) = 109,536
- Round = (109,536 + 32,768) >> 16 = 142,304 >> 16 = **2** ✓ (= u[0])

### 2.4 §2 결론

`synth.BuildExcitation` 의 모든 라인이 §3.9 식 (75) 와 line-by-line *spec-correct*. Q-format 사슬 LMult Q26 → LShr 11 → Q15 → LShl 1 → Q16 → Round → Q0 은 ITU 표준 fixed-point primitive 의 표준 사용 패턴이며, *±1-bit 어긋남 없음*. **결함 위치 아님 — escape hatch E1 부분-발동**.

본 결과는 F-bis-2 ScaleUpSat case 의 *재현* 이다 (F-bis-2 §1.6 표 참조): 두 단계 모두 production 이 spec § 인용과 완전 일치 → spec-위반 가설 부정.

---

## 3. 1순위 b — `synth.Filter::onePass` 분석

### 3.1 §4.2.4 / §A.4.2.4 spec 인용

PDF p.28, §4.2.4 (LP synthesis filter):

> "The reconstructed speech for the subframe of size 40 is given by:
>
>   ŝ(n) = û(n) − Σ_{i=1..10} â_i · ŝ(n−i)        n = 0,...,39"

(식 번호 본 PDF 페이지에는 부여되지 않음; 본 인용은 §4.2.4 첫 문장과 그 다음 식 그대로.)

PDF p.43, §A.4.2.4: G.729A 상속 — 본문 §4.2.4 의 식 동일, AGC 부분만 변경.

§3.10 / §A.3.10 saturation recovery (PDF p.24, §3.10):

> "When overflow occurs, the speech samples and the filter memory are divided by 4 and the filtering is re-done. The output is multiplied by 4 with saturation."

### 3.2 Production line-by-line

`internal/synth/filter.go:60-69` (`onePass`):

```go
func (synth *Synthesizer) onePass(a *[11]int16, u *[40]int16, work *[50]int16) {
	for n := 0; n < 40; n++ {
		lTemp := fixed.LMult(u[n], a[0])
		for i := 1; i <= 10; i++ {
			lTemp = fixed.LMsu(lTemp, a[i], work[10+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[10+n] = fixed.Round(lTemp)
	}
}
```

`internal/synth/filter.go:19-55` (`filterSubframe` saturation recovery):

```go
func (synth *Synthesizer) filterSubframe(a *[11]int16, u, s *[40]int16) {
	var work [50]int16
	copy(work[:10], synth.pastSynth[:])

	fixed.ClearOverflow()
	synth.onePass(a, u, &work)
	if !fixed.Overflow() {
		copy(s[:], work[10:])
		copy(synth.pastSynth[:], work[40:])
		return
	}

	// Pass 2: scale input and past state by 1/2.
	var work2 [50]int16
	for i, v := range synth.pastSynth {
		work2[i] = int16(int32(v) >> 1)
	}
	var uScaled [40]int16
	for i, v := range u {
		uScaled[i] = int16(int32(v) >> 1)
	}
	fixed.ClearOverflow()
	synth.onePass(a, &uScaled, &work2)

	// Scale back up by ×2 with Word16 saturation.
	for i := 10; i < 50; i++ {
		v := int32(work2[i]) << 1
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		work2[i] = int16(v)
	}
	copy(s[:], work2[10:])
	copy(synth.pastSynth[:], work2[40:])
}
```

LMsu 정의 (표준 G.729 primitive): `LMsu(acc, a, b) = LSub(acc, LMult(a, b)) = acc − 2·a·b` (saturating).

### 3.3 LMult/LMsu/LShl/Round 체인 net Q-증폭 검증

a[0] = 4096 (Q12 = 1.0 정확). u (Q0 정수 진폭). work[10+n-i] (Q0 정수 진폭).

| line | 연산 | 입력 Q | 출력 Q | spec equivalent | 일치 |
|------|------|--------|--------|-----------------|---|
| 62 | `LMult(u[n], a[0])` | u Q0 × a[0] Q12 | Q0+12+1 = **Q13**, 값 = 2·u·4096 | u(n)·a_0 in Q13 (= u(n) in Q12 then ×2) | ✓ |
| 64 | `LMsu(lTemp, a[i], work[…])` | Q13 − 2·a[i]·s[n-i] (Q12+0+1=Q13) | **Q13** | 누적 −Σ a_i·s(n−i) Q13 | ✓ |
| (loop end) | lTemp = 2·(u·a[0] − Σ a[i]·work[…]) | | **Q13** = 2·A(z)·s(n) in Q12 | 2·s(n)·4096 in Q13 (= s(n) Q0 의 Q12 표현) | ✓ |
| 66 | `LShl(lTemp, 3)` | Q13 → Q16 | **Q16**, 값 = 16·(A(z)·s(n) in Q12) = (A(z)·s(n)) << 16 | 라운드-prep | ✓ |
| 67 | `Round(lTemp)` | Q16 → Q0 (`+0x8000)>>16`) | **Q0** Word16 | s(n) Q0 (정수 진폭) | ✓ |

**Net Q-증폭 검증**: 출력 work[10+n] = round( (2·(u·a[0] − Σ a[i]·s[n-i])) × 8 / 65536 )
                                       = round( (u·4096 − Σ a[i]·s[n-i]) / 4096 )
                                       = round( u(n) − Σ (a[i]/4096)·s[n-i] )
                                       = round( u(n) − Σ a_real_i · s(n-i) )

→ §4.2.4 식 ŝ(n) = û(n) − Σ â_i·ŝ(n−i) 와 *비트-정확 일치* (Word16 round/sat 한도 내).

**Hand-trace (frame 0 sf0, n=0)**: u[0]=2, past synth = 0 (decoder 초기화 직후), a[]=[4096, −2197, −375, −4, −144, −68, 303, −36, −90, 145, −33].

- lTemp = LMult(2, 4096) = 2·2·4096 = 16,384 (Q13)
- Loop i=1..10: 모든 work[…] = 0 → lTemp 변화 없음 → 16,384
- LShl(16384, 3) = 131,072 (Q16)
- Round = (131,072 + 32,768) >> 16 = 163,840 >> 16 = **2** ✓ (= synth.Filter[0])

### 3.4 Saturation recovery ÷2/×2 (production) vs spec ÷4/×4

**Production `filter.go:31-52`**: `int32(v) >> 1` (÷2) + `int32(v) << 1` (×2 with saturation).
**Spec §3.10 / §A.3.10 (위 §3.1 인용)**: "divided by 4 ... multiplied by 4 with saturation".

→ **Spec-위반 (1-bit). 그러나 본 stimulus 에서 trigger 안 됨**:

ALGTHM frame 0 sf0 hand-trace (위 §3.3): lTemp 최대값 ≈ 16,384, LShl(3) 후 ≈ 131,072 (Q16, Word32 한도 ≪ 2^31). u 값 = [2,2,2,2,0,…], past synth = 0. **Pass 1 에서 LMult/LMsu/LShl/Round 어디서도 overflow 발생 안 함** → `fixed.Overflow()` false → Pass 2 (÷2/×2 recovery) 미진입. 따라서 본 spec-위반은 sample 0..7 결함의 *원인 아님*.

(별도 진단 항목 — F-prep-2 가 이미 §A.3.10 위반 노출. 현재 stimulus 에서 무관, 그러나 향후 다른 frame 에서 trigger 시 결함 노출 가능. 본 보고서 권고 §7 참조.)

### 3.5 §3 결론

`synth.Filter::onePass` 의 모든 라인이 §4.2.4 식 ŝ(n) = û(n) − Σ â_i·ŝ(n−i) 와 line-by-line *spec-correct*. LMult/LMsu/LShl(3)/Round 체인 net Q-증폭 = ×1 (= Q0 입력 → Q0 출력). Hand-trace 가 production 출력 synth.Filter[0]=2 와 비트-정확 재현. **결함 위치 아님 — escape hatch E1 부분-발동**.

`filterSubframe` saturation recovery (÷2/×2) 는 spec §3.10 / §A.3.10 (÷4/×4) 위반이지만, 본 stimulus 에서 미-trigger → sample 0..7 결함의 직접 원인 아님.

---

## 4. 2순위 — `gain.Decode` 분석 (1/2순위 spec-일치 진입)

§2 / §3 가 spec-일치로 결함 위치 부정 → 우선순위 사슬 따라 §3.9 gain VQ + log-gain 예측 + γ̂_c 디코드 분석으로 강하.

### 4.1 §3.9 spec 인용 + GA1=5/GB1=6 손계산

PDF p.21-22, §3.9 (gain quantization):

> "The pitch and fixed-codebook gains are vector-quantized using a conjugate-structure two-stage VQ with 6+4 = 10 bits ... The first stage consists of a 3 bit two-dimensional codebook GBK1 and the second stage consists of a 4 bit two-dimensional codebook GBK2. The first element in each codebook represents the quantized adaptive-codebook gain ĝ_p, and the second element represents the quantized fixed-codebook gain correction factor γ̂_c."
>
> "ĝ_p = GA1[GA][0] + GB1[GB][0]
>  γ̂_c = GA1[GA][1] + GB1[GB][1]"

(본 PDF 의 표기 GA1/GB1 는 본 codebase 의 GainGBK1/GainGBK2 와 동일.)

PDF p.22, §3.9.3 (codebook reordering):

> "To reduce the impact of single bit errors, the GA and GB indices are reordered before transmission. The mapping tables are given in Annex C/D."

→ Spec 명시: 인코더는 GBK 의 *물리적 entry index* 를 *전송 비트열* 로 *매핑* (Map). 디코더는 *전송 비트열* 을 받아 *역-매핑* (Imap) 후 GBK 인덱싱. 본 codebase 의 `GainMap1` / `GainImap1` (`internal/tables/gain_gbk1.go:36-44`) 가 정확히 이 매핑.

### 4.2 Production line-by-line

`internal/gain/vq.go:18-22`:

```go
func decodeVQ(idx Indices) (gpQ14, gammaCQ13 int16) {
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[idx.GA][0]), fixed.Word16(tables.GainGBK2[idx.GB][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[idx.GA][1]), fixed.Word16(tables.GainGBK2[idx.GB][1])))
	return
}
```

`internal/tables/gain_gbk1.go:43-44`:

```go
// GainImap1 is the inverse of GainMap1: given the transmitted GA
// bit pattern, returns the physical GBK1 entry index. Decoder-side:
// `entry = GainGBK1[GainImap1[GA]]`.
var GainImap1 = [8]uint8{5, 1, 7, 4, 2, 0, 6, 3}
```

`internal/tables/gain_gbk2.go:36-38`:

```go
// GainImap2 inverts GainMap2: decoder uses
// `entry = GainGBK2[GainImap2[GB]]`.
var GainImap2 = [16]uint8{2, 14, 3, 13, 0, 15, 1, 12, 6, 10, 7, 9, 4, 11, 5, 8}
```

`grep -rn "GainImap" internal/` (본 보고서 작성 시점):

```
internal/tables/gain_gbk1.go:41:// GainImap1 is the inverse of GainMap1...
internal/tables/gain_gbk1.go:44:var GainImap1 = [8]uint8{5, 1, 7, 4, 2, 0, 6, 3}
internal/tables/gain_gbk2.go:36:// GainImap2 inverts GainMap2...
internal/tables/gain_gbk2.go:38:var GainImap2 = [16]uint8{2, 14, 3, 13, 0, 15, 1, 12, 6, 10, 7, 9, 4, 11, 5, 8}
```

→ **`GainImap1` / `GainImap2` 는 정의되어 있으나 *production decoder 어디에서도 사용되지 않음***. `decodeVQ` 가 `GainGBK1[idx.GA]` 직접 인덱싱 = §3.9.3 의 디코더-측 inverse mapping 누락.

### 4.3 Q-format 사슬 검증 표 (1순위와 동일 형식)

| 단계 | 변수 | spec Q | production Q | 일치 |
|------|------|--------|--------------|---|
| GBK1[…][0] | g_p 성분 | Q14 (GBK 정의) | Q14 | ✓ (값 자체는 Q-포맷 일치) |
| GBK1[…][1] | γ̂_c 성분 | Q13 | Q13 | ✓ |
| `Add(GBK1, GBK2)` | sum | Q14 / Q13 | Q14 / Q13 (saturating) | ✓ |
| **GBK 인덱싱 *대상*** | physical entry | `GBK[Imap[bits]]` | `GBK[bits]` (Imap 누락) | **✗ — 본 결함** |

Q-포맷 자체는 spec 일치. 결함은 *인덱싱 정책* 이 §3.9.3 의 디코더-측 inverse-map 절차를 따르지 않음.

### 4.4 GA1=5, GB1=6 손계산 — production vs spec 비교

| 인덱싱 정책 | GBK1 entry idx | GBK2 entry idx | g_p (Q14) | g_p (real) | γ̂_c (Q13) | γ̂_c (real) |
|-------------|---------------:|---------------:|----------:|-----------:|----------:|-----------:|
| **(가) Production** (`GBK[bits]`) | GBK1[5]=(3242, 9949) | GBK2[6]=(10573, 2966) | 3242 + 10573 = **13815** | 0.843 | 9949 + 2966 = **12915** | 1.577 |
| **(나) Spec § 3.9.3** (`GBK[Imap[bits]]`) | GainImap1[5]=0 → GBK1[0]=(1, 1516) | GainImap2[6]=1 → GBK2[1]=(1994, 0) | 1 + 1994 = **1995** | 0.122 | 1516 + 0 = **1516** | 0.185 |

Diagnostic 출력 `gpQ14=13815` (= 0.843) 가 *(가)* production 분기 일치 → 본 codebase 가 spec § 3.9.3 의 inverse map 을 누락한 것이 *증명됨* (수치 일치).

### 4.5 결론

**§3.9.3 디코더-측 inverse map 누락 — spec-위반 (decodeVQ 가 GainImap1/GainImap2 미적용).** Q-format 자체는 spec 일치. 결함은 *데이터 매핑 정책*. 본 결함이 ALGTHM bit-stream 에 대해 잘못된 g_p / γ̂_c 를 생성하므로, 후속 `gain.Decode` (gc 계산) → `BuildExcitation` (u 계산) → `synth.Filter` (s 계산) 전체 사슬에 1차 영향.

→ **F-tris-3 fix 후보 #1 (확정 spec-위반)**. 단, hand-calc 만으로 sample 0..2 PST/2 정렬 예측 불가 (§6 참조).

---

## 5. 3순위 — pitch / fcb 분석 (조건부 진입)

§4 가 *spec-위반 후보 1건* 발견 → 3순위 진입 *조건부* (2순위가 spec-일치였다면 진입). 본 §은 **참고용 short scan** 만 수행 (확정 결함 후보 발견 후 깊은 hand-calc 보류).

### 5.1 `pitch.AdaptiveCodebook` (tInt=20, tFrac=0)

Frame 0 sf0: `tInt=20, tFrac=0`. `internal/pitch/adaptive.go:39-46`:

```go
if tFrac == 0 && tInt >= 40 {
    base := len(pastExc) - tInt
    for n := 0; n < 40; n++ {
        v[n] = pastExc[base+n]
    }
    return
}
```

tInt=20 < 40 → 위 fast path 미진입. `adaptive.go:48-66` (short pitch) 분기 진입:

```go
if tFrac == 0 {
    base := len(pastExc) - tInt
    for n := 0; n < tInt; n++ {
        v[n] = pastExc[base+n]
    }
}
for n := tInt; n < 40; n++ {
    v[n] = v[n-tInt]
}
```

frame 0 sf0 → `pastExc` 전부 0 → v[0..19] = 0 (pastExc 끝 20 샘플 복사) → v[20..39] = v[0..19] = 0 → **v[0..39] = 0 전체**.

Diagnostic `u[0..7] = [2 2 2 2 0 0 0 0]` 와 정합 (gp·v = 0 → u 는 gc·c 만으로 결정).

§3.7 spec 명시: integer-delay 단순 복사. 본 short-pitch periodicity extension 은 §3.7.1 / §A.3.7 spec 의 인코더 처리와 정합 (디코더 측 단순 모방). 결함 후보 약함 — 모든 출력이 0 인 sf0 frame 0 에서 본 단계의 영향은 *0 누락*.

### 5.2 `fcb.Decode` (C1=0, S1=15)

`internal/fcb/decode.go:20-24` + `internal/fcb/positions.go` (decodePositions) + `internal/fcb/signs.go` (placePulses) + `applyPitchEnhancement(c, t=20, betaQ14)`.

Frame 0 sf0: `betaQ14 = ClampPitchGainForEnhancement(d.prevGpQ14)` 에서 `d.prevGpQ14 = 0` (decoder zero-value) → β 가 spec 의 lower-bound clamp 결정. 이후 c[0..39] 에 ±8192 (Q13) pulse 4개 + pitch enhancement.

C1=0 의 정확한 pulse 위치 는 `decodePositions` 의 13-bit 분해 결과에 의존. S1=15 (= 0b1111) → 4 pulse 모두 +PulseAmplitude (`signs.go:23` 의 sign_bit=1 → +). u[0..3] = 2 = round(0.843·0 + 1.671·1) ⇒ c[0..3] 중 적어도 일부에 +8192 pulse 존재.

본 §의 spec § 인용 분석은 결함 후보 #1 (gain Imap 누락) 식별 후 **보류** — 후속 cycle 권고.

### 5.3 §5 결론

3순위 후보 (pitch / fcb) 의 깊은 분석은 §4 의 후보 #1 발견으로 *deferred*. F-tris-3 사이클이 후보 #1 fix 적용 후 잔여 결함이 남으면 본 §의 깊은 분석을 재개해야 함.

---

## 6. Hand-calc — 후보 fix 가 sample 0/1/2 를 PST/2 에 정렬시키는지

### 6.1 후보 fix 적용 시 g_p / γ̂_c 변화

§4.4 표 *(나)* 에 따라 GainImap1/GainImap2 적용 시:
- g_p (Q14): 13815 → **1995** (= ×0.144)
- γ̂_c (Q13): 12915 → **1516** (= ×0.117)

### 6.2 후보 fix 적용 시 g_c (Q12) 추정

`gain/decode.go:74-92` 의 g_c 계산 사슬:
1. predicted = `predictedLogGain()` (frame 0 sf0 → pastErrors 전부 default = −14336 Q10) → Q10 dB.
2. ecBarDbQ10 = 10·log10(E_c/40) Q10 from `c[]` (≈ 동일 — c 는 unchanged).
3. logGainDbQ10 = predicted − ecBarDbQ10.
4. log2GcQ10 = logGainDbQ10 × invDbScaleQ15 / 32768.
5. **gc0Q14 = pow2Fixed(log2GcQ10 + 14·1024)** (= 2^(log2Gc) × 2^14 in Q0).
6. **gcQ12 = γ̂_c · gc0Q14 / 2^15, sat to int16.**

후보 fix 는 γ̂_c 만 변경 (12915 → 1516, ×0.117). 단계 1-5 는 c[] / pastErrors 의존이므로 *unchanged* — 즉 gc0Q14 는 동일.

따라서 *production* gcQ12 = 6844 = γ̂_c·gc0Q14/2^15 ⇒ gc0Q14 = 6844·32768/12915 ≈ **17363** (= 1.060 real).

후보 fix gcQ12 = 1516·17363/32768 = 26,322,308/32768 ≈ **803** (Q12, = 0.196 real).

### 6.3 후보 fix 적용 시 u[0..3] 추정

frame 0 sf0: v[0..39] = 0 (§5.1) → u(n) = round(γ̂_c·gc_term... — 사실 직접: u(n) = round(gc_real · c_real(n))).

assume c[0..3] 에 한 pulse +8192 (Q13 = +1.0 real) 존재 (production u[0..3]=[2,2,2,2] 와 정합 — pitch enhancement 이후 c 가 양수 4-pulse pattern). 후보 fix u(n) ≈ round(0.196 · c_real(n)).

만약 c_real(0)=1 → u_after(0) ≈ round(0.196) = **0**.

만약 c_real(0)≈3 (pitch enhancement β·c(n−20) 로 누적이 있는 경우, 3·0.196 ≈ 0.588 → round = 1).

→ 후보 fix 적용 시 u[0] ∈ {0, 1} (정확값은 c[] 와 β 의존). Production u[0] = 2.

### 6.4 후보 fix 적용 시 synth.Filter sample 0..2 추정

§3.3 hand-trace: past synth = 0, a[0]=4096, a[1]=−2197, a[2]=−375.

s(0) = u(0) (∵ past = 0)
s(1) = u(1) − (a[1]/4096)·s(0) = u(1) + 0.5365·s(0)
s(2) = u(2) − (a[1]/4096)·s(1) − (a[2]/4096)·s(0) = u(2) + 0.5365·s(1) + 0.0916·s(0)

| 시나리오 | u[0..2] | s(0) | s(1) | s(2) | PST/2 = [1, 2, 1] | 일치? |
|---------|---------|-----:|-----:|-----:|--------|---|
| Production | [2, 2, 2] | 2 | 3.07 → **3** | 4.27 → **4** | [1,2,1] | n=0:Δ+1, n=1:Δ+1, n=2:Δ+3 |
| 후보 fix (u≈[1,1,1]) | [1, 1, 1] | 1 | 1.54 → **2** | 2.13 → **2** | [1,2,1] | n=0:Δ0, n=1:Δ0, n=2:Δ+1 |
| 후보 fix (u≈[0,0,0]) | [0, 0, 0] | 0 | 0 | 0 | [1,2,1] | n=0:Δ−1, n=1:Δ−2, n=2:Δ−1 |

**관찰**:
- 후보 fix 가 u≈[1,1,1] 을 만들면 sample 0/1 비트-정확, sample 2 |Δ|=1 (matchCount tolerance 내).
- 후보 fix 가 u≈[0,0,0] 을 만들면 sample 0 −1, sample 1 −2 (over-correction).
- *후보 fix 의 정확한 u 값은 c[] / β / pow2/log2 비선형 체인 의존* — hand-calc 만으로 결정 불가.

### 6.5 §6 결론 — Hand-calc 정렬 예측 불가

후보 fix (gain Imap 누락 보정) 가 sample 0/1/2 를 PST/2 에 정렬시킬지는:
- 정렬 시나리오 (u≈[1,1,1]): 가능성 있음 — γ̂_c × pulse_amp 에 pitch enhancement 반영이 적절한 경우.
- 과보정 시나리오 (u≈[0,0,0]): 가능성 있음 — γ̂_c 를 0.117× 한 만큼 gc 도 그 비율로 줄어 u 가 0 으로 떨어짐.
- 어느 시나리오인지 *결정하려면* `gain.Decode` 를 후보 fix 적용 상태로 *실제 실행* 또는 c[]·β·pow2·log2 전체 체인 hand-calc 필요. 본 보고서 분석 범위 외.

→ **사용자 명시 escape hatch E2 발동: "hand-calc 결과가 PST/2 정렬 예측 못함 → 후보 부정, 다음 우선순위로 이동". 그러나 §5 가 3순위 깊은 분석 보류 상태이므로, "다음 우선순위" 도 즉시 진입 불가.**

선택지:
- (X1) 후보 fix 의 영향을 *진단 하니스로 측정* — 코드 변경 0, 별도 통과-실험 진단 (gain.decodeVQ 를 test 파일에서 직접 GainImap 적용 후 비교). 본 보고서 분석 범위 외 (분석-only mandate). F-tris-3 / F-quart 진단 task 권고.
- (X2) §5 의 3순위 분석 (pitch / fcb / decodeVQ→Decode pow2 chain) 깊은 hand-calc 진입. 본 보고서 범위 초과 — F-quart 사이클 권고.

---

## 7. F-tris-3 진입 권고

**확정 spec-위반 후보**:
- `internal/gain/vq.go:19-20` — `decodeVQ` 가 `tables.GainImap1[idx.GA]` / `tables.GainImap2[idx.GB]` 적용을 누락. ITU-T G.729 §3.9.3 디코더-측 inverse map 절차 위반. (§4 본 보고서 발견.)

**그러나 hand-calc 정렬 예측 불가** (§6.5) → 본 후보를 *F-tris-3 단일-fix* 로 진입 시 위험:
- 후보 fix 가 over-correction 이면 sample 0..7 결함은 부호 반전된 형태로 *재현* (escape hatch E2 reaffirm).
- 추가 결함 (pitch FIR / fcb pulse 위치 / pitch enhancement β / pow2/log2 체인) 가 누적되어 있을 가능성.

### 7.1 권고 — Escape hatch E2 발동 + 진단 사이클 권고

**F-tris-3 직진 보류**. 대신 *F-quart 사이클 (또는 F-tris-3 을 진단-only task 로 재정의)*:

1. **GainImap 후보 진단 (코드 0-수정)**: 별도 진단 test 가 `decodeVQ` 의 두 분기 출력을 비교 — 본 보고서 §4.4 표 *(나)* 의 g_p=1995 / γ̂_c=1516 으로 `gain.Decode` 호출 → resulting gc / u / s 측정 → PST/2 정렬도 비교.
2. **§5 3순위 깊은 분석**: pitch.AdaptiveCodebook (frame 0 sf0 의 short-pitch periodicity), fcb.Decode (C1=0 pulse 위치, S1=15 sign), pitch enhancement (β·c(n−20)) 각 단계 spec 인용 + Q-format 검증.
3. **gain.Decode 비선형 체인 검증**: predictedLogGain (pastErrors default), ecBarDbQ10 (c[] energy), pow2Fixed / log2Fixed 의 §3.9 spec 일치 line-by-line.

위 3 개 진단을 *별도 commit 0건* 으로 진행 → 데이터 확보 후 fix 후보 단일 식별 → 그 fix 만 단일 commit. 본 절차가 강압-적합 회피의 유일한 안전 경로.

### 7.2 직진 옵션 (사용자 명시 거부 가능)

만약 사용자가 "GainImap fix 단독 진입" 을 명시 승인하면, F-tris-3:
- `internal/gain/vq.go:19-20` 의 두 라인을 `tables.GainGBK1[tables.GainImap1[idx.GA]]` / `tables.GainGBK2[tables.GainImap2[idx.GB]]` 로 변경.
- 진단 test 로 sample 0..7 PST/2 정렬도 측정 (RED 가능성 ≥ 50%).
- RED 시 즉시 revert + F-quart 진입.

본 옵션은 escape hatch E2 의 사용자 override 에 해당.

---

## 8. Escape hatch 평가

| 해치 | 발동 여부 | 근거 |
|------|---------|------|
| **E1** (후보 단계 production = spec § 인용) | **부분-발동 (1순위 a/b)** | §2.4 / §3.5 — `synth.BuildExcitation` 와 `synth.Filter::onePass` 양 단계 모두 line-by-line spec 일치. 두 단계 모두 결함 위치 부정. F-bis-2 ScaleUpSat case 의 *재현*. |
| **E2** (hand-calc 정렬 예측 못함) | **발동** | §6.5 — 2순위 후보 (gain Imap 누락 fix) 의 sample 0/1/2 PST/2 정렬 예측이 c[] / β / pow2/log2 비선형 체인 의존으로 결정 불가. F-tris-3 단일-fix 직진 위험 → F-quart 사이클 권고 (§7.1). |
| **E3** (production 코드 수정/커밋 금지) | **준수** | §0.1 — working tree 변경 0, 본 보고서 파일만 단일 staged → committed. F-bis-1 P fix + 진단 하니스 미커밋 상태 그대로 유지. |
| **E4** (외부 G.729 구현 0건 참조) | **준수** | 본 보고서 인용 = ITU-T G.729 (06/2012) PDF §3.7 / §3.8 / §3.9 / §3.10 / §4.1.5 / §4.1.6 / §4.2.4 / §A.3.10 / §A.4.2 / §A.4.2.4 만. ITU 참조 C / bcg729 / Sipro / FFmpeg 미참조. `decodeVQ` 의 GainImap 누락 발견은 *본 codebase 의 자체 docstring* (`gain_gbk1.go:41-44`, `gain_gbk2.go:36-38`) + spec §3.9.3 인용으로만 도출. |

**최종 판정**: E1 부분-발동 + E2 발동 + E3/E4 준수. **F-tris-3 직진 금지 — 사용자 결정 대기 (§7.1 F-quart 권고 또는 §7.2 단일-fix override)**.
