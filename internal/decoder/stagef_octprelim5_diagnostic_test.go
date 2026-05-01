package decoder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mainVectorPath builds a path into the main G.729 (non-Annex-A) test-vector
// tree. Used for cross-checking against Annex A vectors (Phase 1k Stage
// F-oct-prelim-5).
func mainVectorPath(name string) string {
	return filepath.Join("..", "..", "testdata", "itu", "G729_Release3",
		"g729", "test_vectors", name)
}

// TestDiagnostic_FoctPrelim5PSTSourceVerbatim: Stage F-oct-prelim-5-1 진단.
//
// Annex A `READMETV.txt` + main G.729 `READMETV.txt` 의 PST 생성 명령
// (`decoder file.bit file.pst`) 와 "Intel (PC) format" 16-bit 인용을
// verbatim 으로 dump. ITU Software Package Release 2 (November 2006)
// header 에 의한 동일 release 라는 사실을 정량 확인.
//
// 동시에 Annex A test_vectors 와 main G.729 test_vectors 의 ALGTHM.BIT
// / ALGTHM.PST / PITCH.BIT / PITCH.PST / FIXED.BIT / FIXED.PST 가
// byte-level 동일한지 검증 (= 동일 release 의 동일 binary 산출물인가).
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelim5PSTSourceVerbatim(t *testing.T) {
	annexAReadme := vectorPath("READMETV.txt")
	mainReadme := mainVectorPath("READMETV.txt")
	ensureTestdataPresent(t, annexAReadme, mainReadme)

	t.Logf("──────── Annex A README header (line 1..21) ────────")
	dumpFirstNLines(t, annexAReadme, 21)

	t.Logf("──────── main G.729 README header (line 1..21) ────────")
	dumpFirstNLines(t, mainReadme, 21)

	t.Logf("──────── byte-level diff: Annex A vs main G.729 test_vectors ────────")
	for _, name := range []string{"ALGTHM.BIT", "ALGTHM.PST", "PITCH.BIT", "PITCH.PST", "FIXED.BIT", "FIXED.PST"} {
		annexA := vectorPath(name)
		main := mainVectorPath(name)
		aData, errA := os.ReadFile(annexA)
		mData, errM := os.ReadFile(main)
		if errA != nil || errM != nil {
			t.Logf("[%s] read error  Annex A=%v  main=%v", name, errA, errM)
			continue
		}
		if bytes.Equal(aData, mData) {
			t.Logf("[%s] Annex A vs main BYTE-EQUAL (%d byte)", name, len(aData))
		} else {
			t.Logf("[%s] Annex A vs main MISMATCH  Annex A=%d byte  main=%d byte",
				name, len(aData), len(mData))
			diffCount, firstDiff := byteDiffSummary(aData, mData)
			t.Logf("[%s] mismatch byte count = %d  (first diff offset = %d)",
				name, diffCount, firstDiff)
		}
	}

	t.Logf("──────── 시나리오 분류 dump ────────")
	t.Logf("(P-SRC-1) Annex A 와 main test_vectors BYTE-EQUAL → PST 생성 binary 동일")
	t.Logf("            본 구현 (G.729A) 는 Annex A binary 와 동일 알고리즘 적용")
	t.Logf("            → PST 가 ground-truth 이며 chain 결함은 *우리 구현 내부* 에 존재.")
	t.Logf("(P-SRC-2) Annex A 와 main test_vectors MISMATCH → PST 생성 binary 분기")
	t.Logf("            우리 구현은 Annex A binary 와 정합해야 함 (g729AnnexA 폴더 사용 정합).")
	t.Logf("            mismatch 는 main G.729 (full postfilter) 의 후속 영향이므로 본 cycle 무관.")
}

