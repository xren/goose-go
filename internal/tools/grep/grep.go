package grep

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"goose-go/internal/conversation"
	"goose-go/internal/tools"
)

const (
	defaultMaxResults = 100
	maxFileBytes      = 1 << 20
)

type Tool struct{}

type Arguments struct {
	Path       string `json:"path,omitempty"`
	Pattern    string `json:"pattern"`
	MaxResults int    `json:"max_results,omitempty"`
}

type Match struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type StructuredResult struct {
	Path      string  `json:"path"`
	Pattern   string  `json:"pattern"`
	Truncated bool    `json:"truncated"`
	Matches   []Match `json:"matches"`
}

func New() Tool { return Tool{} }

func (Tool) Definition() tools.Definition {
	return tools.Definition{
		Name:            "grep",
		Description:     "Search text files under a directory with a regular expression.",
		InputSchema:     json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"pattern":{"type":"string"},"max_results":{"type":"integer","minimum":1}},"required":["pattern"],"additionalProperties":false}`),
		Capability:      tools.CapabilityRead,
		ApprovalDefault: tools.ApprovalDefaultAllow,
	}
}

func (Tool) Run(ctx context.Context, call tools.Call) (tools.Result, error) {
	_ = ctx

	var args Arguments
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return tools.Result{}, fmt.Errorf("%w: decode grep arguments: %v", tools.ErrInvalidToolCall, err)
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return tools.Result{}, fmt.Errorf("%w: grep pattern is required", tools.ErrInvalidToolCall)
	}
	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxResults <= 0 {
		args.MaxResults = defaultMaxResults
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return tools.Result{}, fmt.Errorf("%w: invalid grep pattern: %v", tools.ErrInvalidToolCall, err)
	}

	root := args.Path
	if !filepath.IsAbs(root) && call.DefaultWorkingDir != "" {
		root = filepath.Join(call.DefaultWorkingDir, root)
	}
	root = filepath.Clean(root)

	matches := make([]Match, 0, args.MaxResults)
	hitLimit := false
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxFileBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !utf8.Valid(data) {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for idx, line := range lines {
			if !re.MatchString(line) {
				continue
			}
			matches = append(matches, Match{
				Path: path,
				Line: idx + 1,
				Text: line,
			})
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

	truncated := hitLimit
	lines := make([]string, 0, len(matches)+1)
	for _, match := range matches {
		lines = append(lines, fmt.Sprintf("%s:%d: %s", match.Path, match.Line, match.Text))
	}
	if truncated {
		lines = append(lines, "", "[truncated]")
	}

	structured, err := json.Marshal(StructuredResult{
		Path:      root,
		Pattern:   args.Pattern,
		Truncated: truncated,
		Matches:   matches,
	})
	if err != nil {
		return tools.Result{}, fmt.Errorf("marshal grep result: %w", err)
	}
	return tools.Result{
		ToolCallID: call.ID,
		Content:    []conversation.ToolResult{{Type: "text", Text: strings.Join(lines, "\n")}},
		Structured: structured,
	}, nil
}
