package g729

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hunydev/g729/internal/fcbsearch"
	"github.com/hunydev/g729/internal/fixed"
	"github.com/hunydev/g729/internal/gainquant"
	"github.com/hunydev/g729/internal/pitch/closedloop"
	"github.com/hunydev/g729/internal/synth"
)

const (
	encoderClosedLoopHandoffDir      = "testdata/oracle/handoff"
	encoderClosedLoopExpectedName    = "encoder_closedloop_stage_expected_template.csv"
	encoderClosedLoopGotName         = "encoder_closedloop_stage_got.csv"
	encoderClosedLoopVerifierPrompt  = "ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md"
	encoderClosedLoopFrameSamplePath = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/SPEECH.IN"
)

type encoderClosedLoopHandoffRow struct {
	field string
	frame int
	sub   int
	index int
	lag   int
	frac  int
	value int64
}

type encoderClosedLoopHandoffKey struct {
	field string
	frame int
	sub   int
	index int
	lag   int
	frac  int
}

func TestOracleHandoff_WriteEncoderClosedLoopStageHandoff(t *testing.T) {
	if os.Getenv("G729_WRITE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF") != "1" {
		t.Skip("set G729_WRITE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 to refresh encoder closed-loop stage handoff files")
	}

	rows := collectEncoderClosedLoopHandoffRows(t, encoderClosedLoopTargetFrames())
	if len(rows) == 0 {
		t.Fatal("no encoder closed-loop handoff rows collected")
	}

	if err := os.MkdirAll(encoderClosedLoopHandoffDir, 0o755); err != nil {
		t.Fatalf("mkdir handoff dir: %v", err)
	}
	expectedPath := filepath.Join(encoderClosedLoopHandoffDir, encoderClosedLoopExpectedName)
	gotPath := filepath.Join(encoderClosedLoopHandoffDir, encoderClosedLoopGotName)
	if err := writeEncoderClosedLoopHandoffCSV(expectedPath, "expected", rows, true); err != nil {
		t.Fatalf("write expected template: %v", err)
	}
	if err := writeEncoderClosedLoopHandoffCSV(gotPath, "got", rows, false); err != nil {
		t.Fatalf("write got: %v", err)
	}
	t.Logf("wrote %d encoder closed-loop stage handoff rows", len(rows))
}

func TestOracleHandoff_CompareEncoderClosedLoopStageHandoff(t *testing.T) {
	if os.Getenv("G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF") != "1" {
		t.Skip("set G729_COMPARE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF=1 after verifier fills encoder_closedloop_stage_expected_template.csv")
	}

	expectedPath := filepath.Join(encoderClosedLoopHandoffDir, encoderClosedLoopExpectedName)
	expected, blanks, err := readEncoderClosedLoopExpectedHandoff(expectedPath)
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if len(expected) == 0 {
		t.Fatalf("expected handoff has no rows")
	}

	targets := make(map[int]struct{})
	for _, row := range expected {
		targets[row.frame] = struct{}{}
	}
	gotRows := collectEncoderClosedLoopHandoffRows(t, sortedEncoderClosedLoopFrameKeys(targets))
	got := make(map[encoderClosedLoopHandoffKey]int64, len(gotRows))
	for _, row := range gotRows {
		got[encoderClosedLoopHandoffRowKey(row)] = row.value
	}

	var exact, mismatch, missing int
	first := make([]string, 0, 12)
	for _, want := range expected {
		if want.value == encoderClosedLoopBlankExpected {
			continue
		}
		key := encoderClosedLoopHandoffRowKey(want)
		have, ok := got[key]
		if !ok {
			missing++
			mismatch++
			if len(first) < cap(first) {
				first = append(first, fmt.Sprintf("missing got: %+v expected=%d", key, want.value))
			}
			continue
		}
		if have == want.value {
			exact++
			continue
		}
		mismatch++
		if len(first) < cap(first) {
			first = append(first, fmt.Sprintf("mismatch: %+v expected=%d got=%d", key, want.value, have))
		}
	}

	filled := len(expected) - blanks
	if filled == 0 {
		t.Fatalf("expected handoff has no filled numeric cells; verifier output is required before comparison")
	}
	t.Logf("encoder closed-loop stage handoff: exact %d/%d %.2f%% mismatches=%d blanks=%d missing=%d",
		exact, filled, percent(exact, filled), mismatch, blanks, missing)
	for _, msg := range first {
		t.Log(msg)
	}
	if os.Getenv("G729_REQUIRE_COMPLETE_ENCODER_CLOSEDLOOP_STAGE_HANDOFF") == "1" && blanks > 0 {
		t.Fatalf("expected handoff still has %d blank cells", blanks)
	}
	if os.Getenv("G729_REQUIRE_EXACT_ENCODER_CLOSEDLOOP_STAGE_HANDOFF") == "1" && mismatch > 0 {
		t.Fatalf("encoder closed-loop stage handoff has %d mismatches", mismatch)
	}
}

