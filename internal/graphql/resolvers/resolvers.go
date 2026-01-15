// Package resolvers provides GraphQL resolvers that connect to HelixAgent services.
package resolvers

import (
	"context"
	"sync"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/background"
	gqltypes "dev.helix.agent/internal/graphql/types"
	"dev.helix.agent/internal/models"
)

// ServiceRegistry provides access to HelixAgent services for resolvers.
type ServiceRegistry interface {
	// GetProviders returns verified providers
	GetProviders() []ProviderInfo
	// GetProvider returns a provider by ID
	GetProvider(id string) (*ProviderInfo, error)
	// GetProviderScores returns provider scores
	GetProviderScores() []ProviderScoreInfo
	// GetVerificationResults returns verification results
	GetVerificationResults() *VerificationInfo
	// RefreshProvider refreshes a provider
	RefreshProvider(ctx context.Context, id string) (*ProviderInfo, error)
}

// DebateServiceInterface provides debate operations.
type DebateServiceInterface interface {
	// GetDebates returns all debates
	GetDebates(filter *gqltypes.DebateFilter) []gqltypes.Debate
	// GetDebate returns a debate by ID
	GetDebate(id string) (*gqltypes.Debate, error)
	// CreateDebate creates a new debate
	CreateDebate(ctx context.Context, input *gqltypes.CreateDebateInput) (*gqltypes.Debate, error)
	// SubmitResponse submits a debate response
	SubmitResponse(ctx context.Context, input *gqltypes.DebateResponseInput) (*gqltypes.DebateRound, error)
}

// TaskServiceInterface provides task operations.
type TaskServiceInterface interface {
	// GetTasks returns all tasks
	GetTasks(filter *gqltypes.TaskFilter) []gqltypes.Task
	// GetTask returns a task by ID
	GetTask(id string) (*gqltypes.Task, error)
	// CreateTask creates a new task
	CreateTask(ctx context.Context, input *gqltypes.CreateTaskInput) (*gqltypes.Task, error)
	// CancelTask cancels a task
	CancelTask(ctx context.Context, id string) (*gqltypes.Task, error)
}

// ProviderInfo represents provider information for GraphQL.
type ProviderInfo struct {
	ID           string
	Name         string
	Type         string
	Status       string
	Score        float64
	Models       []ModelInfo
	HealthStatus *HealthStatusInfo
	Capabilities *CapabilitiesInfo
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ModelInfo represents model information.
type ModelInfo struct {
	ID                string
	Name              string
	ProviderID        string
	Version           string
	ContextWindow     int
	MaxTokens         int
	SupportsTools     bool
	SupportsVision    bool
	SupportsStreaming bool
	Score             float64
	Rank              int
	CreatedAt         time.Time
}

// HealthStatusInfo represents health status.
type HealthStatusInfo struct {
	Status       string
	LatencyMs    int64
	LastCheck    time.Time
	ErrorMessage string
}

// CapabilitiesInfo represents capabilities.
type CapabilitiesInfo struct {
	Chat            bool
	Completions     bool
	Embeddings      bool
	Vision          bool
	ToolUse         bool
	Streaming       bool
	FunctionCalling bool
}

// ProviderScoreInfo represents provider score breakdown.
type ProviderScoreInfo struct {
	ProviderID        string
	ProviderName      string
	OverallScore      float64
	ResponseSpeed     float64
	ModelEfficiency   float64
	CostEffectiveness float64
	Capability        float64
	Recency           float64
}

// VerificationInfo represents verification results.
type VerificationInfo struct {
	TotalProviders    int
	VerifiedProviders int
	TotalModels       int
	VerifiedModels    int
	OverallScore      float64
	LastVerified      time.Time
}

// ResolverContext holds resolver dependencies.
type ResolverContext struct {
	Services     ServiceRegistry
	DebateSvc    DebateServiceInterface
	TaskSvc      TaskServiceInterface
	TaskRepo     background.TaskRepository
	Logger       *logrus.Logger
	mu           sync.RWMutex
}

// NewResolverContext creates a new resolver context.
func NewResolverContext(logger *logrus.Logger) *ResolverContext {
	if logger == nil {
		logger = logrus.New()
	}
	return &ResolverContext{
		Logger: logger,
	}
}

// SetServices sets the service registry.
func (rc *ResolverContext) SetServices(svc ServiceRegistry) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.Services = svc
}

// SetDebateService sets the debate service.
func (rc *ResolverContext) SetDebateService(svc DebateServiceInterface) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.DebateSvc = svc
}

// SetTaskService sets the task service.
func (rc *ResolverContext) SetTaskService(svc TaskServiceInterface) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.TaskSvc = svc
}

