# Phase 1m — F-Cγ-elsewhere Cycle Plan (parameter decode upstream of LP synthesis)

**Cycle ID**: `F-Cγ-elsewhere` (Phase 1m 1번째 cycle, alternative path **(b)** — parameter decode pipeline upstream of LP synthesis)
**작성일**: 2026-05-07
**선행 commit**:
- `9ab1c91` — gate 17 disposition (Phase 1l alt-path d-i): `t.Skip` + evidence summary, production 0 변경.
- `f902bd9` — Phase 1l F-non-Hpost synthesis + close + alternative path 4-옵션 (a/b/c/d).
- `2ee0009` — Phase 1l HP-2 HP filter frame-edge diagnostic.
- `076b6de` — Phase 1l HP-1 inter-subframe postfilter state diagnostic.
- `308e4f3` — Phase 1l F-non-Hpost cycle plan.

**선행 plan 양식**: `docs/superpowers/plans/2026-05-06-phase1l-stage-f-non-hpost-plan.md`.

**사용자 게이트 (가정)**: G-XS5 = (b) Cγ-elsewhere — parameter decode upstream re-visit (default 권고 ordering 의 rank 2; rank 1 (d-i) 은 `9ab1c91` 으로 이미 처리 완료).

---

## Phase 0 — Context, Invariant, Cumulative Catalog

### 0.1 직전 cycle 정리 (Phase 1l 종결 + gate 17 disposition)

**Phase 1l F-non-Hpost** (`f902bd9`):
- 누적 22 sub-hypothesis 폐기 (16 Phase 1k + 4 Phase 0c + 2 Phase 1l).
- (Hpost-refute) 시나리오 확정 — postfilter chain (Hp/Hf/γ_t/AGC) inter-subframe state + §A.4.2.5 HP filter frame-edge state 모두 EQ.
- alternative path 4-옵션 사용자 게이트 G-XS5 권고: (d) → (b) → (c) → (a).

**gate 17 disposition** (`9ab1c91`):
- `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` 을 `t.Skip` 으로 재분류 (path (d-i) 채택).
- skip 메시지 = 22 sub-hypothesis 폐기 + 3 hard-spec invariant 매핑 + reactivation triggers (mechanism 식별 후 자동 재활성).
- production 0 변경, 회귀 baseline RED noise 제거.

### 0.2 누적 폐기 catalog (22 sub-hypothesis, defect = 0)

22 sub-hypothesis 모두 **postfilter chain (Hp/Hf/γ_t/AGC) + HP filter (§A.4.2.5)**, 즉 **LP synthesis 하류** 에 집중. 직접 측정으로 식별된 hard-spec invariant 3건:

| # | invariant | 출처 §, 출처 cycle |
|---|----------|-----------|
| I-1 | postfilter `agcGainPrev` 의 subframe 경계 carryover (§4.2.4) | HP-1 (`076b6de`) |
| I-2 | §4.3 catch-all zero-init (HP filter `hpX[2]`/`hpY[2]` + 미열거 변수) | HP-2 (`2ee0009`) |
| I-3 | §A.4.2.5 IIR pole-pair impulse decay (frame-edge 1.93/-0.94 정확 추적) | HP-2 (`2ee0009`) |

**핵심 결론**: 22 sub-hypothesis 모두 LP synthesis **하류** → mechanism 위치 = LP synthesis **상류** (parameter decode) 또는 LP synthesis 자체, 또는 spec-외부 (vector derivation uncertainty / errata).

### 0.3 ALGTHM frame 0 sf0 누적 측정-state table (Phase 1l carry)

| 항목 | 값 (Q-format) | 출처 cycle |
|------|---------------|-----------|
| L0 / L1 / L2 / L3 (LSP indices) | bitstream 값 (CE-3 시 dump) | 본 cycle |
| GA1 / GB1 (gain VQ indices) | bitstream 값 (CE-1 시 dump) | 본 cycle |
| C1 / S1 (FCB positions/signs) | bitstream 값 (CE-2 시 dump) | 본 cycle |
| g_p (Q14) | +1995 | F-non-prelim-X-split-2 |
| g_c (Q1 — 측정 시 명세) | +4153 | F-non-prelim-X-split-2 |
| v[0..4] (adaptive cb) | 0 | F-non-prelim-1 |
| c[0..3] (fcb pulse, Q13) | +8192 each | F-non-prelim-X-split-1 |
| u[0..3] (excitation) | +1 each | F-non-prelim-1 |
| u[4..7] | 0 | F-non-prelim-1 |
| syn[5..7] | `[+1, +1, +1]` | F-non-prelim-1 |
| sPf[5..7] | `[+1, +1, +1]` | F-oct-postfix2-prelim-4 |
| post-HP (PST) | `[+2, +2, +2]` | F-oct-postfix2-prelim-4 |
| **want (ALGTHM.PST)** sample 5..7 | `[−1, −1, −1]` | F-oct-postfix-1 / M5 |
| Δ sample 5..7 | uniform **+3** | P0c-2 |
| S* (argmin chain stage) | `postX2` (sample-match=76/80) | P0c-2 |

**핵심 패턴 carry-over**:
- 값 magnitude 자체는 Δ=+3 / 78-sample-match — 즉 **부호 vector 3개** 만 mismatch.
- 22 sub-hypothesis 폐기 시점에 mismatch mechanism 위치 미식별. 본 cycle 가설 = **상류 parameter decode 의 부호/sign-aware mismatch (e.g. gain VQ table sign, FCB sign bit ordering, LSP MA predictor selector L0)** 가 syn 부호를 통해 흐름.

### 0.4 Invariant E1-E5 재확인 (carry, 본 cycle 강압-적합 회피)

