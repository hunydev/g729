# Phase 1k Stage F-quart-2 진단 노트 — pitch / fcb / pitch enhancement spec 인용

**작성일**: 2026-04-28
**범위**: F-tris-2 §5 deferred — 3순위 (`pitch.AdaptiveCodebook`, `fcb.Decode`, pitch enhancement β clamp) 의 ITU-T G.729 §3.7 / §3.7.1 / §3.8 / §3.8.1 / §A.3.7 / §A.3.8 line-by-line spec 인용 + Q-format 검증.
**산출물**: 3순위 후보 spec 일치/위반 분류 + 잔여 spec-위반 후보 ranking 갱신 (F-quart-1 §6.2 의 sample 1 |Δ|=2 잔존 원인 후보 좁힘).
**준수**: ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §3.7 / §3.7.1 / §3.8 / §3.8.1 / §A.3.7 / §A.3.8 만 인용. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조** (E1 무발동).

본 task 는 *코드 읽기 + spec 인용 + Q-format 검증표* 만 수행. Production 코드 0-수정, 신규 test 0건.

---

## 0. Working tree 상태 + escape hatch 평가

### 0.1 Working tree 상태 (task 시작 시점)

| 경로 | 상태 | F-quart-2 변경? |
|------|------|---|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix int64 누산 보존 | No |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (untracked) — F-bis-1/F-tris-1 진단 하니스 보존 | No |
| `internal/decoder/stagef_quart_diagnostic_test.go` | committed at 694e9c2 (F-quart-1) | No |
| `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-2-report.md` | **new (committed by 본 task)** — 본 보고서 | Yes (신규) |

`git diff --stat -- internal/` 출력 (task 시작·종료 동일):

```
internal/lsp/lsp_lp.go | 108 ++++++++++++++++++++++++++++++++++++++++++++---------------------------------------------
1 file changed, 54 insertions(+), 54 deletions(-)
```

**Production 코드 (`internal/lsp/lsp_lp.go` 의 F-bis-1 P fix 외) 0 라인 변경.** `internal/pitch/`, `internal/fcb/`, `internal/gain/`, `internal/synth/`, `internal/postfilter/`, `internal/pcm/`, `internal/decoder/decode.go`, `internal/decoder/subframe.go` 모두 미변경. 본 보고서는 내부 spec § + production 코드 인용 + Q-format hand-trace 만으로 구성된 1-파일 commit.

### 0.2 Escape hatch 평가

본 task 의 escape hatch 정의 (instruction 본문):

| 해치 | 발동 조건 | 본 task 발동? |
|------|---------|---|
| **E1** | 외부 G.729 구현 (참조 C / bcg729 / Sipro / FFmpeg) 인용 흔적 | **No** — 본 보고서 인용 = ITU-T G.729 (06/2012) PDF + 본 codebase 의 self-citing docstring (이미 spec § 인용으로 작성됨) 만. 외부 구현 미참조. |
| **E2** | Production 코드 변경 흔적 (lsp_lp.go F-bis-1 P fix 외) | **No** — §0.1 working tree 표 참조. `internal/pitch`, `internal/fcb` 모두 미변경. |

추가로, 본 task 는 **강압-적합 (forced-fit) 금지** 원칙 하에 spec 식 / 표 / 식별자 가 production 과 *비-trivial* 어긋날 경우 그대로 명시한다. §1, §2, §3 의 결론은 모두 spec 일치 여부를 verbatim 비교 후 도출.

---

## 1. `pitch.AdaptiveCodebook` (§3.7 / §3.7.1 / §A.3.7)

### 1.1 spec 인용

ITU-T G.729 (06/2012) **§3.7 (Pitch period decoding)** 는 디코더 측 pitch lag 을 정수부 `tInt` + 분수부 `tFrac ∈ {-1/3, 0, +1/3}` 로 분해함을 규정한다 (식 33·34, PDF p.13-14). 분수 부분 디코딩 후 두 분기:

- **정수 lag (tFrac = 0)**: adaptive codebook vector `v(n)` 은 `pastExc` 의 단순 시간 시프트 — `v(n) = u(n − tInt)`, `n = 0,...,39`.
- **분수 lag (tFrac ≠ 0)**: §3.7.1 (PDF p.15) 식 (40) 의 1/3-sample FIR interpolation:

