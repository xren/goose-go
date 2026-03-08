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

var ErrInterrupted = errors.New("interrupted")

type storeCloser interface {
	session.Store
	Close() error
}

type RunOptions struct {
	Approve       bool
	DebugProvider bool
	WorkingDir    string
	DBPath        string
	TraceDir      string
	MaxTurns      int
	SessionID     string
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

	record, _, err := loadOrCreateSession(ctx, store, prompt, workingDir, opts.SessionID)
	if err != nil {
		return err
	}

	traceDir := opts.TraceDir
	if traceDir == "" {
		traceDir = filepath.Join(filepath.Dir(dbPath), "traces")
	}
	traceWriter, err := openTraceWriter(traceDir, record.ID)
	if err != nil {
		return fmt.Errorf("open trace writer: %w", err)
	}
	defer func() { _ = traceWriter.Close() }()

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

	if _, werr := fmt.Fprintf(out, "session: %s\n", record.ID); werr != nil {
		return fmt.Errorf("write session header: %w", werr)
	}

	stream, err := runtime.ReplyStream(ctx, record.ID, prompt)
	if err != nil {
		return err
	}

	renderer := newEventRenderer(out)
	var finalErr error
	for event := range stream {
		if err := traceWriter.Write(event); err != nil {
			return fmt.Errorf("write trace event: %w", err)
		}
		if err := renderer.Render(event); err != nil {
			return err
		}

		switch event.Type {
		case agent.EventTypeRunCompleted:
			if event.Result != nil && event.Result.Status == agent.StatusAwaitingApproval {
				finalErr = errors.New("agent is awaiting approval")
			}
		case agent.EventTypeRunInterrupted:
			finalErr = ErrInterrupted
		case agent.EventTypeRunFailed:
			finalErr = event.Err
		}
	}
	if err := renderer.Finish(); err != nil {
		return err
	}

	if errors.Is(finalErr, context.Canceled) {
		finalErr = ErrInterrupted
	}
	if errors.Is(finalErr, agent.ErrMaxTurnsExceeded) {
		return finalErr
	}
	return finalErr
}

type traceWriter struct {
	file *os.File
	enc  *json.Encoder
}