- **E1**: 외부 G.729 구현 0건 참조. spec source = `docs/superpowers/specs/itu/G729E.pdf` (main body §3-4 + Annex A §A.3-A.4) + `testdata/itu/G729_Release3/g729AnnexA/test_vectors/READMETV.txt` + 교과서 (Kondoz, Spanias, Chu, Goldberg/Riek) only. **금지**: ITU reference C (g729a.tar.gz), bcg729, Sipro Lab, FFmpeg G.729 decoder, Annex A binary 일체. 본 cycle 의 모든 verbatim 인용은 위 ALLOWED 출처 only.
- **E2**: production 변경 0 라인 (측정 only). 본 cycle 의 모든 task = 진단 test 추가만. 결함 식별 시 **별도 fix cycle**.
- **E3**: gate 17 = `9ab1c91` 으로 t.Skip 잠정 처리. 본 cycle 결과가 mechanism 식별 시 **gate 17 reactivation trigger** 발동 (§7 참조).
- **E4**: 측정값과 spec 비교 시 **PDF/README verbatim 인용 의무**. cherry-pick 금지. 모든 verdict = `EQ` / `NE` 이진. UNDETERMINED 는 spec 진정 모호 시에만.
- **E5**: 자동 promotion 0 — 측정-only test 는 회귀 게이트 자동 등재 금지. 본 cycle synthesis 결정 후 **명시 사용자 게이트** 통해 promotion.

**강압-적합 회피 절차** (재확인):
1. 측정 결과가 PDF/README mismatch = production bug 후보.
2. **금지**: "거의 정합" / "범위 내 변동" / "자연스러운 구현 관행". sub-hypothesis 별 EQ/NE 이진.
3. **금지**: PDF §3.x / §A.3.x 모호 paragraph 를 우리 구현 정당화로 사용. 모호 = 별도 sub-hypothesis 분리.

### 0.5 누적 contract test gate (19건)

| # | gate | 상태 | 출처 |
|---|------|------|------|
| 1..16 | (Phase 1a~1j 누적) | PASS | 누적 |
| 17 | `TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput` | **SKIP** (`9ab1c91`, t.Skip + reactivation triggers) | F-oct-postfix-1 / Phase 1l 종결 |
| 18 | F-non-prelim-X-split measurement bundle (Cα fcb + Cβ gain) | PASS | F-non-prelim-X-split (`aa9dcf9`) |
| 19 | P0c-reentry measurement bundle (P0c-1/2/3) | pending (E5 사용자 게이트 미수행) | P0c-reentry |

본 cycle 신규 측정 test 3건 = 측정-only, E5 자동 promotion 금지 (잠정 gate 20번 = pending, synthesis 후 사용자 합의).

### 0.6 Working tree 보존 명시

- `internal/decoder/stagef_bis_diagnostic_test.go` (untracked) **변경 금지** — Phase 1k 부터 보존된 진단 test, 본 cycle scope 외. 본 cycle commit 시 `git status` 에 untracked 상태 그대로 유지.

### 0.7 NEW: 본 cycle hypothesis 진술

**Path (b) primary hypothesis**:
> 22 sub-hypothesis 가 LP synthesis **하류** (postfilter + HP) 에 집중되어 0 defect 로 폐기됨 → sample 5..7 sign mismatch 의 mechanism 은 LP synthesis **상류** (parameter decode pipeline) 에 잔존할 가능성이 가장 높다. 후보 surface 3종: (i) **gain VQ decode** (GA/GB → g_p/γ̂_c → g_c reconstruction, §3.9 / §A.3.9 Imap + GBK 테이블), (ii) **FCB position + sign decode** (§3.8 Table 7, 13-bit position + 4-bit sign 의 bit-field ordering), (iii) **LSP/LSF decode + interpolation** (§3.2.4 MA predictor selector L0 + 첫 frame init q_i = cos(i·π/11) + §4.1.2 sf-1/sf-2 보간 weight).

**Defect class (예상)**:
- 부호 mismatch (`+1 → −1` / `+8192 → −8192`) 가 interior sample 에 등장 → 상류 parameter 의 **부호/sign-bearing field** mis-decode (sign bit ordering, table sign convention, predictor selector dispatch). magnitude bug 아님.
- F-non-prelim-X-split-1/2 + F-non-prelim-1 폐기 evidence: 출력 c[]/g_c/u[]/syn 값 정합 (sample 5..7) → mismatch 가 **다른 sample (e.g. boundary cluster) 또는 cross-vector 영역** 에서 상류 origin 일 가능성. 본 cycle 측정 = sample 5..7 한정 폐기 시도가 아닌 **상류 parameter raw value verbatim cross-check**.

---

## Phase 1 — Hypothesis Tree (parameter decode upstream)

```
F-Cγ-elsewhere (path (b), 사용자 G-XS5 가정)
├── CE-1 (gain VQ decode, §3.9 / §A.3.9)
│   ├── GA/GB Imap inverse 적용 (§3.9.3 reorder) verbatim 정합
│   ├── GBK1/GBK2 테이블 entry sign + Q-format verbatim
│   └── g_p / γ̂_c 합산 saturation 동작 verbatim
├── CE-2 (FCB position + sign decode, §3.8)
│   ├── 13-bit position field MSB-first decomposition (§3.8 Table 7) verbatim
│   ├── 4-bit sign field bit→sign convention (§3.8 eq. 45) verbatim
│   └── pitch enhancement β·c(n−t) Q-format (§3.8 후반)
├── CE-3 (LSP MA predictor + interpolation, §3.2.4 / §4.1.2)
│   ├── L0 selector dispatch (predictor set 0 vs 1) verbatim
│   ├── 첫 frame past-residual init = i·π/11 Q13 verbatim
│   ├── 첫 frame prevLSP init = cos(i·π/11) Q15 verbatim
│   └── sf-1/sf-2 보간 weight (sf1 = (prev+curr)/2, sf2 = curr) verbatim
└── CE-4 (synthesis, 3-시나리오 결정 트리)
    ├── (CE-defect)    CE-1/2/3 ≥1 NE → 별도 fix cycle dispatch + gate 17 reactivation
    ├── (CE-refute)    CE-1/2/3 EQ_ALL → path (b) 폐기, alternative (c)/(a) 진입
    └── (CE-ambiguous) PDF §3.x verbatim 부재 + production 정당화 불가 → E4 spec ambiguity 분리
```