> *식 (40) (verbatim, ITU-T G.729 (06/2012) §3.7.1)*:
>
> v(n) = Σ_{i=0..9} u(n − k − i)·b30(t + 3i)
>      + Σ_{i=0..9} u(n − k + 1 + i)·b30(3 − t + 3i)        n = 0,...,39
>
> 여기서 (k, t) 는 (tInt, tFrac) 으로부터 다음과 같이 도출된다:
>
> | tFrac | k | t |
> |-------|---|---|
> | 0 | tInt | 0 |
> | +1/3 | tInt | 1 |
> | −1/3 | tInt − 1 | 2 |

본 식은 `internal/pitch/adaptive.go:14-26` 의 docstring 에 verbatim 으로 인용되어 있다.

**§A.3.7 (G.729A short-pitch handling, PDF p.42)**: 인코더 fast-search 의 일환으로 pitch lag `tInt < 40` (즉 한 subframe 길이 미만) 인 경우 adaptive codebook 의 *periodicity extension* 을 적용:

- `n ∈ [0, tInt)`: `v(n)` = past excitation 으로부터 정수/분수 lag 으로 fetch.
- `n ∈ [tInt, 40)`: `v(n) = v(n − tInt)` (subframe 내부 자기-반복).

이 처리는 G.729 main body 의 식 (40) 을 short-pitch 영역으로 확장하는 데이터 의존 분기이며, **디코더는 인코더가 사용한 v[] 와 비트-정확 일치해야 하므로 동일 periodicity extension 을 수행해야 한다**. (본 codebase 의 `adaptive.go:35-37` docstring 이 이 의무를 명시.)

### 1.2 line-by-line 검증표

`internal/pitch/adaptive.go:39-67` (`AdaptiveCodebook` 본체) 와 spec 식 (40) + §A.3.7 short-pitch periodicity 의 일치 여부:

| 분기 | 조건 | spec 출처 | production 코드 (file:line) | 일치? |
|------|------|-----------|-----------------------------|---|
| (A) Fast path | `tFrac == 0 ∧ tInt ≥ 40` | §3.7 정수 lag 단순 복사 (`v(n) = u(n − tInt)`) | `adaptive.go:40-46` | ✅ — `base = len(pastExc) - tInt`; `v[n] = pastExc[base+n]` for n=0..39. `pastExc[len-1]` = `u(-1)` convention (§0.docstring) → `pastExc[base+n]` = `u(n − tInt)` 정확. |
| (B) FIR interpolation | `tFrac ≠ 0 ∧ tInt ≥ 40` | §3.7.1 식 (40) | `adaptive.go:48-50` → `firInterpolate` (`adaptive.go:72-106`) | ✅ — §1.2.B 본문 분석 참조 (k/t 매핑, b30 인덱싱, 두 sigma 합 모두 식 (40) 와 직접 정합). |
| (C) Short pitch tFrac=0 | `tFrac == 0 ∧ tInt < 40` | §A.3.7 short-pitch periodicity (정수 lag) | `adaptive.go:56-60` (n=0..tInt-1) + `adaptive.go:64-66` (n=tInt..39) | ✅ — 본 분기가 frame 0 sf0 stimulus 의 진입점. §1.3 hand-trace 참조. |
| (D) Short pitch tFrac≠0 | `tFrac ≠ 0 ∧ tInt < 40` | §A.3.7 + 식 (40) interpolation | `adaptive.go:62-63` (firInterpolate, 0..tInt) + `adaptive.go:64-66` (periodicity, tInt..39) | ✅ — interpolation 으로 v[0..tInt-1] 생성 후 동일 periodicity extension. |

#### (B) FIR interpolation 세부 검증 — `firInterpolate` (adaptive.go:72-106)

식 (40) 의 두 sigma:
- 첫 sigma: i=0..9 에 대해 `u(n − k − i) · b30(t + 3i)`.
- 둘째 sigma: i=0..9 에 대해 `u(n − k + 1 + i) · b30(3 − t + 3i)`.

production 매핑 (`adaptive.go:74-80`):

```go
if tFrac == 1 {
    k = tInt
    posPhase, negPhase = 1, 2  // posPhase=t,    negPhase=3-t
} else {  // tFrac == -1
    k = tInt - 1
    posPhase, negPhase = 2, 1  // posPhase=t=2,  negPhase=3-t=1
}
```