// TestDiagnostic_FoctPrelim5BitVectorCompare: Stage F-oct-prelim-5-1 진단.
//
// ALGTHM.BIT / PITCH.BIT / FIXED.BIT 의 frame 0 (10 byte = 80 bit packed)
// 을 byte-level 3-way diff. F-oct-prelim-3 §5 가 세 vector *모두* sample
// 5..7 = 0/3 부호 반전을 동상 측정한 사실의 *공통 silence stimulus*
// 가설 (G3 흡수) 을 정량 검증.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelim5BitVectorCompare(t *testing.T) {
	algthmBit := vectorPath("ALGTHM.BIT")
	pitchBit := vectorPath("PITCH.BIT")
	fixedBit := vectorPath("FIXED.BIT")
	ensureTestdataPresent(t, algthmBit, pitchBit, fixedBit)

	algthmFrames, _ := readG192Frames(t, algthmBit)
	pitchFrames, _ := readG192Frames(t, pitchBit)
	fixedFrames, _ := readG192Frames(t, fixedBit)

	if len(algthmFrames) == 0 || len(pitchFrames) == 0 || len(fixedFrames) == 0 {
		t.Fatalf("empty frames: algthm=%d pitch=%d fixed=%d",
			len(algthmFrames), len(pitchFrames), len(fixedFrames))
	}

	a := algthmFrames[0]
	p := pitchFrames[0]
	f := fixedFrames[0]

	t.Logf("──────── frame 0 raw bytes (10 byte = 80 bit packed) ────────")
	t.Logf("ALGTHM frame 0: %s", hexBytes(a))
	t.Logf("PITCH  frame 0: %s", hexBytes(p))
	t.Logf("FIXED  frame 0: %s", hexBytes(f))

	t.Logf("──────── 3-way byte-level diff (frame 0) ────────")
	for i := 0; i < 10; i++ {
		var ab, pb, fb byte
		if i < len(a) {
			ab = a[i]
		}
		if i < len(p) {
			pb = p[i]
		}
		if i < len(f) {
			fb = f[i]
		}
		mark := "  "
		switch {
		case ab == pb && pb == fb:
			mark = "==" // 3-way 동일
		case ab == pb:
			mark = "AP" // ALGTHM=PITCH ≠ FIXED
		case ab == fb:
			mark = "AF" // ALGTHM=FIXED ≠ PITCH
		case pb == fb:
			mark = "PF" // PITCH=FIXED ≠ ALGTHM
		default:
			mark = "//" // 3-way 모두 상이
		}
		t.Logf("[%d] ALGTHM=%02x PITCH=%02x FIXED=%02x   %s",
			i, ab, pb, fb, mark)
	}

	t.Logf("──────── 시나리오 분류 dump ────────")
	t.Logf("(B-CMP-1) frame 0 BIT byte 3-way 동일 (== 10/10) → 동일 stimulus")
	t.Logf("            → silence-input 정합 가설 강화 (encoder 가 silence 를 동일하게 인코딩)")
	t.Logf("(B-CMP-2) frame 0 BIT byte 일부 다름 → stimulus 분기")
	t.Logf("            → 동일 sample 5..7 = 0/3 결함 발현은 *디코더 출력 메커니즘 공통* 에 기인.")
	t.Logf("            → G3 (Annex A vs main 분기) 영역에서 *분기 위치는 디코더 내부* 임을 함의.")
}

// dumpFirstNLines: file 의 처음 n 라인을 t.Logf 로 출력. 한국어 / ASCII 혼합
// dump 에 사용.
func dumpFirstNLines(t *testing.T, path string, n int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Logf("dumpFirstNLines(%s): %v", path, err)
		return
	}
	lines := strings.Split(string(data), "\n")
	limit := n
	if limit > len(lines) {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		t.Logf("  %2d| %s", i+1, lines[i])
	}
}

// hexBytes: byte slice 를 공백 분리 hex 로 dump.
func hexBytes(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02x", v)
	}
	return strings.Join(parts, " ")
}

// byteDiffSummary: a/b 두 슬라이스의 mismatch byte 수 + 첫 차이 offset.
func byteDiffSummary(a, b []byte) (count int, firstDiff int) {
	firstDiff = -1
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			count++
			if firstDiff < 0 {
				firstDiff = i
			}
		}
	}
	if len(a) != len(b) {
		count += abs(len(a) - len(b))
		if firstDiff < 0 {
			firstDiff = n
		}
	}
	return count, firstDiff
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestDiagnostic_FoctPrelim5HpFilterInitState: Stage F-oct-prelim-5-2 진단.
//
// ITU-T G.729 (06/2012) §4.2.2 식 (151)/(152) HP filter (100 Hz cutoff,
// 2-pole 2-zero IIR) 의 초기 IIR state (x[-1], x[-2], y[-1], y[-2]) 가
// spec 상 0 인지 별도 prescribed value 인지 검증. §4.3 "All filter and
// quantizer states are initialized to zero" 의 zero-init 가정에 정합.
//
// 측정 차원:
//
//(a) production constants (hpB0Q13 / hpB1Q13 / hpB2Q13 / hpNegA1Q12 /
//    hpA2Q13) vs spec real coefficient (0.93980581 / -1.8795834 /
//    0.93980581 / -1.9330735 / 0.93589199) 의 |Δ| 정량.
//(b) zero-input + zero-state 시 sample 0..7 출력 — startup transient
//    가 음수 출력을 생성하는가.
//(c) impulse-input (sample 0 = +1, 그 외 0) + zero-state 시 sample
//    0..7 출력 — IIR step response 의 부호 추세.
//(d) 실제 ALGTHM frame 0 sf0 chain 결과를 hpFilter 입력으로 (= F-sext-1
//    재현) 와 동시에, "spec 상 hpFilter 입력이 0 인 silence frame" 가설
//    검증 — silence input 가정 시 sample 5..7 negative 가 가능한가.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelim5HpFilterInitState(t *testing.T) {
// (a) Q-format quantization error
t.Logf("──────── (a) production Q-format vs spec real coefficient ────────")
type qpair struct {
name     string
qVal     int32
qScale   float64
specReal float64
}
pairs := []qpair{
{"b0", int32(hpB0Q13), 8192.0, 0.93980581},
{"b1", int32(hpB1Q13), 8192.0, -1.8795834},
{"b2", int32(hpB2Q13), 8192.0, 0.93980581},
{"-a1", int32(hpNegA1Q12), 4096.0, 1.9330735}, // production stores |a1| at Q12
{"a2", int32(hpA2Q13), 8192.0, 0.93589199},
}
for _, p := range pairs {
approx := float64(p.qVal) / p.qScale
delta := approx - p.specReal
t.Logf("  %-3s  q=%+6d  approx=%+.8f  spec=%+.8f  |Δ|=%.8f",
p.name, p.qVal, approx, p.specReal, delta)
}

