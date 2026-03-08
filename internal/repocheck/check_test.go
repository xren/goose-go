package repocheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRootFlagsOversizedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal", "big.go"), strings.Repeat("package test\n", 605))

	issues, err := CheckRoot(root, DefaultLimits)
	if err != nil {
		t.Fatalf("check root: %v", err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0], "internal/big.go has 606 lines") {
		t.Fatalf("expected oversized Go file issue, got %v", issues)
	}
}

func TestCheckRootFlagsMissingMarkdownLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "[missing](docs/missing.md)\n")

	issues, err := CheckRoot(root, DefaultLimits)
	if err != nil {
		t.Fatalf("check root: %v", err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0], "README.md references missing path docs/missing.md") {
		t.Fatalf("expected missing markdown link issue, got %v", issues)
	}
}

func TestCheckRootIgnoresGooseReferenceSubmodule(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "goose", "docs", "huge.md"), strings.Repeat("line\n", 1000))

	issues, err := CheckRoot(root, DefaultLimits)
	if err != nil {
		t.Fatalf("check root: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected goose submodule paths to be ignored, got %v", issues)
	}
}

func TestCheckRootAcceptsExistingLocalMarkdownLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "target.md"), "ok\n")
	writeFile(t, filepath.Join(root, "README.md"), "[target](docs/target.md)\n")

	issues, err := CheckRoot(root, DefaultLimits)
	if err != nil {
		t.Fatalf("check root: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
