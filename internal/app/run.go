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
	"goose-go/internal/session"
	"goose-go/internal/tools"
)

const (
	defaultRunDBDir  = ".goose-go"
	defaultRunDBName = "sessions.db"
)

var ErrInterrupted = errors.New("interrupted")

type storeCloser interface {
	session.Store
	Close() error
}

type RunOptions struct {
	RequireApproval bool
	Approve         bool
	DebugProvider   bool
	Provider        string
	Model           string
	WorkingDir      string
	DBPath          string
	TraceDir        string
	MaxTurns        int
	SessionID       string
}

func RunAgent(ctx context.Context, in io.Reader, out io.Writer, prompt string, opts RunOptions) error {
	if strings.TrimSpace(prompt) == "" {
		return errors.New("prompt is required")
	}

	if cmd, ok := LocalCommand(prompt, opts.Provider, opts.Model); ok {
		_, err := fmt.Fprintf(out, "system> /%s\nsystem> %s\n", cmd.Name, strings.ReplaceAll(cmd.Output, "\n", "\nsystem> "))
		if err != nil {
			return fmt.Errorf("write local command output: %w", err)
		}
		return nil
	}

	runtime, err := OpenRuntime(in, out, opts)
	if err != nil {
		return err
	}
	defer func() { _ = runtime.Close() }()

	record, _, err := runtime.LoadOrCreateSession(ctx, prompt, opts.SessionID)
	if err != nil {
		return err
	}

	traceWriter, err := runtime.OpenTraceWriter(record.ID)
	if err != nil {
		return fmt.Errorf("open trace writer: %w", err)
	}
	defer func() { _ = traceWriter.Close() }()

	if _, werr := fmt.Fprintf(out, "session: %s\n", record.ID); werr != nil {
		return fmt.Errorf("write session header: %w", werr)
	}

	stream, err := runtime.Agent().ReplyStream(ctx, record.ID, prompt)
	if err != nil {
		return diagnoseRunError(providerForDiagnostic(runtime), err, opts.DebugProvider)
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
	if finalErr != nil {
		return diagnoseRunError(providerForDiagnostic(runtime), finalErr, opts.DebugProvider)
	}
	return nil
}

type traceWriter struct {
	file *os.File
	enc  *json.Encoder
}

type EventRecorder interface {
	Write(agent.Event) error
	Close() error
}

type traceRecord struct {
	RecordedAt       time.Time                 `json:"recorded_at"`
	Type             agent.EventType           `json:"type"`
	SessionID        string                    `json:"session_id,omitempty"`
	Turn             int                       `json:"turn,omitempty"`
	Delta            string                    `json:"delta,omitempty"`
	Message          *conversation.Message     `json:"message,omitempty"`
	Compaction       *session.Compaction       `json:"compaction,omitempty"`
	CompactionReason session.CompactionTrigger `json:"compaction_reason,omitempty"`
	TokensBefore     int                       `json:"tokens_before,omitempty"`
	ToolCall         *tools.Call               `json:"tool_call,omitempty"`
	ToolResult       *tools.Result             `json:"tool_result,omitempty"`
	Approval         *agent.ApprovalRequest    `json:"approval_request,omitempty"`
	Decision         agent.ApprovalDecision    `json:"approval_decision,omitempty"`
	Result           *agent.Result             `json:"result,omitempty"`
	Error            string                    `json:"error,omitempty"`
}

func openTraceWriter(traceDir string, sessionID string) (EventRecorder, error) {
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
		RecordedAt:       time.Now().UTC(),
		Type:             event.Type,
		SessionID:        event.SessionID,
		Turn:             event.Turn,
		Delta:            event.Delta,
		Message:          event.Message,
		Compaction:       event.Compaction,
		CompactionReason: event.CompactionReason,
		TokensBefore:     event.TokensBefore,
		ToolCall:         event.ToolCall,
		ToolResult:       event.ToolResult,
		Approval:         event.ApprovalRequest,
		Decision:         event.ApprovalDecision,
		Result:           event.Result,
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
	case agent.EventTypeCompactionStarted:
		if r.assistantLineOpen {
			if _, err := io.WriteString(r.out, "\n"); err != nil {
				return fmt.Errorf("terminate assistant line: %w", err)
			}
			r.assistantLineOpen = false
		}
		if _, err := fmt.Fprintf(r.out, "system> compacting context (%s, %d tokens)\n", event.CompactionReason, event.TokensBefore); err != nil {
			return fmt.Errorf("write compaction start: %w", err)
		}
	case agent.EventTypeCompactionCompleted:
		if _, err := fmt.Fprintf(r.out, "system> compaction complete (%s)\n", event.CompactionReason); err != nil {
			return fmt.Errorf("write compaction complete: %w", err)
		}
	case agent.EventTypeCompactionFailed:
		if _, err := fmt.Fprintf(r.out, "system> compaction failed (%s)\n", event.CompactionReason); err != nil {
			return fmt.Errorf("write compaction failure: %w", err)
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

	dbPath := resolveDBPath(workingDir, opts.DBPath)
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
	return context.WithCancel(context.Background())
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

func loadOrCreateSession(ctx context.Context, store session.Store, prompt string, workingDir string, providerName string, modelName string, sessionID string) (session.Session, int, error) {
	if sessionID == "" {
		record, err := store.CreateSession(ctx, session.CreateParams{
			Name:       sessionName(prompt),
			WorkingDir: workingDir,
			Provider:   providerName,
			Model:      modelName,
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

func providerForDiagnostic(runtime *Runtime) string {
	if runtime == nil {
		return defaultProviderName
	}
	providerName, _ := runtime.ProviderModel()
	if providerName == "" {
		return defaultProviderName
	}
	return providerName
}
