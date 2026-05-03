package lsp

import "testing"

// TestComputeF1F2 hand-traces the §3.2.3 eq. 15 recursion for three
// synthetic Q12 LP inputs. Expected values are derived strictly from
// f1(0)=f2(0)=1.0, f1(i+1)=a[i+1]+a[10-i]−f1(i),
// f2(i+1)=a[i+1]−a[10-i]+f2(i), with all coefficients promoted Q12→Q24.
func TestComputeF1F2(t *testing.T) {
	const oneQ24 int32 = 1 << 24

	cases := []struct {
		name       string
		a          [11]int16
		wantF1     [6]int32
		wantF2     [6]int32
	}{
		{
			// All-zero a[1..10]: every (a[i+1]±a[10-i]) term is 0,
			// so f1 alternates ±1 and f2 stays at +1.
			name: "zero_a",
			a:    [11]int16{4096, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantF1: [6]int32{
				oneQ24, -oneQ24, oneQ24, -oneQ24, oneQ24, -oneQ24,
			},
			wantF2: [6]int32{
				oneQ24, oneQ24, oneQ24, oneQ24, oneQ24, oneQ24,
			},
		},
		{
			// a[1]=−1.0 (Q12=−4096), all other a[k]=0.
			// i=0: a[1]+a[10]=−1, a[1]−a[10]=−1.
			//   f1(1)=−1−1=−2; f2(1)=−1+1=0.
			// i≥1: sums all zero → f1 alternates ±2, f2 stays 0.
			name: "negative_a1",
			a:    [11]int16{4096, -4096, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantF1: [6]int32{
				oneQ24, -2 * oneQ24, 2 * oneQ24, -2 * oneQ24, 2 * oneQ24, -2 * oneQ24,
			},
			wantF2: [6]int32{
				oneQ24, 0, 0, 0, 0, 0,
			},
		},
		{
			// Geometric a[k] = 2^(11-k) for k=1..10 in Q12. Hand-traced.
			name: "geometric_a",
			a:    [11]int16{4096, 1024, 512, 256, 128, 64, 32, 16, 8, 4, 2},
			wantF1: [6]int32{
				oneQ24,
				-12574720,
				14688256,
				-13606912,
				14196736,
				-13803520,
			},
			wantF2: [6]int32{
				oneQ24,
				20963328,
				23044096,
				24059904,
				24518656,
				24649728,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f1, f2 [6]int32
			computeF1F2(&tc.a, &f1, &f2)
			for k := 0; k < 6; k++ {
				if f1[k] != tc.wantF1[k] {
					t.Errorf("f1[%d] = %d, want %d", k, f1[k], tc.wantF1[k])
				}
				if f2[k] != tc.wantF2[k] {
					t.Errorf("f2[%d] = %d, want %d", k, f2[k], tc.wantF2[k])
				}
			}
		})
	}
}