| tFrac | spec k | code k | spec t | code posPhase (=t) | spec 3-t | code negPhase (=3-t) | 일치 |
|-------|--------|--------|--------|--------------------|----------|----------------------|------|
| +1/3 | tInt | tInt | 1 | 1 | 2 | 2 | ✅ |
| −1/3 | tInt − 1 | tInt − 1 | 2 | 2 | 1 | 1 | ✅ |

본문 loop (`adaptive.go:85-103`):

```go
backIdx := base + n - i        // = len(pastExc) - k + n - i  →  index of u(n - k - i)
fwdIdx  := base + n + 1 + i    // = len(pastExc) - k + n + 1 + i  →  index of u(n - k + 1 + i)
acc = LMac(acc, fir[posPhase + 3*i], back)  // b30(t + 3i) · u(n - k - i)
acc = LMac(acc, fir[negPhase + 3*i], fwd)   // b30(3 - t + 3i) · u(n - k + 1 + i)
```

식 (40) 두 sigma 와 directly correspondence. b30 인덱싱은 `tables.PitchInterpFIR[t + 3i]` (= `b30(t+3i)`) 으로 `internal/tables/pitch_interp.go:27-32` docstring 의 spec convention 과 일치.

OOB 처리 (`adaptive.go:88-100`): `backIdx < 0 || backIdx ≥ N` 일 때 0-substitute. docstring 이 인용한 §3.7.1 의 "samples u(0+) have not yet been computed and are treated as 0" 와 일치 (디코더에서 u(0+)=current subframe 미생성 영역).

**(B) 분기 spec 일치 ✅. b30 계수표 자체의 ITU 정확성 검증은 본 사이클 범위 외 (`tables/pitch_interp_test.go` 가 길이 31 만 검증 — 값 검증은 Phase 1g 에 배정된 것으로 docstring 표기 — `signs.go:11` 의 "bit-exact verification ... happens in Phase 1g" 참조).**

#### (C) Short pitch tFrac=0 세부 검증

```go
if tFrac == 0 {
    base := len(pastExc) - tInt
    for n := 0; n < tInt; n++ {
        v[n] = pastExc[base+n]    // v(n) = u(n - tInt) for n in [0, tInt)
    }
}
for n := tInt; n < 40; n++ {
    v[n] = v[n-tInt]              // §A.3.7 periodicity extension
}
```

§3.7 정수 lag fetch + §A.3.7 self-replication. ✅ spec 일치.

### 1.3 frame 0 sf0 hand-trace (tInt=20, tFrac=0, pastExc=0)

ALGTHM frame 0 sf0 stimulus:
- `tInt = 20, tFrac = 0` → 분기 (C) Short pitch tFrac=0.
- `pastExc[*] = 0` (decoder 초기화 직후, F-tris-1 §3 진단 하니스 측정).

Step 1: `base := len(pastExc) - 20`. `pastExc[base+n] = 0` for n=0..19 → `v[0..19] = 0`.
Step 2: `v[n] = v[n-20] = 0` for n=20..39 → `v[20..39] = 0`.

→ **`v[0..39] = 0` 전체.** F-tris-1 진단 하니스 출력 `gp·v = 0` (subframe contribution from pitch) 와 정합.

Spec 식 (40) (정수 lag 분기): `v(n) = u(n − 20) = pastExc[len-20+n] = 0`. spec 동일 출력. ✅

### 1.4 결론 (frame 0 sf0 한정)

`pitch.AdaptiveCodebook` 은 §3.7 / §3.7.1 / §A.3.7 와 line-by-line 일치 — **frame 0 sf0 stimulus 한정 결함 위치 아님**. v=0 trivial 분기에서 본 단계의 sample 0..7 PST/2 정렬 영향은 0 (gp·v 항이 식 (66) `u(n) = gp·v(n) + gc·c(n)` 에서 0 으로 사라짐).

본 §의 결과는 *frame 0 sf0 한정*. 다른 frame (특히 frame 1+ 의 voiced sf, tInt ≥ 40 의 분기 (A)/(B)) 에서는 `tables.PitchInterpFIR` 의 b30 계수 ITU 정확성이 별도 검증 대상이지만, 본 사이클 범위 외 (Phase 1g).

---

## 2. `fcb.Decode` (§3.8 / §4.1.5)

### 2.1 spec 인용

ITU-T G.729 (06/2012) **§3.8 (Algebraic codebook structure and search, PDF p.17-19)** 는 4-pulse interleaved single-pulse permutation (ISPP) 알고리즘을 규정.

