package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
)

func TestRunAgentAutoModeRendersTranscript(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(req provider.Request) []provider.Event {
				if hasToolResponse(req.Messages) {
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
					return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
				}
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf hello"}`)))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunAgent(context.Background(), strings.NewReader(""), &out, "say hello", RunOptions{WorkingDir: t.TempDir(), DBPath: t.TempDir() + "/sessions.db"})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	got := out.String()
	for _, want := range []string{"session:", "user> say hello", "assistant requested tool shell", "tool[shell]> hello", "assistant> done"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got %q", want, got)
		}
	}
}

func TestRunAgentModelCommandReturnsConfiguredRuntimeModelWithoutStartingSession(t *testing.T) {
	var out bytes.Buffer
	err := RunAgent(context.Background(), strings.NewReader(""), &out, "/model", RunOptions{})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	got := out.String()
	for _, want := range []string{"system> /model", "system> provider: openai-codex", "system> model: gpt-5-codex"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got %q", want, got)
		}
	}
	if strings.Contains(got, "session:") {
		t.Fatalf("expected local command to avoid session creation, got %q", got)
	}
}

func TestRunAgentContextHasNoDeadline(t *testing.T) {
	ctx, cancel := RunAgentContext()
	defer cancel()

	if _, ok := ctx.Deadline(); ok {
		t.Fatal("expected run agent context to have no deadline")
	}
}

