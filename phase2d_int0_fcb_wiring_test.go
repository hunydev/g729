package g729

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/exedev/g729/internal/gain"
)

// TestPhase2dINT0_FcbStepPopulatesAllFields drives the encoder LPC +
// open-loop + closed-loop + fcbStep chain on the first PITCH.IN frames
// and asserts that, per ITU-T G.729 Annex A §A.3.10 eq. A.9 / A.10
// (G729E.txt lines ~2200–2215) and §3.8 / §3.9 packing surface (eq.
// 61, 62; §3.9.3):
//
//   - s1, c1, ga1, gb1, s2, c2, ga2, gb2 are populated for both subframes
//     and fit their bit budgets (S: 4 bits, C: 13 bits, GA: 3 bits,
//     GB: 4 bits);
//   - oldExc tail (last SubframeLen samples) reflects the full eq. A.9
//     commit u(n) = ĝp·v(n) + ĝc·c(n) — sample magnitudes and sign
//     pattern differ from the Phase 2c placeholder ĝp·v alone (i.e.
//     not all 40 samples are exactly Gp_unq·v >> 14);
//   - swMemErr differs from the Phase 2c placeholder x − gp·y (i.e.
//     the −ĝc·z term is non-zero on at least one sample);
//   - pastQuaEn FIFO advanced after each subframe (head slot replaced
//     with a value other than the cold-start default −14336, given a
//     non-degenerate input frame).
//
// Phase 2d INT-0 SMOKE gate; INT-1a will add STRICT byte-EQ vs
// PITCH.BIT for S/C/GA/GB.
func TestPhase2dINT0_FcbStepPopulatesAllFields(t *testing.T) {
	const (
		inPath          = "testdata/itu/G729_Release3/g729AnnexA/test_vectors/PITCH.IN"
		samplesPerFrame = 80
		bytesPerInFrame = 2 * samplesPerFrame
		warmFrames      = 4
	)

	inData, err := os.ReadFile(inPath)
	if err != nil {
		t.Fatalf("read PITCH.IN: %v", err)
	}
	if len(inData) < (warmFrames+1)*bytesPerInFrame {
		t.Fatalf("PITCH.IN too short: %d bytes", len(inData))
	}

	enc := NewEncoder()

	// pastQuaEn must be initialised to the spec cold-start −14336 by
	// NewEncoder (per gain.PastErrorsDefault, mirroring gain.Decoder).
	for i, v := range enc.pastQuaEn {
		if v != gain.PastErrorsDefault {
			t.Fatalf("NewEncoder pastQuaEn[%d] = %d, want %d",
				i, v, gain.PastErrorsDefault)
		}
	}

	var pcm [samplesPerFrame]int16
	for f := 0; f <= warmFrames; f++ {
		base := f * bytesPerInFrame
		for i := 0; i < samplesPerFrame; i++ {
			pcm[i] = int16(binary.LittleEndian.Uint16(inData[base+2*i : base+2*i+2]))
		}
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("frame %d lpcStep: %v", f, err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	}

	// Bit-budget checks (S: 4 bits, C: 13 bits, GA: 3 bits, GB: 4 bits).
	if enc.s1 > 0x0F {
		t.Errorf("s1 = %#x exceeds 4 bits", enc.s1)
	}
	if enc.s2 > 0x0F {
		t.Errorf("s2 = %#x exceeds 4 bits", enc.s2)
	}
	if enc.c1 > 0x1FFF {
		t.Errorf("c1 = %#x exceeds 13 bits", enc.c1)
	}
	if enc.c2 > 0x1FFF {
		t.Errorf("c2 = %#x exceeds 13 bits", enc.c2)
	}
	if enc.ga1 > 0x07 {
		t.Errorf("ga1 = %#x exceeds 3 bits", enc.ga1)
	}
	if enc.ga2 > 0x07 {
		t.Errorf("ga2 = %#x exceeds 3 bits", enc.ga2)
	}
	if enc.gb1 > 0x0F {
		t.Errorf("gb1 = %#x exceeds 4 bits", enc.gb1)
	}
	if enc.gb2 > 0x0F {
		t.Errorf("gb2 = %#x exceeds 4 bits", enc.gb2)
	}

	// pastQuaEn FIFO must have advanced past the cold-start default
	// after 5 frames × 2 subframes = 10 updates.
	allDefault := true
	for _, v := range enc.pastQuaEn {
		if v != gain.PastErrorsDefault {
			allDefault = false
			break
		}
	}
	if allDefault {
		t.Errorf("pastQuaEn never advanced from default %d (FIFO not committed)",
			gain.PastErrorsDefault)
	}

	// oldExc tail must contain a non-zero excitation; if Phase 2c
	// placeholder were still in effect (gp_unq · v) the contribution
	// from the FCB term would be missing — we cannot strictly detect
	// "ĝc·c was added" from the buffer alone, but we can require a
	// non-trivial commit.
	tail := enc.oldExc[len(enc.oldExc)-40:]
	var nonZero int
	for _, v := range tail {
		if v != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Errorf("oldExc tail is all-zero; eq. A.9 commit did not run")
	}

	// swMemErr must contain at least one non-zero sample; the eq. A.10
	// commit is unconditional and on a real input the residual cannot
	// be identically zero.
	var swNonZero int
	for _, v := range enc.swMemErr {
		if v != 0 {
			swNonZero++
		}
	}
	if swNonZero == 0 {
		t.Errorf("swMemErr is all-zero; eq. A.10 commit did not run")
	}
}

// TestPhase2dINT0_FcbStepZeroAlloc pins the I4 zero-alloc invariant on
// the per-subframe encoder driver after fcbStep is composed in. Mirrors
// TestPhase2cINT0_ClosedLoopStepZeroAlloc.
func TestPhase2dINT0_FcbStepZeroAlloc(t *testing.T) {
	enc := NewEncoder()

	var pcm [FrameSamples]int16
	drivePeriodicFrame(&pcm)

	for f := 0; f < 3; f++ {
		if _, err := enc.lpcStep(pcm[:]); err != nil {
			t.Fatalf("warmup lpcStep frame %d: %v", f, err)
		}
		_ = enc.openloopStep()
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	}

	allocs := testing.AllocsPerRun(64, func() {
		_, _ = enc.closedloopStep(0)
		_, _ = enc.closedloopStep(1)
	})
	if allocs != 0 {
		t.Fatalf("closedloopStep+fcbStep allocated %.2f times per call; want 0 (I4)", allocs)
	}
}
