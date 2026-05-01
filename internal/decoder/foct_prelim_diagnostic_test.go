package decoder

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
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

// TestDiagnostic_FoctPrelimFrameAlignment: Stage F-oct-prelim-2 진단.
//
// ALGTHM.BIT frame 0..3 production 디코딩 결과 sample 0..7 와
// ALGTHM.PST frame 0..3 sample 0..7 의 cross-correlation 으로 frame
// indexing 정합성 측정. 가설 G2 (frame indexing mismatch) 검증.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelimFrameAlignment(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	bitFrames, bads := readG192Frames(t, bitPath)
	pstFrames := readPSTFrames(t, pstPath)

	t.Logf("──────── (a) frame count 비교 ────────")
	t.Logf("BIT frame 수: %d", len(bitFrames))
	t.Logf("PST frame 수: %d", len(pstFrames))
	if len(bitFrames) != len(pstFrames) {
		t.Logf("(WARN) frame 수 불일치 — preroll / trailing silence 가능성 (Δ = %d)",
			len(pstFrames)-len(bitFrames))
	}

	const N = 4
	var bitSamples [N][8]int16
	var dec Decoder
	for i := 0; i < N && i < len(bitFrames); i++ {
		var out [80]int16
		bad := false
		if i < len(bads) {
			bad = bads[i]
		}
		if err := dec.Decode(bitFrames[i], bad, out[:]); err != nil {
			t.Fatalf("Decode[%d]: %v", i, err)
		}
		copy(bitSamples[i][:], out[:8])
		t.Logf("BIT[%d] decoded sample 0..7: %v", i, bitSamples[i])
	}

	var pstSamples [N][8]int16
	for j := 0; j < N && j < len(pstFrames); j++ {
		copy(pstSamples[j][:], pstFrames[j][:8])
		t.Logf("PST[%d] sample 0..7:        %v", j, pstSamples[j])
	}

	t.Logf("──────── (d) (BIT[i], PST[j]) sample 0..7 부호 매칭 점수 (0~8) ────────")
	t.Logf("       PST[0]  PST[1]  PST[2]  PST[3]")
	signMatchScore := func(a, b [8]int16) int {
		score := 0
		for n := 0; n < 8; n++ {
			as := signOfInt16(a[n])
			bs := signOfInt16(b[n])
			if as == bs {
				score++
			}
		}
		return score
	}
	type pair struct{ i, j, score int }
	var best pair
	for i := 0; i < N; i++ {
		row := fmt.Sprintf("BIT[%d]  ", i)
		for j := 0; j < N; j++ {
			s := signMatchScore(bitSamples[i], pstSamples[j])
			row += fmt.Sprintf("%4d    ", s)
			if s > best.score {
				best = pair{i, j, s}
			}
		}
		t.Logf("%s", row)
	}

	t.Logf("──────── (e) PST/2 부호 매칭 점수 표 (PST[j]>>1 와 BIT[i] 비교) ────────")
	t.Logf("       PST/2[0]  PST/2[1]  PST/2[2]  PST/2[3]")
	for i := 0; i < N; i++ {
		row := fmt.Sprintf("BIT[%d]    ", i)
		for j := 0; j < N; j++ {
			var halved [8]int16
			for n := 0; n < 8; n++ {
				halved[n] = int16(int32(pstSamples[j][n]) >> 1)
			}
			s := signMatchScore(bitSamples[i], halved)
			row += fmt.Sprintf("%4d      ", s)
		}
		t.Logf("%s", row)
	}

	t.Logf("──────── F-oct-prelim-2 시나리오 분류 ────────")
	t.Logf("최대 매칭: BIT[%d] ↔ PST[%d] 점수=%d/8", best.i, best.j, best.score)
	t.Logf("(F1) best 가 (i=j=0) 이고 score≥6 → 정상 alignment, G2 반증")
	t.Logf("(F2) best 가 (i=0, j=1) → PST 가 1 frame skip / preroll, G2 발현")
	t.Logf("(F3) best 가 |j−i|>1 → multi-frame skip, G2 강하게 발현")
	t.Logf("(F4) 모든 score≤4 → 매칭 0, G2 반증 + G1/G4/G5 우세")
}

