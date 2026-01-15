package verifier

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/messaging"
)

func TestVerificationEventType_String(t *testing.T) {
	tests := []struct {
		eventType VerificationEventType
		expected  string
	}{
		{VerificationEventStarted, "verification.started"},
		{VerificationEventDiscovered, "verification.provider.discovered"},
		{VerificationEventVerified, "verification.provider.verified"},
		{VerificationEventFailed, "verification.provider.failed"},
		{VerificationEventScored, "verification.provider.scored"},
		{VerificationEventRanked, "verification.provider.ranked"},
		{VerificationEventHealthCheck, "verification.provider.health"},
		{VerificationEventTeamSelected, "verification.debate_team.selected"},
		{VerificationEventCompleted, "verification.completed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.String())
		})
	}
}

func TestVerificationEventType_Topic(t *testing.T) {
	tests := []struct {
		eventType VerificationEventType
		expected  string
	}{
		{VerificationEventDiscovered, TopicProviderDiscovered},
		{VerificationEventVerified, TopicProviderVerified},
		{VerificationEventFailed, TopicProviderVerified},
		{VerificationEventScored, TopicProviderScored},
		{VerificationEventRanked, TopicProviderScored},
		{VerificationEventHealthCheck, TopicProviderHealthCheck},
		{VerificationEventTeamSelected, TopicDebateTeamSelected},
		{VerificationEventCompleted, TopicVerificationComplete},
		{VerificationEventType("unknown"), TopicVerificationEvents},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.Topic())
		})
	}
}

func TestNewVerificationEvent(t *testing.T) {
	event := NewVerificationEvent(VerificationEventStarted)

	assert.NotEmpty(t, event.ID)
	assert.Equal(t, VerificationEventStarted, event.Type)
	assert.NotZero(t, event.Timestamp)
	assert.NotNil(t, event.Details)
}

func TestNewProviderVerificationEvent(t *testing.T) {
	provider := &UnifiedProvider{
		ID:       "test-provider",
		Name:     "Test Provider",
		AuthType: AuthTypeAPIKey,
		Score:    8.5,
		Verified: true,
	}

	event := NewProviderVerificationEvent(VerificationEventVerified, provider)

	assert.NotEmpty(t, event.ID)
	assert.Equal(t, VerificationEventVerified, event.Type)
	assert.Equal(t, "test-provider", event.ProviderID)
	assert.Equal(t, "Test Provider", event.ProviderName)
	assert.Equal(t, "api_key", event.ProviderType)
	assert.Equal(t, 8.5, event.Score)
	assert.True(t, event.Verified)
}

func TestNewProviderVerificationEvent_NilProvider(t *testing.T) {
	event := NewProviderVerificationEvent(VerificationEventVerified, nil)

	assert.NotEmpty(t, event.ID)
	assert.Equal(t, VerificationEventVerified, event.Type)
	assert.Empty(t, event.ProviderID)
}

func TestVerificationEvent_ToMessagingEvent(t *testing.T) {
	event := &VerificationEvent{
		ID:            "event-1",
		Type:          VerificationEventVerified,
		ProviderID:    "provider-1",
		ProviderName:  "Test Provider",
		Score:         9.0,
		Verified:      true,
		Phase:         "verification",
		Timestamp:     time.Now().UTC(),
		CorrelationID: "corr-123",
	}

	msgEvent := event.ToMessagingEvent()

	assert.Equal(t, "event-1", msgEvent.ID)
	assert.Equal(t, messaging.EventType("verification.provider.verified"), msgEvent.Type)
	assert.Equal(t, "helixagent.verifier", msgEvent.Source)
	assert.Equal(t, "provider-1", msgEvent.Subject)
	assert.Equal(t, "corr-123", msgEvent.CorrelationID)
	assert.NotEmpty(t, msgEvent.Data)
}

func TestDefaultVerificationEventPublisherConfig(t *testing.T) {
	config := DefaultVerificationEventPublisherConfig()

	assert.True(t, config.Enabled)
}

func TestNewVerificationEventPublisher(t *testing.T) {
	config := DefaultVerificationEventPublisherConfig()
	publisher := NewVerificationEventPublisher(nil, nil, config)

	assert.NotNil(t, publisher)
	assert.NotEmpty(t, publisher.GetCorrelationID())
}

func TestNewVerificationEventPublisher_NilConfig(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)

	assert.NotNil(t, publisher)
	assert.True(t, publisher.enabled)
}

