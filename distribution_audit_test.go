package g729

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMITDistributionAudit(t *testing.T) {
	requireFileContains(t, "LICENSE",
		"MIT License",
		"Copyright (c) 2026 g729 authors",
		"THE SOFTWARE IS PROVIDED",
	)
	requireFileContains(t, "go.mod",
		"module github.com/hunydev/g729",
	)
	requireFileContains(t, "IP_PROVENANCE.md",
		"MIT License",
		"clean-room",
		"Do not inspect ITU reference C",
		"Do not inspect bcg729",
		"Do not inspect FFmpeg G.729 implementation source",
		"Numeric Oracle Policy",
		"Black-Box Verification",
		"not legal advice",
	)
	requireFileContains(t, "THIRD_PARTY_NOTICES.md",
		"Go standard library only",
		"No vendored third-party source code",
		"No third-party G.729 implementation source code",
		"testdata/itu/",
		"docs/superpowers/specs/itu/",
		"testdata/external/*.g729",
	)

	tracked, ok := gitTrackedFiles(t)
	if !ok {
		return
	}
	for _, path := range tracked {
		clean := filepath.ToSlash(path)
		switch {
		case strings.HasPrefix(clean, "testdata/itu/"):
			t.Fatalf("ITU test vector material must not be tracked: %s", clean)
		case strings.HasPrefix(clean, "docs/superpowers/specs/itu/"):
			t.Fatalf("ITU spec PDF/text material must not be tracked: %s", clean)
		case strings.HasPrefix(clean, "testdata/external/") && clean != "testdata/external/README.md":
			t.Fatalf("external audio/payload samples must not be tracked: %s", clean)
		case isForbiddenImplementationFilename(clean):
			t.Fatalf("forbidden external implementation source-like filename is tracked: %s", clean)
		}
	}
}

func requireFileContains(t *testing.T, path string, needles ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Fatalf("%s does not contain required text %q", path, needle)
		}
	}
}

func gitTrackedFiles(t *testing.T) ([]string, bool) {
	t.Helper()
	cmd := exec.Command("git", "ls-files")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git ls-files unavailable outside a git checkout: %v", err)
		return nil, false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, true
	}
	return lines, true
}

func isForbiddenImplementationFilename(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "g729a.c", "g729.c", "dec_gain.c", "cod_ld8a.c", "dec_ld8a.c", "cb_search.c":
		return true
	default:
		return false
	}
}
