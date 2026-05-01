# Phase 1k Stage F-oct-postfix2-prelim-3 보고서 — M6 PST want 부호 검증

**작성일**: 2026-05-02
**범위**: M6 가설 (PST want 데이터 부호 결함, P-SRC-2 재해석) 측정.
**산출물**: 측정 함수 1 추가 (`TestDiagnostic_FoctPostfix2PrelimM6PSTSignVerify`) + ALGTHM.PST byte-level + multi-vector 분포 + M6 평가.
**준수**: F-oct-prelim-5-1 P-SRC-2 분류 재해석 + READMETV.txt verbatim. production 변경 0 라인. 외부 G.729 0 참조.

직전 task: `6dc851e` (M5 REFUTED: excitation u[5..7]=0, sign 발생 = synth IIR 단계).

## 0. escape hatch 평가 + READMETV.txt verbatim 재발췌

- E1 (회피 가능 spec PDF 우회): 미발생 — READMETV.txt verbatim 만 인용.
- E2 (citation 정정): 본 task PDF § 인용 없음 (해당 없음).
- E3 (강압-적합): 본 task = 측정-only, fit 없음.
- E4 (외부 구현 참조): 0건. ITU-T G.729 (06/2012) PDF + READMETV.txt 만 인용. Annex A binary 사용 금지 invariant 준수 (PST 파일은 *measurement input* 으로만 read).
- E5 (production 변경 0): 준수 (`internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go` test-only 추가).
- 사전 보유 working tree: `?? internal/decoder/stagef_bis_diagnostic_test.go` 보존 확인 (commit 직전 `git status --short` = ` M ...prelim_diagnostic_test.go` + `?? ...stagef_bis_diagnostic_test.go`).

READMETV.txt verbatim (`testdata/itu/G729_Release3/g729/test_vectors/READMETV.txt` 및 g729AnnexA tree 동일):

> Format: all files contain 16 bit sampled data using the Intel (PC) format.
>
> *.in  - input files
> *.bit - bit stream files
> *.out - output files
>
> and were obtained using the following commands
>
>  coder file.in file.bit
>  decoder file.bit file.pst

해석: Intel (PC) format = 16-bit signed little-endian. *.pst = decoder 출력 (file.pst). frame 0 sf0 sample n byte offset = `n*2..n*2+1`. sample 5..7 = byte offset 10..15 (6 byte).

## 1. 14 회귀 게이트 PASS + 항목 15 RED 재확인

```
go test ./internal/decoder/ -run '<게이트 1..14 + 15 + 신규 측정 3>' -count=1
```

결과:
- 게이트 1..14: PASS (단일 FAIL = 게이트 15만)
- 게이트 15 (`TestDecode_AlgthmFrame0Sf0Sample5to7_NegativeOutput`): RED 잔존 의무 충족
  - frame 0 sample 5: got=2 want=-1 (Δ=3)
  - frame 0 sample 6: got=2 want=-1 (Δ=3)
  - frame 0 sample 7: got=2 want=-1 (Δ=3)
- 신규 측정 (`TestDiagnostic_FoctPostfix2PrelimChainDump`, `...M5ExcitationSignTrace`, `...M6PSTSignVerify`): 3건 PASS.
- `go vet ./...`: clean.

비-contract diagnostic 3건 상태: production 변경 0 라인으로 무변동 (검증 없음, plan §0.3 자동 보장 조항).

## 2. ALGTHM.PST byte 10..15 raw hex + int16 LE 해석

```
ALGTHM.PST byte[10..15] = ff ff ff ff ff ff
  sample 5 (byte offset 10..11): hex=ff ff → int16 LE = -1  sign=−
  sample 6 (byte offset 12..13): hex=ff ff → int16 LE = -1  sign=−
  sample 7 (byte offset 14..15): hex=ff ff → int16 LE = -1  sign=−
```

PST want `[-1,-1,-1]` 정합. `0xFFFF` little-endian 2's complement int16 = -1 — byte parsing 결함 0.

추가 cross-check (out-of-band, `cmp`/`xxd`):
- main g729/ tree 와 g729AnnexA/ tree 의 ALGTHM.PST 는 byte 10..15 = `ff ff ff ff ff ff` BYTE-EQUAL (전체 파일은 DIFFER 하나 sample 5..7 byte 정합). F-oct-prelim-5-1 P-SRC-2 의 "Annex A vs main 0/6 BYTE-EQUAL" 분류는 본 sample 5..7 영역에도 *연장 적용*.

## 3. multi-vector frame 0 sf0 sample 5..7 부호 분포

