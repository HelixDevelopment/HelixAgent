// Package verifier provides integration with LLMsVerifier for model verification,
// scoring, and health monitoring capabilities.
package verifier

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// EnhancedScoreComponents represents the 7-component scoring system
// Based on research documents 001-004 for comprehensive debate team selection
type EnhancedScoreComponents struct {
	// Original 5 components
	ResponseSpeed     float64 `json:"response_speed"`     // 20% - API latency measurement
	ModelEfficiency   float64 `json:"model_efficiency"`   // 15% - Token efficiency
	CostEffectiveness float64 `json:"cost_effectiveness"` // 20% - Cost per 1K tokens (higher = cheaper)
	Capability        float64 `json:"capability"`         // 15% - Model capability tier
	Recency           float64 `json:"recency"`            // 5% - Model release date

	// New components from research (Phase 1)
	CodeQuality    float64 `json:"code_quality"`    // 15% - Code generation benchmarks
	ReasoningScore float64 `json:"reasoning_score"` // 10% - Reasoning task performance
}

// EnhancedScoreWeights represents weights for the 7-component system
type EnhancedScoreWeights struct {
	ResponseSpeed     float64 `json:"response_speed" yaml:"response_speed"`
	ModelEfficiency   float64 `json:"model_efficiency" yaml:"model_efficiency"`
	CostEffectiveness float64 `json:"cost_effectiveness" yaml:"cost_effectiveness"`
	Capability        float64 `json:"capability" yaml:"capability"`
	Recency           float64 `json:"recency" yaml:"recency"`
	CodeQuality       float64 `json:"code_quality" yaml:"code_quality"`
	ReasoningScore    float64 `json:"reasoning_score" yaml:"reasoning_score"`
}

// EnhancedScoringResult represents comprehensive scoring for debate team selection
type EnhancedScoringResult struct {
	ModelID           string                  `json:"model_id"`
	ModelName         string                  `json:"model_name"`
	Provider          string                  `json:"provider"`
	OverallScore      float64                 `json:"overall_score"`
	NormalizedScore   float64                 `json:"normalized_score"` // 0-100 scale
	ScoreSuffix       string                  `json:"score_suffix"`
	Components        EnhancedScoreComponents `json:"components"`
	Weights           EnhancedScoreWeights    `json:"weights"`
	CalculatedAt      time.Time               `json:"calculated_at"`
	DataSource        string                  `json:"data_source"`
	ConfidenceScore   float64                 `json:"confidence_score"`   // For weighted voting
	DiversityBonus    float64                 `json:"diversity_bonus"`    // For diverse team selection
	SpecializationTag string                  `json:"specialization_tag"` // code, reasoning, general, etc.

	// Verification data
	VerificationTests map[string]bool `json:"verification_tests,omitempty"`
	Latency           time.Duration   `json:"latency,omitempty"`
}

// EnhancedScoringService provides advanced scoring for debate team selection.
//
// Concurrent-safe by construction: cache and providerScores are safe.Store.
// Both cacheMu and providerScoresMu dropped entirely. The previous
// updateProviderScore running-average write-under-lock is now a
// Store.Update closure (atomic read-then-compute-then-store for a single
// key). The diversity-count read over cache is a snapshot via Range.
type EnhancedScoringService struct {
	weights  *EnhancedScoreWeights
	cache    *safe.Store[string, *EnhancedScoringResult]
	cacheTTL time.Duration
	baseSvc  *ScoringService

	// Provider-level aggregations
	providerScores *safe.Store[string, float64]
}

// DefaultEnhancedWeights returns the default 7-component weights
func DefaultEnhancedWeights() *EnhancedScoreWeights {
	return &EnhancedScoreWeights{
		ResponseSpeed:     0.20,
		ModelEfficiency:   0.15,
		CostEffectiveness: 0.20,
		Capability:        0.15,
		Recency:           0.05,
		CodeQuality:       0.15,
		ReasoningScore:    0.10,
	}
}

// NewEnhancedScoringService creates a new enhanced scoring service
func NewEnhancedScoringService(baseSvc *ScoringService) *EnhancedScoringService {
	return &EnhancedScoringService{
		weights:        DefaultEnhancedWeights(),
		cache:          safe.NewStore[string, *EnhancedScoringResult](),
		cacheTTL:       6 * time.Hour,
		baseSvc:        baseSvc,
		providerScores: safe.NewStore[string, float64](),
	}
}

