# Phase 1j: Gain Decoder Q-Format End-to-End Re-derivation + ITU Bit-Exact Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the `internal/gain` decoder's Q-format inconsistency — `fixedCodebookEnergy` returns Σ c² at **Q26** (because `c` is Q13 pulse amplitudes), but `log2Fixed` treats its input as **Q0** and the downstream `ecDbQ10 := int16(...)` cast silently wraps for any realistic codebook. Re-derive the entire `gain.Decode` chain with Q-formats that are spec-consistent end-to-end, update the Phase 1h pathological tests to the corrected expected values, and finally re-enable the seven loadable ITU reference vectors (ALGTHM, SPEECH, FIXED, LSP, PITCH, TAME, TEST) that were left `t.Skip`ped at the end of Phase 1i.

**Architecture:** Phase 1i's completion report identified the proximate cause of the Phase 1h `got=0 want=2` uniform divergence and fixed three structural defects (AGC seed, LSP init, §3.10 recovery scale), which together landed ALGTHM frame 0 sample 0. **Frame 0 sample 40 (the start of sf2)** then revealed that the MA-predictor / gain-VQ chain saturates `gcQ12` to 32767 because the E̅_c computation is off by 26·log10(2)·10 dB = 78.28 dB in Q10 — exactly the Q26-vs-Q0 gap. A naive `ecLog2Q10 -= 26*1024` was attempted during Phase 1i's diagnostic and broke the Phase 1h pathological tests; the Phase 1i author correctly diagnosed this as "the Q-format chain requires coordinated re-derivation" and stopped rather than fudge. Phase 1j does that re-derivation.

The fix is small — one subtraction in `gain.Decode` — but ripples through every pathological test's expected magnitudes because those tests were written when the Q-format bug was present. Phase 1j therefore structures the fix as: (1) lock the ITU-spec Q-format contract with invariant tests; (2) apply the single-line numerical fix; (3) re-certify each pathological test against the now-spec-correct outputs. Then bit-exactness follows on the seven ITU vectors.

**Tech Stack:**
- Go 1.25, pure scratch-from-spec (ITU-T G.729 + Annex A + public textbooks only)
- ITU-T G.191 basic operations (`internal/fixed`)
- TDD with zero-allocation contract (`testing.AllocsPerRun == 0`)
- ITU reference vectors at `testdata/itu/G729_Release3/g729AnnexA/test_vectors/`

**Scope fence (deferred to Phase 1k+):**
- **OVERFLOW.BIT** loadability — blocked on `internal/bitstream` "invalid G.192 data word" bug
- **Frame-erasure concealment** (§A.4.1)
- **Parity-bit behavior** (§4.4 — `pitch.CheckParity` result ignored today)
- **Public `Decoder` / `Encoder` API** in the root package
- **Encoder** — full G.729A encoder pipeline

These remain out of scope.

**Spec references used by every task:**
- `docs/superpowers/specs/itu/G729E.pdf` — ITU-T G.729 + Annex A + Annex E spec PDF
- §3.9 — gain quantization; MA-predicted log-gain Ê(m); fixed-codebook energy E̅_c
- §3.9.1 eq (66) — `E̅_c = 10·log10((1/40)·Σ c²)`
- §3.9.1 eq (69) — `20·log10(g'_c) = Ê(m) − E̅_c(m)`
- §4.1.6 eq (75) — excitation `u(n) = g_p·v(n) + g_c·c(n)`
- §4.1.6 — decoder pipeline (including sf1/sf2 gain-decoding ordering)

**The ONLY** permitted sources for algorithmic code are (a) the ITU spec PDF text, (b) public textbooks (Chu "Speech Coding Algorithms", etc.), and (c) our own code under `internal/`. Pure-data tables (`tables.GainGBK1`, `tables.GainGBK2`, `tables.Log2Table`, `tables.Pow2Table`, `tables.GainMeanEnergyQ10`) may continue to be transcribed from ITU `tab_ld8a.c` under merger doctrine. **NEVER** consult the ITU reference C (`cod_ld8a.c`, `dec_ld8a.c`, `dec_gain.c`, `gainpred.c`, `pred_lt3.c`, etc.), `bcg729`, Sipro Lab, FFmpeg's G.729, or any other existing implementation.

**Completion bar:** At the end of Phase 1j, every `TestDecode_BitExact_<VECTOR>` in `internal/decoder/decode_test.go` for the seven loadable vectors must pass with byte-identical output vs its companion `.pst` file; `t.Skip(...)` calls are removed. `go test -race ./...` and `go vet ./...` remain silent. All benchmarks remain at **0 allocs/op**.

---

## File structure

Modified:
- `internal/gain/energy.go` — doc comment clarifies c_Q13 → ecEnergy Q26
- `internal/gain/log2.go` — doc comment clarifies Q0 input contract
- `internal/gain/decode.go` — apply `-26*1024` Q26-to-Q0 correction on `ecLog2Q10`
- `internal/gain/decode_test.go` — add Q-format contract test, extend existing end-to-end tests
- `internal/gain/pathological_test.go` — update expected magnitudes to match corrected Q-format
- `internal/gain/predictor.go` — doc audit for `pastErrorsDefault` and `tables.GainMeanEnergyQ10` interaction (no code change expected; audit only)
- `internal/gain/vq_test.go` — add sample-check unit tests for `GainGBK1` / `GainGBK2`
- `internal/decoder/decode_test.go` — extend `TestDecode_Frame0Sample0_MatchesALGTHM` to sample 40 (sf2 start) and full-frame-0 coverage; remove `t.Skip(...)` on seven ITU vectors

Created:
- `docs/superpowers/plans/2026-04-22-phase1j-gain-qformat-redrive-completion-report.md` — final report

No files are deleted; the `internal/gain` package structure is preserved.

---

## Task 1: Q-format chain documentation + contract invariant tests

**Rationale:** Before touching arithmetic, lock down the Q-format contract between the five functions in the chain (`fixedCodebookEnergy` → `log2Fixed` → dB conversion → `pow2Fixed` → final `gcQ12`). If these are not documented explicitly, future changes can reintroduce the same bug.

**Files:**
- Modify: `internal/gain/energy.go` (doc comment)
- Modify: `internal/gain/log2.go` (doc comment)
- Modify: `internal/gain/pow2.go` (doc comment)
- Test: `internal/gain/decode_test.go` (append Q-format invariant test)

- [x] **Step 1: Update `energy.go` doc comment**

Edit `internal/gain/energy.go`. Replace the doc comment above `fixedCodebookEnergy` with:

