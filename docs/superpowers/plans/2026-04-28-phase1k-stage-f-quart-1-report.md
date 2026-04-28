# Phase 1k Stage F-quart-1 진단 노트 — GainImap inverse-map 단일-fix 정렬도 측정

**작성일**: 2026-04-28
**범위**: F-tris-2 §4 가 식별한 *확정 spec-위반 후보* (`gain.decodeVQ` 의 §3.9.3 GainImap1/GainImap2 inverse-map 누락) 가 ALGTHM frame 0 sf0 sample 0..7 PST/2 정렬에 대해 *충분조건* 인지 *부분조건* 인지를 production 코드 0-수정으로 평행 시뮬레이션해 측정한다.
**산출물**: 단일 fix 충분/부분/악화 분류 (S1/S2/S3) + F-quart-2/F-quart-3 진입 가이드.
**준수**: ITU-T G.729 (06/2012) PDF (`docs/superpowers/specs/itu/G729E.pdf`) §3.9 / §3.9.3 / §4.1.6 / §4.2.4 만 인용. 외부 G.729 구현 (ITU 참조 C, bcg729, Sipro Lab, FFmpeg) **0건 참조** (E4 무발동).

---

## 0. Working tree 상태 + escape hatch 평가

### 0.1 Working tree 상태

| 경로 | 상태 | F-quart-1 변경? |
|------|------|---|
| `internal/lsp/lsp_lp.go` | modified (uncommitted) — F-bis-1 P fix int64 누산 보존 | No |
| `internal/decoder/stagef_bis_diagnostic_test.go` | new (uncommitted) — F-bis-1/F-tris-1 진단 하니스 보존 | No |
| `internal/decoder/stagef_quart_diagnostic_test.go` | **new (committed by 본 task)** — F-quart-1 진단 하니스 | Yes (신규) |
| `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quart-1-report.md` | **new (committed by 본 task)** — 본 보고서 | Yes (신규) |

`git diff --stat -- internal/` 출력 (task 시작·종료):

```
internal/lsp/lsp_lp.go | 108 ++++++++++++++++++++++++++++++++++++++++++++---------------------------------------------
1 file changed, 54 insertions(+), 54 deletions(-)
```

Production 코드 (`internal/lsp/lsp_lp.go` 의 F-bis-1 P fix 외) **0 라인** 변경. `internal/synth`, `internal/postfilter`, `internal/pcm`, `internal/gain`, `internal/fcb`, `internal/pitch`, `internal/decoder/decode.go`, `internal/decoder/subframe.go` 모두 미변경.

### 0.2 Escape hatch 평가

| 해치 | 발동 조건 | 본 task 발동? |
|------|---------|---|
| **E1** | 모든 후보가 spec § 인용 line-by-line 일치 (= 결함 0건) | **No** — F-tris-2 §4 가 `decodeVQ` GainImap 누락을 *확정 spec-위반* 으로 식별; 본 task 는 그 fix 의 충분성을 측정. |
| **E2** | 단일 fix 적용으로 sample 0..7 의 |Δ|≤1 LSB 정렬 *전부* 회복 | **Partial** — 8 sample 중 7 sample 회복 (sample 1 은 |Δ|=2). E2 *완전 발동 X*. |
| **E3** | 단일 fix 가 sample 0..7 정렬도를 *악화* | **No** — Branch B 정렬도가 Branch A 보다 *향상* (40-sample matches: 34→39). |
| **E4** | 외부 G.729 구현 1건이라도 인용/대조에 사용 | **No** — ITU-T G.729 (06/2012) PDF 단일 출처. |

E2 *부분-발동* 만 일치 → **시나리오 (S2)** 분류 (§5 참조). F-quart-2 / F-quart-3 진행 필수.

---

## 1. §3.9.3 spec 인용 + production docstring 모순 명시

### 1.1 ITU-T G.729 (06/2012) §3.9.3 verbatim 인용 (PDF p.22)

