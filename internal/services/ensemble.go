package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
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
		return nil, NewConfigurationError("no providers available", nil)
	}

	// Filter providers based on request preferences
	filteredProviders := e.filterProviders(providers, req)
	if len(filteredProviders) == 0 {
		return nil, NewConfigurationError("no suitable providers available for request", nil)
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

	// If no responses and we have errors, return categorized error
	if len(errors) > 0 {
		return nil, NewAllProvidersFailedError(len(errors), len(filteredProviders), errors)
	}

	return nil, NewServiceUnavailableError("no responses received from any provider", 0)
}

func (e *EnsembleService) RunEnsembleStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	e.mu.RLock()
	providers := make(map[string]LLMProvider)
	for k, v := range e.providers {
		providers[k] = v
	}
	e.mu.RUnlock()

	if len(providers) == 0 {
		return nil, NewConfigurationError("no providers available for streaming", nil)
	}

	// Filter providers based on request preferences
	filteredProviders := e.filterProviders(providers, req)
	if len(filteredProviders) == 0 {
		return nil, NewConfigurationError("no suitable providers available for streaming request", nil)
	}

	// Create context with timeout - use longer timeout for streaming
	streamTimeout := e.timeout * 2 // Double timeout for streaming operations
	if streamTimeout < 120*time.Second {
		streamTimeout = 120 * time.Second // Minimum 120 seconds for streaming
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, streamTimeout)

	// MULTI-PROVIDER STREAMING WITH FALLBACK: Use all available providers for true AI debate
	// Each provider contributes chunks that are merged and sent to the client
	// Fallback: If primary providers fail, automatically try remaining providers
	outputChan := make(chan *models.LLMResponse, 100) // Buffered for multi-provider

	// Track provider failures for diagnostics
	type providerError struct {
		name string
		err  error
	}
	failedProviders := make([]providerError, 0)
	var failedMu sync.Mutex

	// Start streams from ALL available providers concurrently
	activeStreams := 0
	streamChans := make([]<-chan *models.LLMResponse, 0, len(filteredProviders))
	providerNames := make([]string, 0, len(filteredProviders))
	failedProviderNames := make([]string, 0)

	// PHASE 1: Try all filtered providers first
	for name, provider := range filteredProviders {
		streamChan, err := provider.CompleteStream(timeoutCtx, req)
		if err != nil {
			// Categorize the error
			categorizedErr := CategorizeError(err, name)
			failedMu.Lock()
			failedProviders = append(failedProviders, providerError{name: name, err: categorizedErr})
			failedProviderNames = append(failedProviderNames, name)
			failedMu.Unlock()
			continue
		}
		streamChans = append(streamChans, streamChan)
		providerNames = append(providerNames, name)
		activeStreams++
	}

	// PHASE 2: FALLBACK - If primary providers failed, try ALL remaining providers
	if activeStreams == 0 && len(failedProviders) > 0 {
		// All filtered providers failed - try ALL providers as fallback
		for name, provider := range providers {
			// Skip already-tried providers
			alreadyTried := false
			for _, failed := range failedProviderNames {
				if name == failed {
					alreadyTried = true
					break
				}
			}
			if alreadyTried {
				continue
			}

			streamChan, err := provider.CompleteStream(timeoutCtx, req)
			if err != nil {
				categorizedErr := CategorizeError(err, name)
				failedMu.Lock()
				failedProviders = append(failedProviders, providerError{name: name, err: categorizedErr})
				failedMu.Unlock()
				continue
			}
			streamChans = append(streamChans, streamChan)
			providerNames = append(providerNames, name)
			activeStreams++
		}
	}

	// If STILL no active streams, return detailed error
	if activeStreams == 0 {
		cancel()
		// Build detailed error with categorized failures
		failedMu.Lock()
		errors := make([]error, len(failedProviders))
		networkErrors := 0
		providerErrors := 0
		for i, pErr := range failedProviders {
			errors[i] = pErr.err
			if llmErr, ok := pErr.err.(*LLMServiceError); ok {
				if llmErr.Type == ErrorTypeNetwork {
					networkErrors++
				} else {
					providerErrors++
				}
			}
		}
		failedMu.Unlock()

		// Return categorized error
		err := NewAllProvidersFailedError(len(failedProviders), len(providers), errors)
		err.Details["network_errors"] = networkErrors
		err.Details["provider_errors"] = providerErrors
		err.Details["attempted_fallback"] = len(failedProviders) > len(filteredProviders)
		return nil, err
	}

	// STREAMING FIX: Use SINGLE provider stream to avoid content interleaving
	// Multi-provider debate only works for non-streaming where we can vote on complete responses
	// For streaming, we pick the first successful provider and use its stream exclusively
	selectedStream := streamChans[0]
	selectedProvider := providerNames[0]

	// Close any additional streams we won't use (cleanup)
	// Note: These channels will be closed by their providers when context is cancelled
	if len(streamChans) > 1 {
		// Just drain the unused streams in background to prevent goroutine leaks
		for i := 1; i < len(streamChans); i++ {
			go func(ch <-chan *models.LLMResponse) {
				for range ch {
					// Drain unused stream
				}
			}(streamChans[i])
		}
	}

	// Forward responses from the selected single provider
	go func() {
		defer close(outputChan)

		for resp := range selectedStream {
			resp.ProviderID = selectedProvider
			resp.ProviderName = selectedProvider

			// Add metadata to track which provider was used
			if resp.Metadata == nil {
				resp.Metadata = make(map[string]interface{})
			}
			resp.Metadata["streaming_provider"] = selectedProvider
			resp.Metadata["available_providers"] = len(streamChans)

			select {
			case outputChan <- resp:
			case <-timeoutCtx.Done():
				return
			}
		}
	}()

	// Return a wrapper that will cancel when closed or timed out
	// The cancel is controlled by the HTTP handler, not the stream completion
	wrappedChan := make(chan *models.LLMResponse)
	go func() {
		defer close(wrappedChan)
		defer cancel() // Cancel only when wrapper is done (consumer finished)

		for resp := range outputChan {
			select {
			case wrappedChan <- resp:
			case <-ctx.Done(): // Use original context, not timeoutCtx
				return
			}
		}
	}()

	return wrappedChan, nil
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

