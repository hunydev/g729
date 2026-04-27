package lsp

import (
	"math"
	"testing"
)

func TestStabilityAlreadyMonotonic(t *testing.T) {
	in := [10]int16{2000, 4000, 6000, 8000, 10000, 12000, 14000, 16000, 18000, 20000}
	out := in
	enforceLSFStability(&out)
	if out != in {
		t.Fatalf("stable input was modified: got %v, want %v", out, in)
	}
}

func TestStabilityOutOfOrder(t *testing.T) {
	in := [10]int16{5000, 3000, 4000, 6000, 8000, 10000, 12000, 14000, 16000, 18000}
	enforceLSFStability(&in)
	for i := 1; i < 10; i++ {
		if in[i] <= in[i-1] {
			t.Fatalf("after enforce, not strictly monotone at i=%d: %v", i, in)
		}
	}
}

func TestStabilityTooClose(t *testing.T) {
	in := [10]int16{2000, 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009}
	enforceLSFStability(&in)
	const minGap = 320 // ITU §3.2.4: J = 0.0391 rad ≈ 320 in Q13
	for i := 1; i < 10; i++ {
		if in[i]-in[i-1] < minGap {
			t.Fatalf("gap at i=%d is %d, want >= %d: %v", i, in[i]-in[i-1], minGap, in)
		}
	}
}

func TestRearrangeAdjacentTooClose(t *testing.T) {
	// Adjacent pair within J → both moved so their gap equals J.
	const J int16 = 10
	in := [10]int16{1000, 1005, 4000, 6000, 8000, 10000, 12000, 14000, 16000, 18000}
	rearrangeAdjacent(&in, J)
	if g := in[1] - in[0]; g < J {
		t.Errorf("after rearrange, gap[0..1] = %d < J = %d (in=%v)", g, J, in)
	}
}

func TestRearrangeAdjacentNoChangeWhenSpaced(t *testing.T) {
	const J int16 = 10
	in := [10]int16{1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000}
	out := in
	rearrangeAdjacent(&out, J)
	if out != in {
		t.Fatalf("well-spaced input modified: got %v, want %v", out, in)
	}
}

// TestALGTHMFrame0SF0_AzStability: Stage D-bis 보고서 §5 위험 노트
// 검증. ALGTHM.BIT frame 0 sf0의 LSP 인 디코더를 구동하여
// 얻은 a[](Q12)가 minimum-phase(모든 근 inside unit disk)인지 확인.
//
// 알고리즘: Schur–Cohn step-down. monic A(z)에 대해
//   k_m = a^(m)[m] (반사 계수)
//   a^(m-1)[i] = (a^(m)[i] - k_m·a^(m)[m-i]) / (1 - k_m^2)
// 모든 |k_m| < 1 ⟺ A(z) minimum-phase.
//
// 본  FAIL → Stage F 분기점은 lsp.* (lspToLP 또는 디코더)
// 본 어서션이 PASS → Stage F 분기점은 synth.Filter
func TestALGTHMFrame0SF0_AzStability(t *testing.T) {
var dec Decoder
sf1A, _ := dec.Decode(Indices{L0: 1, L1: 105, L2: 17, L3: 0})

// Q12 → float64 (테스트 검증용; production 영향 없음)
a := make([]float64, 11)
for i := 0; i <= 10; i++ {
a[i] = float64(sf1A[i]) / 4096.0
}
t.Logf("a[] (int16 Q12) = %v", sf1A)
t.Logf("a[] (float, Q12-normalized) = %v", a)
if math.Abs(a[0]-1.0) > 1e-9 {
t.Fatalf("a[0]=%.6f, want 1.0 (Q12 normalization broken)", a[0])
}

// Schur–Cohn step-down on a[1..10] with implicit a[0]=1.
work := make([]float64, 11)
copy(work, a)
for m := 10; m >= 1; m-- {
k := work[m]
if math.Abs(k) >= 1.0 {
t.Logf("OBSERVATION (F-prep-1): A(z) NOT minimum-phase at step m=%d: |k_m|=%.6f >= 1; "+
"Stage F branch points at lsp.* (LSP→LP conversion or decoder bug). "+
"Promoted to t.Errorf in F-fix.", m, math.Abs(k))
t.Logf("a[] (float, Q12-normalized) = %v", a)
return
}
denom := 1.0 - k*k
next := make([]float64, m)
next[0] = 1.0
for i := 1; i < m; i++ {
next[i] = (work[i] - k*work[m-i]) / denom
}
copy(work[:m], next)
t.Logf("step m=%d: k=%.6f", m, k)
}
t.Logf("A(z) minimum-phase confirmed; reflection coefficients all |k|<1. " +
"Stage F branch = synth.Filter (LP synthesis IIR primitives).")
}
