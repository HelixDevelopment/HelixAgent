package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProvider_Fields(t *testing.T) {
	now := time.Now()
	provider := Provider{
		ID:        "provider-1",
		Name:      "OpenAI",
		Type:      "api_key",
		Status:    "active",
		Score:     9.5,
		Models:    []Model{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "provider-1", provider.ID)
	assert.Equal(t, "OpenAI", provider.Name)
	assert.Equal(t, "api_key", provider.Type)
	assert.Equal(t, "active", provider.Status)
	assert.Equal(t, 9.5, provider.Score)
	assert.NotNil(t, provider.Models)
}

func TestModel_Fields(t *testing.T) {
	now := time.Now()
	model := Model{
		ID:                "gpt-4",
		Name:              "GPT-4",
		ProviderID:        "openai",
		Version:           "2024-01",
		ContextWindow:     128000,
		MaxTokens:         4096,
		SupportsTools:     true,
		SupportsVision:    true,
		SupportsStreaming: true,
		Score:             9.8,
		Rank:              1,
		CreatedAt:         now,
	}

	assert.Equal(t, "gpt-4", model.ID)
	assert.Equal(t, "GPT-4", model.Name)
	assert.Equal(t, 128000, model.ContextWindow)
	assert.True(t, model.SupportsTools)
	assert.True(t, model.SupportsVision)
}

func TestHealthStatus_Fields(t *testing.T) {
	now := time.Now()
	status := HealthStatus{
		Status:       "healthy",
		Latency:      150,
		LastCheck:    now,
		ErrorMessage: "",
	}

	assert.Equal(t, "healthy", status.Status)
	assert.Equal(t, int64(150), status.Latency)
	assert.Equal(t, "", status.ErrorMessage)
}

func TestHealthStatus_WithError(t *testing.T) {
	status := HealthStatus{
		Status:       "unhealthy",
		Latency:      0,
		ErrorMessage: "connection refused",
	}

	assert.Equal(t, "unhealthy", status.Status)
	assert.Equal(t, "connection refused", status.ErrorMessage)
}

func TestCapabilities_AllEnabled(t *testing.T) {
	caps := Capabilities{
		Chat:            true,
		Completions:     true,
		Embeddings:      true,
		Vision:          true,
		ToolUse:         true,
		Streaming:       true,
		FunctionCalling: true,
	}

	assert.True(t, caps.Chat)
	assert.True(t, caps.Completions)
	assert.True(t, caps.Embeddings)
	assert.True(t, caps.Vision)
	assert.True(t, caps.ToolUse)
	assert.True(t, caps.Streaming)
	assert.True(t, caps.FunctionCalling)
}

func TestDebate_Fields(t *testing.T) {
	now := time.Now()
	debate := Debate{
		ID:           "debate-1",
		Topic:        "Should AI be regulated?",
		Status:       "running",
		Participants: []Participant{},
		Rounds:       []DebateRound{},
		Confidence:   0.85,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assert.Equal(t, "debate-1", debate.ID)
	assert.Equal(t, "Should AI be regulated?", debate.Topic)
	assert.Equal(t, "running", debate.Status)
	assert.Equal(t, 0.85, debate.Confidence)
}

func TestDebate_Completed(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(10 * time.Minute)
	debate := Debate{
		ID:          "debate-2",
		Status:      "completed",
		Conclusion:  "AI should have limited regulation",
		CompletedAt: &completedAt,
	}

	assert.Equal(t, "completed", debate.Status)
	assert.NotEmpty(t, debate.Conclusion)
	assert.NotNil(t, debate.CompletedAt)
}

func TestParticipant_Fields(t *testing.T) {
	participant := Participant{
		ID:         "participant-1",
		ProviderID: "anthropic",
		ModelID:    "claude-3-opus",
		Position:   "pro",
		Role:       "primary",
		Score:      9.2,
	}

	assert.Equal(t, "participant-1", participant.ID)
	assert.Equal(t, "anthropic", participant.ProviderID)
	assert.Equal(t, "claude-3-opus", participant.ModelID)
	assert.Equal(t, "pro", participant.Position)
	assert.Equal(t, "primary", participant.Role)
}

func TestDebateRound_Fields(t *testing.T) {
	now := time.Now()
	round := DebateRound{
		ID:          "round-1",
		DebateID:    "debate-1",
		RoundNumber: 1,
		Responses:   []Response{},
		Summary:     "Initial arguments presented",
		CreatedAt:   now,
	}

	assert.Equal(t, "round-1", round.ID)
	assert.Equal(t, 1, round.RoundNumber)
	assert.NotEmpty(t, round.Summary)
}

func TestResponse_Fields(t *testing.T) {
	now := time.Now()
	response := Response{
		ParticipantID: "participant-1",
		Content:       "I believe that...",
		Confidence:    0.92,
		TokenCount:    150,
		Latency:       1200,
		CreatedAt:     now,
	}

	assert.Equal(t, "participant-1", response.ParticipantID)
	assert.NotEmpty(t, response.Content)
	assert.Equal(t, 0.92, response.Confidence)
	assert.Equal(t, 150, response.TokenCount)
	assert.Equal(t, int64(1200), response.Latency)
}

func TestTask_Fields(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(time.Second)
	task := Task{
		ID:        "task-1",
		Type:      "verification",
		Status:    "running",
		Priority:  1,
		Progress:  50,
		CreatedAt: now,
		StartedAt: &startedAt,
	}

	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "verification", task.Type)
	assert.Equal(t, "running", task.Status)
	assert.Equal(t, 50, task.Progress)
	assert.NotNil(t, task.StartedAt)
}

func TestTask_Failed(t *testing.T) {
	task := Task{
		ID:     "task-2",
		Status: "failed",
		Error:  "timeout exceeded",
	}

	assert.Equal(t, "failed", task.Status)
	assert.NotEmpty(t, task.Error)
}

func TestVerificationResults_Fields(t *testing.T) {
	now := time.Now()
	results := VerificationResults{
		TotalProviders:    10,
		VerifiedProviders: 8,
		TotalModels:       100,
		VerifiedModels:    85,
		OverallScore:      8.5,
		LastVerified:      now,
	}

	assert.Equal(t, 10, results.TotalProviders)
	assert.Equal(t, 8, results.VerifiedProviders)
	assert.Equal(t, 100, results.TotalModels)
	assert.Equal(t, 85, results.VerifiedModels)
	assert.Equal(t, 8.5, results.OverallScore)
}

func TestProviderScore_Fields(t *testing.T) {
	score := ProviderScore{
		ProviderID:        "openai",
		ProviderName:      "OpenAI",
		OverallScore:      9.2,
		ResponseSpeed:     9.5,
		ModelEfficiency:   9.0,
		CostEffectiveness: 8.5,
		Capability:        9.8,
		Recency:           9.0,
	}

	assert.Equal(t, "openai", score.ProviderID)
	assert.Equal(t, 9.2, score.OverallScore)
	assert.Equal(t, 9.5, score.ResponseSpeed)
}

func TestConnection_Generic(t *testing.T) {
	conn := Connection[Model]{
		Edges: []Edge[Model]{
			{Node: Model{ID: "model-1"}, Cursor: "cursor1"},
			{Node: Model{ID: "model-2"}, Cursor: "cursor2"},
		},
		PageInfo: PageInfo{
			HasNextPage:     true,
			HasPreviousPage: false,
			StartCursor:     "cursor1",
			EndCursor:       "cursor2",
		},
		TotalCount: 100,
	}

	assert.Len(t, conn.Edges, 2)
	assert.Equal(t, 100, conn.TotalCount)
	assert.True(t, conn.PageInfo.HasNextPage)
	assert.False(t, conn.PageInfo.HasPreviousPage)
}

func TestEdge_Generic(t *testing.T) {
	edge := Edge[Provider]{
		Node:   Provider{ID: "provider-1", Name: "Test"},
		Cursor: "abc123",
	}

	assert.Equal(t, "provider-1", edge.Node.ID)
	assert.Equal(t, "abc123", edge.Cursor)
}

func TestProviderFilter_Fields(t *testing.T) {
	status := "active"
	providerType := "api_key"
	minScore := 8.0
	maxScore := 10.0

	filter := ProviderFilter{
		Status:   &status,
		Type:     &providerType,
		MinScore: &minScore,
		MaxScore: &maxScore,
	}

	assert.Equal(t, "active", *filter.Status)
	assert.Equal(t, "api_key", *filter.Type)
	assert.Equal(t, 8.0, *filter.MinScore)
	assert.Equal(t, 10.0, *filter.MaxScore)
}

func TestDebateFilter_Fields(t *testing.T) {
	status := "completed"
	filter := DebateFilter{
		Status: &status,
	}

	assert.Equal(t, "completed", *filter.Status)
}

func TestTaskFilter_Fields(t *testing.T) {
	status := "pending"
	taskType := "verification"
	filter := TaskFilter{
		Status: &status,
		Type:   &taskType,
	}

	assert.Equal(t, "pending", *filter.Status)
	assert.Equal(t, "verification", *filter.Type)
}

func TestCreateDebateInput_Fields(t *testing.T) {
	input := CreateDebateInput{
		Topic:        "AI Ethics",
		Participants: []string{"openai", "anthropic"},
		RoundCount:   3,
	}

	assert.Equal(t, "AI Ethics", input.Topic)
	assert.Len(t, input.Participants, 2)
	assert.Equal(t, 3, input.RoundCount)
}

func TestDebateResponseInput_Fields(t *testing.T) {
	input := DebateResponseInput{
		DebateID:      "debate-1",
		ParticipantID: "participant-1",
		Content:       "My argument is...",
	}

	assert.Equal(t, "debate-1", input.DebateID)
	assert.Equal(t, "participant-1", input.ParticipantID)
	assert.NotEmpty(t, input.Content)
}

func TestCreateTaskInput_Fields(t *testing.T) {
	input := CreateTaskInput{
		Type:     "verification",
		Priority: 1,
		Payload: map[string]interface{}{
			"provider_id": "openai",
		},
	}

	assert.Equal(t, "verification", input.Type)
	assert.Equal(t, 1, input.Priority)
	assert.NotNil(t, input.Payload)
}

func TestDebateUpdate_Fields(t *testing.T) {
	now := time.Now()
	update := DebateUpdate{
		DebateID:   "debate-1",
		Type:       "round_started",
		Timestamp:  now,
	}

	assert.Equal(t, "debate-1", update.DebateID)
	assert.Equal(t, "round_started", update.Type)
}

func TestTaskProgress_Fields(t *testing.T) {
	now := time.Now()
	progress := TaskProgress{
		TaskID:    "task-1",
		Status:    "running",
		Progress:  75,
		Message:   "Processing...",
		Timestamp: now,
	}

	assert.Equal(t, "task-1", progress.TaskID)
	assert.Equal(t, 75, progress.Progress)
	assert.NotEmpty(t, progress.Message)
}

func TestProviderHealthUpdate_Fields(t *testing.T) {
	now := time.Now()
	update := ProviderHealthUpdate{
		ProviderID: "openai",
		Status: HealthStatus{
			Status:  "healthy",
			Latency: 100,
		},
		Timestamp: now,
	}

	assert.Equal(t, "openai", update.ProviderID)
	assert.Equal(t, "healthy", update.Status.Status)
}

func TestTokenStreamEvent_Fields(t *testing.T) {
	now := time.Now()
	event := TokenStreamEvent{
		RequestID:  "req-1",
		Token:      "Hello",
		IsComplete: false,
		TokenCount: 1,
		Timestamp:  now,
	}

	assert.Equal(t, "req-1", event.RequestID)
	assert.Equal(t, "Hello", event.Token)
	assert.False(t, event.IsComplete)
	assert.Equal(t, 1, event.TokenCount)
}

func TestTokenStreamEvent_Complete(t *testing.T) {
	event := TokenStreamEvent{
		RequestID:  "req-2",
		Token:      "",
		IsComplete: true,
		TokenCount: 100,
	}

	assert.True(t, event.IsComplete)
	assert.Equal(t, 100, event.TokenCount)
}
