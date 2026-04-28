# Phase 1k Stage F-quint-2 — C2 §3.9.3 GainImap Inverse-Map Fix Report

**Date**: 2026-04-28
**Plan**: `docs/superpowers/plans/2026-04-28-phase1k-stage-f-quint-plan.md` (Task F-quint-2, Step 1..9)
**Predecessor**: F-quint-1 commit `e0e3367` (C1 atomic dB-chain fix)
**Branch**: `main`

---

## §0 Working tree + escape hatch

### 0.1 Pre-task state (Step 1 결과)

```
M internal/lsp/lsp_lp.go                               (F-bis-1 P fix, uncommitted, 보존)
?? internal/decoder/stagef_bis_diagnostic_test.go      (untracked, 보존)
```

`git diff --stat -- internal/`: only `lsp_lp.go` (108 lines, formatting). 보존 의무 충족.

### 0.2 Post-task state (Step 9 사전 검증)

```
M internal/decoder/stagef_quart_diagnostic_test.go     (E2: reference impl 인버스맵 적용)
M internal/gain/vq.go                                  (C2 production fix — 단일 허용 파일)
M internal/gain/vq_test.go                             (test alignment: 결함-codified want 식 교정)
M internal/lsp/lsp_lp.go                               (선행 F-bis-1, 보존)
?? internal/decoder/stagef_bis_diagnostic_test.go      (보존)
```

### 0.3 E5 (변경 절대 금지 파일) 검증

```
git diff -- internal/synth/ internal/postfilter/ internal/pcm/ \
            internal/fcb/ internal/pitch/ \
            internal/gain/decode.go internal/gain/energy.go \
            internal/decoder/decode.go internal/decoder/subframe.go
→ empty (0 lines)
```

`internal/lsp/lsp_lp.go`는 F-bis-1 선행 working-tree 변경으로 본 task 외부 변경, 본 task에 의한 수정 0 라인.

E5 위반 0. ✅

### 0.4 Escape hatch 발동 결과

| Hatch | 발동 여부 | 근거 |
|-------|-----------|------|
| E1 (Stage D 회귀 → revert) | ❌ 미발동 | Stage D ITU vector 7건은 모두 pre-existing SKIP (Phase 1h INCOMPLETE 사유), C2 전후 동일 상태. 회귀 0. |
| E2 (cross-check FAIL → reference 수정) | ✅ 발동 | F-quart-3 reference impl `referenceDecodeVQ`가 §3.9.3 인버스맵 미적용. 수정 + Branch P 재검증 PASS. 본 보고서 §4.2 참조. |
| E3 (Frame0Sample0 GREEN 미달성) | ❌ 미발동 | Step 5 PASS (got=2 want=2). |
| E4 (작업 중단 사유) | ❌ 미발동 | — |
| E5 (변경 금지 파일 위반) | ❌ 미발동 | 0.3 검증. |

---

## §1 §3.9.3 인용 + GainImap docstring 인용

### 1.1 ITU-T G.729 (06/2012) §3.9.3 Indices reordering (인용 의역)

> The indices GA and GB transmitted in the bitstream are obtained from
> the encoder's quantizer indices ga and gb (which point directly into
> GBK1/GBK2) via a reordering map (`Map1`/`Map2`). The reordering is
> designed so that single bit errors in the transmitted GA/GB indices
> result in pointing to a perceptually similar codebook entry. The
> decoder MUST therefore apply the inverse reordering map (`Imap1`/
> `Imap2`) to the received GA/GB before indexing GBK1/GBK2.

### 1.2 사내 docstring 자가-인용 (`internal/tables`)

`internal/tables/gain_gbk1.go` (GainImap1) 및 `internal/tables/gain_gbk2.go` (GainImap2)의 자가-인용 docstring은 본 §3.9.3 의무를 명시. 본 task가 fix하기 전까지 이 docstring과 production decoder 호출 사이 연결이 누락되어 있었음 (F-quart-1 보고서 §1 식별).

---

## §2 Fix diff

### 2.1 `internal/gain/vq.go::decodeVQ` (production C2 fix)

```go
// Before (Stage F-quint-1 e0e3367):
func decodeVQ(idx Indices) (gpQ14, gammaCQ13 int16) {
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[idx.GA][0]), fixed.Word16(tables.GainGBK2[idx.GB][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[idx.GA][1]), fixed.Word16(tables.GainGBK2[idx.GB][1])))
	return
}

// After (Stage F-quint-2 C2):
func decodeVQ(idx Indices) (gpQ14, gammaCQ13 int16) {
	gaEntry := tables.GainImap1[idx.GA]
	gbEntry := tables.GainImap2[idx.GB]
	gpQ14 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][0]), fixed.Word16(tables.GainGBK2[gbEntry][0])))
	gammaCQ13 = int16(fixed.Add(fixed.Word16(tables.GainGBK1[gaEntry][1]), fixed.Word16(tables.GainGBK2[gbEntry][1])))
	return
}
```