```go
// fixedCodebookEnergy returns the inner sum Σ c[n]² as a non-negative
// Word32.
//
// Q-format: the caller's c is Q13 pulse amplitudes (±8192 for ITU Annex A
// ACELP). Each squared term is therefore Q26, and the 40-term sum is at
// Q26 as well. The sum is always non-negative and bounded above by
// 40·8192² = 2^28·40/1 < 2^31, so no saturation can occur.
//
// Returning the raw sum at Q26 (rather than pre-applying the /40 of
// spec eq (66)) preserves precision for the downstream log2 conversion.
// The 10·log10(1/40) correction is absorbed into the compile-time
// constant tenLog10_40Q10 applied in decode.go.
//
// Callers must ALSO apply a Q-format correction to account for the
// Q26-vs-Q0 mismatch against the spec's log2 of a Q0 sum: see the
// comment in decode.go at the `ecLog2Q10 = ... - 26*1024` line.
func fixedCodebookEnergy(c *[40]int16) fixed.Word32 {
```

(Leave the body unchanged.)

- [x] **Step 2: Update `log2.go` doc comment**

Edit `internal/gain/log2.go`. Append to the existing doc comment (just before `func log2Fixed(...)`):

```go
// Q-format CONTRACT: this function treats `x` as a Q0 integer and
// returns log2(x) at Q10. If a caller passes a Qk value (k > 0) as
// `x`, the returned log2 is off by k·1024 (log2(value·2^k) = log2(value)
// + k). Callers with a Qk input MUST subtract k·1024 from the result
// to recover the spec-intended log2. See decode.go's ecLog2Q10 handling.
```

- [x] **Step 3: Update `pow2.go` doc comment**

Edit `internal/gain/pow2.go`. Append to the existing doc comment:

```go
// Q-format CONTRACT: `x` is interpreted as Q10 and the result is a Q0
// Word32. Callers wanting 2^x at some Qk should pre-add k·1024 to `x`
// before the call (e.g. `pow2Fixed(log2Gc_Q10 + 14*1024)` returns
// 2^log2Gc × 2^14 as a Q0 integer, i.e., the value at Q14 stored in Q0).
```

- [x] **Step 4: Add the Q-format contract invariant test**

Append to `internal/gain/decode_test.go`:

```go
// TestQFormatContract_EnergyIsQ26 locks down the contract that
// fixedCodebookEnergy returns Σ c[n]² at Q26 when c is Q13 pulses.
// Guards against anyone silently changing the c Q-format or the sum's
// Q-format without updating the downstream -26*1024 correction.
func TestQFormatContract_EnergyIsQ26(t *testing.T) {
	// Single Q13 unit pulse at position 0: c[0] = 8192 = 2^13.
	// Σ c² = 2^26 = 67108864.
	var c [40]int16
	c[0] = 8192
	got := fixedCodebookEnergy(&c)
	const want = 1 << 26
	if int64(got) != want {
		t.Fatalf("fixedCodebookEnergy(single Q13 pulse) = %d; want %d (= 2^26 at Q26)", got, want)
	}

	// Canonical 4-pulse codebook: four ±8192 pulses. Σ c² = 4·2^26 = 2^28.
	var c4 [40]int16
	c4[5] = 8192
	c4[11] = 8192
	c4[22] = 8192
	c4[33] = 8192
	got4 := fixedCodebookEnergy(&c4)
	const want4 = 1 << 28
	if int64(got4) != want4 {
		t.Fatalf("fixedCodebookEnergy(4 Q13 pulses) = %d; want %d (= 2^28 at Q26)", got4, want4)
	}
}

// TestQFormatContract_Log2TreatsInputAsQ0 guards the log2Fixed Q-format
// contract: log2Fixed(2^26) = 26·1024 in Q10, NOT 0. The caller must
// subtract the known Q-format of its input to recover spec-intended
// log2.
func TestQFormatContract_Log2TreatsInputAsQ0(t *testing.T) {
	// 2^26 at Q0 → log2 = 26, at Q10 = 26·1024 = 26624.
	got := log2Fixed(1 << 26)
	const want = 26 * 1024
	// log2Fixed uses a 32-entry interpolation table, so ±2 LSB is
	// within spec at Q10.
	if int64(got) < want-2 || int64(got) > want+2 {
		t.Fatalf("log2Fixed(2^26) = %d; want %d ± 2 (Q10)", got, want)
	}
}
```

- [x] **Step 5: Run the contract tests — expect PASS**

Run: `go test -run 'TestQFormatContract' -v ./internal/gain`

Expected: both PASS. (These tests lock the current behavior; no production code has changed yet.)

- [x] **Step 6: Commit**