// CalculateEnhancedScore calculates comprehensive score for debate team selection
func (es *EnhancedScoringService) CalculateEnhancedScore(ctx context.Context, model *UnifiedModel, provider *UnifiedProvider) (*EnhancedScoringResult, error) {
	cacheKey := provider.ID + ":" + model.ID

	// Check cache
	if cached, ok := es.cache.Get(cacheKey); ok {
		if time.Since(cached.CalculatedAt) < es.cacheTTL {
			return cached, nil
		}
	}

	// Calculate individual components
	components := es.calculateComponents(model, provider)

	// Calculate weighted overall score
	overallScore := es.computeWeightedScore(components)

	// Determine specialization
	specialization := es.determineSpecialization(model, components)

	// Calculate confidence score (for weighted voting in debates)
	confidenceScore := es.calculateConfidenceScore(model, provider)

	// Calculate diversity bonus
	diversityBonus := es.calculateDiversityBonus(provider.ID, model.ID)

	result := &EnhancedScoringResult{
		ModelID:           model.ID,
		ModelName:         model.Name,
		Provider:          provider.ID,
		OverallScore:      overallScore,
		NormalizedScore:   overallScore * 10, // Convert to 0-100
		ScoreSuffix:       formatScoreSuffix(overallScore),
		Components:        components,
		Weights:           *es.weights,
		CalculatedAt:      time.Now(),
		DataSource:        "enhanced",
		ConfidenceScore:   confidenceScore,
		DiversityBonus:    diversityBonus,
		SpecializationTag: specialization,
		VerificationTests: model.TestResults,
		Latency:           model.Latency,
	}

	// Update cache
	es.cache.Put(cacheKey, result)

	// Update provider average
	es.updateProviderScore(provider.ID, overallScore)

	return result, nil
}

// calculateComponents calculates all 7 score components
func (es *EnhancedScoringService) calculateComponents(model *UnifiedModel, provider *UnifiedProvider) EnhancedScoreComponents {
	costScore := es.calculateCostScore(model)

	// Apply subscription type modifier when pricing data is unavailable
	if provider.Subscription != nil && model.CostPerInputToken == 0 && model.CostPerOutputToken == 0 {
		switch provider.Subscription.Type {
		case SubTypeFreeCredits:
			costScore -= 0.5 // May expire soon
		case SubTypeFreeTier:
			costScore -= 0.3 // Rate limited
		case SubTypeFree:
			// No penalty for genuinely free (Ollama, Groq free)
		case SubTypeEnterprise:
			costScore += 0.3 // Most reliable
		}
		// Clamp to valid range
		if costScore < 0 {
			costScore = 0
		}
		if costScore > 10 {
			costScore = 10
		}
	}

	return EnhancedScoreComponents{
		ResponseSpeed:     es.calculateResponseSpeedScore(model),
		ModelEfficiency:   es.calculateModelEfficiencyScore(model),
		CostEffectiveness: costScore,
		Capability:        es.calculateCapabilityScore(model, provider),
		Recency:           es.calculateRecencyScore(model),
		CodeQuality:       es.calculateCodeQualityScore(model),
		ReasoningScore:    es.calculateReasoningScore(model),
	}
}

// calculateResponseSpeedScore calculates speed score based on latency
func (es *EnhancedScoringService) calculateResponseSpeedScore(model *UnifiedModel) float64 {
	if model.Latency == 0 {
		return 7.0 // Default for unknown latency
	}

	latencyMs := model.Latency.Milliseconds()

	// Score based on latency ranges (faster = better)
	switch {
	case latencyMs < 200:
		return 10.0
	case latencyMs < 500:
		return 9.0
	case latencyMs < 1000:
		return 8.0
	case latencyMs < 2000:
		return 7.0
	case latencyMs < 5000:
		return 6.0
	case latencyMs < 10000:
		return 5.0
	default:
		return 4.0
	}
}

