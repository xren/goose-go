package agent

import (
	"context"
	"errors"
	"fmt"

	"goose-go/internal/compaction"
	"goose-go/internal/provider"
	"goose-go/internal/session"
	"goose-go/internal/tools"
)

type ApprovalMode string

type Status string

type ApprovalDecision string

const (
	ApprovalModeAuto    ApprovalMode = "auto"
	ApprovalModeApprove ApprovalMode = "approve"

	StatusCompleted        Status = "completed"
	StatusAwaitingApproval Status = "awaiting_approval"

	ApprovalDecisionAllow ApprovalDecision = "allow"
	ApprovalDecisionDeny  ApprovalDecision = "deny"
)

var ErrMaxTurnsExceeded = errors.New("max turns exceeded")
var ErrApprovalNotPending = errors.New("approval not pending")

type Approver interface {
	Decide(context.Context, ApprovalRequest) (ApprovalDecision, error)
}

type ApproverFunc func(context.Context, ApprovalRequest) (ApprovalDecision, error)

func (f ApproverFunc) Decide(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	return f(ctx, req)
}

type Config struct {
	SystemPrompt string
	Model        provider.ModelConfig
	MaxTurns     int
	ApprovalMode ApprovalMode
	Compaction   compaction.Settings
}

type ApprovalRequest struct {
	SessionID string     `json:"session_id"`
	ToolCall  tools.Call `json:"tool_call"`
}

type Result struct {
	Status             Status          `json:"status"`
	Session            session.Session `json:"session"`
	Turns              int             `json:"turns"`
	PendingApprovalFor *tools.Call     `json:"pending_approval_for,omitempty"`
}

type Agent struct {
	provider provider.Provider
	store    session.Store
	tools    *tools.Registry
	config   Config
	approver Approver
}

func New(store session.Store, p provider.Provider, registry *tools.Registry, config Config, approver Approver) (*Agent, error) {
	if store == nil {
		return nil, errors.New("session store is required")
	}
	if p == nil {
		return nil, errors.New("provider is required")
	}
	if registry == nil {
		return nil, errors.New("tool registry is required")
	}
	if config.ApprovalMode == "" {
		config.ApprovalMode = ApprovalModeAuto
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &Agent{
		provider: p,
		store:    store,
		tools:    registry,
		config:   config,
		approver: approver,
	}, nil
}

func (c Config) validate() error {
	if err := c.Model.Validate(); err != nil {
		return fmt.Errorf("model: %w", err)
	}
	if err := c.Compaction.Validate(); err != nil {
		return fmt.Errorf("compaction: %w", err)
	}
	if c.MaxTurns <= 0 {
		return errors.New("max turns must be > 0")
	}
	return nil
}

func (c Config) contextWindow() int {
	if c.Model.ContextWindow > 0 {
		return c.Model.ContextWindow
	}
	return 200000
}

func toolDefinitions(defs []tools.Definition) []provider.ToolDefinition {
	out := make([]provider.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, provider.ToolDefinition{
			Name:        def.Name,
			Description: def.Description,
			InputSchema: def.InputSchema,
		})
	}
	return out
}
