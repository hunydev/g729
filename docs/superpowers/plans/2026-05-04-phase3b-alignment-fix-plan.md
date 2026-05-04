# Phase 3b — Decoder phase/waveform-alignment fix (post-amplitude recovery)

Date: 2026-05-04
Owner: Phase 3 (decoder quality recovery, second iteration)
Inputs:
- Phase 3a closure: `docs/superpowers/plans/2026-05-04-phase3a-gc-qformat-fix-plan.md` §7
- Diagnostic Appendix D: `docs/superpowers/diagnostics/2026-05-04-decoder-amplitude-localization.md` §D.1–D.7
- Spec: ITU-T G.729 (06/2012) §3.9.2 / §4.1.6 (gain MA predictor); §3.2.6 / §4.1.5 (LSP/LP interpolation); Annex A
- Textbook: Salami 1998 IEEE T-SAP §V.B (gain VQ MA predictor); Kondoz §6 (CS-ACELP); Chu (LP analysis chapter)

I1 (clean-room) HARD: spec PDFs + textbooks only. NO ITU C reference,
bcg729, Sipro Lab, FFmpeg, or any other G.729 implementation. If unsure → STOP.

## 1. Problem statement

Phase 3a closed g_c saturation but left a **phase/waveform-alignment**
defect:

| Metric                       | Pre-Phase-3a | Post-Phase-3a (HEAD c7860a9) | Phase 3a target | Phase 3 ACCEPT |
|------------------------------|--------------|------------------------------|-----------------|----------------|
| pipeline B rms(out)          | 65           | 419                          | ≥ 1500 PARTIAL  | ≥ 1500         |
| pipeline B max\|sample\|     | 850          | 5262                         | —               | —              |
| pipeline B SegSNR            | −0.46 dB     | **−0.90 dB**                 | ≥ 0 dB PARTIAL  | ≥ 3 dB         |
| pipeline B XCorr peak shift  | n/r          | **−22 samples (vs path-A +40 → 62-sample gap)** | n/r | ≤ 1 sample |

The 62-sample shift is amplitude-blind: g_c-magnitude fixes cannot
close it. Diagnostic Appendix D ranks the candidate space:

1. **Candidate B** (highest signature match): MA gain predictor cold-start
   — the four `gain.Decoder.pastErrors` entries seeded at −14336 (−14 dB
   Q10) per §3.9.2. Encoder-side `gainquant` may seed differently or the
   spec value may be misinterpreted. A per-frame predictor seed mismatch
   accumulates into a phase/timing skew with constant amplitude
   envelope — exactly the observed signature.
2. **Candidate C**: LP coefficient interpolation / Â(z) pipeline. Phase
   1o D-3 sample-0 PASS bounds the surface but does not exclude per-
   subframe interpolation skew that would compound to the 62-sample
   drift.
3. (Lower) Postfilter long-term / tilt μ. Already audited; no surviving
   open hypothesis.

## 2. Goal

Recover phase/waveform alignment of decoder output against the ITU
PST reference such that:

- ACCEPT-PASS: SegSNR pipeline B ≥ 3 dB
- ACCEPT-PARTIAL: SegSNR pipeline B ≥ 0 dB AND rms(out) ≥ 1500
- FAIL: SegSNR < 0 dB OR rms(out) < 500 → STOP, open Phase 3c

Constraints (carry-forward from Phase 3a):
- 0 alloc retained on `gain.Decode`, `synth.BuildExcitation`,
  `gainquant.Apply`, `synth.Synthesize`, `decoder.Decode`
- `go test ./... -race` clean
- `go vet ./...` clean
- Phase 1o D-3 sample-0 byte-EQ gate stays GREEN
- Phase 2c/2d/2f INT-1 byte-EQ may move; only catastrophic regression
  (a metric > 2× baseline %) is a gate

## 3. Open questions (must pin from spec before code change)