- 식 (45) (sign convention): 각 pulse 의 부호는 1-bit 으로 인코드되며, sign_bit = 1 → +1, sign_bit = 0 → −1 의 ±단위 펄스 진폭.
- 표 7 (PDF p.18, 4 track 의 위치 후보): 40-sample subframe 을 5-sample stride 로 4 track 에 분할.

  | track | pulse | 위치 후보 (8개) | residue mod 5 |
  |-------|-------|----------------|---------------|
  | 0 | i₀ | 0, 5, 10, 15, 20, 25, 30, 35 | 0 |
  | 1 | i₁ | 1, 6, 11, 16, 21, 26, 31, 36 | 1 |
  | 2 | i₂ | 2, 7, 12, 17, 22, 27, 32, 37 | 2 |
  | 3 | i₃ | 3 또는 4, 8 또는 9, ..., 38 또는 39 | 3 또는 4 (jx 비트로 선택) |

  pulse 위치 인코딩: 13-bit 코드 = `i₀(3) | i₁(3) | i₂(3) | jx(1) | i₃(3)` (MSB-first).

- 식 (46) (PDF p.19, pitch enhancement / pre-emphasis filter, **§3.8 본문**):

  > c'(n) = c(n) + β·c'(n − T),     n = T,...,39

- 식 (47): β = ĝ_p^(m-1) (이전 subframe 디코드 pitch gain), clamp 범위 0.2 ≤ β ≤ 0.8.
- 식 (48): T < 40 일 때만 enhancement 적용 (T ≥ 40 이면 c[] 은 변경 없음).

§A.3.8 (G.729A algebraic codebook): G.729 main body 와 *동일 4-pulse 구조* — focused-search 알고리즘만 단순화 (인코더 측). 디코더는 영향 없음.

**식 번호 표기 주의**: 본 plan (line 320-323) 는 식 (61)/(62)/(64), 본 codebase docstring (`enhance.go:6`, `signs.go:6`) 는 식 (45)/(46)/(47) 을 인용. ITU-T G.729 (06/2012) PDF 를 단일 출처로 보면 **§3.8 의 식 번호는 (45)-(48)** 가 정합 (PDF p.17-19 의 표 7 인접 식 번호). plan 의 (61)/(62)/(64) 는 plan 작성 시 PDF 다른 절 번호 (예: 인코더 측 §3.10 또는 다른 edition) 로 작성된 것으로 보임 — *식 내용 자체* 는 동일하므로 본 보고서는 **§3.8 식 (45)/(46)/(47)/(48)** 표기를 사용 (production docstring 과 일치). 이는 편집상 표기 차이일 뿐 결함 후보 아님.

### 2.2 line-by-line 검증표

| 단계 | spec 출처 | production 파일·함수 | 검증 결과 |
|------|-----------|--------------------|---------|
| 1. 위치 디코딩 | §3.8 표 7 | `internal/fcb/positions.go::decodePositions` | ✅ — §2.2.1 |
| 2. sign 적용 | §3.8 식 (45) | `internal/fcb/signs.go::placePulses` | ✅ — §2.2.2 |
| 3. pitch enhancement | §3.8 식 (46) + (48) | `internal/fcb/enhance.go::applyPitchEnhancement` | ✅ — §2.2.3 |

#### 2.2.1 `decodePositions` (positions.go:15-27)

```go
func decodePositions(code uint16) [4]int {
    i0 := int((code >> 10) & 0x07)   // bits 12..10
    i1 := int((code >> 7) & 0x07)    // bits  9..7
    i2 := int((code >> 4) & 0x07)    // bits  6..4
    jx := int((code >> 3) & 0x01)    // bit       3
    i3 := int(code & 0x07)           // bits  2..0
    return [4]int{
        5 * i0,                       // track 0: 0,5,...,35
        5*i1 + 1,                     // track 1: 1,6,...,36
        5*i2 + 2,                     // track 2: 2,7,...,37
        5*i3 + 3 + jx,                // track 3: 3,4,8,9,...,38,39
    }
}
```

표 7 의 4 track stride-5 후보 집합과 비트 layout 모두 정합:
- track 0..2 의 8 위치 = `5·iₖ + k` for iₖ ∈ [0,7], k ∈ {0,1,2} ✅
- track 3 의 16 위치 = `5·i₃ + 3 + jx` for i₃ ∈ [0,7], jx ∈ {0,1} ✅
- 13-bit total = 3+3+3+1+3 ✅

