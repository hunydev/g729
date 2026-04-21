package gain

import (
	"testing"

	"github.com/exedev/g729/internal/fixed"
)

func TestFixedCodebookEnergy_Zero(t *testing.T) {
	var c [40]int16
	if got := fixedCodebookEnergy(&c); got != 0 {
		t.Fatalf("energy(zero) = %d, want 0", got)
	}
}

func TestFixedCodebookEnergy_SinglePulse(t *testing.T) {
	var c [40]int16
	c[5] = 8192
	const want fixed.Word32 = 8192 * 8192
	if got := fixedCodebookEnergy(&c); got != want {
		t.Fatalf("energy(single pulse) = %d, want %d", got, want)
	}
}

func TestFixedCodebookEnergy_FourPulses(t *testing.T) {
	var c [40]int16
	c[0], c[11], c[17], c[24] = 8192, -8192, 8192, -8192
	const want fixed.Word32 = 4 * 8192 * 8192
	if got := fixedCodebookEnergy(&c); got != want {
		t.Fatalf("energy(4 pulses) = %d, want %d", got, want)
	}
}

func TestFixedCodebookEnergy_SquaringIsUnsigned(t *testing.T) {
	var cp, cn [40]int16
	cp[3] = 8192
	cn[3] = -8192
	if ep, en := fixedCodebookEnergy(&cp), fixedCodebookEnergy(&cn); ep != en {
		t.Errorf("energy(+pulse)=%d, energy(-pulse)=%d", ep, en)
	}
}
