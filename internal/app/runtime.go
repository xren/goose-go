package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"goose-go/internal/agent"
	"goose-go/internal/compaction"
	"goose-go/internal/provider"
	"goose-go/internal/provider/openaicodex"
	"goose-go/internal/session"
	sqlitestore "goose-go/internal/storage/sqlite"
	"goose-go/internal/tools"
	"goose-go/internal/tools/shell"
)

type Runtime struct {
	store      storeCloser
	agent      *agent.Agent
	workingDir string
	traceDir   string
	provider   string
	model      string
}

const (
	defaultProviderName = "openai-codex"
	defaultModelName    = "gpt-5-codex"
)

var newRunProvider = func(debugOut io.Writer) (provider.Provider, error) {
	if debugOut != nil {
		return openaicodex.New(openaicodex.WithDebugWriter(debugOut))
	}
	return openaicodex.New()
}

var openRunStore = func(path string) (storeCloser, error) {
	return sqlitestore.Open(path)
}

func OpenRuntime(in io.Reader, out io.Writer, opts RunOptions) (*Runtime, error) {
	workingDir, err := resolveWorkingDir(opts.WorkingDir)
	if err != nil {
		return nil, err
	}
	if opts.MaxTurns <= 0 {
		opts.MaxTurns = 8
	}

	dbPath := resolveDBPath(workingDir, opts.DBPath)

	store, err := openRunStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open session store: %w", err)
	}

	var debugOut io.Writer
	if opts.DebugProvider {
		debugOut = out
	}
	p, err := newRunProvider(debugOut)
	if err != nil {
		_ = store.Close()
		return nil, diagnoseRunError(defaultProviderName, fmt.Errorf("create %s provider: %w", defaultProviderName, err), opts.DebugProvider)
	}

	registry := tools.NewRegistry()
	if err := registry.Register(shell.New()); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("register shell tool: %w", err)
	}

	approvalMode := agent.ApprovalModeAuto
	if opts.RequireApproval {
		approvalMode = agent.ApprovalModeApprove
	}
	var approver agent.Approver
	if opts.Approve {
		approver = interactiveApprover{in: in, out: out}
	}

	runtime, err := agent.New(store, p, registry, agent.Config{
		SystemPrompt: defaultRunSystemPrompt,
		Model: provider.ModelConfig{
			Provider:      defaultProviderName,
			Model:         defaultModelName,
			ContextWindow: openAICodexContextWindow,
		},
		Compaction:   compaction.DefaultSettings(),
		MaxTurns:     opts.MaxTurns,
		ApprovalMode: approvalMode,
	}, approver)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("create agent runtime: %w", err)
	}

	traceDir := opts.TraceDir
	if traceDir == "" {
		traceDir = filepath.Join(filepath.Dir(dbPath), "traces")
	}

	return &Runtime{
		store:      store,
		agent:      runtime,
		workingDir: workingDir,
		traceDir:   traceDir,
		provider:   defaultProviderName,
		model:      defaultModelName,
	}, nil
}

func (r *Runtime) Close() error {
	if r == nil || r.store == nil {
		return nil
	}
	return r.store.Close()
}

func (r *Runtime) LoadOrCreateSession(ctx context.Context, prompt string, sessionID string) (session.Session, int, error) {
	return loadOrCreateSession(ctx, r.store, prompt, r.workingDir, sessionID)
}

func (r *Runtime) ReplayConversation(ctx context.Context, sessionID string) (session.Session, error) {
	return r.store.GetSession(ctx, sessionID)
}

func (r *Runtime) ListSessions(ctx context.Context) ([]session.Summary, error) {
	return r.store.ListSessions(ctx)
}

func (r *Runtime) OpenTraceWriter(sessionID string) (EventRecorder, error) {
	return openTraceWriter(r.traceDir, sessionID)
}

func (r *Runtime) Agent() *agent.Agent {
	return r.agent
}

func (r *Runtime) WorkingDir() string {
	return r.workingDir
}

func (r *Runtime) ProviderModel() (string, string) {
	return r.provider, r.model
}

func (r *Runtime) ReplyStream(ctx context.Context, sessionID string, prompt string) (<-chan agent.Event, error) {
	return r.agent.ReplyStream(ctx, sessionID, prompt)
}

func (r *Runtime) PendingApproval(ctx context.Context, sessionID string) (*agent.ApprovalRequest, error) {
	return r.agent.PendingApproval(ctx, sessionID)
}

func (r *Runtime) ResolveApproval(ctx context.Context, sessionID string, decision agent.ApprovalDecision) (agent.Result, error) {
	return r.agent.ResolveApproval(ctx, sessionID, decision)
}

func (r *Runtime) ResolveApprovalStream(ctx context.Context, sessionID string, decision agent.ApprovalDecision) (<-chan agent.Event, error) {
	return r.agent.ResolveApprovalStream(ctx, sessionID, decision)
}

const openAICodexContextWindow = 128000

func resolveDBPath(workingDir string, dbPath string) string {
	if dbPath != "" {
		return dbPath
	}
	return filepath.Join(workingDir, defaultRunDBDir, defaultRunDBName)
}