✅ §3.8 표 7 line-by-line 일치.

#### 2.2.2 `placePulses` (signs.go:17-28)

```go
for i := range c { c[i] = 0 }                        // zero-clear (식 (46) 사전 조건)
for i := 0; i < 4; i++ {
    if (signs >> (3 - uint(i))) & 1 == 1 {
        c[positions[i]] = PulseAmplitude              // +8192 (Q13 = +1.0)
    } else {
        c[positions[i]] = -PulseAmplitude             // -8192
    }
}
```

식 (45) sign_bit→±1 매핑과 정합. `PulseAmplitude = 8192` (Q13 = +1.0, `types.go:14`).

비트 순서 convention (sign_bit_0 = MSB) 은 codebase docstring (`signs.go:11`) 이 명시한 대로 "encoder convention not pinned by the spec" — Phase 1g 의 ITU 비트-정확 vector 검증으로 확정 예정. 본 사이클 (frame 0 sf0 stimulus 한정) 에서는 결함 후보 아님.

✅ §3.8 식 (45) 산술 / Q-format 일치.

#### 2.2.3 `applyPitchEnhancement` (enhance.go:40-54)

```go
func applyPitchEnhancement(c *[40]int16, t int, betaQ14 int16) {
    if t < 1 || t >= 40 { return }              // 식 (48): T < 40 만 적용
    if betaQ14 == 0 { return }                  // β=0 → no-op
    bQ14 := fixed.Word16(betaQ14)
    for n := t; n < 40; n++ {                   // 식 (46): n = T..39
        prod := fixed.LMult(bQ14, fixed.Word16(c[n-t]))   // Q14 × Q13 → Q28 (LMult doubles to Q29? — 아래 표)
        prod = fixed.LShl(prod, 1)
        delta := fixed.Round(prod)
        c[n] = int16(fixed.Add(fixed.Word16(c[n]), delta))
    }
}
```

식 (46) `c'(n) = c(n) + β·c'(n − T), n = T..39` 의 in-place IIR 구현. 식 (48) `T < 40` 가드 (`t >= 40` 조기 return) ✅. `t < 1` 가드는 (i) Word16 음수 불가 (T ≥ 0) + (ii) T=0 시 무한 self-loop 방지의 sanity check — spec 함축 (T=0 은 pitch lag 의미상 비정상).

**Q-format 사슬** (docstring 인용):

| 단계 | 입력 | 연산 | 출력 |
|------|------|------|------|
| `LMult(βQ14, c[n-t])` | Q14 × Q13 | (a*b)<<1 | Q14+Q13+1 = **Q28** |
| `LShl(prod, 1)` | Q28 | <<1 | **Q29** |
| `Round(prod)` | Q29 | >>16 (with rounding) | Q29-16 = **Q13** |
| `Add(c[n], delta)` | Q13 + Q13 | sat add | **Q13** |

→ `c[]` Q13 보존, `β·c(n−T)` 항이 Q13 으로 정확히 더해짐. ✅ Q-format 일치.

**In-place cascading** (docstring `enhance.go:38-39` 명시): `c[n-t]` for n ∈ [t..39] 는 *post-filtered* 값을 읽어 IIR 동작을 구현. 식 (46) 우변 `c'(n−T)` (post-filtered) 와 일치 — 만약 `c[n-T]` 의 *pre-filter* 값을 읽었다면 FIR 동작이 되어 spec 위반. ✅ spec 일치.

### 2.3 frame 0 sf0 hand-trace (C1=0, S1=15)

ALGTHM frame 0 sf0 stimulus (F-tris-1 진단 하니스 측정):
- `idx.Positions = C1 = 0` (13-bit 0).
- `idx.Signs = S1 = 15` (4-bit 0b1111 → 4 pulse 모두 + sign).
- pitch lag `t = tInt = 20`.
- `betaQ14 = ClampPitchGainForEnhancement(d.prevGpQ14)` 에서 `d.prevGpQ14 = 0` (decoder zero-value) → §3 참조 → β = `betaLowerQ14 = 3277` (Q14 = 0.2).

Step 1 — `decodePositions(0)`: i0=i1=i2=jx=i3=0 → positions = [5·0, 5·0+1, 5·0+2, 5·0+3+0] = **[0, 1, 2, 3]**.