**기대 entropy** (사전):
- (CE-defect) ≈ 35% — 22 폐기 누적 base rate (16% defect/cycle Phase 1k) 대비 상류 surface 의 측정 부재 가산.
- (CE-refute) ≈ 50% — base rate 우세.
- (CE-ambiguous) ≈ 15% — §3.8 Table 7 의 bit-field MSB/LSB 표기 vs Annex A simplified 표기 차이 후보.

---

## Phase 2 — Pre-cycle exploration 결과 (parameter decode pipeline survey)

본 절은 plan 작성 직전 수행한 production code 직접 read 결과. ALL CITATIONS 는 production file path + line, ALLOWED 출처 only.

### 2.1 main decode loop — `internal/decoder/decode.go`

`Decoder.Decode(packed, bad, out)` (line 18~50):
1. `bitstream.Unpack(packed, &f)` — 80-bit 패킷 → `bitstream.Frame` 16 field (line 27~30).
2. `d.lsp.Decode(lsp.Indices{L0, L1, L2, L3})` → `(sf1A, sf2A [11]int16)` Q12 LP coefficient 2개 subframe 분 (line 32~37). **§3.2.4 + §4.1.2** governs.
3. `pitch.DecodeDelaySubframe1(P1)` + `pitch.CheckParity(P1, P0)` (line 39~40) — §3.7.
4. `pitch.DecodeDelaySubframe2(P2, tInt1)` (line 42).
5. `decodeSubframe(sf1A, tInt1, tFrac1, C1, S1, GA1, GB1, out[:40])` + `decodeSubframe(sf2A, ...)` (line 44~45).
6. `pcm.ScaleUpSat(out, out)` — §A.4.2.5 step 2 (×2 scaling, 본 cycle scope 외, 이미 EQ 확정).

`decodeSubframe(...)` (`internal/decoder/subframe.go:21`):
- `betaQ14 := fcb.ClampPitchGainForEnhancement(d.prevGpQ14)` — §3.8 enhancement clamp.
- `pitch.AdaptiveCodebook(tInt, tFrac, d.pastExc, &v)` — §3.7.
- `fcb.Decode(fcb.Indices{Positions: C, Signs: S}, tInt, betaQ14, &c)` — §3.8.
- `d.gn.Decode(gain.Indices{GA, GB}, &c)` → `(gpQ14, gcQ12)` — §3.9.
- `synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)` — §3.10.
- `d.syn.Filter(sfA, &u, &s)` — §3.10 LP synthesis.
- 이후 postfilter / HP / past-exc slide / prevGp 갱신 — **본 cycle scope 외 (하류 EQ 확정)**.

### 2.2 gain VQ decode — `internal/gain/`

- **entry**: `(*gain.Decoder).Decode(idx Indices, c *[40]int16) (gpQ14, gcQ12 int16)` (`internal/gain/decode.go:38`).
- **input bitfield**: `Indices{GA uint8 (3 bits), GB uint8 (4 bits)}`.
- **output**: `(gpQ14, gcQ12)` Q14 + Q12 int16. side effect = `d.pastErrors[0..3]` Q10 dB MA predictor FIFO 갱신.
- **상세 stage**:
  1. `decodeVQ(idx)` (`internal/gain/vq.go:19`) — `tables.GainImap1[GA]` / `tables.GainImap2[GB]` 역치환 (§3.9.3 reorder) 후 `tables.GainGBK1` / `tables.GainGBK2` 테이블에서 `(gp Q14, γ̂_c Q13)` component-wise 합산 (Word16 saturation).
  2. `predictedLogGain()` (`internal/gain/predictor.go`) — Ê(m) = E̅ + Σ b_i·Û(m-i) Q10 dB (§3.9 eq. 69).
  3. `pow2Fixed` + `log2Fixed` Q-format 변환 (`internal/gain/pow2.go`, `log2.go`).
  4. `g_c = γ̂_c · g_c0` 합산 saturation.
  5. MA predictor FIFO advance.
- **spec governs**: §3.9 (gain VQ) + §A.3.9 (Annex A simplification, conjugate-structure 2-stage).

### 2.3 FCB position+sign decode — `internal/fcb/`

- **entry**: `fcb.Decode(idx Indices, t int, betaQ14 int16, c *[40]int16)` (`internal/fcb/decode.go:20`).
- **input bitfield**: `Indices{Positions uint16 (13 bits), Signs uint8 (4 bits)}`.
- **output**: `c[40]int16` Q13.
- **상세 stage**:
  1. `decodePositions(idx.Positions)` (`internal/fcb/positions.go:15`) — 13-bit MSB-first decomposition (§3.8 Table 7):
     - bits 12..10 → i0 ∈ [0,7] → pos[0] = 5·i0 (track 0).
     - bits  9..7 → i1 → pos[1] = 5·i1 + 1 (track 1).
     - bits  6..4 → i2 → pos[2] = 5·i2 + 2 (track 2).
     - bit  3 → jx ∈ {0,1} (track 3 half select).
     - bits  2..0 → i3 → pos[3] = 5·i3 + 3 + jx (track 3).
  2. `placePulses(positions, idx.Signs, c)` (`internal/fcb/signs.go:17`) — sign 비트 (3-i) 가 1 → +PulseAmplitude (Q13 = +8192), 0 → −PulseAmplitude. **§3.8 eq. 45**.
  3. `applyPitchEnhancement(c, t, betaQ14)` (`internal/fcb/enhance.go`) — c(n) ← c(n) + β·c(n − t).
