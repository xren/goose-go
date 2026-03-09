package prompt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	AGENTSFilename         = "AGENTS.md"
	DefaultRunSystemPrompt = "You are a concise terminal coding assistant. Use the shell tool when it is the clearest way to inspect or modify the local environment."
)

type ContextFile struct {
	Path    string
	Content string
}

func BuildRunSystemPrompt(base string, workingDir string) (string, error) {
	contextFiles, err := LoadAGENTS(workingDir)
	if len(contextFiles) == 0 {
		return base, err
	}

	var builder strings.Builder
	builder.WriteString(base)
	builder.WriteString("\n\n# Project Context\n\n")
	builder.WriteString("Project-specific instructions and guidelines:\n\n")
	for _, file := range contextFiles {
		builder.WriteString("## ")
		builder.WriteString(file.Path)
		builder.WriteString("\n\n")
		builder.WriteString(file.Content)
		builder.WriteString("\n\n")
	}
	return builder.String(), err
}

func LoadAGENTS(workingDir string) ([]ContextFile, error) {
	localDirs, err := localDirectories(workingDir)
	if err != nil {
		return nil, err
	}

	files := make([]ContextFile, 0, len(localDirs))
	var loadErrs []error
	for _, dir := range localDirs {
		path := filepath.Join(dir, AGENTSFilename)
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			loadErrs = append(loadErrs, fmt.Errorf("stat %s: %w", path, err))
			continue
		}
		if info.IsDir() {
			loadErrs = append(loadErrs, fmt.Errorf("read %s: path is a directory", path))
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("read %s: %w", path, err))
			continue
		}
		files = append(files, ContextFile{
			Path:    path,
			Content: string(content),
		})
	}

	return files, errors.Join(loadErrs...)
}

func localDirectories(workingDir string) ([]string, error) {
	resolved, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}

	gitRoot := findGitRoot(resolved)
	if gitRoot == "" {
		return []string{resolved}, nil
	}

	var dirs []string
	for current := resolved; ; current = filepath.Dir(current) {
		dirs = append(dirs, current)
		if current == gitRoot {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	for left, right := 0, len(dirs)-1; left < right; left, right = left+1, right-1 {
		dirs[left], dirs[right] = dirs[right], dirs[left]
	}
	return dirs, nil
}

func findGitRoot(start string) string {
	for current := start; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
	}
}