func TestRunAgentApproveModePrompts(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(req provider.Request) []provider.Event {
				if hasToolResponse(req.Messages) {
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("understood"))
					return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
				}
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`)))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunAgent(context.Background(), strings.NewReader("n\n"), &out, "run pwd", RunOptions{RequireApproval: true, Approve: true, WorkingDir: t.TempDir(), DBPath: t.TempDir() + "/sessions.db"})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	got := out.String()
	for _, want := range []string{"approve tool shell", "tool[shell]> tool execution denied by user", "assistant> understood"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got %q", want, got)
		}
	}
}

func TestRunAgentResumeUsesExistingSessionAndOnlyPrintsNewMessages(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(req provider.Request) []provider.Event {
				last := req.Messages[len(req.Messages)-1]
				text := last.Content[0].Text.Text
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("reply to: "+text))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, nil
	}

	dbPath := t.TempDir() + "/sessions.db"
	var first bytes.Buffer
	if err := RunAgent(context.Background(), strings.NewReader(""), &first, "first prompt", RunOptions{WorkingDir: t.TempDir(), DBPath: dbPath}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	sessionID := strings.Split(strings.SplitN(first.String(), "\n", 2)[0], ": ")[1]

	var second bytes.Buffer
	if err := RunAgent(context.Background(), strings.NewReader(""), &second, "second prompt", RunOptions{WorkingDir: t.TempDir(), DBPath: dbPath, SessionID: sessionID}); err != nil {
		t.Fatalf("second run: %v", err)
	}

	got := second.String()
	if strings.Contains(got, "user> first prompt") {
		t.Fatalf("expected resumed run to omit old transcript, got %q", got)
	}
	for _, want := range []string{"session: " + sessionID, "user> second prompt", "assistant> reply to: second prompt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected resumed output to contain %q, got %q", want, got)
		}
	}
}

func TestListSessions(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(req provider.Request) []provider.Event {
				last := req.Messages[len(req.Messages)-1]
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("reply to: "+last.Content[0].Text.Text))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, nil
	}

	dbPath := t.TempDir() + "/sessions.db"
	if err := RunAgent(context.Background(), strings.NewReader(""), io.Discard, "alpha", RunOptions{WorkingDir: t.TempDir(), DBPath: dbPath}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := RunAgent(context.Background(), strings.NewReader(""), io.Discard, "beta", RunOptions{WorkingDir: t.TempDir(), DBPath: dbPath}); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var out bytes.Buffer
	if err := ListSessions(context.Background(), &out, RunOptions{WorkingDir: t.TempDir(), DBPath: dbPath}); err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	got := out.String()
	for _, want := range []string{"alpha", "beta", dbPath[:0]} {
		_ = want
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Fatalf("expected session list to contain created sessions, got %q", got)
	}
}

func TestRunAgentInterruptedRendersPersistedTranscript(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	ctx, cancel := context.WithCancel(context.Background())

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respondStream: func(ctx context.Context, req provider.Request) <-chan provider.Event {
				ch := make(chan provider.Event, 1)
				go func() {
					defer close(ch)
					cancel()
					<-ctx.Done()
					ch <- provider.Event{Type: provider.EventTypeError, Err: ctx.Err()}
				}()
				_ = req
				return ch
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunAgent(ctx, strings.NewReader(""), &out, "interrupt me", RunOptions{WorkingDir: t.TempDir(), DBPath: t.TempDir() + "/sessions.db"})
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected interrupted error, got %v", err)
	}
	got := out.String()
	for _, want := range []string{"session:", "user> interrupt me", "interrupted"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected interrupted output to contain %q, got %q", want, got)
		}
	}
}

func TestRunAgentWritesTraceFile(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(_ provider.Request) []provider.Event {
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong"))
				return []provider.Event{
					{Type: provider.EventTypeTextDelta, Delta: "pong"},
					{Type: provider.EventTypeMessageComplete, Message: &msg},
					{Type: provider.EventTypeDone},
				}
			},
		}, nil
	}

	var out bytes.Buffer
	traceDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	if err := RunAgent(context.Background(), strings.NewReader(""), &out, "ping", RunOptions{
		WorkingDir: t.TempDir(),
		DBPath:     dbPath,
		TraceDir:   traceDir,
	}); err != nil {
		t.Fatalf("run agent: %v", err)
	}

	sessionID := strings.Split(strings.SplitN(out.String(), "\n", 2)[0], ": ")[1]
	tracePath := filepath.Join(traceDir, sessionID+".jsonl")
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("expected trace lines")
	}

	var foundTypes []string
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode trace line: %v", err)
		}
		if _, ok := record["recorded_at"]; !ok {
			t.Fatalf("expected recorded_at in trace line: %s", line)
		}
		foundTypes = append(foundTypes, record["type"].(string))
	}

	for _, want := range []string{"run_started", "user_message_persisted", "turn_started", "provider_text_delta", "assistant_message_complete", "assistant_message_persisted", "run_completed"} {
		if !contains(foundTypes, want) {
			t.Fatalf("expected trace types to contain %q, got %v", want, foundTypes)
		}
	}
}

func TestRunAgentClassifiesAuthMissing(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return nil, errors.New("codex auth file not found at /tmp/auth.json; run `codex login`")
	}

	var out bytes.Buffer
	err := RunAgent(context.Background(), strings.NewReader(""), &out, "ping", RunOptions{
		WorkingDir: t.TempDir(),
		DBPath:     t.TempDir() + "/sessions.db",
	})
	var diag *DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("expected diagnostic error, got %T", err)
	}
	if diag.Category != DiagnosticAuthMissing {
		t.Fatalf("expected auth_missing, got %q", diag.Category)
	}
}

func TestRunAgentClassifiesProviderHTTPError(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(_ provider.Request) []provider.Event {
				return []provider.Event{
					{Type: provider.EventTypeError, Err: errors.New("codex request failed: status 401: nope")},
				}
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunAgent(context.Background(), strings.NewReader(""), &out, "ping", RunOptions{
		WorkingDir: t.TempDir(),
		DBPath:     t.TempDir() + "/sessions.db",
	})
	var diag *DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("expected diagnostic error, got %T", err)
	}
	if diag.Category != DiagnosticProviderHTTP {
		t.Fatalf("expected provider_http_error, got %q", diag.Category)
	}
}

func TestRunAgentClassifiesEmptyProviderResponse(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(_ provider.Request) []provider.Event {
				return []provider.Event{{Type: provider.EventTypeDone}}
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunAgent(context.Background(), strings.NewReader(""), &out, "ping", RunOptions{
		WorkingDir: t.TempDir(),
		DBPath:     t.TempDir() + "/sessions.db",
	})
	var diag *DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("expected diagnostic error, got %T", err)
	}
	if diag.Category != DiagnosticProviderEmpty {
		t.Fatalf("expected provider_empty_response, got %q", diag.Category)
	}
}

type scriptedAppProvider struct {
	respond       func(provider.Request) []provider.Event
	respondStream func(context.Context, provider.Request) <-chan provider.Event
}

func (s scriptedAppProvider) streamWithRequest(req provider.Request) (<-chan provider.Event, error) {
	ch := make(chan provider.Event, len(s.respond(req)))
	for _, event := range s.respond(req) {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (s scriptedAppProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	_ = ctx
	if s.respondStream != nil {
		return s.respondStream(ctx, req), nil
	}
	return s.streamWithRequest(req)
}

func hasToolResponse(messages []conversation.Message) bool {
	for _, message := range messages {
		for _, content := range message.Content {
			if content.Type == conversation.ContentTypeToolResponse {
				return true
			}
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestRunAgentResumeUsesPersistedProviderModel(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(req provider.Request) []provider.Event {
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text(req.Model.Model))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, nil
	}

	dbPath := t.TempDir() + "/sessions.db"
	var first bytes.Buffer
	if err := RunAgent(context.Background(), strings.NewReader("y\n"), &first, "first prompt", RunOptions{RequireApproval: true, Approve: true, WorkingDir: t.TempDir(), DBPath: dbPath, Model: "gpt-5.3-codex"}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	sessionID := strings.Split(strings.SplitN(first.String(), "\n", 2)[0], ": ")[1]

	var second bytes.Buffer
	if err := RunAgent(context.Background(), strings.NewReader("y\n"), &second, "second prompt", RunOptions{RequireApproval: true, Approve: true, WorkingDir: t.TempDir(), DBPath: dbPath, SessionID: sessionID}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !strings.Contains(second.String(), "assistant> gpt-5.3-codex") {
		t.Fatalf("expected resumed run to use persisted model, got %q", second.String())
	}
}

func TestRunAgentIncludesAGENTSInSystemPrompt(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	subdir := filepath.Join(root, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Always mention the repo root."), 0o644); err != nil {
		t.Fatalf("write AGENTS: %v", err)
	}

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(req provider.Request) []provider.Event {
				if !strings.Contains(req.SystemPrompt, "Always mention the repo root.") {
					t.Fatalf("expected system prompt to include AGENTS contents, got %q", req.SystemPrompt)
				}
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("ok"))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, nil
	}

	var out bytes.Buffer
	if err := RunAgent(context.Background(), strings.NewReader(""), &out, "hello", RunOptions{
		WorkingDir: subdir,
		DBPath:     filepath.Join(t.TempDir(), "sessions.db"),
	}); err != nil {
		t.Fatalf("run agent: %v", err)
	}
}