func encoderClosedLoopTargetFrames() []int {
	return []int{
		0, 1, 2, 3, 4, 5,
		100, 101,
		500, 501,
		1000, 1001,
		1500, 1501,
		2000, 2001,
		2500, 2501,
		2750, 2751,
		3000, 3001,
		3500, 3501,
	}
}

func collectEncoderClosedLoopHandoffRows(t *testing.T, targetFrames []int) []encoderClosedLoopHandoffRow {
	t.Helper()
	targets := make(map[int]struct{}, len(targetFrames))
	maxFrame := 0
	for _, frame := range targetFrames {
		targets[frame] = struct{}{}
		if frame > maxFrame {
			maxFrame = frame
		}
	}

	data, err := os.ReadFile(encoderClosedLoopFrameSamplePath)
	if err != nil {
		t.Fatalf("read %s: %v", encoderClosedLoopFrameSamplePath, err)
	}
	samples := handoffS16LEToSamples(data)
	if have, want := len(samples)/FrameSamples, maxFrame+1; have < want {
		t.Fatalf("SPEECH.IN has %d frames, need %d", have, want)
	}

	enc := NewEncoder()
	rows := make([]encoderClosedLoopHandoffRow, 0, len(targetFrames)*2*2102)
	for frame := 0; frame <= maxFrame; frame++ {
		off := frame * FrameSamples
		if _, err := enc.lpcStep(samples[off : off+FrameSamples]); err != nil {
			t.Fatalf("lpcStep frame %d: %v", frame, err)
		}
		_ = enc.openloopStep()
		want := false
		if _, ok := targets[frame]; ok {
			want = true
		}
		enc.closedloopStepHandoff(frame, 0, want, &rows)
		enc.closedloopStepHandoff(frame, 1, want, &rows)
	}
	return rows
}

func (e *Encoder) closedloopStepHandoff(frame, sub int, collect bool, rows *[]encoderClosedLoopHandoffRow) {
	aHat := &e.aHatSF1
	if sub == 1 {
		aHat = &e.aHatSF2
	}
	sStart := 120 + 40*sub
	sFrame := (*[40]int16)(e.oldSpeech[sStart : sStart+40])

	var r, x, h, xb, v, y [closedloop.SubframeLen]int16
	lpResidualSubframe(sFrame, aHat, &e.lpResidualMemQ, &r)
	closedloop.TargetSignal(aHat, &r, &e.swMemErr, &x)
	closedloop.ImpulseResponse(aHat, &h)
	closedloop.BackwardFilter(&x, &h, &xb)

	centre := e.tOp
	if sub == 1 {
		centre = e.intT1
	}
	var excSearch [closedloop.PitchMaxInt + closedloop.SubframeLen]int16
	copy(excSearch[:closedloop.PitchMaxInt], e.oldExc[len(e.oldExc)-closedloop.PitchMaxInt:])
	copy(excSearch[closedloop.PitchMaxInt:], r[:])
	excSlice := excSearch[:]
	intLag, _ := closedloop.SearchInteger(&xb, excSlice, centre, sub)
	frac := closedloop.RefineFraction(&xb, excSlice, intLag, sub == 1 || intLag < 85)
	closedloop.AdaptiveVector(excSlice, intLag, frac, &v)
	gp := closedloop.GpAndY(&x, &v, &h, &y)

	if sub == 0 {
		e.intT1 = intLag
		e.frac1 = frac
		e.p1 = closedloop.EncodeP1(intLag, frac)
		e.p0 = closedloop.EncodeP0(e.p1)
	} else {
		tmin, _ := closedloop.Subframe2Window(e.intT1)
		e.intT2 = intLag
		e.frac2 = frac
		e.p2 = closedloop.EncodeP2(intLag, frac, tmin)
	}

	e.fcbStepHandoff(frame, sub, collect, rows, &r, &x, &h, &xb, &v, &y, gp, intLag, frac)
	copy(e.lpResidualMemQ[:], sFrame[30:40])
}

