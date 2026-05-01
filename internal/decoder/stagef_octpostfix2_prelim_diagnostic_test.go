package decoder

import (
	"testing"

	"github.com/exedev/g729/internal/bitstream"
	"github.com/exedev/g729/internal/fcb"
	"github.com/exedev/g729/internal/gain"
	"github.com/exedev/g729/internal/lsp"
	"github.com/exedev/g729/internal/pitch"
	"github.com/exedev/g729/internal/postfilter"
	"github.com/exedev/g729/internal/synth"
)

// TestDiagnostic_FoctPostfix2PrelimChainDump dumps the Annex A postfilter
// chain stage outputs for ALGTHM frame 0 sf0 sample 5..7 — the common
// ground-truth for Tasks F-oct-postfix2-prelim-2/3/4 (M5/M6/M1'+M3
// hypothesis differential measurement).
//
// Spec ground-truth: ITU-T G.729 (06/2012) §A.4.2 (PDF p.43) chain
// order = long-term → short-term → tilt → AGC. F-oct-postfix synthesis
// (8907847) §2.4 identifies the sign-determining term as residing
// *outside* tilt compensation (Δ=0 measurement); this dump enables
// stage-by-stage sign tracing.
//
// production 변경 0. assertion 0 (measurement-only).
func TestDiagnostic_FoctPostfix2PrelimChainDump(t *testing.T) {
	bitPath := vectorPath("ALGTHM.BIT")
	pstPath := vectorPath("ALGTHM.PST")
	ensureTestdataPresent(t, bitPath, pstPath)

	frames, bads := readG192Frames(t, bitPath)
	wantFrames := readPSTFrames(t, pstPath)

	var d Decoder
	var out [frameSamples]int16
	if err := d.Decode(frames[0], bads[0], out[:]); err != nil {
		t.Fatalf("Decode frame 0: %v", err)
	}

	t.Logf("ALGTHM frame 0 sf0 sample 5..7 (PST want = [%d %d %d])",
		wantFrames[0][5], wantFrames[0][6], wantFrames[0][7])
	t.Logf("  decoded out[5..7] (post-hpfilter)            = [%d %d %d]",
		out[5], out[6], out[7])
	t.Logf("  delta vs PST want                            = [%d %d %d]",
		int32(out[5])-int32(wantFrames[0][5]),
		int32(out[6])-int32(wantFrames[0][6]),
		int32(out[7])-int32(wantFrames[0][7]))
	// Additional stage dumps (excitation, synth IIR, postfilter chain)
	// are added in Tasks 2/4 via stage-specific harnesses or Decoder
	// instrumentation hooks if exposed; this baseline records the
	// externally observable terminal output for cross-reference.
}