func (e *EnsembleService) majorityVoting(responses []*models.LLMResponse, config *models.EnsembleConfig) *models.LLMResponse {
	if len(responses) == 0 {
		return nil
	}

	if len(responses) == 1 {
		responses[0].Selected = true
		responses[0].SelectionScore = 1.0
		return responses[0]
	}

	// Implement real majority voting by comparing response content
	// Group responses by similarity (using exact match for simplicity)
	contentCounts := make(map[string]int)
	contentToResponse := make(map[string]*models.LLMResponse)

	for _, resp := range responses {
		// Normalize content for comparison (trim whitespace)
		normalizedContent := strings.TrimSpace(resp.Content)
		contentCounts[normalizedContent]++
		if _, exists := contentToResponse[normalizedContent]; !exists {
			contentToResponse[normalizedContent] = resp
		}
	}

	// Find the content with the highest count (majority)
	// Iterate over responses in order to ensure deterministic tie-breaking
	// (prefer earlier responses when counts are equal)
	var selectedResponse *models.LLMResponse
	maxCount := 0
	seenContents := make(map[string]bool)
	for _, resp := range responses {
		normalizedContent := strings.TrimSpace(resp.Content)
		if seenContents[normalizedContent] {
			continue // Already considered this content
		}
		seenContents[normalizedContent] = true
		count := contentCounts[normalizedContent]
		if count > maxCount {
			maxCount = count
			selectedResponse = contentToResponse[normalizedContent]
		}
	}

	// Check if we have a true majority (> 50% of votes)
	// If not, fallback to confidence-based selection when FallbackToBest is enabled
	hasTrueMajority := float64(maxCount) > float64(len(responses))/2.0
	fallbackEnabled := config != nil && config.FallbackToBest

	if !hasTrueMajority && fallbackEnabled {
		// No true majority - fallback to selecting by highest confidence
		selectedResponse = e.selectBestResponse(responses)
		if selectedResponse.Metadata == nil {
			selectedResponse.Metadata = make(map[string]interface{})
		}
		selectedResponse.Metadata["voting_method"] = "majority_fallback_confidence"
		selectedResponse.Metadata["fallback_reason"] = "no_true_majority"
		selectedResponse.Metadata["max_vote_count"] = maxCount
		selectedResponse.Metadata["total_responses"] = len(responses)
		return selectedResponse
	}

	// Select the response with majority/plurality content
	selectedResponse.Selected = true
	selectedResponse.SelectionScore = float64(maxCount) / float64(len(responses))

	// Add metadata about the voting result
	if selectedResponse.Metadata == nil {
		selectedResponse.Metadata = make(map[string]interface{})
	}
	if hasTrueMajority {
		selectedResponse.Metadata["voting_method"] = "majority"
	} else {
		selectedResponse.Metadata["voting_method"] = "plurality"
	}
	selectedResponse.Metadata["vote_count"] = maxCount
	selectedResponse.Metadata["total_responses"] = len(responses)

	return selectedResponse
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
