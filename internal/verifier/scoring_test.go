package verifier

import (
	"context"
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
