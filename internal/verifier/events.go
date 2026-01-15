// Package verifier provides event types for verification pipeline events.
package verifier

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/messaging"
)

// Verification event topics.
const (
	TopicVerificationEvents   = "helixagent.events.verification"
	TopicProviderDiscovered   = "helixagent.events.verification.discovered"
	TopicProviderVerified     = "helixagent.events.verification.verified"
	TopicProviderScored       = "helixagent.events.verification.scored"
	TopicProviderHealthCheck  = "helixagent.events.verification.health"
	TopicDebateTeamSelected   = "helixagent.events.verification.debate_team"
	TopicVerificationComplete = "helixagent.events.verification.complete"
)

// VerificationEventType represents the type of verification event.
type VerificationEventType string

const (
	VerificationEventStarted       VerificationEventType = "verification.started"
	VerificationEventDiscovered    VerificationEventType = "verification.provider.discovered"
	VerificationEventVerified      VerificationEventType = "verification.provider.verified"
	VerificationEventFailed        VerificationEventType = "verification.provider.failed"
	VerificationEventScored        VerificationEventType = "verification.provider.scored"
	VerificationEventRanked        VerificationEventType = "verification.provider.ranked"
	VerificationEventHealthCheck   VerificationEventType = "verification.provider.health"
	VerificationEventTeamSelected  VerificationEventType = "verification.debate_team.selected"
	VerificationEventCompleted     VerificationEventType = "verification.completed"
)

// String returns the string representation of the event type.
func (e VerificationEventType) String() string {
	return string(e)
}

// Topic returns the appropriate topic for this event type.
func (e VerificationEventType) Topic() string {
	switch e {
	case VerificationEventDiscovered:
		return TopicProviderDiscovered
	case VerificationEventVerified, VerificationEventFailed:
		return TopicProviderVerified
	case VerificationEventScored, VerificationEventRanked:
		return TopicProviderScored
	case VerificationEventHealthCheck:
		return TopicProviderHealthCheck
	case VerificationEventTeamSelected:
		return TopicDebateTeamSelected
	case VerificationEventCompleted:
		return TopicVerificationComplete
	default:
		return TopicVerificationEvents
	}
}