// SetTaskRepository sets the task repository.
func (rc *ResolverContext) SetTaskRepository(repo background.TaskRepository) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.TaskRepo = repo
}

// Global resolver context (set at startup)
var globalContext *ResolverContext
var contextMu sync.RWMutex

// SetGlobalContext sets the global resolver context.
func SetGlobalContext(ctx *ResolverContext) {
	contextMu.Lock()
	defer contextMu.Unlock()
	globalContext = ctx
}

// GetGlobalContext returns the global resolver context.
func GetGlobalContext() *ResolverContext {
	contextMu.RLock()
	defer contextMu.RUnlock()
	return globalContext
}

// ResolveProviders resolves the providers query.
func ResolveProviders(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.Services == nil {
		return []gqltypes.Provider{}, nil
	}

	providers := ctx.Services.GetProviders()
	result := make([]gqltypes.Provider, 0, len(providers))

	// Apply filters if provided
	filter := extractProviderFilter(p.Args)

	for _, prov := range providers {
		if filter != nil {
			if filter.Status != nil && prov.Status != *filter.Status {
				continue
			}
			if filter.Type != nil && prov.Type != *filter.Type {
				continue
			}
			if filter.MinScore != nil && prov.Score < *filter.MinScore {
				continue
			}
			if filter.MaxScore != nil && prov.Score > *filter.MaxScore {
				continue
			}
		}

		result = append(result, convertToGQLProvider(prov))
	}

	return result, nil
}

// ResolveProvider resolves the provider query.
func ResolveProvider(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.Services == nil {
		return nil, nil
	}

	id, ok := p.Args["id"].(string)
	if !ok {
		return nil, nil
	}

	provider, err := ctx.Services.GetProvider(id)
	if err != nil || provider == nil {
		return nil, err
	}

	result := convertToGQLProvider(*provider)
	return &result, nil
}

// ResolveDebates resolves the debates query.
func ResolveDebates(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.DebateSvc == nil {
		return []gqltypes.Debate{}, nil
	}

	filter := extractDebateFilter(p.Args)
	return ctx.DebateSvc.GetDebates(filter), nil
}

// ResolveDebate resolves the debate query.
func ResolveDebate(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.DebateSvc == nil {
		return nil, nil
	}

	id, ok := p.Args["id"].(string)
	if !ok {
		return nil, nil
	}

	return ctx.DebateSvc.GetDebate(id)
}

// ResolveTasks resolves the tasks query.
func ResolveTasks(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.TaskSvc == nil {
		// Fallback to repository if service not available
		if ctx != nil && ctx.TaskRepo != nil {
			return resolveTasksFromRepo(p, ctx.TaskRepo)
		}
		return []gqltypes.Task{}, nil
	}

	filter := extractTaskFilter(p.Args)
	return ctx.TaskSvc.GetTasks(filter), nil
}

// ResolveTask resolves the task query.
func ResolveTask(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.TaskSvc == nil {
		// Fallback to repository
		if ctx != nil && ctx.TaskRepo != nil {
			return resolveTaskFromRepo(p, ctx.TaskRepo)
		}
		return nil, nil
	}

	id, ok := p.Args["id"].(string)
	if !ok {
		return nil, nil
	}

	return ctx.TaskSvc.GetTask(id)
}

// ResolveVerificationResults resolves the verificationResults query.
func ResolveVerificationResults(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.Services == nil {
		return &gqltypes.VerificationResults{}, nil
	}

	info := ctx.Services.GetVerificationResults()
	if info == nil {
		return &gqltypes.VerificationResults{}, nil
	}

	return &gqltypes.VerificationResults{
		TotalProviders:    info.TotalProviders,
		VerifiedProviders: info.VerifiedProviders,
		TotalModels:       info.TotalModels,
		VerifiedModels:    info.VerifiedModels,
		OverallScore:      info.OverallScore,
		LastVerified:      info.LastVerified,
	}, nil
}

// ResolveProviderScores resolves the providerScores query.
func ResolveProviderScores(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.Services == nil {
		return []gqltypes.ProviderScore{}, nil
	}

	scores := ctx.Services.GetProviderScores()
	result := make([]gqltypes.ProviderScore, 0, len(scores))

	for _, score := range scores {
		result = append(result, gqltypes.ProviderScore{
			ProviderID:        score.ProviderID,
			ProviderName:      score.ProviderName,
			OverallScore:      score.OverallScore,
			ResponseSpeed:     score.ResponseSpeed,
			ModelEfficiency:   score.ModelEfficiency,
			CostEffectiveness: score.CostEffectiveness,
			Capability:        score.Capability,
			Recency:           score.Recency,
		})
	}

	return result, nil
}

