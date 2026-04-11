// Package helixqa provides an adapter bridging HelixAgent internals to the
// digital.vasic.helixqa extracted module. It exposes autonomous QA pipeline
// management, findings retrieval, and report generation through a simplified
// interface suitable for HelixAgent handlers and services.
package helixqa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"digital.vasic.helixqa/pkg/autonomous"
	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/learning"
	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/memory"
)

// SessionStatus mirrors the pipeline session status for adapter consumers.
type SessionStatus string

const (
	// StatusPending indicates the session has not yet started.
	StatusPending SessionStatus = "pending"
	// StatusRunning indicates the session is actively executing.
	StatusRunning SessionStatus = "running"
	// StatusCompleted indicates the session finished successfully.
	StatusCompleted SessionStatus = "completed"
	// StatusFailed indicates the session terminated with an error.
	StatusFailed SessionStatus = "failed"
)

// SessionResult is the adapter-level representation of an autonomous
// QA session outcome, decoupled from the internal pipeline types.
type SessionResult struct {
	Status         SessionStatus `json:"status"`
	SessionID      string        `json:"session_id"`
	Duration       time.Duration `json:"duration"`
	TestsPlanned   int           `json:"tests_planned"`
	TestsRun       int           `json:"tests_run"`
	IssuesFound    int           `json:"issues_found"`
	TicketsCreated int           `json:"tickets_created"`
	CoveragePct    float64       `json:"coverage_pct"`
	Error          string        `json:"error,omitempty"`
}

