package decoder

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
)

// TestPhase3aDiag1_GcUnsaturatedTaps_SPEECH dumps the per-subframe
// distribution of the gain-decoder unsaturated 32-bit intermediates
// across the ITU SPEECH.BIT vector, to localize which intermediate in
// (*gain.Decoder).Decode first overflows the int16 envelope.
//
// References (clean-room, public spec only):
//
//	ITU-T G.729 §3.9 / §3.9.2 (gain quantization; eq. (74)/(75))
//	Salami 1998 IEEE T-SAP §V.B (CS-ACELP gain quantization)
//	Kondoz §6 (gain VQ)
//
// Informational: t.Logf only — no t.Errorf.
func TestPhase3aDiag1_GcUnsaturatedTaps_SPEECH(t *testing.T) {
	bitPath := filepath.Join("../../testdata/itu/G729_Release3/g729AnnexA/test_vectors", "SPEECH.BIT")
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}

	r := bytes.NewReader(bitData)
	var packed [bitstream.FrameBytes]byte
	var dec Decoder

	type i32stat struct {
		min, max int32
		sum      int64
	}
	type i16stat struct {
		min, max int16
		sum      int64
	}
	var (
		predicted        i16stat
		ecBar            i16stat
		log2Gc           i32stat
		gc0Unsat         i32stat
		prodUnsat        i32stat
		gc0WrapInt16     int
		prodWrapInt16    int
		zeroEnergyCount  int
		nSub             int
		showSubs         = 10
	)
	predicted.min, ecBar.min = math.MaxInt16, math.MaxInt16
	predicted.max, ecBar.max = math.MinInt16, math.MinInt16
	log2Gc.min, gc0Unsat.min, prodUnsat.min = math.MaxInt32, math.MaxInt32, math.MaxInt32
	log2Gc.max, gc0Unsat.max, prodUnsat.max = math.MinInt32, math.MinInt32, math.MinInt32

	t.Logf("First %d subframes — gain-decoder unsaturated taps:", showSubs)
	t.Logf("%5s %3s %8s %8s %8s %12s %12s %8s %8s %8s",
		"frame", "sf", "pred", "ecBar", "log2Gc", "gc0Q14_un", "prodQ12_un",
		"gpQ14", "gcQ12", "gammaC")

	frames := 0
	for {
		if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
			break
		}
		taps, derr := dec.DecodeWithTaps(packed[:])
		if derr != nil {
			t.Fatalf("DecodeWithTaps frame %d: %v", frames, derr)
		}
		for sf := 0; sf < 2; sf++ {
			g := taps.Sub[sf].GainTaps
			if g.ZeroEnergyGuard {
				zeroEnergyCount++
			}
			if g.Predicted < predicted.min {
				predicted.min = g.Predicted
			}
			if g.Predicted > predicted.max {
				predicted.max = g.Predicted
			}
			predicted.sum += int64(g.Predicted)

			if g.EcBarDbQ10 < ecBar.min {
				ecBar.min = g.EcBarDbQ10
			}
			if g.EcBarDbQ10 > ecBar.max {
				ecBar.max = g.EcBarDbQ10
			}
			ecBar.sum += int64(g.EcBarDbQ10)

			if g.Log2GcQ10 < log2Gc.min {
				log2Gc.min = g.Log2GcQ10
			}
			if g.Log2GcQ10 > log2Gc.max {
				log2Gc.max = g.Log2GcQ10
			}
			log2Gc.sum += int64(g.Log2GcQ10)

			if g.Gc0Q14Unsat < gc0Unsat.min {
				gc0Unsat.min = g.Gc0Q14Unsat
			}
			if g.Gc0Q14Unsat > gc0Unsat.max {
				gc0Unsat.max = g.Gc0Q14Unsat
			}
			gc0Unsat.sum += int64(g.Gc0Q14Unsat)
			if g.Gc0Q14Unsat > 32767 || g.Gc0Q14Unsat < -32768 {
				gc0WrapInt16++
			}

			if g.ProdUnsat < prodUnsat.min {
				prodUnsat.min = g.ProdUnsat
			}
			if g.ProdUnsat > prodUnsat.max {
				prodUnsat.max = g.ProdUnsat
			}
			prodUnsat.sum += int64(g.ProdUnsat)
			if g.ProdUnsat > 32767 || g.ProdUnsat < -32768 {
				prodWrapInt16++
			}

			if nSub < showSubs {
				t.Logf("%5d %3d %8d %8d %8d %12d %12d %8d %8d %8d",
					frames, sf+1,
					g.Predicted, g.EcBarDbQ10, g.Log2GcQ10,
					g.Gc0Q14Unsat, g.ProdUnsat,
					g.GpQ14Final, g.GcQ12Final, g.GammaCQ13)
			}
			nSub++
		}
		frames++
	}

	if nSub == 0 {
		t.Fatalf("no subframes decoded")
	}

	mean := func(s int64) float64 { return float64(s) / float64(nSub) }
	t.Logf("")
	t.Logf("Aggregate over %d frames (%d subframes):", frames, nSub)
	t.Logf("  predicted    Q10dB : min=%d  max=%d  mean=%.1f", predicted.min, predicted.max, mean(predicted.sum))
	t.Logf("  ecBarDbQ10   Q10dB : min=%d  max=%d  mean=%.1f", ecBar.min, ecBar.max, mean(ecBar.sum))
	t.Logf("  log2GcQ10    Q10   : min=%d  max=%d  mean=%.1f", log2Gc.min, log2Gc.max, mean(log2Gc.sum))
	t.Logf("  gc0Q14_unsat int32 : min=%d  max=%d  mean=%.1f", gc0Unsat.min, gc0Unsat.max, mean(gc0Unsat.sum))
	t.Logf("  prodQ12_unsat int32: min=%d  max=%d  mean=%.1f", prodUnsat.min, prodUnsat.max, mean(prodUnsat.sum))
	t.Logf("")
	t.Logf("Wrap counts (subframes outside int16 envelope):")
	t.Logf("  gc0Q14_unsat > 32767 OR < -32768 : %d / %d  (%.2f%%)",
		gc0WrapInt16, nSub, 100*float64(gc0WrapInt16)/float64(nSub))
	t.Logf("  prodQ12_unsat outside int16     : %d / %d  (%.2f%%)",
		prodWrapInt16, nSub, 100*float64(prodWrapInt16)/float64(nSub))
	t.Logf("  zero-energy guard fired          : %d / %d", zeroEnergyCount, nSub)
}