// ResolveCreateDebate resolves the createDebate mutation.
func ResolveCreateDebate(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.DebateSvc == nil {
		return nil, nil
	}

	input := extractCreateDebateInput(p.Args)
	if input == nil {
		return nil, nil
	}

	return ctx.DebateSvc.CreateDebate(p.Context, input)
}

// ResolveSubmitDebateResponse resolves the submitDebateResponse mutation.
func ResolveSubmitDebateResponse(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.DebateSvc == nil {
		return nil, nil
	}

	input := extractDebateResponseInput(p.Args)
	if input == nil {
		return nil, nil
	}

	return ctx.DebateSvc.SubmitResponse(p.Context, input)
}

// ResolveCreateTask resolves the createTask mutation.
func ResolveCreateTask(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.TaskSvc == nil {
		return nil, nil
	}

	input := extractCreateTaskInput(p.Args)
	if input == nil {
		return nil, nil
	}

	return ctx.TaskSvc.CreateTask(p.Context, input)
}

// ResolveCancelTask resolves the cancelTask mutation.
func ResolveCancelTask(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.TaskSvc == nil {
		return nil, nil
	}

	id, ok := p.Args["id"].(string)
	if !ok {
		return nil, nil
	}

	return ctx.TaskSvc.CancelTask(p.Context, id)
}

// ResolveRefreshProvider resolves the refreshProvider mutation.
func ResolveRefreshProvider(p graphql.ResolveParams) (interface{}, error) {
	ctx := GetGlobalContext()
	if ctx == nil || ctx.Services == nil {
		return nil, nil
	}

	id, ok := p.Args["id"].(string)
	if !ok {
		return nil, nil
	}

	provider, err := ctx.Services.RefreshProvider(p.Context, id)
	if err != nil || provider == nil {
		return nil, err
	}

	result := convertToGQLProvider(*provider)
	return &result, nil
}

// Helper functions

func extractProviderFilter(args map[string]interface{}) *gqltypes.ProviderFilter {
	filterArg, ok := args["filter"]
	if !ok || filterArg == nil {
		return nil
	}

	filterMap, ok := filterArg.(map[string]interface{})
	if !ok {
		return nil
	}

	filter := &gqltypes.ProviderFilter{}
	if status, ok := filterMap["status"].(string); ok {
		filter.Status = &status
	}
	if typ, ok := filterMap["type"].(string); ok {
		filter.Type = &typ
	}
	if minScore, ok := filterMap["min_score"].(float64); ok {
		filter.MinScore = &minScore
	}
	if maxScore, ok := filterMap["max_score"].(float64); ok {
		filter.MaxScore = &maxScore
	}

	return filter
}

func extractDebateFilter(args map[string]interface{}) *gqltypes.DebateFilter {
	filterArg, ok := args["filter"]
	if !ok || filterArg == nil {
		return nil
	}

	filterMap, ok := filterArg.(map[string]interface{})
	if !ok {
		return nil
	}

	filter := &gqltypes.DebateFilter{}
	if status, ok := filterMap["status"].(string); ok {
		filter.Status = &status
	}

	return filter
}

func extractTaskFilter(args map[string]interface{}) *gqltypes.TaskFilter {
	filterArg, ok := args["filter"]
	if !ok || filterArg == nil {
		return nil
	}

	filterMap, ok := filterArg.(map[string]interface{})
	if !ok {
		return nil
	}

	filter := &gqltypes.TaskFilter{}
	if status, ok := filterMap["status"].(string); ok {
		filter.Status = &status
	}
	if typ, ok := filterMap["type"].(string); ok {
		filter.Type = &typ
	}

	return filter
}

func extractCreateDebateInput(args map[string]interface{}) *gqltypes.CreateDebateInput {
	inputArg, ok := args["input"]
	if !ok || inputArg == nil {
		return nil
	}

	inputMap, ok := inputArg.(map[string]interface{})
	if !ok {
		return nil
	}

	input := &gqltypes.CreateDebateInput{}
	if topic, ok := inputMap["topic"].(string); ok {
		input.Topic = topic
	}
	if participants, ok := inputMap["participants"].([]interface{}); ok {
		for _, p := range participants {
			if s, ok := p.(string); ok {
				input.Participants = append(input.Participants, s)
			}
		}
	}
	if roundCount, ok := inputMap["round_count"].(int); ok {
		input.RoundCount = roundCount
	}

	return input
}

func extractDebateResponseInput(args map[string]interface{}) *gqltypes.DebateResponseInput {
	inputArg, ok := args["input"]
	if !ok || inputArg == nil {
		return nil
	}

	inputMap, ok := inputArg.(map[string]interface{})
	if !ok {
		return nil
	}

	input := &gqltypes.DebateResponseInput{}
	if debateID, ok := inputMap["debate_id"].(string); ok {
		input.DebateID = debateID
	}
	if participantID, ok := inputMap["participant_id"].(string); ok {
		input.ParticipantID = participantID
	}
	if content, ok := inputMap["content"].(string); ok {
		input.Content = content
	}

	return input
}

