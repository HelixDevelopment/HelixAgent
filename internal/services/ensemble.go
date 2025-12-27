package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// EnsembleService manages multiple LLM providers and implements voting strategies
type EnsembleService struct {
	providers map[string]LLMProvider
	strategy  string
	timeout   time.Duration
	mu        sync.RWMutex
}

// LLMProvider interface for all LLM providers
type LLMProvider interface {
	Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)
}

// EnsembleResult contains the results from ensemble voting
type EnsembleResult struct {
	Responses    []*models.LLMResponse
	Selected     *models.LLMResponse
	VotingMethod string
	Scores       map[string]float64
	Metadata     map[string]any
}

// VotingStrategy defines different voting strategies
type VotingStrategy interface {
	Vote(responses []*models.LLMResponse, req *models.LLMRequest) (*models.LLMResponse, map[string]float64, error)
}

// ConfidenceWeightedStrategy implements confidence-weighted voting
type ConfidenceWeightedStrategy struct{}

// MajorityVoteStrategy implements majority voting
type MajorityVoteStrategy struct{}

// QualityWeightedStrategy implements quality-based voting
type QualityWeightedStrategy struct{}

func NewEnsembleService(strategy string, timeout time.Duration) *EnsembleService {
	return &EnsembleService{
		providers: make(map[string]LLMProvider),
		strategy:  strategy,
		timeout:   timeout,
	}
}

func (e *EnsembleService) RegisterProvider(name string, provider LLMProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.providers[name] = provider
}

func (e *EnsembleService) RemoveProvider(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.providers, name)
}

func (e *EnsembleService) GetProviders() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.providers))
	for name := range e.providers {
		names = append(names, name)
	}
	return names
}