// TestDiagnostic_FoctPrelimMultiVectorScan: Stage F-oct-prelim-3 진단.
//
// 6 ITU vector (ALGTHM, TEST, SPEECH, LSP, PITCH, FIXED) 의 frame 0
// sf0 sample 0..7 production 디코딩 결과와 PST want 의 sample 5..7
// 부호 비교. ALGTHM 특이성 (G5) vs 일반 결함 (G1/G3/G4) 분리.
//
// 측정-only — assertion 0. production 변경 0 (E5).
func TestDiagnostic_FoctPrelimMultiVectorScan(t *testing.T) {
	type vec struct {
		name             string
		bitName, pstName string
	}
	vectors := []vec{
		{"ALGTHM", "ALGTHM.BIT", "ALGTHM.PST"},
		{"TEST", "TEST.BIT", "TEST.pst"},
		{"SPEECH", "SPEECH.BIT", "SPEECH.PST"},
		{"LSP", "LSP.BIT", "LSP.PST"},
		{"PITCH", "PITCH.BIT", "PITCH.PST"},
		{"FIXED", "FIXED.BIT", "FIXED.PST"},
	}

	type result struct {
		name           string
		prod           [8]int16
		want           [8]int16
		matchCount5to7 int
		signs          [8]string
	}
	var results []result

	for _, v := range vectors {
		bitPath := vectorPath(v.bitName)
		pstPath := vectorPath(v.pstName)
		if _, err := os.Stat(pstPath); err != nil {
			alt := vectorPath(strings.ToUpper(v.pstName))
			if _, err2 := os.Stat(alt); err2 == nil {
				pstPath = alt
			} else {
				altLow := vectorPath(strings.ToLower(v.pstName))
				if _, err3 := os.Stat(altLow); err3 == nil {
					pstPath = altLow
				}
			}
		}
		if _, err := os.Stat(bitPath); err != nil {
			t.Logf("vector %s: BIT missing (%v) — skip", v.name, err)
			continue
		}
		if _, err := os.Stat(pstPath); err != nil {
			t.Logf("vector %s: PST missing (%v) — skip", v.name, err)
			continue
		}

		bitFrames, bads := readG192Frames(t, bitPath)
		pstFrames := readPSTFrames(t, pstPath)
		if len(bitFrames) == 0 || len(pstFrames) == 0 {
			t.Logf("vector %s: empty frames — skip", v.name)
			continue
		}

		var dec Decoder
		var out [80]int16
		bad := false
		if len(bads) > 0 {
			bad = bads[0]
		}
		if err := dec.Decode(bitFrames[0], bad, out[:]); err != nil {
			t.Logf("vector %s: Decode error %v — skip", v.name, err)
			continue
		}

		var r result
		r.name = v.name
		copy(r.prod[:], out[:8])
		copy(r.want[:], pstFrames[0][:8])
		for n := 0; n < 8; n++ {
			if signOfInt16(r.prod[n]) == signOfInt16(r.want[n]) {
				r.signs[n] = "="
			} else {
				r.signs[n] = "≠"
			}
		}
		for n := 5; n <= 7; n++ {
			if r.signs[n] == "=" {
				r.matchCount5to7++
			}
		}
		results = append(results, r)
	}

	t.Logf("──────── (a) 6 vector × frame 0 sf0 sample 0..7 ────────")
	for _, r := range results {
		t.Logf("[%s]", r.name)
		t.Logf("  prod = %v", r.prod)
		t.Logf("  want = %v", r.want)
		t.Logf("  sign = %v  (sample 5..7 일치 %d/3)", r.signs, r.matchCount5to7)
	}

	t.Logf("──────── (b) sample 5..7 부호 일치 분포 요약 ────────")
	t.Logf("vector       sample5  sample6  sample7  match5..7")
	allMatch := len(results) > 0
	allMismatch := len(results) > 0
	for _, r := range results {
		t.Logf("  %-10s   %s        %s        %s        %d/3",
			r.name, r.signs[5], r.signs[6], r.signs[7], r.matchCount5to7)
		if r.matchCount5to7 < 3 {
			allMatch = false
		}
		if r.matchCount5to7 > 0 {
			allMismatch = false
		}
	}

	t.Logf("──────── F-oct-prelim-3 시나리오 분류 ────────")
	switch {
	case allMatch:
		t.Logf("(V1) 모든 vector 가 sample 5..7 부호 정합 → ALGTHM 도 정합 ?")
		t.Logf("     (이 case 가 발현하면 ALGTHM 자체 측정에 모순 — F-sept-4 회귀 의심)")
	case allMismatch:
		t.Logf("(V2) 모든 vector 에서 sample 5..7 부호 반전 → 일반 결함 (G1/G3/G4)")
		t.Logf("     F-oct 권고: chain 외부 결함 추적 (PST format / hpFilter startup / PST/2 가설)")
	default:
		t.Logf("(V3) mixed — 일부 vector 정합 / 일부 반전")
		t.Logf("     ALGTHM-specific 거동 (G5 발현) 가능성 + vector-specific 분포 분석 의무")
	}
}
