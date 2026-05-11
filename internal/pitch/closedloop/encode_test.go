package closedloop

import (
	"testing"

	"github.com/hunydev/g729/internal/pitch"
)

// ENC-1 unit tests for P1/P0/P2 packing per ITU-T G.729 §3.7.2
// (G729E.txt lines 1168–1185 / eq. 41–42) with §4.1.3 (lines
// 1505–1523) as the canonical decoder inverse used for round-trip
// validation.

// TestEncodeP1_RoundTripFractional sweeps representable (intLag, frac)
// pairs in the fractional region plus the lower edge intLag=19 with frac=1
// (P1=0, the smallest codepoint reachable by the inverse), and asserts the
// decoder pitch.DecodeDelaySubframe1 inverts EncodeP1 byte-exactly.
func TestEncodeP1_RoundTripFractional(t *testing.T) {
	cases := []struct {
		intLag int16
		frac   int8
	}{
		{19, 1},
		{20, -1}, {20, 0}, {20, 1},
		{50, -1}, {50, 0}, {50, 1},
		{84, -1}, {84, 0}, {84, 1},
		{85, -1}, {85, 0},
	}
	for _, c := range cases {
		p1 := EncodeP1(c.intLag, c.frac)
		gotInt, gotFrac := pitch.DecodeDelaySubframe1(p1)
		if int16(gotInt) != c.intLag || int8(gotFrac) != c.frac {
			t.Errorf("EncodeP1(%d,%d)=%d → decode=(%d,%d), want (%d,%d)",
				c.intLag, c.frac, p1, gotInt, gotFrac, c.intLag, c.frac)
		}
	}
}

func TestEncodeP1_UpperFractionalEdgeIsUnrepresentable(t *testing.T) {
	p1 := EncodeP1(85, +1)
	if p1 != 197 {
		t.Fatalf("EncodeP1(85,+1)=%d, want 197; P1=198 decodes as (86,0)", p1)
	}
	gotInt, gotFrac := pitch.DecodeDelaySubframe1(p1)
	if gotInt != 85 || gotFrac != 0 {
		t.Fatalf("EncodeP1(85,+1) normalized decode=(%d,%d), want (85,0)", gotInt, gotFrac)
	}
}

// TestEncodeP1_RoundTripInteger sweeps the integer-only region
// intLag ∈ [86, 143], frac = 0 (§3.7.2 eq. 41 second branch).
func TestEncodeP1_RoundTripInteger(t *testing.T) {
	for intLag := int16(86); intLag <= 143; intLag++ {
		p1 := EncodeP1(intLag, 0)
		gotInt, gotFrac := pitch.DecodeDelaySubframe1(p1)
		if int16(gotInt) != intLag || gotFrac != 0 {
			t.Errorf("EncodeP1(%d,0)=%d → decode=(%d,%d), want (%d,0)",
				intLag, p1, gotInt, gotFrac, intLag)
		}
	}
}

// TestEncodeP1_Boundaries pins the exact P1 codepoints at the
// fractional/integer transition (§3.7.2 eq. 41).
func TestEncodeP1_Boundaries(t *testing.T) {
	cases := []struct {
		intLag int16
		frac   int8
		want   uint8
	}{
		{19, 1, 0},
		{20, -1, 1},
		{20, 0, 2},
		{85, 0, 197},
		{86, 0, 198},
		{143, 0, 255},
	}
	for _, c := range cases {
		got := EncodeP1(c.intLag, c.frac)
		if got != c.want {
			t.Errorf("EncodeP1(%d,%d)=%d, want %d", c.intLag, c.frac, got, c.want)
		}
	}
}

