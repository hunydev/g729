# Phase 3a — Fixed-codebook gain (g_c) Q-format fix

Date: 2026-05-04
Owner: Phase 3 (decoder amplitude recovery)
Inputs:
- Diagnostic report: `docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md`
- Master encoder plan: `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md`
- Spec: ITU-T G.729 (06/2012) §3.9.2, §4.1.6 eq. (74)/(75); Annex A unchanged
- Textbook: Salami 1998 IEEE T-SAP §V.B; Kondoz §6 (CS-ACELP gain quantization)

I1 (clean-room) HARD RULE: no peeking at ITU C reference, bcg729, Sipro
Lab, FFmpeg, or any other G.729 implementation. Spec PDFs + textbooks
only. If unsure → STOP and report.

## 1. Problem statement

Phase 3 entry test (`phase3_roundtrip_quality_test.go`) shows our decoder,
fed ITU `SPEECH.BIT`, produces output with RMS = 65 vs reference RMS ≈
2095 (≈ 33× shortfall). The diagnostic report localizes the entire
shortfall to the excitation `u`, which has measured corpus RMS ≈ 5.3 vs
expected O(150–300). All upstream stages (bitstream unpack, T, v[], c[],
g_p) and downstream stages (1/Â(z), postfilter, HP, ScaleUpSat) are
healthy.

The single failing element is `internal/gain/Decoder.Decode`'s `g_c`
reconstruction. Symptoms:

- `gcQ12` saturates int16 at both extrema (min −32 768 / max +32 767)
- `gcQ12` returns negative values, but g_c is non-negative by spec
- The single-Q12 `int16` envelope cannot represent the spec g_c dynamic
  range (~10⁻³ to ~10⁴)

The bug is symmetric on the encoder side: `internal/gainquant` uses
the same Q-format envelope when applying quantized gains in the
closed-loop search and when committing the excitation memory. Encoder
byte-EQ pins (Phase 2c/2d INT-1{a,b}) are FAIL-DEFERRED so a fix here
must not regress what little EQ already passes; conversely, this fix
may *improve* upstream encoder pins as a side effect.

## 2. Goal

Replace the saturating single-Q12 `g_c` representation with a wider
representation that covers the spec dynamic range without overflow,
threading the change through:
- `internal/gain/Decoder.Decode` (decoder-side reconstruction)
- `internal/synth/BuildExcitation` (consumer)
- `internal/gainquant` (encoder-side Apply / closed-loop)
- `internal/decoder/*` and `internal/encoder/*` call sites

Acceptance:
- `phase3_roundtrip_quality_test.go` SegSNR pipeline B ≥ 3 dB (target,
  TBD in INT-1; floor TBD)
- Phase 1 byte-EQ tests still PASS where they passed before
- Phase 2c/2d INT-1{a,b} byte-EQ does not regress (tracked, not gated)
- 0 allocations in `gain.Decoder.Decode` and `synth.BuildExcitation`
  bench remains stable
- `go test ./... -race` clean
- `go vet ./...` clean

## 3. Open questions (must pin from spec before code change)

| ID | Question | Resolution method | Pinned in task |
|---|---|---|---|
| OQ-GCREP | Spec representation of g_c on the wire / inside the synthesis pipeline: (a) mantissa+exponent (gc_mant Q14 + gc_exp int8), (b) Word32 Q-format that brackets the full range (e.g. Q16 in int32), or (c) two-level (gc0 + γ̂_c) kept separate until the multiply in BuildExcitation | Read G.729 §3.9.2 eq. (74)/(75), Salami 1998 §V.B, Kondoz §6 | DIAG-1 |
| OQ-BWIDTH | Required dynamic range bracket on g_c (and gc0) measured on SPEECH/PITCH/ALGTHM corpora | Extend diag 02 to dump gc0Q14, predicted, ecBarDbQ10 with no clipping | DIAG-1 |
| OQ-EXC-Q | New Q-format for u[n] — keep Q0 Word16 with saturation, or widen to Word32 Q15 internally and saturate on store? | Spec §4.1.6: u is Q0 Word16; keep Q0 with saturation. Pin in REF-1 design note | REF-1 |
| OQ-ENC-SYM | Does the encoder closed-loop search need the same wider repr, or does its FCB target normalization keep gc within Q12 there? | Inspect `internal/gainquant.Apply` and run a parallel diag on encoder-side gc | DIAG-2 |

If any OQ cannot be pinned from the allowed sources, STOP and ask.

## 4. Tasks (TDD — failing test → minimal impl → green → commit)

Each commit message ends with the standard trailer:

    Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>

`git add` specific files only — never `-A`/`-u`/`.`. No `git push`.

### DIAG-1 — extend gain decoder taps and pin OQ-GCREP / OQ-BWIDTH

- Add a test-only `gain.Decoder.DecodeWithTaps` (or extend existing
  `predictor_export_test.go`) returning `(gpQ14, gcQ12, predicted,
  ecBarDbQ10, log2GcQ10, gc0Q14_unsaturated_int32, gammaC)`. NO
  production-API change.
