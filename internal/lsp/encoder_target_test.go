package lsp

import (
	"math"
	"testing"

	"github.com/exedev/g729/internal/tables"
)

// expectedTargetLSF computes the eq. (23) closed form in float64 for
// use as an independent reference in the Q13 fixed-point assertions.
func expectedTargetLSF(selector uint8, mem *[4][10]int16, omega *[10]int16) [10]int16 {
	preds := &tables.MAPredictorsLSP[selector]
	var out [10]int16
	for i := 0; i < 10; i++ {
		omegaR := float64(omega[i]) / 8192.0
		sumPmem := 0.0
		sumP := 0.0
		for k := 0; k < 4; k++ {
			pr := float64(preds[k][i]) / 32768.0
			mr := float64(mem[k][i]) / 8192.0
			sumPmem += pr * mr
			sumP += pr
		}
		lr := (omegaR - sumPmem) / (1.0 - sumP)
		v := math.Round(lr * 8192.0)
		switch {
		case v > 32767:
			out[i] = 32767
		case v < -32768:
			out[i] = -32768
		default:
			out[i] = int16(v)
		}
	}
	return out
}

// TestComputeTargetLSF_StartupZeroMemory exercises the codec-start
// case: ω(m) = i·π/11 (Q13) and the predictor memory is all zero, so
// the predictor history contribution vanishes and l_i collapses to
// ω_i / (1 − Σ P).
func TestComputeTargetLSF_StartupZeroMemory(t *testing.T) {
	const piQ13 = 25736
	var omega [10]int16
	for i := 0; i < 10; i++ {
		omega[i] = int16(((i+1)*piQ13 + 5) / 11)
	}

	for _, sel := range []uint8{0, 1} {
		var mem [4][10]int16
		var got [10]int16
		computeTargetLSF(sel, &mem, &omega, &got)
		want := expectedTargetLSF(sel, &mem, &omega)
		for i := 0; i < 10; i++ {
			if d := int(got[i]) - int(want[i]); d < -4 || d > 4 {
				t.Errorf("sel=%d i=%d: got=%d want=%d (diff=%d)", sel, i, got[i], want[i], d)
			}
		}
	}
}

// TestComputeTargetLSF_NonzeroMemory exercises the second branch of
// eq. (23): ω(m) = 0 and one non-zero past-residual frame in mem[0],
// so l_i = (−P_{i,0}·mem[0][i]) / (1 − Σ P_{i,k}).
func TestComputeTargetLSF_NonzeroMemory(t *testing.T) {
	var omega [10]int16
	var mem [4][10]int16
	for i := 0; i < 10; i++ {
		mem[0][i] = int16(1000 + i*150)
	}

	for _, sel := range []uint8{0, 1} {
		var got [10]int16
		computeTargetLSF(sel, &mem, &omega, &got)
		want := expectedTargetLSF(sel, &mem, &omega)
		for i := 0; i < 10; i++ {
			if d := int(got[i]) - int(want[i]); d < -4 || d > 4 {
				t.Errorf("sel=%d i=%d: got=%d want=%d (diff=%d)", sel, i, got[i], want[i], d)
			}
		}
	}
}

// TestComputeTargetLSF_DoesNotMutateInputs guards against accidental
// in-place writes; the encoder's L0 search loop calls this routine
// repeatedly with the same memory snapshot.
func TestComputeTargetLSF_DoesNotMutateInputs(t *testing.T) {
	var omega [10]int16
	var mem [4][10]int16
	for i := 0; i < 10; i++ {
		omega[i] = int16(2000 + i*200)
		for k := 0; k < 4; k++ {
			mem[k][i] = int16(500 + k*100 + i*7)
		}
	}
	memCopy := mem
	omegaCopy := omega
	var target [10]int16
	computeTargetLSF(1, &mem, &omega, &target)
	if mem != memCopy {
		t.Errorf("computeTargetLSF mutated mem")
	}
	if omega != omegaCopy {
		t.Errorf("computeTargetLSF mutated omega")
	}
}
