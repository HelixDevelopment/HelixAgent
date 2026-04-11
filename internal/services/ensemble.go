package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/models"
)

// EnsembleService manages multiple LLM providers and implements voting strategies
type EnsembleService struct {
	providers     map[string]LLMProvider
	strategy      string
	timeout       time.Duration
	mu            sync.RWMutex
	scoreProvider LLMsVerifierScoreProvider // Optional: provides LLMsVerifier scores for provider ordering
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

// SetScoreProvider sets the LLMsVerifier score provider for dynamic provider ordering
func (e *EnsembleService) SetScoreProvider(sp LLMsVerifierScoreProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.scoreProvider = sp
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
	startTime := time.Now()

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
	type providerResult struct {
		resp *models.LLMResponse
		err  error
	}
	resultChan := make(chan providerResult, len(filteredProviders))

	for name, provider := range filteredProviders {
		go func(name string, provider LLMProvider) {
			resp, err := provider.Complete(timeoutCtx, req)
			if err != nil {
				resultChan <- providerResult{err: fmt.Errorf("provider %s failed: %w", name, err)}
				return
			}
			if resp == nil {
				resultChan <- providerResult{err: fmt.Errorf("provider %s returned nil response", name)}
				return
			}
			// Add provider metadata
			resp.ProviderID = name
			resp.ProviderName = name
			resultChan <- providerResult{resp: resp}
		}(name, provider)
	}

	// Collect responses with timeout protection
	// Return as soon as we have enough responses OR the timeout expires
	responses := make([]*models.LLMResponse, 0, len(filteredProviders))
	var errors []error
	received := 0
	total := len(filteredProviders)

	// Minimum responses needed to proceed (at least 1, or 2 if enough providers)
	minResponses := 1
	if total >= 3 {
		minResponses = 2
	}

	// Use a shorter "early return" timer: if we have enough responses, don't wait for stragglers
	earlyReturnTimer := time.NewTimer(e.timeout / 2)
	defer earlyReturnTimer.Stop()

collectLoop:
	for received < total {
		select {
		case result := <-resultChan:
			received++
			if result.err != nil {
				errors = append(errors, result.err)
			} else {
				responses = append(responses, result.resp)
			}
			// If all providers responded, exit immediately
			if received >= total {
				break collectLoop
			}
		case <-earlyReturnTimer.C:
			// Half the timeout elapsed - if we have enough responses, return early
			if len(responses) >= minResponses {
				break collectLoop
			}
			// Otherwise keep waiting until full timeout
		case <-timeoutCtx.Done():
			// Timeout expired - return whatever we have
			break collectLoop
		}
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
				"execution_time":       time.Since(startTime).Milliseconds(),
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

	// DETERMINISTIC PROVIDER ORDERING: Sort providers by priority to ensure
	// high-quality verified providers are tried first (fixes random map iteration)
	sortedProviderNames := e.getSortedProviderNames(filteredProviders)

	// PHASE 1: Try all filtered providers first (in sorted priority order)
	for _, name := range sortedProviderNames {
		provider := filteredProviders[name]
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
		// All filtered providers failed - try ALL providers as fallback (in sorted order)
		sortedFallbackNames := e.getSortedProviderNames(providers)
		for _, name := range sortedFallbackNames {
			provider := providers[name]
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
	// CONCURRENCY FIX: Use WaitGroup to track drain goroutines and context awareness to prevent leaks
	if len(streamChans) > 1 {
		var drainWg sync.WaitGroup
		for i := 1; i < len(streamChans); i++ {
			drainWg.Add(1)
			go func(ch <-chan *models.LLMResponse) {
				defer drainWg.Done()
				for {
					select {
					case _, ok := <-ch:
						if !ok {
							return // Channel closed
						}
						// Drain unused stream
					case <-timeoutCtx.Done():
						return // Context cancelled, stop draining
					}
				}
			}(streamChans[i])
		}
		// Don't block - let drain goroutines run in background but they will exit on context cancel
		go func() {
			drainWg.Wait()
		}()
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

	// If no preferred providers specified, use all available providers
	if req.EnsembleConfig == nil || len(req.EnsembleConfig.PreferredProviders) == 0 {
		for k, v := range providers {
			filtered[k] = v
		}
		return filtered
	}

	// Filter by preferred providers - respect explicit preferences strictly
	for _, preferred := range req.EnsembleConfig.PreferredProviders {
		if provider, exists := providers[preferred]; exists {
			filtered[preferred] = provider
		}
	}

	// If MinProviders is set and we don't have enough, add more providers
	if req.EnsembleConfig != nil && req.EnsembleConfig.MinProviders > 0 {
		if len(filtered) < req.EnsembleConfig.MinProviders {
			// Add providers not in the preferred list until we meet the minimum
			for name, provider := range providers {
				if len(filtered) >= req.EnsembleConfig.MinProviders {
					break
				}
				if _, exists := filtered[name]; !exists {
					filtered[name] = provider
				}
			}
		}
	}

	return filtered
}

// getSortedProviderNames returns provider names sorted by LLMsVerifier scores for deterministic ordering.
// This ensures streaming always tries high-quality verified providers first.
// Priority order:
// 1. Verified OAuth2 providers (claude-oauth, qwen-oauth) - sorted by LLMsVerifier score (highest first)
// 2. All other providers - sorted by LLMsVerifier score (highest first)
// 3. Providers without scores - alphabetically sorted
func (e *EnsembleService) getSortedProviderNames(providers map[string]LLMProvider) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}

	// Get provider scores from LLMsVerifier
	providerScores := make(map[string]float64)
	if e.scoreProvider != nil {
		for _, name := range names {
			// Try to get score by provider name
			if score, ok := e.scoreProvider.GetProviderScore(name); ok {
				providerScores[name] = score
			} else {
				// Try base provider name (e.g., "claude" for "claude-oauth")
				baseName := strings.TrimSuffix(name, "-oauth")
				if score, ok := e.scoreProvider.GetProviderScore(baseName); ok {
					providerScores[name] = score
				}
			}
		}
	}

	// Helper to check if provider is OAuth2
	isOAuth := func(name string) bool {
		return strings.HasSuffix(name, "-oauth")
	}

	// Sort providers:
	// 1. OAuth2 providers first (by score, highest first)
	// 2. Non-OAuth providers second (by score, highest first)
	// 3. Within same category: by score descending, then alphabetically
	sort.Slice(names, func(i, j int) bool {
		oauthI := isOAuth(names[i])
		oauthJ := isOAuth(names[j])

		// OAuth2 providers come first
		if oauthI != oauthJ {
			return oauthI // OAuth providers have priority
		}

		// Get scores (0 if not found)
		scoreI := providerScores[names[i]]
		scoreJ := providerScores[names[j]]

		// Sort by score (highest first)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}

		// Same score - sort alphabetically for deterministic ordering
		return names[i] < names[j]
	})

	return names
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
		// Select the highest-confidence response from the majority
		// group. Seed `selected` with the first element of the group
		// (guaranteed non-empty because len(largestGroup) > len/2 >= 1)
		// so a cohort where every confidence is 0 — or equal — still
		// yields a deterministic winner. The prior strict `>` loop
		// left `selected` nil when no response had positive confidence,
		// which crashed the mark-pass below (caught by
		// tests/e2e/ensemble_voting_e2e_test.go:73).
		selected := largestGroup[0]
		maxConfidence := selected.Confidence

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