// Finding is the adapter-level representation of a QA finding,
// decoupled from the internal memory types.
type Finding struct {
	ID            string `json:"id"`
	SessionID     string `json:"session_id"`
	Severity      string `json:"severity"`
	Category      string `json:"category"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	ReproSteps    string `json:"repro_steps"`
	EvidencePaths string `json:"evidence_paths"`
	Platform      string `json:"platform"`
	Screen        string `json:"screen"`
	Status        string `json:"status"`
	FoundDate     string `json:"found_date"`
}

// SessionConfig holds the parameters for launching an autonomous QA
// session through the adapter.
type SessionConfig struct {
	ProjectRoot      string
	Platforms        []string
	OutputDir        string
	IssuesDir        string
	BanksDir         string
	Timeout          time.Duration
	AndroidDevice    string
	AndroidDevices   []string
	AndroidPackage   string
	WebURL           string
	DesktopDisplay   string
	FFmpegPath       string
	CuriosityEnabled bool
	CuriosityTimeout time.Duration
	VisionHost       string
	VisionUser       string
	VisionModel      string
	UseLlamaCpp      bool
	LlamaCppModel    string
	LlamaCppMMProj   string
	LlamaCppFreeGPU  bool
	MemoryDBPath     string
	LLMProviders     []llm.ProviderConfig
}

// Adapter bridges HelixAgent to the digital.vasic.helixqa module.
// It manages the memory store lifecycle and exposes autonomous QA
// pipeline operations.
type Adapter struct {
	logger  *logrus.Logger
	store   *memory.Store
	storeMu sync.Mutex
	dbPath  string
}

// New creates a new HelixQA adapter with the provided logger.
// If logger is nil, a default logrus logger is used.
func New(logger *logrus.Logger) *Adapter {
	if logger == nil {
		logger = logrus.New()
	}
	return &Adapter{
		logger: logger,
	}
}

// Initialize opens or creates the SQLite memory store at the given
// path. It is safe to call concurrently; subsequent calls are
// no-ops once the store is open. If dbPath is empty,
// "data/memory.db" relative to the working directory is used.
func (a *Adapter) Initialize(dbPath string) error {
	if dbPath == "" {
		dbPath = filepath.Join("data", "memory.db")
	}

	a.storeMu.Lock()
	defer a.storeMu.Unlock()

	if a.store != nil {
		// Already initialized — idempotent no-op.
		return nil
	}

	a.dbPath = dbPath
	store, err := memory.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("helixqa adapter: init store: %w", err)
	}
	a.store = store
	a.logger.WithField("db_path", dbPath).Info(
		"helixqa adapter: memory store initialized")
	return nil
}

// Close releases the memory store connection. Safe to call
// multiple times.
func (a *Adapter) Close() error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store != nil {
		err := a.store.Close()
		a.store = nil
		return err
	}
	return nil
}

// RunAutonomousSession launches a full autonomous QA pipeline with
// the provided configuration. It initializes the LLM provider chain,
// builds the pipeline config, and runs all phases (Learn, Plan,
// Execute, Analyze).
func (a *Adapter) RunAutonomousSession(
	ctx context.Context,
	cfg *SessionConfig,
) (*SessionResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("helixqa adapter: session config is required")
	}

	// Ensure store is initialized.
	dbPath := cfg.MemoryDBPath
	if dbPath == "" {
		dbPath = filepath.Join(cfg.OutputDir, "memory.db")
	}
	if err := a.Initialize(dbPath); err != nil {
		return nil, fmt.Errorf("helixqa adapter: init store: %w", err)
	}

	// Build LLM provider from configs.
	provider, err := a.buildProvider(cfg.LLMProviders)
	if err != nil {
		return nil, fmt.Errorf("helixqa adapter: build provider: %w", err)
	}

	// Convert adapter config to pipeline config.
	pipelineCfg := a.toPipelineConfig(cfg)

	// Run knowledge base credential discovery.
	if cfg.ProjectRoot != "" {
		reader := learning.NewProjectReader(cfg.ProjectRoot)
		creds := reader.ExtractCredentials(cfg.ProjectRoot)
		if len(creds) > 0 && pipelineCfg.QACredentials == nil {
			pipelineCfg.QACredentials = creds
		}
	}

	a.storeMu.Lock()
	store := a.store
	a.storeMu.Unlock()

	// Create and run the pipeline.
	pipeline := autonomous.NewSessionPipeline(pipelineCfg, provider, store)
	result, err := pipeline.Run(ctx)
	if err != nil {
		return &SessionResult{
			Status: StatusFailed,
			Error:  err.Error(),
		}, err
	}

	return a.toSessionResult(result), nil
}

// GetFindings retrieves findings by status from the memory store.
// Valid statuses: "open", "fixed", "verified". Empty status returns
// all open findings.
func (a *Adapter) GetFindings(status string) ([]Finding, error) {
	a.storeMu.Lock()
	store := a.store
	a.storeMu.Unlock()

	if store == nil {
		return nil, fmt.Errorf("helixqa adapter: store not initialized")
	}

	if status == "" {
		status = "open"
	}

	findings, err := store.ListFindingsByStatus(status)
	if err != nil {
		return nil, fmt.Errorf(
			"helixqa adapter: list findings: %w", err)
	}

	result := make([]Finding, len(findings))
	for i, f := range findings {
		result[i] = toAdapterFinding(f)
	}
	return result, nil
}

// GetFinding retrieves a single finding by ID.
func (a *Adapter) GetFinding(id string) (*Finding, error) {
	a.storeMu.Lock()
	store := a.store
	a.storeMu.Unlock()

	if store == nil {
		return nil, fmt.Errorf("helixqa adapter: store not initialized")
	}

	f, err := store.GetFinding(id)
	if err != nil {
		return nil, fmt.Errorf(
			"helixqa adapter: get finding %q: %w", id, err)
	}
	if f == nil {
		return nil, fmt.Errorf(
			"helixqa adapter: finding %q not found", id)
	}

	result := toAdapterFinding(*f)
	return &result, nil
}

// UpdateFindingStatus updates the status of a finding (e.g. from
// "open" to "fixed").
func (a *Adapter) UpdateFindingStatus(id, status string) error {
	a.storeMu.Lock()
	store := a.store
	a.storeMu.Unlock()

	if store == nil {
		return fmt.Errorf("helixqa adapter: store not initialized")
	}

	return store.UpdateFindingStatus(id, status)
}

// DiscoverCredentials reads .env files and project documentation
// from the given project root to extract QA credentials.
func (a *Adapter) DiscoverCredentials(
	projectRoot string,
) (map[string]string, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf(
			"helixqa adapter: project root is required")
	}
	if _, err := os.Stat(projectRoot); err != nil {
		return nil, fmt.Errorf(
			"helixqa adapter: stat project root: %w", err)
	}

	reader := learning.NewProjectReader(projectRoot)
	return reader.ExtractCredentials(projectRoot), nil
}

// DiscoverKnowledge reads project documentation, CLAUDE.md files,
// and extracts constraints from the given project root.
func (a *Adapter) DiscoverKnowledge(
	projectRoot string,
) (*learning.KnowledgeBase, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf(
			"helixqa adapter: project root is required")
	}

	reader := learning.NewProjectReader(projectRoot)
	kb := learning.NewKnowledgeBase()

	docs, err := reader.ReadDocs()
	if err != nil {
		a.logger.WithError(err).Warn(
			"helixqa adapter: read docs failed")
	} else {
		for _, doc := range docs {
			kb.Docs = append(kb.Docs, doc)
		}
	}

	claudeMDs, err := reader.ReadClaudeMDs()
	if err != nil {
		a.logger.WithError(err).Warn(
			"helixqa adapter: read CLAUDE.md files failed")
	} else {
		for _, doc := range claudeMDs {
			kb.Docs = append(kb.Docs, doc)
		}
	}

	constraints := reader.ExtractConstraints(kb.Docs)
	kb.Constraints = constraints
	kb.Credentials = reader.ExtractCredentials(projectRoot)

	return kb, nil
}

// SupportedPlatforms returns the list of platforms supported by
// the HelixQA module.
func (a *Adapter) SupportedPlatforms() []string {
	return []string{
		string(config.PlatformAndroid),
		string(config.PlatformAndroidTV),
		string(config.PlatformWeb),
		string(config.PlatformDesktop),
		string(config.PlatformCLI),
		string(config.PlatformAPI),
	}
}

// buildProvider creates an adaptive LLM provider from the given
// configurations. If no configs are provided, it attempts to
// discover providers from environment variables.
func (a *Adapter) buildProvider(
	configs []llm.ProviderConfig,
) (llm.Provider, error) {
	if len(configs) == 0 {
		// Auto-discover from environment.
		configs = a.discoverProviderConfigs()
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf(
			"helixqa adapter: no LLM provider configs available")
	}

	provider, err := llm.NewAdaptiveFromConfigs(configs)
	if err != nil {
		return nil, fmt.Errorf(
			"helixqa adapter: create adaptive provider: %w", err)
	}

	return provider, nil
}

// discoverProviderConfigs checks environment variables for known
// LLM provider API keys and builds configs for each available
// provider.
func (a *Adapter) discoverProviderConfigs() []llm.ProviderConfig {
	var configs []llm.ProviderConfig

	for name, envKey := range llm.ProviderEnvKeys {
		apiKey := os.Getenv(envKey)
		if apiKey == "" {
			continue
		}
		cfg := llm.ProviderConfig{
			Name:   name,
			APIKey: apiKey,
		}
		configs = append(configs, cfg)
		a.logger.WithFields(logrus.Fields{
			"provider": name,
			"env_key":  envKey,
		}).Debug("helixqa adapter: discovered LLM provider")
	}

	return configs
}

// toPipelineConfig converts adapter SessionConfig to the internal
// PipelineConfig used by the autonomous pipeline.
func (a *Adapter) toPipelineConfig(
	cfg *SessionConfig,
) *autonomous.PipelineConfig {
	return &autonomous.PipelineConfig{
		ProjectRoot:        cfg.ProjectRoot,
		Platforms:          cfg.Platforms,
		OutputDir:          cfg.OutputDir,
		IssuesDir:          cfg.IssuesDir,
		BanksDir:           cfg.BanksDir,
		Timeout:            cfg.Timeout,
		AndroidDevice:      cfg.AndroidDevice,
		AndroidDevices:     cfg.AndroidDevices,
		AndroidPackage:     cfg.AndroidPackage,
		WebURL:             cfg.WebURL,
		DesktopDisplay:     cfg.DesktopDisplay,
		FFmpegPath:         cfg.FFmpegPath,
		CuriosityEnabled:   cfg.CuriosityEnabled,
		CuriosityTimeout:   cfg.CuriosityTimeout,
		VisionHost:         cfg.VisionHost,
		VisionUser:         cfg.VisionUser,
		VisionModel:        cfg.VisionModel,
		UseLlamaCpp:        cfg.UseLlamaCpp,
		LlamaCppModelPath:  cfg.LlamaCppModel,
		LlamaCppMMProjPath: cfg.LlamaCppMMProj,
		LlamaCppFreeGPU:    cfg.LlamaCppFreeGPU,
	}
}

// toSessionResult converts an internal PipelineResult to the
// adapter-level SessionResult.
func (a *Adapter) toSessionResult(
	r *autonomous.PipelineResult,
) *SessionResult {
	if r == nil {
		return &SessionResult{Status: StatusFailed}
	}
	status := StatusCompleted
	if r.Error != "" {
		status = StatusFailed
	}
	return &SessionResult{
		Status:         status,
		SessionID:      r.SessionID,
		Duration:       r.Duration,
		TestsPlanned:   r.TestsPlanned,
		TestsRun:       r.TestsRun,
		IssuesFound:    r.IssuesFound,
		TicketsCreated: r.TicketsCreated,
		CoveragePct:    r.CoveragePct,
		Error:          r.Error,
	}
}

// toAdapterFinding converts an internal memory.Finding to the
// adapter-level Finding type.
func toAdapterFinding(f memory.Finding) Finding {
	return Finding{
		ID:            f.ID,
		SessionID:     f.SessionID,
		Severity:      f.Severity,
		Category:      f.Category,
		Title:         f.Title,
		Description:   f.Description,
		ReproSteps:    f.ReproSteps,
		EvidencePaths: f.EvidencePaths,
		Platform:      f.Platform,
		Screen:        f.Screen,
		Status:        f.Status,
		FoundDate:     f.FoundDate,
	}
}
