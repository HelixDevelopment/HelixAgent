package resolvers

import (
	"context"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gqltypes "dev.helix.agent/internal/graphql/types"
)

// MockServiceRegistry implements ServiceRegistry for testing.
type MockServiceRegistry struct {
	providers           []ProviderInfo
	providerScores      []ProviderScoreInfo
	verificationResults *VerificationInfo
}

func (m *MockServiceRegistry) GetProviders() []ProviderInfo {
	return m.providers
}

func (m *MockServiceRegistry) GetProvider(id string) (*ProviderInfo, error) {
	for _, p := range m.providers {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, nil
}

func (m *MockServiceRegistry) GetProviderScores() []ProviderScoreInfo {
	return m.providerScores
}

func (m *MockServiceRegistry) GetVerificationResults() *VerificationInfo {
	return m.verificationResults
}

func (m *MockServiceRegistry) RefreshProvider(ctx context.Context, id string) (*ProviderInfo, error) {
	for _, p := range m.providers {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, nil
}

// MockDebateService implements DebateServiceInterface for testing.
type MockDebateService struct {
	debates []gqltypes.Debate
}

func (m *MockDebateService) GetDebates(filter *gqltypes.DebateFilter) []gqltypes.Debate {
	if filter != nil && filter.Status != nil {
		var filtered []gqltypes.Debate
		for _, d := range m.debates {
			if d.Status == *filter.Status {
				filtered = append(filtered, d)
			}
		}
		return filtered
	}
	return m.debates
}

func (m *MockDebateService) GetDebate(id string) (*gqltypes.Debate, error) {
	for _, d := range m.debates {
		if d.ID == id {
			return &d, nil
		}
	}
	return nil, nil
}

func (m *MockDebateService) CreateDebate(ctx context.Context, input *gqltypes.CreateDebateInput) (*gqltypes.Debate, error) {
	debate := &gqltypes.Debate{
		ID:        "new-debate-id",
		Topic:     input.Topic,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.debates = append(m.debates, *debate)
	return debate, nil
}

func (m *MockDebateService) SubmitResponse(ctx context.Context, input *gqltypes.DebateResponseInput) (*gqltypes.DebateRound, error) {
	return &gqltypes.DebateRound{
		ID:          "new-round-id",
		DebateID:    input.DebateID,
		RoundNumber: 1,
		CreatedAt:   time.Now(),
	}, nil
}

// MockTaskService implements TaskServiceInterface for testing.
type MockTaskService struct {
	tasks []gqltypes.Task
}

func (m *MockTaskService) GetTasks(filter *gqltypes.TaskFilter) []gqltypes.Task {
	if filter != nil && filter.Status != nil {
		var filtered []gqltypes.Task
		for _, t := range m.tasks {
			if t.Status == *filter.Status {
				filtered = append(filtered, t)
			}
		}
		return filtered
	}
	return m.tasks
}

func (m *MockTaskService) GetTask(id string) (*gqltypes.Task, error) {
	for _, t := range m.tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, nil
}

func (m *MockTaskService) CreateTask(ctx context.Context, input *gqltypes.CreateTaskInput) (*gqltypes.Task, error) {
	task := &gqltypes.Task{
		ID:        "new-task-id",
		Type:      input.Type,
		Status:    "pending",
		Priority:  input.Priority,
		CreatedAt: time.Now(),
	}
	m.tasks = append(m.tasks, *task)
	return task, nil
}

func (m *MockTaskService) CancelTask(ctx context.Context, id string) (*gqltypes.Task, error) {
	for i, t := range m.tasks {
		if t.ID == id {
			m.tasks[i].Status = "cancelled"
			return &m.tasks[i], nil
		}
	}
	return nil, nil
}

func TestNewResolverContext(t *testing.T) {
	ctx := NewResolverContext(nil)
	require.NotNil(t, ctx)
	assert.NotNil(t, ctx.Logger)
}

func TestNewResolverContext_WithLogger(t *testing.T) {
	logger := logrus.New()
	ctx := NewResolverContext(logger)
	require.NotNil(t, ctx)
	assert.Equal(t, logger, ctx.Logger)
}

func TestSetGlobalContext(t *testing.T) {
	ctx := NewResolverContext(nil)
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	retrieved := GetGlobalContext()
	assert.Equal(t, ctx, retrieved)
}

func TestResolveProviders_NoContext(t *testing.T) {
	SetGlobalContext(nil)

	params := graphql.ResolveParams{}
	result, err := ResolveProviders(params)

	assert.NoError(t, err)
	assert.IsType(t, []gqltypes.Provider{}, result)
	assert.Empty(t, result)
}

func TestResolveProviders_WithServices(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.Services = &MockServiceRegistry{
		providers: []ProviderInfo{
			{ID: "provider-1", Name: "Test Provider 1", Status: "active", Score: 8.5},
			{ID: "provider-2", Name: "Test Provider 2", Status: "active", Score: 7.0},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{Args: map[string]interface{}{}}
	result, err := ResolveProviders(params)

	assert.NoError(t, err)
	providers, ok := result.([]gqltypes.Provider)
	require.True(t, ok)
	assert.Len(t, providers, 2)
	assert.Equal(t, "provider-1", providers[0].ID)
	assert.Equal(t, "provider-2", providers[1].ID)
}

func TestResolveProviders_WithFilter(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.Services = &MockServiceRegistry{
		providers: []ProviderInfo{
			{ID: "provider-1", Name: "Test Provider 1", Status: "active", Type: "api_key", Score: 8.5},
			{ID: "provider-2", Name: "Test Provider 2", Status: "inactive", Type: "oauth", Score: 7.0},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"filter": map[string]interface{}{
				"status": "active",
			},
		},
	}
	result, err := ResolveProviders(params)

	assert.NoError(t, err)
	providers, ok := result.([]gqltypes.Provider)
	require.True(t, ok)
	assert.Len(t, providers, 1)
	assert.Equal(t, "provider-1", providers[0].ID)
}

func TestResolveProviders_WithScoreFilter(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.Services = &MockServiceRegistry{
		providers: []ProviderInfo{
			{ID: "provider-1", Name: "Test Provider 1", Score: 8.5},
			{ID: "provider-2", Name: "Test Provider 2", Score: 7.0},
			{ID: "provider-3", Name: "Test Provider 3", Score: 6.0},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"filter": map[string]interface{}{
				"min_score": 7.0,
			},
		},
	}
	result, err := ResolveProviders(params)

	assert.NoError(t, err)
	providers, ok := result.([]gqltypes.Provider)
	require.True(t, ok)
	assert.Len(t, providers, 2)
}

func TestResolveProvider_NoContext(t *testing.T) {
	SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Args: map[string]interface{}{"id": "test-id"},
	}
	result, err := ResolveProvider(params)

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveProvider_Found(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.Services = &MockServiceRegistry{
		providers: []ProviderInfo{
			{ID: "provider-1", Name: "Test Provider", Score: 8.5},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Args: map[string]interface{}{"id": "provider-1"},
	}
	result, err := ResolveProvider(params)

	assert.NoError(t, err)
	provider, ok := result.(*gqltypes.Provider)
	require.True(t, ok)
	require.NotNil(t, provider)
	assert.Equal(t, "provider-1", provider.ID)
}

func TestResolveProvider_NotFound(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.Services = &MockServiceRegistry{
		providers: []ProviderInfo{},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Args: map[string]interface{}{"id": "nonexistent"},
	}
	result, err := ResolveProvider(params)

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveDebates_NoContext(t *testing.T) {
	SetGlobalContext(nil)

	params := graphql.ResolveParams{}
	result, err := ResolveDebates(params)

	assert.NoError(t, err)
	assert.IsType(t, []gqltypes.Debate{}, result)
}

func TestResolveDebates_WithService(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.DebateSvc = &MockDebateService{
		debates: []gqltypes.Debate{
			{ID: "debate-1", Topic: "Test Topic", Status: "running"},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{Args: map[string]interface{}{}}
	result, err := ResolveDebates(params)

	assert.NoError(t, err)
	debates, ok := result.([]gqltypes.Debate)
	require.True(t, ok)
	assert.Len(t, debates, 1)
}

func TestResolveDebate_Found(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.DebateSvc = &MockDebateService{
		debates: []gqltypes.Debate{
			{ID: "debate-1", Topic: "Test Topic"},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Args: map[string]interface{}{"id": "debate-1"},
	}
	result, err := ResolveDebate(params)

	assert.NoError(t, err)
	debate, ok := result.(*gqltypes.Debate)
	require.True(t, ok)
	require.NotNil(t, debate)
	assert.Equal(t, "debate-1", debate.ID)
}

func TestResolveTasks_NoContext(t *testing.T) {
	SetGlobalContext(nil)

	params := graphql.ResolveParams{}
	result, err := ResolveTasks(params)

	assert.NoError(t, err)
	assert.IsType(t, []gqltypes.Task{}, result)
}

func TestResolveTasks_WithService(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.TaskSvc = &MockTaskService{
		tasks: []gqltypes.Task{
			{ID: "task-1", Type: "test", Status: "running"},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{Args: map[string]interface{}{}}
	result, err := ResolveTasks(params)

	assert.NoError(t, err)
	tasks, ok := result.([]gqltypes.Task)
	require.True(t, ok)
	assert.Len(t, tasks, 1)
}

func TestResolveTask_Found(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.TaskSvc = &MockTaskService{
		tasks: []gqltypes.Task{
			{ID: "task-1", Type: "test"},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Args: map[string]interface{}{"id": "task-1"},
	}
	result, err := ResolveTask(params)

	assert.NoError(t, err)
	task, ok := result.(*gqltypes.Task)
	require.True(t, ok)
	require.NotNil(t, task)
	assert.Equal(t, "task-1", task.ID)
}

func TestResolveVerificationResults_NoContext(t *testing.T) {
	SetGlobalContext(nil)

	params := graphql.ResolveParams{}
	result, err := ResolveVerificationResults(params)

	assert.NoError(t, err)
	assert.IsType(t, &gqltypes.VerificationResults{}, result)
}

func TestResolveVerificationResults_WithServices(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.Services = &MockServiceRegistry{
		verificationResults: &VerificationInfo{
			TotalProviders:    10,
			VerifiedProviders: 8,
			TotalModels:       50,
			VerifiedModels:    40,
			OverallScore:      8.5,
			LastVerified:      time.Now(),
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{}
	result, err := ResolveVerificationResults(params)

	assert.NoError(t, err)
	results, ok := result.(*gqltypes.VerificationResults)
	require.True(t, ok)
	assert.Equal(t, 10, results.TotalProviders)
	assert.Equal(t, 8, results.VerifiedProviders)
}

func TestResolveProviderScores_NoContext(t *testing.T) {
	SetGlobalContext(nil)

	params := graphql.ResolveParams{}
	result, err := ResolveProviderScores(params)

	assert.NoError(t, err)
	assert.IsType(t, []gqltypes.ProviderScore{}, result)
}

func TestResolveProviderScores_WithServices(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.Services = &MockServiceRegistry{
		providerScores: []ProviderScoreInfo{
			{ProviderID: "provider-1", OverallScore: 8.5},
			{ProviderID: "provider-2", OverallScore: 7.0},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{}
	result, err := ResolveProviderScores(params)

	assert.NoError(t, err)
	scores, ok := result.([]gqltypes.ProviderScore)
	require.True(t, ok)
	assert.Len(t, scores, 2)
}

func TestResolveCreateDebate_NoContext(t *testing.T) {
	SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"topic": "Test Topic",
			},
		},
	}
	result, err := ResolveCreateDebate(params)

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestResolveCreateDebate_WithService(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.DebateSvc = &MockDebateService{debates: []gqltypes.Debate{}}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"topic": "Test Topic",
			},
		},
	}
	result, err := ResolveCreateDebate(params)

	assert.NoError(t, err)
	debate, ok := result.(*gqltypes.Debate)
	require.True(t, ok)
	require.NotNil(t, debate)
	assert.Equal(t, "Test Topic", debate.Topic)
}

func TestResolveCreateTask_WithService(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.TaskSvc = &MockTaskService{tasks: []gqltypes.Task{}}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"type":     "background",
				"priority": 5,
			},
		},
	}
	result, err := ResolveCreateTask(params)

	assert.NoError(t, err)
	task, ok := result.(*gqltypes.Task)
	require.True(t, ok)
	require.NotNil(t, task)
	assert.Equal(t, "background", task.Type)
}

func TestResolveCancelTask_WithService(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.TaskSvc = &MockTaskService{
		tasks: []gqltypes.Task{
			{ID: "task-1", Status: "running"},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args:    map[string]interface{}{"id": "task-1"},
	}
	result, err := ResolveCancelTask(params)

	assert.NoError(t, err)
	task, ok := result.(*gqltypes.Task)
	require.True(t, ok)
	require.NotNil(t, task)
	assert.Equal(t, "cancelled", task.Status)
}

func TestResolveRefreshProvider_WithService(t *testing.T) {
	ctx := NewResolverContext(nil)
	ctx.Services = &MockServiceRegistry{
		providers: []ProviderInfo{
			{ID: "provider-1", Name: "Test Provider"},
		},
	}
	SetGlobalContext(ctx)
	defer SetGlobalContext(nil)

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args:    map[string]interface{}{"id": "provider-1"},
	}
	result, err := ResolveRefreshProvider(params)

	assert.NoError(t, err)
	provider, ok := result.(*gqltypes.Provider)
	require.True(t, ok)
	require.NotNil(t, provider)
	assert.Equal(t, "provider-1", provider.ID)
}

func TestConvertToGQLProvider(t *testing.T) {
	info := ProviderInfo{
		ID:        "test-id",
		Name:      "Test Provider",
		Type:      "api_key",
		Status:    "active",
		Score:     8.5,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Models: []ModelInfo{
			{
				ID:            "model-1",
				Name:          "Test Model",
				ProviderID:    "test-id",
				SupportsTools: true,
			},
		},
		HealthStatus: &HealthStatusInfo{
			Status:    "healthy",
			LatencyMs: 100,
		},
		Capabilities: &CapabilitiesInfo{
			Chat:      true,
			Streaming: true,
		},
	}

	result := convertToGQLProvider(info)

	assert.Equal(t, "test-id", result.ID)
	assert.Equal(t, "Test Provider", result.Name)
	assert.Equal(t, "api_key", result.Type)
	assert.Equal(t, "active", result.Status)
	assert.Equal(t, 8.5, result.Score)
	assert.Len(t, result.Models, 1)
	assert.Equal(t, "model-1", result.Models[0].ID)
	assert.NotNil(t, result.HealthStatus)
	assert.Equal(t, "healthy", result.HealthStatus.Status)
	assert.NotNil(t, result.Capabilities)
	assert.True(t, result.Capabilities.Chat)
}

func TestExtractProviderFilter(t *testing.T) {
	args := map[string]interface{}{
		"filter": map[string]interface{}{
			"status":    "active",
			"type":      "api_key",
			"min_score": 7.0,
			"max_score": 9.0,
		},
	}

	filter := extractProviderFilter(args)

	require.NotNil(t, filter)
	assert.Equal(t, "active", *filter.Status)
	assert.Equal(t, "api_key", *filter.Type)
	assert.Equal(t, 7.0, *filter.MinScore)
	assert.Equal(t, 9.0, *filter.MaxScore)
}

func TestExtractProviderFilter_NoFilter(t *testing.T) {
	args := map[string]interface{}{}

	filter := extractProviderFilter(args)

	assert.Nil(t, filter)
}

func TestExtractDebateFilter(t *testing.T) {
	args := map[string]interface{}{
		"filter": map[string]interface{}{
			"status": "running",
		},
	}

	filter := extractDebateFilter(args)

	require.NotNil(t, filter)
	assert.Equal(t, "running", *filter.Status)
}

func TestExtractTaskFilter(t *testing.T) {
	args := map[string]interface{}{
		"filter": map[string]interface{}{
			"status": "pending",
			"type":   "background",
		},
	}

	filter := extractTaskFilter(args)

	require.NotNil(t, filter)
	assert.Equal(t, "pending", *filter.Status)
	assert.Equal(t, "background", *filter.Type)
}

func TestExtractCreateDebateInput(t *testing.T) {
	args := map[string]interface{}{
		"input": map[string]interface{}{
			"topic":        "Test Topic",
			"participants": []interface{}{"p1", "p2"},
			"round_count":  3,
		},
	}

	input := extractCreateDebateInput(args)

	require.NotNil(t, input)
	assert.Equal(t, "Test Topic", input.Topic)
	assert.Len(t, input.Participants, 2)
	assert.Equal(t, 3, input.RoundCount)
}

func TestExtractDebateResponseInput(t *testing.T) {
	args := map[string]interface{}{
		"input": map[string]interface{}{
			"debate_id":      "debate-1",
			"participant_id": "participant-1",
			"content":        "Test response",
		},
	}

	input := extractDebateResponseInput(args)

	require.NotNil(t, input)
	assert.Equal(t, "debate-1", input.DebateID)
	assert.Equal(t, "participant-1", input.ParticipantID)
	assert.Equal(t, "Test response", input.Content)
}

func TestExtractCreateTaskInput(t *testing.T) {
	args := map[string]interface{}{
		"input": map[string]interface{}{
			"type":     "background",
			"priority": 5,
		},
	}

	input := extractCreateTaskInput(args)

	require.NotNil(t, input)
	assert.Equal(t, "background", input.Type)
	assert.Equal(t, 5, input.Priority)
}

func TestResolverContext_SetServices(t *testing.T) {
	ctx := NewResolverContext(nil)
	svc := &MockServiceRegistry{}

	ctx.SetServices(svc)

	assert.Equal(t, svc, ctx.Services)
}

func TestResolverContext_SetDebateService(t *testing.T) {
	ctx := NewResolverContext(nil)
	svc := &MockDebateService{}

	ctx.SetDebateService(svc)

	assert.Equal(t, svc, ctx.DebateSvc)
}

func TestResolverContext_SetTaskService(t *testing.T) {
	ctx := NewResolverContext(nil)
	svc := &MockTaskService{}

	ctx.SetTaskService(svc)

	assert.Equal(t, svc, ctx.TaskSvc)
}