- **spec governs**: §3.8 + §A.3.8 (Table 7 + eq. 45 + enhancement filter).
- **F-non-prelim-X-split-1 evidence** (`fd0b381`): ALGTHM frame 0 sf0 의 c[0..3] = +8192 each → positions = {0,1,2,3} (i0=i1=i2=jx=i3=0), signs = `0b1111` 가설 검증됨, sample 5..7 sign mismatch 폐기. 단 cross-vector 일관성 미측정.

### 2.4 LSP/LSF decode + interpolation — `internal/lsp/`

- **entry**: `(*lsp.Decoder).Decode(idx Indices) (sf1, sf2 [11]int16)` (`internal/lsp/decoder.go:61`).
- **input bitfield**: `Indices{L0 uint8 (1 bit), L1 uint8 (7 bits), L2 uint8 (5 bits), L3 uint8 (5 bits)}`.
- **output**: `(sf1A, sf2A [11]int16)` Q12 LP coefficient (sf1A[0] = sf2A[0] = 4096 = 1.0).
- **상세 stage**:
  1. 첫 frame: `pastResiduals[k] = initialPastResidual` (i·π/11 Q13, line 37~48). **§3.2.4**.
  2. `combineResidual(L1, L2, L3, &residual)` (`internal/lsp/codebook.go:18`) — split-VQ 합산.
  3. `rearrangeAdjacent(residual, J=0.0012)` + `rearrangeAdjacent(residual, J=0.0006)` — pre-predictor pair-rearrangement.
  4. `applyPredictor(L0, residual, lsf)` (`internal/lsp/predictor.go:28`) — order-4 MA 예측 (§3.2.4 eq. 20), FIFO advance.
  5. `enforceLSFStability(lsf)` — post-predictor stability.
  6. `lsfToLSP(lsf[i])` element-wise.
  7. 첫 frame: `prevLSP = initialPrevLSP` (cos(i·π/11) Q15, line 29~32).
  8. `interpolateLSP(prevLSP, lsp, sf1, sf2)` (`internal/lsp/interpolate.go:13`) — sf1 = (prev+curr)/2, sf2 = curr. **§4.1.2**.
  9. `lspToLP(sfX, &sfA)` — Chebyshev expansion.
  10. `prevLSP = lsp` (next frame carry).
- **spec governs**: §3.2.4 (MA predictor + 4-stage init) + §4.1.2 (subframe interpolation) + §A.3.2 (Annex A simplification).

### 2.5 bitstream unpack — `internal/bitstream/` (boundary, 재조사 금지)

- `bitstream.Unpack(packed, &f)` → `Frame{L0, L1, L2, L3, P1, P0, C1, S1, GA1, GB1, P2, C2, S2, GA2, GB2}` 16 field (`internal/bitstream/types.go:16~30`).
- 13-bit / 7-bit / 5-bit / 4-bit / 3-bit / 1-bit field 비트폭 명시.
- **이미 정합 boundary** (Phase 1g 이전 누적 PASS gate). 본 cycle 에서는 unpack 결과만 입력으로 사용, **재조사 금지** (E2 spirit + scope limit).

### 2.6 Spec section 매핑 (요약)

| 모듈 | 함수 | spec § (PDF G729E.pdf) | 비고 |
|------|------|-----------------------|------|
| `lsp.Decode` | combineResidual / applyPredictor / interpolateLSP | §3.2.4 (MA pred init, eq. 19/20) + §4.1.2 (interp) + §A.3.2 (Annex A) | L0 selector, init q_i, weight 0.5/0.5 |
| `fcb.Decode` | decodePositions / placePulses / applyPitchEnhancement | §3.8 Table 7 + eq. 45 + §A.3.8 | 13-bit MSB-first, sign bit (3-i), Q13 amp |
| `gain.Decode` | decodeVQ + predictedLogGain + log2Fixed/pow2Fixed | §3.9 (eq. 69) + §3.9.3 (reorder Imap) + §A.3.9 (conjugate-structure) | Imap 역, GBK Q14/Q13, FIFO Q10 dB |
| `pitch.DecodeDelaySubframe*` | (boundary) | §3.7 (out-of-scope; 별도 cycle) | tInt/tFrac, parity P0 |

---

## Phase 3 — Task 분해 (3 측정 task + 1 synthesis, TDD 측정-only)

### Task CE-1: gain VQ decode — Imap + GBK 테이블 verbatim cross-check (ALGTHM frame 0 sf0)

**Sub-hypothesis (CE-1)**:
> ALGTHM frame 0 sf0 의 (GA1, GB1) bitstream 값 → `tables.GainImap1[GA] / GainImap2[GB]` 역치환 (§3.9.3) → `(gp Q14, γ̂_c Q13)` GBK 합산 결과가 **§3.9 / §A.3.9 verbatim 인용 + §3.9.3 reorder 정의** 와 component-wise EQ 이다. 부호 (특히 `γ̂_c`) NE 시 → g_c 부호가 sample 5..7 부호 mismatch 의 직접 mechanism.

**측정 design**:
- ALGTHM frame 0 unpack → (GA1, GB1) raw bit 값 dump.
- Imap 역치환 결과 `(gaEntry, gbEntry)` dump.
- `GainGBK1[gaEntry][0,1]` + `GainGBK2[gbEntry][0,1]` 4-tuple raw 값 dump (Q14 + Q13).
- saturation 유무 (`fixed.Add` overflow) flag dump.
- 최종 `(gpQ14, γ̂_c Q13)` 출력 dump.
- **PDF §3.9 / §A.3.9 verbatim 인용** (test 내 `// G.729 §3.9 ...` 주석 형식) + Imap reorder (§3.9.3) verbatim 인용.
- spec verbatim 정의된 GBK 값 (Annex A Table A.* 의 ALLOWED-source verbatim) 과 production `tables.GainGBK*` 의 (gaEntry, gbEntry) 행 byte-level 비교.