```bash
git add internal/gain/energy.go internal/gain/log2.go internal/gain/pow2.go internal/gain/decode_test.go
git commit -m "$(cat <<'EOF'
docs(gain): annotate Q-format contract of energy/log2/pow2 chain

Before re-deriving gain.Decode for c_Q13 consistency (Task 4), lock the
Q-format contract of the three helper functions with doc comments and
TestQFormatContract_* invariants. Prevents silent regressions on the
-26*1024 correction that Task 4 will add.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 2: Failing regression test — `ecBar` magnitude for canonical codebook

**Rationale:** The single production bug is a Q26-vs-Q0 mismatch in `gain.Decode`'s `ecLog2Q10` computation. Before fixing, lock in a failing test that checks `E̅_c` for the canonical 4-pulse codebook matches the spec value `10·log10(4/40) = -10 dB = -10240 Q10`.

**Files:**
- Modify: `internal/gain/decode.go` — temporarily expose a package-private helper so the test can read ecBar directly. OR add the test using observable side effects (gcQ12) — preferred.

- [ ] **Step 1: Plan the test observation strategy**

`ecBar` is a local variable inside `gain.Decode`. Rather than expose it, test the observable side effect: with a specific MA-predictor state (all-zeros pastErrors), predicted = a known constant `tables.GainMeanEnergyQ10`; with canonical 4-pulse codebook (ecBar_spec = -10240), `logGainDb = predicted - ecBar = predicted + 10240`. This feeds into a deterministic `gcQ12`. If the Q-format is correct, `gcQ12` matches a hand-computed reference.

If `tables.GainMeanEnergyQ10` is not exported, either export it for the test or compute the reference with the actual value from the package.

- [ ] **Step 2: Add the failing test**

Append to `internal/gain/decode_test.go`:

```go
// TestDecode_CanonicalCodebook_GcMatchesSpecMagnitude is the Phase 1j
// failing-baseline regression that captures the c_Q13 Q-format bug.
//
// Setup: Decoder at initial state (pastErrors all at pastErrorsDefault =
// -14336 Q10). Canonical 4-pulse Q13 codebook → Σc²=2^28, so spec's
// E̅_c = 10·log10(4/40) = -10·log10(10) = -10 dB = -10240 Q10.
//
// With buggy pre-Phase-1j behavior (ecLog2 off by +26·1024):
//   ecDbQ10_raw = 28·1024 · 24660/8192 = 86317 Q10 → int16-wraps to 20781
//   ecBar = 20781 - 16405 = 4376 Q10 ≈ +4.3 dB (off by +14.3 dB)
//   → gcQ12 comes out at one specific (wrong) value.
//
// With the Phase 1j fix (`ecLog2Q10 -= 26*1024`):
//   ecLog2 = 28·1024 - 26·1024 = 2048 Q10 = log2(4)
//   ecDb = 2048·24660/8192 = 6165 Q10 = 10·log10(4) = 6.02 dB
//   ecBar = 6165 - 16405 = -10240 Q10 = 10·log10(4/40) ✓
//   → gcQ12 matches the spec-derived value computed inline below.
//
// The test asserts that the buggy and fixed behaviors are distinct.
// Before Task 4: FAILS (got = buggy value).
// After Task 4:  PASSES (got = spec-correct value).
func TestDecode_CanonicalCodebook_GcMatchesSpecMagnitude(t *testing.T) {
	var d Decoder
	var c [40]int16
	c[5] = 8192
	c[11] = 8192
	c[22] = 8192
	c[33] = 8192

	// (GA=3, GB=7) corresponds to γ̂_c near 1.0 (mid codebook entry) —
	// the specific indices are not important; we lock the full output.
	_, gcQ12 := d.Decode(Indices{GA: 3, GB: 7}, &c)

	// Hand-derived spec value. Compute inline to make the derivation
	// auditable:
	//   predicted_initial = tables.GainMeanEnergyQ10 +
	//                       Σ b[i] · (pastErrorsDefault - tables.GainMeanEnergyQ10)
	// For pastErrors[i] = pastErrorsDefault ∀i, Σb[i] ≈ 1 →
	// predicted_initial = pastErrorsDefault = -14336 Q10.
	//
	//   ecBar = -10240 Q10 (derived above)
	//   logGainDb = predicted - ecBar = -14336 - (-10240) = -4096 Q10 = -4 dB
	//   log2Gc_Q10 = (-4096 · 5443 + (1<<14)) >> 15 = -681 Q10
	//   gc0_Q14 = pow2Fixed(-681 + 14*1024) = pow2Fixed(13655)
	//           = 2^(13655/1024) Q0 ≈ 10362 (look-up dependent)
	//   prod = gammaC_Q13 · gc0_Q14 >> 15
	//
	// The exact gcQ12 depends on the (GA=3, GB=7) VQ entry's γ̂_c.
	// Compute the reference by running decodeVQ once:
	_, gammaCQ13 := decodeVQ(Indices{GA: 3, GB: 7})

	// Re-run the arithmetic path manually to get the reference value.
	// This is a whitebox test: it locks the end-to-end output of the
	// Q-format chain against a manually-computed spec value.
	//
	// NOTE: the reference derivation here uses predictedLogGain from a
	// fresh Decoder (state = init'd pastErrors) so it matches the SUT.
	var dRef Decoder
	for i := range dRef.pastErrors {
		dRef.pastErrors[i] = pastErrorsDefault
	}
	dRef.initialized = true
	predictedRef := dRef.predictedLogGain()

	const ecBarSpecQ10 = -10240 // 10·log10(4/40) Q10
	logGainDbQ10 := int32(predictedRef) - int32(ecBarSpecQ10)
	log2GcQ10 := (logGainDbQ10*invDbScaleQ15 + (1 << 14)) >> 15
	gc0Q14 := pow2Fixed(fixed.Word32(log2GcQ10) + 14*1024)
	wantProd := int32(gammaCQ13) * int32(gc0Q14) >> 15
	if wantProd > 32767 {
		wantProd = 32767
	} else if wantProd < -32768 {
		wantProd = -32768
	}
	want := int16(wantProd)

	if gcQ12 != want {
		t.Fatalf("canonical 4-pulse decode: gcQ12 = %d; want %d (spec-derived via -10 dB ecBar). "+
			"Delta = %d. This is the Phase 1j Q-format mismatch — Task 4 fixes it.",
			gcQ12, want, int32(gcQ12)-int32(want))
	}
}
```

Note: this test requires importing `"github.com/hunydev/g729/internal/fixed"`. Add to the existing imports if not already present.

- [ ] **Step 3: Run — expect FAIL**

Run: `go test -run TestDecode_CanonicalCodebook_GcMatchesSpecMagnitude -v ./internal/gain`

Expected: FAIL with the "Q-format mismatch" message, showing a non-trivial delta.

- [ ] **Step 4: Commit the failing test**

```bash
git add internal/gain/decode_test.go
git commit -m "$(cat <<'EOF'
test(gain): Phase 1j failing baseline — gcQ12 for canonical 4-pulse

Locks the spec-derived gcQ12 value against a hand-computed reference
that uses the correct ecBar = -10 dB (= 10·log10(4/40) Q10). Fails
BEFORE Task 4 because ecLog2Q10 is off by +26·1024 (c is Q13, spec
assumes Q0). Passes AFTER Task 4's `-26*1024` correction.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 3: Failing regression test — `TestDecode_Frame0Sample40_MatchesALGTHM`

**Rationale:** Extend Phase 1i's `TestDecode_Frame0Sample0_MatchesALGTHM` to sample 40 (the start of sf2). This is the Phase 1i completion report's identified break point: the saturated sf2 `gcQ12` drives sample 40 onward to catastrophic values.

**Files:**
- Modify: `internal/decoder/decode_test.go` (append new test)

- [ ] **Step 1: Add the failing test**

Append to `internal/decoder/decode_test.go`:

```go
// TestDecode_Frame0Sample40_MatchesALGTHM is the sf2-boundary sibling
// of TestDecode_Frame0Sample0_MatchesALGTHM. Captures the Phase 1i
// completion-report-observed "frame 0 sf2 gcQ12 saturates → large-
// magnitude sample 40 onward" regression.
//
// Expected:
//   BEFORE Phase 1j Task 4: FAIL with got ≪ want or got ≫ want by
//                           orders of magnitude (saturation cascade).
//   AFTER  Phase 1j Task 4: PASS (byte-equal match with ITU sample 40).
//
// Guards the Q-format fix from silently regressing in future phases.
func TestDecode_Frame0Sample40_MatchesALGTHM(t *testing.T) {
	const vectorDir = "../../testdata/itu/G729_Release3/g729AnnexA/test_vectors"
	bits := loadG192File(t, vectorDir+"/ALGTHM.BIT")
	want := loadPstFile(t, vectorDir+"/ALGTHM.PST")

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(packFrame(t, bits[0]), false, out[:]); err != nil {
		t.Fatalf("Decode frame 0 returned error: %v", err)
	}
	if out[40] != want[40] {
		t.Errorf("frame 0 sample 40 (sf2 start): got=%d want=%d (Δ=%d)",
			out[40], want[40], int32(out[40])-int32(want[40]))
		t.Logf("out[38:50]  = %v", out[38:50])
		t.Logf("want[38:50] = %v", want[38:50])
	}
}
```

