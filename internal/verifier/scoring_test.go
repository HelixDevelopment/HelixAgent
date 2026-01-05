package verifier

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDefaultWeights(t *testing.T) {
	weights := DefaultWeights()
	if weights == nil {
		t.Fatal("DefaultWeights returned nil")
	}

	// Check that all weights are set
	if weights.ResponseSpeed <= 0 {
		t.Error("ResponseSpeed weight should be positive")
	}
	if weights.ModelEfficiency <= 0 {
		t.Error("ModelEfficiency weight should be positive")
	}
	if weights.CostEffectiveness <= 0 {
		t.Error("CostEffectiveness weight should be positive")
	}
	if weights.Capability <= 0 {
		t.Error("Capability weight should be positive")
	}
	if weights.Recency <= 0 {
		t.Error("Recency weight should be positive")
	}

	// Check that weights sum to approximately 1.0
	total := weights.ResponseSpeed + weights.ModelEfficiency +
		weights.CostEffectiveness + weights.Capability + weights.Recency
	if total < 0.99 || total > 1.01 {
		t.Errorf("weights should sum to ~1.0, got %f", total)
	}
}

func TestNewScoringService(t *testing.T) {
	svc, err := NewScoringService(nil)
	if err != nil {
		t.Fatalf("NewScoringService failed: %v", err)
	}
	if svc == nil {
		t.Fatal("NewScoringService returned nil")
	}
}

func TestNewScoringService_WithConfig(t *testing.T) {
	cfg := DefaultConfig()
	svc, err := NewScoringService(cfg)
	if err != nil {
		t.Fatalf("NewScoringService failed: %v", err)
	}
	if svc == nil {
		t.Fatal("NewScoringService returned nil")
	}
}