Step 2 — `placePulses([0,1,2,3], 0b1111, c)`: c 전체 zero-clear, 이후 i=0..3 모두 sign_bit=1 → `c[0]=c[1]=c[2]=c[3]=+8192`. → **c[0..3] = +8192, c[4..39] = 0.**

Step 3 — `applyPitchEnhancement(c, t=20, betaQ14=3277)`:
- 가드: t=20 ∈ [1, 40), betaQ14 ≠ 0 → loop 진입.
- n=20: c[n-20]=c[0]=8192. `LMult(3277, 8192)` = 3277·8192·2 = 53,673,984 (Q28 doubled표기). `LShl(_, 1)` → 107,347,968 (Q29). `Round` → round(107,347,968 / 2^16) ≈ **1638** (Q13). c[20] = 0 + 1638 = **1638**.
- n=21: c[1]=8192 → 동일 → c[21] = **1638**.
- n=22: c[2]=8192 → c[22] = **1638**.
- n=23: c[3]=8192 → c[23] = **1638**.
- n=24..39: c[n-20] = c[4..19] = 0 → delta = 0 → c[24..39] 변경 없음.

(In-place cascading: n=24 시 c[n-20]=c[4]=0 으로 pre/post 동일 — frame 0 sf0 에서는 cascading 효과가 발현되지 않음. 첫 IIR feedback 은 n ≥ 40 (다음 subframe 또는 다음 frame) 에 발현.)

→ **c[0..3] = +8192, c[4..19] = 0, c[20..23] = +1638, c[24..39] = 0.**

float 환산: c_real(0..3) = +1.0, c_real(20..23) = +0.200 (= β = 0.2 의 직접 가시화 — pulse 진폭 1.0 × β 0.2 = 0.2).

본 hand-trace 는 F-tris-2 §5.2 의 정성 추정 ("c[0..3] 중 적어도 일부에 +8192 pulse 존재") 을 정량 확정.

### 2.4 결론

`fcb.Decode` 의 3 단계 (decodePositions / placePulses / applyPitchEnhancement) 모두 §3.8 식 (45)/(46)/(47)/(48) 및 표 7 와 line-by-line 일치. **frame 0 sf0 stimulus 한정 결함 위치 아님.**

c[] 출력은 frame 0 sf0 의 g_c·c(n) 항 (식 66) 의 비-trivial 입력. F-quart-1 §6.2 의 sample 1 잔여 |Δ|=2 가 c[] 에서 유래할 가능성은 본 §의 spec 일치 결론으로 *낮음* — 다만 c[] 자체는 spec-correct 이라도 g_c (gain reconstruction) 가 변하면 g_c·c 도 변함 → sample 1 잔여 결함의 후보는 §3 (β clamp) 또는 §3.9 비선형 체인 (gain prediction, log2/pow2) 으로 좁혀진다.

---

## 3. Pitch enhancement β clamp (§3.8 식 47, **plan 의 §3.8.1**)

### 3.1 spec 인용

§3.8 식 (47) (PDF p.19):

> β = ĝ_p^(m-1),     0.2 ≤ β ≤ 0.8

여기서 ĝ_p^(m-1) 은 *이전 subframe 의 디코드 pitch gain* (= 본 codebase 의 `prevGpQ14`). clamp 범위 [0.2, 0.8] 는 hard limit.

**식 번호 표기 주의**: plan 은 "§3.8.1" 로 표기, ITU PDF 본문은 "§3.8 식 (47)" — 동일 내용, 절 세부 번호의 표기 차이. 결함 후보 아님.

### 3.2 `ClampPitchGainForEnhancement` 값 검증 (enhance.go:5-24)

```go
const (
    betaLowerQ14 = 3277   // round(0.2 · 2^14) = round(3276.8)
    betaUpperQ14 = 13107  // round(0.8 · 2^14) = round(13107.2)
)

func ClampPitchGainForEnhancement(gpPrevQ14 int16) int16 {
    if gpPrevQ14 < betaLowerQ14 { return betaLowerQ14 }
    if gpPrevQ14 > betaUpperQ14 { return betaUpperQ14 }
    return gpPrevQ14
}
```

상수 검증:
- `0.2 × 2^14 = 0.2 × 16384 = 3276.8` → round = **3277** ✅
- `0.8 × 2^14 = 0.8 × 16384 = 13107.2` → round = **13107** ✅