// calculateModelEfficiencyScore calculates token efficiency score
func (es *EnhancedScoringService) calculateModelEfficiencyScore(model *UnifiedModel) float64 {
	// Infer efficiency from model characteristics
	modelLower := strings.ToLower(model.ID)

	// High efficiency models
	if containsIgnoreCase(modelLower, "turbo") || containsIgnoreCase(modelLower, "flash") ||
		containsIgnoreCase(modelLower, "instant") {
		return 9.0
	}

	// Medium-high efficiency
	if containsIgnoreCase(modelLower, "mini") || containsIgnoreCase(modelLower, "haiku") ||
		containsIgnoreCase(modelLower, "small") {
		return 8.5
	}

	// Standard efficiency
	if containsIgnoreCase(modelLower, "pro") || containsIgnoreCase(modelLower, "sonnet") {
		return 7.5
	}

	// Large models (lower efficiency due to size)
	if containsIgnoreCase(modelLower, "opus") || containsIgnoreCase(modelLower, "ultra") ||
		containsIgnoreCase(modelLower, "405b") {
		return 6.5
	}

	return 7.0 // Default
}

// calculateCostScore calculates cost effectiveness (higher = cheaper)
func (es *EnhancedScoringService) calculateCostScore(model *UnifiedModel) float64 {
	// Check pricing data if available
	if model.CostPerInputToken > 0 || model.CostPerOutputToken > 0 {
		avgCost := (model.CostPerInputToken + model.CostPerOutputToken) / 2
		// Invert so lower cost = higher score
		if avgCost < 0.001 {
			return 10.0
		} else if avgCost < 0.005 {
			return 9.0
		} else if avgCost < 0.01 {
			return 8.0
		} else if avgCost < 0.05 {
			return 7.0
		} else if avgCost < 0.1 {
			return 6.0
		}
		return 5.0
	}

	// Infer from model name
	modelLower := strings.ToLower(model.ID)

	// Free/very cheap models
	if containsIgnoreCase(modelLower, "free") || containsIgnoreCase(modelLower, "nano") ||
		containsIgnoreCase(modelLower, "llama") || containsIgnoreCase(modelLower, "mistral-7b") {
		return 9.5
	}

	// Cheap models
	if containsIgnoreCase(modelLower, "haiku") || containsIgnoreCase(modelLower, "flash") ||
		containsIgnoreCase(modelLower, "mini") || containsIgnoreCase(modelLower, "small") {
		return 8.5
	}

	// Medium cost
	if containsIgnoreCase(modelLower, "turbo") || containsIgnoreCase(modelLower, "sonnet") {
		return 7.0
	}

	// Expensive models
	if containsIgnoreCase(modelLower, "opus") || containsIgnoreCase(modelLower, "ultra") ||
		containsIgnoreCase(modelLower, "4o") || containsIgnoreCase(modelLower, "pro-max") {
		return 5.5
	}

	return 7.0 // Default
}

// calculateCapabilityScore calculates model capability tier
func (es *EnhancedScoringService) calculateCapabilityScore(model *UnifiedModel, provider *UnifiedProvider) float64 {
	modelLower := strings.ToLower(model.ID)

	// Flagship/Ultra tier
	if containsIgnoreCase(modelLower, "opus") || containsIgnoreCase(modelLower, "ultra") ||
		containsIgnoreCase(modelLower, "4o") || containsIgnoreCase(modelLower, "405b") ||
		containsIgnoreCase(modelLower, "grok-3") {
		return 9.5
	}

	// Pro tier
	if containsIgnoreCase(modelLower, "sonnet") || containsIgnoreCase(modelLower, "pro") ||
		containsIgnoreCase(modelLower, "large") || containsIgnoreCase(modelLower, "70b") ||
		containsIgnoreCase(modelLower, "grok-2") {
		return 8.5
	}

	// Standard tier
	if containsIgnoreCase(modelLower, "turbo") || containsIgnoreCase(modelLower, "chat") ||
		containsIgnoreCase(modelLower, "standard") || containsIgnoreCase(modelLower, "8b") {
		return 7.5
	}

	// Fast/Lite tier
	if containsIgnoreCase(modelLower, "haiku") || containsIgnoreCase(modelLower, "flash") ||
		containsIgnoreCase(modelLower, "mini") || containsIgnoreCase(modelLower, "instant") {
		return 7.0
	}

	// Provider tier bonus
	if provider.Tier <= 2 {
		return 8.0
	} else if provider.Tier <= 4 {
		return 7.0
	}

	return 6.5 // Default
}