func TestScoringService_CalculateScore(t *testing.T) {
	svc, _ := NewScoringService(nil)

	result, err := svc.CalculateScore(context.Background(), "gpt-4")
	if err != nil {
		t.Fatalf("CalculateScore failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}

	// Score should be between 0 and 10
	if result.OverallScore < 0 || result.OverallScore > 10 {
		t.Errorf("OverallScore should be between 0 and 10, got %f", result.OverallScore)
	}

	// ScoreSuffix should be set
	if result.ScoreSuffix == "" {
		t.Error("ScoreSuffix should be set")
	}
}

func TestScoringService_CalculateScore_Cache(t *testing.T) {
	svc, _ := NewScoringService(nil)

	// First call
	result1, _ := svc.CalculateScore(context.Background(), "gpt-4")

	// Second call should return cached result
	result2, _ := svc.CalculateScore(context.Background(), "gpt-4")

	if result1.CalculatedAt != result2.CalculatedAt {
		t.Error("second call should return cached result")
	}
}

func TestScoringService_GetTopModels(t *testing.T) {
	svc, _ := NewScoringService(nil)

	models, err := svc.GetTopModels(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetTopModels failed: %v", err)
	}
	if models == nil {
		t.Error("expected non-nil slice")
	}
}

func TestScoringResult_Fields(t *testing.T) {
	now := time.Now()
	result := &ScoringResult{
		ModelID:      "gpt-4",
		ModelName:    "GPT-4",
		OverallScore: 9.5,
		ScoreSuffix:  "(SC:9.5)",
		Components: ScoreComponents{
			SpeedScore:      9.0,
			EfficiencyScore: 8.5,
			CostScore:       7.0,
			CapabilityScore: 10.0,
			RecencyScore:    9.5,
		},
		CalculatedAt: now,
		DataSource:   "models.dev",
	}

	if result.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if result.OverallScore != 9.5 {
		t.Error("OverallScore mismatch")
	}
	if result.ScoreSuffix != "(SC:9.5)" {
		t.Error("ScoreSuffix mismatch")
	}
	if result.Components.SpeedScore != 9.0 {
		t.Error("SpeedScore mismatch")
	}
	if result.DataSource != "models.dev" {
		t.Error("DataSource mismatch")
	}
}

func TestScoreComponents_Fields(t *testing.T) {
	components := ScoreComponents{
		SpeedScore:      9.0,
		EfficiencyScore: 8.5,
		CostScore:       7.0,
		CapabilityScore: 10.0,
		RecencyScore:    9.5,
	}

	if components.SpeedScore != 9.0 {
		t.Error("SpeedScore mismatch")
	}
	if components.EfficiencyScore != 8.5 {
		t.Error("EfficiencyScore mismatch")
	}
	if components.CostScore != 7.0 {
		t.Error("CostScore mismatch")
	}
	if components.CapabilityScore != 10.0 {
		t.Error("CapabilityScore mismatch")
	}
	if components.RecencyScore != 9.5 {
		t.Error("RecencyScore mismatch")
	}
}

func TestScoreWeights_Fields(t *testing.T) {
	weights := &ScoreWeights{
		ResponseSpeed:     0.25,
		ModelEfficiency:   0.20,
		CostEffectiveness: 0.25,
		Capability:        0.20,
		Recency:           0.10,
	}

	if weights.ResponseSpeed != 0.25 {
		t.Error("ResponseSpeed mismatch")
	}
	if weights.ModelEfficiency != 0.20 {
		t.Error("ModelEfficiency mismatch")
	}
	if weights.CostEffectiveness != 0.25 {
		t.Error("CostEffectiveness mismatch")
	}
	if weights.Capability != 0.20 {
		t.Error("Capability mismatch")
	}
	if weights.Recency != 0.10 {
		t.Error("Recency mismatch")
	}
}

func TestModelWithScore_Fields(t *testing.T) {
	model := &ModelWithScore{
		ModelID:      "gpt-4",
		Name:         "GPT-4",
		Provider:     "openai",
		OverallScore: 9.5,
		ScoreSuffix:  "(SC:9.5)",
	}

	if model.ModelID != "gpt-4" {
		t.Error("ModelID mismatch")
	}
	if model.Name != "GPT-4" {
		t.Error("Name mismatch")
	}
	if model.Provider != "openai" {
		t.Error("Provider mismatch")
	}
	if model.OverallScore != 9.5 {
		t.Error("OverallScore mismatch")
	}
}

func TestScoringResult_ZeroValue(t *testing.T) {
	var result ScoringResult

	if result.ModelID != "" {
		t.Error("zero ModelID should be empty")
	}
	if result.OverallScore != 0 {
		t.Error("zero OverallScore should be 0")
	}
}

func TestScoreComponents_ZeroValue(t *testing.T) {
	var components ScoreComponents

	if components.SpeedScore != 0 {
		t.Error("zero SpeedScore should be 0")
	}
	if components.CapabilityScore != 0 {
		t.Error("zero CapabilityScore should be 0")
	}
}

func TestScoreWeights_ZeroValue(t *testing.T) {
	var weights ScoreWeights

	if weights.ResponseSpeed != 0 {
		t.Error("zero ResponseSpeed should be 0")
	}
	if weights.Capability != 0 {
		t.Error("zero Capability should be 0")
	}
}

func TestScoringService_ConcurrentAccess(t *testing.T) {
	svc, _ := NewScoringService(nil)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = svc.CalculateScore(context.Background(), "gpt-4")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// =====================================================
// ADDITIONAL SCORING SERVICE TESTS FOR COMPREHENSIVE COVERAGE
// =====================================================

func TestScoringService_CalculateScore_DifferentModels(t *testing.T) {
	tests := []struct {
		name    string
		modelID string
	}{
		{"gpt-4", "gpt-4"},
		{"gpt-4o", "gpt-4o"},
		{"gpt-4-turbo", "gpt-4-turbo"},
		{"claude-3-opus", "claude-3-opus"},
		{"claude-3-sonnet", "claude-3-sonnet"},
		{"gemini-pro", "gemini-pro"},
		{"unknown model", "unknown-model-xyz"},
	}

	svc, _ := NewScoringService(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.CalculateScore(context.Background(), tt.modelID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if result.ModelID != tt.modelID {
				t.Errorf("expected ModelID %s, got %s", tt.modelID, result.ModelID)
			}
			// Score should be in valid range
			if result.OverallScore < 0 || result.OverallScore > 10 {
				t.Errorf("OverallScore out of range: %f", result.OverallScore)
			}
		})
	}
}

func TestScoringService_GetTopModels_Limit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
	}{
		{"limit 1", 1},
		{"limit 5", 5},
		{"limit 10", 10},
		{"limit 0", 0},
		{"limit -1", -1},
	}

	svc, _ := NewScoringService(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			models, err := svc.GetTopModels(context.Background(), tt.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if models == nil {
				t.Error("expected non-nil slice")
			}
			// For non-positive limits, expect empty or error handling
			if tt.limit > 0 && len(models) > tt.limit {
				t.Errorf("expected at most %d models, got %d", tt.limit, len(models))
			}
		})
	}
}