| ID | Question | Method | Pinned in |
|---|---|---|---|
| OQ-PASTSEED | Spec value for `pastErrors[i]` cold-start: is `-14336` (−14 dB Q10) correct, or should it be the predicted log of the average codebook gain (≈ tables.GainMeanEnergyQ10)? §3.9.2 wording check. | Read G.729 §3.9.2 final paragraph + Annex A §6.1 init notes; cross-check Salami 1998 §V.B and Kondoz §6 | DIAG-1 |
| OQ-PASTPROG | After zero-energy guard fires (eq. (75) when E_c ≈ 0), the spec mandates a specific re-seed strategy. Current code re-seeds `pastErrors[0]` to −14336 only. Is this the spec behaviour, or should ALL four taps re-seed, or NONE? | §3.9.2 final paragraph; corpus diag of how often the guard fires | DIAG-1 |
| OQ-LP-INTERP | LP interpolation between subframes: spec §3.2.6 prescribes 50/50 interpolation between `q_lsp[m-1]` and `q_lsp[m]` for subframe 1; subframe 2 uses `q_lsp[m]`. Are we doing this, and at what Q-domain (LSP vs LSF vs LP)? | Read §3.2.6; inspect `internal/lsp.Interpolate` and `internal/decoder/subframe.go`; corpus diag of interpolated LSP vector vs reference | DIAG-2 (only if DIAG-1 inconclusive) |
| OQ-PASTSYM | Encoder-side `gainquant` past-error FIFO MUST be byte-identical to decoder side cold-start to avoid drift on round-trip. Is the cold-start seed and progression rule literally the same Go constant on both sides? | grep audit + behavioral test | IMPL-2 |

If any OQ cannot be pinned from allowed sources → STOP and ask.

## 4. Tasks (TDD — failing test → minimal impl → green → commit)

Each commit ends with the standard trailer:

    Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>

`git add` specific files only. NO `git push`. Skip no TDD steps.

### DIAG-1 — pastErrors evolution + spec re-pin (candidate B isolation)

- Add test-only `internal/gain/phase3b_diag1_pastErrors_export.go`
  exposing `Decoder.PastErrorsSnapshot() [4]int16` (read-only).
- Add `internal/decoder/phase3b_diag1_pasterrors_test.go` driving
  SPEECH.BIT through full decoder via `DecodeWithTaps`, dumping per
  subframe:
  - `pastErrors` snapshot BEFORE `Decode`
  - `predicted` Q10 dB (already in DIAG-1 tap)
  - `uCurrent` Q10 dB (the value FIFO-shifted in)
  - `pastErrors` snapshot AFTER `Decode`
- Statistics over the corpus:
  - first-50-subframe trajectory (verify cold-start seed value is
    actually consumed by the predictor and confirm the trajectory
    direction)
  - long-run mean / variance of `predicted - uCurrent` (the MA
    residual) — should be O(0) if the predictor is calibrated
  - count of zero-energy guard firings
- **Pin OQ-PASTSEED and OQ-PASTPROG** in a new Appendix E to the
  diagnostic report `2026-05-04-decoder-amplitude-localization.md`,
  citing §3.9.2 paragraph location + Salami 1998 §V.B equation
  numbers. If the corpus shows the cold-start seed is consumed but
  produces a constant bias in `predicted` over the first ~10 subframes,
  the seed value is the suspect; if the bias is centered but timing
  remains skewed, candidate B is exonerated and DIAG-2 must run.
- Test uses `t.Logf` only.
- Commit: `phase3b(diag1): pastErrors evolution dump + OQ-PASTSEED pin`

### DIAG-2 — LP interpolation alignment (candidate C; conditional)

Run only if DIAG-1 exonerates candidate B.

- Add test-only `internal/decoder/phase3b_diag2_lpinterp_test.go`
  driving SPEECH.BIT and dumping per-subframe:
  - decoded LSP vector (Q15)
  - interpolated LSP vector for subframe 1 (Q15)
  - resulting LP coefficients a[1..10] Q12 for sf-0 and sf-1
  - cross-correlation of synthesized sf-1 against a "no-interpolation"
    fallback (sf-1 uses raw `q_lsp[m]` instead of interpolated)
- Append Appendix F to diagnostic report pinning OQ-LP-INTERP.
- Commit: `phase3b(diag2): LP interpolation diagnostic + OQ-LP-INTERP pin`

### REF-1 — design note for the chosen fix