**Cross-reference (production vs spec)**:
- production: `decodeVQ` (`vq.go:19`) — `gpQ14 = GBK1[Imap1[GA]][0] + GBK2[Imap2[GB]][0]`, `γ̂_c = GBK1[Imap1[GA]][1] + GBK2[Imap2[GB]][1]`. saturation = `fixed.Add` Word16.
- spec: §3.9 + §A.3.9 verbatim (PDF 인용 의무) — 같은 식이어야 함. 부호 / index ordering / Q-format mismatch 시 NE.

**Pass/Fail (escape hatch, P0c-2 pattern)**:
- **EQ_ALL** (4-tuple 값 + 합산 결과 모두 spec verbatim 일치, sign EQ): CE-1 폐기 → CE-2 진행.
- **≥1 NE** (특히 γ̂_c 부호 NE): (CE-defect) evidence → CE-2/3 보조 측정 + 별도 fix cycle dispatch.
- **spec verbatim 인용 부재** (§3.9.3 Imap reorder paragraph 모호): UNDETERMINED → E4 분리 sub-hypothesis.

**TDD 절차**:
1. RED: `internal/gain/phase1m_ce1_gainvq_diagnostic_test.go` 신규 — `TestDiagnostic_Phase1mCe1GainVQTableVerbatim`. sub-test = ALGTHM frame 0 sf0 + sf1 (cross-subframe sanity).
2. GREEN: production 0 변경 (E2). test = 측정 + log + classifier `classifyCe1GainVQ(...)`.
3. dump 확인: GA/GB raw → Imap → GBK row → sum → saturation flag.
4. **commit**:
   ```
   test(gain): add Phase 1m CE-1 gain VQ Imap+GBK verbatim diagnostic

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

---

### Task CE-2: FCB position+sign decode — bit-field ordering verbatim cross-check (ALGTHM + FIXED + PITCH frame 0 sf0)

**Sub-hypothesis (CE-2)**:
> §3.8 Table 7 (13-bit position) + §3.8 eq. 45 (4-bit sign) verbatim 인용에서 명시된 **MSB-first bit allocation + sign bit→sign convention** 이 production `decodePositions` (`positions.go:15`) + `placePulses` (`signs.go:17`) 와 EQ. cross-vector (low-energy ALGTHM, high-energy FIXED/PITCH) 3 frame 의 (positions, signs) 결과가 이 verbatim 정의와 component-wise EQ. NE 시 → c[] 의 부호 패턴이 sample 5..7 부호 mismatch (또는 FIXED/PITCH interior `[40..64]` Δ) 의 직접 mechanism.

**측정 design**:
- 3 vector × frame 0 × sf0 unpack → (C1 13-bit, S1 4-bit) raw 값 dump.
- `decodePositions(C1)` 결과 4-tuple positions dump + 각 비트필드 분해 (i0/i1/i2/jx/i3) dump.
- `placePulses(positions, S1, c)` 후 c[] 의 nonzero index/value dump (Q13 ±8192).
- **§3.8 Table 7 verbatim 인용**: PDF "i0 ∈ {0,5,...,35}", track-residue mapping, jx half-select 정의.
- **§3.8 eq. 45 verbatim 인용**: sign bit→amplitude mapping convention (특히 bit ordering: `s0` = MSB or LSB).
- spec verbatim vs production EQ/NE 4-cell verdict (positions × signs × 3 vector) = 12-cell matrix.

**Cross-reference (production vs spec)**:
- production positions: bits 12..10 → i0 (MSB-first), bits 9..7 → i1, bits 6..4 → i2, bit 3 → jx, bits 2..0 → i3.
- production signs: bit (3-i) of S → sign of pulse i (i.e. s0 = bit 3 = MSB).
- spec: §3.8 Table 7 verbatim — Annex A simplification 도 동일한 13-bit allocation 인지 §A.3.8 verbatim 확인.
- production code 의 doc 주석 (`signs.go:8-12`) 이 "exact bit→sign mapping is an encoder convention not pinned by spec" 이라 명시 — 이 부분이 본 sub-hypothesis 의 **핵심 NE 후보**.

**Pass/Fail**:
- **EQ_ALL** (12-cell): CE-2 폐기 → CE-3 진행.
- **≥1 NE** (특히 sign bit ordering NE): (CE-defect) evidence + gate 17 reactivation 후보.
- **spec ambiguity**: §3.8 eq. 45 의 bit ordering verbatim 부재 + §A.3.8 도 모호 시 → E4 분리 (production code 자체 doc 주석이 인정한 ambiguity).

**TDD 절차**:
1. RED: `internal/fcb/phase1m_ce2_position_sign_diagnostic_test.go` 신규 — `TestDiagnostic_Phase1mCe2PositionSignVerbatim`. 3 sub-test (ALGTHM/FIXED/PITCH).
2. GREEN: production 0 변경 (E2).
3. dump: (C1, S1) → (i0,i1,i2,jx,i3) → positions[4] → c[] nonzero pattern.
4. **commit**:
   ```
   test(fcb): add Phase 1m CE-2 position+sign bit ordering diagnostic

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

---

### Task CE-3: LSP MA predictor + interpolation — first-frame init + L0 selector + weight verbatim (ALGTHM frame 0)

