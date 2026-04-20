package fixed

import "testing"

// Confirms that the arithmetic primitives do not allocate. If a future
// change introduces allocation in a hot primitive, this test fails.
func TestNoAllocationInPrimitives(t *testing.T) {
	var s16 Word16 = 12345
	var s32 Word32 = 1234567890

	cases := []struct {
		name string
		fn   func()
	}{
		{"Add", func() { _ = Add(s16, s16) }},
		{"Sub", func() { _ = Sub(s16, s16) }},
		{"Negate", func() { _ = Negate(s16) }},
		{"AbsS", func() { _ = AbsS(s16) }},
		{"LAdd", func() { _ = LAdd(s32, s32) }},
		{"LSub", func() { _ = LSub(s32, s32) }},
		{"LNegate", func() { _ = LNegate(s32) }},
		{"LAbs", func() { _ = LAbs(s32) }},
		{"LMult", func() { _ = LMult(s16, s16) }},
		{"LMac", func() { _ = LMac(s32, s16, s16) }},
		{"LMsu", func() { _ = LMsu(s32, s16, s16) }},
		{"Mult", func() { _ = Mult(s16, s16) }},
		{"MultR", func() { _ = MultR(s16, s16) }},
		{"Round", func() { _ = Round(s32) }},
		{"Shl", func() { _ = Shl(s16, 3) }},
		{"Shr", func() { _ = Shr(s16, 3) }},
		{"ShrR", func() { _ = ShrR(s16, 3) }},
		{"LShl", func() { _ = LShl(s32, 3) }},
		{"LShr", func() { _ = LShr(s32, 3) }},
		{"LShrR", func() { _ = LShrR(s32, 3) }},
		{"NormS", func() { _ = NormS(s16) }},
		{"NormL", func() { _ = NormL(s32) }},
		{"DivS", func() { _ = DivS(100, 200) }},
		{"Saturate", func() { _ = Saturate(s32) }},
		{"ExtractH", func() { _ = ExtractH(s32) }},
		{"ExtractL", func() { _ = ExtractL(s32) }},
		{"LDepositH", func() { _ = LDepositH(s16) }},
		{"LDepositL", func() { _ = LDepositL(s16) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(1000, tc.fn)
			if allocs != 0 {
				t.Errorf("%s allocated %.2f times per call, want 0", tc.name, allocs)
			}
		})
	}
}
