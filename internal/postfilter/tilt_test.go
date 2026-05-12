package postfilter

import "testing"

func TestApplyTilt_ZeroMuIsIdentity(t *testing.T) {
	var pf Postfilter
	var sIn, sOut [subframeLen]int16
	for i := range sIn {
		sIn[i] = int16(200 + i*3)
	}

	pf.applyTiltWithMu(&sIn, 0, &sOut)

	for i := range sOut {
		if sOut[i] != sIn[i] {
			t.Errorf("sOut[%d] = %d, want %d (μ = 0 is identity)",
				i, sOut[i], sIn[i])
		}
	}
}

func TestApplyTilt_HalfMuAmplifies(t *testing.T) {
	var pf Postfilter
	var sIn, sOut [subframeLen]int16
	for i := range sIn {
		sIn[i] = 100
	}

	pf.applyTiltWithMu(&sIn, 16384, &sOut)

	if sOut[0] < 99 || sOut[0] > 101 {
		t.Errorf("sOut[0] = %d, want 100 (pastTiltInput = 0)", sOut[0])
	}
	for i := 1; i < subframeLen; i++ {
		if sOut[i] < 149 || sOut[i] > 151 {
			t.Errorf("sOut[%d] = %d, want 150 (±1)", i, sOut[i])
		}
	}
}

func TestApplyTilt_UpdatesPastTiltInput(t *testing.T) {
	var pf Postfilter
	var sIn, sOut [subframeLen]int16
	for i := range sIn {
		sIn[i] = int16(i + 1)
	}

	pf.applyTiltWithMu(&sIn, 0, &sOut)

	if pf.pastTiltInput != sIn[subframeLen-1] {
		t.Errorf("pastTiltInput = %d, want %d",
			pf.pastTiltInput, sIn[subframeLen-1])
	}
}

func TestComputeTiltMu_IdentityCascade_ZeroMu(t *testing.T) {
	a := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var pf Postfilter
	mu := pf.computeTiltMu(&a, &a)
	if mu != 0 {
		t.Fatalf("identity cascade: want μ=0, got %d", mu)
	}
}

func TestComputeTiltMu_SinglePoleHalf(t *testing.T) {
	aNum := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	aDen := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var pf Postfilter
	mu := pf.computeTiltMu(&aNum, &aDen)
	const want = -13107
	if mu < want-8 || mu > want+8 {
		t.Fatalf("μ = 0.8 · (-0.5) Q15: want %d ± 8, got %d", want, mu)
	}
}

func TestComputeTiltMu_SinglePoleMinusHalf(t *testing.T) {
	aNum := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	aDen := [11]int16{4096, 2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var pf Postfilter
	mu := pf.computeTiltMu(&aNum, &aDen)
	if mu != 0 {
		t.Fatalf("Annex A k1' >= 0 branch: want μ=0, got %d", mu)
	}
}

func TestComputeTiltMu_DoesNotDependOnAGCState(t *testing.T) {
	aNum := [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	aDen := [11]int16{4096, -2048, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	var pfA, pfB Postfilter
	pfB.agcGainPrev = 1 << 24
	pfB.initialized = true
	muA := pfA.computeTiltMu(&aNum, &aDen)
	muB := pfB.computeTiltMu(&aNum, &aDen)
	if muA != muB {
		t.Fatalf("computeTiltMu depended on AGC state: zero-state=%d initialized=%d", muA, muB)
	}
}