func TestVerificationEventPublisher_IsEnabled_NoHub(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)

	// Should be false when hub is nil
	assert.False(t, publisher.IsEnabled())
}

func TestVerificationEventPublisher_SetCorrelationID(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)

	publisher.SetCorrelationID("custom-corr-id")
	assert.Equal(t, "custom-corr-id", publisher.GetCorrelationID())
}

func TestVerificationEventPublisher_Publish_NoHub(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	event := NewVerificationEvent(VerificationEventStarted)

	// Should not error when hub is nil
	err := publisher.Publish(context.Background(), event)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_Publish_NilEvent(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)

	err := publisher.Publish(context.Background(), nil)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishStarted(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)

	err := publisher.PublishStarted(context.Background(), 10)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishProviderDiscovered(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	provider := &UnifiedProvider{
		ID:       "test-provider",
		Name:     "Test Provider",
		AuthType: AuthTypeOAuth,
		Models:   []UnifiedModel{},
	}

	err := publisher.PublishProviderDiscovered(context.Background(), provider)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishProviderVerified(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	provider := &UnifiedProvider{
		ID:       "test-provider",
		Verified: true,
		Score:    8.5,
	}

	err := publisher.PublishProviderVerified(context.Background(), provider)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishProviderFailed(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	provider := &UnifiedProvider{
		ID: "test-provider",
	}

	err := publisher.PublishProviderFailed(context.Background(), provider, assert.AnError)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishProviderScored(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	provider := &UnifiedProvider{
		ID:    "test-provider",
		Score: 9.0,
		TestResults: map[string]bool{
			"basic":    true,
			"advanced": true,
		},
	}

	err := publisher.PublishProviderScored(context.Background(), provider)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishProviderRanked(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	provider := &UnifiedProvider{
		ID:    "test-provider",
		Score: 9.0,
	}

	err := publisher.PublishProviderRanked(context.Background(), provider, 1)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishHealthCheck(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	provider := &UnifiedProvider{
		ID: "test-provider",
	}

	err := publisher.PublishHealthCheck(context.Background(), provider, true, 150)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishDebateTeamSelected(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	team := &DebateTeamResult{
		TotalLLMs:  15,
		OAuthFirst: true,
		Positions: []*DebatePosition{
			{
				Position: 1,
				Role:     "Proponent",
				Primary: &DebateLLM{
					Provider:  "claude",
					ModelName: "claude-3-opus",
					Score:     9.5,
				},
			},
		},
	}

	err := publisher.PublishDebateTeamSelected(context.Background(), team)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishDebateTeamSelected_NilTeam(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)

	err := publisher.PublishDebateTeamSelected(context.Background(), nil)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishCompleted(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)
	result := &StartupResult{
		TotalProviders:  10,
		VerifiedCount:   8,
		FailedCount:     2,
		SkippedCount:    0,
		OAuthProviders:  2,
		APIKeyProviders: 5,
		FreeProviders:   1,
		DurationMs:      5000,
		Errors:          []StartupError{},
	}

	err := publisher.PublishCompleted(context.Background(), result)
	assert.NoError(t, err)
}

func TestVerificationEventPublisher_PublishCompleted_NilResult(t *testing.T) {
	publisher := NewVerificationEventPublisher(nil, nil, nil)

	err := publisher.PublishCompleted(context.Background(), nil)
	assert.NoError(t, err)
}

func TestTopicConstants(t *testing.T) {
	assert.Equal(t, "helixagent.events.verification", TopicVerificationEvents)
	assert.Equal(t, "helixagent.events.verification.discovered", TopicProviderDiscovered)
	assert.Equal(t, "helixagent.events.verification.verified", TopicProviderVerified)
	assert.Equal(t, "helixagent.events.verification.scored", TopicProviderScored)
	assert.Equal(t, "helixagent.events.verification.health", TopicProviderHealthCheck)
	assert.Equal(t, "helixagent.events.verification.debate_team", TopicDebateTeamSelected)
	assert.Equal(t, "helixagent.events.verification.complete", TopicVerificationComplete)
}

func TestGenerateVerificationEventID(t *testing.T) {
	id1 := generateVerificationEventID()
	time.Sleep(time.Millisecond)
	id2 := generateVerificationEventID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestRandomVerificationString(t *testing.T) {
	str1 := randomVerificationString(8)
	time.Sleep(time.Millisecond)
	str2 := randomVerificationString(8)

	require.Len(t, str1, 8)
	require.Len(t, str2, 8)
	assert.NotEqual(t, str1, str2)
}
