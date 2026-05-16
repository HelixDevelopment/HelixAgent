package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"digital.vasic.debate/comprehensive"
)

// SetComprehensiveIntegration sets the comprehensive debate integration and enables it
func (ds *DebateService) SetComprehensiveIntegration(integration *comprehensive.IntegrationManager) {
	ds.comprehensiveIntegration = integration
	ds.useComprehensiveSystem = true
	ds.logger.Info("[Comprehensive Debate] Integration configured and enabled")
}

// EnableComprehensiveSystem enables the comprehensive debate system
func (ds *DebateService) EnableComprehensiveSystem(enable bool) {
	ds.useComprehensiveSystem = enable
	ds.logger.WithField("enabled", enable).Info("[Comprehensive Debate] System toggled")
}

// conductComprehensiveDebate executes a debate using the new comprehensive multi-agent system
func (ds *DebateService) conductComprehensiveDebate(
	ctx context.Context,
	config *DebateConfig,
	startTime time.Time,
	sessionID string,
) (*DebateResult, error) {
	ds.logger.WithFields(logrus.Fields{
		"debate_id": config.DebateID,
		"topic":     config.Topic,
	}).Info("[Comprehensive Debate] Starting multi-agent debate")

	// Convert service DebateConfig to comprehensive DebateRequest
	compReq := &comprehensive.DebateRequest{
		ID:        config.DebateID,
		Topic:     config.Topic,
		Context:   config.Topic, // Use topic as context for now
		Language:  "go",         // Default to Go, could be detected from topic
		MaxRounds: 3,            // Default rounds
	}

	// Execute debate through comprehensive system
	compResp, err := ds.comprehensiveIntegration.ExecuteDebate(ctx, compReq)
	if err != nil {
		ds.logger.WithError(err).Error("[Comprehensive Debate] Debate execution failed")
		return nil, fmt.Errorf("comprehensive debate failed: %w", err)
	}

	endTime := time.Now()

	// Convert comprehensive response to service DebateResult
	result := &DebateResult{
		DebateID:        config.DebateID,
		SessionID:       sessionID,
		Topic:           config.Topic,
		StartTime:       startTime,
		EndTime:         endTime,
		Duration:        endTime.Sub(startTime),
		TotalRounds:     compResp.RoundsConducted,
		RoundsConducted: compResp.RoundsConducted,
		Participants:    []ParticipantResponse{},
		Consensus: &ConsensusResult{
			Reached:        compResp.Success,
			Achieved:       compResp.Success,
			Confidence:     compResp.QualityScore,
			AgreementLevel: compResp.QualityScore,
			FinalPosition:  fmt.Sprintf("Comprehensive debate completed with %d rounds", compResp.RoundsConducted),
			Summary:        fmt.Sprintf("Comprehensive debate completed with %d rounds", compResp.RoundsConducted),
			KeyPoints:      []string{},
			Disagreements:  []string{},
			Timestamp:      endTime,
			QualityScore:   compResp.QualityScore,
		},
		QualityScore: compResp.QualityScore,
		FinalScore:   compResp.QualityScore,
		Success:      compResp.Success,
		Metadata: map[string]any{
			"comprehensive_debate": true,
			"rounds_conducted":     compResp.RoundsConducted,
			"phases":               len(compResp.Phases),
		},
	}

	ds.logger.WithFields(logrus.Fields{
		"debate_id":     config.DebateID,
		"success":       result.Success,
		"total_rounds":  result.TotalRounds,
		"quality_score": result.QualityScore,
		"duration":      result.Duration,
	}).Info("[Comprehensive Debate] Multi-agent debate completed")

	return result, nil
}

// conductComprehensiveDebateStreaming executes a streaming debate using the comprehensive system
func (ds *DebateService) conductComprehensiveDebateStreaming(
	ctx context.Context,
	config *DebateConfig,
	startTime time.Time,
	sessionID string,
	streamHandler comprehensive.StreamHandler,
) (*DebateResult, error) {
	ds.logger.WithFields(logrus.Fields{
		"debate_id": config.DebateID,
		"topic":     config.Topic,
	}).Info("[Comprehensive Debate] Starting streaming multi-agent debate")

	// Convert service DebateConfig to comprehensive DebateStreamRequest
	compReq := &comprehensive.DebateStreamRequest{
		DebateRequest: &comprehensive.DebateRequest{
			ID:        config.DebateID,
			Topic:     config.Topic,
			Context:   config.Topic,
			Language:  "go",
			MaxRounds: 3,
		},
		Stream:        true,
		StreamHandler: streamHandler,
	}

	// Execute streaming debate through comprehensive system
	compResp, err := ds.comprehensiveIntegration.StreamDebateRequest(ctx, compReq)
	if err != nil {
		ds.logger.WithError(err).Error("[Comprehensive Debate] Streaming debate execution failed")
		return nil, fmt.Errorf("comprehensive streaming debate failed: %w", err)
	}

	endTime := time.Now()

	// Build participant responses from comprehensive agents
	participants := make([]ParticipantResponse, 0, len(compResp.Participants))
	for _, agentID := range compResp.Participants {
		// Note: In a full implementation, we'd look up the actual agent details
		participants = append(participants, ParticipantResponse{
			ParticipantID: agentID,
			Response:      "Contributed to comprehensive debate",
			Timestamp:     endTime,
		})
	}

	// Convert comprehensive response to service DebateResult
	result := &DebateResult{
		DebateID:        config.DebateID,
		SessionID:       sessionID,
		Topic:           config.Topic,
		StartTime:       startTime,
		EndTime:         endTime,
		Duration:        endTime.Sub(startTime),
		TotalRounds:     compResp.RoundsConducted,
		RoundsConducted: compResp.RoundsConducted,
		Participants:    participants,
		AllResponses:    participants,
		Consensus: &ConsensusResult{
			Reached:        compResp.Success,
			Achieved:       compResp.Success,
			Confidence:     compResp.QualityScore,
			AgreementLevel: compResp.QualityScore,
			FinalPosition:  fmt.Sprintf("Comprehensive streaming debate completed with %d rounds", compResp.RoundsConducted),
			Summary:        fmt.Sprintf("Comprehensive streaming debate completed with %d rounds", compResp.RoundsConducted),
			KeyPoints:      []string{},
			Disagreements:  []string{},
			Timestamp:      endTime,
			QualityScore:   compResp.QualityScore,
		},
		QualityScore: compResp.QualityScore,
		FinalScore:   compResp.QualityScore,
		Success:      compResp.Success,
		Metadata: map[string]any{
			"comprehensive_debate": true,
			"streaming":            true,
			"rounds_conducted":     compResp.RoundsConducted,
			"phases":               len(compResp.Phases),
			"participants":         len(compResp.Participants),
		},
	}

	ds.logger.WithFields(logrus.Fields{
		"debate_id":     config.DebateID,
		"success":       result.Success,
		"total_rounds":  result.TotalRounds,
		"quality_score": result.QualityScore,
		"duration":      result.Duration,
		"participants":  len(participants),
	}).Info("[Comprehensive Debate] Streaming multi-agent debate completed")

	return result, nil
}