clamp 로직: `< lower` → lower, `> upper` → upper, else identity. 식 (47) 의 `0.2 ≤ β ≤ 0.8` 와 정확히 일치. **음수 입력 (`gpPrevQ14 < 0`) 은 `< 3277` 가드로 lower bound 로 흡수** — spec 은 음수 g_p 에 대한 명시 처리 없으나, g_p 가 spec 상 비음수 (식 (43) `g_p ≥ 0` 함축) 임을 고려하면 안전한 fallback.

✅ §3.8 식 (47) 일치.

### 3.3 frame 0 sf0 prevGpQ14=0 처리

frame 0 sf0 stimulus: `prevGpQ14 = 0` (decoder `Decoder` struct 의 zero-value, 첫 frame 의 첫 subframe 이전에는 디코드된 g_p 이력이 없음).

Clamp: `0 < 3277` → return `3277` (= 0.2 Q14). ✅ spec 의 lower bound 로 정확히 흡수.

**Spec 의 "first subframe initialization" 명시 부재**: §3.8 식 (47) 는 ĝ_p^(m-1) 의 초기값 (m=0 시) 을 명시적으로 규정하지 않음 (PDF p.19). 본 codebase 는 zero-init → clamp 로 lower bound 흡수 — spec 의 *함의* (β 가 항상 [0.2, 0.8] 에 위치) 와 정합. 이는 spec-implied behavior 이며 별도 결함 후보 아님.

**다만 다른 디코더 초기화 컨벤션 (e.g., ĝ_p^(-1) = 0.2 직접 init, 또는 ĝ_p^(-1) = 1.0 클램프 후 → 0.8)** 도 spec 으로 *허용 가능* — 단 clamp 범위 [0.2, 0.8] 안에 있으므로 frame 0 sf0 의 β 출력은 모든 합리적 init 에서 0.2~0.8 사이. zero-init + clamp = 0.2 (lower bound) 는 그 중 *최소 가정* 이며, sample 1 의 |Δ|=2 잔여 결함이 β 초기화 misalign 으로 유발될 가능성이 있음 (본 §의 결과만으로는 결정 불가; F-quart-3 의 §3.9 비선형 체인 검증과 교차 분석 필요).

→ **결함 후보 약함, 그러나 잠재적 sample 1 misalign source 1건 식별** (β init = 0.2 vs spec-허용 다른 init 의 비교는 ITU 비트-스트림 reference 측정으로만 결정 가능 — Phase 1g).

---

## 4. 종합 — 3순위 spec 일치/위반 분류 + 잔여 ranking 갱신

### 4.1 본 task 분류 결과

| 단계 | spec § | production | 분류 | frame 0 sf0 결함 가능성 |
|------|--------|-----------|------|------------------------|
| `pitch.AdaptiveCodebook` (§1) | §3.7 / §3.7.1 / §A.3.7 | `internal/pitch/adaptive.go` | **spec 일치 ✅** | **0 (v=0 trivial)** |
| `fcb.decodePositions` (§2.2.1) | §3.8 표 7 | `internal/fcb/positions.go` | **spec 일치 ✅** | **0 (frame 0 sf0 한정)** |
| `fcb.placePulses` (§2.2.2) | §3.8 식 (45) | `internal/fcb/signs.go` | **spec 일치 ✅** | **낮음 (sign convention 은 Phase 1g 검증)** |
| `fcb.applyPitchEnhancement` (§2.2.3) | §3.8 식 (46) + (48) | `internal/fcb/enhance.go` | **spec 일치 ✅** | **낮음 (Q-format / IIR 정확)** |
| `ClampPitchGainForEnhancement` (§3) | §3.8 식 (47) | `internal/fcb/enhance.go:5-24` | **spec 일치 ✅** | **약함 (β init = 0.2 vs spec-허용 다른 init)** |

→ **3순위 5개 단계 전부 line-by-line spec 일치. 새로운 확정 spec-위반 후보 0건.**

본 §은 F-tris-2 §5 의 deferred 항목을 closure — 3순위 분석은 모두 *결함 위치 부정* 으로 수렴.

### 4.2 잔여 spec-위반 후보 ranking (F-quart-1 §6.2 + 본 task 수렴)

F-quart-1 §6.2 가 식별한 sample 1 잔여 |Δ|=2 의 원인 후보 (§3.9.3 fix 적용 후 잔존):

