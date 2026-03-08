package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goose-go/internal/agent"
	"goose-go/internal/conversation"
	"goose-go/internal/provider"
	"goose-go/internal/provider/openaicodex"
	"goose-go/internal/session"
	sqlitestore "goose-go/internal/storage/sqlite"
	"goose-go/internal/tools"
	"goose-go/internal/tools/shell"
)

const (
	defaultRunSystemPrompt = "You are a concise terminal coding assistant. Use the shell tool when it is the clearest way to inspect or modify the local environment."
	defaultRunDBDir        = ".goose-go"
	defaultRunDBName       = "sessions.db"
)

type storeCloser interface {
	session.Store
	Close() error
}

type RunOptions struct {
	Approve       bool
	DebugProvider bool
	WorkingDir    string
	DBPath        string
	MaxTurns      int
}

var newRunProvider = func(debugOut io.Writer) (provider.Provider, error) {
	if debugOut != nil {
		return openaicodex.New(openaicodex.WithDebugWriter(debugOut))
	}
	return openaicodex.New()
}

var openRunStore = func(path string) (storeCloser, error) {
	return sqlitestore.Open(path)
}

func RunAgent(ctx context.Context, in io.Reader, out io.Writer, prompt string, opts RunOptions) error {
	if strings.TrimSpace(prompt) == "" {
		return errors.New("prompt is required")
	}

	workingDir, err := resolveWorkingDir(opts.WorkingDir)
	if err != nil {
		return err
	}
	if opts.MaxTurns <= 0 {
		opts.MaxTurns = 8
	}

	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(workingDir, defaultRunDBDir, defaultRunDBName)
	}

	store, err := openRunStore(dbPath)
	if err != nil {
		return fmt.Errorf("open session store: %w", err)
	}
	defer func() { _ = store.Close() }()

	var debugOut io.Writer
	if opts.DebugProvider {
		debugOut = out
	}
	p, err := newRunProvider(debugOut)
	if err != nil {
		return fmt.Errorf("create openai-codex provider: %w", err)
	}

	registry := tools.NewRegistry()
	if err := registry.Register(shell.New()); err != nil {
		return fmt.Errorf("register shell tool: %w", err)
	}

	record, err := store.CreateSession(ctx, session.CreateParams{
		Name:       sessionName(prompt),
		WorkingDir: workingDir,
		Type:       session.TypeTerminal,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	approvalMode := agent.ApprovalModeAuto
	var approver agent.Approver
	if opts.Approve {
		approvalMode = agent.ApprovalModeApprove
		approver = interactiveApprover{in: in, out: out}
	}

	runtime, err := agent.New(store, p, registry, agent.Config{
		SystemPrompt: defaultRunSystemPrompt,
		Model: provider.ModelConfig{
			Provider: "openai-codex",
			Model:    "gpt-5-codex",
		},
		MaxTurns:     opts.MaxTurns,
		ApprovalMode: approvalMode,
	}, approver)
	if err != nil {
		return fmt.Errorf("create agent runtime: %w", err)
	}

	result, err := runtime.Reply(ctx, record.ID, prompt)
	if err != nil && !errors.Is(err, agent.ErrMaxTurnsExceeded) {
		return err
	}

	if _, werr := fmt.Fprintf(out, "session: %s\n", result.Session.ID); werr != nil {
		return fmt.Errorf("write session header: %w", werr)
	}
	if rerr := renderConversation(out, result.Session.Conversation); rerr != nil {
		return rerr
	}

	if errors.Is(err, agent.ErrMaxTurnsExceeded) {
		return err
	}
	if result.Status == agent.StatusAwaitingApproval {
		return errors.New("agent is awaiting approval")
	}
	return nil
}

func RunAgentContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

type interactiveApprover struct {
	in  io.Reader
	out io.Writer
}

func (a interactiveApprover) Decide(_ context.Context, req agent.ApprovalRequest) (agent.ApprovalDecision, error) {
	if _, err := fmt.Fprintf(a.out, "approve tool %s %s? [y/N]: ", req.ToolCall.Name, compactArgs(req.ToolCall.Arguments)); err != nil {
		return "", fmt.Errorf("write approval prompt: %w", err)
	}

	reader := bufio.NewReader(a.in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read approval input: %w", err)
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "y" || answer == "yes" {
		return agent.ApprovalDecisionAllow, nil
	}
	return agent.ApprovalDecisionDeny, nil
}

func renderConversation(out io.Writer, conv conversation.Conversation) error {
	toolNames := map[string]string{}
	for _, message := range conv.Messages {
		for _, content := range message.Content {
			if content.Type == conversation.ContentTypeToolRequest && content.ToolRequest != nil {
				toolNames[content.ToolRequest.ID] = content.ToolRequest.Name
			}
		}
	}

	for _, message := range conv.Messages {
		switch message.Role {
		case conversation.RoleUser:
			if err := renderTextBlocks(out, "user", message.Content); err != nil {
				return err
			}
		case conversation.RoleAssistant:
			for _, content := range message.Content {
				switch content.Type {
				case conversation.ContentTypeText:
					if _, err := fmt.Fprintf(out, "assistant> %s\n", content.Text.Text); err != nil {
						return fmt.Errorf("write assistant text: %w", err)
					}
				case conversation.ContentTypeToolRequest:
					if _, err := fmt.Fprintf(out, "assistant requested tool %s %s\n", content.ToolRequest.Name, compactArgs(content.ToolRequest.Arguments)); err != nil {
						return fmt.Errorf("write assistant tool request: %w", err)
					}
				}
			}
		case conversation.RoleTool:
			for _, content := range message.Content {
				if content.Type != conversation.ContentTypeToolResponse || content.ToolResponse == nil {
					continue
				}
				name := toolNames[content.ToolResponse.ID]
				if name == "" {
					name = "unknown"
				}
				for _, result := range content.ToolResponse.Content {
					if _, err := fmt.Fprintf(out, "tool[%s]> %s\n", name, result.Text); err != nil {
						return fmt.Errorf("write tool response: %w", err)
					}
				}
			}
		}
	}
	return nil
}

func renderTextBlocks(out io.Writer, prefix string, content []conversation.Content) error {
	for _, item := range content {
		if item.Type != conversation.ContentTypeText || item.Text == nil {
			continue
		}
		if _, err := fmt.Fprintf(out, "%s> %s\n", prefix, item.Text.Text); err != nil {
			return fmt.Errorf("write %s text: %w", prefix, err)
		}
	}
	return nil
}

func resolveWorkingDir(workingDir string) (string, error) {
	if workingDir != "" {
		return workingDir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return cwd, nil
}

func sessionName(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if len(trimmed) <= 48 {
		return trimmed
	}
	return trimmed[:45] + "..."
}

func compactArgs(raw json.RawMessage) string {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "{}"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return string(raw)
	}
	return string(data)
}