func (e *EnsembleService) RunEnsemble(ctx context.Context, req *models.LLMRequest) (*EnsembleResult, error) {
	e.mu.RLock()
	providers := make(map[string]LLMProvider)
	for k, v := range e.providers {
		providers[k] = v
	}
	e.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Filter providers based on request preferences
	filteredProviders := e.filterProviders(providers, req)
	if len(filteredProviders) == 0 {
		return nil, fmt.Errorf("no suitable providers available")
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Execute requests concurrently
	responses := make([]*models.LLMResponse, 0, len(filteredProviders))
	responseChan := make(chan *models.LLMResponse, len(filteredProviders))
	errorChan := make(chan error, len(filteredProviders))

	var wg sync.WaitGroup
	for name, provider := range filteredProviders {
		wg.Add(1)
		go func(name string, provider LLMProvider) {
			defer wg.Done()

			resp, err := provider.Complete(timeoutCtx, req)
			if err != nil {
				errorChan <- fmt.Errorf("provider %s failed: %w", name, err)
				return
			}

			// Add provider metadata
			resp.ProviderID = name
			resp.ProviderName = name
			responseChan <- resp
		}(name, provider)
	}

	// Wait for all providers to complete
	go func() {
		wg.Wait()
		close(responseChan)
		close(errorChan)
	}()

	// Collect responses
	for resp := range responseChan {
		responses = append(responses, resp)
	}

	// Check for errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	// If we have some responses, proceed with voting
	if len(responses) > 0 {
		selected, scores, err := e.vote(responses, req)
		if err != nil {
			return nil, fmt.Errorf("voting failed: %w", err)
		}

		return &EnsembleResult{
			Responses:    responses,
			Selected:     selected,
			VotingMethod: e.strategy,
			Scores:       scores,
			Metadata: map[string]any{
				"total_providers":      len(providers),
				"successful_providers": len(responses),
				"failed_providers":     len(errors),
				"errors":               errors,
				"execution_time":       time.Since(time.Now()).Milliseconds(),
			},
		}, nil
	}

	// If no responses and we have errors, return the errors
	if len(errors) > 0 {
		return nil, fmt.Errorf("all providers failed: %v", errors)
	}

	return nil, fmt.Errorf("no responses received from any provider")
}

func (e *EnsembleService) RunEnsembleStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	e.mu.RLock()
	providers := make(map[string]LLMProvider)
	for k, v := range e.providers {
		providers[k] = v
	}
	e.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Filter providers based on request preferences
	filteredProviders := e.filterProviders(providers, req)
	if len(filteredProviders) == 0 {
		return nil, fmt.Errorf("no suitable providers available")
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// For streaming, we'll use the first available provider for now
	// In a more sophisticated implementation, we could merge streams
	for name, provider := range filteredProviders {
		streamChan, err := provider.CompleteStream(timeoutCtx, req)
		if err != nil {
			continue
		}

		// Wrap responses with provider info
		wrappedChan := make(chan *models.LLMResponse)
		go func() {
			defer close(wrappedChan)
			for resp := range streamChan {
				resp.ProviderID = name
				resp.ProviderName = name
				wrappedChan <- resp
			}
		}()

		return wrappedChan, nil
	}

	return nil, fmt.Errorf("no providers available for streaming")
}

func (e *EnsembleService) filterProviders(providers map[string]LLMProvider, req *models.LLMRequest) map[string]LLMProvider {
	filtered := make(map[string]LLMProvider)

	// If no preferred providers specified, use all
	if req.EnsembleConfig == nil || len(req.EnsembleConfig.PreferredProviders) == 0 {
		for k, v := range providers {
			filtered[k] = v
		}
		return filtered
	}

	// Filter by preferred providers
	for _, preferred := range req.EnsembleConfig.PreferredProviders {
		if provider, exists := providers[preferred]; exists {
			filtered[preferred] = provider
		}
	}

	// If we don't have enough preferred providers, add more
	minProviders := 2
	if req.EnsembleConfig != nil && req.EnsembleConfig.MinProviders > 0 {
		minProviders = req.EnsembleConfig.MinProviders
	}

	if len(filtered) < minProviders {
		// Add additional providers until we reach minimum
		for name, provider := range providers {
			if _, exists := filtered[name]; !exists {
				filtered[name] = provider
				if len(filtered) >= minProviders {
					break
				}
			}
		}
	}

	return filtered
}

func (e *EnsembleService) vote(responses []*models.LLMResponse, req *models.LLMRequest) (*models.LLMResponse, map[string]float64, error) {
	var strategy VotingStrategy

	switch e.strategy {
	case "confidence_weighted":
		strategy = &ConfidenceWeightedStrategy{}
	case "majority_vote":
		strategy = &MajorityVoteStrategy{}
	case "quality_weighted":
		strategy = &QualityWeightedStrategy{}
	default:
		strategy = &ConfidenceWeightedStrategy{} // Default
	}

	return strategy.Vote(responses, req)
}

// ConfidenceWeightedStrategy implementation
func (s *ConfidenceWeightedStrategy) Vote(responses []*models.LLMResponse, req *models.LLMRequest) (*models.LLMResponse, map[string]float64, error) {
	if len(responses) == 0 {
		return nil, nil, fmt.Errorf("no responses to vote on")
	}

	scores := make(map[string]float64)

	// Calculate weighted scores
	for _, resp := range responses {
		// Base score from confidence
		score := resp.Confidence

		// Apply weights based on response characteristics
		score = s.applyQualityWeights(resp, score)

		// Apply provider-specific weights if available
		if req.EnsembleConfig != nil && len(req.EnsembleConfig.PreferredProviders) > 0 {
			for i, preferred := range req.EnsembleConfig.PreferredProviders {
				if resp.ProviderName == preferred {
					// Higher weight for more preferred providers
					weight := 1.0 + (float64(len(req.EnsembleConfig.PreferredProviders)-i) * 0.1)
					score *= weight
					break
				}
			}
		}

		scores[resp.ID] = score
	}

	// Select response with highest score
	var selected *models.LLMResponse
	maxScore := -1.0

	for _, resp := range responses {
		if score, exists := scores[resp.ID]; exists && score > maxScore {
			maxScore = score
			selected = resp
		}
	}

	if selected == nil {
		return nil, scores, fmt.Errorf("failed to select response")
	}

	// Mark as selected
	for _, resp := range responses {
		resp.Selected = (resp.ID == selected.ID)
		resp.SelectionScore = scores[resp.ID]
	}

	return selected, scores, nil
}

func (s *ConfidenceWeightedStrategy) applyQualityWeights(resp *models.LLMResponse, baseScore float64) float64 {
	score := baseScore

	// Length factor - prefer responses with reasonable length
	contentLength := len(resp.Content)
	if contentLength > 50 && contentLength < 1000 {
		score *= 1.1
	} else if contentLength >= 1000 && contentLength < 2000 {
		score *= 1.05
	} else if contentLength >= 2000 {
		score *= 0.95 // Too long might be verbose
	}

	// Response time factor - prefer faster responses
	if resp.ResponseTime < 1000 { // < 1 second
		score *= 1.1
	} else if resp.ResponseTime < 3000 { // < 3 seconds
		score *= 1.05
	} else if resp.ResponseTime > 10000 { // > 10 seconds
		score *= 0.9
	}

	// Token efficiency factor
	if resp.TokensUsed > 0 {
		efficiency := float64(len(resp.Content)) / float64(resp.TokensUsed)
		if efficiency > 3.0 {
			score *= 1.1
		} else if efficiency > 2.0 {
			score *= 1.05
		}
	}

	// Finish reason factor
	switch resp.FinishReason {
	case "stop":
		score *= 1.1
	case "length":
		score *= 0.95
	case "content_filter":
		score *= 0.7
	}

	return score
}

// MajorityVoteStrategy implementation
func (s *MajorityVoteStrategy) Vote(responses []*models.LLMResponse, req *models.LLMRequest) (*models.LLMResponse, map[string]float64, error) {
	if len(responses) == 0 {
		return nil, nil, fmt.Errorf("no responses to vote on")
	}

	// For majority voting, we'll use content similarity as a proxy
	// This is a simplified implementation - in practice, you'd use semantic similarity
	scores := make(map[string]float64)

	// Group similar responses
	responseGroups := make(map[string][]*models.LLMResponse)
	for _, resp := range responses {
		// Use first 100 characters as a simple grouping key
		key := resp.Content
		if len(key) > 100 {
			key = key[:100]
		}
		responseGroups[key] = append(responseGroups[key], resp)
	}

	// Find the largest group
	var largestGroup []*models.LLMResponse
	for _, group := range responseGroups {
		if len(group) > len(largestGroup) {
			largestGroup = group
		}
	}

	// If we have a clear majority, use it
	if len(largestGroup) > len(responses)/2 {
		// Select the highest confidence response from the majority group
		var selected *models.LLMResponse
		maxConfidence := 0.0

		for _, resp := range largestGroup {
			scores[resp.ID] = 1.0 // Majority vote score
			if resp.Confidence > maxConfidence {
				maxConfidence = resp.Confidence
				selected = resp
			}
		}

		// Mark all responses
		for _, resp := range responses {
			resp.Selected = (resp.ID == selected.ID)
			resp.SelectionScore = scores[resp.ID]
		}

		return selected, scores, nil
	}

	// No clear majority, fall back to confidence weighted
	fallback := &ConfidenceWeightedStrategy{}
	return fallback.Vote(responses, req)
}

// QualityWeightedStrategy implementation
func (s *QualityWeightedStrategy) Vote(responses []*models.LLMResponse, req *models.LLMRequest) (*models.LLMResponse, map[string]float64, error) {
	if len(responses) == 0 {
		return nil, nil, fmt.Errorf("no responses to vote on")
	}

	scores := make(map[string]float64)

	for _, resp := range responses {
		// Calculate quality score based on multiple factors
		score := s.calculateQualityScore(resp)
		scores[resp.ID] = score
	}

	// Select response with highest quality score
	var selected *models.LLMResponse
	maxScore := -1.0

	for _, resp := range responses {
		if score, exists := scores[resp.ID]; exists && score > maxScore {
			maxScore = score
			selected = resp
		}
	}

	// Mark all responses
	for _, resp := range responses {
		resp.Selected = (resp.ID == selected.ID)
		resp.SelectionScore = scores[resp.ID]
	}

	return selected, scores, nil
}

func (s *QualityWeightedStrategy) calculateQualityScore(resp *models.LLMResponse) float64 {
	score := 0.0

	// Confidence factor (30% weight)
	score += resp.Confidence * 0.3

	// Response time factor (20% weight)
	timeScore := math.Max(0, 1.0-(float64(resp.ResponseTime)/10000.0))
	score += timeScore * 0.2

	// Token efficiency factor (20% weight)
	if resp.TokensUsed > 0 {
		efficiency := math.Min(1.0, float64(len(resp.Content))/float64(resp.TokensUsed))
		score += efficiency * 0.2
	}

	// Content length factor (15% weight)
	lengthScore := 0.0
	contentLength := len(resp.Content)
	if contentLength > 50 && contentLength < 1000 {
		lengthScore = 1.0
	} else if contentLength >= 1000 && contentLength < 2000 {
		lengthScore = 0.8
	} else if contentLength >= 2000 {
		lengthScore = 0.6
	} else {
		lengthScore = 0.4
	}
	score += lengthScore * 0.15

	// Finish reason factor (15% weight)
	finishScore := 0.0
	switch resp.FinishReason {
	case "stop":
		finishScore = 1.0
	case "length":
		finishScore = 0.7
	case "content_filter":
		finishScore = 0.3
	default:
		finishScore = 0.5
	}
	score += finishScore * 0.15

	return score
}

// Legacy methods for backward compatibility
func (e *EnsembleService) ProcessEnsemble(ctx context.Context, req *models.LLMRequest, responses []*models.LLMResponse) (*models.LLMResponse, error) {
	if req.EnsembleConfig == nil {
		// No ensemble, return best response
		return e.selectBestResponse(responses), nil
	}

	config := req.EnsembleConfig

	switch config.Strategy {
	case "confidence_weighted":
		return e.confidenceWeightedVoting(responses, config), nil
	case "majority_vote":
		return e.majorityVoting(responses, config), nil
	default:
		return e.selectBestResponse(responses), nil
	}
}

func (e *EnsembleService) confidenceWeightedVoting(responses []*models.LLMResponse, config *models.EnsembleConfig) *models.LLMResponse {
	if len(responses) == 0 {
		return nil
	}

	// Sort by confidence
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Confidence > responses[j].Confidence
	})

	// Select highest confidence if above threshold
	if responses[0].Confidence >= config.ConfidenceThreshold {
		responses[0].Selected = true
		responses[0].SelectionScore = responses[0].Confidence
		return responses[0]
	}

	// Fallback to best
	if config.FallbackToBest {
		responses[0].Selected = true
		responses[0].SelectionScore = responses[0].Confidence
		return responses[0]
	}

	return nil
}

func (e *EnsembleService) majorityVoting(responses []*models.LLMResponse, _ *models.EnsembleConfig) *models.LLMResponse {
	// Simplified majority voting
	if len(responses) == 0 {
		return nil
	}

	// For now, return first
	responses[0].Selected = true
	responses[0].SelectionScore = 1.0
	return responses[0]
}

func (e *EnsembleService) selectBestResponse(responses []*models.LLMResponse) *models.LLMResponse {
	if len(responses) == 0 {
		return nil
	}

	best := responses[0]
	for _, resp := range responses[1:] {
		if resp.Confidence > best.Confidence {
			best = resp
		}
	}

	best.Selected = true
	best.SelectionScore = best.Confidence
	return best
}
