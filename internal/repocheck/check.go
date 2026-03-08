package repocheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Limits struct {
	Go       int
	GoTest   int
	Markdown int
	Makefile int
}

var DefaultLimits = Limits{
	Go:       600,
	GoTest:   650,
	Markdown: 300,
	Makefile: 150,
}

var markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

func Check() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return CheckRoot(root, DefaultLimits)
}

func CheckRoot(root string, limits Limits) ([]string, error) {
	var issues []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if shouldSkipDir(root, path) {
				return filepath.SkipDir
			}
			return nil
		}

		if !shouldCheckFile(path) {
			return nil
		}

		lineCount, err := countLines(path)
		if err != nil {
			return err
		}
		if issue := checkFileSize(root, path, lineCount, limits); issue != "" {
			issues = append(issues, issue)
		}

		if filepath.Ext(path) == ".md" {
			fileIssues, err := checkMarkdownLinks(root, path)
			if err != nil {
				return err
			}
			issues = append(issues, fileIssues...)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk repo: %w", err)
	}

	return issues, nil
}

func shouldSkipDir(root string, path string) bool {
	if path == root {
		return false
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	top := strings.Split(rel, string(filepath.Separator))[0]
	switch top {
	case ".git", ".goose-go", "bin", "goose":
		return true
	default:
		return false
	}
}

func shouldCheckFile(path string) bool {
	switch {
	case strings.HasSuffix(path, ".go"):
		return true
	case strings.HasSuffix(path, ".md"):
		return true
	case filepath.Base(path) == "Makefile":
		return true
	default:
		return false
	}
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return 0, nil
	}
	return strings.Count(string(data), "\n") + 1, nil
}

func checkFileSize(root string, path string, lineCount int, limits Limits) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}

	switch {
	case filepath.Base(path) == "Makefile":
		if lineCount > limits.Makefile {
			return fmt.Sprintf("%s has %d lines (limit %d for Makefile)", rel, lineCount, limits.Makefile)
		}
		return ""
	case strings.HasSuffix(path, "_test.go"):
		if lineCount > limits.GoTest {
			return fmt.Sprintf("%s has %d lines (limit %d for test Go files)", rel, lineCount, limits.GoTest)
		}
		return ""
	case strings.HasSuffix(path, ".go"):
		if lineCount > limits.Go {
			return fmt.Sprintf("%s has %d lines (limit %d for Go files)", rel, lineCount, limits.Go)
		}
		return ""
	case strings.HasSuffix(path, ".md"):
		if lineCount > limits.Markdown {
			return fmt.Sprintf("%s has %d lines (limit %d for Markdown files)", rel, lineCount, limits.Markdown)
		}
		return ""
	}
	return ""
}

func checkMarkdownLinks(root string, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read markdown %s: %w", path, err)
	}

	var issues []string
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(string(data), -1) {
		target := strings.TrimSpace(match[1])
		if target == "" || isExternalLink(target) {
			continue
		}

		resolved := resolveMarkdownTarget(root, filepath.Dir(path), target)
		if resolved == "" {
			continue
		}
		if _, err := os.Stat(resolved); err != nil {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			issues = append(issues, fmt.Sprintf("%s references missing path %s", rel, target))
		}
	}

	return issues, nil
}

func isExternalLink(target string) bool {
	switch {
	case strings.HasPrefix(target, "#"):
		return true
	case strings.Contains(target, "://"):
		return true
	case strings.HasPrefix(target, "mailto:"):
		return true
	default:
		return false
	}
}

func resolveMarkdownTarget(root string, baseDir string, target string) string {
	target = strings.SplitN(target, "#", 2)[0]
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "file://")
	if target == "" {
		return ""
	}

	if filepath.IsAbs(target) {
		return target
	}

	return filepath.Clean(filepath.Join(baseDir, target))
}