func TestScoringService_CalculateScore_Caching(t *testing.T) {
	svc, _ := NewScoringService(nil)

	// First call
	result1, err := svc.CalculateScore(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call - should be cached
	result2, err := svc.CalculateScore(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check timestamps are the same (cached)
	if result1.CalculatedAt != result2.CalculatedAt {
		t.Error("expected cached result with same timestamp")
	}

	// Check scores are the same
	if result1.OverallScore != result2.OverallScore {
		t.Error("expected same score from cache")
	}
}

func TestScoringService_calculateSpeedScore(t *testing.T) {
	svc, _ := NewScoringService(nil)

	tests := []struct {
		name     string
		modelID  string
		minScore float64
		maxScore float64
	}{
		{"fast model (groq)", "groq-llama", 8.0, 10.0},
		{"standard model (gpt-4)", "gpt-4", 7.0, 9.0},
		{"unknown model", "unknown", 5.0, 7.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := svc.calculateSpeedScore(tt.modelID)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("expected score in range [%f, %f], got %f", tt.minScore, tt.maxScore, score)
			}
		})
	}
}

func TestScoringService_calculateEfficiencyScore(t *testing.T) {
	svc, _ := NewScoringService(nil)

	tests := []struct {
		name    string
		modelID string
	}{
		{"gpt-4", "gpt-4"},
		{"claude", "claude-3-opus"},
		{"gemini", "gemini-pro"},
		{"unknown", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := svc.calculateEfficiencyScore(tt.modelID)
			if score < 0 || score > 10 {
				t.Errorf("efficiency score out of range: %f", score)
			}
		})
	}
}

func TestScoringService_calculateCostScore(t *testing.T) {
	svc, _ := NewScoringService(nil)

	tests := []struct {
		name    string
		modelID string
	}{
		{"expensive (gpt-4)", "gpt-4"},
		{"cheap (gpt-3.5)", "gpt-3.5-turbo"},
		{"free (ollama)", "llama:7b"},
		{"unknown", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := svc.calculateCostScore(tt.modelID)
			if score < 0 || score > 10 {
				t.Errorf("cost score out of range: %f", score)
			}
		})
	}
}

func TestScoringService_calculateCapabilityScore(t *testing.T) {
	svc, _ := NewScoringService(nil)

	tests := []struct {
		name    string
		modelID string
	}{
		{"high capability (gpt-4)", "gpt-4"},
		{"medium capability (gpt-3.5)", "gpt-3.5-turbo"},
		{"claude opus", "claude-3-opus"},
		{"unknown", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := svc.calculateCapabilityScore(tt.modelID)
			if score < 0 || score > 10 {
				t.Errorf("capability score out of range: %f", score)
			}
		})
	}
}

func TestScoringService_calculateRecencyScore(t *testing.T) {
	svc, _ := NewScoringService(nil)

	tests := []struct {
		name    string
		modelID string
	}{
		{"recent (gpt-4o)", "gpt-4o"},
		{"older (gpt-3.5)", "gpt-3.5-turbo"},
		{"very old", "text-davinci-003"},
		{"unknown", "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := svc.calculateRecencyScore(tt.modelID)
			if score < 0 || score > 10 {
				t.Errorf("recency score out of range: %f", score)
			}
		})
	}
}

func TestScoringService_GetModelScore_NotFound(t *testing.T) {
	svc, _ := NewScoringService(nil)

	// Without calculating first, cache is empty
	result, err := svc.GetModelScore(context.Background(), "non-existent-model")
	if err != nil {
		// Expected - model not in cache
		return
	}

	// If no error, it should have calculated the score
	if result != nil && result.ModelID != "non-existent-model" {
		t.Errorf("unexpected model ID: %s", result.ModelID)
	}
}