// (b) zero-input + zero-state hpFilter
t.Logf("──────── (b) zero-input + zero-state hpFilter sample 0..7 ────────")
{
var d Decoder
d.Reset()
var in [subframeLen]int16 // 모두 0
var out [subframeLen]int16
d.hpFilter(&in, out[:])
t.Logf("  hpFilter(0...) [0..7] = %v", out[:8])
t.Logf("  hpFilter(0...) [0..%d] all-zero? %v",
subframeLen-1, allZeroInt16(out[:]))
}

// (c) impulse-input (sample 0 = +1) + zero-state hpFilter
t.Logf("──────── (c) impulse(+1 at n=0) + zero-state hpFilter sample 0..7 ────────")
{
var d Decoder
d.Reset()
var in [subframeLen]int16
in[0] = 1
var out [subframeLen]int16
d.hpFilter(&in, out[:])
t.Logf("  hpFilter(δ[0]=+1) [0..7] = %v", out[:8])
t.Logf("  hpFilter(δ[0]=+1) [0..%d] = %v", subframeLen-1, out[:])
}

// (d) impulse-input (sample 0 = +2 = chain output sample 0) + zero-state
t.Logf("──────── (d) chain-like impulse (sample 0 = +2) + zero-state ────────")
{
var d Decoder
d.Reset()
var in [subframeLen]int16
// F-sept-4 chain output sample 0..7 = [2, 4, 3, 3, 1, 1, 1, 1]
// 단 본 측정은 sample 0 만 +2 로 driving — IIR step 단일 응답 분리
in[0] = 2
var out [subframeLen]int16
d.hpFilter(&in, out[:])
t.Logf("  hpFilter(δ[0]=+2) [0..7] = %v", out[:8])
}

// (e) F-sext-1 chain replay (sample 0..7 = [2, 4, 3, 3, 1, 1, 1, 1])
t.Logf("──────── (e) F-sept-4 chain output as hpFilter input + zero-state ────────")
{
var d Decoder
d.Reset()
var in [subframeLen]int16
chain := [8]int16{2, 4, 3, 3, 1, 1, 1, 1}
for i, v := range chain {
in[i] = v
}
// sample 8..39 도 chain output 이 있어야 정확하지만, 본 측정은 sample
// 0..7 의 IIR boundary  관찰 — sample 8.. 는 0 으로 두고 IIR 의 잔향
// 포함.
var out [subframeLen]int16
d.hpFilter(&in, out[:])
t.Logf("  hpFilter(chain[0..7], 0...) [0..7] = %v", out[:8])
t.Logf("  hpFilter expectation = sample 5..7 부호 추적")
for n := 5; n <= 7; n++ {
t.Logf("    [%d]  in=%+d  out=%+d  부호 (in/out) = %s / %s",
n, in[n], out[n],
signOfInt16(in[n]), signOfInt16(out[n]))
}
}

// (f) 시나리오 분류 dump
t.Logf("──────── (f) 시나리오 분류 (hpFilter init state) ────────")
	t.Logf("(H-INIT-1) zero-input + zero-state → hpFilter all-zero")
	t.Logf("            spec §4.3 zero-init 정합. silence frame 0 의 negative")
	t.Logf("            output 메커니즘은 hpFilter 단독으로 *불가능*.")
t.Logf("(H-INIT-2) zero-input + zero-state → hpFilter nonzero")
t.Logf("            spec §4.3 zero-init 위반 또는 production primitive 결함.")
t.Logf("            E2 / E5 발동 검토.")
t.Logf("(H-RESP-1) chain-input + zero-state → sample 5..7 부호 = + (chain 동상)")
t.Logf("            hpFilter 가 sample 5..7 부호 반전 발생시키지 않음 (F-sext-1 §4 동상).")
t.Logf("            negative output 메커니즘은 chain 외부 (= PST 자체) 또는 *상류 결함*.")
t.Logf("(H-RESP-2) chain-input + zero-state → sample 5..7 부호 = − (반전)")
t.Logf("            hpFilter step response 가 startup transient 로 sample 5+ 에서")
t.Logf("            부호 반전 — F-sext-2 가설 정량 확정.")
}

// allZeroInt16: 모든 element 가 0 이면 true.
func allZeroInt16(s []int16) bool {
for _, v := range s {
if v != 0 {
return false
}
}
return true
}