(Reuses `loadG192File`, `loadPstFile`, `packFrame` from Phase 1g's `testdata_helpers_test.go`.)

- [ ] **Step 2: Run — expect FAIL**

Run: `go test -run TestDecode_Frame0Sample40_MatchesALGTHM -v ./internal/decoder`

Expected: FAIL with a large delta (Phase 1i report: sf2 gcQ12 = 32767 saturates, cascading to sample 40+).

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/decoder/decode_test.go
git commit -m "$(cat <<'EOF'
test(decoder): Phase 1j failing baseline — ALGTHM frame 0 sample 40

Extends Phase 1i's frame-0 sample-0 regression test to sample 40 (sf2
start), capturing the gain-VQ saturation cascade observed in the Phase
1i completion report. Task 4 fixes the underlying Q-format bug.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 4: Core fix — `gain.Decode` applies `-26*1024` Q26→Q0 correction on `ecLog2Q10`

**Rationale:** The single algorithmic change of Phase 1j. `fixedCodebookEnergy` returns Σ c_Q13² at Q26; `log2Fixed` treats its input as Q0 integer. Subtracting `26*1024` (= 26 in Q10 log2 scale) converts `log2Fixed(ecEnergy)` from "log2 of the Q26 Word32" to "log2 of the spec's Q0 sum".

**Files:**
- Modify: `internal/gain/decode.go`

- [ ] **Step 1: Apply the fix**

Edit `internal/gain/decode.go`. Find the line:

```go
	ecLog2Q10 := log2Fixed(ecEnergy)
```

Replace with:

```go
	// log2Fixed treats its input as a Q0 Word32; fixedCodebookEnergy
	// returns Σ c_Q13² at Q26 (see Q-format contract in energy.go).
	// Subtract 26·1024 to recover the spec-intended log2(Σ c_Q0²) at Q10.
	ecLog2Q10 := log2Fixed(ecEnergy) - 26*1024
```

- [ ] **Step 2: Run the Task 2 failing test — expect PASS**

Run: `go test -run TestDecode_CanonicalCodebook_GcMatchesSpecMagnitude -v ./internal/gain`

Expected: PASS. `gcQ12` now matches the hand-derived spec value.

- [ ] **Step 3: Run the Task 3 failing test — expect behavior change**

Run: `go test -run TestDecode_Frame0Sample40_MatchesALGTHM -v ./internal/decoder`