**Sub-hypothesis (CE-3)**:
> §3.2.4 (MA predictor 4-stage + first-frame init q_i = i·π/11 Q13 + prevLSP init = cos(i·π/11) Q15) + §4.1.2 (sf-1 interpolation weight 0.5/0.5) + L0 selector dispatch (predictor set 0 vs 1) verbatim 인용 결과가 production `lsp.Decoder.Decode` 와 EQ. NE 시 → sf1A/sf2A LP coefficient 의 변형이 syn 부호를 통해 sample 5..7 mismatch 의 직접 mechanism.

**측정 design**:
- ALGTHM frame 0 unpack → (L0, L1, L2, L3) raw 값 dump.
- 첫 frame 진입 직전 `pastResiduals[0..3][0..9]` snapshot (init 적용 직후) — 40 entry 각각 verbatim init 값 (i·π/11 Q13, i=1..10 × 4 frame) 과 EQ 비교.
- 첫 frame 진입 직전 `prevLSP[0..9]` snapshot — verbatim cos(i·π/11) Q15 와 EQ 비교.
- `combineResidual(L1, L2, L3, residual)` 후 residual[0..9] Q13 dump.
- `applyPredictor(L0, residual, lsf)` 결과 lsf[0..9] Q13 dump + selector dispatch (preds 포인터 = `MAPredictorsLSP[L0]`) 검증.
- `lsfToLSP` 후 lsp[0..9] Q15 dump.
- `interpolateLSP(prevLSP, lsp, sf1, sf2)` 결과 sf1[0..9] + sf2[0..9] dump + weight 검증 (sf1[i] = (prev[i]+curr[i])>>1, sf2[i] = curr[i]).
- `lspToLP` 후 sf1A[0..10] + sf2A[0..10] Q12 dump.
- **PDF §3.2.4 verbatim 인용**: "the initial values of the past quantized residuals are l̂_i^{(k)} = i·π/11" verbatim, "previous frame LSP q_i^{(0)} = cos(i·π/11)" verbatim, eq. 19 (split-VQ combine), eq. 20 (MA predictor).
- **PDF §4.1.2 verbatim 인용**: "q_i^{(1)} = 0.5·q_i^{(prev)} + 0.5·q_i^{(curr)}, q_i^{(2)} = q_i^{(curr)}" verbatim weight.

**Cross-reference (production vs spec)**:
- production `initialPastResidual` (`decoder.go:37~48`): `[2340, 4679, ..., 23396]` — round(i · 25736 / 11) 검증 의무.
- production `initialPrevLSP` (`decoder.go:29~32`): `[31441, 27566, ..., -31441]` — cos(i·π/11) Q15 검증 의무 (PDF spec 가 i 범위 = 1..10 인지 0..9 인지 verbatim 확인).
- production `interpolateLSP` (`interpolate.go:13`): `(prev[i] + curr[i]) >> 1` — bit shift rounding (negative round-down) 이 spec verbatim "0.5 multiplication" 와 EQ/NE 인지 (§4.1.2 의 rounding 모드 명시 여부).

**Pass/Fail**:
- **EQ_ALL** (init 4-tuple + selector + weight + lsp/sf1/sf2 spec-trace): CE-3 폐기 → CE-4 synthesis 진행.
- **≥1 NE** (특히 init 값 NE 또는 선택자 dispatch NE 또는 weight rounding NE): (CE-defect) evidence + gate 17 reactivation 후보.
- **spec ambiguity** (rounding mode 또는 init i-범위 verbatim 부재): E4 분리.