// TestEncodeP2_RoundTrip sweeps every encodable P2 codepoint:
// (tmin-1,+1), all fractions for [tmin,tmax], and (tmax+1,-1),
// asserting decoder pitch.DecodeDelaySubframe2 inverts EncodeP2
// byte-exactly.
func TestEncodeP2_RoundTrip(t *testing.T) {
	t1Ints := []int16{20, 26, 55, 105, 143}
	for _, t1Int := range t1Ints {
		tmin, _ := Subframe2Window(t1Int)
		for d := int16(-1); d <= 10; d++ {
			intT2 := tmin + d
			for _, frac := range []int8{-1, 0, 1} {
				if intT2 == tmin-1 && frac != 1 {
					continue
				}
				if intT2 == tmin+10 && frac != -1 {
					continue
				}
				if intT2 > tmin-1 && intT2 < tmin+10 {
					// All three fractions are encodable.
				} else if intT2 != tmin-1 && intT2 != tmin+10 {
					continue
				}
				p2 := EncodeP2(intT2, frac, tmin)
				gotInt, gotFrac := pitch.DecodeDelaySubframe2(p2, int(t1Int))
				if int16(gotInt) != intT2 || int8(gotFrac) != frac {
					t.Errorf("EncodeP2(intT2=%d,frac=%d,tmin=%d t1=%d)=%d → decode=(%d,%d), want (%d,%d)",
						intT2, frac, tmin, t1Int, p2, gotInt, gotFrac, intT2, frac)
				}
				if p2 > 31 {
					t.Errorf("EncodeP2(intT2=%d,frac=%d,tmin=%d)=%d exceeds 5-bit range",
						intT2, frac, tmin, p2)
				}
			}
		}
	}
}

// TestEncodeP2_BoundaryFormula pins eq. 42:
// P2 = 3·(intT2 − tmin) + frac + 2, frac ∈ {-1, 0, 1}.
func TestEncodeP2_BoundaryFormula(t *testing.T) {
	cases := []struct {
		intT2 int16
		frac  int8
		tmin  int16
		want  uint8
	}{
		{20, -1, 20, 1},
		{20, 0, 20, 2},
		{20, 1, 20, 3},
		{29, 0, 20, 29},
		{30, -1, 20, 31},
		{54, 1, 55, 0},
		{50, 0, 50, 2},
		{55, 1, 50, 18},
	}
	for _, c := range cases {
		got := EncodeP2(c.intT2, c.frac, c.tmin)
		if got != c.want {
			t.Errorf("EncodeP2(%d,%d,%d)=%d, want %d",
				c.intT2, c.frac, c.tmin, got, c.want)
		}
	}
}

// TestEncodeP0_MatchesDecoderParity asserts the encoder-side P0
// bit equals the decoder's expected parity for every reachable
// fractional-region P1, and that pitch.CheckParity passes when
// the encoder emits (P1, P0).
func TestEncodeP0_MatchesDecoderParity(t *testing.T) {
	for p1 := 0; p1 < 256; p1++ {
		p0 := EncodeP0(uint8(p1))
		if !pitch.CheckParity(uint8(p1), p0) {
			t.Errorf("EncodeP0(%d)=%d failed pitch.CheckParity", p1, p0)
		}
		if p0 > 1 {
			t.Errorf("EncodeP0(%d)=%d not a single bit", p1, p0)
		}
	}
}

// TestEncode_ZeroAlloc enforces I4 for all three encoder primitives.
func TestEncode_ZeroAlloc(t *testing.T) {
	if got := testing.AllocsPerRun(128, func() {
		_ = EncodeP1(50, 0)
	}); got != 0 {
		t.Fatalf("EncodeP1 allocs/op = %v, want 0", got)
	}
	if got := testing.AllocsPerRun(128, func() {
		_ = EncodeP2(55, 1, 50)
	}); got != 0 {
		t.Fatalf("EncodeP2 allocs/op = %v, want 0", got)
	}
	if got := testing.AllocsPerRun(128, func() {
		_ = EncodeP0(123)
	}); got != 0 {
		t.Fatalf("EncodeP0 allocs/op = %v, want 0", got)
	}
}