// calculateRecencyScore calculates score based on model release date
func (es *EnhancedScoringService) calculateRecencyScore(model *UnifiedModel) float64 {
	modelLower := strings.ToLower(model.ID)

	// 2026 models (newest)
	if containsIgnoreCase(modelLower, "2026") || containsIgnoreCase(modelLower, "v4") ||
		containsIgnoreCase(modelLower, "grok-3") {
		return 10.0
	}

	// 2025 models
	if containsIgnoreCase(modelLower, "2025") || containsIgnoreCase(modelLower, "4.5") ||
		containsIgnoreCase(modelLower, "1.5") || containsIgnoreCase(modelLower, "grok-2") {
		return 9.0
	}

	// 2024 models
	if containsIgnoreCase(modelLower, "2024") || containsIgnoreCase(modelLower, "3.5") ||
		containsIgnoreCase(modelLower, "2.0") {
		return 8.0
	}

	// 2023 and older
	if containsIgnoreCase(modelLower, "2023") || containsIgnoreCase(modelLower, "3.0") {
		return 6.5
	}

	// Version number inference
	if containsIgnoreCase(modelLower, "-4") || containsIgnoreCase(modelLower, "v4") {
		return 9.0
	}
	if containsIgnoreCase(modelLower, "-3") || containsIgnoreCase(modelLower, "v3") {
		return 8.0
	}
	if containsIgnoreCase(modelLower, "-2") || containsIgnoreCase(modelLower, "v2") {
		return 7.0
	}

	return 7.5 // Default
}

// calculateCodeQualityScore calculates code generation benchmark score
func (es *EnhancedScoringService) calculateCodeQualityScore(model *UnifiedModel) float64 {
	modelLower := strings.ToLower(model.ID)

	// Specialized code models (highest scores)
	if containsIgnoreCase(modelLower, "coder") || containsIgnoreCase(modelLower, "codestral") ||
		containsIgnoreCase(modelLower, "deepseek-coder") || containsIgnoreCase(modelLower, "code") {
		return 9.5
	}

	// Models known for good code generation
	if containsIgnoreCase(modelLower, "opus") || containsIgnoreCase(modelLower, "grok") ||
		containsIgnoreCase(modelLower, "4o") || containsIgnoreCase(modelLower, "sonnet") {
		return 9.0
	}

	// Good general models with code capability
	if containsIgnoreCase(modelLower, "pro") || containsIgnoreCase(modelLower, "turbo") ||
		containsIgnoreCase(modelLower, "instruct") {
		return 8.0
	}

	// Check for code capability in metadata
	if model.TestResults != nil && model.TestResults["code_visibility"] {
		return 8.0
	}

	// Check capabilities slice
	for _, cap := range model.Capabilities {
		if containsIgnoreCase(cap, "code") {
			return 7.5
		}
	}

	return 6.5 // Default
}

// calculateReasoningScore calculates reasoning task performance score
func (es *EnhancedScoringService) calculateReasoningScore(model *UnifiedModel) float64 {
	modelLower := strings.ToLower(model.ID)

	// Specialized reasoning models
	if containsIgnoreCase(modelLower, "reasoner") || containsIgnoreCase(modelLower, "reasoning") ||
		containsIgnoreCase(modelLower, "think") {
		return 9.5
	}

	// Models known for strong reasoning
	if containsIgnoreCase(modelLower, "opus") || containsIgnoreCase(modelLower, "4o") ||
		containsIgnoreCase(modelLower, "ultra") || containsIgnoreCase(modelLower, "grok-3") {
		return 9.0
	}

	// Good reasoning capability
	if containsIgnoreCase(modelLower, "sonnet") || containsIgnoreCase(modelLower, "pro") ||
		containsIgnoreCase(modelLower, "large") || containsIgnoreCase(modelLower, "70b") {
		return 8.5
	}

	// Standard reasoning
	if containsIgnoreCase(modelLower, "turbo") || containsIgnoreCase(modelLower, "chat") {
		return 7.5
	}

	// Fast models (lower reasoning due to optimization)
	if containsIgnoreCase(modelLower, "haiku") || containsIgnoreCase(modelLower, "flash") ||
		containsIgnoreCase(modelLower, "instant") || containsIgnoreCase(modelLower, "mini") {
		return 6.5
	}

	return 7.0 // Default
}

