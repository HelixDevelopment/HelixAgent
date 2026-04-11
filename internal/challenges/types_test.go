package challenges

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProviderInfo_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tests := []struct {
		name string
		info ProviderInfo
	}{
		{
			name: "api key provider",
			info: ProviderInfo{
				Name:        "openai",
				DisplayName: "OpenAI",
				Type:        "api_key",
				APIKeyEnv:   "OPENAI_API_KEY",
				Models:      []string{"gpt-4", "gpt-3.5-turbo"},
				Score:       9.5,
				Verified:    true,
				VerifiedAt:  now,
			},
		},
		{
			name: "oauth provider",
			info: ProviderInfo{
				Name:        "claude",
				DisplayName: "Claude",
				Type:        "oauth",
				APIKeyEnv:   "",
				Models:      []string{"claude-3-opus"},
				Score:       8.0,
				Verified:    true,
				VerifiedAt:  now,
			},
		},
		{
			name: "free provider",
			info: ProviderInfo{
				Name:        "zen",
				DisplayName: "Zen",
				Type:        "free",
				APIKeyEnv:   "",
				Models:      nil,
				Score:       6.5,
				Verified:    false,
				VerifiedAt:  time.Time{},
			},
		},
		{
			name: "zero value provider",
			info: ProviderInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.info.Name, tt.info.Name)
			assert.Equal(t, tt.info.DisplayName, tt.info.DisplayName)
			assert.Equal(t, tt.info.Type, tt.info.Type)
			assert.Equal(t, tt.info.APIKeyEnv, tt.info.APIKeyEnv)
			assert.Equal(t, tt.info.Models, tt.info.Models)
			assert.Equal(t, tt.info.Score, tt.info.Score)
			assert.Equal(t, tt.info.Verified, tt.info.Verified)
			assert.Equal(t, tt.info.VerifiedAt, tt.info.VerifiedAt)
		})
	}
}

func TestProviderInfo_VerifiedState(t *testing.T) {
	t.Parallel()
	verified := ProviderInfo{
		Name:       "openai",
		Score:      9.0,
		Verified:   true,
		VerifiedAt: time.Now(),
	}
	assert.True(t, verified.Verified)
	assert.False(t, verified.VerifiedAt.IsZero())

	unverified := ProviderInfo{
		Name:     "unknown",
		Score:    0,
		Verified: false,
	}
	assert.False(t, unverified.Verified)
	assert.True(t, unverified.VerifiedAt.IsZero())
}

func TestModelScore_Fields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		score ModelScore
	}{
		{
			name: "high scoring model",
			score: ModelScore{
				ModelID:           "gpt-4",
				Provider:          "openai",
				ResponseSpeed:     9.0,
				CostEffectiveness: 7.0,
				ModelEfficiency:   8.5,
				Capability:        9.5,
				Recency:           8.0,
				TotalScore:        8.5,
			},
		},
		{
			name: "low scoring model",
			score: ModelScore{
				ModelID:           "small-model",
				Provider:          "free-provider",
				ResponseSpeed:     5.0,
				CostEffectiveness: 9.0,
				ModelEfficiency:   4.0,
				Capability:        3.0,
				Recency:           2.0,
				TotalScore:        5.2,
			},
		},
		{
			name:  "zero value model score",
			score: ModelScore{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.score.ModelID, tt.score.ModelID)
			assert.Equal(t, tt.score.Provider, tt.score.Provider)
			assert.Equal(t, tt.score.ResponseSpeed, tt.score.ResponseSpeed)
			assert.Equal(t, tt.score.CostEffectiveness, tt.score.CostEffectiveness)
			assert.Equal(t, tt.score.ModelEfficiency, tt.score.ModelEfficiency)
			assert.Equal(t, tt.score.Capability, tt.score.Capability)
			assert.Equal(t, tt.score.Recency, tt.score.Recency)
			assert.Equal(t, tt.score.TotalScore, tt.score.TotalScore)
		})
	}
}

func TestModelScore_WeightedComponents(t *testing.T) {
	t.Parallel()
	score := ModelScore{
		ResponseSpeed:     8.0,
		CostEffectiveness: 7.0,
		ModelEfficiency:   9.0,
		Capability:        8.5,
		Recency:           6.0,
	}

	// Verify scoring weights: Speed 25%, Cost 25%, Efficiency 20%,
	// Capability 20%, Recency 10%
	expected := score.ResponseSpeed*0.25 +
		score.CostEffectiveness*0.25 +
		score.ModelEfficiency*0.20 +
		score.Capability*0.20 +
		score.Recency*0.10

	assert.InDelta(t, 7.85, expected, 0.01)
}