func TestScoringService_GetModelScore_Found(t *testing.T) {
	svc, _ := NewScoringService(nil)

	// Calculate first
	_, err := svc.CalculateScore(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now get from cache
	result, err := svc.GetModelScore(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ModelID != "test-model" {
		t.Errorf("expected model ID 'test-model', got '%s'", result.ModelID)
	}
}

func TestScoringService_ClearCache(t *testing.T) {
	svc, _ := NewScoringService(nil)

	// Calculate some scores
	svc.CalculateScore(context.Background(), "model1")
	svc.CalculateScore(context.Background(), "model2")

	// Clear cache
	svc.ClearCache()

	// Verify cache is empty using CacheSize method
	cacheLen := svc.CacheSize()

	if cacheLen != 0 {
		t.Errorf("expected empty cache, got %d entries", cacheLen)
	}
}

func TestScoringService_GetAllScores(t *testing.T) {
	svc, _ := NewScoringService(nil)

	// Calculate some scores
	svc.CalculateScore(context.Background(), "model1")
	svc.CalculateScore(context.Background(), "model2")

	// Get all scores
	scores := svc.GetAllScores()

	if len(scores) != 2 {
		t.Errorf("expected 2 scores, got %d", len(scores))
	}
}

func TestScoringService_GetAllScores_Empty(t *testing.T) {
	svc, _ := NewScoringService(nil)

	scores := svc.GetAllScores()

	if scores == nil {
		t.Error("expected non-nil slice")
	}
	if len(scores) != 0 {
		t.Errorf("expected 0 scores, got %d", len(scores))
	}
}

func TestScoreWeights_SumToOne(t *testing.T) {
	weights := DefaultWeights()

	sum := weights.ResponseSpeed + weights.ModelEfficiency +
		weights.CostEffectiveness + weights.Capability + weights.Recency

	// Allow small floating point errors
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("weights should sum to ~1.0, got %f", sum)
	}
}

func TestScoreWeights_AllPositive(t *testing.T) {
	weights := DefaultWeights()

	if weights.ResponseSpeed <= 0 {
		t.Errorf("ResponseSpeed should be positive, got %f", weights.ResponseSpeed)
	}
	if weights.ModelEfficiency <= 0 {
		t.Errorf("ModelEfficiency should be positive, got %f", weights.ModelEfficiency)
	}
	if weights.CostEffectiveness <= 0 {
		t.Errorf("CostEffectiveness should be positive, got %f", weights.CostEffectiveness)
	}
	if weights.Capability <= 0 {
		t.Errorf("Capability should be positive, got %f", weights.Capability)
	}
	if weights.Recency <= 0 {
		t.Errorf("Recency should be positive, got %f", weights.Recency)
	}
}

func TestScoringService_CalculateScore_WithCustomWeights(t *testing.T) {
	cfg := &Config{
		Scoring: ScoringConfig{
			Weights: ScoringWeightsConfig{
				ResponseSpeed:     0.5,
				ModelEfficiency:   0.2,
				CostEffectiveness: 0.1,
				Capability:        0.1,
				Recency:           0.1,
			},
		},
	}

	svc, err := NewScoringService(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := svc.CalculateScore(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if result.OverallScore < 0 || result.OverallScore > 10 {
		t.Errorf("OverallScore out of range: %f", result.OverallScore)
	}
}

func TestScoringService_ComputeWeightedScore(t *testing.T) {
	svc, _ := NewScoringService(nil)

	components := ScoreComponents{
		SpeedScore:      10.0,
		EfficiencyScore: 10.0,
		CostScore:       10.0,
		CapabilityScore: 10.0,
		RecencyScore:    10.0,
	}

	score := svc.computeWeightedScore(components)

	// With all scores at 10 and weights summing to 1, result should be 10
	if score < 9.9 || score > 10.1 {
		t.Errorf("expected score ~10.0, got %f", score)
	}
}

func TestScoringService_ComputeWeightedScore_Mixed(t *testing.T) {
	svc, _ := NewScoringService(nil)

	components := ScoreComponents{
		SpeedScore:      5.0,
		EfficiencyScore: 5.0,
		CostScore:       5.0,
		CapabilityScore: 5.0,
		RecencyScore:    5.0,
	}

	score := svc.computeWeightedScore(components)

	// With all scores at 5 and weights summing to 1, result should be 5
	if score < 4.9 || score > 5.1 {
		t.Errorf("expected score ~5.0, got %f", score)
	}
}

func TestScoringResult_ScoreSuffix_Format(t *testing.T) {
	svc, _ := NewScoringService(nil)

	result, _ := svc.CalculateScore(context.Background(), "test-model")

	// ScoreSuffix should be in format (SC:X.X)
	if result.ScoreSuffix == "" {
		t.Error("ScoreSuffix should not be empty")
	}
	if !strings.HasPrefix(result.ScoreSuffix, "(SC:") {
		t.Errorf("ScoreSuffix should start with '(SC:', got %s", result.ScoreSuffix)
	}
	if !strings.HasSuffix(result.ScoreSuffix, ")") {
		t.Errorf("ScoreSuffix should end with ')', got %s", result.ScoreSuffix)
	}
}

func TestModelWithScore_Sorting(t *testing.T) {
	models := []*ModelWithScore{
		{ModelID: "low", OverallScore: 3.0},
		{ModelID: "high", OverallScore: 9.0},
		{ModelID: "medium", OverallScore: 6.0},
	}

	// Sort by score descending
	for i := 0; i < len(models)-1; i++ {
		for j := i + 1; j < len(models); j++ {
			if models[j].OverallScore > models[i].OverallScore {
				models[i], models[j] = models[j], models[i]
			}
		}
	}

	if models[0].ModelID != "high" {
		t.Errorf("expected 'high' first, got '%s'", models[0].ModelID)
	}
	if models[1].ModelID != "medium" {
		t.Errorf("expected 'medium' second, got '%s'", models[1].ModelID)
	}
	if models[2].ModelID != "low" {
		t.Errorf("expected 'low' third, got '%s'", models[2].ModelID)
	}
}
