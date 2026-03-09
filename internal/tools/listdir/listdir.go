package listdir

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"goose-go/internal/conversation"
	"goose-go/internal/tools"
)

const defaultMaxEntries = 200

type Tool struct{}

type Arguments struct {
	Path       string `json:"path,omitempty"`
	MaxEntries int    `json:"max_entries,omitempty"`
}

type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type StructuredResult struct {
	Path      string  `json:"path"`
	Truncated bool    `json:"truncated"`
	Entries   []Entry `json:"entries"`
}

func New() Tool { return Tool{} }

func (Tool) Definition() tools.Definition {
	return tools.Definition{
		Name:            "list_dir",
		Description:     "List directory entries from the local workspace.",
		InputSchema:     json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"max_entries":{"type":"integer","minimum":1}},"additionalProperties":false}`),
		Capability:      tools.CapabilityRead,
		ApprovalDefault: tools.ApprovalDefaultAllow,
	}
}

func (Tool) Run(ctx context.Context, call tools.Call) (tools.Result, error) {
	_ = ctx

	var args Arguments
	if len(call.Arguments) > 0 {
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return tools.Result{}, fmt.Errorf("%w: decode list_dir arguments: %v", tools.ErrInvalidToolCall, err)
		}
	}
	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxEntries <= 0 {
		args.MaxEntries = defaultMaxEntries
	}

	resolvedPath := args.Path
	if !filepath.IsAbs(resolvedPath) && call.DefaultWorkingDir != "" {
		resolvedPath = filepath.Join(call.DefaultWorkingDir, resolvedPath)
	}
	resolvedPath = filepath.Clean(resolvedPath)

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return tools.Result{}, fmt.Errorf("stat %s: %w", resolvedPath, err)
	}
	if !info.IsDir() {
		return tools.Result{}, fmt.Errorf("list_dir %s: path is not a directory", resolvedPath)
	}

	dirEntries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return tools.Result{}, fmt.Errorf("read directory %s: %w", resolvedPath, err)
	}
	sort.Slice(dirEntries, func(i, j int) bool { return dirEntries[i].Name() < dirEntries[j].Name() })

	truncated := len(dirEntries) > args.MaxEntries
	if truncated {
		dirEntries = dirEntries[:args.MaxEntries]
	}

	entries := make([]Entry, 0, len(dirEntries))
	lines := make([]string, 0, len(dirEntries)+1)
	for _, entry := range dirEntries {
		entryInfo, err := entry.Info()
		if err != nil {
			return tools.Result{}, fmt.Errorf("stat directory entry %s: %w", entry.Name(), err)
		}
		path := filepath.Join(resolvedPath, entry.Name())
		item := Entry{
			Name:  entry.Name(),
			Path:  path,
			IsDir: entry.IsDir(),
			Size:  entryInfo.Size(),
		}
		entries = append(entries, item)
		line := entry.Name()
		if entry.IsDir() {
			line += "/"
		}
		lines = append(lines, line)
	}
	if truncated {
		lines = append(lines, "", "[truncated]")
	}

	structured, err := json.Marshal(StructuredResult{
		Path:      resolvedPath,
		Truncated: truncated,
		Entries:   entries,
	})
	if err != nil {
		return tools.Result{}, fmt.Errorf("marshal list_dir result: %w", err)
	}

	return tools.Result{
		ToolCallID: call.ID,
		Content:    []conversation.ToolResult{{Type: "text", Text: strings.Join(lines, "\n")}},
		Structured: structured,
	}, nil
}
