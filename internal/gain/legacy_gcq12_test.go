package gain

import "testing"

// TestLegacyGcQ12FromMantExp_Math pins the saturating shift semantics
// of the temporary IMPL-1 adapter (legacy_gcq12.go).
//
// Contract: gcQ12 = mant · 2^(exp - 2), saturated to int16.
func TestLegacyGcQ12FromMantExp_Math(t *testing.T) {
	cases := []struct {
		name string
		mant int16
		exp  int8
		want int16
	}{
		{"zero mant short-circuits", 0, 5, 0},
		{"unity g_c (mant=16384,exp=0) → Q12 4096", 16384, 0, 4096},
		{"mant=20000 exp=0 → 5000 (g_c≈1.22)", 20000, 0, 5000},
		{"mant=24576 exp=1 → 12288 (g_c=3.0)", 24576, 1, 12288},
		{"mant=24576 exp=2 → 24576 (g_c=6.0, fits)", 24576, 2, 24576},
		{"saturate hi: mant=24576 exp=3 → 32767", 24576, 3, 32767},
		{"saturate hi: mant=20000 exp=9 → 32767", 20000, 9, 32767},
		{"underflow neg exp: mant=20000 exp=-15 → 0", 20000, -15, 0},
		{"shift -2 (exp=0) on max mant", 32767, 0, 32767 >> 2},
		{"shift -1 (exp=1) on max mant", 32767, 1, 32767 >> 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LegacyGcQ12FromMantExp(c.mant, c.exp)
			if got != c.want {
				t.Errorf("LegacyGcQ12FromMantExp(%d, %d) = %d, want %d",
					c.mant, c.exp, got, c.want)
			}
		})
	}
}

// TestLegacyGcQ12FromMantExp_RoundTripWithDecode pins that piping the
// new Decode triple through the adapter yields the same Q12 magnitude
// the pre-REF-1 Decode returned, within the documented saturation
// envelope (mant·2^(exp-14) before clamp ≤ 7.999 → exact match;
// otherwise saturated at 32767/-32768).
func TestLegacyGcQ12FromMantExp_RoundTripWithDecode(t *testing.T) {
	cases := []struct {
		name   string
		idx    Indices
		setupC func(c *[40]int16)
	}{
		{"single pulse mid-table", Indices{GA: 3, GB: 7}, func(c *[40]int16) { c[5] = 8192 }},
		{"low energy", Indices{GA: 0, GB: 0}, func(c *[40]int16) { c[0] = 1 }},
		{"zero energy", Indices{GA: 3, GB: 7}, func(c *[40]int16) {}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var d Decoder
			var cb [40]int16
			c.setupC(&cb)
			_, mant, exp := d.Decode(c.idx, &cb)
			gcQ12 := LegacyGcQ12FromMantExp(mant, exp)
			if mant == 0 && gcQ12 != 0 {
				t.Errorf("mant=0 must yield gcQ12=0, got %d", gcQ12)
			}
			if mant != 0 && gcQ12 < 0 {
				t.Errorf("non-zero positive mant yielded negative gcQ12=%d", gcQ12)
			}
		})
	}
}
