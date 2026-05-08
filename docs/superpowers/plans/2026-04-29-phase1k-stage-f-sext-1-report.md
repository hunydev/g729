# Phase 1k Stage F-sext-1 보고서 — postfilter §A.4.2 chain trace

**작성일**: 2026-04-29
**범위**: F-quint-3 §3.3 의 sample 5..7 부호 반전 (|Δ|=2) 의 chain
        boundary 식별 진단.
**산출물**: synth.Filter → postfilter.Filter → hpFilter → pcm.ScaleUpSat
            4 stage 별 sample 5..7 부호 + 절대값 측정값 표.
**준수**: ITU-T G.729 (06/2012) §4 + §A.4.2 인용. 외부 G.729 구현 0건 참조.
**production 변경**: 0 라인.

---

## 0. Working tree 상태 + escape hatch 평가 (E1/E2/E3/E4/E5)

### 0.1 사전 working tree (Step 1)

```
M internal/lsp/lsp_lp.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```

`internal/lsp/lsp_lp.go` (별도 F-bis-1 P fix uncommitted) 와
`stagef_bis_diagnostic_test.go` (untracked) 는 본 task 진입 전부터
존재하며, 본 task 동안 **0 변경 보존** 한다.

### 0.2 사후 working tree (Step 7)

```
M  internal/lsp/lsp_lp.go                                   ← 미변경 보존
?? internal/decoder/stagef_bis_diagnostic_test.go           ← 미변경 보존
?? internal/decoder/stagef_sext_diagnostic_test.go          ← 본 task
?? docs/superpowers/plans/2026-04-29-phase1k-stage-f-sext-1-report.md
```

### 0.3 Escape hatch 평가표

| Hatch | 정의 (요약) | 본 task 발동 여부 | 근거 |
|-------|-------------|-------------------|------|
| E1 | 회귀 게이트 (Phase 1i 가드 / F-quart-3 cross-check / D·D-bis contract) FAIL | **미발동** | §2 + §5 회귀 게이트 PASS 확인. plan-허용 3건 외 새 FAIL 없음. |
| E2 | 진단 결과가 plan 의 핵심 가설을 뒤집어 후속 task 무의미 | **미발동** | 시나리오 분류 (i) 도출 — 후속 진단 (F-sept 상류 fix cycle) 의 출발점 명확. |
| E3 | 측정값 부족 / 재현 불가 | **미발동** | ALGTHM frame 0 sf0 chain 4 stage 부호 + 값 모두 재현 가능 (PASS). |
| E4 | 외부 G.729 구현 (ITU 참조 C / bcg729 / Sipro Lab / FFmpeg) 인용 | **미발동** | 본 task 의 신규 코드 + 보고서 모두 spec PDF 기반. 외부 구현 인용 0건. |
| E5 | production (`internal/**` 중 `*_test.go` 가 아닌 파일) 변경 | **미발동** | `git diff -- internal/` 의 변경은 *사전부터 보유 중이던* `lsp/lsp_lp.go` 만 (본 task 가 추가 수정 0). 신규 파일 2건 모두 *_test.go / *.md. |

---

## 1. §4 + §A.4.2 chain sequence 인용

ITU-T G.729 (06/2012) 의 디코더 출력 chain (sf 단위):

1. **§4 합성 필터 (synthesis filter)** — `1/Â(z)` 적용. 본 구현: `internal/synth/synthesizer.go` `Synthesizer.Filter`.
2. **§A.4.2 적응 postfilter** — long-term + short-term + tilt + AGC. 본 구현: `internal/postfilter/postfilter.go` `Postfilter.Filter`.
3. **§4.2.2 출력 high-pass filter** — 100 Hz cutoff, 2-pole 2-zero IIR (식 (151)/(152)). 본 구현: `internal/decoder/hpfilter.go` `Decoder.hpFilter` (계수: `hpB0Q13=7699`, `hpB1Q13=-15399`, `hpB2Q13=7699`, `hpNegA1Q12=7918`, `hpA2Q13=7667`).
4. **출력 스케일** — `pcm.ScaleUpSat` (sample × 2 with saturation).

본 진단은 위 4 boundary 의 sample 5..7 출력을 capture 하여 부호 반전 boundary 를 단일하게 식별한다.

---

## 2. 회귀 게이트 baseline (Step 1 출력)

세 개의 baseline 게이트 모두 **PASS**:

```
$ go test ./internal/decoder/ -run TestDecode_Frame0Sample0_MatchesALGTHM -v
=== RUN   TestDecode_Frame0Sample0_MatchesALGTHM
--- PASS: TestDecode_Frame0Sample0_MatchesALGTHM (0.00s)
PASS
ok  	github.com/hunydev/g729/internal/decoder	(cached)

$ go test ./internal/decoder/ -run TestDiagnostic_FquartGainReferenceCrossCheck -v
... (생략) ...
--- PASS: TestDiagnostic_FquartGainReferenceCrossCheck (0.00s)
PASS

$ go test ./internal/decoder/ -run TestDiagnostic_FquartGainImap_Sf0Sample0to7 -v
... (생략) ...
--- PASS: TestDiagnostic_FquartGainImap_Sf0Sample0to7 (0.00s)
PASS
```

---

## 3. 진단 측정값

### 3.1 4 stage chain trace raw output (Step 3)