func (e *Encoder) fcbStepHandoff(
	frame, sub int,
	collect bool,
	rows *[]encoderClosedLoopHandoffRow,
	r, x, h, xb, v, y *[closedloop.SubframeLen]int16,
	gpUnq int16,
	intLag int16,
	frac int8,
) {
	const N = closedloop.SubframeLen
	var xPrime [N]int16
	fcbsearch.AdjustedTarget(x, y, gpUnq, &xPrime)

	var d [N]int32
	fcbsearch.CorrelationD(&xPrime, h, &d)

	var signs [N]int16
	var dAbs [N]int32
	fcbsearch.SignsFromD(&d, &signs, &dAbs)

	var phi [N][N]int32
	fcbsearch.PhiPrime(h, &signs, &phi)

	var positions [4]int8
	var sumOut [2]int64
	fcbsearch.SearchDepthFirst(&dAbs, &phi, &positions, &sumOut)

	var c [N]int16
	fcbsearch.BuildCode(&positions, &signs, intLag, e.prevGpQ14, &c)

	var z [N]int16
	fcbsearch.FilterCode(&c, h, &z)

	gpcPredQ12 := gainquant.PredictedGcQ12(&e.pastQuaEn, &c)
	const gainSearchFixedContributionScale = 4
	gpcSearchQ12 := scaleInt32ForGainSearch(gpcPredQ12, gainSearchFixedContributionScale)
	gaPhys, gbPhys, gpHatQ14, gammaCQ13 := gainquant.SearchConjugate(x, y, &z, gpcSearchQ12)
	gpHatQ14 = gainquant.Tame(gpHatQ14, &e.oldExc)
	gaBits, gbBits := gainquant.PackGains(gaPhys, gbPhys)
	s := fcbsearch.PackS(&positions, &signs)
	cPacked := fcbsearch.PackC(&positions)
	if sub == 0 {
		e.s1, e.c1, e.ga1, e.gb1 = s, cPacked, gaBits, gbBits
	} else {
		e.s2, e.c2, e.ga2, e.gb2 = s, cPacked, gaBits, gbBits
	}

	if collect {
		appendEncoderClosedLoopRows(rows, frame, sub, int(intLag), int(frac), r, x, h, xb, v, y, &xPrime, &d, &signs, &dAbs, &phi, &positions, &c, &z, gpUnq, s, cPacked, gaBits, gbBits, gpcPredQ12)
	}

	_, gcMantQ14, gcExp := gainquant.Reconstruct(&e.pastQuaEn, &c, gaPhys, gbPhys)
	for n := 30; n < N; n++ {
		gpY := applyGainQ14ToQ0(gpHatQ14, y[n])
		gcZ := applyGcToQ12(gcMantQ14, gcExp, z[n])
		e.swMemErr[n-30] = fixed.Saturate(int32(x[n]) - gpY - gcZ)
	}

	copy(e.oldExc[:len(e.oldExc)-N], e.oldExc[N:])
	base := len(e.oldExc) - N
	var uHat [N]int16
	synth.BuildExcitation(gpHatQ14, gcMantQ14, gcExp, v, &c, &uHat)
	copy(e.oldExc[base:], uHat[:])
	gainquant.UpdatePastQuaEn(&e.pastQuaEn, gammaCQ13)
	e.prevGpQ14 = gpHatQ14
	e.prevTaming = false
}

