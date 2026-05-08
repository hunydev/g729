package fcbsearch_test

import (
	"testing"

	"github.com/hunydev/g729/internal/fcbsearch"
)

// CB-2 RED tests for §3.8.1 eq. 51 + 56 + 57 (G729E.txt lines 1252–1273).
//
// φ(i,j) = Σ_{n=j..39} h(n−i)·h(n−j)        (eq. 51, i ≤ j)
// φ′(i,j) = sign[d(i)]·sign[d(j)]·φ(i,j)     (eq. 56, off-diagonal)
// φ′(i,i) = 0.5·φ(i,i)                       (eq. 57, sign² = 1)
//
// Q-format pin (OQ-Q-FORMAT-A10 default per Phase 2d sub-plan §3 line 462):
//   - h     : Q12 int16 (Phase 2c HI-1 convention)
//   - signs : ±1 int16 (CB-3 SignsFromD output)
//   - φ′    : Q24 int32, full symmetric storage; φ′[i][j] = φ′[j][i].
//             Diagonal stores the eq. 57 value 0.5·φ(i,i) (i.e. the
//             "E/2" form used by eq. 59), so the depth-first search can
//             accumulate energy without an extra ½ factor.

func TestPhiPrime_IdentityImpulse(t *testing.T) {
	// h = δ in Q12: h[0] = 4096, rest = 0. signs all +1.
	//   φ(0,0) = h(0)² = 4096² = 2²⁴; φ′(0,0) = 0.5·2²⁴ = 2²³.
	//   φ(i,i) for i>0 = 0 (h(n−i) = 0 unless n=i and h(0)=4096; n≥i so n=i
	//                    gives h(0)·h(0), but only when i≤39 — wait, that
	//                    *is* nonzero for n=i). Recompute carefully.
	//
	// Re-derive: φ(i,i) = Σ_{n=i..39} h(n−i)². Substituting m=n−i:
	// φ(i,i) = Σ_{m=0..39−i} h(m)². With h = δ at m=0, h(0)² = 4096²,
	// so φ(i,i) = 2²⁴ for every i∈[0,39] (the i=0 term m=0 is always
	// inside the summation range). Thus φ′(i,i) = 2²³ for all i.
	//
	// Off-diagonal φ(i,j) for i<j: φ(i,j) = Σ_{n=j..39} h(n−i)·h(n−j).
	// h(n−j) is nonzero only at n=j (m=0), giving h(j−i)·h(0). For
	// j>i, j−i ≥ 1 → h(j−i) = 0. So φ(i,j) = 0 for all i≠j.
	var h [40]int16
	h[0] = 4096
	var signs [40]int16
	for n := range signs {
		signs[n] = +1
	}

	var phi [40][40]int32
	fcbsearch.PhiPrime(&h, &signs, &phi)

	const wantDiag = int32(1 << 23) // 0.5 · 2²⁴
	for i := 0; i < 40; i++ {
		if phi[i][i] != wantDiag {
			t.Fatalf("phi[%d][%d]=%d want %d (0.5·h(0)² in Q24)",
				i, i, phi[i][i], wantDiag)
		}
		for j := 0; j < 40; j++ {
			if i == j {
				continue
			}
			if phi[i][j] != 0 {
				t.Fatalf("phi[%d][%d]=%d want 0 (off-diag of δ-impulse)",
					i, j, phi[i][j])
			}
		}
	}
}

