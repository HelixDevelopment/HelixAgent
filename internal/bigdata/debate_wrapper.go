package bigdata

import (
	"context"
	"time"

	"dev.helix.agent/internal/services"
	"github.com/sirupsen/logrus"
)

// DebateServiceWrapper wraps the debate service to add big data capabilities
type DebateServiceWrapper struct {
	debateService        *services.DebateService
	debateIntegration    *DebateIntegration
	analyticsIntegration *AnalyticsIntegration
	entityIntegration    *EntityIntegration
	logger               *logrus.Logger
	enableBigData        bool
}

// NewDebateServiceWrapper creates a new debate service wrapper
func NewDebateServiceWrapper(
	debateService *services.DebateService,
	debateIntegration *DebateIntegration,
	analyticsIntegration *AnalyticsIntegration,
	entityIntegration *EntityIntegration,
	logger *logrus.Logger,
	enableBigData bool,
) *DebateServiceWrapper {
	return &DebateServiceWrapper{
		debateService:        debateService,
		debateIntegration:    debateIntegration,
		analyticsIntegration: analyticsIntegration,
		entityIntegration:    entityIntegration,
		logger:               logger,
		enableBigData:        enableBigData,
	}
}

// RunDebate runs a debate with big data integration
func (dsw *DebateServiceWrapper) RunDebate(ctx context.Context, config *services.DebateConfig) (*services.DebateResult, error) {
	startTime := time.Now()

	// Extract conversation ID from metadata
	conversationID := ""
	if metadata, ok := config.Metadata["conversation_id"].(string); ok {
		conversationID = metadata
	}

	// Get unlimited context if conversation ID is provided
	if dsw.enableBigData && conversationID != "" {
		conversationCtx, err := dsw.debateIntegration.GetConversationContext(
			ctx,
			conversationID,
			4000, // Default max tokens
		)
		if err != nil {
			dsw.logger.WithError(err).Warn("Failed to get conversation context, continuing without context")
		} else {
			// Store context in metadata (since config.Context doesn't exist)
			if config.Metadata == nil {
				config.Metadata = make(map[string]any)
			}
			config.Metadata["conversation_context"] = conversationCtx
			dsw.logger.WithFields(logrus.Fields{
				"conversation_id": conversationID,
				"message_count":   len(conversationCtx.Messages),
				"entity_count":    len(conversationCtx.Entities),
				"compressed":      conversationCtx.Compressed,
			}).Info("Loaded conversation context from Kafka")
		}
	}

	// Run the actual debate using ConductDebate (since RunDebate doesn't exist)
	result, err := dsw.debateService.ConductDebate(ctx, config)
	if err != nil {
		return nil, err
	}

	duration := time.Since(startTime)

	// Publish completion event to big data pipeline
	if dsw.enableBigData {
		//nolint:gosec // G118: fire-and-forget bigdata publish, intentionally decoupled from the debate request context
		go dsw.publishDebateCompletion(context.Background(), config, result, duration)
	}

	return result, nil
}

// publishDebateCompletion publishes debate completion events to big data systems
func (dsw *DebateServiceWrapper) publishDebateCompletion(ctx context.Context, config *services.DebateConfig, result *services.DebateResult, duration time.Duration) {
	// Extract metadata from config
	conversationID := ""
	if metadata, ok := config.Metadata["conversation_id"].(string); ok {
		conversationID = metadata
	}
	userID := ""
	if metadata, ok := config.Metadata["user_id"].(string); ok {
		userID = metadata
	}
	sessionID := ""
	if metadata, ok := config.Metadata["session_id"].(string); ok {
		sessionID = metadata
	}

	// Determine winner and confidence from consensus
	winner := ""
	confidence := 0.0
	winningParticipantName := ""
	winnerProvider := ""
	winnerModel := ""
	if result.Consensus != nil {
		winner = result.Consensus.FinalPosition
		confidence = result.Consensus.Confidence
		// Get the actual winning participant name from voting summary
		if result.Consensus.VotingSummary.Winner != "" {
			winningParticipantName = result.Consensus.VotingSummary.Winner
			// Find the winning participant to get provider and model
			for _, p := range result.Participants {
				if p.ParticipantName == winningParticipantName {
					winnerProvider = p.LLMProvider
					winnerModel = p.LLMModel
					break
				}
			}
		}
	}
	// Fallback to extraction from winner string if provider/model not found
	if winnerProvider == "" {
		winnerProvider = dsw.extractProviderFromWinner(winner)
	}
	if winnerModel == "" {
		winnerModel = dsw.extractModelFromWinner(winner)
	}

	// Publish to Kafka for conversation event log
	completion := &DebateCompletion{
		DebateID:       result.DebateID,
		ConversationID: conversationID,
		UserID:         userID,
		SessionID:      sessionID,
		Topic:          config.Topic,
		Rounds:         result.TotalRounds,
		Winner:         winner,
		WinnerProvider: winnerProvider,
		WinnerModel:    winnerModel,
		Confidence:     confidence,
		Duration:       duration,
		StartedAt:      result.StartTime,
		CompletedAt:    result.EndTime,
		Participants:   dsw.convertParticipants(result.Participants, winningParticipantName),
		Outcome:        dsw.determineOutcome(result),
	}

	if err := dsw.debateIntegration.PublishDebateCompletion(ctx, completion); err != nil {
		dsw.logger.WithError(err).Error("Failed to publish debate completion")
	}

	// Publish debate metrics to analytics
	if dsw.analyticsIntegration != nil {
		totalTokens := dsw.calculateTotalTokens(result.Participants)
		if err := dsw.analyticsIntegration.RecordDebateCompletion(
			ctx,
			result.DebateID,
			config.Topic,
			result.TotalRounds,
			duration,
			len(result.Participants),
			winner,
			completion.WinnerProvider,
			completion.WinnerModel,
			confidence,
			totalTokens,
			completion.Outcome,
		); err != nil {
			dsw.logger.WithError(err).Error("Failed to record debate metrics")
		}
	}

}