| 순위 | 후보 | spec § | 본 task 영향 |
|------|------|--------|---|
| **1** (강) | §3.9.4 — gain reconstruction `g_c = γ̂_c · g_c'` 의 Q-format 변환 (Q13 → Q12) rounding direction (F-tris-2 §3 식별) | §3.9.4 | 본 task 영향 없음 — F-quart-3 의 비선형 체인 검증 대상으로 유지. |
| **2** (강) | §3.9.1 / §3.9.2 — gain prediction MA-predictor (`gain.Decoder.pastErrors` FIFO) 의 zero-init vs spec-correct -14dB init | §3.9.1, §3.9.2 | 본 task 영향 없음 — F-quart-3 검증 대상. |
| **3** (약) | §3.8 식 (47) — β init (= ĝ_p^(-1)) 의 zero-init vs spec-허용 다른 init | §3.8 식 (47) | **본 task §3.3 식별** — β init = 0.2 (lower bound) 가 spec-허용 다른 init 과 frame 0 sf0 c[20..23] 값 차이 유발 가능. 단 spec § 자체로는 일치 (clamp 안에 위치). Phase 1g 비트-스트림 reference 측정으로 결정. |
| 부정 | pitch.AdaptiveCodebook | §3.7 | **본 task §1.4** — frame 0 sf0 v=0, 결함 위치 아님. |
| 부정 | fcb.decodePositions / placePulses / applyPitchEnhancement | §3.8 | **본 task §2.4** — line-by-line spec 일치, frame 0 sf0 결함 위치 아님. |

### 4.3 F-quart-3 진입 권고

본 task §4.1 가 3순위 5개 단계 전부 spec 일치로 결정 → 잔여 sample 1 |Δ|=2 의 원인은 **§3.9 비선형 체인 (gain prediction MA + log2/pow2 + reconstruction)** 으로 강하게 좁혀짐.

F-quart-3 진입 시:
1. **§3.9.4 reconstruction Q-format** (F-tris-2 §3 후보) line-by-line.
2. **§3.9.1 / §3.9.2 prediction MA pastErrors init** — `gain.Decoder.pastErrors` zero-value vs spec 의 -14 dB (= -14336 Q10) init 비교. 이미 F-tris-2 §3 가 docstring 으로 예고.
3. **§4.2.1 / §4.2.2 postfilter / hpFilter** — F-quart-1 §6.3 가 권고한 비선형 체인 잔여 검증.

위 3 항목을 production 0-수정 + spec 인용 line-by-line 으로 수행 → 단일 fix 후보 식별 후 F-quint 사이클 진입.

---

## 5. Escape hatch 종합

| 해치 | 발동 여부 | 근거 |
|------|---------|------|
| **E1** (외부 G.729 구현 인용 흔적) | **무발동** | 인용 출처 = ITU-T G.729 (06/2012) PDF (§3.7 / §3.7.1 / §3.8 / §A.3.7 / §A.3.8) + 본 codebase 의 self-citing docstring 만. 외부 구현 (참조 C / bcg729 / Sipro / FFmpeg) 미참조. |
| **E2** (production 변경 흔적) | **무발동** | §0.1 — `git diff --stat -- internal/` 출력은 F-bis-1 P fix 의 lsp_lp.go 만 (task 시작·종료 동일). `internal/pitch`, `internal/fcb` 모두 미변경. 본 commit 은 보고서 1 파일. |

강압-적합 (forced-fit) 회피: §1.4 / §2.4 / §3.3 모두 spec 인용 → production 비교 → 결론의 순서로 작성. spec 식 번호 표기 차이 (§2.1 / §3.1) 는 결함 후보로 격상하지 않고 표기 주의로 처리 — 식 *내용* 이 일치하기 때문.

---

## 부록 — 본 task 시점 working tree 상태 (commit 후)

```
$ git diff --stat -- internal/
 internal/lsp/lsp_lp.go | 108 ++++++++++++++++++++++++++++++++++++++++++++---------------------------------------------
 1 file changed, 54 insertions(+), 54 deletions(-)
```

- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) — F-bis-1/F-tris-1 진단 하니스 보존 (F-quart-3 baseline).
- `internal/decoder/stagef_quart_diagnostic_test.go` — F-quart-1 commit (694e9c2) 에 포함.
- `internal/lsp/lsp_lp.go` (modified, uncommitted) — F-bis-1 P fix int64 누산 보존 (F-quart-3 baseline).
- 본 보고서 `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-2-report.md` 는 본 commit 의 단일 staged 파일.