- Add `internal/gain/phase3a_diag1_gc_taps_test.go` running the corpus
  via `internal/decoder` over SPEECH.BIT and dumping per-subframe
  `gc0Q14_unsaturated` distribution (min/max/mean), and confirming
  the int16 wrap hypothesis.
- Update report `docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md`
  with the unsaturated distribution. Pin OQ-GCREP and OQ-BWIDTH in
  a new appendix with spec citations (eq. numbers + textbook page).
- Commit msg: `phase3a(diag1): unsaturated gc0 taps + OQ-GCREP pin`

### REF-1 — design note for new g_c representation

- File: `docs/superpowers/plans/2026-05-04-phase3a-gcrep-design.md`.
- Choose ONE of (a)/(b)/(c) from OQ-GCREP based on DIAG-1 numbers and
  spec language. Strong default: **(a) mantissa Q14 + exponent int8**,
  i.e. gc = gc_mant * 2^gc_exp, because §3.9.2 expresses g_c via the
  predicted log-gain followed by 2^x — exponent is naturally available.
- Specify the new public-internal API:
  - `gain.Decoder.Decode(idx, c) (gpQ14 int16, gcMantQ14 int16, gcExp int8)`
  - `synth.BuildExcitation(gpQ14, gcMantQ14 int16, gcExp int8, v, c, u)`
- Document the Q-format alignment math for BuildExcitation:
  - lCode = LMult(gcMantQ14, c[n])  // Q28 in Word32
  - shift = 13 - gcExp    // align to Q15 like lPitch
  - lCode = LShr(lCode, shift) (or LShl if shift<0, with saturation)
  - lSum  = LAdd(lPitch, lCode); u[n] = Round(LShl(lSum, 1))
- Pin OQ-EXC-Q.
- No code change. Commit:
  `phase3a(ref1): g_c mantissa+exponent design note`

### IMPL-1 — gain decoder returns mantissa+exponent

- Failing test first: `internal/gain/decode_mantexp_test.go` with a
  table of (predicted log gain, ecBar, gammaC) inputs and expected
  (gcMantQ14, gcExp) outputs derived purely from spec arithmetic.
  Include the cases that previously saturated (large gc and tiny gc).
- Run, confirm RED.
- Modify `internal/gain/decode.go` to:
  - Compute `log2GcQ10` as before
  - Split into integer and fractional parts: `gcExp_pre = log2GcQ10 >> 10`,
    `frac = log2GcQ10 & 0x3FF`
  - Apply γ̂_c (still Q13) by adding `log2(γ̂_c)` to log2 *before* the
    pow2 — keeps mantissa range tight
  - `gcMantQ14 = pow2Fixed_frac(frac)` (pow2 of [0,1) → [Q14 1.0,
    Q14 ~2.0)). New helper `pow2FracQ14` if needed.
  - `gcExp` adjusted for the offset chosen (so gcMantQ14 ∈ [16384, ~32767)
    always and ec_exp absorbs the rest)
- Run test, confirm GREEN.
- Commit: `phase3a(impl1): gain decoder returns (gpQ14, gcMantQ14, gcExp)`

### IMPL-2 — synth.BuildExcitation consumes mantissa+exponent

- Failing test first: `internal/synth/buildexcitation_mantexp_test.go`
  - Cases with gcExp = 0 (matches old Q12 path scaled), gcExp > 0
    (large gc), gcExp < 0 (tiny gc); each with deterministic c[40] and v[40]
    and a Q0 expected u[40] derived from spec arithmetic
- Update signature, doc.go, and BuildExcitation body.
- Update existing `internal/synth/*_test.go` and
  `internal/synth/synthesizer*.go` call sites (these tests already
  used hard-coded Q12 gc values — convert them or guard with a
  legacy adapter `BuildExcitationQ12` that wraps and is used only by
  legacy tests).
- Confirm all `go test ./internal/synth/... -race` PASS.
- Commit: `phase3a(impl2): BuildExcitation mantissa+exponent path`

### IMPL-3 — gain encoder (gainquant.Apply) parallel update

- Failing test first: `internal/gainquant/apply_mantexp_test.go`
  mirroring IMPL-1 but on the encoder side. Use the same spec
  arithmetic table.
- Update `internal/gainquant/predictor.go` (Apply / quantized-gain
  output) and `internal/gainquant/conjugate.go` if it consumed gcQ12
  to compute the candidate excitation energy.
- Update `internal/encoder.go` `fcbStep` and `closedloopStep` call
  sites to consume the new triple and feed BuildExcitation correctly.
- Confirm Phase 2c/2d INT-1{a,b} byte-EQ does not regress (re-run and
  capture deltas; record in commit message body).
- Commit: `phase3a(impl3): gainquant + encoder Apply mantissa+exponent`

### IMPL-4 — decoder integration

- Wire the new triple through `internal/decoder` (subframe step, Decode,
  DecodeWithTaps shim — keep the test-only shim updated for the new
  signature).
