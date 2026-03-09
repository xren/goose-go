package findfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"goose-go/internal/conversation"
	"goose-go/internal/tools"
)

const defaultMaxResults = 200

type Tool struct{}

type Arguments struct {
	Path       string `json:"path,omitempty"`
	Pattern    string `json:"pattern"`
	MaxResults int    `json:"max_results,omitempty"`
}

type StructuredResult struct {
	Path       string   `json:"path"`
	Pattern    string   `json:"pattern"`
	Truncated  bool     `json:"truncated"`
	MatchPaths []string `json:"match_paths"`
}

func New() Tool { return Tool{} }

func (Tool) Definition() tools.Definition {
	return tools.Definition{
		Name:            "find_files",
		Description:     "Find files under a directory using a substring or glob pattern.",
		InputSchema:     json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"pattern":{"type":"string"},"max_results":{"type":"integer","minimum":1}},"required":["pattern"],"additionalProperties":false}`),
		Capability:      tools.CapabilityRead,
		ApprovalDefault: tools.ApprovalDefaultAllow,
	}
}

func (Tool) Run(ctx context.Context, call tools.Call) (tools.Result, error) {
	_ = ctx

	var args Arguments
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return tools.Result{}, fmt.Errorf("%w: decode find_files arguments: %v", tools.ErrInvalidToolCall, err)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return tools.Result{}, fmt.Errorf("%w: find_files pattern is required", tools.ErrInvalidToolCall)
	}
	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxResults <= 0 {
		args.MaxResults = defaultMaxResults
	}

	root := args.Path
	if !filepath.IsAbs(root) && call.DefaultWorkingDir != "" {
		root = filepath.Join(call.DefaultWorkingDir, root)
	}
	root = filepath.Clean(root)

	var matches []string
	hitLimit := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if matchesPattern(args.Pattern, path, root) {
			matches = append(matches, path)
			if len(matches) >= args.MaxResults {
				hitLimit = true
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return tools.Result{}, fmt.Errorf("walk %s: %w", root, err)
	}
	slices.Sort(matches)
	truncated := hitLimit

	lines := make([]string, len(matches))
	copy(lines, matches)
	if truncated {
		lines = append(lines, "", "[truncated]")
	}
	structured, err := json.Marshal(StructuredResult{
		Path:       root,
		Pattern:    args.Pattern,
		Truncated:  truncated,
		MatchPaths: matches,
	})
	if err != nil {
		return tools.Result{}, fmt.Errorf("marshal find_files result: %w", err)
	}
	return tools.Result{
		ToolCallID: call.ID,
		Content:    []conversation.ToolResult{{Type: "text", Text: strings.Join(lines, "\n")}},
		Structured: structured,
	}, nil
}

func matchesPattern(pattern string, path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	base := filepath.Base(path)
	if hasGlob(pattern) {
		ok, _ := filepath.Match(pattern, rel)
		if ok {
			return true
		}
		ok, _ = filepath.Match(pattern, base)
		return ok
	}
	return strings.Contains(rel, pattern) || strings.Contains(base, pattern)
}

func hasGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}
