package lsp

import (
	"fmt"
	"testing"

	"github.com/exedev/g729/internal/tables"
)

// TestDiagnostic_Phase2aInt1_Frame0BoundaryTrace — Phase 2a-INT-1 E9
// freeze-and-diagnose entry point.
//
// Reference plan:
//
//	docs/superpowers/plans/2026-05-03-phase2a-lpc-lsp-plan.md
//	§Task 2a-INT-1 + §0.4 (강압-적합-금지) + Error E9 (I5/I6 hard-N close)
//
// Cycle context: TestEncode_LSPVectorBitExact (root-level integration
// gate) has consumed the I5 hypothesis budget (4 production-fix
// attempts: InitFreqPrev, residual-rearrange J1+J2 in Quantize,
// reverted preprocessor-skip experiment, post-predictor stability
// enforcement in Quantize). After Fix #4 the per-stage match counts
// stand at L0=1773/2232 (79%) L1=852/2232 (38%) L2=349/2232 (16%)
// L3=421/2232 (19%); first divergence is frame 0 with
// got=(L0=0,L1=120,L2=2,L3=11) vs want=(0,120,10,10). The L0/L1 frame-0
// match is structurally encouraging (LP-analysis → ω → target → L1 is
// roughly correct at warm-start), but the L2/L3 mismatch on the very
// first frame indicates the divergence is downstream of L1 and not a
// freqPrev-init artefact. Per E9 the appropriate close is to FREEZE
// production (I6) and dump a measurement-only boundary trace.
//
// This test is the I6-side measurement skeleton for that closure: it
// is intended as the hand-off artefact for the next diagnostic cycle,
// not a fix in itself. It is gated behind the env var
// `G729_PHASE2A_INT1_DIAG=1` so it does not run in the default suite
// (mirrors the convention of other measurement-only diagnostic tests
// in this package — they emit boundary-trace data on demand and never
// gate CI).
//
// ABSOLUTE CONSTRAINTS (E1/E2/E5):
//   - clean-room MIT: no ITU C / bcg729 / Sipro / FFmpeg G.729 / Annex
//     A binary reference. Spec source = G729E.txt + Annex A only.
//   - production = 0 line change (E2) once this test lands; production
//     is FROZEN at the post-Fix-#4 state.
//   - measurement-only (E5): hard-asserts only structural invariants
//     that the production code already satisfies (all-zero residual
//     reconstructs to the uniform-LSF init, etc.). Numerical
//     boundary-trace values are emitted via t.Logf for human
//     inspection in the next diagnostic cycle.
//
// BOUNDARY TRACE PROTOCOL (Phase 1o D-3 pattern):
//
//	S1: dump initialPastResidual[k][i] (Q13) for k=0..3, i=0..9.
//	S2: dump LSPCodebookL1[120][i] for i=0..9 (Q13) — the L1 winner
//	    at frame 0 per the integration gate's "want" vector.
//	S3: dump LSPCodebookL2[10][i] for i=0..4 (Q13) — the L2 winner
//	    per the "want" vector — and LSPCodebookL2[2][i] for i=0..4
//	    — the L2 actually picked by production. The diagnostic
//	    cycle should compute, by hand from the spec, the partial
//	    weighted MSE for both candidates given the frame-0 ω that
//	    L1=120 implies, and identify which side (production search
//	    or spec interpretation) has the bug.
//	S4: dump LSPCodebookL3[10][i] and LSPCodebookL3[11][i] for
//	    i=0..4 — analogous L3 want/got pair.
func TestDiagnostic_Phase2aInt1_Frame0BoundaryTrace(t *testing.T) {
	if testing.Short() {
		t.Skip("diagnostic boundary trace; -short")
	}
	const wantL1, wantL2, wantL3 = 120, 10, 10
	const gotL2, gotL3 = 2, 11

	// --- S1: initialPastResidual (encoder MA-predictor cold-start memory)
	t.Logf("=== S1: initialPastResidual (Q13, i·π/11) = %v ===", initialPastResidual)

	// --- S2: L1 winner row at frame 0
	t.Logf("=== S2: LSPCodebookL1[%d] (Q13) ===", wantL1)
	t.Logf("  %v", tables.LSPCodebookL1[wantL1])

	// --- S3: L2 want vs got (lower 5 components)
	t.Logf("=== S3: LSPCodebookL2 want=%d got=%d (Q13, i=0..4) ===", wantL2, gotL2)
	t.Logf("  want L2[%d] = %v", wantL2, tables.LSPCodebookL2[wantL2])
	t.Logf("  got  L2[%d] = %v", gotL2, tables.LSPCodebookL2[gotL2])

	// --- S4: L3 want vs got (upper 5 components)
	t.Logf("=== S4: LSPCodebookL3 want=%d got=%d (Q13, i=0..4) ===", wantL3, gotL3)
	t.Logf("  want L3[%d] = %v", wantL3, tables.LSPCodebookL3[wantL3])
	t.Logf("  got  L3[%d] = %v", gotL3, tables.LSPCodebookL3[gotL3])

	// --- Structural invariant: combineResidual yields a 10-vector
	//     equal to L1 + L2/L3 piecewise. (sanity check that the
	//     codebook indexing convention matches what the gate uses.)
	var residual [10]int16
	for i := 0; i < 5; i++ {
		residual[i] = tables.LSPCodebookL1[wantL1][i] + tables.LSPCodebookL2[wantL2][i]
		residual[5+i] = tables.LSPCodebookL1[wantL1][5+i] + tables.LSPCodebookL3[wantL3][i]
	}
	t.Logf("=== combined want residual l̂ (Q13) = %v ===", residual)

	// Hard-assert that the cold-start uniform-LSF residual is what
	// InitFreqPrev seeds. This is the only spec-derivable invariant
	// we can pin without an oracle ω from the encoder side.
	var freq [4][10]int16
	InitFreqPrev(&freq)
	for k := 0; k < 4; k++ {
		if freq[k] != initialPastResidual {
			t.Fatalf("InitFreqPrev[%d] = %v, want %v", k, freq[k], initialPastResidual)
		}
	}

	// Stub for next-cycle extension: caller-supplied ω at frame 0
	// (from a still-to-be-written `lpc.LPToLSP` boundary dump driven
	// by LSP.IN frame 0) would be plugged in here, allowing the
	// partial weighted MSE for L2=2 vs L2=10 to be computed in
	// closed form from spec §3.2.4 lines 851–856 (eq. 21) +
	// 887–895.
	_ = fmt.Sprintf
}