**TDD 절차**:
1. RED: `internal/lsp/phase1m_ce3_init_interp_diagnostic_test.go` 신규 — `TestDiagnostic_Phase1mCe3InitInterpVerbatim`.
2. GREEN: production 0 변경 (E2).
3. dump: (L0,L1,L2,L3) → init snapshot → residual → lsf → lsp → sf1/sf2 → sf1A/sf2A.
4. **commit**:
   ```
   test(lsp): add Phase 1m CE-3 init+interp verbatim diagnostic

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

---

### Task CE-4: synthesis + 3-scenario decision tree

**목적**: CE-1 + CE-2 + CE-3 verdict 결합 → mechanism 식별 또는 path (b) 폐기 결정 + gate 17 reactivation 권고 + 사용자 게이트.

**선행 의무**: CE-1/2/3 commit 완료 후 dispatch.

**TDD 절차**: synthesis = report-only, test 추가 없음.

1. report 작성: `docs/superpowers/plans/2026-05-07-phase1m-stage-f-cgamma-elsewhere-synthesis-report.md`.
   - CE-1 verdict matrix 요약.
   - CE-2 verdict matrix 12-cell 요약.
   - CE-3 verdict matrix 요약.
   - 3-시나리오 결정 트리 적용.
   - gate 17 reactivation 결정 (mechanism 식별 시 자동 reactivation).
   - 차기 cycle 권고 + 사용자 게이트 G-XS6 양식.
2. **commit**:
   ```
   docs(plans): F-Cγ-elsewhere synthesis + Phase 1m decision

   Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
   ```

**3-시나리오 결정 트리**:

| 시나리오 | 조건 | 다음 cycle | gate 17 |
|---------|------|-----------|---------|
| **(CE-defect)** | CE-1/2/3 ≥1 NE | parameter decode fix cycle (별도, 1~2 cy) | reactivation (skip 해제) |
| **(CE-refute)** | CE-1/2/3 EQ_ALL | alternative path (c) corrigendum / (a) multi-frame | skip 유지 |
| **(CE-ambiguous)** | ≥1 spec ambiguity (E4) + 다른 cell EQ | (c) corrigendum 진입 (errata + Appendix I/II/III 확인) | skip 유지 |

---

## Phase 4 — 회귀 게이트 (각 commit 직후)

각 task commit 직후:
1. `go vet ./...` — clean 필수.
2. 누적 19 gate dump:
   - 1..16 PASS.
   - 17 **SKIP** (`9ab1c91` t.Skip + reactivation triggers, mechanism 식별 시까지 유지).
   - 18 PASS.
   - 19 pending.
3. 본 cycle 신규 측정 test 3건 = 측정-only, E5 자동 등재 금지.
4. test 실행:
   - CE-1: `go test ./internal/gain/ -run Phase1mCe1 -v`.
   - CE-2: `go test ./internal/fcb/ -run Phase1mCe2 -v`.
   - CE-3: `go test ./internal/lsp/ -run Phase1mCe3 -v`.
   - 누적: `go test ./...` (17 SKIP 유지 확인).

---

## Phase 5 — Anti-goals (명시 금지 list)

본 cycle 에서 절대 수행 금지:

1. **외부 G.729 구현 참조 (E1)**: ITU reference C source (g729a.tar.gz), bcg729, Sipro Lab, FFmpeg G.729 decoder, Annex A binary 어떤 것도 fetch / read / 인용 / 정신적 참조 금지. PDF table 값 verbatim 인용 시 **반드시 ALLOWED 출처 (PDF G729E.pdf §3-4 + Annex A §A.3-A.4)** 만 사용. 특히 `tables.GainGBK*` / `tables.MAPredictorsLSP` / `tables.LSPCodebookL*` 의 값 검증은 PDF 의 해당 Annex A Table verbatim 인용으로만 수행. 외부 구현의 동일 값 우연 일치는 검증 evidence 가 아님.
2. **production 변경 (E2)**: 본 cycle 의 모든 task 는 측정-only. CE-1/2/3 ≥1 NE 발견 시에도 fix 는 별도 cycle 에서 수행. production code line 변동 = 0.
3. **gate 17 reactivation 강행 (E3)**: 본 cycle synthesis 에서 mechanism 식별 시에만 reactivation 권고 (사용자 게이트 통과 후). CE-refute / CE-ambiguous 시 skip 유지.
4. **bitstream unpack 재조사 (scope)**: `internal/bitstream/` 는 이미 정합 boundary. 본 cycle 입력으로만 사용, decode 검증 금지. 본 cycle scope = unpack 결과 → parameter decode → LP synthesis 직전.
5. **postfilter / HP filter 재방문 (scope)**: 22 sub-hypothesis 폐기 evidence 가 하류 EQ 확정. 본 cycle scope = 상류 only.
6. **encoder side 진단 (scope)**: 본 codec 는 decoder-only delivery. encoder 측 가정 (e.g. GA/GB reorder 가 encoder 측에서 어떻게 적용되는지) 은 spec verbatim 인용 한도 내에서만 참조.
7. **OVERFLOW.BIT bit-stream framing 버그 (scope)**: 별도 issue. 본 cycle 과 무관.
8. **spec ambiguity cherry-pick (E4)**: §3.8 eq. 45 의 sign bit ordering verbatim 부재 시 "production 의 bit (3-i) 가 자연스러우니 EQ" 로 정당화 금지. 모호 = E4 분리 sub-hypothesis + UNDETERMINED verdict.
9. **자동 promotion (E5)**: CE-1/2/3 측정 test 의 회귀 게이트 자동 등재 금지. CE-4 synthesis 후 사용자 게이트 G-XS6 통과 시에만 promotion.
10. **untracked 진단 파일 변동**: `internal/decoder/stagef_bis_diagnostic_test.go` 본 cycle 동안 stage / commit / move / delete 금지.

---

## Phase 6 — Escape hatch E1-E5 (Cγ-elsewhere 특수 trigger)

| code | 발동 조건 | 행동 |
|------|----------|------|
| E1 | 외부 G.729 구현 참조 유혹 (특히 GBK 테이블 값 검증 시 reference C 의 동일 테이블 cross-check 욕구, Annex A binary 의 same-input cross-check) | 즉시 차단. 검증 = PDF Annex A Table verbatim 인용 only. |
| E2 | production 변경 유혹 (CE-2 sign bit ordering NE 발견 시 `(3-i)` → `i` 즉시 swap 욕구, CE-3 init 값 NE 시 `initialPastResidual` 재계산 욕구) | 즉시 차단. fix = 별도 cycle. 본 cycle = 측정 + report only. |
| E3 | gate 17 즉시 reactivation 욕구 (CE-N NE 발견 시) | 차단. reactivation = synthesis + 사용자 게이트 후. |
| E4 | spec verbatim 부재를 production 정당화로 사용 (특히 §3.8 sign bit ordering, §4.1.2 rounding mode) | 차단. 모호 = 별도 sub-hypothesis 분리 + UNDETERMINED. |
| E5 | 측정 test 자동 promotion (CE-1/2/3 commit 시 회귀 게이트 등재) | 차단. promotion = CE-4 + 사용자 G-XS6 후. |

---

## Phase 7 — Reactivation map (gate 17 ↔ 본 cycle finding)

본 cycle 결과와 gate 17 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`, `9ab1c91` t.Skip) 의 상호작용:

| 본 cycle 결과 | gate 17 처치 | 정당화 |
|--------------|-------------|--------|
| **(CE-defect)** + NE field 가 ALGTHM frame 0 sf0 sample 5..7 부호에 직접 propagation 가능 | **reactivation 권고** (synthesis 시 명시) — t.Skip 해제 + RED 복귀 + fix cycle 진입 후 GREEN 회복 | 22 sub-hypothesis 폐기 evidence 의 "spec-correct 분류" 가 무효화됨. mechanism 식별 = skip 정당화 소멸. |
| **(CE-defect)** + NE field 가 sample 5..7 영향 미상 (e.g. CE-2 NE 가 FIXED/PITCH 만 영향) | **부분 reactivation 검토** — gate 17 skip 유지 + 신규 gate (cross-vector) 추가 | sample 5..7 mechanism 미식별 + 별도 cross-vector mechanism 식별. |
| **(CE-refute)** | **skip 유지** — `9ab1c91` reactivation triggers 추가 갱신 (path (b) 소진 명시) | 23 sub-hypothesis 폐기 누적 → "spec-correct, vector-derivation-uncertain" 분류 강화. |
| **(CE-ambiguous)** | **skip 유지** + (c) corrigendum cycle 진입 trigger 추가 | spec verbatim 부재 = errata 확인 사유 발생. |