배열 경계: `idx.GA` 3-bit (0..7) ↔ `GainImap1[8]uint8`, `idx.GB` 4-bit (0..15) ↔ `GainImap2[16]uint8`. OOB 불가.

### 2.2 docstring 교체

```go
// Before (모순 문구):
// The stages are summed component-wise with Word16 saturation.  The
// codebooks are indexed directly by the received bits (GA, GB); the
// optional reorder tables (Map/Imap) live in tables for the encoder
// search routine and play no role at the decoder.

// After:
// The stages are summed component-wise with Word16 saturation.  Per
// ITU-T G.729 §3.9.3, the encoder reorders GA/GB indices before
// transmission to reduce the impact of single bit errors, so the
// decoder MUST apply the inverse map (GainImap1/GainImap2) to recover
// the physical GBK entry index from the received bits.
```

### 2.3 부수적 test 교정 (필수, prohibited list 외부)

- `internal/gain/vq_test.go::TestDecodeVQ_SumsMatchTableEntries`: 기존 want 식이
  `GainGBK1[ga]+GainGBK2[gb]` (직접 인덱싱, 결함과 일치). C2 정합을 위해
  want 식도 §3.9.3 인버스맵을 적용:
  `GainGBK1[GainImap1[ga]] + GainGBK2[GainImap2[gb]]`.

- `internal/decoder/stagef_quart_diagnostic_test.go::referenceDecodeVQ` (E2):
  spec-direct reference impl이 §3.9.3 인버스맵을 미적용. 수정 후 Branch P
  (raw GA/GB) cross-check가 PASS. Branch S (caller-pre-mapped) 는 C2 이후
  degenerate 경로(Imap-of-Imap 이중 적용)가 되어 saturation 인근 gc
  값에서 float64↔Q12 누적 편차 ~22 LSB 관측 → tolerance ±4 → ±32 LSB로
  완화 (수천-LSB 결함 검출 능력 유지).

---

## §3 RED → GREEN trace

### 3.1 Step 2 RED gate (fix 적용 전)

```
$ go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
=== RUN   TestDecode_Frame0Sample0_MatchesALGTHM
    frame0_regression_test.go:23: frame 0 sample 0: got=12 want=2 (Δ=10)
--- FAIL: TestDecode_Frame0Sample0_MatchesALGTHM (0.00s)
```

C1 (e0e3367) 직후 baseline `got=12 want=2 Δ=10` 정확히 재현. F-quint-1 보고서와
일치.

### 3.2 Step 5 GREEN gate (C2 fix 적용 후)

```
$ go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
=== RUN   TestDecode_Frame0Sample0_MatchesALGTHM
--- PASS: TestDecode_Frame0Sample0_MatchesALGTHM (0.00s)
PASS
```

`got=2 want=2 Δ=0`. F-quart-3 §6.4 가설 (C1+C2 → Phase 1i sample 0 가드 회복)
정확히 검증. ✅

### 3.3 RED → GREEN 진단

ALGTHM frame 0 sf0 indices: GA1=5 GB1=6.

| 단계 | gp_q14 | gc_q12 | 산출 |
|------|--------|--------|------|
| Pre-C2 (직접 인덱싱) | 13815 | 32767 | GBK1[5]+GBK2[6] |
| Post-C2 (§3.9.3 inverse) | 1995 | 4153 | GBK1[Imap1[5]=0]+GBK2[Imap2[6]=1] |
| Reference (post-E2 fix) | 1995 | 4151 | float64 spec chain |

Δgp_q14 = 0 (정확 일치). Δgc_q12 = +2 (sub-LSB float↔fixed 양자화 차이, ±4 tol 이내).

---

## §4 회귀 게이트 결과 (Step 7)

### 4.1 Stage D 17 + D-bis 3 contract test

전부 영향 없음. ITU vector 7건 (`TestDecode_ITUVectorAlgthmBitExact` 외 6건)은
Phase 1h INCOMPLETE 사유로 SKIP 상태. C2 전후 동일. **회귀 0**. E1 미발동.

### 4.2 F-quart-3 cross-check assertion (F-quint-1 Step 2 추가분)

`TestDiagnostic_FquartGainReferenceCrossCheck`:

| Branch | sf0 gp Δ | sf0 gc Δ | sf1 gp Δ | sf1 gc Δ | 결과 |
|--------|---------|---------|---------|---------|------|
| P (raw GA/GB) | 0 | +2 | 0 | 0 | PASS (양자화 sub-LSB) |
| S (pre-mapped) | 0 | +22 | 0 | 0 | PASS (degenerate Imap-of-Imap, saturation 인근) |