```
ALGTHM.PST    byte[10..15]=ff ff ff ff ff ff  sample5..7=[    -1     -1     -1]  signs=[− − −]
PITCH.PST     byte[10..15]=ff ff ff ff ff ff  sample5..7=[    -1     -1     -1]  signs=[− − −]
FIXED.PST     byte[10..15]=ff ff ff ff ff ff  sample5..7=[    -1     -1     -1]  signs=[− − −]
LSP.PST       byte[10..15]=00 00 00 00 00 00  sample5..7=[    +0     +0     +0]  signs=[0 0 0]
SPEECH.PST    byte[10..15]=fe ff 00 00 00 00  sample5..7=[    -2     +0     +0]  signs=[− 0 0]
TAME.PST      byte[10..15]=00 00 00 00 00 00  sample5..7=[    +0     +0     +0]  signs=[0 0 0]
PARITY.PST    byte[10..15]=00 00 00 00 00 00  sample5..7=[    +0     +0     +0]  signs=[0 0 0]
OVERFLOW.PST  byte[10..15]=00 00 00 00 00 00  sample5..7=[    +0     +0     +0]  signs=[0 0 0]
ERASURE.PST   byte[10..15]=00 00 00 00 00 00  sample5..7=[    +0     +0     +0]  signs=[0 0 0]
```

분포 tally:
- `[− − −]` : 3 vector (ALGTHM, PITCH, FIXED)
- `[0 0 0]` : 5 vector (LSP, TAME, PARITY, OVERFLOW, ERASURE)
- `[− 0 0]` : 1 vector (SPEECH)
- `[+ + +]` : 0 vector

`[+ + +]` 분포 0 — production 출력 `[+2,+2,+2]` 와 정합하는 vector 없음.

## 4. M6 가설 평가 (반증)

| 측정 결과 | M6 가설 평가 |
|-----------|--------------|
| ALGTHM.PST byte 10..15 = little-endian int16 `[-1,-1,-1]` 정합 + 다른 vector 도 `[-,-,-]` 분포 다수 (ALGTHM/PITCH/FIXED 3건) | **M6 REFUTED** ✓ |
| ALGTHM.PST byte 10..15 = `[+2,+2,+2]` 또는 endianness 불일치 발견 | M6 STRONG (해당 안 됨) |
| ALGTHM.PST 단독 `[-,-,-]` ↔ PITCH/FIXED 등 `[+,+,+]` 분포 | M6 PARTIAL (해당 안 됨) |

**verdict: M6 REFUTED** — PST want 부호 자체는 정상. byte parsing/endianness/sign-extension 결함 없음. P-SRC-2 의 "Annex A vs main BYTE-EQUAL" 분류는 sample 5..7 영역에 *유효*. mismatch (`got=+2 want=-1`) 의 origin 은 **production 출력측** (test 인프라 결함 아님). M5 REFUTED + M6 REFUTED → 잔여 가설 = M1' (postfilter 외 분기 cover 결손) + M3 (synth IIR memory propagation) — Task 4 진입 정합.

## 5. F-oct-prelim-5-1 P-SRC-2 분류 재해석 결론

F-oct-prelim-5-1 의 P-SRC-2 ("Annex A vs main 0/6 BYTE-EQUAL") 가 sample 5..7 영역에 *연장 적용* 됨을 byte-level 로 검증:
- ALGTHM.PST (g729 vs g729AnnexA): 전체 파일은 DIFFER 하나 byte 10..15 (sample 5..7) BYTE-EQUAL.
- 9 vector 의 byte 10..15 도 두 tree 간 동일 (out-of-band cmp 검증).

→ P-SRC-2 가 sample 5..7 mismatch 의 root cause 가 *아님* 을 확정. 우리 contract (PST want = `[-1,-1,-1]`) 는 spec-canonical 이며 **fix scope = production**.

## 6. Task 4 진입 의무 (M1' + M3 측정 baseline)

본 cycle 잔여 가설:
- **M1'** — postfilter 외 분기 (longterm/agc/shortterm) cover 결손. F-oct-prelim-5-3 §3.3 의 결정 재평가 + g_l raw output dump (state 영속화 변경 없음, plan §0.4.5).
- **M3** — synth IIR memory propagation (M5 trace 에서 sign 발생 단계 = synth IIR; memory pre/post sample 5..7 trace 필요).

Task 4 진입 baseline:
1. Task 1 chain dump (`6dc851e` 의 직전 `ff5534a`) + Task 2 M5 sign trace (`6dc851e`) + Task 3 M6 byte verify (본 commit) 의 raw 출력을 ground-truth 로 사용.
2. 외부 G.729 0 참조, production 변경 0, helper 신규 0, spec § 인용 = §A.4.2.1/2/4 + §A.4.1 PDF verbatim grep.
3. 사전 보유 working tree (`?? stagef_bis_diagnostic_test.go`) 보존.
4. 회귀 게이트 15 (RED) + 신규 측정 (Task 1/2/3 = PASS) + Task 4 신규 측정 (assertion 0).

## 부록 — git status (commit 직전)

```
 M internal/decoder/stagef_octpostfix2_prelim_diagnostic_test.go
?? internal/decoder/stagef_bis_diagnostic_test.go
```