**reactivation 명시 의무** (CE-defect 채택 시):
- gate 17 t.Skip 메시지 갱신 = 본 cycle synthesis commit 에 production change 1 라인 (test file only) 포함 → E2 예외 (test-side reactivation 은 E2 scope 외, 단 사용자 게이트 G-XS6 통과 의무).
- 또는 별도 fix cycle 의 첫 commit 에서 reactivation (preferred — synthesis commit 의 production-test-zero 원칙 유지).

---

## Phase 8 — Self-review (작성자)

- ✅ Phase 0 (Phase 1l 종결 + gate 17 disposition + 22 누적 sub-hypothesis catalog + 3 hard-spec invariant) 포함.
- ✅ Phase 0.3 measured-state table (parameter decode raw 값 dump 추가 — CE-1/2/3 시 채워질 entry).
- ✅ Phase 0.4 강압-적합 회피 절차 (외부 구현 0건 / 측정-only / 모호 분리).
- ✅ Phase 0.5 19 gate 상태 (17 SKIP 갱신).
- ✅ Phase 0.6 untracked file 보존 명시.
- ✅ Phase 0.7 path (b) hypothesis 진술 (3 후보 surface + defect class 예상).
- ✅ Phase 2 pre-cycle exploration (decode loop + 3 모듈 entry + spec § 매핑).
- ✅ Task 4개 분해 (CE-1 gain VQ / CE-2 FCB position+sign / CE-3 LSP init+interp / CE-4 synthesis).
- ✅ 각 task TDD (RED → GREEN → commit) + escape hatch + spec verbatim 인용 의무.
- ✅ 각 task verbatim commit message + Co-authored-by trailer.
- ✅ 회귀 게이트 + E5 자동 promotion 금지.
- ✅ Anti-goals 10 항목 (외부 구현 / production / scope / ambiguity / promotion / untracked file).
- ✅ Reactivation map (4 시나리오 × gate 17 처치).
- ✅ Escape hatch E1-E5 (Cγ-elsewhere 특수 trigger).

**위험 요소**:
- (R-A) §3.9.3 Imap reorder paragraph 가 verbatim 명시 부재 + production code 가 ITU-관행 reorder 적용 시 → CE-1 ambiguity 발생. E4 분리 의무.
- (R-B) §3.8 eq. 45 의 sign bit ordering ("s_0 = MSB" 또는 "s_0 = LSB") verbatim 미명시 시 → CE-2 ambiguity. production code doc (`signs.go:8-12`) 자체 인정. E4 분리 의무 + (c) corrigendum trigger 강력 후보.
- (R-C) §4.1.2 sf-1 interpolation 의 rounding mode (`(prev+curr) >> 1` = floor-toward-neg-inf) verbatim 미명시 시 → CE-3 ambiguity. negative LSP value 에서 -1 LSB drift 후보.
- (R-D) Annex A Table 의 PDF 표 verbatim cross-check 시 PDF text recognition 신뢰도 한계 → table 값 byte-level 비교에 spot-check (10~20 entry) 만 수행, 전체 enumerate 금지 (cost vs gain 균형).
- (R-E) F-non-prelim-X-split-1 (Cα fcb pulse) + F-non-prelim-X-split-2 (Cβ gain g_c) + F-non-prelim-1 (Y a[0..10] sign) 가 일부 영역 폐기 → 잔여 surface 가 비어있을 위험. CE-1/2/3 가 폐기된 영역의 측정 차원 (raw bitstream / table verbatim / cross-vector) 에서 신규 surface 임을 commit message + sub-hypothesis 진술에 명시.

---

## Phase 9 — Execution Handoff

**다음 dispatch**: `CE-1` (Task 1, gain VQ Imap+GBK verbatim cross-check).

**선행 의무 (dispatch 직전)**:
1. Phase 0.5 19 gate baseline 확인 (`go test ./...` + 17 SKIP 유지 + 본 cycle 신규 test 0건).
2. PDF §3.9 + §A.3.9 + §3.9.3 Imap reorder paragraph 위치 사전 확인 (verbatim 인용 준비).
3. `internal/gain/types.go` + `internal/tables/` GainGBK1/GBK2/Imap1/Imap2 export 가시성 확인 (test 위치 = `internal/gain/` package-internal).
4. ALGTHM.BIT 의 frame 0 raw bytes 확인 (testdata helper 활용).

**완료 trigger**: CE-3 commit 직후 → CE-4 synthesis dispatch → 3-시나리오 verdict + gate 17 reactivation 권고 + 사용자 게이트 G-XS6.

---

**Plan 종료.** 본 commit = `F-Cγ-elsewhere` cycle 0번째 (plan-only) commit. 다음 commit = CE-1 (`test(gain): add Phase 1m CE-1 gain VQ Imap+GBK verbatim diagnostic`).

---

## Task 진행 status

- [ ] Task 1 — CE-1 (gain VQ Imap+GBK verbatim) — pending.
- [ ] Task 2 — CE-2 (FCB position+sign bit ordering verbatim) — pending.
- [ ] Task 3 — CE-3 (LSP init+interp verbatim) — pending.
- [ ] Task 4 — CE-4 (synthesis + 3-시나리오 결정 트리 + gate 17 reactivation 권고) — pending.