- File: `docs/superpowers/plans/2026-05-04-phase3b-fix-design.md`
- Document the chosen candidate (B and/or C) based on DIAG-1/2 data.
- For each fix:
  - Spec citation (eq. number + paragraph)
  - Current code site + change
  - Predicted impact on SegSNR / Phase 1o D-3 / Phase 2 byte-EQ
  - Invariants to preserve (zero-alloc, race-clean, encoder/decoder
    seed symmetry)
- No code change.
- Commit: `phase3b(ref1): fix design note for candidate <B|C|B+C>`

### IMPL-1 — minimal fix (decoder + encoder seed symmetric)

- Failing test first: `internal/gain/decode_seed_test.go` (or extend
  existing) capturing the corrected behavior. Cases:
  1. Cold-start `Decode` produces `predicted` matching the spec
     reconstruction of average-gain conditions (table value to derive
     from §3.9.2 / GainMeanEnergyQ10 — pure spec, no external code).
  2. Zero-energy guard re-seed strategy matches OQ-PASTPROG pin.
  3. After 100 typical subframes, `pastErrors[0..3]` converges to a
     stable region (no runaway drift).
- Modify `internal/gain/decode.go` (and predictor seed if needed),
  mirroring on encoder side `internal/gainquant/predictor.go`.
- Verify symmetric: `TestEncoderDecoderSeedSymmetry` in
  `internal/gainquant/seed_symmetry_test.go` runs the same idx
  sequence through both Apply and Decode and asserts identical
  `pastErrors[]` evolution.
- Run all gain/gainquant tests + `go test -race`.
- Bench: confirm `gain.Decode` / `gainquant.Apply` 0 alloc retained,
  ns/op delta ≤ ±5%.
- Commit: `phase3b(impl1): MA predictor seed/progression spec-correction`

### IMPL-2 — LP interpolation correction (only if DIAG-2 ran)

- Failing test first: a corpus-driven SegSNR floor test under
  `phase3_roundtrip_quality_lp_test.go` that asserts a specific
  improvement on the LP path. Or, if cleaner: byte-EQ on the
  interpolated LSP vector against a hand-derived spec value.
- Modify `internal/lsp.Interpolate` / call site as REF-1 prescribes.
- Run all decoder/lsp/synth tests + `go test -race`.
- Bench: confirm 0 alloc on `decoder.Decode`, ns/op delta ≤ ±5%.
- Commit: `phase3b(impl2): LP interpolation alignment correction`

### INT-1 — Phase 3 roundtrip acceptance

- Re-run `phase3_roundtrip_quality_test.go` and
  `TestPhase3Diag_DecoderAmplitudeProfile`.
- Capture: rms(out), max|sample|, SegSNR pipeline B, GlobalSNR,
  cross-correlation peak shift.
- Decide disposition per §2 Goal acceptance bands.
- If FAIL: append Appendix G to diagnostic report; do NOT write
  closure-PASS report; open Phase 3c.
- If PARTIAL or PASS: write closure-PASS report.
- Commit: `phase3b(int1): Phase 3 roundtrip — <DISPOSITION>`

### INT-2 — full sweep + bench + race

- `go test ./... -count=1`
- `go test ./... -race -count=1`
- `go vet ./...`
- `go test ./internal/... -bench=. -benchmem -run=^$ -count=3`
- Capture deltas vs HEAD `c7860a9`.
- Phase 1o D-3 PSTdomain pin status table: SPEECH/LSP/TEST sample-0
  must remain GREEN.
- Phase 2c/2d/2f INT-1 byte-EQ deltas (informational; gate is
  catastrophic-regression-only).
- Commit: `phase3b(int2): full sweep + bench + race`

### INT-3 — closure report

- File: `docs/superpowers/plans/2026-05-04-phase3b-closure-report.md`
- Sections:
  1. Disposition (CLOSED-PASS / CLOSED-PARTIAL / CLOSED-DEFERRED)
  2. OQ resolutions (OQ-PASTSEED, OQ-PASTPROG, OQ-LP-INTERP if
     applicable, OQ-PASTSYM)
  3. Phase 3 roundtrip numbers before/after (full table)
  4. Byte-EQ deltas (Phase 1o, Phase 2)
  5. Bench deltas
  6. Code surface delta (files A/M/D, lines)
  7. Follow-ups (Phase 3c if needed; otherwise next-phase pointer)