- Confirm `go build ./...` clean.
- Confirm `internal/decoder/itu_vector_pstdomain_test.go` (Phase 1o
  D-3 sample-0 byte-EQ) still PASS.
- Commit: `phase3a(impl4): decoder integration of mantissa+exponent g_c`

### INT-1 — Phase 3 entry roundtrip pass

- Re-run `phase3_roundtrip_quality_test.go` and
  `phase3_roundtrip_quality_diag_test.go`. Capture:
  - rms(out), max|out|, SegSNR pipeline B
  - rms(u) per subframe — should now be O(100–300)
  - Verify gc0Q14_unsaturated no longer wraps
- If SegSNR ≥ 3 dB: ACCEPT; pin OQ-GCREP final disposition.
- If SegSNR < 3 dB but rms(u) recovered: amplitude is fixed but a
  *different* defect remains (likely candidate B from diagnostic
  report — MA predictor cold-start). Open Phase 3b.
- Commit: `phase3a(int1): Phase 3 entry roundtrip — RMS=<X> SegSNR=<Y>`

### INT-2 — zero-alloc + race + bench

- `go test ./internal/gain/... ./internal/synth/... ./internal/gainquant/... -bench=. -benchmem -race`
- Confirm Decode and BuildExcitation remain 0 alloc/op.
- Capture ns/op deltas vs HEAD `bae0d94` baseline.
- Commit: `phase3a(int2): zero-alloc + race + bench`

### INT-3 — closure report

- File: `docs/superpowers/plans/2026-05-04-phase3a-closure-report.md`
- Summarize: OQ resolutions, byte-EQ deltas (Phase 1o, Phase 2c/2d
  INT-1{a,b}), pipeline B SegSNR before/after, new bench numbers.
- Disposition: CLOSED-PASS / CLOSED-PARTIAL / CLOSED-DEFERRED based
  on INT-1 SegSNR.
- Update master plan
  `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` with a
  note linking the closure report (encoder-side regressions/improvements).
- Commit: `phase3a(int3): closure report`

## 5. Out of scope

- Postfilter / HP tuning (already verified healthy)
- Decoder LP-coefficient pipeline (verified by Phase 1o D-3 byte-EQ)
- Encoder closed-loop pitch search (Phase 2c FAIL-DEFERRED, separately
  tracked); Phase 3a may improve C1/C2 byte-EQ as a side effect but
  does not target it
- New external dependencies

## 6. Risks

- R1: spec representation may not be cleanly mantissa+exponent — DIAG-1
  must pin OQ-GCREP first. Falls back to (b) Word32 Q16 if (a) cannot
  be derived purely from §3.9.2.
- R2: BuildExcitation API change ripples to many tests — IMPL-2
  provides `BuildExcitationQ12` legacy adapter to bound test churn.
- R3: encoder-side IMPL-3 may *worsen* one of the FAIL-DEFERRED byte-EQ
  metrics. Acceptable if SegSNR pipeline B improves; document in
  INT-3 closure.

---

## 7. Final disposition (INT-1 + IMPL-4 landed)

Date: 2026-05-04 (post-c7fcc06)
Disposition: **CLOSED-DEFERRED → Phase 3b**

| Task    | Status |
|---------|--------|
| DIAG-1  | [x] DONE (commit b4f6b05) |
| REF-1   | [x] DONE (commit f9de742) |
| IMPL-1  | [x] DONE (commit b0e6955) |
| IMPL-2  | [x] DONE (commit f137fbf) |
| IMPL-3  | [x] DONE (commit c7fcc06) |
| IMPL-4  | [x] DONE (audit at INT-1; production wiring 100% native; legacy adapter KEPT for 10 test-only call sites — see internal/gain/legacy_gcq12.go header) |
| INT-1   | [x] DONE — **FAIL gate triggered** on Phase 3 acceptance harness; see Appendix D in `docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md` |
| INT-2   | [x] DONE — bench + race captured in Appendix D.6 |
| INT-3 (closure-PASS report) | **WITHHELD** per FAIL guard — replaced by Appendix D FAIL diagnostic |

### Outcome summary

- Pipeline B rms(out): **65 → 419** (+6.45×)
- Pipeline B max\|sample\|: **850 → 5262** (+6.19×)
- Pipeline B SegSNR: **−0.46 dB → −0.90 dB** (−0.44 dB)
- Acceptance: SegSNR < 0 AND rms < 500 → **FAIL**

Amplitude envelope recovered (g_c0 no longer wraps int16); residual
defect is phase / waveform alignment (cross-correlation peak at −22
samples vs path-A intrinsic +40). Diagnosis points to Phase 3b
candidate B (MA predictor cold-start) first, candidate C (LP
coefficient pipeline) second.

### I1 risk

**none** — clean-room maintained; no ITU C reference, bcg729, Sipro,
FFmpeg, or any other G.729 implementation consulted. All work derived
from ITU-T G.729 PDF, Salami 1998 §V.B, Kondoz §6, and prior
in-repo derivations.
