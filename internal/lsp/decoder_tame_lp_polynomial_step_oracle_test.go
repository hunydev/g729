package lsp

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"testing"
)

func TestOracleHandoff_CompareTAMELPPolynomialStep(t *testing.T) {
	if os.Getenv("G729_COMPARE_TAME_LP_POLYNOMIAL_STEP") != "1" {
		t.Skip("set G729_COMPARE_TAME_LP_POLYNOMIAL_STEP=1 to compare the TAME LP polynomial-step oracle")
	}

	path := os.Getenv("G729_TAME_LP_POLYNOMIAL_STEP_EXPECTED")
	if path == "" {
		path = "/home/exedev/g729_untracked/verifier-output/decoder_tame_lp_polynomial_step_expected.csv"
	}

	expected, order, inputs, err := readTAMELPPolynomialStepExpected(path)
	if err != nil {
		t.Fatalf("read TAME LP polynomial-step oracle: %v", err)
	}
	got, err := collectTAMELPPolynomialStepGot(expected, inputs)
	if err != nil {
		t.Fatalf("collect TAME LP polynomial-step got: %v", err)
	}

	type stat struct {
		exact    int
		total    int
		missing  int
		mismatch int
	}
	stats := map[string]*stat{}
	fieldOrder := make([]string, 0)
	seenField := map[string]bool{}
	firstMismatches := make([]string, 0, 12)

	for _, k := range order {
		if !seenField[k.stage] {
			seenField[k.stage] = true
			fieldOrder = append(fieldOrder, k.stage)
		}
		s := stats[k.stage]
		if s == nil {
			s = &stat{}
			stats[k.stage] = s
		}
		s.total++
		want := expected[k]
		have, ok := got[k]
		switch {
		case !ok:
			s.missing++
			if len(firstMismatches) < cap(firstMismatches) {
				firstMismatches = append(firstMismatches, fmt.Sprintf(
					"%s frame=%d sub=%d poly=%s step=%d inner=%d stage=%s index=%d missing got want=%d",
					k.source, k.frame, k.sub, k.poly, k.rootStep, k.innerJ, k.stage, k.index, want,
				))
			}
		case have == want:
			s.exact++
		default:
			s.mismatch++
			if len(firstMismatches) < cap(firstMismatches) {
				firstMismatches = append(firstMismatches, fmt.Sprintf(
					"%s frame=%d sub=%d poly=%s step=%d inner=%d stage=%s index=%d got=%d want=%d delta=%d",
					k.source, k.frame, k.sub, k.poly, k.rootStep, k.innerJ, k.stage, k.index,
					have, want, have-want,
				))
			}
		}
	}

	var exact, total, missing, mismatches int
	t.Logf("TAME LP polynomial-step oracle: %s", path)
	for _, field := range fieldOrder {
		s := stats[field]
		exact += s.exact
		total += s.total
		missing += s.missing
		mismatches += s.mismatch
		t.Logf("  %-24s exact %4d/%4d  mismatches=%4d  missing=%4d",
			field, s.exact, s.total, s.mismatch, s.missing)
	}
	t.Logf("  TOTAL exact %d/%d %.2f%%  mismatches=%d  missing=%d",
		exact, total, 100.0*float64(exact)/float64(total), mismatches, missing)
	for _, m := range firstMismatches {
		t.Logf("  first mismatch: %s", m)
	}

	if os.Getenv("G729_REQUIRE_EXACT_TAME_LP_POLYNOMIAL_STEP") == "1" && exact != total {
		t.Fatalf("TAME LP polynomial-step oracle mismatch: exact=%d total=%d mismatches=%d missing=%d",
			exact, total, mismatches, missing)
	}
}

type tameLPPolynomialStepKey struct {
	source   string
	frame    int
	sub      int
	poly     string
	rootStep int
	innerJ   int
	stage    string
	index    int
}

type tameLPPolynomialInputKey struct {
	source string
	frame  int
	sub    int
}

func readTAMELPPolynomialStepExpected(path string) (
	map[tameLPPolynomialStepKey]int64,
	[]tameLPPolynomialStepKey,
	map[tameLPPolynomialInputKey][10]int16,
	error,
) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, nil, fmt.Errorf("empty csv")
	}
	wantHeader := []string{"source", "frame", "sub", "poly", "root_step", "inner_j", "stage", "index", "expected", "note"}
	if len(rows[0]) != len(wantHeader) {
		return nil, nil, nil, fmt.Errorf("header width = %d, want %d", len(rows[0]), len(wantHeader))
	}
	for i, want := range wantHeader {
		if rows[0][i] != want {
			return nil, nil, nil, fmt.Errorf("header[%d] = %q, want %q", i, rows[0][i], want)
		}
	}

	expected := make(map[tameLPPolynomialStepKey]int64, len(rows)-1)
	order := make([]tameLPPolynomialStepKey, 0, len(rows)-1)
	inputs := map[tameLPPolynomialInputKey][10]int16{}
	for rowNum, row := range rows[1:] {
		if len(row) != len(wantHeader) {
			return nil, nil, nil, fmt.Errorf("row %d width = %d, want %d", rowNum+2, len(row), len(wantHeader))
		}
		frame, err := strconv.Atoi(row[1])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d frame: %w", rowNum+2, err)
		}
		sub, err := strconv.Atoi(row[2])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d sub: %w", rowNum+2, err)
		}
		rootStep, err := strconv.Atoi(row[4])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d root_step: %w", rowNum+2, err)
		}
		innerJ, err := strconv.Atoi(row[5])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d inner_j: %w", rowNum+2, err)
		}
		index, err := strconv.Atoi(row[7])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d index: %w", rowNum+2, err)
		}
		value, err := strconv.ParseInt(row[8], 10, 64)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("row %d expected: %w", rowNum+2, err)
		}
		k := tameLPPolynomialStepKey{
			source:   row[0],
			frame:    frame,
			sub:      sub,
			poly:     row[3],
			rootStep: rootStep,
			innerJ:   innerJ,
			stage:    row[6],
			index:    index,
		}
		if _, exists := expected[k]; exists {
			return nil, nil, nil, fmt.Errorf("duplicate key at row %d: %+v", rowNum+2, k)
		}
		expected[k] = value
		order = append(order, k)

		if k.stage == "input_lsp_q15" {
			ik := tameLPPolynomialInputKey{source: k.source, frame: k.frame, sub: k.sub}
			in := inputs[ik]
			if index < 0 || index >= len(in) {
				return nil, nil, nil, fmt.Errorf("row %d input_lsp_q15 index out of range: %d", rowNum+2, index)
			}
			in[index] = int16(value)
			inputs[ik] = in
		}
	}
	return expected, order, inputs, nil
}