// VerificationEvent represents a verification pipeline event.
type VerificationEvent struct {
	// ID is the unique event identifier.
	ID string `json:"id"`
	// Type is the event type.
	Type VerificationEventType `json:"type"`
	// ProviderID is the provider this event relates to (if applicable).
	ProviderID string `json:"provider_id,omitempty"`
	// ProviderName is the provider name.
	ProviderName string `json:"provider_name,omitempty"`
	// ProviderType is the provider type (api_key, oauth, free).
	ProviderType string `json:"provider_type,omitempty"`
	// ModelID is the model this event relates to (if applicable).
	ModelID string `json:"model_id,omitempty"`
	// Score is the provider score (if applicable).
	Score float64 `json:"score,omitempty"`
	// Verified indicates if verification passed.
	Verified bool `json:"verified,omitempty"`
	// Error contains error information if applicable.
	Error string `json:"error,omitempty"`
	// Phase indicates the pipeline phase.
	Phase string `json:"phase,omitempty"`
	// Details contains additional event data.
	Details map[string]interface{} `json:"details,omitempty"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// CorrelationID links related events.
	CorrelationID string `json:"correlation_id,omitempty"`
}

// NewVerificationEvent creates a new verification event.
func NewVerificationEvent(eventType VerificationEventType) *VerificationEvent {
	return &VerificationEvent{
		ID:        generateVerificationEventID(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Details:   make(map[string]interface{}),
	}
}

// NewProviderVerificationEvent creates an event for a provider.
func NewProviderVerificationEvent(eventType VerificationEventType, provider *UnifiedProvider) *VerificationEvent {
	event := NewVerificationEvent(eventType)
	if provider != nil {
		event.ProviderID = provider.ID
		event.ProviderName = provider.Name
		event.ProviderType = string(provider.AuthType)
		event.Score = provider.Score
		event.Verified = provider.Verified
	}
	return event
}

// ToMessagingEvent converts VerificationEvent to messaging.Event.
func (e *VerificationEvent) ToMessagingEvent() *messaging.Event {
	data, _ := json.Marshal(e)
	return &messaging.Event{
		ID:            e.ID,
		Type:          messaging.EventType(e.Type),
		Source:        "helixagent.verifier",
		Subject:       e.ProviderID,
		Data:          data,
		DataSchema:    "application/json",
		Timestamp:     e.Timestamp,
		CorrelationID: e.CorrelationID,
	}
}

// VerificationEventPublisher publishes verification events.
type VerificationEventPublisher struct {
	hub           *messaging.MessagingHub
	logger        *logrus.Logger
	enabled       bool
	correlationID string
}

// VerificationEventPublisherConfig holds configuration.
type VerificationEventPublisherConfig struct {
	// Enabled enables event publishing.
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// DefaultVerificationEventPublisherConfig returns default configuration.
func DefaultVerificationEventPublisherConfig() *VerificationEventPublisherConfig {
	return &VerificationEventPublisherConfig{
		Enabled: true,
	}
}

// NewVerificationEventPublisher creates a new verification event publisher.
func NewVerificationEventPublisher(
	hub *messaging.MessagingHub,
	logger *logrus.Logger,
	config *VerificationEventPublisherConfig,
) *VerificationEventPublisher {
	if config == nil {
		config = DefaultVerificationEventPublisherConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &VerificationEventPublisher{
		hub:           hub,
		logger:        logger,
		enabled:       config.Enabled,
		correlationID: generateVerificationEventID(), // Correlation ID for this verification run
	}
}

// SetCorrelationID sets the correlation ID for all events.
func (p *VerificationEventPublisher) SetCorrelationID(id string) {
	p.correlationID = id
}

// GetCorrelationID returns the correlation ID.
func (p *VerificationEventPublisher) GetCorrelationID() string {
	return p.correlationID
}

// IsEnabled returns whether publishing is enabled.
func (p *VerificationEventPublisher) IsEnabled() bool {
	return p.enabled && p.hub != nil
}

// Publish publishes a verification event.
func (p *VerificationEventPublisher) Publish(ctx context.Context, event *VerificationEvent) error {
	if !p.enabled || p.hub == nil || event == nil {
		return nil
	}

	event.CorrelationID = p.correlationID
	topic := event.Type.Topic()
	msgEvent := event.ToMessagingEvent()

	if err := p.hub.PublishEvent(ctx, topic, msgEvent); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"event_type":  event.Type,
			"provider_id": event.ProviderID,
			"topic":       topic,
		}).Debug("Failed to publish verification event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"event_type":  event.Type,
		"provider_id": event.ProviderID,
	}).Debug("Published verification event")

	return nil
}

// PublishStarted publishes a verification started event.
func (p *VerificationEventPublisher) PublishStarted(ctx context.Context, totalProviders int) error {
	event := NewVerificationEvent(VerificationEventStarted)
	event.Phase = "started"
	event.Details["total_providers"] = totalProviders
	return p.Publish(ctx, event)
}

// PublishProviderDiscovered publishes a provider discovered event.
func (p *VerificationEventPublisher) PublishProviderDiscovered(ctx context.Context, provider *UnifiedProvider) error {
	event := NewProviderVerificationEvent(VerificationEventDiscovered, provider)
	event.Phase = "discovery"
	if provider != nil {
		event.Details["auth_type"] = provider.AuthType
		event.Details["model_count"] = len(provider.Models)
	}
	return p.Publish(ctx, event)
}

// PublishProviderVerified publishes a provider verified event.
func (p *VerificationEventPublisher) PublishProviderVerified(ctx context.Context, provider *UnifiedProvider) error {
	event := NewProviderVerificationEvent(VerificationEventVerified, provider)
	event.Phase = "verification"
	event.Verified = true
	return p.Publish(ctx, event)
}

// PublishProviderFailed publishes a provider verification failed event.
func (p *VerificationEventPublisher) PublishProviderFailed(ctx context.Context, provider *UnifiedProvider, err error) error {
	event := NewProviderVerificationEvent(VerificationEventFailed, provider)
	event.Phase = "verification"
	event.Verified = false
	if err != nil {
		event.Error = err.Error()
	}
	return p.Publish(ctx, event)
}

// PublishProviderScored publishes a provider scored event.
func (p *VerificationEventPublisher) PublishProviderScored(ctx context.Context, provider *UnifiedProvider) error {
	event := NewProviderVerificationEvent(VerificationEventScored, provider)
	event.Phase = "scoring"
	if provider != nil {
		event.Details["test_results"] = provider.TestResults
	}
	return p.Publish(ctx, event)
}

// PublishProviderRanked publishes a provider ranked event.
func (p *VerificationEventPublisher) PublishProviderRanked(ctx context.Context, provider *UnifiedProvider, rank int) error {
	event := NewProviderVerificationEvent(VerificationEventRanked, provider)
	event.Phase = "ranking"
	event.Details["rank"] = rank
	return p.Publish(ctx, event)
}

// PublishHealthCheck publishes a health check event.
func (p *VerificationEventPublisher) PublishHealthCheck(ctx context.Context, provider *UnifiedProvider, healthy bool, latencyMs int64) error {
	event := NewProviderVerificationEvent(VerificationEventHealthCheck, provider)
	event.Phase = "health"
	event.Details["healthy"] = healthy
	event.Details["latency_ms"] = latencyMs
	return p.Publish(ctx, event)
}

// PublishDebateTeamSelected publishes a debate team selected event.
func (p *VerificationEventPublisher) PublishDebateTeamSelected(ctx context.Context, team *DebateTeamResult) error {
	event := NewVerificationEvent(VerificationEventTeamSelected)
	event.Phase = "debate_team"
	if team != nil {
		event.Details["total_llms"] = team.TotalLLMs
		event.Details["position_count"] = len(team.Positions)
		event.Details["oauth_first"] = team.OAuthFirst
		primaryModels := make([]string, 0, len(team.Positions))
		for _, pos := range team.Positions {
			if pos != nil && pos.Primary != nil {
				primaryModels = append(primaryModels, pos.Primary.Provider+":"+pos.Primary.ModelName)
			}
		}
		event.Details["primary_models"] = primaryModels
	}
	return p.Publish(ctx, event)
}

// PublishCompleted publishes a verification completed event.
func (p *VerificationEventPublisher) PublishCompleted(ctx context.Context, result *StartupResult) error {
	event := NewVerificationEvent(VerificationEventCompleted)
	event.Phase = "completed"
	if result != nil {
		event.Details["total_providers"] = result.TotalProviders
		event.Details["verified_count"] = result.VerifiedCount
		event.Details["failed_count"] = result.FailedCount
		event.Details["skipped_count"] = result.SkippedCount
		event.Details["oauth_providers"] = result.OAuthProviders
		event.Details["api_key_providers"] = result.APIKeyProviders
		event.Details["free_providers"] = result.FreeProviders
		event.Details["duration_ms"] = result.DurationMs
		event.Details["error_count"] = len(result.Errors)
	}
	return p.Publish(ctx, event)
}

// generateVerificationEventID generates a unique event ID.
func generateVerificationEventID() string {
	return time.Now().UTC().Format("20060102150405.000000000") + "-" + randomVerificationString(8)
}

// randomVerificationString generates a random string.
func randomVerificationString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
