package synth

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/fixed"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
)

// TestDiagnostic_FoctPostfix2PrelimM3IIRMemory: Stage F-oct-postfix2-prelim-4
// Step 4 (M3 hypothesis 재진입: synth IIR memory propagation 가 sample 5..7
// 부호 결함의 원인인가?).
//
// F-oct-prelim-5-3 §3.3 의 M3 폐기 결정은 "synth IIR memory init = zero
// dump" 즉 codec-start 의 pastSynth = [0;10] 정상에 근거. 본 task 는
// sample 5..7 한정으로 IIR memory propagation pattern (pre-sample-5 /
// post-sample-7) 을 측정하여 §3.3 결정을 재평가.
//
// Spec § 인용:
//
//	ITU-T G.729 (06/2012) §A.4.1 — "LP synthesis filter": same as §3.10.
//	§3.10:  ŝ(n) = u(n) − Σ_{i=1..10} a_i · ŝ(n−i)
//	§4.3 Table 9:  pastSynth initialised to 0 at codec start.
//
// Production 변경 0. assertion 0 (측정-only).
func TestDiagnostic_FoctPostfix2PrelimM3IIRMemory(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, _ := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var f bitstream.Frame
	if err := bitstream.Unpack(frames[0], &f); err != nil {
		t.Fatalf("Unpack frame 0: %v", err)
	}

	// (1) sf0 LP coefficients (Q12) — same path as F-sept-3.
	var lspDec lsp.Decoder
	lspDec.Reset()
	sfA, _ := lspDec.Decode(lsp.Indices{
		L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
	})

	// (2) excitation u[]
	tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
	const pastExcLen = 153 // pitchMax(143) + 10
	var pastExc [pastExcLen]int16
	var v [40]int16
	pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
	betaQ14 := fcb.ClampPitchGainForEnhancement(0)
	var c [40]int16
	fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)
	var gn gain.Decoder
	gn.Reset()
	gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
	var u [40]int16
	BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

	t.Logf("──────── M3 fixture (ALGTHM frame 0 sf0) ────────")
	t.Logf("LP a[0..10] (Q12, a[0]=4096) = %v", sfA)
	t.Logf("excitation u[0..7] = [%+d %+d %+d %+d %+d %+d %+d %+d]",
		u[0], u[1], u[2], u[3], u[4], u[5], u[6], u[7])
	t.Logf("PST want sample 5..7 = [%+d %+d %+d]   signs=[%s %s %s]",
		wantFrames[0][5], wantFrames[0][6], wantFrames[0][7],
		signOfInt16(wantFrames[0][5]), signOfInt16(wantFrames[0][6]), signOfInt16(wantFrames[0][7]))

	// (3) Codec-start pastSynth (per §4.3 Table 9): all zero.
	var sy Synthesizer
	sy.Reset()
	pastSynthStart := sy.pastSynth
	t.Logf("pastSynth (Synthesizer.Reset, codec-start) = %v   (§4.3 Table 9 all-zero invariant)",
		pastSynthStart)

	// (4) Per-sample IIR replay — same arithmetic as filter.go onePass,
	// but capturing the IIR memory state pre-sample-5 and post-sample-7.
	// The "memory" at time n consists of the previous 10 outputs:
	//
	//	mem[i] = ŝ(n-1-i)     i = 0..9    (mem[0] = most recent)
	//
	// Equivalent to work[10+n-1-i] in onePass.  We replay using fixed
	// primitives so this matches Pass-1 exactly.  (Pass-2 overflow
	// recovery is detected separately via fixed.Overflow flag below.)
	var work [50]int16
	copy(work[:10], pastSynthStart[:])

	fixed.ClearOverflow()
	for n := 0; n < 8; n++ {
		lTemp := fixed.LMult(u[n], sfA[0])
		for i := 1; i <= 10; i++ {
			lTemp = fixed.LMsu(lTemp, sfA[i], work[10+n-i])
		}
		lTemp = fixed.LShl(lTemp, 3)
		work[10+n] = fixed.Round(lTemp)
	}

	// IIR memory pre-sample-5 = previous 10 outputs at time n=5
	//                         = work[10+5-1-i] for i=0..9  = work[14..5]
	// IIR memory post-sample-7 = previous 10 outputs at time n=8 (end of sample 7)
	//                         = work[17..8]
	var memPreSample5 [10]int16
	for i := 0; i < 10; i++ {
		memPreSample5[i] = work[14-i]
	}
	var memPostSample7 [10]int16
	for i := 0; i < 10; i++ {
		memPostSample7[i] = work[17-i]
	}
	syn0to7 := [8]int16{
		work[10], work[11], work[12], work[13],
		work[14], work[15], work[16], work[17],
	}
	pass1Overflow := fixed.Overflow()

	t.Logf("──────── M3 IIR memory dump ────────")
	t.Logf("[M3 IIR memory pre-sample-5]  mem[0..9] (ŝ(4..-5)) = %v", memPreSample5)
	t.Logf("[M3 IIR memory post-sample-7] mem[0..9] (ŝ(7..-2)) = %v", memPostSample7)
	t.Logf("syn[0..7] (replayed Pass-1) = %v", syn0to7)
	t.Logf("Pass-1 fixed.Overflow = %v", pass1Overflow)

	// (5) Cross-check vs production Synthesizer.Filter (Pass-1 only — Pass-2
	// triggers reset of overflow flag inside Filter; we record both).
	var syProd Synthesizer
	syProd.Reset()
	var sProd [40]int16
	fixed.ClearOverflow()
	syProd.Filter(&sfA, &u, &sProd)
	postProdOverflow := fixed.Overflow()
	t.Logf("syn[0..7] (production Synthesizer.Filter) = %v",
		[8]int16{sProd[0], sProd[1], sProd[2], sProd[3], sProd[4], sProd[5], sProd[6], sProd[7]})
	t.Logf("post-Filter fixed.Overflow = %v  (Pass-2 triggered if Pass-1 overflow but post-flag clean)",
		postProdOverflow)

	// (6) Memory propagation pattern — does the IIR state change sign
	// across sample 5..7?  F-oct-prelim-5-3 §3.3 polled "all-zero memory"
	// dump at codec-start; this widens to sample 5..7 propagation.
	signChanges := 0
	for i := 0; i < 10; i++ {
		s1 := signOfInt16(memPreSample5[i])
		s2 := signOfInt16(memPostSample7[i])
		if s1 != "0" && s2 != "0" && s1 != s2 {
			signChanges++
		}
	}
	t.Logf("memory sign changes (pre-5 vs post-7, non-zero pairs only) = %d / 10", signChanges)

	// (7) Forced sign-flip stimulus — negate u[] and re-run filter.
	// Linear IIR (Pass-1 only, no asymmetric saturation) → output should
	// fully invert.  Asymmetric behaviour (sign asymmetry, saturation
	// boundary effects, memory bias) → partial / no inversion.
	var uNeg [40]int16
	for i, x := range u {
		v := -int32(x)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		uNeg[i] = int16(v)
	}
	var syNeg Synthesizer
	syNeg.Reset()
	var sNeg [40]int16
	fixed.ClearOverflow()
	syNeg.Filter(&sfA, &uNeg, &sNeg)
	negOverflow := fixed.Overflow()

	t.Logf("──────── M3 forced sign-flip stimulus ────────")
	t.Logf("(-u)  excitation 0..7 = %v",
		[8]int16{uNeg[0], uNeg[1], uNeg[2], uNeg[3], uNeg[4], uNeg[5], uNeg[6], uNeg[7]})
	t.Logf("syn(-u) sample 0..7   = %v",
		[8]int16{sNeg[0], sNeg[1], sNeg[2], sNeg[3], sNeg[4], sNeg[5], sNeg[6], sNeg[7]})
	t.Logf("post-Filter overflow (negated stimulus) = %v", negOverflow)

	fullyInverted := true
	for n := 0; n < 8; n++ {
		// sNeg[n] should equal -sProd[n] (modulo Word16 saturation symmetry).
		expected := -int32(sProd[n])
		if expected > 32767 {
			expected = 32767
		} else if expected < -32768 {
			expected = -32768
		}
		if int32(sNeg[n]) != expected {
			fullyInverted = false
		}
	}
	t.Logf("syn(-u) == -syn(+u) for sample 0..7 ? %v   (Pass-1 linear IIR invariant)",
		fullyInverted)

	// (8) M3 가설 평가.
	t.Logf("──────── M3 hypothesis evaluation ────────")
	syn5to7Signs := [3]string{
		signOfInt16(sProd[5]), signOfInt16(sProd[6]), signOfInt16(sProd[7]),
	}
	wantSigns := [3]string{
		signOfInt16(wantFrames[0][5]),
		signOfInt16(wantFrames[0][6]),
		signOfInt16(wantFrames[0][7]),
	}
	t.Logf("synth IIR syn[5..7] signs = [%s %s %s]   ;   PST want signs = [%s %s %s]",
		syn5to7Signs[0], syn5to7Signs[1], syn5to7Signs[2],
		wantSigns[0], wantSigns[1], wantSigns[2])

	switch {
	case syn5to7Signs == wantSigns:
		t.Logf("(시나리오 M3-A) syn[5..7] sign == PST want")
		t.Logf("   → IIR 출력 부호 정상. M3 반증. 결함 위치 = postfilter (M1' 또는 다른).")
	case !fullyInverted:
		t.Logf("(시나리오 M3-B) syn(-u) ≠ -syn(+u) → IIR 비대칭 (saturation/Pass-2)")
		t.Logf("   → M3 유력. F-oct-prelim-5-3 §3.3 결정 재평가 필요 (sample 5..7 한정).")
	case signChanges == 0 && pastSynthStart == ([10]int16{}):
		t.Logf("(시나리오 M3-C) memory propagation 정상 (zero init + sign change 0) + linear IIR")
		t.Logf("   → IIR 산술 spec 정합. M3 반증 (F-oct-prelim-5-3 §3.3 결정 유지).")
		t.Logf("   결함 = excitation 입력 u[] 또는 LP a[] (M5/M6 영역).")
	default:
		t.Logf("(시나리오 M3-D) 부호 불일치 + 선형 IIR — 입력측 결함이 IIR 을 통해 부호 보존되어 전파")
		t.Logf("   → M3 반증, 입력측 결함 유력 (M5 가설 또는 LP 변환).")
	}

	t.Logf("[M3 결정] sample 5..7 동안 memory 부호 변화 = %d/10 pair (non-zero); linear IIR sign-invariant = %v",
		signChanges, fullyInverted)
}