> "To reduce the impact of single bit errors, the GA and GB indices are reordered before transmission. The mapping tables are given in Annex C/D."

§3.9 (gain quantization, PDF p.21-22) 는 인코더가 conjugate-structure two-stage VQ 의 best entry index 를 reorder 한 GA/GB 를 비트스트림에 적재함을 규정. §4.2.4 (디코더 측 gain VQ, PDF p.27) 는 비트스트림에서 추출한 GA/GB 에 대해 §3.9 의 reorder 의 *역사상* 을 적용해 GBK1/GBK2 의 물리 entry 를 인덱싱해야 함을 함축한다 (§3.9.3 의 reorder 가 채널 비트 순서에만 적용되므로 디코더는 그 역사상으로 entry index 를 회복).

### 1.2 Production docstring 의 spec-위반 정당화 (`internal/gain/vq.go:14-17`)

```go
// The stages are summed component-wise with Word16 saturation.  The
// codebooks are indexed directly by the received bits (GA, GB); the
// optional reorder tables (Map/Imap) live in tables for the encoder
// search routine and play no role at the decoder.
```

이 docstring 은 **§3.9.3 와 정면 모순**:

- §3.9.3: "the GA and GB indices are reordered before transmission" → 디코더는 inverse map (`GainImap1`, `GainImap2`) 적용 후 GBK 인덱싱 의무.
- Production docstring: "play no role at the decoder" → inverse map 미적용 정당화.

`internal/tables/gain_gbk1.go:36-44` 와 `internal/tables/gain_gbk2.go:32-38` 의 `GainImap1`/`GainImap2` 정의 자체는 spec-correct 이며 그 docstring 은 디코더 의무를 정확히 명시:

```
// GainImap1 is the inverse of GainMap1: given the transmitted GA
// bit pattern, the decoder looks up
// `entry = GainGBK1[GainImap1[GA]]`.
```

따라서 `internal/gain/vq.go` 의 docstring 은 **spec-위반 자기-정당화** — F-quint 사이클의 fix 대상 1순위 (코드 fix + docstring 수정 동반).

---

## 2. F-quart-1 진단 test 설명 (production 코드 0-수정 평행 시뮬레이션)

### 2.1 평행 시뮬레이션 전략

본 task 는 production `decodeVQ` 코드 수정 0 으로 §3.9.3 spec-fix 분기를 시뮬레이션하기 위해 **test 코드 자체에서 sf0 GA1/GB1 을 GainImap1[GA1]/GainImap2[GB1] 로 변환한 새 `gain.Indices` 를 만들어 production `gain.Decoder.Decode` 에 전달**한다.

- Production `decodeVQ` 는 받은 GA/GB 로 `GBK[bits]` 직접 인덱싱 (§3.9.3 위반).
- Test 가 미리 inverse map 한 GA'/GB' 를 넘기므로, 같은 production 코드 경로지만 *결과적으로* `GBK[GainImap[bits]]` (= §3.9.3 spec-correct) 와 동일.

### 2.2 두 분기 instance 분리 의무

`gain.Decoder.Decode` 는 unexported FIFO state (`pastErrors`, `prevGpQ14`) 를 갖고, decoder 체인의 `synth.Synthesizer.pastSynth` / `postfilter.Postfilter.agcGainPrev` / `decoder.Decoder.pastExc`/`hpX`/`hpY` 도 매 호출 갱신된다. 두 분기를 비교하려면 **별도 `decoder.Decoder` instance 2 개** 필요. F-quart-1 은 frame 0 sf0 만 측정하므로 두 instance 모두 zero-value (= §4.3 초기화) 에서 출발 → state 발산 없이 정확한 1-호출 비교 보장.

### 2.3 4 boundary 측정점

`internal/decoder/subframe.go:21-49` 의 파이프라인 그대로:

| Boundary | API | 도메인 |
|----------|-----|---|
| ① synth.Filter | `synth.Synthesizer.Filter(sfA, &u, &s)` | Pre-postfilter PCM |
| ② postfilter.Filter | `postfilter.Postfilter.Filter(sfA, tInt, &s, &sPf)` | Post-§4.2.1 PCM |
| ③ hpFilter | `decoder.Decoder.hpFilter(&sPf, hpOut[:])` | Post-§4.2.2 HP-filtered PCM |
| ④ pcm.ScaleUpSat | `pcm.ScaleUpSat(hpOut[:], scaled[:])` | PST 도메인 (×2 saturating) |

PST 비교 도메인은 ③ (hpFilter, PST/2 직접 비교) 와 ④ (pcm.ScaleUpSat, PST 직접 비교) 둘 다 측정.

### 2.4 sanity check 결과 (절대 제약 §4)

`stagef_quart_diagnostic_test.go` 는 Branch A 의 `synth.Filter` sample 0..7 가 F-tris-1 baseline `[2 3 4 4 3 2 1 1]` 와 일치하는지 *test 본체에서* `t.Fatalf` 검증. 본 실행 결과 일치 확인:

```
Branch A sanity check OK: synth.Filter[0..7] == F-tris-1 baseline [2 3 4 4 3 2 1 1].
```

→ test infra 결함 없음. 측정 결과 신뢰 가능.

---

## 3. Branch A (production) sample 0..7 + 40-sample match count

frame 0 sf0 indices: **GA1=5, GB1=6** (transmitted bits, §3.9.3 reorder 적용된 형태).

| Boundary | sample 0..7 | matches/40 (vs PST/2) |
|----------|-------------|---------------------|
| synth.Filter | `[2 3 4 4 3 2 1 1]` | 33/40 |
| postfilter.Filter | `[2 2 3 4 3 2 1 2]` | 32/40 |
| hpFilter | `[2 2 3 3 2 1 0 1]` | **34/40** |
| pcm.ScaleUpSat | `[4 4 6 6 4 2 0 2]` (PST 도메인) | n/a |

Gain VQ 출력: g_p (Q14) = **13815** (≈ 0.843), γ̂_c (Q12) = **6844** (≈ §3.9.1 mantissa form). γ̂_c (Q13) = 12915 (= 6844 ×2 적분, F-tris-2 §4.4 표 일치).

→ F-tris-1 §0.2 baseline 과 완전 일치 (절대 제약 §4 충족).

---

## 4. Branch B (spec-fix) sample 0..7 + 40-sample match count

`GainImap1[5] = 0`, `GainImap2[6] = 1` → Branch B 호출 indices: **GA=0, GB=1**.

| Boundary | sample 0..7 | matches/40 (vs PST/2) |
|----------|-------------|---------------------|
| synth.Filter | `[0 0 0 0 0 0 0 0]` | 39/40 |
| postfilter.Filter | `[0 0 0 0 0 0 0 0]` | 39/40 |
| hpFilter | `[0 0 0 0 0 0 0 0]` | **39/40** |
| pcm.ScaleUpSat | `[0 0 0 0 0 0 0 0]` (PST 도메인) | n/a |

Gain VQ 출력: g_p (Q14) = **1995** (≈ 0.122), γ̂_c (Q12) = **803** (γ̂_c (Q13) = 1516 ≈ 0.185). F-tris-2 §4.4 표의 spec §3.9.3 분기 행과 *정확히 일치* — inverse-map 의 entry 회복 검증 완료.

g_p · g_c · v + g_c · c 의 결과가 미세 (excitation u sample-0 = 2 이지만 큰 여기 신호와 곱한 후 합성 시 0 으로 saturating-rounded) → synth.Filter 출력 전 sample 0 으로 수렴.

---

## 5. 비교표 + 시나리오 분류

### 5.1 비교표