func collectTAMELPPolynomialStepGot(
	expected map[tameLPPolynomialStepKey]int64,
	inputs map[tameLPPolynomialInputKey][10]int16,
) (map[tameLPPolynomialStepKey]int64, error) {
	got := make(map[tameLPPolynomialStepKey]int64)
	for ik, lsp := range inputs {
		for _, poly := range []string{"F1", "F2"} {
			offset := 0
			if poly == "F2" {
				offset = 1
			}
			add := func(rootStep, innerJ int, stage string, index int, value int64) {
				k := tameLPPolynomialStepKey{
					source:   ik.source,
					frame:    ik.frame,
					sub:      ik.sub,
					poly:     poly,
					rootStep: rootStep,
					innerJ:   innerJ,
					stage:    stage,
					index:    index,
				}
				if _, want := expected[k]; want {
					got[k] = value
				}
			}
			addVector := func(rootStep, innerJ int, stage string, values *[6]int64) {
				for i, v := range values {
					add(rootStep, innerJ, stage, i, v)
				}
			}
			for i, v := range lsp {
				add(0, -1, "input_lsp_q15", i, int64(v))
			}

			var f [6]int64
			addVector(0, -1, "poly_before_q24", &f)
			f[0] = 1 << 24
			q := int64(lsp[offset])
			product := lspPolyProductQ24(q, f[0])
			f[1] = -product
			add(0, 0, "product_q24", 0, product)
			add(0, 0, "acc_after_product_q24", 0, -product)
			add(0, 0, "acc_after_add_q24", 0, -product)
			addVector(0, -1, "poly_after_q24", &f)

			for step := 1; step < 5; step++ {
				old := f
				addVector(step, -1, "poly_before_q24", &old)
				q = int64(lsp[2*step+offset])
				var next [6]int64
				next[0] = old[0]
				product = lspPolyProductQ24(q, old[0])
				next[1] = old[1] - product
				add(step, 0, "product_q24", 0, product)
				add(step, 0, "acc_after_product_q24", 0, next[1])
				add(step, 0, "acc_after_add_q24", 0, next[1])
				for inner := 1; inner <= step; inner++ {
					j := step - inner + 1
					product = lspPolyProductQ24(q, old[j])
					addend := old[j-1]
					if j == step {
						addend <<= 1
					}
					afterAdd := old[j+1] + addend
					afterProduct := afterAdd - product
					next[j+1] = afterProduct
					add(step, inner, "product_q24", 0, product)
					add(step, inner, "acc_after_product_q24", 0, afterProduct)
					add(step, inner, "acc_after_add_q24", 0, afterAdd)
				}
				f = next
				addVector(step, -1, "poly_after_q24", &f)
			}

			post := f
			for i := 5; i >= 1; i-- {
				if poly == "F1" {
					post[i] += post[i-1]
				} else {
					post[i] -= post[i-1]
				}
			}
			for i, v := range post {
				add(4, -1, "post_poly_q28", i, v<<4)
			}
		}

		var a [11]int16
		LSPToLP(&lsp, &a)
		var f1, f2 [6]int64
		buildLSPPolyQ24(&lsp, 0, &f1)
		buildLSPPolyQ24(&lsp, 1, &f2)
		for i := 5; i >= 1; i-- {
			f1[i] += f1[i-1]
			f2[i] -= f2[i-1]
		}
		addFinal := func(stage string, index int, value int64) {
			k := tameLPPolynomialStepKey{
				source:   ik.source,
				frame:    ik.frame,
				sub:      ik.sub,
				poly:     "F1",
				rootStep: 4,
				innerJ:   -1,
				stage:    stage,
				index:    index,
			}
			if _, want := expected[k]; want {
				got[k] = value
			}
		}
		addFinal("lp_sum_q28", 0, 1<<28)
		addFinal("lp_a_q12", 0, int64(a[0]))
		for i := 1; i <= 5; i++ {
			addFinal("lp_sum_q28", i, (f1[i]+f2[i])<<3)
			addFinal("lp_sum_q28", 11-i, (f1[i]-f2[i])<<3)
			addFinal("lp_a_q12", i, int64(a[i]))
			addFinal("lp_a_q12", 11-i, int64(a[11-i]))
		}
	}
	return got, nil
}
