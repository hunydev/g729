package g729

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/gainquant"
)

// TestPhase2fTAME1_Slot5_OQTamingThrSweep is the Phase 2f TAME-1
// slot 5/5 OQ-TAMING-THR procedural close-out per sub-plan §5
// disposition tree third branch + §12 stop condition: "TAME-1 GA*/GB*
// < Phase 2d INT-1a baseline → MUST escalate slot 5/5 (OQ-TAMING-THR)
// before proceeding to INT-1."
//
// Sweep matrix (sub-plan §5 TAME-1 step 2, §8 slot 5/5 row):
//
//	gp clamp ∈ {0.9, 0.95, 1.0} × E threshold ∈ {2³², 2³³, 2³⁴} = 9 variants
//
// For each variant we swap gainquant.GpClipQ14 / TameEnergyThresholdQ0
// (demoted from const → var for slot-5/5 swappability), encode the
// full TAME.IN corpus with a fresh Encoder, byte-compare each output
// to the canonical 10-byte ground truth recovered from TAME.BIT via
// bitstream.ReadG192Frame, and tabulate the GA*/GB* + full-frame
// match rates. Originals are restored on test exit.
//
// Disposition rule (sub-plan §5 step 2 third branch):
//   - any variant whose GA1, GB1, GA2, GB2 all ≥ Phase 2d INT-1a
//     baseline (12.15 / 5.29 / 11.77 / 4.52 %) → pin that variant
//     (re-promote constants in a follow-up commit).
//   - all 9 variants miss → record FAIL-DEFERRED routing per I-2f-5;
//     OQ-TAMING-THR pin holds at the carryover values; remaining
//     shortfall traces to Phase 2c P1/P2 + Phase 2d S/C/C1/C2 = 0 %
//     FAIL-DEFERRED structural blocker.
//
// This test is informational (t.Logf, not t.Errorf): the sweep
// PRODUCES the sweep table that lets INT-3 record the slot-5/5
// disposition. The sibling TestPhase2fTAME1_ByteEQ remains the gating
// disposition test (its t.Errorf surfaces the floor breach). I-2f-5,
// I8.
func TestPhase2fTAME1_Slot5_OQTamingThrSweep(t *testing.T) {
	const (
		inPath  = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.IN"
		bitPath = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/TAME.BIT"

		samplesPerFrame  = FrameSamples
		bytesPerInFrame  = 2 * samplesPerFrame
		bytesPerBitFrame = 164
		totalFrames      = 128
		expectedInBytes  = totalFrames * bytesPerInFrame
		expectedBitBytes = totalFrames * bytesPerBitFrame
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read TAME.IN: %v", err)
	}
	if len(inData) != expectedInBytes {
		t.Fatalf("TAME.IN size = %d, want %d", len(inData), expectedInBytes)
	}
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read TAME.BIT: %v", err)
	}
	if len(bitData) != expectedBitBytes {
		t.Fatalf("TAME.BIT size = %d, want %d", len(bitData), expectedBitBytes)
	}

	bitReader := bytes.NewReader(bitData)
	want := make([][bitstream.FrameBytes]byte, totalFrames)
	for f := 0; f < totalFrames; f++ {
		if _, rerr := bitstream.ReadG192Frame(bitReader, want[f][:]); rerr != nil {
			t.Fatalf("ReadG192Frame frame %d: %v", f, rerr)
		}
	}

	type variant struct {
		gpLabel string
		eLabel  string
		gpQ14   int16
		eThresh int64
	}
	variants := []variant{
		{"0.9", "2^32", 14746, 1 << 32},
		{"0.9", "2^33", 14746, 1 << 33},
		{"0.9", "2^34", 14746, 1 << 34},
		{"0.95", "2^32", 15565, 1 << 32},
		{"0.95", "2^33", 15565, 1 << 33},
		{"0.95", "2^34", 15565, 1 << 34},
		{"1.0", "2^32", 16384, 1 << 32},
		{"1.0", "2^33", 16384, 1 << 33},
		{"1.0", "2^34", 16384, 1 << 34},
	}

	type variantResult struct {
		v                        variant
		full, ga1, gb1, ga2, gb2 float64
	}
	results := make([]variantResult, 0, len(variants))

	origGp := gainquant.GpClipQ14
	origE := gainquant.TameEnergyThresholdQ0
	defer func() {
		gainquant.GpClipQ14 = origGp
		gainquant.TameEnergyThresholdQ0 = origE
	}()

	for _, v := range variants {
		gainquant.GpClipQ14 = v.gpQ14
		gainquant.TameEnergyThresholdQ0 = v.eThresh

		enc := NewEncoder()
		var pcm [samplesPerFrame]int16
		var got [bitstream.FrameBytes]byte

		var fullMatch, ga1, gb1, ga2, gb2 int
		for f := 0; f < totalFrames; f++ {
			base := f * bytesPerInFrame
			for i := 0; i < samplesPerFrame; i++ {
				pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
			}
			if encErr := enc.EncodeFrame(pcm[:], got[:]); encErr != nil {
				t.Fatalf("variant gp=%s E=%s frame %d: EncodeFrame: %v",
					v.gpLabel, v.eLabel, f, encErr)
			}
			if bytes.Equal(got[:], want[f][:]) {
				fullMatch++
			}
			var gotF, refF bitstream.Frame
			if uErr := bitstream.Unpack(got[:], &gotF); uErr != nil {
				t.Fatalf("Unpack got: %v", uErr)
			}
			if uErr := bitstream.Unpack(want[f][:], &refF); uErr != nil {
				t.Fatalf("Unpack want: %v", uErr)
			}
			if gotF.GA1 == refF.GA1 {
				ga1++
			}
			if gotF.GB1 == refF.GB1 {
				gb1++
			}
			if gotF.GA2 == refF.GA2 {
				ga2++
			}
			if gotF.GB2 == refF.GB2 {
				gb2++
			}
		}
		pct := func(n int) float64 { return 100.0 * float64(n) / float64(totalFrames) }
		results = append(results, variantResult{
			v:    v,
			full: pct(fullMatch),
			ga1:  pct(ga1),
			gb1:  pct(gb1),
			ga2:  pct(ga2),
			gb2:  pct(gb2),
		})
	}

	// Phase 2d INT-1a baselines (the plausibility floor).
	const (
		floorGA1 = 12.15
		floorGB1 = 5.29
		floorGA2 = 11.77
		floorGB2 = 4.52
	)

	t.Logf("Phase 2f TAME-1 slot 5/5 OQ-TAMING-THR sweep — 9 variants × 128 frames")
	t.Logf("Phase 2d INT-1a baseline (plausibility floor): GA1≥%.2f%% GB1≥%.2f%% GA2≥%.2f%% GB2≥%.2f%%",
		floorGA1, floorGB1, floorGA2, floorGB2)
	t.Logf("%-6s %-6s | %-7s %-7s %-7s %-7s %-7s",
		"gpClip", "Eth", "full", "GA1", "GB1", "GA2", "GB2")

	bestIdx := -1
	for i, r := range results {
		mark := ""
		if r.ga1 >= floorGA1 && r.gb1 >= floorGB1 && r.ga2 >= floorGA2 && r.gb2 >= floorGB2 {
			mark = " ← all 4 floors met"
			if bestIdx < 0 {
				bestIdx = i
			}
		}
		t.Logf("%-6s %-6s | %6.2f%% %6.2f%% %6.2f%% %6.2f%% %6.2f%%%s",
			r.v.gpLabel, r.v.eLabel,
			r.full, r.ga1, r.gb1, r.ga2, r.gb2, mark)
	}

	if bestIdx >= 0 {
		w := results[bestIdx]
		t.Logf("Slot 5/5 disposition: PIN-CANDIDATE — variant gp=%s (Q14=%d) E=%s (=%d) "+
			"meets all 4 Phase 2d INT-1a floors. Follow-up commit MUST re-promote "+
			"gainquant.GpClipQ14 / TameEnergyThresholdQ0 to these values and re-run "+
			"TestPhase2fTAME1_ByteEQ.",
			w.v.gpLabel, w.v.gpQ14, w.v.eLabel, w.v.eThresh)
		// Sanity-print one suggested production-side commit stub.
		t.Logf("Suggested edit: GpClipQ14 = %d ; TameEnergyThresholdQ0 = %d",
			w.v.gpQ14, w.v.eThresh)
	} else {
		t.Logf("Slot 5/5 disposition: NO-WINNER — none of the 9 variants meet all 4 "+
			"Phase 2d INT-1a floors. OQ-TAMING-THR pin holds at the carryover values "+
			"(GpClipQ14=%d, TameEnergyThresholdQ0=2^33=%d). Per I-2f-5, the residual "+
			"shortfall routes upstream to Phase 2c P1/P2 + Phase 2d S/C/C1/C2 = 0%% "+
			"FAIL-DEFERRED structural blocker (the dominant cause of GA*/GB* miss is "+
			"that the ACELP search disagrees on every TAME frame upstream of the "+
			"taming branch, so no taming-threshold knob can recover the bit-EQ rate). "+
			"Slot 5/5 closed; INT-1 may now proceed.",
			origGp, origE)
	}
	_ = fmt.Sprintf // retained to satisfy linters if logging path changes
}
