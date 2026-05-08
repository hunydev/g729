package decoder

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/hunydev/g729/internal/bitstream"
	"github.com/hunydev/g729/internal/pcm"
)

// TestPhase3Diag_SynthesisBypass measures RMS at every stage of the
// per-subframe pipeline so we can read off WHICH stage drops amplitude:
//
//	u    : excitation feeding 1/Â(z)
//	s    : output of synthesis filter 1/Â(z)
//	sPf  : output of postfilter (Annex A short+long-term + tilt + AGC)
//	hp   : output of decoder HP filter (pre ScaleUpSat ×2)
//	out  : final 80-sample frame after pcm.ScaleUpSat (×2)
//
// Also runs a parallel "synthesis-bypass" reconstruction in which the
// postfilter is skipped (out_bypass = ScaleUpSat(hp(s))) so the
// contribution of the postfilter to amplitude can be isolated.
//
// References (clean-room): ITU-T G.729 §4.1.6 (excitation), §3.10
// (synthesis filter / overflow recovery), §A.4.2 (Annex A postfilter).
//
// Informational: t.Logf only.
func TestPhase3Diag_SynthesisBypass_SPEECH(t *testing.T) {
	bitPath := filepath.Join("../../testdata/itu/G729_Release3/g729AnnexA/test_vectors", "SPEECH.BIT")
	pstPath := filepath.Join("../../testdata/itu/G729_Release3/g729AnnexA/test_vectors", "SPEECH.PST")
	bitData, err := os.ReadFile(bitPath)
	if err != nil {
		t.Fatalf("read SPEECH.BIT: %v", err)
	}
	pstData, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read SPEECH.PST: %v", err)
	}

	r := bytes.NewReader(bitData)
	var packed [bitstream.FrameBytes]byte
	var dec Decoder

	var sumU2, sumS2, sumSPf2, sumHp2, sumOut2 float64
	var sumOutBypass2 float64
	var nSub int
	var maxAbsOut, maxAbsBypass int16

	frames := 0
	const showFrames = 12
	t.Logf("Per-subframe RMS at every stage (first %d frames):", showFrames)
	t.Logf("%5s %3s %9s %9s %9s %9s",
		"frame", "sf", "u_RMS", "s_RMS", "sPf_RMS", "hp_RMS")

	for {
		if _, rerr := bitstream.ReadG192Frame(r, packed[:]); rerr != nil {
			break
		}
		taps, derr := dec.DecodeWithTaps(packed[:])
		if derr != nil {
			t.Fatalf("DecodeWithTaps frame %d: %v", frames, derr)
		}
		// Bypass-postfilter alternative: take taps.S (post-1/Â(z), pre-pf),
		// run a copy through HP-filter via same coefficients-only logic
		// using a side decoder snapshot. To avoid mutating the main HP
		// state, just compare RMS at hp output of MAIN path vs RMS at
		// taps.S directly (s, no PF, no HP, no scale).
		for sf := 0; sf < 2; sf++ {
			s := &taps.Sub[sf]
			uR := rmsArr(s.U[:])
			sR := rmsArr(s.S[:])
			pfR := rmsArr(s.SPf[:])
			hpR := rmsArr(s.HpOut[:])
			sumU2 += uR * uR
			sumS2 += sR * sR
			sumSPf2 += pfR * pfR
			sumHp2 += hpR * hpR
			nSub++

			// Also: what if we just doubled S (bypass pf and hp)? Used
			// as a "pure synthesis amplitude" reference.
			var doubled [40]int16
			pcm.ScaleUpSat(s.S[:], doubled[:])
			d2 := rmsArr(doubled[:])
			sumOutBypass2 += d2 * d2
			for _, v := range doubled {
				a := v
				if a < 0 {
					a = -a
				}
				if a > maxAbsBypass {
					maxAbsBypass = a
				}
			}

			if frames < showFrames {
				t.Logf("%5d %3d %9.2f %9.2f %9.2f %9.2f",
					frames, sf+1, uR, sR, pfR, hpR)
			}
		}
		// per-frame final output max
		var fOut [80]int16 = taps.Output
		for _, v := range fOut {
			a := v
			if a < 0 {
				a = -a
			}
			if a > maxAbsOut {
				maxAbsOut = a
			}
		}
		var oR float64
		for _, v := range fOut {
			oR += float64(v) * float64(v)
		}
		sumOut2 += oR / 80.0
		frames++
	}

	if nSub == 0 {
		t.Fatalf("no subframes")
	}

	t.Logf("")
	t.Logf("Aggregate stage RMS over %d subframes (%d frames):", nSub, frames)
	t.Logf("  rms(u)            = %9.2f   excitation (Q0)", math.Sqrt(sumU2/float64(nSub)))
	t.Logf("  rms(s)            = %9.2f   post 1/Â(z) (Q0, pre-postfilter)",
		math.Sqrt(sumS2/float64(nSub)))
	t.Logf("  rms(sPf)          = %9.2f   post Annex A postfilter (Q0)",
		math.Sqrt(sumSPf2/float64(nSub)))
	t.Logf("  rms(hp)           = %9.2f   post HP filter (Q0, pre ×2)",
		math.Sqrt(sumHp2/float64(nSub)))
	t.Logf("  rms(out, ×2)      = %9.2f   final ScaleUpSat output (Q0)",
		math.Sqrt(sumOut2/float64(frames)))
	t.Logf("  rms(2·s, bypass)  = %9.2f   ScaleUpSat applied to s, postfilter+HP bypassed",
		math.Sqrt(sumOutBypass2/float64(nSub)))
	t.Logf("")
	t.Logf("max |sample| final     : %d", maxAbsOut)
	t.Logf("max |sample| bypass    : %d", maxAbsBypass)

	// PST reference RMS for orientation
	const fs = 80
	nFrames := len(pstData) / (2 * fs)
	if nFrames > frames {
		nFrames = frames
	}
	var pstSum float64
	for f := 0; f < nFrames; f++ {
		var fr float64
		for i := 0; i < fs; i++ {
			off := 2 * (f*fs + i)
			s := int16(uint16(pstData[off]) | uint16(pstData[off+1])<<8)
			fr += float64(s) * float64(s)
		}
		pstSum += fr / float64(fs)
	}
	t.Logf("rms(SPEECH.PST) ref    = %9.2f   ITU PST output (×2 already applied)",
		math.Sqrt(pstSum/float64(nFrames)))
}