func extractCreateTaskInput(args map[string]interface{}) *gqltypes.CreateTaskInput {
	inputArg, ok := args["input"]
	if !ok || inputArg == nil {
		return nil
	}

	inputMap, ok := inputArg.(map[string]interface{})
	if !ok {
		return nil
	}

	input := &gqltypes.CreateTaskInput{}
	if typ, ok := inputMap["type"].(string); ok {
		input.Type = typ
	}
	if priority, ok := inputMap["priority"].(int); ok {
		input.Priority = priority
	}

	return input
}

func convertToGQLProvider(p ProviderInfo) gqltypes.Provider {
	provider := gqltypes.Provider{
		ID:        p.ID,
		Name:      p.Name,
		Type:      p.Type,
		Status:    p.Status,
		Score:     p.Score,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}

	// Convert models
	for _, m := range p.Models {
		provider.Models = append(provider.Models, gqltypes.Model{
			ID:                m.ID,
			Name:              m.Name,
			ProviderID:        m.ProviderID,
			Version:           m.Version,
			ContextWindow:     m.ContextWindow,
			MaxTokens:         m.MaxTokens,
			SupportsTools:     m.SupportsTools,
			SupportsVision:    m.SupportsVision,
			SupportsStreaming: m.SupportsStreaming,
			Score:             m.Score,
			Rank:              m.Rank,
			CreatedAt:         m.CreatedAt,
		})
	}

	// Convert health status
	if p.HealthStatus != nil {
		provider.HealthStatus = &gqltypes.HealthStatus{
			Status:       p.HealthStatus.Status,
			Latency:      p.HealthStatus.LatencyMs,
			LastCheck:    p.HealthStatus.LastCheck,
			ErrorMessage: p.HealthStatus.ErrorMessage,
		}
	}

	// Convert capabilities
	if p.Capabilities != nil {
		provider.Capabilities = &gqltypes.Capabilities{
			Chat:            p.Capabilities.Chat,
			Completions:     p.Capabilities.Completions,
			Embeddings:      p.Capabilities.Embeddings,
			Vision:          p.Capabilities.Vision,
			ToolUse:         p.Capabilities.ToolUse,
			Streaming:       p.Capabilities.Streaming,
			FunctionCalling: p.Capabilities.FunctionCalling,
		}
	}

	return provider
}

func resolveTasksFromRepo(p graphql.ResolveParams, repo background.TaskRepository) (interface{}, error) {
	filter := extractTaskFilter(p.Args)

	var tasks []*models.BackgroundTask
	var err error

	if filter != nil && filter.Status != nil {
		status := models.TaskStatus(*filter.Status)
		tasks, err = repo.GetByStatus(p.Context, status, 100, 0)
	} else {
		tasks, err = repo.GetPendingTasks(p.Context, 100)
	}

	if err != nil {
		return nil, err
	}

	result := make([]gqltypes.Task, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, convertToGQLTask(t))
	}

	return result, nil
}

func resolveTaskFromRepo(p graphql.ResolveParams, repo background.TaskRepository) (interface{}, error) {
	id, ok := p.Args["id"].(string)
	if !ok {
		return nil, nil
	}

	task, err := repo.GetByID(p.Context, id)
	if err != nil || task == nil {
		return nil, err
	}

	result := convertToGQLTask(task)
	return &result, nil
}

func convertToGQLTask(t *models.BackgroundTask) gqltypes.Task {
	task := gqltypes.Task{
		ID:        t.ID,
		Type:      t.TaskType,
		Status:    string(t.Status),
		Priority:  priorityToInt(t.Priority),
		Progress:  int(t.Progress),
		CreatedAt: t.CreatedAt,
	}

	if t.LastError != nil && *t.LastError != "" {
		task.Error = *t.LastError
	}
	if t.StartedAt != nil && !t.StartedAt.IsZero() {
		task.StartedAt = t.StartedAt
	}
	if t.CompletedAt != nil && !t.CompletedAt.IsZero() {
		task.CompletedAt = t.CompletedAt
	}

	return task
}

// priorityToInt converts TaskPriority to an integer for GraphQL.
func priorityToInt(p models.TaskPriority) int {
	switch p {
	case models.TaskPriorityCritical:
		return 0
	case models.TaskPriorityHigh:
		return 1
	case models.TaskPriorityNormal:
		return 2
	case models.TaskPriorityLow:
		return 3
	case models.TaskPriorityBackground:
		return 4
	default:
		return 2 // Default to normal
	}
}
