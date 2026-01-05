package verifier

import (
	"context"
	"fmt"
	"math"
	"math/rand"
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

// calculateSpeedScore calculates speed score based on model characteristics
func (s *ScoringService) calculateSpeedScore(modelID string) float64 {
	// Fast models known for speed
	fastModels := map[string]float64{
		"groq":      9.0,
		"gpt-3.5":   8.0,
		"claude-3-haiku": 8.5,
		"gemini-flash": 8.5,
	}

	// Standard speed models
	standardModels := map[string]float64{
		"gpt-4":     7.5,
		"claude-3":  7.5,
		"gemini":    7.0,
	}

	for pattern, score := range fastModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Min(10, score+rand.Float64())
		}
	}

	for pattern, score := range standardModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Min(10, score+rand.Float64()*0.5)
		}
	}

	// Default score for unknown models
	return 5.0 + rand.Float64()*2
}

// calculateEfficiencyScore calculates efficiency score based on model characteristics
func (s *ScoringService) calculateEfficiencyScore(modelID string) float64 {
	// Efficient models
	efficientModels := map[string]float64{
		"gpt-4o":    9.0,
		"claude-3.5": 9.0,
		"gemini-pro": 8.0,
		"llama":     7.5,
	}

	for pattern, score := range efficientModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Min(10, score+rand.Float64()*0.5)
		}
	}

	// Default efficiency
	return 5.0 + rand.Float64()*3
}

// calculateCostScore calculates cost score (higher = cheaper)
func (s *ScoringService) calculateCostScore(modelID string) float64 {
	// Free/cheap models
	cheapModels := map[string]float64{
		"llama":     9.0,
		"ollama":    10.0,
		"gpt-3.5":   8.0,
		"claude-3-haiku": 8.0,
	}

	// Expensive models
	expensiveModels := map[string]float64{
		"gpt-4":     5.0,
		"claude-3-opus": 4.0,
		"gemini-ultra": 4.5,
	}

	for pattern, score := range cheapModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Min(10, score+rand.Float64()*0.5)
		}
	}

	for pattern, score := range expensiveModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Max(0, score+rand.Float64()*1.5)
		}
	}

	// Default cost score
	return 5.0 + rand.Float64()*2
}

// calculateCapabilityScore calculates capability score based on model characteristics
func (s *ScoringService) calculateCapabilityScore(modelID string) float64 {
	// High capability models
	highCapModels := map[string]float64{
		"gpt-4":         9.5,
		"gpt-4o":        9.5,
		"claude-3-opus": 9.5,
		"claude-3.5":    9.0,
		"gemini-ultra":  9.0,
	}

	// Medium capability models
	medCapModels := map[string]float64{
		"gpt-3.5":       7.0,
		"claude-3-sonnet": 8.0,
		"gemini-pro":    8.0,
		"llama-3":       7.5,
	}

	for pattern, score := range highCapModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Min(10, score+rand.Float64()*0.5)
		}
	}

	for pattern, score := range medCapModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Min(10, score+rand.Float64()*0.5)
		}
	}

	// Default capability
	return 5.0 + rand.Float64()*2
}

// calculateRecencyScore calculates recency score based on model release date
func (s *ScoringService) calculateRecencyScore(modelID string) float64 {
	// Recent models
	recentModels := map[string]float64{
		"gpt-4o":        9.5,
		"gpt-4-turbo":   9.0,
		"claude-3.5":    9.0,
		"gemini-1.5":    9.0,
	}

	// Older models
	olderModels := map[string]float64{
		"gpt-3.5":       6.0,
		"gpt-4":         7.5,
		"claude-3":      8.0,
		"text-davinci":  4.0,
	}

	for pattern, score := range recentModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Min(10, score+rand.Float64()*0.5)
		}
	}

	for pattern, score := range olderModels {
		if containsIgnoreCase(modelID, pattern) {
			return math.Min(10, score+rand.Float64()*0.5)
		}
	}

	// Default recency
	return 5.0 + rand.Float64()*2
}

// GetModelScore retrieves a cached score or calculates a new one
func (s *ScoringService) GetModelScore(ctx context.Context, modelID string) (*ScoringResult, error) {
	s.cacheMu.RLock()
	result, ok := s.cache[modelID]
	s.cacheMu.RUnlock()

	if ok {
		return result, nil
	}

	// Calculate if not in cache
	return s.CalculateScore(ctx, modelID)
}

// ClearCache clears all cached scores (alias for InvalidateAllCache)
func (s *ScoringService) ClearCache() {
	s.InvalidateAllCache()
}

// GetAllScores returns all cached scoring results
func (s *ScoringService) GetAllScores() []*ScoringResult {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	results := make([]*ScoringResult, 0, len(s.cache))
	for _, result := range s.cache {
		results = append(results, result)
	}
	return results
}

// CacheSize returns the number of items in the cache
func (s *ScoringService) CacheSize() int {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return len(s.cache)
}

// computeWeightedScore calculates a weighted score from components
func (s *ScoringService) computeWeightedScore(components ScoreComponents) float64 {
	return components.SpeedScore*s.weights.ResponseSpeed +
		components.EfficiencyScore*s.weights.ModelEfficiency +
		components.CostScore*s.weights.CostEffectiveness +
		components.CapabilityScore*s.weights.Capability +
		components.RecencyScore*s.weights.Recency
}