```
$ go test ./internal/decoder/ -run TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 -v
=== RUN   TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7
    stagef_sext_diagnostic_test.go:49: PST/2 sample 5..7 = [-1 -1 -1]
    stagef_sext_diagnostic_test.go:99: ──────── sample 5..7 chain trace ────────
    stagef_sext_diagnostic_test.go:100: stage              [   5    6    7]  부호분포
    stagef_sext_diagnostic_test.go:101: synth.Filter       [   1    1    1]  [+ + +]
    stagef_sext_diagnostic_test.go:102: postfilter.Filter  [   1    1    1]  [+ + +]
    stagef_sext_diagnostic_test.go:103: hpFilter           [   1    1    1]  [+ + +]
    stagef_sext_diagnostic_test.go:104: pcm.ScaleUpSat     [   2    2    2]  [+ + +]  (PST 도메인)
    stagef_sext_diagnostic_test.go:105: PST want sample 5..7         = [-1 -1 -1]
    stagef_sext_diagnostic_test.go:106: PST/2 spec-target sample 5..7 = [-1 -1 -1]
    stagef_sext_diagnostic_test.go:111: ──────── sample 5 부호 boundary ────────
    stagef_sext_diagnostic_test.go:114: synth.Filter       sample 5 부호 = + (값 1)
    stagef_sext_diagnostic_test.go:114: postfilter.Filter  sample 5 부호 = + (값 1)
    stagef_sext_diagnostic_test.go:114: hpFilter           sample 5 부호 = + (값 1)
    stagef_sext_diagnostic_test.go:114: pcm.ScaleUpSat     sample 5 부호 = + (값 2)
    stagef_sext_diagnostic_test.go:116: PST want sample 5 부호 = − (값 -1)
    stagef_sext_diagnostic_test.go:117: PST/2  sample 5 부호 = − (값 -1)
    stagef_sext_diagnostic_test.go:120: gain VQ: gp_q14=1995 gc_q12=4153   tInt=20 tFrac=0   beta_q14=3277
--- PASS: TestDiagnostic_FsextPostfilterChain_Sf0Sample5to7 (0.00s)
PASS
ok  	github.com/hunydev/g729/internal/decoder	0.001s
```

### 3.2 sample 5..7 부호 분포 표 (Step 4)

| stage | sample 5 | sample 6 | sample 7 | 부호분포 |
|-------|---------:|---------:|---------:|----------|
| synth.Filter        |  1 |  1 |  1 | [+ + +] |
| postfilter.Filter   |  1 |  1 |  1 | [+ + +] |
| hpFilter            |  1 |  1 |  1 | [+ + +] |
| pcm.ScaleUpSat      |  2 |  2 |  2 | [+ + +] |
| **PST want**        | **−1** | **−1** | **−1** | **[− − −]** |
| **PST/2 spec-target** | **−1** | **−1** | **−1** | **[− − −]** |

**보조 측정값**: `gp_q14=1995`, `gc_q12=4153`, `tInt=20`, `tFrac=0`, `beta_q14=3277`.

---

## 4. boundary 시나리오 분류

**분류: 시나리오 (i)** — `synth.Filter` sample 5 = `+` (값 1) 이미 부호 반전 발생. PST want sample 5 = `−` (값 −1). 즉 부호 반전이 chain 의 *첫 boundary* (synthesis filter 출력) 에서 이미 관측되며, postfilter / hpFilter / pcm 의 어느 하류 단계도 부호를 뒤집지 않는다 (3 단계 모두 `+` 유지).

**근거 1-2 문장**: 4 stage 모두 sample 5..7 = `[+ + +]` 로 동일한 부호 분포를 보이므로, 부호 반전의 발생 위치는 *상류* (synthesis filter 입력 = excitation `u[]` 또는 LP coefficients `Â(z)`) 에 존재한다. postfilter §A.4.2 와 HP filter §4.2.2 모두 본 결함의 직접 원인이 *아님*.

**함의**:
- F-sext-2 (HP filter startup transient) 와 F-sext-3 (HP filter reference cross-check) 는 본 결함의 원인이 아님이 정량 확정 → 두 task 의 우선순위는 **유보** 가능.
- 결함은 F-quint cycle 의 fix (gain VQ, FCB enhancement, pitch β) 로도 해소되지 않은 *상류* (gain decode 잔여 / synthesis filter 산술 / LP coefficient interpolation) 결함이며, F-sept production fix cycle 의 진단 대상이다.

---

## 5. F-sext-2 / F-sext-3 / F-sept 진입 권고

1. **F-sept-1 (권고)** — 상류 진단 cycle 신규 가동. 후보:
   - (a) `synth.Synthesizer.Filter` (`internal/synth/synthesizer.go`) 의 sample 5 출력값 `+1` vs ITU 참조 (-1 expected) 의 산술 boundary 추적. excitation `u[5]` (= `gpQ14·v[5] + gcQ12·c[5]` shift) 의 부호 + 절대값 측정.
   - (b) `lsp.Decoder.Decode` 출력 LP coefficients `Â(z)` 의 sf0 (frame 0) 값과 ITU 참조의 `q_p` LSP-derived LP 비교.
   - (c) `pitch.AdaptiveCodebook` 의 `v[5]` + `fcb.Decode` 의 `c[5]` 부호 sanity check.
2. **F-sext-2 (HP startup transient)** — *유보*. 본 task 측정으로 hpFilter 가 sample 5 의 `+` 부호를 *유지* 함이 확인 (반전 ≠ HP filter 결함). plan §4.2.2 startup transient 가설은 본 boundary 와 무관.
3. **F-sext-3 (HP reference cross-check)** — *유보*. 동 사유. (단, 별도 sample range 또는 다른 frame 에서 HP filter 결함이 의심되는 측정이 등장하면 재가동.)
4. **F-sext-4 (종합 보고서)** — 본 보고서 + (F-sept 진입 권고) 를 종합하여 작성.

---

**보고 종료.**
