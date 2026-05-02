package decoder

import "testing"

// phase1o_d3_batch_measurement_test.go — DOCUMENTATION ARTIFACT, not an
// executable assertion. Records the per-vector diff signatures gathered
// during the Phase 1o D-3 batch measurement-only pilot. Each row was
// captured by temporarily removing the t.Skip in decode_test.go,
// running the bit-exact harness, and then restoring the original skip
// text verbatim (decode_test.go diff confirmed empty post-pilot).
//
// Reference plan: docs/superpowers/plans/2026-05-09-phase1o-decoder-
// domain-closure-plan.md §D-3.
//
// TAME-pattern reference (commit 654ffe4):
//   - Window:    BROAD (early ±2 + late-frame growth)
//   - Magnitude: GROWING within frame (±2 → ±38 by sample 56)
//   - Cascade:   CROSS-FRAME (frame 1 sample 0 = +23, frame 2 sample 0 = +790)
//   - Verdict:   state-bearing defect (postfilter long-term memory,
//                AGC gain memory, HP filter state, or LP synth past
//                diverging across frames).
//
// Gate-17 reference (D-1b 6633b28, ALGTHM-only, disposed PASS-by-design):
//   - Window:    NARROW (frame 0 subframe 0 samples 5..7)
//   - Magnitude: BOUNDED (≤ ±3)
//   - Cascade:   NONE (transient; subsequent frames clean)
//
// ╔══════════════════════════════════════════════════════════════════════════╗
// ║ Phase 1o D-3 BATCH MEASUREMENT MATRIX                                    ║
// ╠══════════╦═══════════════╦═══════╦═══════╦═══════════╦═══════╦═══════════╣
// ║ Vector   ║ first-div     ║ got   ║ want  ║ max |Δ|   ║ casc. ║ category  ║
// ║          ║ (frame, samp) ║       ║       ║           ║       ║           ║
// ╠══════════╬═══════════════╬═══════╬═══════╬═══════════╬═══════╬═══════════╣
// ║ TAME*    ║ (0,   1)      ║    0  ║    2  ║       790 ║  YES  ║ TAME (ref)║
// ║ SPEECH   ║ (0,   0)      ║    2  ║    0  ║     32104 ║  YES  ║ TAME-SHAPED║
// ║ FIXED    ║ (0,   1)      ║    2  ║    4  ║      2144 ║  YES  ║ TAME-SHAPED║
// ║ LSP      ║ (0,  40)      ║    2  ║    0  ║     11774 ║  YES  ║ TAME-SHAPED║
// ║ PITCH    ║ (0,   1)      ║    6  ║    4  ║     25456 ║  YES  ║ TAME-SHAPED║
// ║ TEST     ║ (0,  40)      ║    2  ║    0  ║     10166 ║  YES  ║ TAME-SHAPED║
// ║ OVERFLOW ║ (0,   1)      ║    0  ║    2  ║     55406 ║  YES  ║ TAME-SHAPED║
// ╚══════════╩═══════════════╩═══════╩═══════╩═══════════╩═══════╩═══════════╝
//   * TAME row reproduced from commit 654ffe4 measurement.
//
// All 6 newly-measured vectors share the TAME signature:
//   (a) early-frame divergence with small ±2 magnitude window;
//   (b) within-frame magnitude growth to 10²–10⁵ range by frame end;
//   (c) cross-frame cascade where frame N sample 0 |Δ| grows roughly
//       monotonically with N, indicating a state component
//       (postfilter long-term memory, AGC gain memory, HP filter
//       state, LP synthesis past, or excitation/pitch memory) is
//       drifting away from the ITU reference each frame.
//
// SPEECH frames-with-diff = 3750/3750, FIXED 120/120, LSP 2232/2232,
// PITCH 1835/1835, TEST 176/176, OVERFLOW 384/384 — i.e. every frame
// of every vector is corrupted, consistent with a single common
// state-bearing root cause whose effect compounds.
//
// Common-root-cause verdict: YES — all 6 vectors are TAME-SHAPED.
// None is GATE-17-SHAPED (no narrow ≤±3 transient window) and none
// is NOVEL (every signature matches TAME's growth+cascade profile).
// One diagnostic cycle on the TAME state-bearing surface should
// dispose of all 6 simultaneously.
//
// Recommendation (next cycle): (α) DIAGNOSTIC CYCLE on the common
// TAME root cause — a stage-by-stage trace on TAME frames 0–2 that
// instruments each candidate state container (postfilter long-term
// gain memory g_l, AGC gain g_agc(n-1), HP filter biquad y[n-1]/y[n-2],
// synth memory mem[], pastExc[]) and reports which container's value
// at the frame-0/frame-1 boundary first diverges from the ITU
// stage-output reference. Do NOT pursue (β) batch known-difference
// demote — the cascade magnitudes (10³–10⁵) are far above any
// reasonable "known acceptable difference" threshold.

func TestPhase1o_D3_BatchMeasurementRecord(t *testing.T) {
	t.Skip("Phase 1o D-3 batch measurement record — documentation " +
		"artifact only; the matrix lives in the file-level comment " +
		"above. Each row was captured via temporary t.Skip removal " +
		"in decode_test.go followed by restoration; decode_test.go " +
		"diff is empty post-pilot. See docs/superpowers/plans/" +
		"2026-05-09-phase1o-decoder-domain-closure-plan.md §D-3.")

	type row struct {
		Vector            string
		FirstDivFrame     int
		FirstDivSample    int
		Got               int16
		Want              int16
		MaxAbsDelta       int
		FramesWithDiff    int
		FramesTotal       int
		CrossFrameCascade bool
		Category          string // TAME-SHAPED | GATE-17-SHAPED | NOVEL
	}
	_ = []row{
		{"TAME", 0, 1, 0, 2, 790, 128, 128, true, "TAME (reference, 654ffe4)"},
		{"SPEECH", 0, 0, 2, 0, 32104, 3750, 3750, true, "TAME-SHAPED"},
		{"FIXED", 0, 1, 2, 4, 2144, 120, 120, true, "TAME-SHAPED"},
		{"LSP", 0, 40, 2, 0, 11774, 2232, 2232, true, "TAME-SHAPED"},
		{"PITCH", 0, 1, 6, 4, 25456, 1835, 1835, true, "TAME-SHAPED"},
		{"TEST", 0, 40, 2, 0, 10166, 176, 176, true, "TAME-SHAPED"},
		{"OVERFLOW", 0, 1, 0, 2, 55406, 384, 384, true, "TAME-SHAPED"},
	}
}