// Helper functions

func (dsw *DebateServiceWrapper) extractProviderFromWinner(winner string) string {
	// Winner format: "provider/model" or just "provider"
	// Extract provider name before the slash
	for i, char := range winner {
		if char == '/' {
			return winner[:i]
		}
	}
	return winner
}

func (dsw *DebateServiceWrapper) extractModelFromWinner(winner string) string {
	// Extract model name after the slash
	for i, char := range winner {
		if char == '/' {
			if i+1 < len(winner) {
				return winner[i+1:]
			}
			return ""
		}
	}
	return ""
}

func (dsw *DebateServiceWrapper) convertParticipants(participants []services.ParticipantResponse, winningParticipantName string) []DebateParticipant {
	result := make([]DebateParticipant, len(participants))

	for i, p := range participants {
		tokensUsed := 0
		if p.Metadata != nil {
			if val, ok := p.Metadata["tokens_used"]; ok {
				switch v := val.(type) {
				case int:
					tokensUsed = v
				case float64:
					tokensUsed = int(v)
				}
			}
		}

		won := false
		if winningParticipantName != "" && p.ParticipantName == winningParticipantName {
			won = true
		}

		result[i] = DebateParticipant{
			Provider:     p.LLMProvider,
			Model:        p.LLMModel,
			Position:     p.Role,
			ResponseTime: int(p.ResponseTime.Milliseconds()),
			TokensUsed:   tokensUsed,
			Confidence:   p.Confidence,
			Won:          won,
		}
	}
	return result
}

func (dsw *DebateServiceWrapper) determineOutcome(result *services.DebateResult) string {
	if result.ErrorMessage != "" {
		return "error"
	}
	if !result.Success {
		return "abandoned"
	}
	return "successful"
}

func (dsw *DebateServiceWrapper) calculateTotalTokens(participants []services.ParticipantResponse) int {
	total := 0
	// ParticipantResponse doesn't have token count, so we estimate based on response length
	for _, p := range participants {
		// Estimate ~4 chars per token
		total += len(p.Response) / 4
	}
	return total
}

// RecordProviderCall records a provider API call for analytics
func (dsw *DebateServiceWrapper) RecordProviderCall(
	ctx context.Context,
	provider, model, requestID string,
	responseTime time.Duration,
	tokensUsed int,
	success bool,
	errorType string,
) {
	if !dsw.enableBigData || dsw.analyticsIntegration == nil {
		return
	}

	if err := dsw.analyticsIntegration.RecordProviderRequest(
		ctx,
		provider,
		model,
		requestID,
		responseTime,
		tokensUsed,
		success,
		errorType,
	); err != nil {
		dsw.logger.WithError(err).Debug("Failed to record provider call")
	}
}

// RecordDebateRound records a single debate round for analytics
func (dsw *DebateServiceWrapper) RecordDebateRound(
	ctx context.Context,
	debateID, provider, model string,
	round int,
	responseTime time.Duration,
	tokensUsed int,
	confidence float64,
) {
	if !dsw.enableBigData || dsw.analyticsIntegration == nil {
		return
	}

	if err := dsw.analyticsIntegration.RecordDebateRound(
		ctx,
		debateID,
		provider,
		model,
		round,
		responseTime,
		tokensUsed,
		confidence,
	); err != nil {
		dsw.logger.WithError(err).Debug("Failed to record debate round")
	}
}