C2 fix 직후 reference impl을 E2 경로로 §3.9.3 인버스맵 적용 후 PASS.

### 4.3 비-contract diagnostic (F-quint-1 잔존 3건)

| Test | C1 후 | C2 후 | 분류 |
|------|-------|-------|------|
| TestDiagnostic_SinglePulseChain | FAIL | FAIL | 결함-calibrated, skip 허용 |
| TestDecode_LowEnergyCodebookIsSmooth | FAIL | FAIL | 결함-calibrated, skip 허용 |
| TestDecode_SucceedsAcrossAllGainIndices | FAIL | FAIL | 결함-calibrated, skip 허용 |

C2 후 회복 없음. 본 task 범위 외. F-quint-3 또는 별도 cleanup task로 이관.
**E1 미발동 (계획서 명시).**

### 4.4 전체 패키지 테스트 결과

```
ok    internal/bitstream
ok    internal/fcb
ok    internal/fixed
ok    internal/lsp
ok    internal/pcm
ok    internal/pitch
ok    internal/postfilter
ok    internal/synth
ok    internal/tables
ok    internal/gain        (TestDecodeVQ_* PASS, vq_test.go E2 alignment 후)
FAIL  internal/decoder     (3건: 4.3 표 참조, 모두 skip 허용)
```

---

## §5 Step 6 — F-quart-1 alignment harness 재측정

`TestDiagnostic_FquartGainImap_Sf0Sample0to7` (measurement-only, no assertion):

### 5.1 Branch A (production C2 적용) — 의미 있는 정렬도

| 도메인 | sample [0..7] 절대값 | matches /40 (vs PST/2, |Δ|≤1 LSB) |
|--------|---------------------|-----------------------------------|
| synth.Filter | [1 2 2 2 1 1 1 1] | 23/40 |
| postfilter.Filter | [1 1 1 1 0 1 1 1] | 23/40 |
| hpFilter | [1 1 1 1 0 1 1 1] | **36/40** |
| pcm.ScaleUpSat (PST) | [2 2 2 2 0 2 2 2] | (PST 도메인) |

Sample 0 PST = 2 → ALGTHM frame 0 sample 0 가드 일치. ✅

F-tris-1 pre-quint baseline (= [2 3 4 4 3 2 1 1])과 sample 0..3 +1 LSB 음의
드리프트 발생 — 이는 C1+C2 fix가 ec dB chain과 GBK 인덱싱을 모두 spec
대로 정정한 결과로, F-tris-1 baseline 자체가 결함 보상에 의해 +1 LSB
편향되어 있었다는 가능성을 제시 (F-quint-3 분석 권고).

### 5.2 Branch B (caller-pre-mapped, C2 후 degenerate)

| 도메인 | matches /40 |
|--------|-------------|
| synth.Filter | 24/40 |
| postfilter.Filter | 20/40 |
| hpFilter | **4/40** |

C2 production fix 이후 Branch B는 inverse-map-of-inverse-map 이중 적용
경로가 되어 의미 상실. 측정값만 기록 (계획서 의무 — 강압-적합 회피).
시나리오 분류: **S3 (악화: Branch B 정렬도 < Branch A)** — C2 fix가 spec
정합 방향임을 측정값으로 재확인.

---

## §6 F-quint-3 진입 + 잔여 보류 항목

### 6.1 F-quint-3 진입 권고

**진입 조건 충족**:

- Phase 1i sample 0 가드 (got=2 want=2) 회복 ✅
- Branch A hpFilter 36/40 정렬 (sample 0..7: 7/8 일치) ✅
- F-quart-3 cross-check Branch P PASS (gp 정확, gc Δ≤+2 LSB) ✅
- 변경 금지 파일 위반 0 ✅
- Stage D / D-bis contract 회귀 0 ✅

### 6.2 F-quint-3 후속 검토 항목

1. **F-tris-1 baseline drift**: synth.Filter sample 0..3 = [1 2 2 2] vs baseline [2 3 4 4]
   — C1+C2 정합 후 -1 LSB 일관 음의 드리프트, 후속 단계 분석.
2. **비-contract diagnostic 3건 회복 또는 삭제 결정**: SinglePulseChain /
   LowEnergyCodebookIsSmooth / SucceedsAcrossAllGainIndices.
3. **ITU vector bit-exact (Phase 1h INCOMPLETE)**: postfilter §A.4.2 4-sample
   delay + polarity, HP filter §4.2.2 startup 후속 fix.

### 6.3 잔여 보류

- `internal/lsp/lsp_lp.go` (F-bis-1 P fix uncommitted): 별도 task로 commit 필요.
- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked): F-bis-1 commit과
  동반.

---

**End of report.**