// computeWeightedScore calculates the final weighted score
func (es *EnhancedScoringService) computeWeightedScore(components EnhancedScoreComponents) float64 {
	score := components.ResponseSpeed*es.weights.ResponseSpeed +
		components.ModelEfficiency*es.weights.ModelEfficiency +
		components.CostEffectiveness*es.weights.CostEffectiveness +
		components.Capability*es.weights.Capability +
		components.Recency*es.weights.Recency +
		components.CodeQuality*es.weights.CodeQuality +
		components.ReasoningScore*es.weights.ReasoningScore

	// Normalize to 0-10 scale
	return math.Min(10.0, math.Max(0.0, score))
}

// determineSpecialization determines the model's specialization based on scores
func (es *EnhancedScoringService) determineSpecialization(model *UnifiedModel, components EnhancedScoreComponents) string {
	// Check model name for explicit specialization
	modelLower := strings.ToLower(model.ID)

	if containsIgnoreCase(modelLower, "coder") || containsIgnoreCase(modelLower, "code") {
		return "code"
	}
	if containsIgnoreCase(modelLower, "reasoner") || containsIgnoreCase(modelLower, "reasoning") {
		return "reasoning"
	}
	if containsIgnoreCase(modelLower, "vision") || containsIgnoreCase(modelLower, "visual") {
		return "vision"
	}
	if containsIgnoreCase(modelLower, "online") || containsIgnoreCase(modelLower, "search") {
		return "search"
	}
	if containsIgnoreCase(modelLower, "embed") {
		return "embedding"
	}

	// Infer from component scores
	if components.CodeQuality >= 9.0 {
		return "code"
	}
	if components.ReasoningScore >= 9.0 {
		return "reasoning"
	}
	if components.ResponseSpeed >= 9.0 {
		return "speed"
	}
	if components.CostEffectiveness >= 9.0 {
		return "economy"
	}

	return "general"
}

// calculateConfidenceScore calculates confidence score for weighted voting
// Based on Document 003: Weighted voting formula L* = argmax Σcᵢ · 𝟙[aᵢ = L]
func (es *EnhancedScoringService) calculateConfidenceScore(model *UnifiedModel, provider *UnifiedProvider) float64 {
	baseConfidence := 0.7 // Start with decent confidence

	// Verification status boost
	if model.Verified {
		baseConfidence += 0.1
	}

	// Test results boost
	if model.TestResults != nil {
		passedTests := 0
		totalTests := len(model.TestResults)
		for _, passed := range model.TestResults {
			if passed {
				passedTests++
			}
		}
		if totalTests > 0 {
			testRatio := float64(passedTests) / float64(totalTests)
			baseConfidence += testRatio * 0.1
		}
	}

	// Provider tier boost
	if provider.Tier <= 2 {
		baseConfidence += 0.05
	}

	// OAuth provider boost (more reliable)
	if provider.AuthType == AuthTypeOAuth {
		baseConfidence += 0.05
	}

	// Latency reliability boost
	if model.Latency > 0 && model.Latency < 2*time.Second {
		baseConfidence += 0.05
	}

	return math.Min(1.0, baseConfidence)
}

// calculateDiversityBonus calculates diversity bonus for team selection
// Based on Document 003: "Productive Chaos" philosophy with diversity metric.
//
// Ranges over es.cache via safe.Store.Range — the Store's own lock
// serialises this scan with concurrent CalculateEnhancedScore writes.
// Historical BUGFIX #38 (wrong-lock) no longer applicable after the
// CONST-029 migration: there is only one lock (the Store's internal
// one) and it is correctly held during both Range and Put.
func (es *EnhancedScoringService) calculateDiversityBonus(providerID, modelID string) float64 {
	// Count how many models from this provider are already in cache
	providerCount := 0
	prefix := providerID + ":"
	es.cache.Range(func(key string, _ *EnhancedScoringResult) bool {
		if strings.HasPrefix(key, prefix) {
			providerCount++
		}
		return true
	})

	// Less represented providers get bonus (encourages diversity)
	switch {
	case providerCount == 0:
		return 0.5 // First model from provider gets bonus
	case providerCount == 1:
		return 0.2
	case providerCount == 2:
		return 0.1
	default:
		return 0.0 // No bonus for overrepresented providers
	}
}

