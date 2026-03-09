package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAGENTSLoadsCWDFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, AGENTSFilename)
	if err := os.WriteFile(path, []byte("cwd rules"), 0o644); err != nil {
		t.Fatalf("write AGENTS: %v", err)
	}

	files, err := LoadAGENTS(dir)
	if err != nil {
		t.Fatalf("load AGENTS: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 context file, got %d", len(files))
	}
	if files[0].Path != path {
		t.Fatalf("expected path %q, got %q", path, files[0].Path)
	}
	if files[0].Content != "cwd rules" {
		t.Fatalf("expected content to match, got %q", files[0].Content)
	}
}

func TestLoadAGENTSLoadsGitRootAncestorsInOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	subdir := filepath.Join(root, "subdir")
	leaf := filepath.Join(subdir, "leaf")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatalf("mkdir leaf: %v", err)
	}
	rootPath := filepath.Join(root, AGENTSFilename)
	subdirPath := filepath.Join(subdir, AGENTSFilename)
	if err := os.WriteFile(rootPath, []byte("root rules"), 0o644); err != nil {
		t.Fatalf("write root AGENTS: %v", err)
	}
	if err := os.WriteFile(subdirPath, []byte("subdir rules"), 0o644); err != nil {
		t.Fatalf("write subdir AGENTS: %v", err)
	}

	files, err := LoadAGENTS(leaf)
	if err != nil {
		t.Fatalf("load AGENTS: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 context files, got %d", len(files))
	}
	if files[0].Path != rootPath || files[1].Path != subdirPath {
		t.Fatalf("expected outer-to-inner ordering, got %+v", files)
	}
}

func TestLoadAGENTSWithoutGitRootLoadsOnlyCWD(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "subdir")
	leaf := filepath.Join(subdir, "leaf")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatalf("mkdir leaf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, AGENTSFilename), []byte("root rules"), 0o644); err != nil {
		t.Fatalf("write root AGENTS: %v", err)
	}
	leafPath := filepath.Join(leaf, AGENTSFilename)
	if err := os.WriteFile(leafPath, []byte("leaf rules"), 0o644); err != nil {
		t.Fatalf("write leaf AGENTS: %v", err)
	}

	files, err := LoadAGENTS(leaf)
	if err != nil {
		t.Fatalf("load AGENTS: %v", err)
	}
	if len(files) != 1 || files[0].Path != leafPath {
		t.Fatalf("expected only cwd AGENTS, got %+v", files)
	}
}

func TestLoadAGENTSMissingFileReturnsEmpty(t *testing.T) {
	files, err := LoadAGENTS(t.TempDir())
	if err != nil {
		t.Fatalf("load AGENTS: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no context files, got %d", len(files))
	}
}

func TestLoadAGENTSSkipsUnreadablePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, AGENTSFilename)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir AGENTS dir: %v", err)
	}

	files, err := LoadAGENTS(dir)
	if err == nil {
		t.Fatal("expected non-fatal load error")
	}
	if len(files) != 0 {
		t.Fatalf("expected unreadable path to be skipped, got %+v", files)
	}
}

func TestBuildRunSystemPromptIncludesProjectContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, AGENTSFilename)
	if err := os.WriteFile(path, []byte("be careful"), 0o644); err != nil {
		t.Fatalf("write AGENTS: %v", err)
	}

	got, err := BuildRunSystemPrompt("base prompt", dir)
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	for _, want := range []string{"base prompt", "# Project Context", path, "be careful"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected prompt to contain %q, got %q", want, got)
		}
	}
}

func TestBuildRunSystemPromptOmitsProjectContextWhenNoFiles(t *testing.T) {
	got, err := BuildRunSystemPrompt("base prompt", t.TempDir())
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}
	if got != "base prompt" {
		t.Fatalf("expected base prompt only, got %q", got)
	}
}