func appendEncoderClosedLoopRows(
	rows *[]encoderClosedLoopHandoffRow,
	frame, sub, lag, frac int,
	r, x, h, xb, v, y, xPrime *[closedloop.SubframeLen]int16,
	d *[closedloop.SubframeLen]int32,
	signs *[closedloop.SubframeLen]int16,
	dAbs *[closedloop.SubframeLen]int32,
	phi *[closedloop.SubframeLen][closedloop.SubframeLen]int32,
	positions *[4]int8,
	c, z *[closedloop.SubframeLen]int16,
	gpUnq int16,
	s uint8,
	cPacked uint16,
	gaBits, gbBits uint8,
	gpcPredQ12 int32,
) {
	add := func(field string, index int, value int64) {
		*rows = append(*rows, encoderClosedLoopHandoffRow{
			field: field,
			frame: frame,
			sub:   sub,
			index: index,
			lag:   lag,
			frac:  frac,
			value: value,
		})
	}
	add("pitch_int", -1, int64(lag))
	add("pitch_frac", -1, int64(frac))
	add("unquant_gp_q14", -1, int64(gpUnq))
	add("s_bits", -1, int64(s))
	add("c_bits", -1, int64(cPacked))
	add("ga_bits", -1, int64(gaBits))
	add("gb_bits", -1, int64(gbBits))
	add("gpc_pred_q12", -1, int64(gpcPredQ12))

	appendVector16Rows(rows, frame, sub, lag, frac, "r", r)
	appendVector16Rows(rows, frame, sub, lag, frac, "x", x)
	appendVector16Rows(rows, frame, sub, lag, frac, "h", h)
	appendVector16Rows(rows, frame, sub, lag, frac, "xb", xb)
	appendVector16Rows(rows, frame, sub, lag, frac, "v", v)
	appendVector16Rows(rows, frame, sub, lag, frac, "y", y)
	appendVector16Rows(rows, frame, sub, lag, frac, "x_prime", xPrime)
	appendVector16Rows(rows, frame, sub, lag, frac, "sign", signs)
	appendVector16Rows(rows, frame, sub, lag, frac, "c", c)
	appendVector16Rows(rows, frame, sub, lag, frac, "z", z)
	appendVector32Rows(rows, frame, sub, lag, frac, "d", d)
	appendVector32Rows(rows, frame, sub, lag, frac, "d_abs", dAbs)

	for i, p := range positions {
		add("fcb_position", i, int64(p))
		add("fcb_position_sign", i, int64(signs[p]))
	}
	for i := 0; i < closedloop.SubframeLen; i++ {
		for j := 0; j < closedloop.SubframeLen; j++ {
			add("phi", i*closedloop.SubframeLen+j, int64(phi[i][j]))
		}
	}

	var A, B, C, D, F int64
	for i := 0; i < closedloop.SubframeLen; i++ {
		xi := int64(x[i])
		yi := int64(y[i])
		zi := int64(z[i])
		A += (yi * yi) << 24
		B += zi * zi
		C += (yi * zi) << 12
		D += (xi * yi) << 24
		F += (xi * zi) << 12
	}
	add("gain_corr_A", -1, A)
	add("gain_corr_B", -1, B)
	add("gain_corr_C", -1, C)
	add("gain_corr_D", -1, D)
	add("gain_corr_F", -1, F)
}

func appendVector16Rows(rows *[]encoderClosedLoopHandoffRow, frame, sub, lag, frac int, field string, v *[closedloop.SubframeLen]int16) {
	for i, x := range v {
		*rows = append(*rows, encoderClosedLoopHandoffRow{field: field, frame: frame, sub: sub, index: i, lag: lag, frac: frac, value: int64(x)})
	}
}

func appendVector32Rows(rows *[]encoderClosedLoopHandoffRow, frame, sub, lag, frac int, field string, v *[closedloop.SubframeLen]int32) {
	for i, x := range v {
		*rows = append(*rows, encoderClosedLoopHandoffRow{field: field, frame: frame, sub: sub, index: i, lag: lag, frac: frac, value: int64(x)})
	}
}