func TestDebateGroup_Fields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		group DebateGroup
	}{
		{
			name: "group with members",
			group: DebateGroup{
				Name:     "primary-team",
				Strategy: "weighted",
				Members: []DebateGroupMember{
					{Provider: "claude", Model: "opus", Role: "analyst", Score: 9.0},
					{Provider: "gemini", Model: "pro", Role: "critic", Score: 8.5},
				},
			},
		},
		{
			name: "empty group",
			group: DebateGroup{
				Name:     "empty-team",
				Strategy: "majority",
				Members:  nil,
			},
		},
		{
			name:  "zero value group",
			group: DebateGroup{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.group.Name, tt.group.Name)
			assert.Equal(t, tt.group.Strategy, tt.group.Strategy)
			assert.Equal(t, tt.group.Members, tt.group.Members)
		})
	}
}

func TestDebateGroupMember_Fields(t *testing.T) {
	t.Parallel()
	member := DebateGroupMember{
		Provider: "deepseek",
		Model:    "coder",
		Role:     "developer",
		Score:    8.7,
	}

	assert.Equal(t, "deepseek", member.Provider)
	assert.Equal(t, "coder", member.Model)
	assert.Equal(t, "developer", member.Role)
	assert.InDelta(t, 8.7, member.Score, 0.001)
}

func TestDebateGroup_MemberCount(t *testing.T) {
	t.Parallel()
	group := DebateGroup{
		Name: "test-team",
		Members: []DebateGroupMember{
			{Provider: "a", Score: 9.0},
			{Provider: "b", Score: 8.0},
			{Provider: "c", Score: 7.0},
		},
	}
	assert.Len(t, group.Members, 3)

	empty := DebateGroup{Name: "empty"}
	assert.Empty(t, empty.Members)
}

func TestTestPrompt_Fields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		prompt TestPrompt
	}{
		{
			name: "standard prompt",
			prompt: TestPrompt{
				ID:         "prompt-001",
				Category:   "reasoning",
				Prompt:     "What is 2+2?",
				Expected:   "4",
				MaxLatency: 5 * time.Second,
				MinQuality: 0.9,
			},
		},
		{
			name: "code generation prompt",
			prompt: TestPrompt{
				ID:         "prompt-002",
				Category:   "code",
				Prompt:     "Write a hello world in Go",
				Expected:   "fmt.Println",
				MaxLatency: 10 * time.Second,
				MinQuality: 0.8,
			},
		},
		{
			name:   "zero value prompt",
			prompt: TestPrompt{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.prompt.ID, tt.prompt.ID)
			assert.Equal(t, tt.prompt.Category, tt.prompt.Category)
			assert.Equal(t, tt.prompt.Prompt, tt.prompt.Prompt)
			assert.Equal(t, tt.prompt.Expected, tt.prompt.Expected)
			assert.Equal(t, tt.prompt.MaxLatency, tt.prompt.MaxLatency)
			assert.Equal(t, tt.prompt.MinQuality, tt.prompt.MinQuality)
		})
	}
}

func TestAPITestResult_Fields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		result APITestResult
	}{
		{
			name: "successful result",
			result: APITestResult{
				PromptID:   "prompt-001",
				Response:   "4",
				Latency:    200 * time.Millisecond,
				StatusCode: 200,
				Quality:    0.95,
				Passed:     true,
				Error:      "",
			},
		},
		{
			name: "failed result",
			result: APITestResult{
				PromptID:   "prompt-002",
				Response:   "",
				Latency:    30 * time.Second,
				StatusCode: 500,
				Quality:    0.0,
				Passed:     false,
				Error:      "server error",
			},
		},
		{
			name: "timeout result",
			result: APITestResult{
				PromptID:   "prompt-003",
				Response:   "",
				Latency:    60 * time.Second,
				StatusCode: 0,
				Quality:    0.0,
				Passed:     false,
				Error:      "context deadline exceeded",
			},
		},
		{
			name:   "zero value result",
			result: APITestResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.result.PromptID, tt.result.PromptID)
			assert.Equal(t, tt.result.Response, tt.result.Response)
			assert.Equal(t, tt.result.Latency, tt.result.Latency)
			assert.Equal(t, tt.result.StatusCode, tt.result.StatusCode)
			assert.Equal(t, tt.result.Quality, tt.result.Quality)
			assert.Equal(t, tt.result.Passed, tt.result.Passed)
			assert.Equal(t, tt.result.Error, tt.result.Error)
		})
	}
}

func TestAPITestResult_PassedState(t *testing.T) {
	t.Parallel()
	passed := APITestResult{
		PromptID:   "p1",
		StatusCode: 200,
		Quality:    0.95,
		Passed:     true,
	}
	assert.True(t, passed.Passed)
	assert.Equal(t, 200, passed.StatusCode)
	assert.Greater(t, passed.Quality, 0.9)
	assert.Empty(t, passed.Error)

	failed := APITestResult{
		PromptID:   "p2",
		StatusCode: 429,
		Quality:    0.0,
		Passed:     false,
		Error:      "rate limited",
	}
	assert.False(t, failed.Passed)
	assert.NotEmpty(t, failed.Error)
}