// --- helpers (test-only, package-local; mirrors decoder/testdata_helpers_test.go) ---

func vectorPath(name string) string {
	return filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729AnnexA", "test_vectors", name)
}

func ensureTestdataPresent(tb testing.TB, paths ...string) {
	tb.Helper()
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			tb.Skipf("missing test vector %s: %v", p, err)
		}
	}
}

func readG192Frames(tb testing.TB, path string) ([][]byte, []bool) {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("readG192Frames: %v", err)
	}
	frames, bads, err := bitstream.ReadG192File(bytes.NewReader(data))
	if err != nil {
		tb.Fatalf("ReadG192File(%s): %v", path, err)
	}
	return frames, bads
}

func readPSTFrames(tb testing.TB, path string) [][80]int16 {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("readPSTFrames: %v", err)
	}
	const frameSamples = 80
	if len(data)%(frameSamples*2) != 0 {
		tb.Fatalf("readPSTFrames(%s): size %d not multiple of %d",
			path, len(data), frameSamples*2)
	}
	nFrames := len(data) / (frameSamples * 2)
	out := make([][80]int16, nFrames)
	for i := 0; i < nFrames; i++ {
		for n := 0; n < frameSamples; n++ {
			off := (i*frameSamples + n) * 2
			out[i][n] = int16(binary.LittleEndian.Uint16(data[off : off+2]))
		}
	}
	return out
}

func signOfInt16(v int16) string {
	switch {
	case v > 0:
		return "+"
	case v < 0:
		return "−"
	default:
		return "0"
	}
}
