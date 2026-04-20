package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthStatusIsHealthy(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"healthy", "healthy", true},
		{"degraded", "degraded", false},
		{"unhealthy", "unhealthy", false},
		{"unknown", "unknown", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := &HealthStatus{Status: tt.status}
			assert.Equal(t, tt.want, hs.IsHealthy())
		})
	}
}

func TestPaginatedResponse(t *testing.T) {
	pr := PaginatedResponse{
		Total:    100,
		Page:     2,
		PageSize: 10,
		HasNext:  true,
		HasPrev:  true,
	}
	assert.Equal(t, int64(100), pr.Total)
	assert.Equal(t, 2, pr.Page)
	assert.Equal(t, 10, pr.PageSize)
	assert.True(t, pr.HasNext)
	assert.True(t, pr.HasPrev)
}

func TestVersionInfoString(t *testing.T) {
	v := VersionInfo{
		Service: "autotemp",
		Version: "1.2.3",
		Commit:  "abc123",
	}
	assert.Equal(t, "autotemp/1.2.3 (abc123)", v.String())
}

func TestTokenUsage(t *testing.T) {
	u := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
	}
	assert.Equal(t, 100, u.PromptTokens)
	assert.Equal(t, 200, u.CompletionTokens)
	assert.Equal(t, 300, u.TotalTokens)
}

func TestScoreBreakdown(t *testing.T) {
	s := ScoreBreakdown{
		Relevance:  85.5,
		Clarity:    90.0,
		Utility:    88.0,
		Creativity: 75.0,
		Coherence:  92.0,
		Safety:     95.0,
		Overall:    87.5,
	}
	assert.Equal(t, 85.5, s.Relevance)
	assert.Equal(t, 90.0, s.Clarity)
	assert.Equal(t, 88.0, s.Utility)
	assert.Equal(t, 75.0, s.Creativity)
	assert.Equal(t, 92.0, s.Coherence)
	assert.Equal(t, 95.0, s.Safety)
	assert.Equal(t, 87.5, s.Overall)
}

func TestProgressUpdate(t *testing.T) {
	pu := ProgressUpdate{
		Stage:           "generate",
		Status:          "running",
		Message:         "Processing prompt",
		PercentComplete: 42.5,
		Duration:        5 * time.Second,
	}
	assert.Equal(t, "generate", pu.Stage)
	assert.Equal(t, "running", pu.Status)
	assert.Equal(t, "Processing prompt", pu.Message)
	assert.Equal(t, 42.5, pu.PercentComplete)
	assert.Equal(t, 5*time.Second, pu.Duration)
}

func TestPipelineResult(t *testing.T) {
	pr := PipelineResult{
		Stages: []StageResult{
			{Stage: "stage1", Status: "success", Duration: 1 * time.Second},
			{Stage: "stage2", Status: "success", Duration: 2 * time.Second},
		},
		TotalDuration: 3 * time.Second,
		Success:       true,
	}
	assert.Len(t, pr.Stages, 2)
	assert.Equal(t, 3*time.Second, pr.TotalDuration)
	assert.True(t, pr.Success)
}

func TestHealthCheck(t *testing.T) {
	hc := HealthCheck{
		Name:         "database",
		Status:       "pass",
		Message:      "connected",
		ResponseTime: 10 * time.Millisecond,
	}
	assert.Equal(t, "database", hc.Name)
	assert.Equal(t, "pass", hc.Status)
	assert.Equal(t, "connected", hc.Message)
	assert.Equal(t, 10*time.Millisecond, hc.ResponseTime)
}
