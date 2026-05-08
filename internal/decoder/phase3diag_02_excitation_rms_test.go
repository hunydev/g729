package decoder

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
)

// TestPhase3Diag_ExcitationRMS measures per-subframe RMS of the
// adaptive codebook v[], fixed codebook c[], and total excitation u[]
// across SPEECH.BIT, plus the gp / gc / T distributions. Aim: localize
// whether the amplitude collapse appears already at the excitation
// stage (gp · v + gc · c) or only later (synthesis filter / postfilter).
//
// References (clean-room, public spec only):
//
//	ITU-T G.729 §3.7 (adaptive codebook), §3.8 (fixed codebook),
//	§3.9 (gain quantization), §4.1.6 (excitation reconstruction).
//
// Informational: t.Logf only.
func TestPhase3Diag_ExcitationRMS_SPEECH(t *testing.T) {
	bitPath := filepath.Join("../../testdata/itu/G729_Release3/g729AnnexA/test_vectors", "SPEECH.BIT")
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}

	r := bytes.NewReader(bitData)
	var packed [bitstream.FrameBytes]byte
	var dec Decoder

	// Per-subframe accumulators
	var sumV2, sumC2, sumU2 float64
	var nSub int
	type bucket struct {
		min, max int
		sum      int64
	}
	var gpStats, gcStats bucket
	var tDist [144 + 8]int // pitch lag histogram T_int 0..151
	gpStats.min, gcStats.min = 1<<30, 1<<30
	gpStats.max, gcStats.max = -(1 << 30), -(1 << 30)

	frames := 0
	const showFrames = 8
	t.Logf("Per-subframe excitation tap snapshot (first %d frames, both subframes):", showFrames)
	t.Logf("%5s %3s %5s %5s %6s %6s %9s %9s %9s",
		"frame", "sf", "Tint", "Tfrac", "gpQ14", "gcQ12", "vRMS", "cRMS", "uRMS")

	for {
		if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
			break
		}
		taps, derr := dec.DecodeWithTaps(packed[:])
		if derr != nil {
			t.Fatalf("DecodeWithTaps frame %d: %v", frames, derr)
		}
		for sf := 0; sf < 2; sf++ {
			s := &taps.Sub[sf]
			vR := rmsArr(s.V[:])
			cR := rmsArr(s.C[:])
			uR := rmsArr(s.U[:])
			sumV2 += vR * vR
			sumC2 += cR * cR
			sumU2 += uR * uR
			nSub++

			gp := int(s.GpQ14)
			gc := int(s.GcQ12)
			if gp < gpStats.min {
				gpStats.min = gp
			}
			if gp > gpStats.max {
				gpStats.max = gp
			}
			gpStats.sum += int64(gp)
			if gc < gcStats.min {
				gcStats.min = gc
			}
			if gc > gcStats.max {
				gcStats.max = gc
			}
			gcStats.sum += int64(gc)
			if s.TInt >= 0 && s.TInt < len(tDist) {
				tDist[s.TInt]++
			}
			if frames < showFrames {
				t.Logf("%5d %3d %5d %5d %6d %6d %9.2f %9.2f %9.2f",
					frames, sf+1, s.TInt, s.TFrac, s.GpQ14, s.GcQ12, vR, cR, uR)
			}
		}
		frames++
	}

	if nSub == 0 {
		t.Fatalf("no subframes decoded")
	}

	t.Logf("")
	t.Logf("Aggregate over %d frames (%d subframes):", frames, nSub)
	t.Logf("  rms(v) avg = %.2f   (Q0; adaptive codebook output)", math.Sqrt(sumV2/float64(nSub)))
	t.Logf("  rms(c) avg = %.2f   (Q13; fixed codebook output)", math.Sqrt(sumC2/float64(nSub)))
	t.Logf("  rms(u) avg = %.2f   (Q0; total excitation u = gp·v + gc·c)",
		math.Sqrt(sumU2/float64(nSub)))
	t.Logf("")
	t.Logf("gpQ14: min=%d max=%d mean=%.1f  (Q14; 16384 = unity)",
		gpStats.min, gpStats.max, float64(gpStats.sum)/float64(nSub))
	t.Logf("gcQ12: min=%d max=%d mean=%.1f  (Q12; 4096 = unity)",
		gcStats.min, gcStats.max, float64(gcStats.sum)/float64(nSub))

	t.Logf("")
	t.Logf("T_int distribution (top 12 by count):")
	type tc struct {
		t, n int
	}
	var topT []tc
	for tt, n := range tDist {
		if n > 0 {
			topT = append(topT, tc{tt, n})
		}
	}
	// simple selection of top 12
	for i := 0; i < 12 && i < len(topT); i++ {
		bestIdx := i
		for j := i + 1; j < len(topT); j++ {
			if topT[j].n > topT[bestIdx].n {
				bestIdx = j
			}
		}
		topT[i], topT[bestIdx] = topT[bestIdx], topT[i]
		t.Logf("  T_int=%3d : %d", topT[i].t, topT[i].n)
	}
}

func rmsArr(s []int16) float64 {
	if len(s) == 0 {
		return 0
	}
	var e float64
	for _, v := range s {
		e += float64(v) * float64(v)
	}
	return math.Sqrt(e / float64(len(s)))
}