// TestDiagnostic_FoctPostfix2PrelimM5ExcitationSignTrace measures the
// M5 hypothesis (pre-postfilter sign defect) for ALGTHM frame 0 sf0
// sample 5..7 — a 3-point sign trace across the excitation, synth IIR
// output, and pre-postfilter input. F-sept-1 covered sample 5 only and
// F-sept-3 traced synth IIR sample 0..7 without identifying the
// sign-determining stage; this test fills the sample 5..7 gap and
// surfaces the inter-stage sign transitions for M5 acceptance/rejection.
//
// Spec ground-truth (PDF verbatim grep, see report §0):
//   - ITU-T G.729 (06/A.4.1 — "Same as described in clause 4.1"2012) 
//     (excitation reconstruction = §4.1.5: u(n) = ĝ_p · v(n) + ĝ_c · c(n)).
//   - ITU-T G.729 (06/2012) §A.4.2 (PDF p.43) — postfilter cascade
//     (long-term → short-term → tilt → AGC), input s(n) = synthesis
//     filter 1/Â(z) output (Annex A simplification: pre-IIR variant is
//     not used; the conventional 1/Â(z/γd) cascade is replaced by the
//     short-term postfilter Hf alone).
//
// Note (E2 declaration): Plan §Task 2 cited "§A.3.5 Excitation
// reconstruction"; PDF grep shows §A.3.5 = "Computation of the impulse
// response" (encoder side). The substantive excitation citation is
// §A.4.1 → §4.1.5 as recorded above. Citation correction documented in
// the report §0 per E2.
//
// production 변경 0. assertion 0 (measurement-only).
func TestDiagnostic_FoctPostfix2PrelimM5ExcitationSignTrace(t *testing.T) {
bitPath := vectorPath("ALGTHM.BIT")
pstPath := vectorPath("ALGTHM.PST")
ensureTestdataPresent(t, bitPath, pstPath)

frames, _ := readG192Frames(t, bitPath)
wantFrames := readPSTFrames(t, pstPath)

var f bitstream.Frame
if err := bitstream.Unpack(frames[0], &f); err != nil {
t.Fatalf("Unpack frame 0: %v", err)
}

// (1) sf0 LP coefficients (Q12) — F-sept 동상 path.
var lspDec lsp.Decoder
lspDec.Reset()
sfA, _ := lspDec.Decode(lsp.Indices{
L0: uint8(f.L0), L1: uint8(f.L1), L2: uint8(f.L2), L3: uint8(f.L3),
})

// (2) pitch / fcb / gain / excitation u[]
tInt, tFrac := pitch.DecodeDelaySubframe1(uint8(f.P1))
var pastExc [pastExcLen]int16
var v [subframeLen]int16
pitch.AdaptiveCodebook(tInt, tFrac, pastExc[:], &v)
betaQ14 := fcb.ClampPitchGainForEnhancement(0)
var c [subframeLen]int16
fcb.Decode(fcb.Indices{Positions: f.C1, Signs: uint8(f.S1)}, tInt, betaQ14, &c)
var gn gain.Decoder
gn.Reset()
gpQ14, gcQ12 := gn.Decode(gain.Indices{GA: uint8(f.GA1), GB: uint8(f.GB1)}, &c)
var u [subframeLen]int16
synth.BuildExcitation(gpQ14, gcQ12, &v, &c, &u)

// (3) production synth.Filter — 합성 IIR (s == pre-postfilter input).
var syn synth.Synthesizer
syn.Reset()
var s [subframeLen]int16
syn.Filter(&sfA, &u, &s)

// (4) production postfilter.Filter — postfilter chain output sPf.
var pst postfilter.Postfilter
var sPf [subframeLen]int16
pst.Filter(&sfA, tInt, &s, &sPf)

// (5) sample 5..7  3-point sign trace (excitation u → syn IIR s →
//     pre-postfilter (= s) ; sPf 와 PST want 는 cross-check 로 병기).
t.Logf("-------- M5 excitation pre-postfilter sign trace (ALGTHM frame 0 sf0) --------")
t.Logf("PST want sample 5..7 = [%+d %+d %+d]  (signs=[%s %s %s])",
wantFrames[0][5], wantFrames[0][6], wantFrames[0][7],
signOfInt16(wantFrames[0][5]), signOfInt16(wantFrames[0][6]), signOfInt16(wantFrames[0][7]))
for n := 5; n <= 7; n++ {
t.Logf("[M5 sample %d] excitation u[%d]=%+6d  syn[%d]=%+6d  pre-post[%d]=%+6d  postfilter sPf[%d]=%+6d  sign chain=[%s,%s,%s,%s]",
n, n, u[n], n, s[n], n, s[n], n, sPf[n],
signOfInt16(u[n]), signOfInt16(s[n]), signOfInt16(s[n]), signOfInt16(sPf[n]))
}

// (6) 부호 전환 단계 식별 — 3-point chain = excitation / syn IIR / pre-post.
//     pre-post == syn (postfilter 입력 = synth IIR 출력); 둘 사이 전환 0 의무.
//     postfilter sPf 와 PST want 비교는 cross-check 보조 (chain 후단 stage).
t.Logf("──────── M5 sign-transition decision ────────")
stages := []struct {
name string
v5   int16
v6   int16
v7   int16
}{
{"excitation u", u[5], u[6], u[7]},
{"syn IIR s   ", s[5], s[6], s[7]},
{"pre-post in ", s[5], s[6], s[7]},
{"postfilter  ", sPf[5], sPf[6], sPf[7]},
{"PST want    ", wantFrames[0][5], wantFrames[0][6], wantFrames[0][7]},
}
for i, st := range stages {
t.Logf("  stage %d %s : [%+6d %+6d %+6d]  signs=[%s %s %s]",
i+1, st.name, st.v5, st.v6, st.v7,
signOfInt16(st.v5), signOfInt16(st.v6), signOfInt16(st.v7))
}
for i := 1; i < len(stages); i++ {
prev := stages[i-1]
cur := stages[i]
flips := [3]bool{
signOfInt16(prev.v5) != signOfInt16(cur.v5),
signOfInt16(prev.v6) != signOfInt16(cur.v6),
signOfInt16(prev.v7) != signOfInt16(cur.v7),
}
if flips[0] || flips[1] || flips[2] {
t.Logf("  >>> sign flip between stage %d (%s) and stage %d (%s) at sample(s): %v",
i, prev.name, i+1, cur.name, flipPositions(flips))
}
}

// (7) M5 가설 평가 (plan §Step 4 표 기준).
t.Logf("──────── M5 hypothesis evaluation ────────")
uSigns := [3]string{signOfInt16(u[5]), signOfInt16(u[6]), signOfInt16(u[7])}
wantSigns := [3]string{signOfInt16(wantFrames[0][5]), signOfInt16(wantFrames[0][6]), signOfInt16(wantFrames[0][7])}
t.Logf("excitation u[5..7] signs = [%s %s %s]  ;  PST want signs = [%s %s %s]",
uSigns[0], uSigns[1], uSigns[2], wantSigns[0], wantSigns[1], wantSigns[2])
switch {
case uSigns[0] == "0" && uSigns[1] == "0" && uSigns[2] == "0":
t.Logf("verdict: M5 REFUTED — excitation u[5..7] = 0 (부호 무 ; sign 발생은 chain 후단 합성 IIR 단계). M5 가설 (excitation 자체 부호 결함) 미해당.")
case uSigns == wantSigns:
t.Logf("verdict: M5 PARTIAL/REFUTED — excitation 부호가 PST want 와 정합 (chain 후단 결함 가능)")
case uSigns[0] != wantSigns[0] && uSigns[1] != wantSigns[1] && uSigns[2] != wantSigns[2] &&
uSigns[0] == uSigns[1] && uSigns[1] == uSigns[2]:
t.Logf("verdict: M5 STRONG — excitation 부호가 PST want 와 sample-uniform 반전 (excitation 자체 결함 유력)")
default:
t.Logf("verdict: M5 PARTIAL — sample 별 부호 분포 mixed (Task 5 종합으 인계)")
}
}

// flipPositions returns the sample offsets (5..7) at which a sign flip
// was observed; helper local to this measurement test.
func flipPositions(flips [3]bool) []int {
var out []int
for i, f := range flips {
if f {
out = append(out, 5+i)
}
}
return out
}
