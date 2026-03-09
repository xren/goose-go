package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"goose-go/internal/conversation"
	"goose-go/internal/tools"
)

type Tool struct{}

type Arguments struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir,omitempty"`
}

type StructuredResult struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir,omitempty"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
}

func New() Tool {
	return Tool{}
}

func (Tool) Definition() tools.Definition {
	return tools.Definition{
		Name:        "shell",
		Description: "Run a shell command in the local environment.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": { "type": "string" },
    "working_dir": { "type": "string" }
  },
  "required": ["command"],
  "additionalProperties": false
}`),
		Capability:      tools.CapabilityExec,
		ApprovalDefault: tools.ApprovalDefaultAsk,
	}
}

func (Tool) Run(ctx context.Context, call tools.Call) (tools.Result, error) {
	var args Arguments
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return tools.Result{}, fmt.Errorf("%w: decode shell arguments: %v", tools.ErrInvalidToolCall, err)
	}
	if args.Command == "" {
		return tools.Result{}, fmt.Errorf("%w: shell command is required", tools.ErrInvalidToolCall)
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", "-lc", args.Command)
	workingDir := args.WorkingDir
	if workingDir == "" {
		workingDir = call.DefaultWorkingDir
	}
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	output, err := cmd.CombinedOutput()
	exitCode := 0
	isError := false
	if err != nil {
		isError = true
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return tools.Result{}, fmt.Errorf("run shell command: %w", err)
		}
	}

	structured := StructuredResult{
		Command:    args.Command,
		WorkingDir: workingDir,
		ExitCode:   exitCode,
		Stdout:     string(output),
	}
	structuredJSON, err := json.Marshal(structured)
	if err != nil {
		return tools.Result{}, fmt.Errorf("marshal shell result: %w", err)
	}

	content := []conversation.ToolResult{}
	if len(output) > 0 {
		content = append(content, conversation.ToolResult{
			Type: "text",
			Text: string(output),
		})
	}
	if len(content) == 0 {
		content = append(content, conversation.ToolResult{
			Type: "text",
			Text: "(no output)",
		})
	}

	return tools.Result{
		ToolCallID: call.ID,
		IsError:    isError,
		Content:    content,
		Structured: structuredJSON,
	}, nil
}