type traceRecord struct {
	RecordedAt time.Time              `json:"recorded_at"`
	Type       agent.EventType        `json:"type"`
	SessionID  string                 `json:"session_id,omitempty"`
	Turn       int                    `json:"turn,omitempty"`
	Delta      string                 `json:"delta,omitempty"`
	Message    *conversation.Message  `json:"message,omitempty"`
	ToolCall   *tools.Call            `json:"tool_call,omitempty"`
	ToolResult *tools.Result          `json:"tool_result,omitempty"`
	Approval   *agent.ApprovalRequest `json:"approval_request,omitempty"`
	Decision   agent.ApprovalDecision `json:"approval_decision,omitempty"`
	Result     *agent.Result          `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

func openTraceWriter(traceDir string, sessionID string) (*traceWriter, error) {
	if err := os.MkdirAll(traceDir, 0o755); err != nil {
		return nil, fmt.Errorf("create trace directory: %w", err)
	}
	path := filepath.Join(traceDir, sessionID+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open trace file: %w", err)
	}
	return &traceWriter{
		file: file,
		enc:  json.NewEncoder(file),
	}, nil
}

func (w *traceWriter) Write(event agent.Event) error {
	record := traceRecord{
		RecordedAt: time.Now().UTC(),
		Type:       event.Type,
		SessionID:  event.SessionID,
		Turn:       event.Turn,
		Delta:      event.Delta,
		Message:    event.Message,
		ToolCall:   event.ToolCall,
		ToolResult: event.ToolResult,
		Approval:   event.ApprovalRequest,
		Decision:   event.ApprovalDecision,
		Result:     event.Result,
	}
	if event.Err != nil {
		record.Error = event.Err.Error()
	}
	if err := w.enc.Encode(record); err != nil {
		return err
	}
	return nil
}

func (w *traceWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	return w.file.Close()
}

type eventRenderer struct {
	out                 io.Writer
	assistantLineOpen   bool
	assistantDeltaShown bool
}

func newEventRenderer(out io.Writer) *eventRenderer {
	return &eventRenderer{out: out}
}

func (r *eventRenderer) Render(event agent.Event) error {
	switch event.Type {
	case agent.EventTypeTurnStarted:
		r.assistantDeltaShown = false
	case agent.EventTypeUserMessagePersisted:
		if event.Message != nil {
			return renderTextBlocks(r.out, "user", event.Message.Content)
		}
	case agent.EventTypeProviderTextDelta:
		if !r.assistantLineOpen {
			if _, err := io.WriteString(r.out, "assistant> "); err != nil {
				return fmt.Errorf("write assistant prefix: %w", err)
			}
			r.assistantLineOpen = true
		}
		if _, err := io.WriteString(r.out, event.Delta); err != nil {
			return fmt.Errorf("write assistant delta: %w", err)
		}
		r.assistantDeltaShown = true
	case agent.EventTypeAssistantMessageComplete:
		if event.Message == nil {
			return nil
		}
		if r.assistantLineOpen {
			if _, err := io.WriteString(r.out, "\n"); err != nil {
				return fmt.Errorf("terminate assistant line: %w", err)
			}
			r.assistantLineOpen = false
		}
		if r.assistantDeltaShown {
			return nil
		}
		for _, content := range event.Message.Content {
			if content.Type != conversation.ContentTypeText || content.Text == nil {
				continue
			}
			if _, err := fmt.Fprintf(r.out, "assistant> %s\n", content.Text.Text); err != nil {
				return fmt.Errorf("write assistant text: %w", err)
			}
		}
	case agent.EventTypeToolCallDetected:
		if r.assistantLineOpen {
			if _, err := io.WriteString(r.out, "\n"); err != nil {
				return fmt.Errorf("terminate assistant line: %w", err)
			}
			r.assistantLineOpen = false
		}
		if event.ToolCall != nil {
			if _, err := fmt.Fprintf(r.out, "assistant requested tool %s %s\n", event.ToolCall.Name, compactArgs(event.ToolCall.Arguments)); err != nil {
				return fmt.Errorf("write assistant tool request: %w", err)
			}
		}
	case agent.EventTypeToolMessagePersisted:
		if event.ToolResult != nil && event.ToolCall != nil {
			for _, result := range event.ToolResult.Content {
				if _, err := fmt.Fprintf(r.out, "tool[%s]> %s\n", event.ToolCall.Name, result.Text); err != nil {
					return fmt.Errorf("write tool response: %w", err)
				}
			}
		}
	case agent.EventTypeRunInterrupted:
		if r.assistantLineOpen {
			if _, err := io.WriteString(r.out, "\n"); err != nil {
				return fmt.Errorf("terminate assistant line: %w", err)
			}
			r.assistantLineOpen = false
		}
		if _, err := io.WriteString(r.out, "interrupted\n"); err != nil {
			return fmt.Errorf("write interrupted notice: %w", err)
		}
	}
	return nil
}

func (r *eventRenderer) Finish() error {
	if !r.assistantLineOpen {
		return nil
	}
	if _, err := io.WriteString(r.out, "\n"); err != nil {
		return fmt.Errorf("terminate assistant line: %w", err)
	}
	r.assistantLineOpen = false
	return nil
}

func ListSessions(ctx context.Context, out io.Writer, opts RunOptions) error {
	workingDir, err := resolveWorkingDir(opts.WorkingDir)
	if err != nil {
		return err
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

	sessions, err := store.ListSessions(ctx)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		_, err := io.WriteString(out, "no sessions\n")
		return err
	}

	for _, item := range sessions {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%d\n", item.ID, item.Name, item.WorkingDir, item.MessageCount); err != nil {
			return fmt.Errorf("write session list: %w", err)
		}
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

func loadOrCreateSession(ctx context.Context, store session.Store, prompt string, workingDir string, sessionID string) (session.Session, int, error) {
	if sessionID == "" {
		record, err := store.CreateSession(ctx, session.CreateParams{
			Name:       sessionName(prompt),
			WorkingDir: workingDir,
			Type:       session.TypeTerminal,
		})
		if err != nil {
			return session.Session{}, 0, fmt.Errorf("create session: %w", err)
		}
		return record, 0, nil
	}

	record, err := store.GetSession(ctx, sessionID)
	if err != nil {
		return session.Session{}, 0, fmt.Errorf("load session %s: %w", sessionID, err)
	}
	return record, len(record.Conversation.Messages), nil
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