func TestPhiPrime_TwoTapGolden(t *testing.T) {
	// h = [a, b, 0, ..., 0] in Q12. signs all +1 → φ′ = φ off-diag,
	// 0.5·φ on diagonal (eq. 57).
	//
	// φ(i,i) for i ∈ [0, 38]: Σ_{m=0..39−i} h(m)² = a² + b² (m=0 and m=1).
	// φ(38,38): m∈[0,1] still in range → a² + b².
	// φ(39,39): m∈[0,0] → a² only.
	//
	// φ(i,j) for j = i+1, i ≤ 38:
	//   Σ_{n=j..39} h(n−i)·h(n−j). h(n−j) nonzero at n=j (h(0)=a) and
	//   n=j+1 (h(1)=b, only if j+1 ≤ 39). Pairs:
	//     n=j   : h(j−i)·h(0) = h(1)·a = b·a   (since j−i = 1)
	//     n=j+1 : h(j+1−i)·h(1) = h(2)·b = 0   (h(2)=0)
	//   So φ(i, i+1) = a·b for i ≤ 38.
	//
	// φ(i,j) for j ≥ i+2: h(j−i) = 0 (only taps 0,1) → φ = 0.
	var h [40]int16
	a, b := int16(3000), int16(-1500)
	h[0], h[1] = a, b
	var signs [40]int16
	for n := range signs {
		signs[n] = +1
	}

	var phi [40][40]int32
	fcbsearch.PhiPrime(&h, &signs, &phi)

	aa := int32(a) * int32(a)
	bb := int32(b) * int32(b)
	ab := int32(a) * int32(b)

	for i := 0; i < 39; i++ {
		want := (aa + bb) >> 1
		if phi[i][i] != want {
			t.Fatalf("phi[%d][%d]=%d want %d (0.5·(a²+b²))", i, i, phi[i][i], want)
		}
	}
	if phi[39][39] != aa>>1 {
		t.Fatalf("phi[39][39]=%d want %d (0.5·a²)", phi[39][39], aa>>1)
	}
	for i := 0; i < 39; i++ {
		if phi[i][i+1] != ab {
			t.Fatalf("phi[%d][%d]=%d want %d (a·b)", i, i+1, phi[i][i+1], ab)
		}
		if phi[i+1][i] != ab {
			t.Fatalf("phi[%d][%d]=%d want %d (symmetric a·b)", i+1, i, phi[i+1][i], ab)
		}
	}
	for i := 0; i < 40; i++ {
		for j := i + 2; j < 40; j++ {
			if phi[i][j] != 0 {
				t.Fatalf("phi[%d][%d]=%d want 0 (taps 0,1 only)", i, j, phi[i][j])
			}
			if phi[j][i] != 0 {
				t.Fatalf("phi[%d][%d]=%d want 0 (symmetric)", j, i, phi[j][i])
			}
		}
	}
}

func TestPhiPrime_SignsAbsorbed(t *testing.T) {
	// Same h as TwoTapGolden but flip sign on indices 1 and 2. Off-diagonal
	// terms whose i+j is odd should flip sign; diagonal must remain the
	// same (sign² = 1 per eq. 57).
	var h [40]int16
	a, b := int16(3000), int16(-1500)
	h[0], h[1] = a, b
	var signs [40]int16
	for n := range signs {
		signs[n] = +1
	}
	signs[1] = -1
	signs[2] = -1

	var phi [40][40]int32
	fcbsearch.PhiPrime(&h, &signs, &phi)

	aa := int32(a) * int32(a)
	bb := int32(b) * int32(b)
	ab := int32(a) * int32(b)

	// Diagonal: unchanged by sign flips (sign² = 1).
	for i := 0; i < 39; i++ {
		want := (aa + bb) >> 1
		if phi[i][i] != want {
			t.Fatalf("phi[%d][%d]=%d want %d (diagonal sign-invariant)",
				i, i, phi[i][i], want)
		}
	}
	if phi[39][39] != aa>>1 {
		t.Fatalf("phi[39][39]=%d want %d", phi[39][39], aa>>1)
	}

	// Off-diagonal sign rule per eq. 56:
	//   φ′(0,1) = sign[d(0)]·sign[d(1)]·a·b = (+1)·(−1)·a·b = −a·b
	//   φ′(1,2) = (−1)·(−1)·a·b = +a·b
	//   φ′(2,3) = (−1)·(+1)·a·b = −a·b
	//   φ′(3,4) = (+1)·(+1)·a·b = +a·b   (and so on)
	cases := []struct {
		i, j int
		want int32
	}{
		{0, 1, -ab},
		{1, 2, +ab},
		{2, 3, -ab},
		{3, 4, +ab},
	}
	for _, c := range cases {
		if phi[c.i][c.j] != c.want {
			t.Fatalf("phi[%d][%d]=%d want %d (signs absorbed)",
				c.i, c.j, phi[c.i][c.j], c.want)
		}
		if phi[c.j][c.i] != c.want {
			t.Fatalf("phi[%d][%d]=%d want %d (symmetric)",
				c.j, c.i, phi[c.j][c.i], c.want)
		}
	}
}

func TestPhiPrime_NoAlloc(t *testing.T) {
	var h, signs [40]int16
	for n := range h {
		h[n] = int16(2000 - 47*n)
		if n%2 == 0 {
			signs[n] = +1
		} else {
			signs[n] = -1
		}
	}
	var phi [40][40]int32
	if got := testing.AllocsPerRun(32, func() {
		fcbsearch.PhiPrime(&h, &signs, &phi)
	}); got != 0 {
		t.Fatalf("PhiPrime allocations/op = %v, want 0 (caller-owned scratch)", got)
	}
}