func writeEncoderClosedLoopHandoffCSV(path, valueColumn string, rows []encoderClosedLoopHandoffRow, blankValue bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"field", "frame", "sub", "index", "lag", "frac", valueColumn}); err != nil {
		return err
	}
	for _, row := range rows {
		value := strconv.FormatInt(row.value, 10)
		if blankValue {
			value = ""
		}
		rec := []string{
			row.field,
			strconv.Itoa(row.frame),
			strconv.Itoa(row.sub),
			strconv.Itoa(row.index),
			strconv.Itoa(row.lag),
			strconv.Itoa(row.frac),
			value,
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return w.Error()
}

const encoderClosedLoopBlankExpected = int64(-1 << 63)

func readEncoderClosedLoopExpectedHandoff(path string) ([]encoderClosedLoopHandoffRow, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, 0, err
	}
	wantHeader := []string{"field", "frame", "sub", "index", "lag", "frac", "expected"}
	if fmt.Sprint(header) != fmt.Sprint(wantHeader) {
		return nil, 0, fmt.Errorf("unexpected header %v", header)
	}
	var rows []encoderClosedLoopHandoffRow
	var blanks int
	line := 1
	for {
		rec, err := r.Read()
		line++
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: %w", line, err)
		}
		if len(rec) != 7 {
			return nil, 0, fmt.Errorf("line %d: columns=%d, want 7", line, len(rec))
		}
		row, err := parseEncoderClosedLoopHandoffKey(rec, line)
		if err != nil {
			return nil, 0, err
		}
		if rec[6] == "" {
			row.value = encoderClosedLoopBlankExpected
			blanks++
		} else {
			value, err := strconv.ParseInt(rec[6], 10, 64)
			if err != nil {
				return nil, 0, fmt.Errorf("line %d expected: %w", line, err)
			}
			row.value = value
		}
		rows = append(rows, row)
	}
	return rows, blanks, nil
}

func parseEncoderClosedLoopHandoffKey(rec []string, line int) (encoderClosedLoopHandoffRow, error) {
	frame, err := strconv.Atoi(rec[1])
	if err != nil {
		return encoderClosedLoopHandoffRow{}, fmt.Errorf("line %d frame: %w", line, err)
	}
	sub, err := strconv.Atoi(rec[2])
	if err != nil {
		return encoderClosedLoopHandoffRow{}, fmt.Errorf("line %d sub: %w", line, err)
	}
	index, err := strconv.Atoi(rec[3])
	if err != nil {
		return encoderClosedLoopHandoffRow{}, fmt.Errorf("line %d index: %w", line, err)
	}
	lag, err := strconv.Atoi(rec[4])
	if err != nil {
		return encoderClosedLoopHandoffRow{}, fmt.Errorf("line %d lag: %w", line, err)
	}
	frac, err := strconv.Atoi(rec[5])
	if err != nil {
		return encoderClosedLoopHandoffRow{}, fmt.Errorf("line %d frac: %w", line, err)
	}
	return encoderClosedLoopHandoffRow{field: rec[0], frame: frame, sub: sub, index: index, lag: lag, frac: frac}, nil
}

func encoderClosedLoopHandoffRowKey(row encoderClosedLoopHandoffRow) encoderClosedLoopHandoffKey {
	return encoderClosedLoopHandoffKey{
		field: row.field,
		frame: row.frame,
		sub:   row.sub,
		index: row.index,
		lag:   row.lag,
		frac:  row.frac,
	}
}

func sortedEncoderClosedLoopFrameKeys(m map[int]struct{}) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		v := out[i]
		j := i - 1
		for j >= 0 && out[j] > v {
			out[j+1] = out[j]
			j--
		}
		out[j+1] = v
	}
	return out
}

func handoffS16LEToSamples(data []byte) []int16 {
	out := make([]int16, len(data)/2)
	for i := range out {
		out[i] = int16(uint16(data[2*i]) | uint16(data[2*i+1])<<8)
	}
	return out
}
