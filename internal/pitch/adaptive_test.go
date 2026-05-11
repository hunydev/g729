package pitch

import (
	"testing"

	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/tables"
)

func TestAdaptiveCodebookIntegerDelay(t *testing.T) {
	// Convention: pastExc[len(pastExc)-1] is u(-1), one sample before
	// the current subframe's v[0]. For integer delay T, v[n] = u(n−T)
	// = pastExc[len − T + n].
	var pastExc [250]int16
	for i := range pastExc {
		pastExc[i] = int16(i)
	}
	var v [40]int16
	AdaptiveCodebook(60, 0, pastExc[:], &v)
	for n := 0; n < 40; n++ {
		want := int16(190 + n) // pastExc[250 − 60 + n]
		if v[n] != want {
			t.Errorf("v[%d] = %d, want %d (integer delay 60)", n, v[n], want)
		}
	}
}

func TestAdaptiveCodebookIntegerDelayLargest(t *testing.T) {
	var pastExc [250]int16
	for i := range pastExc {
		pastExc[i] = int16(i - 100)
	}
	var v [40]int16
	AdaptiveCodebook(143, 0, pastExc[:], &v)
	for n := 0; n < 40; n++ {
		want := pastExc[250-143+n]
		if v[n] != want {
			t.Errorf("v[%d] = %d, want %d (integer delay 143)", n, v[n], want)
		}
	}
}

func TestAdaptiveCodebookFractionalPartitionOfUnity(t *testing.T) {
	// pastExc all 1s → v[n] should be ≈ 1 for any fractional offset
	// (partition of unity within rounding). T_int = 50 keeps all FIR
	// taps within the past-excitation buffer.
	var pastExc [250]int16
	for i := range pastExc {
		pastExc[i] = 1
	}
	for _, tFrac := range []int{-1, 0, 1} {
		var v [40]int16
		AdaptiveCodebook(60, tFrac, pastExc[:], &v)
		for n := 0; n < 40; n++ {
			if v[n] < 0 || v[n] > 2 {
				t.Errorf("v[%d] = %d at tFrac=%d, want ≈ 1 (partition of unity)",
					n, v[n], tFrac)
			}
		}
	}
}

func TestAdaptiveCodebookFractionalVariesWithTFrac(t *testing.T) {
	var pastExc [250]int16
	for i := range pastExc {
		pastExc[i] = int16((i * 37) & 0x3FFF)
	}
	var v0, vNeg, vPos [40]int16
	AdaptiveCodebook(60, 0, pastExc[:], &v0)
	AdaptiveCodebook(60, -1, pastExc[:], &vNeg)
	AdaptiveCodebook(60, 1, pastExc[:], &vPos)

	if v0 == vNeg {
		t.Error("AdaptiveCodebook tFrac=0 and tFrac=-1 produced identical output")
	}
	if v0 == vPos {
		t.Error("AdaptiveCodebook tFrac=0 and tFrac=+1 produced identical output")
	}
	if vNeg == vPos {
		t.Error("AdaptiveCodebook tFrac=-1 and tFrac=+1 produced identical output")
	}
}

func TestAdaptiveCodebookShortPitchIntegerDelay(t *testing.T) {
	var pastExc [200]int16
	for i := 180; i < 200; i++ {
		pastExc[i] = int16(i - 179)
	}
	var v [40]int16
	AdaptiveCodebook(20, 0, pastExc[:], &v)

	for n := 0; n < 20; n++ {
		want := int16(n + 1)
		if v[n] != want {
			t.Errorf("v[%d] = %d, want %d (pre-replication window)", n, v[n], want)
		}
	}
	for n := 20; n < 40; n++ {
		want := v[n-20]
		if v[n] != want {
			t.Errorf("v[%d] = %d, want v[%d] = %d (replicated)", n, v[n], n-20, want)
		}
	}
}

func TestAdaptiveCodebookShortPitchBoundary(t *testing.T) {
	var pastExc [200]int16
	for i := 100; i < 200; i++ {
		pastExc[i] = int16(i - 100)
	}
	var v [40]int16
	AdaptiveCodebook(39, 0, pastExc[:], &v)
	if v[39] != v[0] {
		t.Errorf("v[39] = %d, want v[0] = %d (T_int=39 replication at last sample)",
			v[39], v[0])
	}
}

func TestAdaptiveCodebookShortPitchFractionalUsesCurrentSamples(t *testing.T) {
	const (
		tInt  = 20
		tFrac = 1
	)
	var pastExc [200]int16
	for i := 0; i < len(pastExc); i++ {
		pastExc[i] = int16((i*73)%5000 - 2500)
	}

	var v [40]int16
	AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)

	// At n=tInt, eq. (40)'s forward branch references u(0..9), which
	// are the adaptive-vector samples already produced in this subframe.
	k, posPhase, negPhase := tInt+1, 2, 1
	var acc fixed.Word32
	for i := 0; i < Linter; i++ {
		backRel := tInt - k - i
		fwdRel := tInt - k + 1 + i
		var back, fwd int16
		if backRel < 0 {
			back = pastExc[len(pastExc)+backRel]
		} else {
			back = v[backRel]
		}
		if fwdRel < 0 {
			fwd = pastExc[len(pastExc)+fwdRel]
		} else {
			fwd = v[fwdRel]
		}
		acc = fixed.LMac(acc, tables.PitchInterpFIR[posPhase+3*i], back)
		acc = fixed.LMac(acc, tables.PitchInterpFIR[negPhase+3*i], fwd)
	}
	want := fixed.Round(acc)
	if v[tInt] != want {
		t.Fatalf("v[%d] = %d, want %d from recursive current-subframe taps", tInt, v[tInt], want)
	}
	if v[tInt] == v[0] {
		t.Fatalf("v[%d] = v[0] = %d; fractional short-pitch path fell back to integer-period repetition", tInt, v[tInt])
	}
}
