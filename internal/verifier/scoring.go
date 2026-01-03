package verifier

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"llm-verifier/database"
	"llm-verifier/scoring"
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
	engine          *scoring.ScoringEngine
	modelsDevClient *scoring.ModelsDevClient
	db              *database.Database
	weights         *ScoreWeights
	cache           map[string]*ScoringResult
	cacheMu         sync.RWMutex
	cacheTTL        time.Duration
}

// NewScoringService creates a new scoring service
func NewScoringService(db *database.Database, cfg *Config) (*ScoringService, error) {
	var modelsDevClient *scoring.ModelsDevClient
	if cfg.Scoring.ModelsDevEnabled {
		modelsDevClient = scoring.NewModelsDevClient(cfg.Scoring.ModelsDevEndpoint)
	}

	engine := scoring.NewScoringEngine(db, modelsDevClient, nil)

	weights := &ScoreWeights{
		ResponseSpeed:     cfg.Scoring.Weights.ResponseSpeed,
		ModelEfficiency:   cfg.Scoring.Weights.ModelEfficiency,
		CostEffectiveness: cfg.Scoring.Weights.CostEffectiveness,
		Capability:        cfg.Scoring.Weights.Capability,
		Recency:           cfg.Scoring.Weights.Recency,
	}

	return &ScoringService{
		engine:          engine,
		modelsDevClient: modelsDevClient,
		db:              db,
		weights:         weights,
		cache:           make(map[string]*ScoringResult),
		cacheTTL:        cfg.Scoring.CacheTTL,
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

	// Calculate new score
	config := scoring.ScoringConfig{
		Weights: scoring.ScoreWeights{
			ResponseSpeed:     s.weights.ResponseSpeed,
			ModelEfficiency:   s.weights.ModelEfficiency,
			CostEffectiveness: s.weights.CostEffectiveness,
			Capability:        s.weights.Capability,
			Recency:           s.weights.Recency,
		},
	}

	score, err := s.engine.CalculateComprehensiveScore(ctx, modelID, config)
	if err != nil {
		// Fallback to basic scoring if models.dev fails
		return s.calculateBasicScore(ctx, modelID)
	}

	result := &ScoringResult{
		ModelID:      modelID,
		ModelName:    score.ModelName,
		OverallScore: score.OverallScore,
		ScoreSuffix:  score.ScoreSuffix,
		Components: ScoreComponents{
			SpeedScore:      score.Components.SpeedScore,
			EfficiencyScore: score.Components.EfficiencyScore,
			CostScore:       score.Components.CostScore,
			CapabilityScore: score.Components.CapabilityScore,
			RecencyScore:    score.Components.RecencyScore,
		},
		CalculatedAt: score.LastCalculated,
		DataSource:   score.DataSource,
	}

	// Update cache
	s.cacheMu.Lock()
	s.cache[modelID] = result
	s.cacheMu.Unlock()

	return result, nil
}

// calculateBasicScore calculates a basic score without models.dev data
func (s *ScoringService) calculateBasicScore(ctx context.Context, modelID string) (*ScoringResult, error) {
	baseScore := 5.0

	// Get model from database if available
	if s.db != nil {
		models, err := s.db.GetModels()
		if err == nil {
			for _, m := range models {
				if m.ModelID == modelID {
					// Use database scores if available
					if m.OverallScore > 0 {
						baseScore = m.OverallScore
					}
					break
				}
			}
		}
	}

	// Ensure score is within bounds
	baseScore = math.Max(0, math.Min(10, baseScore))

	return &ScoringResult{
		ModelID:      modelID,
		ModelName:    modelID,
		OverallScore: baseScore,
		ScoreSuffix:  fmt.Sprintf("(SC:%.1f)", baseScore),
		Components: ScoreComponents{
			SpeedScore:      5.0,
			EfficiencyScore: 5.0,
			CostScore:       5.0,
			CapabilityScore: 5.0,
			RecencyScore:    5.0,
		},
		CalculatedAt: time.Now(),
		DataSource:   "basic",
	}, nil
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
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	models, err := s.engine.GetTopModels(ctx, limit)
	if err != nil {
		return nil, err
	}

	results := make([]*ModelWithScore, len(models))
	for i, m := range models {
		results[i] = &ModelWithScore{
			ModelID:      m.ModelID,
			Name:         m.Name,
			Provider:     m.ProviderName,
			OverallScore: m.OverallScore,
			ScoreSuffix:  fmt.Sprintf("(SC:%.1f)", m.OverallScore),
		}
	}

	return results, nil
}

// GetModelsByScoreRange returns models within a score range
func (s *ScoringService) GetModelsByScoreRange(ctx context.Context, minScore, maxScore float64, limit int) ([]*ModelWithScore, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	models, err := s.engine.GetModelsByScoreRange(ctx, minScore, maxScore, limit)
	if err != nil {
		return nil, err
	}

	results := make([]*ModelWithScore, len(models))
	for i, m := range models {
		results[i] = &ModelWithScore{
			ModelID:      m.ModelID,
			Name:         m.Name,
			Provider:     m.ProviderName,
			OverallScore: m.OverallScore,
			ScoreSuffix:  fmt.Sprintf("(SC:%.1f)", m.OverallScore),
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
	s.engine.SetWeights(scoring.ScoreWeights{
		ResponseSpeed:     weights.ResponseSpeed,
		ModelEfficiency:   weights.ModelEfficiency,
		CostEffectiveness: weights.CostEffectiveness,
		Capability:        weights.Capability,
		Recency:           weights.Recency,
	})

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