```
Branch       Boundary           [ 0..  7]                       matches/40
A (prod)     synth.Filter       [ 2  3  4  4  3  2  1  1]       33/40
A (prod)     postfilter.Filter  [ 2  2  3  4  3  2  1  2]       32/40
A (prod)     hpFilter           [ 2  2  3  3  2  1  0  1]       34/40
A (prod)     pcm.ScaleUpSat     [ 4  4  6  6  4  2  0  2]       (PST 도메인)
B (spec)     synth.Filter       [ 0  0  0  0  0  0  0  0]       39/40
B (spec)     postfilter.Filter  [ 0  0  0  0  0  0  0  0]       39/40
B (spec)     hpFilter           [ 0  0  0  0  0  0  0  0]       39/40
B (spec)     pcm.ScaleUpSat     [ 0  0  0  0  0  0  0  0]       (PST 도메인)
```

### 5.2 Branch B hpFilter sample 0..7 |Δ| vs PST/2 (= `[1 2 1 1 0 -1 -1 -1]`)

| n | hpB | PST/2 | Δ | |Δ|≤1? |
|---|-----|------|---|---|
| 0 | 0 | 1 | -1 | ✓ |
| **1** | 0 | **2** | **-2** | **✗** |
| 2 | 0 | 1 | -1 | ✓ |
| 3 | 0 | 1 | -1 | ✓ |
| 4 | 0 | 0 | 0 | ✓ |
| 5 | 0 | -1 | +1 | ✓ |
| 6 | 0 | -1 | +1 | ✓ |
| 7 | 0 | -1 | +1 | ✓ |

8 sample 중 **7 sample 회복**, sample 1 만 |Δ|=2 잔존. 40-sample matches **34→39** (+5 sample 회복).

### 5.3 시나리오 분류: **(S2) 부분조건**

§0.2 의 시나리오 정의 표에 따라:

- **(S1) 충분조건** 미달: sample 0..7 *전부* |Δ|≤1 + 40/40 일치 요구 → sample 1 |Δ|=2, 40-sample 39/40 → 미달.
- **(S3) 악화** 미해당: Branch B (39/40) > Branch A (34/40) → 향상.
- → **(S2) 부분조건**: 단일 fix 가 sample 0..7 의 *대부분* 을 회복하나 sample 1 잔존 + 40-sample 1 sample 잔존.

본 분류는 §0.2 Escape hatch E2 의 *부분-발동* 과 일치한다. F-quart-2/F-quart-3 진행 필수 — 다중 fix 후보 (예: post-§3.9.3 fix 잔여인 sample 1 의 +2 LSB 편차의 *별도 단계 spec-위반*) 존재 가능.

---

## 6. F-quart-2 / F-quart-3 진입 가이드

### 6.1 본 task 가 확정한 사실

1. **§3.9.3 GainImap inverse-map 누락은 frame 0 sf0 sample 0..7 결함의 충분조건 *아님*.** 단일 fix 적용 시:
   - sample 0, 2, 3, 4, 5, 6, 7 회복 (|Δ|≤1).
   - sample 1 은 |Δ|=2 잔존 (= 별도 spec-위반 후보 존재).
2. **§3.9.3 fix 는 정렬도 강력 향상 — 회피 불가.** 40-sample matches 34→39 (+15% 향상). F-quint 사이클의 fix 1순위 confirmed.
3. **§3.9.3 fix 는 frame 0 sf0 PST 자체와의 정렬도도 강력 향상.** Branch A pcm.ScaleUpSat sample 0..7 = `[4 4 6 6 4 2 0 2]` (PST want = `[2 4 3 3 1 -1 -1 -1]`, 큰 |Δ| 다수) vs Branch B = `[0 0 0 0 0 0 0 0]` (대부분 |Δ|≤2). Branch B 가 PST 도메인에서도 명백히 우수.

### 6.2 F-quart-2 (3순위 spec 인용) 가 검증해야 할 항목

§3.9.3 fix 적용 후 sample 1 |Δ|=2 잔존의 원인 후보:

- **§3.9.1 / §3.9.2 — gain prediction MA-predictor (`gain.Decoder.pastErrors` FIFO)**: zero-init `pastErrors` 가 §4.3 spec-correct 인지 (특히 §3.9.2 의 4-th order MA predictor 의 -14 dB 초기값 vs Q-format 0 초기값 비교).
- **§3.9.4 — gain reconstruction `g_c = γ̂_c · g_c'`**: F-tris-2 §3 가 식별한 후보 — γ̂_c 의 Q-format 변환 (Q13 → Q12) 시 rounding direction.
- **§3.7 / §3.7.1 / §A.3.7 — pitch.AdaptiveCodebook**: frame 0 sf0 (tInt=20, tFrac=0, pastExc=0) → v=0 trivial 분기, 결함 위치 *아님* (F-tris-2 §5.4 와 일치).
- **§3.8 / §4.1.5 — fcb.Decode + pitch enhancement**: F-tris-2 §5.5 가 일부 검증; F-quart-2 가 line-by-line 확장.

### 6.3 F-quart-3 (비선형 체인 잔여 결함) 가 검증해야 할 항목

§3.9.3 fix 적용 후의 비선형 체인 (synth → postfilter → hpFilter → ScaleUpSat) 의 §4.2 spec 일치 line-by-line. Branch B 의 sample 1 |Δ|=2 가 비선형 체인 *내부* 의 별도 결함에서 유래하는지 (예: postfilter 의 §4.2.1 단순화 항이 spec 과 어긋난 위치 가중) 검증.

### 6.4 F-quart-4 (종합 + F-quint 권고) 가 통합해야 할 권고

본 task 결과는 다음 ranking 의 강력한 증거:

1. **1 순위 (확정)**: `internal/gain/vq.go:18-22` 의 `decodeVQ` 에 GainImap1/GainImap2 inverse-map 적용 + `vq.go:14-17` docstring 의 §3.9.3 모순 문구 수정. F-quint 사이클 fix 후보 1.
2. **2 순위 (강력 후보)**: F-quart-2/F-quart-3 가 식별할 sample 1 잔여 |Δ|=2 의 원인 (§3.9.4 reconstruction 또는 §3.9.1-§3.9.2 prediction).
3. **3 순위 (검증 대상)**: 비선형 체인 §4.2.1/§4.2.2 의 잔여 spec 위반 (있다면).

단일 fix (1 순위만) 적용 시 PST 정렬도 향상은 강력하나 sample 1 잔존 → F-quint 사이클은 *복합 fix* 로 ranking 권고.

---

## 7. 결론

**시나리오 분류**: **(S2) 부분조건**.

§3.9.3 GainImap inverse-map 단일 fix 는:

- frame 0 sf0 sample 0..7 의 7/8 sample 정렬 회복 (|Δ|≤1).
- 40-sample matches 34→39 향상.
- sample 1 의 |Δ|=2 잔존 (= 별도 단일 spec-위반 후보 1 건 이상 존재).

→ **F-quart-2 진입 권고**: pitch / fcb / pitch enhancement line-by-line spec 인용 + sample 1 잔여 결함 후보 식별.

**E3 미발동** — 단일 fix 가 정렬도를 *악화* 시키지 않음. 단일 fix 부정 ≠ 다중 fix 부정 → F-quart-2/F-quart-3 그대로 진행.

---

## Appendix — 측정 reproduce 명령

```bash
go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
```

본 task 시점 working tree (commit 후):

```
$ git diff --stat -- internal/
 internal/lsp/lsp_lp.go | 108 ++++++++++++++++++++++++++++++++++++++++++++---------------------------------------------
 1 file changed, 54 insertions(+), 54 deletions(-)
```

`internal/decoder/stagef_quart_diagnostic_test.go` 와 본 보고서는 본 commit 에 포함. `internal/decoder/stagef_bis_diagnostic_test.go` 와 `internal/lsp/lsp_lp.go` 의 P fix 는 working tree 미커밋 상태 보존 (F-quart-2/F-quart-3/F-quart-4 가 동일 baseline 위에서 진단 가능하도록).
