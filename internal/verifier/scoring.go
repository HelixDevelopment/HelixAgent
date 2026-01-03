package verifier

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ScoringResult represents the scoring result for a model
type ScoringResult struct {
	ModelID      string          `json:"model_id"`
	ModelName    string          `json:"model_name"`
	OverallScore float64         `json:"overall_score"`
	ScoreSuffix  string          `json:"score_suffix"`
	Components   ScoreComponents `json:"components"`
	CalculatedAt time.Time       `json:"calculated_at"`
	DataSource   string          `json:"data_source"`
}

// ScoreComponents represents the individual scoring components
type ScoreComponents struct {
	SpeedScore      float64 `json:"speed_score"`
	EfficiencyScore float64 `json:"efficiency_score"`
	CostScore       float64 `json:"cost_score"`
	CapabilityScore float64 `json:"capability_score"`
	RecencyScore    float64 `json:"recency_score"`
}

// ScoreWeights represents the weights for scoring components
type ScoreWeights struct {
	ResponseSpeed     float64 `json:"response_speed" yaml:"response_speed"`
	ModelEfficiency   float64 `json:"model_efficiency" yaml:"model_efficiency"`
	CostEffectiveness float64 `json:"cost_effectiveness" yaml:"cost_effectiveness"`
	Capability        float64 `json:"capability" yaml:"capability"`
	Recency           float64 `json:"recency" yaml:"recency"`
}

// ModelWithScore represents a model with its score
type ModelWithScore struct {
	ModelID      string  `json:"model_id"`
	Name         string  `json:"name"`
	Provider     string  `json:"provider"`
	OverallScore float64 `json:"overall_score"`
	ScoreSuffix  string  `json:"score_suffix"`
}

// ScoringService manages model scoring operations
type ScoringService struct {
	weights  *ScoreWeights
	cache    map[string]*ScoringResult
	cacheMu  sync.RWMutex
	cacheTTL time.Duration
}

// NewScoringService creates a new scoring service
func NewScoringService(cfg *Config) (*ScoringService, error) {
	weights := DefaultWeights()
	if cfg != nil && cfg.Scoring.Weights.ResponseSpeed > 0 {
		weights = &ScoreWeights{
			ResponseSpeed:     cfg.Scoring.Weights.ResponseSpeed,
			ModelEfficiency:   cfg.Scoring.Weights.ModelEfficiency,
			CostEffectiveness: cfg.Scoring.Weights.CostEffectiveness,
			Capability:        cfg.Scoring.Weights.Capability,
			Recency:           cfg.Scoring.Weights.Recency,
		}
	}

	cacheTTL := 6 * time.Hour
	if cfg != nil && cfg.Scoring.CacheTTL > 0 {
		cacheTTL = cfg.Scoring.CacheTTL
	}

	return &ScoringService{
		weights:  weights,
		cache:    make(map[string]*ScoringResult),
		cacheTTL: cacheTTL,
	}, nil
}

// CalculateScore calculates comprehensive score for a model
func (s *ScoringService) CalculateScore(ctx context.Context, modelID string) (*ScoringResult, error) {
	// Check cache first
	s.cacheMu.RLock()
	if cached, ok := s.cache[modelID]; ok {
		if time.Since(cached.CalculatedAt) < s.cacheTTL {
			s.cacheMu.RUnlock()
			return cached, nil
		}
	}
	s.cacheMu.RUnlock()

	// Calculate basic score
	return s.calculateBasicScore(ctx, modelID)
}

// calculateBasicScore calculates a basic score
func (s *ScoringService) calculateBasicScore(ctx context.Context, modelID string) (*ScoringResult, error) {
	// Base scores based on model name patterns
	baseScore := 5.0

	// Adjust based on known model families
	modelPatterns := map[string]float64{
		"gpt-4":          9.0,
		"gpt-4o":         9.5,
		"claude-3":       9.0,
		"claude-3.5":     9.5,
		"claude-opus":    9.5,
		"gemini-pro":     8.5,
		"gemini-ultra":   9.0,
		"llama-3":        7.5,
		"mistral-large":  8.0,
		"deepseek-coder": 7.5,
		"qwen":           7.0,
	}

	for pattern, score := range modelPatterns {
		if containsIgnoreCase(modelID, pattern) {
			baseScore = score
			break
		}
	}

	// Ensure score is within bounds
	baseScore = math.Max(0, math.Min(10, baseScore))

	result := &ScoringResult{
		ModelID:      modelID,
		ModelName:    modelID,
		OverallScore: baseScore,
		ScoreSuffix:  fmt.Sprintf("(SC:%.1f)", baseScore),
		Components: ScoreComponents{
			SpeedScore:      baseScore,
			EfficiencyScore: baseScore,
			CostScore:       baseScore,
			CapabilityScore: baseScore,
			RecencyScore:    baseScore,
		},
		CalculatedAt: time.Now(),
		DataSource:   "basic",
	}

	// Update cache
	s.cacheMu.Lock()
	s.cache[modelID] = result
	s.cacheMu.Unlock()

	return result, nil
}

