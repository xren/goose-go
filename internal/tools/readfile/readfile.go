package readfile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"goose-go/internal/conversation"
	"goose-go/internal/tools"
)

const defaultMaxBytes = 64 * 1024

type Tool struct{}

type Arguments struct {
	Path     string `json:"path"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

type StructuredResult struct {
	Path      string `json:"path"`
	ReadBytes int    `json:"read_bytes"`
	MaxBytes  int    `json:"max_bytes"`
	Truncated bool   `json:"truncated"`
}

func New() Tool {
	return Tool{}
}

func (Tool) Definition() tools.Definition {
	return tools.Definition{
		Name:            "read_file",
		Description:     "Read a UTF-8 text file from the local workspace.",
		InputSchema:     json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"max_bytes":{"type":"integer","minimum":1}},"required":["path"],"additionalProperties":false}`),
		Capability:      tools.CapabilityRead,
		ApprovalDefault: tools.ApprovalDefaultAllow,
	}
}

func (Tool) Run(ctx context.Context, call tools.Call) (tools.Result, error) {
	_ = ctx

	var args Arguments
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return tools.Result{}, fmt.Errorf("%w: decode read_file arguments: %v", tools.ErrInvalidToolCall, err)
	}
	if args.Path == "" {
		return tools.Result{}, fmt.Errorf("%w: read_file path is required", tools.ErrInvalidToolCall)
	}

	maxBytes := args.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
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
	if info.IsDir() {
		return tools.Result{}, fmt.Errorf("read_file %s: path is a directory", resolvedPath)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return tools.Result{}, fmt.Errorf("read %s: %w", resolvedPath, err)
	}
	if !utf8.Valid(data) {
		return tools.Result{}, fmt.Errorf("read_file %s: file is not valid UTF-8 text", resolvedPath)
	}

	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		truncated = true
	}

	structuredJSON, err := json.Marshal(StructuredResult{
		Path:      resolvedPath,
		ReadBytes: len(data),
		MaxBytes:  maxBytes,
		Truncated: truncated,
	})
	if err != nil {
		return tools.Result{}, fmt.Errorf("marshal read_file result: %w", err)
	}

	contentText := string(data)
	if truncated {
		contentText += "\n\n[truncated]"
	}

	return tools.Result{
		ToolCallID: call.ID,
		Content: []conversation.ToolResult{{
			Type: "text",
			Text: contentText,
		}},
		Structured: structuredJSON,
	}, nil
}
