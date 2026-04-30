package decoder

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

// TestDiagnostic_FoctPrelimPSTFormat: Stage F-oct-prelim-1 진단.
//
// readPSTFrames (testdata_helpers_test.go:30) 의 ALGTHM.PST 해석이
// 16-bit LittleEndian PCM, 80 sample/frame 가정에 정합인지 측정.
//
// 측정 차원:
//
//	(a) 파일 크기 vs 가정 (nFrames × 160 byte).
//	(b) frame 0 raw byte hex dump (160 byte).
//	(c) LittleEndian vs BigEndian sample 0..7 해석.
//	(d) scaling 후보 (값 그대로 / >>1 / <<1) sample 0..7 dump.
//	(e) chain output [+2, +4, +3, +3, +1, +1, +1, +1] 과 cross-correlation.
//	(f) readPSTFrames 호출 결과와 LittleEndian 해석 일치 검증.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelimPSTFormat(t *testing.T) {
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, pstPath)

	data, err := os.ReadFile(pstPath)
	if err != nil {
		t.Fatalf("read PST: %v", err)
	}

	const bytesPerFrame = 160 // 80 sample × 2 byte
	t.Logf("──────── (a) ALGTHM.PST 파일 크기 분석 ────────")
	t.Logf("총 byte: %d", len(data))
	t.Logf("가정 (80 sample/frame × 2 byte): bytesPerFrame = %d", bytesPerFrame)
	t.Logf("nFrames (가정) = %d (잉여 byte = %d)",
		len(data)/bytesPerFrame, len(data)%bytesPerFrame)
	if len(data)%bytesPerFrame != 0 {
		t.Logf("(WARN) 파일 크기가 80-sample frame stride 의 정수배 아님 — G2/G4 의심")
	}

	t.Logf("──────── (b) ALGTHM.PST frame 0 raw byte (160 byte = 80 sample × 2) ────────")
	for off := 0; off < 160 && off < len(data); off += 16 {
		end := off + 16
		if end > len(data) {
			end = len(data)
		}
		var line string
		for i := off; i < end; i++ {
			line += fmt.Sprintf("%02x ", data[i])
		}
		t.Logf("[%04x]  %s", off, line)
	}

	t.Logf("──────── (c) endian 양쪽 해석 (frame 0 sample 0..7) ────────")
	var leSamples, beSamples [8]int16
	for n := 0; n < 8; n++ {
		off := n * 2
		leSamples[n] = int16(binary.LittleEndian.Uint16(data[off : off+2]))
		beSamples[n] = int16(binary.BigEndian.Uint16(data[off : off+2]))
	}
	t.Logf("LittleEndian (현재 readPSTFrames 가정): %v", leSamples)
	t.Logf("BigEndian (대안):                       %v", beSamples)

	t.Logf("──────── (d) scaling 후보 (LittleEndian 기준) sample 0..7 ────────")
	t.Logf("값 그대로 (×1):  %v", leSamples)
	var leHalf, leDouble [8]int32
	for n := 0; n < 8; n++ {
		leHalf[n] = int32(leSamples[n]) >> 1
		leDouble[n] = int32(leSamples[n]) << 1
	}
	t.Logf("PST/2 (>>1):     %v", leHalf)
	t.Logf("PST·2 (<<1):     %v", leDouble)

	chain := [8]int16{+2, +4, +3, +3, +1, +1, +1, +1} // F-sept-4 정합 chain output
	t.Logf("──────── (e) chain vs PST 해석 부호 비교 (sample 0..7) ────────")
	t.Logf("chain output (F-sept-4): %v", chain)
	matchSign := func(a int32, b int16) string {
		signEq := (a > 0 && b > 0) || (a < 0 && b < 0) || (a == 0 && b == 0)
		if signEq {
			return "="
		}
		return "≠"
	}
	for n := 0; n < 8; n++ {
		t.Logf("  [%d]  chain=%+d  LE=%+d (부호%s)  LE/2=%+d (부호%s)  BE=%+d (부호%s)",
			n, chain[n],
			leSamples[n], matchSign(int32(leSamples[n]), chain[n]),
			leHalf[n], matchSign(leHalf[n], chain[n]),
			beSamples[n], matchSign(int32(beSamples[n]), chain[n]))
	}

	want := readPSTFrames(t, pstPath)
	t.Logf("──────── (f) readPSTFrames 결과 vs LE 직접 해석 (frame 0 sample 0..7) ────────")
	t.Logf("readPSTFrames frame 0 sample 0..7: %v", want[0][:8])
	t.Logf("LE 직접 해석            sample 0..7: %v", leSamples)
	identical := true
	for n := 0; n < 8; n++ {
		if want[0][n] != leSamples[n] {
			identical = false
			break
		}
	}
	if identical {
		t.Logf("→ readPSTFrames 출력 = LittleEndian 직접 해석 (예상대로)")
	} else {
		t.Logf("(WARN) readPSTFrames 출력 ≠ LittleEndian 직접 해석 — helper 결함")
	}

	t.Logf("──────── F-oct-prelim-1 시나리오 분류 ────────")
	t.Logf("LE sample 0..7 = %v (현재 가정)", leSamples)
	t.Logf("→ scenario hint: sample 0..4 부호 일치 표 + sample 5..7 부호 분포")
	t.Logf("(P1) LE 해석에서 sample 0..4 모두 chain 부호 일치 → endian 정상")
	t.Logf("(P2) BE 해석에서 sample 0..4 모두 chain 부호 일치 → endian 반대 (helper 결함)")
	t.Logf("(P3) 어떤 endian 에서도 sample 0..4 일관 부호 일치 0 → sample stride 어긋남")
	t.Logf("(P4) PST/2 (>>1) sample 0..4 가 chain 과 정확히 일치 (값 동일) → PST = 2·decode 가설 강화")
	t.Logf("(P5) 위 분류로 결정 불가 → Task 2/3 결합 분석 필요")
}
