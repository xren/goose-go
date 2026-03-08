package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"goose-go/internal/models"

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

const defaultProviderName = "openai-codex"
const defaultMaxTurns = 10000

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
		opts.MaxTurns = defaultMaxTurns
	}
	selection, err := resolveRuntimeSelection(opts)
	if err != nil {
		return nil, err
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
	p, err := openProvider(selection.Provider, debugOut)
	if err != nil {
		_ = store.Close()
		return nil, diagnoseRunError(selection.Provider, fmt.Errorf("create %s provider: %w", selection.Provider, err), opts.DebugProvider)
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
		Model:        selection.ModelConfig,
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
		provider:   selection.Provider,
		model:      selection.Model,
	}, nil
}

func (r *Runtime) Close() error {
	if r == nil || r.store == nil {
		return nil
	}
	return r.store.Close()
}

func (r *Runtime) LoadOrCreateSession(ctx context.Context, prompt string, sessionID string) (session.Session, int, error) {
	record, count, err := loadOrCreateSession(ctx, r.store, prompt, r.workingDir, r.provider, r.model, sessionID)
	if err != nil {
		return session.Session{}, 0, err
	}
	if sessionID != "" && record.Provider != "" && record.Model != "" {
		r.applySessionSelection(record.Provider, record.Model)
	}
	return record, count, nil
}

func (r *Runtime) ReplayConversation(ctx context.Context, sessionID string) (session.Session, error) {
	record, err := r.store.GetSession(ctx, sessionID)
	if err != nil {
		return session.Session{}, err
	}
	if record.Provider != "" && record.Model != "" {
		r.applySessionSelection(record.Provider, record.Model)
	}
	return record, nil
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

func resolveDBPath(workingDir string, dbPath string) string {
	if dbPath != "" {
		return dbPath
	}
	return filepath.Join(workingDir, defaultRunDBDir, defaultRunDBName)
}

type runtimeSelection struct {
	Provider    string
	Model       string
	ModelConfig provider.ModelConfig
}

func resolveRuntimeSelection(opts RunOptions) (runtimeSelection, error) {
	providerID := opts.Provider
	if providerID == "" {
		providerID = defaultProviderName
	}
	_, modelSpec, err := models.ResolveSelection(providerID, opts.Model)
	if err != nil {
		return runtimeSelection{}, err
	}
	return runtimeSelection{
		Provider:    providerID,
		Model:       string(modelSpec.ID),
		ModelConfig: models.ToModelConfig(modelSpec),
	}, nil
}

func openProvider(providerID string, debugOut io.Writer) (provider.Provider, error) {
	switch providerID {
	case string(models.ProviderOpenAICodex):
		return newRunProvider(debugOut)
	default:
		return nil, fmt.Errorf("unsupported provider %q", providerID)
	}
}

func (r *Runtime) Selection() (string, string) {
	return r.provider, r.model
}

func (r *Runtime) applySessionSelection(providerName string, modelName string) {
	_, modelSpec, err := models.ResolveSelection(providerName, modelName)
	if err != nil {
		return
	}
	r.provider = providerName
	r.model = string(modelSpec.ID)
	if r.agent != nil {
		r.agent.SetModelConfig(models.ToModelConfig(modelSpec))
	}
}

func (r *Runtime) ListAvailableModels(ctx context.Context) ([]models.Availability, error) {
	return models.NewResolver().ListAvailable(ctx, r.provider)
}

func (r *Runtime) SetSelection(ctx context.Context, providerName string, modelName string, sessionID string) error {
	_, modelSpec, err := models.ResolveSelection(providerName, modelName)
	if err != nil {
		return err
	}
	if sessionID != "" {
		record, err := r.store.UpdateSessionSelection(ctx, sessionID, providerName, string(modelSpec.ID))
		if err != nil {
			return err
		}
		r.applySessionSelection(record.Provider, record.Model)
		return nil
	}
	r.applySessionSelection(providerName, string(modelSpec.ID))
	return nil
}
