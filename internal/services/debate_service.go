package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

// DebateService provides core debate functionality with real LLM provider calls
type DebateService struct {
	logger           *logrus.Logger
	providerRegistry *ProviderRegistry
	cogneeService    *CogneeService
}

// NewDebateService creates a new debate service
func NewDebateService(logger *logrus.Logger) *DebateService {
	return &DebateService{
		logger: logger,
	}
}

// NewDebateServiceWithDeps creates a new debate service with dependencies for real LLM calls
func NewDebateServiceWithDeps(
	logger *logrus.Logger,
	providerRegistry *ProviderRegistry,
	cogneeService *CogneeService,
) *DebateService {
	return &DebateService{
		logger:           logger,
		providerRegistry: providerRegistry,
		cogneeService:    cogneeService,
	}
}

// SetProviderRegistry sets the provider registry for LLM calls
func (ds *DebateService) SetProviderRegistry(registry *ProviderRegistry) {
	ds.providerRegistry = registry
}

// SetCogneeService sets the Cognee service for enhanced insights
func (ds *DebateService) SetCogneeService(service *CogneeService) {
	ds.cogneeService = service
}

// roundResult holds the results from a single debate round
type roundResult struct {
	Round     int
	Responses []ParticipantResponse
	StartTime time.Time
	EndTime   time.Time
}

// ConductDebate conducts a debate with the given configuration
func (ds *DebateService) ConductDebate(
	ctx context.Context,
	config *DebateConfig,
) (*DebateResult, error) {
	startTime := time.Now()
	sessionID := fmt.Sprintf("session-%s-%s", config.DebateID, uuid.New().String()[:8])

	ds.logger.Infof("Starting debate %s with topic: %s", config.DebateID, config.Topic)

	// Check if we have provider registry for real calls
	if ds.providerRegistry == nil {
		ds.logger.Warn("No provider registry configured, using simulated debate")
		return ds.conductSimulatedDebate(ctx, config, startTime, sessionID)
	}

	// Conduct real debate with LLM providers
	return ds.conductRealDebate(ctx, config, startTime, sessionID)
}

// conductRealDebate executes a debate with real LLM provider calls
func (ds *DebateService) conductRealDebate(
	ctx context.Context,
	config *DebateConfig,
	startTime time.Time,
	sessionID string,
) (*DebateResult, error) {
	allResponses := make([]ParticipantResponse, 0)
	roundResults := make([]roundResult, 0, config.MaxRounds)

	// Create context with overall timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	// Execute rounds
	previousResponses := make([]ParticipantResponse, 0)
	for round := 1; round <= config.MaxRounds; round++ {
		ds.logger.Infof("Starting round %d of debate %s", round, config.DebateID)

		roundStart := time.Now()
		responses, err := ds.executeRound(timeoutCtx, config, round, previousResponses)
		if err != nil {
			ds.logger.WithError(err).Errorf("Round %d failed", round)
			// Continue with partial results if we have some
			if len(responses) == 0 {
				break
			}
		}

		roundResults = append(roundResults, roundResult{
			Round:     round,
			Responses: responses,
			StartTime: roundStart,
			EndTime:   time.Now(),
		})

		allResponses = append(allResponses, responses...)
		previousResponses = responses

		// Check for early consensus
		if ds.checkEarlyConsensus(responses) {
			ds.logger.Infof("Early consensus reached at round %d", round)
			break
		}

		// Check context cancellation
		select {
		case <-timeoutCtx.Done():
			ds.logger.Warn("Debate timeout reached")
			break
		default:
			continue
		}
	}

	endTime := time.Now()

	// Analyze responses for consensus
	consensus := ds.analyzeConsensus(allResponses, config.Topic)

	// Calculate quality scores from actual responses
	qualityScore := ds.calculateQualityScore(allResponses)
	finalScore := ds.calculateFinalScore(allResponses, consensus)

	// Find best response
	bestResponse := ds.findBestResponse(allResponses)

	// Build result
	result := &DebateResult{
		DebateID:        config.DebateID,
		SessionID:       sessionID,
		Topic:           config.Topic,
		StartTime:       startTime,
		EndTime:         endTime,
		Duration:        endTime.Sub(startTime),
		TotalRounds:     config.MaxRounds,
		RoundsConducted: len(roundResults),
		AllResponses:    allResponses,
		Participants:    ds.getLatestParticipantResponses(allResponses, config.Participants),
		BestResponse:    bestResponse,
		Consensus:       consensus,
		QualityScore:    qualityScore,
		FinalScore:      finalScore,
		Success:         len(allResponses) > 0,
		Metadata:        make(map[string]interface{}),
	}

	// Add Cognee insights if enabled
	if config.EnableCognee && ds.cogneeService != nil {
		insights, err := ds.generateCogneeInsights(timeoutCtx, config, allResponses)
		if err != nil {
			ds.logger.WithError(err).Warn("Failed to generate Cognee insights")
		} else {
			result.CogneeEnhanced = true
			result.CogneeInsights = insights
		}
	}

	// Add metadata
	result.Metadata["total_responses"] = len(allResponses)
	result.Metadata["providers_used"] = ds.getUniqueProviders(allResponses)
	result.Metadata["avg_response_time"] = ds.calculateAvgResponseTime(allResponses)

	return result, nil
}