// updateProviderScore updates the average score for a provider.
// Store.Update is an atomic read-compute-write for a single key, so
// two concurrent callers for the same providerID compute their
// running-average against each other's latest value, not a stale one.
func (es *EnhancedScoringService) updateProviderScore(providerID string, score float64) {
	es.providerScores.Update(providerID, func(current float64, present bool) (float64, bool) {
		if present {
			return (current + score) / 2, true
		}
		return score, true
	})
}

// GetTopScoringModels returns the top N models by enhanced score
func (es *EnhancedScoringService) GetTopScoringModels(limit int) []*EnhancedScoringResult {
	results := es.cache.Values()

	// Sort by overall score (with diversity bonus)
	sort.Slice(results, func(i, j int) bool {
		scoreI := results[i].OverallScore + results[i].DiversityBonus
		scoreJ := results[j].OverallScore + results[j].DiversityBonus
		return scoreI > scoreJ
	})

	if limit > len(results) {
		limit = len(results)
	}

	return results[:limit]
}

// GetModelsBySpecialization returns models with a specific specialization
func (es *EnhancedScoringService) GetModelsBySpecialization(specialization string, limit int) []*EnhancedScoringResult {
	results := make([]*EnhancedScoringResult, 0)
	es.cache.Range(func(_ string, result *EnhancedScoringResult) bool {
		if result.SpecializationTag == specialization {
			results = append(results, result)
		}
		return true
	})

	// Sort by overall score
	sort.Slice(results, func(i, j int) bool {
		return results[i].OverallScore > results[j].OverallScore
	})

	if limit > len(results) {
		limit = len(results)
	}

	return results[:limit]
}

// SelectDebateTeamFromScores selects the optimal 12 LLMs for debate from scored models
// Based on research: 12 specialized agents for comprehensive debate
func (es *EnhancedScoringService) SelectDebateTeamFromScores(minScore float64) []*EnhancedScoringResult {
	// Filter by minimum score (snapshot via Range)
	eligible := make([]*EnhancedScoringResult, 0)
	es.cache.Range(func(_ string, result *EnhancedScoringResult) bool {
		if result.OverallScore >= minScore {
			eligible = append(eligible, result)
		}
		return true
	})

	// Sort by score with diversity bonus
	sort.Slice(eligible, func(i, j int) bool {
		scoreI := eligible[i].OverallScore + eligible[i].DiversityBonus
		scoreJ := eligible[j].OverallScore + eligible[j].DiversityBonus
		return scoreI > scoreJ
	})

	// Select up to 12 LLMs with diversity constraints
	selected := make([]*EnhancedScoringResult, 0, 12)
	providerCounts := make(map[string]int)
	maxPerProvider := 3 // Ensure diversity

	for _, result := range eligible {
		if len(selected) >= 12 {
			break
		}

		// Check provider diversity
		if providerCounts[result.Provider] >= maxPerProvider {
			continue
		}

		selected = append(selected, result)
		providerCounts[result.Provider]++
	}

	return selected
}

// CalculateWeightedVote calculates weighted vote for a response
// Formula from Document 003: L* = argmax Σcᵢ · 𝟙[aᵢ = L]
func (es *EnhancedScoringService) CalculateWeightedVote(responses map[string]string, confidences map[string]float64) string {
	// Group responses by content
	votes := make(map[string]float64)

	for modelID, response := range responses {
		confidence, ok := confidences[modelID]
		if !ok {
			confidence = 0.5 // Default confidence
		}
		votes[response] += confidence
	}

	// Find response with highest weighted vote
	var bestResponse string
	var bestScore float64

	for response, score := range votes {
		if score > bestScore {
			bestScore = score
			bestResponse = response
		}
	}

	return bestResponse
}

// Helper function
func formatScoreSuffix(score float64) string {
	return "(ESC:" + strings.TrimRight(strings.TrimRight(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(
						strings.Replace(
							strings.Replace(
								strings.Replace(
									strings.Replace(
										strings.Replace(
											strings.Replace(
												"X.XX",
												"X", strconv.Itoa(int(score)), 1),
											".", ".", 1),
										"X", strconv.Itoa(int(score*10)%10), 1),
									"X", strconv.Itoa(int(score*100)%10), 1),
								"X", "", -1),
							"", "", -1),
						"", "", -1),
					"", "", -1),
				"", "", -1),
			"", "", -1),
		"0"), ".") + ")"
}

// Note: containsIgnoreCase is already defined in discovery.go
