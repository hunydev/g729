package lsp

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"
)

const overwriteVerifierExpectedEnv = "G729_OVERWRITE_VERIFIER_EXPECTED"

func guardVerifierExpectedTemplate(path, valueColumn string) error {
	if os.Getenv(overwriteVerifierExpectedEnv) == "1" {
		return nil
	}

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	valueIdx := -1
	for i, h := range rows[0] {
		if h == valueColumn {
			valueIdx = i
			break
		}
	}
	if valueIdx < 0 {
		return fmt.Errorf("%s is missing %q column", path, valueColumn)
	}

	var filled int
	for _, row := range rows[1:] {
		if len(row) <= valueIdx {
			continue
		}
		if strings.TrimSpace(row[valueIdx]) != "" {
			filled++
		}
	}
	if filled > 0 {
		return fmt.Errorf("refusing to overwrite verifier-filled expected template %s (%d filled cells); set %s=1 to regenerate from scratch", path, filled, overwriteVerifierExpectedEnv)
	}
	return nil
}
