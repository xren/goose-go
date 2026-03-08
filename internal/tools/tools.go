package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"goose-go/internal/conversation"
)

type Tool interface {
	Definition() Definition
	Run(ctx context.Context, call Call) (Result, error)
}

type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type Call struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Arguments         json.RawMessage `json:"arguments,omitempty"`
	DefaultWorkingDir string          `json:"-"`
}

type Result struct {
	ToolCallID string                    `json:"tool_call_id"`
	IsError    bool                      `json:"is_error"`
	Content    []conversation.ToolResult `json:"content,omitempty"`
	Structured json.RawMessage           `json:"structured,omitempty"`
}

type Registry struct {
	tools map[string]Tool
}

var (
	ErrToolNotFound      = errors.New("tool not found")
	ErrDuplicateTool     = errors.New("duplicate tool")
	ErrInvalidToolCall   = errors.New("invalid tool call")
	ErrInvalidToolResult = errors.New("invalid tool result")
)

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(tool Tool) error {
	def := tool.Definition()
	if err := def.Validate(); err != nil {
		return fmt.Errorf("definition: %w", err)
	}
	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateTool, def.Name)
	}
	r.tools[def.Name] = tool
	return nil
}

func (r *Registry) Get(name string) (Tool, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}
	return tool, nil
}

func (r *Registry) Definitions() []Definition {
	defs := make([]Definition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

func (r *Registry) Execute(ctx context.Context, call Call) (Result, error) {
	if err := call.Validate(); err != nil {
		return Result{}, err
	}

	tool, err := r.Get(call.Name)
	if err != nil {
		return Result{}, err
	}

	result, err := tool.Run(ctx, call)
	if err != nil {
		return Result{}, err
	}
	if err := result.Validate(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (d Definition) Validate() error {
	if d.Name == "" {
		return errors.New("tool definition name is required")
	}
	return nil
}

func (c Call) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidToolCall)
	}
	if c.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidToolCall)
	}
	return nil
}

func (r Result) Validate() error {
	if r.ToolCallID == "" {
		return fmt.Errorf("%w: tool_call_id is required", ErrInvalidToolResult)
	}
	return nil
}

func (r Result) ToConversationContent() conversation.Content {
	return conversation.ToolResponse(r.ToolCallID, r.IsError, r.Content, r.Structured)
}
