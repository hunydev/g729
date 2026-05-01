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