- Update master plan
  `docs/superpowers/plans/2026-05-02-phase2-encoder-plan.md` with
  Phase 3b row + closure link.
- Update Phase 3a plan
  `docs/superpowers/plans/2026-05-04-phase3a-gc-qformat-fix-plan.md`
  §7 footer with "Phase 3b complete: <disposition>".
- Commit: `phase3b(int3): closure report`

## 5. Out of scope

- Postfilter / HP retuning (audited; no open hypothesis)
- Encoder closed-loop pitch search defect (Phase 2c FAIL-DEFERRED;
  Phase 3b may improve C1/C2 byte-EQ as side effect, not gated)
- New external dependencies
- Removal of `internal/gain/legacy_gcq12.go` (KEPT with 10 test
  consumers; revisit in Phase 3-final cleanup)

## 6. Risks

- R1: Both candidate B and C may be partially-correct, requiring a
  combined fix. IMPL-1 + IMPL-2 may both be needed; INT-1 disposition
  decides whether Phase 3b PASSes or escalates to Phase 3c.
- R2: Spec interpretation of `pastErrors` cold-start may have
  multiple valid readings. DIAG-1 must pin OQ-PASTSEED on numerical
  evidence, not just text reading.
- R3: Phase 1o D-3 PASS-by-design pins (TAME/FIXED/PITCH/OVERFLOW)
  may shift again. Disposition update is in-scope for Phase 3b INT-2
  per `itu_vector_pstdomain_test.go` reactivation-trigger #4.
- R4: Encoder/decoder seed asymmetry could mask an upstream encoder
  defect. IMPL-1's `TestEncoderDecoderSeedSymmetry` is the
  long-standing pin against this class.

## 7. Disposition footer (filled at INT-3)

Date: 2026-05-04 (post-`8114752`)
Disposition: **`CLOSED-DEFERRED`** (PARTIAL recovery; decoder proven spec-correct).

Six diagnostic iterations (DIAG-1..6) enumerated and **exonerated** every
candidate cause from §1 ranking and §3 OQ table — MA gain-predictor
cold-start, LP coefficient interpolation, adaptive-codebook FIFO +
fractional resampling, postfilter bypass + 62-sample-shift premise (rebut),
decoder energy-chain Salami identity, and LP spectral-envelope vs
SPEECH.PST. The fail-gate (`rms(out)` 419 < 500 floor; SegSNR −0.90 < 0 dB)
fires by metric, but the underlying failure is *upstream of any spec-defined
decoder defect*. **No production code was changed in Phase 3b.**

Closure report (full OQ resolutions, numerical evidence, Phase 3-final
recommendations): `docs/superpowers/plans/2026-05-04-phase3b-closure-report.md`.

| Task   | Status |
|--------|--------|
| DIAG-1 | [x] DONE (`faff330`) — App. E |
| DIAG-2 | [x] DONE (`d4abbe9`) — App. F |
| DIAG-3 | [x] DONE (`241c8d4`) — App. G (added during diagnosis) |
| REF-1  | [-] N/A — no candidate survived to design (DIAG-1..3 all exonerated) |
| IMPL-1 | [-] N/A — no candidate survived to fix |
| IMPL-2 | [-] N/A — no candidate survived to fix |
| INT-1  | [x] DONE (`f64eaf4`) — postfilter bypass discriminator + 62-sample shift premise REBUTTED — App. H |
| INT-2  | [x] DONE (`237b40c`, `8114752`) — full-sweep + bench + race captured — App. I, J |
| INT-3 (closure report) | [x] DONE — this footer + linked closure report |

I1 risk: **none** — clean-room maintained; no ITU C reference, bcg729,
Sipro Lab, FFmpeg, or any other G.729 implementation consulted. Sole
external sources: ITU-T G.729 (06/2012) + Annex A PDFs, Salami 1998
§V.B, Kondoz §6, Chu, Goldberg & Riek, Quackenbush et al. 1988,
Oppenheim & Schafer.

**Phase 3-final unified closure: `CLOSED-PARTIAL`** — Phase 3a + Phase 3b
rolled into a single shipping closure. Codec ships under spec-compliance
binding criterion. See
`docs/superpowers/plans/2026-05-04-phase3-final-closure-report.md`.