// executeRound executes a single debate round with all participants
func (ds *DebateService) executeRound(
	ctx context.Context,
	config *DebateConfig,
	round int,
	previousResponses []ParticipantResponse,
) ([]ParticipantResponse, error) {
	responses := make([]ParticipantResponse, 0, len(config.Participants))
	responseChan := make(chan ParticipantResponse, len(config.Participants))
	errorChan := make(chan error, len(config.Participants))

	var wg sync.WaitGroup
	for _, participant := range config.Participants {
		wg.Add(1)
		go func(p ParticipantConfig) {
			defer wg.Done()

			resp, err := ds.getParticipantResponse(ctx, config, p, round, previousResponses)
			if err != nil {
				errorChan <- fmt.Errorf("participant %s failed: %w", p.Name, err)
				return
			}
			responseChan <- resp
		}(participant)
	}

	// Wait for all participants
	go func() {
		wg.Wait()
		close(responseChan)
		close(errorChan)
	}()

	// Collect responses
	for resp := range responseChan {
		responses = append(responses, resp)
	}

	// Collect errors (for logging)
	var errs []error
	for err := range errorChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 && len(responses) == 0 {
		return responses, fmt.Errorf("all participants failed: %v", errs)
	}

	return responses, nil
}

// getParticipantResponse gets a response from a specific participant
func (ds *DebateService) getParticipantResponse(
	ctx context.Context,
	config *DebateConfig,
	participant ParticipantConfig,
	round int,
	previousResponses []ParticipantResponse,
) (ParticipantResponse, error) {
	startTime := time.Now()

	// Get the provider
	provider, err := ds.providerRegistry.GetProvider(participant.LLMProvider)
	if err != nil {
		return ParticipantResponse{}, fmt.Errorf("provider not found: %w", err)
	}

	// Build the prompt with context from previous responses
	prompt := ds.buildDebatePrompt(config.Topic, participant, round, previousResponses)

	// Create LLM request
	llmRequest := &models.LLMRequest{
		ID:        uuid.New().String(),
		SessionID: config.DebateID,
		Prompt:    prompt,
		Messages: []models.Message{
			{
				Role:    "system",
				Content: ds.buildSystemPrompt(participant),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		ModelParams: models.ModelParameters{
			Model:       participant.LLMModel,
			Temperature: 0.7,
			MaxTokens:   1024,
		},
	}

	// Make the actual LLM call
	llmResponse, err := provider.Complete(ctx, llmRequest)
	if err != nil {
		return ParticipantResponse{}, fmt.Errorf("LLM call failed: %w", err)
	}

	responseTime := time.Since(startTime)

	// Calculate quality score for this response
	qualityScore := ds.calculateResponseQuality(llmResponse)

	// Build participant response
	response := ParticipantResponse{
		ParticipantID:   participant.ParticipantID,
		ParticipantName: participant.Name,
		Role:            participant.Role,
		Round:           round,
		RoundNumber:     round,
		Response:        llmResponse.Content,
		Content:         llmResponse.Content,
		Confidence:      llmResponse.Confidence,
		QualityScore:    qualityScore,
		ResponseTime:    responseTime,
		LLMProvider:     participant.LLMProvider,
		LLMModel:        participant.LLMModel,
		LLMName:         participant.LLMModel,
		Timestamp:       startTime,
		Metadata: map[string]any{
			"tokens_used":    llmResponse.TokensUsed,
			"finish_reason":  llmResponse.FinishReason,
			"provider_id":    llmResponse.ProviderID,
			"response_time":  llmResponse.ResponseTime,
		},
	}

	// Add Cognee analysis if enabled
	if config.EnableCognee && ds.cogneeService != nil {
		analysis, err := ds.analyzeWithCognee(ctx, llmResponse.Content)
		if err == nil {
			response.CogneeEnhanced = true
			response.CogneeAnalysis = analysis
		}
	}

	return response, nil
}

// buildSystemPrompt builds the system prompt for a participant
func (ds *DebateService) buildSystemPrompt(participant ParticipantConfig) string {
	roleDescriptions := map[string]string{
		"proposer":  "You are presenting and defending the main argument. Be persuasive and provide evidence.",
		"opponent":  "You are challenging the main argument. Identify weaknesses and present counterarguments.",
		"critic":    "You are analyzing both sides objectively. Point out strengths and weaknesses.",
		"mediator":  "You are facilitating the discussion. Summarize key points and seek common ground.",
		"debater":   "You are participating in a balanced debate. Present your perspective clearly.",
		"analyst":   "You are analyzing the debate topic deeply. Provide insights and analysis.",
		"moderator": "You are moderating the discussion. Keep the debate focused and productive.",
	}

	roleDesc := roleDescriptions[participant.Role]
	if roleDesc == "" {
		roleDesc = "You are participating in an AI debate. Contribute thoughtfully to the discussion."
	}

	return fmt.Sprintf(
		"You are %s, participating as a %s in an AI debate. %s "+
			"Keep your responses focused, well-reasoned, and constructive. "+
			"Acknowledge valid points from others while maintaining your position when warranted.",
		participant.Name,
		participant.Role,
		roleDesc,
	)
}

// buildDebatePrompt builds the prompt for a debate round
func (ds *DebateService) buildDebatePrompt(
	topic string,
	participant ParticipantConfig,
	round int,
	previousResponses []ParticipantResponse,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("DEBATE TOPIC: %s\n\n", topic))
	sb.WriteString(fmt.Sprintf("ROUND: %d\n", round))
	sb.WriteString(fmt.Sprintf("YOUR ROLE: %s (%s)\n\n", participant.Name, participant.Role))

	if len(previousResponses) > 0 {
		sb.WriteString("PREVIOUS RESPONSES:\n")
		sb.WriteString("-------------------\n")
		for _, resp := range previousResponses {
			sb.WriteString(fmt.Sprintf("[%s (%s) - Round %d]:\n%s\n\n",
				resp.ParticipantName, resp.Role, resp.Round, resp.Content))
		}
		sb.WriteString("-------------------\n\n")
		sb.WriteString("Based on the previous responses, provide your perspective on the topic. ")
		sb.WriteString("Address points raised by others and advance the discussion.\n")
	} else {
		sb.WriteString("This is the opening round. Present your initial position on the topic.\n")
	}

	sb.WriteString("\nYour response:")

	return sb.String()
}

// calculateResponseQuality calculates quality score for a single response
func (ds *DebateService) calculateResponseQuality(resp *models.LLMResponse) float64 {
	score := 0.0

	// Base score from confidence (if provided by LLM)
	if resp.Confidence > 0 {
		score += resp.Confidence * 0.3
	} else {
		score += 0.7 * 0.3 // Default confidence
	}

	// Content length factor (20% weight)
	contentLength := len(resp.Content)
	lengthScore := 0.0
	if contentLength >= 100 && contentLength <= 500 {
		lengthScore = 1.0
	} else if contentLength > 500 && contentLength <= 1000 {
		lengthScore = 0.9
	} else if contentLength > 1000 && contentLength <= 2000 {
		lengthScore = 0.8
	} else if contentLength > 2000 {
		lengthScore = 0.7
	} else if contentLength > 50 {
		lengthScore = 0.6
	} else {
		lengthScore = 0.3
	}
	score += lengthScore * 0.2

	// Response completeness (20% weight) - based on finish reason
	completenessScore := 0.5
	switch resp.FinishReason {
	case "stop":
		completenessScore = 1.0
	case "end_turn":
		completenessScore = 1.0
	case "length":
		completenessScore = 0.7
	case "content_filter":
		completenessScore = 0.3
	}
	score += completenessScore * 0.2

	// Token efficiency (15% weight)
	if resp.TokensUsed > 0 && contentLength > 0 {
		efficiency := float64(contentLength) / float64(resp.TokensUsed)
		efficiencyScore := math.Min(1.0, efficiency/4.0)
		score += efficiencyScore * 0.15
	} else {
		score += 0.5 * 0.15
	}

	// Coherence indicators (15% weight)
	coherenceScore := ds.analyzeCoherence(resp.Content)
	score += coherenceScore * 0.15

	return math.Min(1.0, score)
}

// analyzeCoherence analyzes text coherence
func (ds *DebateService) analyzeCoherence(content string) float64 {
	if len(content) == 0 {
		return 0.0
	}

	score := 0.5 // Base score

	// Check for structure indicators
	structureIndicators := []string{
		"first", "second", "third", "finally",
		"however", "therefore", "because", "although",
		"in conclusion", "to summarize", "for example",
		"on the other hand", "furthermore", "moreover",
	}

	contentLower := strings.ToLower(content)
	structureCount := 0
	for _, indicator := range structureIndicators {
		if strings.Contains(contentLower, indicator) {
			structureCount++
		}
	}

	if structureCount >= 3 {
		score += 0.3
	} else if structureCount >= 1 {
		score += 0.15
	}

	// Check for sentence variety (basic check)
	sentences := strings.Split(content, ".")
	if len(sentences) >= 3 && len(sentences) <= 15 {
		score += 0.2
	} else if len(sentences) >= 2 {
		score += 0.1
	}

	return math.Min(1.0, score)
}

// calculateQualityScore calculates overall quality score from all responses
func (ds *DebateService) calculateQualityScore(responses []ParticipantResponse) float64 {
	if len(responses) == 0 {
		return 0.0
	}

	totalScore := 0.0
	for _, resp := range responses {
		totalScore += resp.QualityScore
	}

	return totalScore / float64(len(responses))
}

// calculateFinalScore calculates the final debate score
func (ds *DebateService) calculateFinalScore(responses []ParticipantResponse, consensus *ConsensusResult) float64 {
	if len(responses) == 0 {
		return 0.0
	}

	// Quality contributes 50%
	qualityScore := ds.calculateQualityScore(responses)

	// Consensus contributes 30%
	consensusScore := 0.0
	if consensus != nil {
		consensusScore = consensus.AgreementLevel
	}

	// Participation contributes 20%
	participationScore := 1.0
	uniqueParticipants := make(map[string]bool)
	for _, resp := range responses {
		uniqueParticipants[resp.ParticipantID] = true
	}
	if len(uniqueParticipants) < 2 {
		participationScore = 0.5
	}

	return qualityScore*0.5 + consensusScore*0.3 + participationScore*0.2
}

// analyzeConsensus analyzes responses for consensus
func (ds *DebateService) analyzeConsensus(responses []ParticipantResponse, topic string) *ConsensusResult {
	if len(responses) == 0 {
		return &ConsensusResult{
			Reached:        false,
			Achieved:       false,
			Confidence:     0.0,
			ConsensusLevel: 0.0,
			AgreementLevel: 0.0,
			FinalPosition:  "No responses to analyze",
			KeyPoints:      []string{},
			Disagreements:  []string{},
			Timestamp:      time.Now(),
		}
	}

	// Analyze agreement between responses
	agreementScore := ds.calculateAgreementScore(responses)
	keyPoints := ds.extractKeyPoints(responses)
	disagreements := ds.extractDisagreements(responses)

	// Calculate consensus confidence
	confidence := agreementScore
	if len(disagreements) > 0 {
		confidence *= 0.9 // Reduce confidence if there are disagreements
	}

	consensusReached := agreementScore >= 0.7

	// Generate final position summary
	finalPosition := ds.generateFinalPosition(responses, consensusReached)

	return &ConsensusResult{
		Reached:        consensusReached,
		Achieved:       consensusReached,
		Confidence:     confidence,
		ConsensusLevel: agreementScore,
		AgreementLevel: agreementScore,
		AgreementScore: agreementScore,
		FinalPosition:  finalPosition,
		KeyPoints:      keyPoints,
		Disagreements:  disagreements,
		Summary:        fmt.Sprintf("Debate on '%s' with %d responses", topic, len(responses)),
		VotingSummary: VotingSummary{
			Strategy:         "quality_weighted",
			TotalVotes:       len(responses),
			VoteDistribution: ds.getVoteDistribution(responses),
			Winner:           ds.getWinner(responses),
			Margin:           agreementScore,
		},
		Timestamp:    time.Now(),
		QualityScore: ds.calculateQualityScore(responses),
	}
}

// calculateAgreementScore calculates how much responses agree
func (ds *DebateService) calculateAgreementScore(responses []ParticipantResponse) float64 {
	if len(responses) < 2 {
		return 1.0 // Single response is self-consistent
	}

	// Calculate pairwise similarity
	totalSimilarity := 0.0
	comparisons := 0

	for i := 0; i < len(responses); i++ {
		for j := i + 1; j < len(responses); j++ {
			similarity := ds.calculateTextSimilarity(responses[i].Content, responses[j].Content)
			totalSimilarity += similarity
			comparisons++
		}
	}

	if comparisons == 0 {
		return 0.5
	}

	return totalSimilarity / float64(comparisons)
}

// calculateTextSimilarity calculates similarity between two texts
func (ds *DebateService) calculateTextSimilarity(text1, text2 string) float64 {
	// Simple word overlap similarity (Jaccard-like)
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	wordSet1 := make(map[string]bool)
	for _, w := range words1 {
		wordSet1[w] = true
	}

	wordSet2 := make(map[string]bool)
	for _, w := range words2 {
		wordSet2[w] = true
	}

	intersection := 0
	for w := range wordSet1 {
		if wordSet2[w] {
			intersection++
		}
	}

	union := len(wordSet1) + len(wordSet2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// extractKeyPoints extracts key points from responses
func (ds *DebateService) extractKeyPoints(responses []ParticipantResponse) []string {
	keyPoints := make([]string, 0)
	keyPhrases := make(map[string]int)

	// Simple key phrase extraction
	for _, resp := range responses {
		sentences := strings.Split(resp.Content, ".")
		for _, sentence := range sentences {
			sentence = strings.TrimSpace(sentence)
			if len(sentence) > 20 && len(sentence) < 200 {
				// Look for sentences with key indicators
				indicators := []string{"important", "key", "main", "significant", "essential", "crucial"}
				for _, indicator := range indicators {
					if strings.Contains(strings.ToLower(sentence), indicator) {
						keyPhrases[sentence]++
						break
					}
				}
			}
		}
	}

	// Get top key points
	type kv struct {
		Key   string
		Value int
	}
	var sortedPhrases []kv
	for k, v := range keyPhrases {
		sortedPhrases = append(sortedPhrases, kv{k, v})
	}
	sort.Slice(sortedPhrases, func(i, j int) bool {
		return sortedPhrases[i].Value > sortedPhrases[j].Value
	})

	for i, kv := range sortedPhrases {
		if i >= 5 {
			break
		}
		keyPoints = append(keyPoints, kv.Key)
	}

	// If no key points found, extract first sentence from best responses
	if len(keyPoints) == 0 && len(responses) > 0 {
		for i, resp := range responses {
			if i >= 3 {
				break
			}
			sentences := strings.Split(resp.Content, ".")
			if len(sentences) > 0 && len(strings.TrimSpace(sentences[0])) > 10 {
				keyPoints = append(keyPoints, strings.TrimSpace(sentences[0]))
			}
		}
	}

	return keyPoints
}

// extractDisagreements extracts disagreements from responses
func (ds *DebateService) extractDisagreements(responses []ParticipantResponse) []string {
	disagreements := make([]string, 0)

	disagreementIndicators := []string{
		"however", "but", "disagree", "contrary", "oppose",
		"on the other hand", "in contrast", "nevertheless",
	}

	for _, resp := range responses {
		sentences := strings.Split(resp.Content, ".")
		for _, sentence := range sentences {
			sentence = strings.TrimSpace(sentence)
			for _, indicator := range disagreementIndicators {
				if strings.Contains(strings.ToLower(sentence), indicator) && len(sentence) > 20 {
					disagreements = append(disagreements, sentence)
					break
				}
			}
		}
	}

	// Limit to top 5 disagreements
	if len(disagreements) > 5 {
		disagreements = disagreements[:5]
	}

	return disagreements
}

// generateFinalPosition generates the final position summary
func (ds *DebateService) generateFinalPosition(responses []ParticipantResponse, consensusReached bool) string {
	if len(responses) == 0 {
		return "No position established"
	}

	if consensusReached {
		return "Consensus reached: Participants found common ground on the key aspects of the topic"
	}

	return "Discussion ongoing: Multiple perspectives presented with varying viewpoints"
}

// getVoteDistribution gets the vote distribution by participant
func (ds *DebateService) getVoteDistribution(responses []ParticipantResponse) map[string]int {
	distribution := make(map[string]int)
	for _, resp := range responses {
		distribution[resp.ParticipantName]++
	}
	return distribution
}

// getWinner gets the best performing participant
func (ds *DebateService) getWinner(responses []ParticipantResponse) string {
	if len(responses) == 0 {
		return ""
	}

	bestScore := 0.0
	winner := ""
	scores := make(map[string]float64)
	counts := make(map[string]int)

	for _, resp := range responses {
		scores[resp.ParticipantName] += resp.QualityScore
		counts[resp.ParticipantName]++
	}

	for name, score := range scores {
		avgScore := score / float64(counts[name])
		if avgScore > bestScore {
			bestScore = avgScore
			winner = name
		}
	}

	return winner
}

// checkEarlyConsensus checks if early consensus has been reached
func (ds *DebateService) checkEarlyConsensus(responses []ParticipantResponse) bool {
	if len(responses) < 2 {
		return false
	}

	agreementScore := ds.calculateAgreementScore(responses)
	return agreementScore >= 0.85
}

// findBestResponse finds the best response from all responses
func (ds *DebateService) findBestResponse(responses []ParticipantResponse) *ParticipantResponse {
	if len(responses) == 0 {
		return nil
	}

	best := responses[0]
	for _, resp := range responses[1:] {
		if resp.QualityScore > best.QualityScore {
			best = resp
		}
	}

	return &best
}

// getLatestParticipantResponses gets the latest response from each participant
func (ds *DebateService) getLatestParticipantResponses(
	allResponses []ParticipantResponse,
	participants []ParticipantConfig,
) []ParticipantResponse {
	latest := make(map[string]ParticipantResponse)

	for _, resp := range allResponses {
		if existing, ok := latest[resp.ParticipantID]; !ok || resp.Round > existing.Round {
			latest[resp.ParticipantID] = resp
		}
	}

	result := make([]ParticipantResponse, 0, len(participants))
	for _, p := range participants {
		if resp, ok := latest[p.ParticipantID]; ok {
			result = append(result, resp)
		}
	}

	return result
}

// getUniqueProviders gets unique provider names from responses
func (ds *DebateService) getUniqueProviders(responses []ParticipantResponse) []string {
	providers := make(map[string]bool)
	for _, resp := range responses {
		providers[resp.LLMProvider] = true
	}

	result := make([]string, 0, len(providers))
	for p := range providers {
		result = append(result, p)
	}
	return result
}

// calculateAvgResponseTime calculates average response time
func (ds *DebateService) calculateAvgResponseTime(responses []ParticipantResponse) time.Duration {
	if len(responses) == 0 {
		return 0
	}

	total := time.Duration(0)
	for _, resp := range responses {
		total += resp.ResponseTime
	}

	return total / time.Duration(len(responses))
}

// analyzeWithCognee analyzes a response with Cognee
func (ds *DebateService) analyzeWithCognee(ctx context.Context, content string) (*CogneeAnalysis, error) {
	if ds.cogneeService == nil {
		return nil, fmt.Errorf("cognee service not configured")
	}

	startTime := time.Now()

	// Search for relevant context
	searchResult, err := ds.cogneeService.SearchMemory(ctx, content, "", 5)
	if err != nil {
		return nil, fmt.Errorf("cognee search failed: %w", err)
	}

	// Extract entities and key phrases
	entities := make([]string, 0)
	keyPhrases := make([]string, 0)

	// Extract from vector results
	for _, mem := range searchResult.VectorResults {
		if len(entities) < 10 {
			entities = append(entities, mem.Content[:min(50, len(mem.Content))])
		}
	}

	// Extract key phrases from content
	sentences := strings.Split(content, ".")
	for i, s := range sentences {
		if i >= 5 {
			break
		}
		s = strings.TrimSpace(s)
		if len(s) > 10 {
			keyPhrases = append(keyPhrases, s)
		}
	}

	// Determine sentiment
	sentiment := ds.analyzeSentiment(content)

	return &CogneeAnalysis{
		Enhanced:         true,
		OriginalResponse: content,
		Sentiment:        sentiment,
		Entities:         entities,
		KeyPhrases:       keyPhrases,
		Confidence:       searchResult.RelevanceScore,
		ProcessingTime:   time.Since(startTime),
	}, nil
}

// analyzeSentiment performs simple sentiment analysis
func (ds *DebateService) analyzeSentiment(content string) string {
	contentLower := strings.ToLower(content)

	positiveWords := []string{"agree", "good", "excellent", "great", "positive", "support", "beneficial", "advantage"}
	negativeWords := []string{"disagree", "bad", "poor", "negative", "oppose", "harmful", "disadvantage", "problem"}
	neutralWords := []string{"however", "although", "consider", "perspective", "viewpoint"}

	positiveCount := 0
	negativeCount := 0
	neutralCount := 0

	for _, word := range positiveWords {
		if strings.Contains(contentLower, word) {
			positiveCount++
		}
	}
	for _, word := range negativeWords {
		if strings.Contains(contentLower, word) {
			negativeCount++
		}
	}
	for _, word := range neutralWords {
		if strings.Contains(contentLower, word) {
			neutralCount++
		}
	}

	if positiveCount > negativeCount+neutralCount {
		return "positive"
	} else if negativeCount > positiveCount+neutralCount {
		return "negative"
	}
	return "neutral"
}

// generateCogneeInsights generates comprehensive Cognee insights
func (ds *DebateService) generateCogneeInsights(
	ctx context.Context,
	config *DebateConfig,
	responses []ParticipantResponse,
) (*CogneeInsights, error) {
	if ds.cogneeService == nil {
		return nil, fmt.Errorf("cognee service not configured")
	}

	startTime := time.Now()

	// Combine all response content
	var combinedContent strings.Builder
	for _, resp := range responses {
		combinedContent.WriteString(resp.Content)
		combinedContent.WriteString("\n\n")
	}

	// Search for insights
	searchResult, err := ds.cogneeService.SearchMemory(ctx, combinedContent.String(), "", 10)
	if err != nil {
		ds.logger.WithError(err).Warn("Cognee search failed during insights generation")
	}

	// Get graph insights if available
	var graphResults []map[string]interface{}
	if searchResult != nil {
		graphResults = searchResult.GraphResults
	}
	_ = graphResults // Used for future knowledge graph integration

	// Extract entities from responses
	entities := make([]Entity, 0)
	entities = append(entities, Entity{
		Text:       config.Topic,
		Type:       "TOPIC",
		Confidence: 1.0,
	})

	// Add participant entities
	for _, p := range config.Participants {
		entities = append(entities, Entity{
			Text:       p.Name,
			Type:       "PARTICIPANT",
			Confidence: 0.9,
		})
	}

	// Calculate quality metrics from responses
	avgQuality := ds.calculateQualityScore(responses)
	coherenceScore := 0.0
	if len(responses) > 0 {
		for _, resp := range responses {
			coherenceScore += ds.analyzeCoherence(resp.Content)
		}
		coherenceScore /= float64(len(responses))
	}

	// Build knowledge graph
	nodes := make([]Node, 0)
	edges := make([]Edge, 0)

	// Add topic node
	nodes = append(nodes, Node{
		ID:    "topic-1",
		Label: config.Topic,
		Type:  "topic",
		Properties: map[string]any{
			"central": true,
		},
	})

	// Add participant nodes and edges
	for i, p := range config.Participants {
		nodeID := fmt.Sprintf("participant-%d", i+1)
		nodes = append(nodes, Node{
			ID:    nodeID,
			Label: p.Name,
			Type:  "participant",
			Properties: map[string]any{
				"role":     p.Role,
				"provider": p.LLMProvider,
			},
		})
		edges = append(edges, Edge{
			Source: nodeID,
			Target: "topic-1",
			Type:   "discusses",
			Weight: 1.0,
		})
	}

	// Generate recommendations based on debate quality
	recommendations := ds.generateRecommendations(responses, avgQuality)

	// Build topic modeling
	topicModeling := make(map[string]float64)
	topicModeling[config.Topic] = 0.9
	topicModeling["debate"] = 0.8
	topicModeling["discussion"] = 0.7

	// Calculate relevance and innovation scores
	relevanceScore := 0.0
	if searchResult != nil {
		relevanceScore = searchResult.RelevanceScore
	}
	if relevanceScore == 0 {
		relevanceScore = avgQuality * 0.9
	}

	innovationScore := ds.calculateInnovationScore(responses)

	return &CogneeInsights{
		DatasetName:     "debate-insights",
		EnhancementTime: time.Since(startTime),
		SemanticAnalysis: SemanticAnalysis{
			MainThemes:     []string{config.Topic, "debate", "discussion"},
			CoherenceScore: coherenceScore,
		},
		EntityExtraction: entities,
		SentimentAnalysis: SentimentAnalysis{
			OverallSentiment: ds.getOverallSentiment(responses),
			SentimentScore:   ds.calculateSentimentScore(responses),
		},
		KnowledgeGraph: KnowledgeGraph{
			Nodes:           nodes,
			Edges:           edges,
			CentralConcepts: []string{config.Topic},
		},
		Recommendations: recommendations,
		QualityMetrics: &QualityMetrics{
			Coherence:    coherenceScore,
			Relevance:    relevanceScore,
			Accuracy:     avgQuality,
			Completeness: float64(len(responses)) / float64(len(config.Participants)*config.MaxRounds),
			OverallScore: (coherenceScore + relevanceScore + avgQuality) / 3,
		},
		TopicModeling:   topicModeling,
		CoherenceScore:  coherenceScore,
		RelevanceScore:  relevanceScore,
		InnovationScore: innovationScore,
	}, nil
}

// generateRecommendations generates recommendations based on debate quality
func (ds *DebateService) generateRecommendations(responses []ParticipantResponse, avgQuality float64) []string {
	recommendations := []string{
		"Consider diverse perspectives",
		"Focus on evidence-based arguments",
		"Maintain respectful discourse",
	}

	if avgQuality < 0.5 {
		recommendations = append(recommendations, "Increase response depth and detail")
		recommendations = append(recommendations, "Provide more supporting evidence")
	}

	if len(responses) < 4 {
		recommendations = append(recommendations, "Consider additional debate rounds")
	}

	return recommendations
}

// getOverallSentiment gets the overall sentiment from all responses
func (ds *DebateService) getOverallSentiment(responses []ParticipantResponse) string {
	positive := 0
	negative := 0
	neutral := 0

	for _, resp := range responses {
		sentiment := ds.analyzeSentiment(resp.Content)
		switch sentiment {
		case "positive":
			positive++
		case "negative":
			negative++
		default:
			neutral++
		}
	}

	if positive > negative && positive > neutral {
		return "positive"
	} else if negative > positive && negative > neutral {
		return "negative"
	}
	return "neutral"
}

// calculateSentimentScore calculates a numeric sentiment score
func (ds *DebateService) calculateSentimentScore(responses []ParticipantResponse) float64 {
	if len(responses) == 0 {
		return 0.5
	}

	totalScore := 0.0
	for _, resp := range responses {
		sentiment := ds.analyzeSentiment(resp.Content)
		switch sentiment {
		case "positive":
			totalScore += 0.8
		case "negative":
			totalScore += 0.2
		default:
			totalScore += 0.5
		}
	}

	return totalScore / float64(len(responses))
}

// calculateInnovationScore calculates how innovative the responses are
func (ds *DebateService) calculateInnovationScore(responses []ParticipantResponse) float64 {
	if len(responses) == 0 {
		return 0.0
	}

	// Look for innovative language patterns
	innovativeIndicators := []string{
		"new approach", "novel", "innovative", "creative",
		"alternative", "different perspective", "unique",
		"unexplored", "fresh", "original",
	}

	innovativeCount := 0
	for _, resp := range responses {
		contentLower := strings.ToLower(resp.Content)
		for _, indicator := range innovativeIndicators {
			if strings.Contains(contentLower, indicator) {
				innovativeCount++
				break
			}
		}
	}

	// Also consider response diversity
	diversity := 1.0 - ds.calculateAgreementScore(responses)

	return (float64(innovativeCount)/float64(len(responses))*0.6 + diversity*0.4)
}

// conductSimulatedDebate provides backward compatible simulated debate
func (ds *DebateService) conductSimulatedDebate(
	ctx context.Context,
	config *DebateConfig,
	startTime time.Time,
	sessionID string,
) (*DebateResult, error) {
	result := &DebateResult{
		DebateID:        config.DebateID,
		SessionID:       sessionID,
		Topic:           config.Topic,
		StartTime:       startTime,
		EndTime:         startTime.Add(config.Timeout),
		Duration:        config.Timeout,
		TotalRounds:     config.MaxRounds,
		RoundsConducted: config.MaxRounds,
		Success:         true,
		QualityScore:    0.85,
		FinalScore:      0.87,
		Metadata:        make(map[string]interface{}),
	}

	// Add participants
	for _, participant := range config.Participants {
		result.Participants = append(result.Participants, ParticipantResponse{
			ParticipantID:   participant.ParticipantID,
			ParticipantName: participant.Name,
			Role:            participant.Role,
			Round:           1,
			RoundNumber:     1,
			Response:        fmt.Sprintf("Response from %s", participant.Name),
			Content:         fmt.Sprintf("Content from %s", participant.Name),
			Confidence:      0.9,
			QualityScore:    0.85,
			ResponseTime:    5 * time.Second,
			LLMProvider:     participant.LLMProvider,
			LLMModel:        participant.LLMModel,
			LLMName:         participant.LLMModel,
			Timestamp:       startTime,
		})
	}

	// Add consensus
	result.Consensus = &ConsensusResult{
		Reached:        true,
		Achieved:       true,
		Confidence:     0.85,
		ConsensusLevel: 0.85,
		AgreementLevel: 0.85,
		AgreementScore: 0.85,
		FinalPosition:  "Agreement reached",
		KeyPoints:      []string{"Point 1", "Point 2"},
		Disagreements:  []string{},
		Summary:        "Consensus summary",
		Timestamp:      startTime,
		QualityScore:   0.85,
	}

	if config.EnableCognee {
		result.CogneeEnhanced = true
		start := time.Now()

		// Generate insights using topic as query
		insights := &CogneeInsights{
			DatasetName:     "debate-insights",
			EnhancementTime: time.Since(start),
			SemanticAnalysis: SemanticAnalysis{
				MainThemes:     []string{config.Topic, "debate", "discussion"},
				CoherenceScore: 0.85,
			},
			EntityExtraction: []Entity{
				{Text: config.Topic, Type: "TOPIC", Confidence: 1.0},
			},
			SentimentAnalysis: SentimentAnalysis{
				OverallSentiment: "neutral",
				SentimentScore:   0.7,
			},
			KnowledgeGraph: KnowledgeGraph{
				Nodes:           []Node{},
				Edges:           []Edge{},
				CentralConcepts: []string{config.Topic},
			},
			Recommendations: []string{
				"Consider diverse perspectives",
				"Focus on evidence-based arguments",
				"Maintain respectful discourse",
			},
			QualityMetrics: &QualityMetrics{
				Coherence:    0.9,
				Relevance:    0.85,
				Accuracy:     0.88,
				Completeness: 0.87,
				OverallScore: 0.87,
			},
			TopicModeling: map[string]float64{
				config.Topic: 0.9,
			},
			CoherenceScore:  0.85,
			RelevanceScore:  0.82,
			InnovationScore: 0.75,
		}

		result.CogneeInsights = insights
	}

	return result, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LLMProviderInterface defines the interface for LLM providers used in debate
type LLMProviderInterface interface {
	Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
}

// Ensure llm.LLMProvider satisfies our interface
var _ LLMProviderInterface = (llm.LLMProvider)(nil)