// BatchCalculateScores calculates scores for multiple models
func (s *ScoringService) BatchCalculateScores(ctx context.Context, modelIDs []string) ([]*ScoringResult, error) {
	results := make([]*ScoringResult, 0, len(modelIDs))
	var wg sync.WaitGroup
	resultChan := make(chan *ScoringResult, len(modelIDs))
	errChan := make(chan error, len(modelIDs))

	for _, modelID := range modelIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			score, err := s.CalculateScore(ctx, id)
			if err != nil {
				errChan <- err
				return
			}
			resultChan <- score
		}(modelID)
	}

	wg.Wait()
	close(resultChan)
	close(errChan)

	for result := range resultChan {
		results = append(results, result)
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].OverallScore > results[j].OverallScore
	})

	return results, nil
}

// GetTopModels returns top scoring models
func (s *ScoringService) GetTopModels(ctx context.Context, limit int) ([]*ModelWithScore, error) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	models := make([]*ScoringResult, 0, len(s.cache))
	for _, result := range s.cache {
		models = append(models, result)
	}

	// Sort by score descending
	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	if limit > len(models) {
		limit = len(models)
	}

	results := make([]*ModelWithScore, limit)
	for i := 0; i < limit; i++ {
		results[i] = &ModelWithScore{
			ModelID:      models[i].ModelID,
			Name:         models[i].ModelName,
			Provider:     "", // Would need provider info from elsewhere
			OverallScore: models[i].OverallScore,
			ScoreSuffix:  fmt.Sprintf("(SC:%.1f)", models[i].OverallScore),
		}
	}

	return results, nil
}

// GetModelsByScoreRange returns models within a score range
func (s *ScoringService) GetModelsByScoreRange(ctx context.Context, minScore, maxScore float64, limit int) ([]*ModelWithScore, error) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	filtered := make([]*ScoringResult, 0)
	for _, result := range s.cache {
		if result.OverallScore >= minScore && result.OverallScore <= maxScore {
			filtered = append(filtered, result)
		}
	}

	// Sort by score descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].OverallScore > filtered[j].OverallScore
	})

	if limit > len(filtered) {
		limit = len(filtered)
	}

	results := make([]*ModelWithScore, limit)
	for i := 0; i < limit; i++ {
		results[i] = &ModelWithScore{
			ModelID:      filtered[i].ModelID,
			Name:         filtered[i].ModelName,
			Provider:     "",
			OverallScore: filtered[i].OverallScore,
			ScoreSuffix:  fmt.Sprintf("(SC:%.1f)", filtered[i].OverallScore),
		}
	}

	return results, nil
}

// UpdateWeights updates scoring weights
func (s *ScoringService) UpdateWeights(weights *ScoreWeights) error {
	// Validate weights sum to 1.0
	sum := weights.ResponseSpeed + weights.ModelEfficiency +
		weights.CostEffectiveness + weights.Capability + weights.Recency

	if math.Abs(sum-1.0) > 0.001 {
		return fmt.Errorf("weights must sum to 1.0, got %.3f", sum)
	}

	s.weights = weights

	// Clear cache when weights change
	s.cacheMu.Lock()
	s.cache = make(map[string]*ScoringResult)
	s.cacheMu.Unlock()

	return nil
}

// GetWeights returns the current scoring weights
func (s *ScoringService) GetWeights() *ScoreWeights {
	return s.weights
}

// GetModelNameWithScore returns model name with score suffix
func (s *ScoringService) GetModelNameWithScore(ctx context.Context, modelID, modelName string) (string, error) {
	score, err := s.CalculateScore(ctx, modelID)
	if err != nil {
		return modelName, err
	}

	return fmt.Sprintf("%s %s", modelName, score.ScoreSuffix), nil
}

// InvalidateCache invalidates the score cache for a model
func (s *ScoringService) InvalidateCache(modelID string) {
	s.cacheMu.Lock()
	delete(s.cache, modelID)
	s.cacheMu.Unlock()
}

// InvalidateAllCache clears all cached scores
func (s *ScoringService) InvalidateAllCache() {
	s.cacheMu.Lock()
	s.cache = make(map[string]*ScoringResult)
	s.cacheMu.Unlock()
}

// DefaultWeights returns the default scoring weights
func DefaultWeights() *ScoreWeights {
	return &ScoreWeights{
		ResponseSpeed:     0.25,
		ModelEfficiency:   0.20,
		CostEffectiveness: 0.25,
		Capability:        0.20,
		Recency:           0.10,
	}
}