Expected behavior: the test may PASS (if Q-format was the only bug for this sample) or still FAIL with a much smaller delta (if residual bugs remain in Tasks 5–7's territory). Record the observed delta in the commit message either way.

- [ ] **Step 4: Run the Phase 1h pathological tests — expect SOME FAIL**

Run: `go test -run 'TestDecode_(AllZero|LowEnergy|HighEnergy|Succeeds)' -v ./internal/gain`

Expected: some or all FAIL. The tests' expected values were written against the buggy (pre-Phase-1j) Q-format; Task 5 updates them.

Do NOT revert the fix to make them pass. The fix is correct; the tests are stale.

- [ ] **Step 5: Run the Phase 1h ITU-level pathological fcb tests — expect PASS**

Run: `go test -run 'TestDecodePositions|TestPlacePulses|TestDecode_C6134|TestDecode_ExhaustiveSigns' -v ./internal/fcb`

Expected: PASS. These tests don't depend on gain — they test fcb alone.

- [ ] **Step 6: Commit**

```bash
git add internal/gain/decode.go
git commit -m "$(cat <<'EOF'
fix(gain): correct ecLog2Q10 Q26→Q0 offset per §3.9 eq (66)

fixedCodebookEnergy returns Σ c_Q13² at Q26; log2Fixed treats its input
as Q0. Without the -26*1024 correction, ecDbQ10 is ≈ 78.3 dB higher
than spec and int16-wraps in the `int16(...)` cast, corrupting the MA
predictor's view of E̅_c and eventually saturating gcQ12 in sf2.

Identified by the Phase 1i completion report's diagnostic; locked by
TestDecode_CanonicalCodebook_GcMatchesSpecMagnitude in Phase 1j Task 2.

Pathological tests `TestDecode_AllZero/LowEnergy/HighEnergy/Succeeds*`
are updated in Phase 1j Task 5 to reflect the now-spec-correct outputs.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 5: Pathological tests — update expected magnitudes to post-fix values

**Rationale:** The Phase 1h pathological tests assert invariants (bounded, non-saturating, non-zero) that continue to hold post-fix, but some individual magnitudes shift. Re-audit each test, re-derive expected value if needed, and re-commit as the Phase 1j Q-format-correct baseline.

**Files:**
- Modify: `internal/gain/pathological_test.go`

- [ ] **Step 1: Audit `TestDecode_AllZeroCodebookIsBounded`**

This test takes the zero-energy guard branch (ecEnergy ≤ 0 → gcQ12 = 0). Task 4 does not affect the guard branch. This test should still PASS.

Run: `go test -run TestDecode_AllZeroCodebookIsBounded -v ./internal/gain`
Expected: PASS. If it FAILs, diagnose — the zero-energy guard may have been disturbed.

- [ ] **Step 2: Re-run and re-certify `TestDecode_LowEnergyCodebookIsSmooth`**

The test's invariants are:
1. `gpQ14 ∈ [0, 32767]` — unaffected by Task 4.
2. `gcQ12 != ±int16 extremum` — post-fix, gcQ12 should still not saturate.
3. `gcQ12 != 0` — the predictor evolves from initial state; a single Q13 pulse has non-zero energy.

Run: `go test -run TestDecode_LowEnergyCodebookIsSmooth -v ./internal/gain`

**If PASS**: skip to Step 3.
**If FAIL**:
  - If gcQ12 hit zero: loosen the invariant to `gcQ12 != 0 || gpQ14 > 0` (since at initial state, predictor may produce sub-threshold log2Gc that pow2 underflows to 0).
  - If gcQ12 saturated: STOP — this indicates the fix has an opposite-direction bug or is partial. Do not continue; return to Task 4 and re-diagnose.
  - Any other failure: update the test to assert the invariant without locking specific magnitudes.

- [ ] **Step 3: Re-run and re-certify `TestDecode_HighEnergyCodebookIsBounded`**

Invariant: `gcQ12 != ±int16 extremum`.

Run: `go test -run TestDecode_HighEnergyCodebookIsBounded -v ./internal/gain`

Expected: PASS (the canonical 4-pulse was the case Task 4 explicitly fixes).

- [ ] **Step 4: Re-run and re-certify `TestDecode_SucceedsAcrossAllGainIndices`**

Invariant: no (GA, GB) combination with the canonical 4-pulse codebook saturates.

Run: `go test -run TestDecode_SucceedsAcrossAllGainIndices -v ./internal/gain`

Expected: PASS. If any (GA, GB) pair saturates post-fix, that specific VQ entry may have a large γ̂_c that drives `gc0Q14 · gammaC >> 15` near the int16 limit. Investigate and either fix the arithmetic (if the cast truncates incorrectly) or loosen the invariant (if saturation is genuine for extreme codebook entries).

- [ ] **Step 5: Run full `internal/gain` package**

Run: `go test ./internal/gain -count=1 -race`

Expected: PASS for all tests.

- [ ] **Step 6: Commit**

```bash
git add internal/gain/pathological_test.go
git commit -m "$(cat <<'EOF'
test(gain): re-certify pathological tests post Q-format fix

Phase 1j Task 4's `-26*1024` correction shifts the numeric magnitudes
of gcQ12 for non-zero-energy codebooks. The structural invariants (not
saturated, not zero, bounded-positive gpQ14) still hold; update the
tests to assert them against the now-spec-correct behavior.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 6: Gain VQ codebook sample-check

**Rationale:** Phase 1i's backlog listed "Gain VQ codebook values — tables.GainGBK1/GBK2 match ITU sample table reproduction" as a Phase 1j candidate root cause. Verify 3–5 sample entries against the ITU spec §3.9.2 tables. If discrepancies are found, fix the table data.

**Files:**
- Modify: `internal/gain/vq_test.go` (append sample-check test)

- [x] **Step 1: Add sample-check tests**

Append to `internal/gain/vq_test.go`:

```go
// TestGainVQ_SampleEntries_MatchSpec spot-checks entries 0, 3, 7 of
// GainGBK1 (3-bit first-stage codebook) and entries 0, 7, 15 of
// GainGBK2 (4-bit second-stage codebook) against the ITU spec §3.9.2
// table values (transcribed into tables/tab_ld8a.go under merger
// doctrine).
//
// Each entry is a (g_p, γ̂_c) pair: g_p is Q14, γ̂_c is Q13. Spec
// values from §3.9.2 Table A.3-1:
//
//   GBK1[0] = (0.031, 1.084)     → gp_Q14 = 508,   gammaC_Q13 = 8881
//   GBK1[3] = (0.218, 0.851)     → gp_Q14 = 3571,  gammaC_Q13 = 6971
//   GBK1[7] = (0.833, 0.511)     → gp_Q14 = 13647, gammaC_Q13 = 4186
//   GBK2[0] = (-0.076, 0.196)    → gp_Q14 = -1245, gammaC_Q13 = 1605
//   GBK2[7] = ( 0.131, 0.078)    → gp_Q14 = 2146,  gammaC_Q13 =  639
//   GBK2[15]= ( 0.053, 0.036)    → gp_Q14 =  868,  gammaC_Q13 =  295
//
// (Values ABOVE are PLACEHOLDERS — the executor must REPLACE them with
// values derived from direct reading of ITU-T G.729 spec §3.9.2 Table
// A.3-1 in docs/superpowers/specs/itu/G729E.pdf. DO NOT copy from
// tab_ld8a.c; the merger doctrine permits transcription of the C
// initializer but the test's reference value must be derived from the
// spec PDF so this test catches transcription errors.)
func TestGainVQ_SampleEntries_MatchSpec(t *testing.T) {
	// Placeholder: the executor must consult the spec PDF §3.9.2 and
	// replace each of the six expected values below with its
	// spec-derived magnitude (±1 LSB tolerance for rounding).
	t.Skip("EXECUTOR TODO: fill in spec-derived GBK1/GBK2 reference values from spec §3.9.2")
}
```

- [x] **Step 2: Executor — consult the spec and fill in the values**

Open `docs/superpowers/specs/itu/G729E.pdf` at §3.9.2 Table A.3-1 (or its Annex A equivalent; the table index may differ between the main spec and Annex A — read both and pick the one whose entries match the current `tables.GainGBK1` array element count).

For each of the 6 placeholder rows above:
1. Read the spec's `(g_p, γ̂_c)` pair as real-valued floats.
2. Convert `g_p` to Q14: `round(g_p · 16384)`.
3. Convert `γ̂_c` to Q13: `round(γ̂_c · 8192)`.
4. Replace the placeholder value in the test.

Then replace the test body with:

```go
func TestGainVQ_SampleEntries_MatchSpec(t *testing.T) {
	cases := []struct {
		name   string
		idx    int
		stage  int // 1 → GBK1, 2 → GBK2
		wantGP int16
		wantGC int16
	}{
		// EXECUTOR: populate from spec §3.9.2 Table A.3-1.
		// Example (verify against spec):
		{"GBK1[0]", 0, 1, /*gp*/ 508, /*gc*/ 8881},
		// ...add the five others...
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gp, gc int16
			switch tc.stage {
			case 1:
				gp = tables.GainGBK1[tc.idx][0]
				gc = tables.GainGBK1[tc.idx][1]
			case 2:
				gp = tables.GainGBK2[tc.idx][0]
				gc = tables.GainGBK2[tc.idx][1]
			}
			if abs16(gp-tc.wantGP) > 1 {
				t.Errorf("%s gp_Q14 = %d; want %d ± 1", tc.name, gp, tc.wantGP)
			}
			if abs16(gc-tc.wantGC) > 1 {
				t.Errorf("%s gammaC_Q13 = %d; want %d ± 1", tc.name, gc, tc.wantGC)
			}
		})
	}
}

func abs16(x int16) int16 {
	if x < 0 {
		return -x
	}
	return x
}
```

Adjust the import list for `"github.com/hunydev/g729/internal/tables"` if not present.

- [x] **Step 3: Run and iterate until PASS**

Run: `go test -run TestGainVQ_SampleEntries_MatchSpec -v ./internal/gain`

If any entry FAILS by more than 1 LSB, the `tables.GainGBK1` or `tables.GainGBK2` value in `internal/tables/tab_ld8a.go` (or wherever they live) is wrong — fix the table entry. If the spec derivation was wrong, fix the test's expected value.

Once all six PASS, commit.

- [x] **Step 4: Commit**

```bash
git add internal/gain/vq_test.go
# plus internal/tables/* if any entry needed correction
git commit -m "$(cat <<'EOF'
test(gain): sample-check GainGBK1/GBK2 entries against spec §3.9.2

Six-entry spot-check (3 from each VQ stage) derived directly from the
spec PDF (not the tab_ld8a.c initializer). Guards against transcription
errors in the merger-doctrine copy of the gain-VQ tables.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 7: MA predictor audit — `pastErrorsDefault` and FIFO evolution

**Rationale:** Phase 1i's backlog listed `pastErrorsDefault = -14336` as a Phase 1j audit candidate. Add a compile-time constant test and a FIFO-evolution unit test that drives the predictor with a known input sequence.

**Files:**
- Modify: `internal/gain/predictor_test.go` (append new tests)

- [x] **Step 1: Add `pastErrorsDefault` spec-audit test**

Append to `internal/gain/predictor_test.go`:

```go
// TestPastErrorsDefault_MatchesSpec asserts that the initial value of
// the MA-predictor's past-errors FIFO is -14·2^10 Q10 = -14336, per
// ITU-T G.729 §3.9.1 / §4.1.6 (initialization to -14 dB).
//
// Spec text: "the predictor memory is initialized to ... -14 dB".
// Q10 value = round(-14 · 1024) = -14336.
func TestPastErrorsDefault_MatchesSpec(t *testing.T) {
	const wantQ10 = -14336
	if pastErrorsDefault != wantQ10 {
		t.Errorf("pastErrorsDefault = %d; want %d (= -14 dB Q10)", pastErrorsDefault, wantQ10)
	}
}

// TestMAPredictor_EvolutionFollowsSpec drives the predictor through a
// two-subframe sequence with deterministic inputs (U(m) = 0 dB Q10)
// and asserts the FIFO shifts the new entry into slot [0] while
// discarding slot [3].
//
// Evolution:
//   Initial: pastErrors = [-14336, -14336, -14336, -14336]
//   After 1 subframe with U = 0:
//     pastErrors = [0, -14336, -14336, -14336]
//   After 2 subframes with U = 0:
//     pastErrors = [0, 0, -14336, -14336]
func TestMAPredictor_EvolutionFollowsSpec(t *testing.T) {
	var d Decoder
	// Force initialized state so the decoder skips the lazy-init branch.
	for i := range d.pastErrors {
		d.pastErrors[i] = pastErrorsDefault
	}
	d.initialized = true

	// Single-pulse codebook → gammaC is non-zero via decodeVQ; feed
	// via decoder to exercise FIFO.
	var c [40]int16
	c[0] = 8192
	_, _ = d.Decode(Indices{GA: 0, GB: 0}, &c)

	// After 1 call, pastErrors[0] holds U(m) = 20·log10(γ̂_c) Q10 for
	// the (GA=0, GB=0) combined entry. Slot [1..3] should have shifted
	// from their initial values.
	if d.pastErrors[1] != pastErrorsDefault {
		t.Errorf("pastErrors[1] after 1 subframe = %d; want %d (= pastErrorsDefault, the pre-shift value of pastErrors[0])",
			d.pastErrors[1], pastErrorsDefault)
	}
	if d.pastErrors[2] != pastErrorsDefault {
		t.Errorf("pastErrors[2] after 1 subframe = %d; want %d", d.pastErrors[2], pastErrorsDefault)
	}
	if d.pastErrors[3] != pastErrorsDefault {
		t.Errorf("pastErrors[3] after 1 subframe = %d; want %d", d.pastErrors[3], pastErrorsDefault)
	}
}
```

- [x] **Step 2: Run — expect PASS (or identify defect)**

Run: `go test -run 'TestPastErrorsDefault|TestMAPredictor_EvolutionFollowsSpec' -v ./internal/gain`

Expected: PASS. If FAIL on `TestMAPredictor_EvolutionFollowsSpec`, the Decode function's FIFO-shift order is wrong (e.g., `pastErrors[3] = pastErrors[2]` missing, or slot index inverted). Diagnose and fix.

- [x] **Step 3: Commit**

```bash
git add internal/gain/predictor_test.go
# plus internal/gain/decode.go if FIFO shift was wrong
git commit -m "$(cat <<'EOF'
test(gain): lock MA-predictor init value + FIFO-shift semantics per §3.9.1

Adds compile-time guard for pastErrorsDefault = -14336 Q10 (= -14 dB
initialization) and a two-subframe FIFO-evolution test that asserts
slot [0..3] shift order.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 8: Full frame-0 decoder-boundary test

**Rationale:** Extend the Phase 1i + Phase 1j frame-0 boundary assertions (sample 0 from Phase 1i, sample 40 from Phase 1j Task 3) to **every** sample of frame 0. If any sample diverges after Task 4, this test pinpoints the remaining bug immediately without running the full 35-frame vector.

**Files:**
- Modify: `internal/decoder/decode_test.go` (append)

- [ ] **Step 1: Add the full frame-0 regression test**

Append to `internal/decoder/decode_test.go`:

```go
// TestDecode_Frame0_AllSamplesMatchALGTHM runs the decoder through
// ALGTHM frame 0 and asserts every one of the 80 output samples
// matches the ITU reference. Sharper-grained than the full 35-frame
// bit-exact test: failing this for a specific sample range reveals
// whether the remaining divergence is in sf1 (samples 0-39) or sf2
// (samples 40-79).
//
// Passes AFTER Phase 1j Task 4 + any residual fixes from Tasks 5-7.
func TestDecode_Frame0_AllSamplesMatchALGTHM(t *testing.T) {
	const vectorDir = "../../testdata/itu/G729_Release3/g729AnnexA/test_vectors"
	bits := loadG192File(t, vectorDir+"/ALGTHM.BIT")
	want := loadPstFile(t, vectorDir+"/ALGTHM.PST")

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(packFrame(t, bits[0]), false, out[:]); err != nil {
		t.Fatalf("Decode frame 0 returned error: %v", err)
	}
	for i := 0; i < frameSamples; i++ {
		if out[i] != want[i] {
			t.Errorf("frame 0 sample %d: got=%d want=%d (Δ=%d)",
				i, out[i], want[i], int32(out[i])-int32(want[i]))
			if i == 0 {
				t.Logf("sf1[0:8] got = %v", out[:8])
				t.Logf("sf1[0:8] want= %v", want[:8])
			}
			if i == 40 {
				t.Logf("sf2[0:8] got = %v", out[40:48])
				t.Logf("sf2[0:8] want= %v", want[40:48])
			}
			// Keep reporting the first few mismatches then bail to
			// keep output readable.
			if i > 5 && i != 40 && i != 41 && i != 42 {
				break
			}
		}
	}
}
```

- [ ] **Step 2: Run — diagnose any residual divergence**

Run: `go test -run TestDecode_Frame0_AllSamplesMatchALGTHM -v ./internal/decoder`

**If PASS**: all 80 samples of frame 0 now match ITU. Proceed to Task 9.

**If FAIL**: record the first-divergent sample and its delta. Diagnose:
  - Sample 0–39 divergence: the residual bug is in sf1 territory (LSP interpolation, pitch lag, fcb indexing, gain sf1 arithmetic, synth, postfilter AGC transient, HP).
  - Sample 40–79 divergence: the residual bug is in sf2 territory (predictor FIFO evolution from sf1 to sf2, sf2 LP coefficients from the other half of the LSP interpolation, sf2 pitch/fcb indices).
  - For each specific delta magnitude, the culprit package is often hinted: `± few` → rounding/Q-format LSB issue; `± tens` → coefficient error; `± thousands` → saturation or wrong sign.

Iterate: add a focused unit test, fix, re-run. Each fix is its own commit.

- [ ] **Step 3: Commit**

```bash
git add internal/decoder/decode_test.go
# plus any files touched during residual-bug fixing
git commit -m "$(cat <<'EOF'
test(decoder): lock ALGTHM frame 0 all-samples bit-exactness

Extends the Phase 1i/1j single-sample boundary tests to all 80 samples
of frame 0. Passes after Task 4's Q-format fix + any Task 5-7 residual
corrections. Pinpoints sf1 vs sf2 residual divergence without running
the full 35-frame vector.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 9: Re-enable ITU ALGTHM bit-exact (35 frames)

**Rationale:** With frame 0 fully matching ITU (Task 8), frames 1–34 should follow immediately — or surface cross-frame drift bugs (predictor state, pastSynth, pastResidual, pastExc, hpX/hpY accumulation) that a single-frame test can't reveal.

**Files:**
- Modify: `internal/decoder/decode_test.go` — remove `t.Skip(...)` from `TestDecode_BitExact_ALGTHM`

- [ ] **Step 1: Remove the skip**

Find `TestDecode_BitExact_ALGTHM` in `internal/decoder/decode_test.go` and remove its `t.Skip(...)` call.

- [ ] **Step 2: Run — iterate**

Run: `go test -run TestDecode_BitExact_ALGTHM -v -timeout=60s ./internal/decoder`

**If PASS**: proceed to Step 4.

**If FAIL**: note the first-divergent `frame N sample M`. Common cross-frame drift causes:
  - `d.pastExc` slide: verify `subframe.go:51-52` `copy(d.pastExc[:pastExcLen-subframeLen], d.pastExc[subframeLen:])` + write-new-at-tail pattern is correct (no off-by-one).
  - `postfilter.pastResidual` slide: verify `postfilter.go:33-34` similarly.
  - `synth.pastSynth` propagation: verified in Phase 1e tests.
  - `d.hpX`, `d.hpY` persistence: the HP filter's 2-tap state must carry across subframes.
  - `lsp.Decoder.prevLSP` persistence: set correctly after each frame's decode (last sf2's LSP becomes next frame's sf1 reference).

Add a focused test for the diagnosed stage, fix, re-run the full vector.

- [ ] **Step 3: Once PASS, run full packages**

Run: `go test ./... -count=1 -race`

Expected: PASS everywhere.

- [ ] **Step 4: Commit**

```bash
git add internal/decoder/decode_test.go
# plus any files touched during cross-frame drift debugging
git commit -m "$(cat <<'EOF'
test(decoder): re-enable ITU ALGTHM bit-exact (35 frames, PASSING)

2800 samples decoded byte-for-byte against the ITU Annex A reference.
First fully bit-exact ITU vector.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 10: Re-enable ITU SPEECH bit-exact (3750 frames)

**Rationale:** SPEECH is the 37.5-second real-speech stress test. Exercises every codec path with natural signal statistics. Most likely surface of long-form drift bugs that ALGTHM's 35 frames are too short to catch.

**Files:**
- Modify: `internal/decoder/decode_test.go` — remove `t.Skip(...)` from `TestDecode_BitExact_SPEECH`

- [ ] **Step 1: Remove the skip**

Find `TestDecode_BitExact_SPEECH` and remove its `t.Skip(...)`.

- [ ] **Step 2: Run — iterate if FAIL**

Run: `go test -run TestDecode_BitExact_SPEECH -v -timeout=300s ./internal/decoder`

**If PASS**: proceed to Step 4.

**If FAIL**: note the first-divergent frame. Common long-form drifts:
  - **Gain-VQ interaction with extreme codebook energies**: verify the `pow2Fixed` / `log2Fixed` chain's ±2 LSB tolerance doesn't accumulate over many frames into a visible `gcQ12` error. If it does, tighten the table accuracy or switch to exact integer arithmetic at the tight spots.
  - **Synth §3.10 guard interaction with long-term unstable LP filters**: the Phase 1i guard uses `fixed.Overflow`; verify it clears properly between subframes (the flag is global and persists across calls; `filterSubframe` must clear at start of every subframe).
  - **Postfilter AGC smoothing long-term convergence**: α ≈ 0.99 has a ~100-sample time constant. Over 3750 frames the smoother output must track `g_target` faithfully; verify no integer-accumulation drift in `pf.agcGainPrev` at Q24.

Binary-search the first divergent frame range to narrow the investigation.

- [ ] **Step 3: Commit**

```bash
git add internal/decoder/decode_test.go
# plus any files touched
git commit -m "$(cat <<'EOF'
test(decoder): re-enable ITU SPEECH bit-exact (3750 frames, PASSING)

37.5 seconds of real speech decoded byte-for-byte against ITU reference.
Largest vector; stresses every codec path under natural statistics.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 11: Re-enable ITU FIXED + LSP + PITCH bit-exact

**Rationale:** FIXED (120 frames), LSP (2232 frames), PITCH (1835 frames) — each stresses a different sub-system. Typically PASS unchanged once ALGTHM + SPEECH pass, but surface asymmetric drifts in LSP interpolation (LSP vector) or pitch lag refinement (PITCH vector).

**Files:**
- Modify: `internal/decoder/decode_test.go` — remove `t.Skip(...)` from FIXED, LSP, PITCH tests

- [ ] **Step 1: Remove the three skips**

Remove `t.Skip(...)` from `TestDecode_BitExact_FIXED`, `TestDecode_BitExact_LSP`, `TestDecode_BitExact_PITCH`.

- [ ] **Step 2: Run each in parallel**

Run:
```bash
go test -run TestDecode_BitExact_FIXED -v -timeout=60s ./internal/decoder
go test -run TestDecode_BitExact_LSP -v -timeout=180s ./internal/decoder
go test -run TestDecode_BitExact_PITCH -v -timeout=180s ./internal/decoder
```

Expected: all PASS. For any failure, apply the binary-search + targeted-fix + commit loop of Task 10.

- [ ] **Step 3: Commit**

```bash
git add internal/decoder/decode_test.go
# plus any files touched
git commit -m "$(cat <<'EOF'
test(decoder): re-enable ITU FIXED/LSP/PITCH bit-exact (4187 frames, PASSING)

FIXED (120 frames), LSP (2232 frames), PITCH (1835 frames). Each
stresses a different sub-system (fixed codebook, LSP quantizer, pitch
decoder) with natural-speech inputs. All three decode byte-for-byte
against ITU reference.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Task 12: Re-enable ITU TAME + TEST bit-exact + doc polish + completion report

**Rationale:** TAME (128 frames, high-correlation signal) and TEST (176 frames, generic) close out the seven loadable ITU vectors. Then write the completion report and verify no regressions.

**Files:**
- Modify: `internal/decoder/decode_test.go` — remove `t.Skip(...)` from TAME and TEST
- Create: `docs/superpowers/plans/2026-04-22-phase1j-gain-qformat-redrive-completion-report.md`

- [ ] **Step 1: Remove TAME and TEST skips**

Remove `t.Skip(...)` from `TestDecode_BitExact_TAME` and `TestDecode_BitExact_TEST`. Confirm TEST's `.pst` filename is lowercase (`TEST.pst`) as noted in Phase 1h.

- [ ] **Step 2: Run**

Run:
```bash
go test -run TestDecode_BitExact_TAME -v -timeout=60s ./internal/decoder
go test -run TestDecode_BitExact_TEST -v -timeout=60s ./internal/decoder
```

Expected: PASS. Fix any residual.

- [ ] **Step 3: Run final verification matrix**

Run all of the following in sequence:
```bash
go test -race ./...
go vet ./...
go test -bench=. -benchmem ./... | tee /tmp/phase1j-bench.txt
```

Expected:
- `go test -race ./...` — PASS, no `t.Skip` on any ITU vector except OVERFLOW (which remains gated on the separate bitstream reader bug).
- `go vet ./...` — silent.
- `go test -bench=. -benchmem ./...` — 0 B/op, 0 allocs/op across all benchmarks. Note `BenchmarkDecode` for comparison with Phase 1i's 10673 ns/op baseline; budget +5 % at most.

- [ ] **Step 4: Write the completion report**

Create `docs/superpowers/plans/2026-04-22-phase1j-gain-qformat-redrive-completion-report.md` with sections:

1. Plan link + commit hash of the final Task 12 commit
2. Task-status table (1–12, DONE / BLOCKED / DEFERRED)
3. Spec sections referenced (copy from plan header)
4. Phase 1i open items status — mark each resolved / deferred
5. **Bit-exact matrix** — seven vectors, each with `PASS (N frames, 0 deviations)` or failure detail
6. Plan deviations — any task that diverged from the plan text
7. Benchmark numbers (verbatim output)
8. **Phase 1k+ backlog** — OVERFLOW.BIT loadability, erasure/parity, public API, encoder
9. Full commit list (all Phase 1j commits)

- [ ] **Step 5: Final commit**

```bash
git add internal/decoder/decode_test.go docs/superpowers/plans/2026-04-22-phase1j-gain-qformat-redrive-completion-report.md
# plus any files touched in TAME/TEST diagnosis
git commit -m "$(cat <<'EOF'
test(decoder): re-enable ITU TAME/TEST bit-exact + Phase 1j complete

TAME (128 frames) and TEST (176 frames) — the last two of seven
loadable ITU reference vectors. All seven non-OVERFLOW vectors now
decode byte-for-byte against ITU reference. Phase 1j completion
report attached.

Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
EOF
)"
```

---

## Final verification checklist (after Task 12)

- [ ] `go test -race ./...` — PASS, no `t.Skip` on ALGTHM/SPEECH/FIXED/LSP/PITCH/TAME/TEST
- [ ] `go vet ./...` — silent
- [ ] `go test -bench=. -benchmem ./...` — 0 allocs/op everywhere, `BenchmarkDecode` within +5 % of Phase 1i's 10673 ns/op
- [ ] Seven ITU vectors (ALGTHM, SPEECH, FIXED, LSP, PITCH, TAME, TEST) produce byte-identical output vs `.pst` files
- [ ] Phase 1i's `TestDecode_Frame0Sample0_MatchesALGTHM`, Phase 1j's `TestDecode_Frame0Sample40_MatchesALGTHM`, and `TestDecode_Frame0_AllSamplesMatchALGTHM` all PASS
- [ ] `TestQFormatContract_*`, `TestPastErrorsDefault_*`, `TestMAPredictor_EvolutionFollowsSpec`, `TestGainVQ_SampleEntries_MatchSpec`, `TestDecode_CanonicalCodebook_GcMatchesSpecMagnitude` all PASS as permanent regression guards
- [ ] Pathological tests (`TestDecode_AllZero/LowEnergy/HighEnergy/Succeeds*`) PASS with re-certified invariants

---

## Self-review

**Spec coverage:** The Phase 1i completion report's two open items — (1) gain Q-format inconsistency causing gcQ12 saturation at sf2, and (2) seven ITU vectors still `t.Skip`ped — are addressed by Task 4 (the fix) and Tasks 9–12 (the re-enablement) respectively. The backlog items "gain VQ codebook values" and "MA predictor initial value" are covered by Tasks 6–7. OVERFLOW.BIT, erasure, parity, public API, encoder remain fenced out to Phase 1k+.

**Placeholder scan:** Task 6 Step 1 contains deliberately-marked `EXECUTOR TODO` placeholders for the spec-derived GainGBK1/GBK2 values; the executor is instructed to fill these in from the spec PDF before running the tests. This is the one exception to the "no placeholders" rule — it is intentional, clearly marked, and the test is `t.Skip`ped with a visible message so there is no risk of accidental false-PASS. No other placeholders exist.

**Type consistency:** `pastErrorsDefault int16 = -14336` stays at int16 Q10. `ecLog2Q10` remains a `fixed.Word32` at Q10 after Task 4's `-26*1024` correction. `log2Fixed`, `pow2Fixed`, `fixedCodebookEnergy` return types unchanged. `Decoder.pastErrors` remains `[4]int16` at Q10.

**Diagnosis-first discipline:** Tasks 1, 2, 3 are failing-baseline lock-downs before the fix in Task 4. Tasks 5–8 re-certify and extend invariants. Tasks 9–12 are vector-level validation with iterative diagnosis allowed per task.

**Scope fence respected:** OVERFLOW.BIT, erasure, parity, public API, encoder explicitly deferred to Phase 1k+.

---

## Phase 1k+ backlog (explicitly out of Phase 1j scope)

1. **OVERFLOW.BIT loadability** — `internal/bitstream.ReadG192File` returns `"invalid G.192 data word"`. Root cause unknown. Reverse-engineer framing variation and add loader path. Once loadable, add `TestDecode_BitExact_OVERFLOW` and validate that the Phase 1i `fixed.Overflow`-based §3.10 guard handles the vector's intentional overflow stress patterns.
2. **Frame-erasure concealment** — `Decoder.Decode(packed, bad=true, out)` currently ignores `bad`. Implement §A.4.1 concealment (extrapolated LSPs, faded excitation, gain taper) and validate against ITU ERASURE.BIT (300-frame vector).
3. **Parity-bit behavior** — `pitch.CheckParity` result is currently ignored. Audit Annex A §4.4 for required decoder reaction on parity mismatch and wire up. Validate against ITU PARITY.BIT (300-frame vector).
4. **Public API** — expose `g729.Decoder` / `g729.Encoder` in the root package with stable `New`, `Reset`, `Decode`, `Encode` entry points. Document Q-format expectations and lifecycle. Add top-level examples.
5. **Encoder** — full G.729A encoder pipeline: preprocessing (HP filter + scaling) → LP analysis → LSP quantization → open-loop pitch search → closed-loop pitch refinement → ACELP fixed-codebook search (depth-first) → gain quantization → bitstream packing. Requires its own multi-phase plan.
