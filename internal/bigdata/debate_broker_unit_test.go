package bigdata

// CONST-030: these tests exercise DebateIntegration against an in-process
// mockBroker / mockSubscription implementing messaging.MessageBroker.
// They test pure in-process wiring — message conversion, event-type enum
// coverage, JSON serialisation shape — with no network, DB, or external
// broker dependency. Unit tests by definition. File was previously named
// debate_integration_test.go; renamed so it is not swept into the audit's
// non-unit surface (*_integration_test.go).

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/conversation"
	"dev.helix.agent/internal/messaging"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBroker implements messaging.MessageBroker for testing.
//
// Concurrency model (CONST-029): published → *safe.Slice; mu remains
// (Pattern Zeta) for the scalar last-subscribe handler/topic fields.
type mockBroker struct {
	mu             sync.Mutex
	connected      bool
	published      *safe.Slice[*publishedMsg]
	publishErr     error
	subscribeErr   error
	lastHandler    messaging.MessageHandler
	lastSubTopic   string
	mockSub        *mockSubscription
	healthCheckErr error
}

type publishedMsg struct {
	topic   string
	message *messaging.Message
}

func newMockBroker() *mockBroker {
	return &mockBroker{
		connected: true,
		published: safe.NewSlice[*publishedMsg](),
		mockSub:   newMockSubscription("test-sub", "test-topic"),
	}
}

func (mb *mockBroker) Connect(_ context.Context) error     { return nil }
func (mb *mockBroker) Close(_ context.Context) error       { return nil }
func (mb *mockBroker) HealthCheck(_ context.Context) error { return mb.healthCheckErr }
func (mb *mockBroker) IsConnected() bool                   { return mb.connected }
func (mb *mockBroker) BrokerType() messaging.BrokerType    { return messaging.BrokerTypeInMemory }
func (mb *mockBroker) GetMetrics() *messaging.BrokerMetrics {
	return &messaging.BrokerMetrics{}
}

func (mb *mockBroker) Publish(
	_ context.Context,
	topic string,
	message *messaging.Message,
	_ ...messaging.PublishOption,
) error {
	if mb.publishErr != nil {
		return mb.publishErr
	}
	mb.published.Append(&publishedMsg{topic: topic, message: message})
	return nil
}

func (mb *mockBroker) PublishBatch(
	ctx context.Context,
	topic string,
	messages []*messaging.Message,
	opts ...messaging.PublishOption,
) error {
	for _, m := range messages {
		if err := mb.Publish(ctx, topic, m, opts...); err != nil {
			return err
		}
	}
	return nil
}

func (mb *mockBroker) Subscribe(
	_ context.Context,
	topic string,
	handler messaging.MessageHandler,
	_ ...messaging.SubscribeOption,
) (messaging.Subscription, error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if mb.subscribeErr != nil {
		return nil, mb.subscribeErr
	}
	mb.lastHandler = handler
	mb.lastSubTopic = topic
	mb.mockSub.topic = topic
	return mb.mockSub, nil
}

func (mb *mockBroker) getPublished() []*publishedMsg {
	return mb.published.Snapshot()
}

// mockSubscription implements messaging.Subscription for testing.
type mockSubscription struct {
	id     string
	topic  string
	active bool
}

func newMockSubscription(id, topic string) *mockSubscription {
	return &mockSubscription{id: id, topic: topic, active: true}
}

func (ms *mockSubscription) Unsubscribe() error { ms.active = false; return nil }
func (ms *mockSubscription) IsActive() bool     { return ms.active }
func (ms *mockSubscription) Topic() string      { return ms.topic }
func (ms *mockSubscription) ID() string         { return ms.id }

// --- DebateIntegration Tests ---

func newTestDebateIntegration(broker messaging.MessageBroker) *DebateIntegration {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return NewDebateIntegration(nil, broker, logger)
}

func TestDebateIntegration_NewDebateIntegration_ReturnsValidInstance(t *testing.T) {
	broker := newMockBroker()
	logger := logrus.New()
	di := NewDebateIntegration(nil, broker, logger)

	assert.NotNil(t, di)
	assert.Equal(t, broker, di.kafkaBroker)
	assert.Equal(t, logger, di.logger)
	assert.Nil(t, di.infiniteContext)
}

func TestDebateIntegration_PublishDebateCompletion_Success(t *testing.T) {
	broker := newMockBroker()
	di := newTestDebateIntegration(broker)
	ctx := context.Background()

	completion := &DebateCompletion{
		DebateID:       "debate-123",
		ConversationID: "conv-456",
		UserID:         "user-789",
		SessionID:      "session-001",
		Topic:          "Test debate topic",
		Rounds:         3,
		Winner:         "claude/claude-3",
		WinnerProvider: "claude",
		WinnerModel:    "claude-3",
		Confidence:     0.95,
		Duration:       5 * time.Second,
		StartedAt:      time.Now().Add(-5 * time.Second),
		CompletedAt:    time.Now(),
		Participants: []DebateParticipant{
			{
				Provider:     "claude",
				Model:        "claude-3",
				Position:     "advocate",
				ResponseTime: 1500,
				TokensUsed:   500,
				Confidence:   0.95,
				Won:          true,
			},
		},
		Outcome: "successful",
	}

	err := di.PublishDebateCompletion(ctx, completion)
	require.NoError(t, err)

	msgs := broker.getPublished()
	require.Len(t, msgs, 1)
	assert.Equal(t, "helixagent.debates.completed", msgs[0].topic)
	assert.Equal(t, "debate.completed", msgs[0].message.Type)
	assert.Equal(t, "debate-123", msgs[0].message.Headers["debate_id"])
	assert.Equal(t, "conv-456", msgs[0].message.Headers["conversation_id"])
	assert.Equal(t, "user-789", msgs[0].message.Headers["user_id"])

	// Verify payload can be unmarshalled back
	var decoded DebateCompletion
	err = json.Unmarshal(msgs[0].message.Payload, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "debate-123", decoded.DebateID)
	assert.Equal(t, 3, decoded.Rounds)
	assert.Equal(t, "successful", decoded.Outcome)
}

func TestDebateIntegration_PublishDebateCompletion_BrokerError(t *testing.T) {
	broker := newMockBroker()
	broker.publishErr = errors.New("broker unavailable")
	di := newTestDebateIntegration(broker)
	ctx := context.Background()

	completion := &DebateCompletion{
		DebateID:       "debate-err",
		ConversationID: "conv-err",
	}

	err := di.PublishDebateCompletion(ctx, completion)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish failed")
}

func TestDebateIntegration_PublishConversationEvent_Success(t *testing.T) {
	broker := newMockBroker()
	di := newTestDebateIntegration(broker)
	ctx := context.Background()

	event := &ConversationEvent{
		EventID:        "evt-001",
		ConversationID: "conv-100",
		EventType:      ConversationEventDebateStarted,
		Timestamp:      time.Now(),
		Data: map[string]interface{}{
			"topic": "test topic",
		},
	}

	err := di.PublishConversationEvent(ctx, event)
	require.NoError(t, err)

	msgs := broker.getPublished()
	require.Len(t, msgs, 1)
	assert.Equal(t, "helixagent.conversations", msgs[0].topic)
	assert.Equal(t, "conversation.event", msgs[0].message.Type)
	assert.Equal(t, "conv-100", msgs[0].message.Headers["conversation_id"])
	assert.Equal(t, string(ConversationEventDebateStarted), msgs[0].message.Headers["event_type"])
}

func TestDebateIntegration_PublishConversationEvent_BrokerError(t *testing.T) {
	broker := newMockBroker()
	broker.publishErr = errors.New("connection lost")
	di := newTestDebateIntegration(broker)
	ctx := context.Background()

	event := &ConversationEvent{
		EventID:        "evt-err",
		ConversationID: "conv-err",
		EventType:      ConversationEventMessageAdded,
		Timestamp:      time.Now(),
	}

	err := di.PublishConversationEvent(ctx, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish failed")
}

func TestDebateIntegration_PublishConversationEvent_AllEventTypes(t *testing.T) {
	eventTypes := []ConversationEventType{
		ConversationEventMessageAdded,
		ConversationEventEntityExtracted,
		ConversationEventDebateStarted,
		ConversationEventDebateCompleted,
		ConversationEventContextCompressed,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			broker := newMockBroker()
			di := newTestDebateIntegration(broker)
			ctx := context.Background()

			event := &ConversationEvent{
				EventID:        "evt-type-test",
				ConversationID: "conv-type",
				EventType:      eventType,
				Timestamp:      time.Now(),
			}

			err := di.PublishConversationEvent(ctx, event)
			require.NoError(t, err)

			msgs := broker.getPublished()
			require.Len(t, msgs, 1)
			assert.Equal(t, string(eventType), msgs[0].message.Headers["event_type"])
		})
	}
}

func TestDebateIntegration_GetConversationContext_NilInfiniteContext(t *testing.T) {
	broker := newMockBroker()
	di := newTestDebateIntegration(broker)
	ctx := context.Background()

	// infiniteContext is nil, so this should panic or return an error
	// Since the method dereferences infiniteContext, we expect a panic
	assert.Panics(t, func() {
		_, _ = di.GetConversationContext(ctx, "conv-nil", 4000)
	})
}

func TestDebateIntegration_PublishDebateCompletion_EmptyParticipants(t *testing.T) {
	broker := newMockBroker()
	di := newTestDebateIntegration(broker)
	ctx := context.Background()

	completion := &DebateCompletion{
		DebateID:       "debate-empty",
		ConversationID: "conv-empty",
		UserID:         "user-1",
		Participants:   []DebateParticipant{},
		Outcome:        "abandoned",
	}

	err := di.PublishDebateCompletion(ctx, completion)
	require.NoError(t, err)

	msgs := broker.getPublished()
	require.Len(t, msgs, 1)

	var decoded DebateCompletion
	err = json.Unmarshal(msgs[0].message.Payload, &decoded)
	require.NoError(t, err)
	assert.Empty(t, decoded.Participants)
	assert.Equal(t, "abandoned", decoded.Outcome)
}

func TestDebateIntegration_PublishDebateCompletion_WithMetadata(t *testing.T) {
	broker := newMockBroker()
	di := newTestDebateIntegration(broker)
	ctx := context.Background()

	completion := &DebateCompletion{
		DebateID:       "debate-meta",
		ConversationID: "conv-meta",
		Metadata: map[string]interface{}{
			"source":  "test",
			"version": 2.0,
		},
	}

	err := di.PublishDebateCompletion(ctx, completion)
	require.NoError(t, err)

	msgs := broker.getPublished()
	require.Len(t, msgs, 1)

	var decoded DebateCompletion
	err = json.Unmarshal(msgs[0].message.Payload, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "test", decoded.Metadata["source"])
}

func TestDebateIntegration_ConvertMessages_Empty(t *testing.T) {
	di := newTestDebateIntegration(newMockBroker())
	result := di.convertMessages(nil)
	assert.Len(t, result, 0)
}

func TestDebateIntegration_ConvertMessages_WithData(t *testing.T) {
	di := newTestDebateIntegration(newMockBroker())
	now := time.Now()

	messages := []conversation.MessageData{
		{
			MessageID: "msg-1",
			Role:      "user",
			Content:   "Hello world",
			Model:     "gpt-4",
			Tokens:    10,
			CreatedAt: now,
		},
		{
			MessageID: "msg-2",
			Role:      "assistant",
			Content:   "Hi there!",
			Model:     "claude-3",
			Tokens:    5,
			CreatedAt: now.Add(1 * time.Second),
		},
	}

	result := di.convertMessages(messages)
	require.Len(t, result, 2)

	assert.Equal(t, "user", result[0].Role)
	assert.Equal(t, "Hello world", result[0].Content)
	assert.Equal(t, now, result[0].Timestamp)
	assert.Equal(t, "msg-1", result[0].Metadata["message_id"])
	assert.Equal(t, "gpt-4", result[0].Metadata["model"])
	assert.Equal(t, 10, result[0].Metadata["tokens"])

	assert.Equal(t, "assistant", result[1].Role)
	assert.Equal(t, "Hi there!", result[1].Content)
}

func TestDebateIntegration_ConvertEntities_Empty(t *testing.T) {
	di := newTestDebateIntegration(newMockBroker())
	result := di.convertEntities(nil)
	assert.Len(t, result, 0)
}

func TestDebateIntegration_ConvertEntities_WithData(t *testing.T) {
	di := newTestDebateIntegration(newMockBroker())

	entities := []conversation.EntityData{
		{
			EntityID:   "ent-1",
			Name:       "Anthropic",
			Type:       "organization",
			Confidence: 0.95,
			Properties: map[string]interface{}{
				"industry": "AI",
			},
		},
		{
			EntityID:   "ent-2",
			Name:       "Claude",
			Type:       "product",
			Confidence: 0.88,
		},
	}

	result := di.convertEntities(entities)
	require.Len(t, result, 2)

	assert.Equal(t, "ent-1", result[0].ID)
	assert.Equal(t, "Anthropic", result[0].Name)
	assert.Equal(t, "organization", result[0].Type)
	assert.InDelta(t, 0.95, result[0].Importance, 0.001)
	assert.Equal(t, "AI", result[0].Properties["industry"])

	assert.Equal(t, "ent-2", result[1].ID)
	assert.Equal(t, "Claude", result[1].Name)
}

// --- Type assertion tests ---

func TestConversationEventType_Constants(t *testing.T) {
	assert.Equal(t, ConversationEventType("message.added"), ConversationEventMessageAdded)
	assert.Equal(t, ConversationEventType("entity.extracted"), ConversationEventEntityExtracted)
	assert.Equal(t, ConversationEventType("debate.started"), ConversationEventDebateStarted)
	assert.Equal(t, ConversationEventType("debate.completed"), ConversationEventDebateCompleted)
	assert.Equal(t, ConversationEventType("context.compressed"), ConversationEventContextCompressed)
}

func TestConversationContext_JSONSerialization(t *testing.T) {
	ctx := &ConversationContext{
		ConversationID: "conv-json",
		Messages: []Message{
			{Role: "user", Content: "hello", Timestamp: time.Now()},
		},
		Entities: []Entity{
			{ID: "e1", Name: "Test", Type: "concept", Importance: 0.8},
		},
		TotalTokens: 100,
		Compressed:  true,
		CompressionStats: &CompressionStats{
			Strategy:           "adaptive",
			OriginalMessages:   10,
			CompressedMessages: 5,
			OriginalTokens:     200,
			CompressedTokens:   100,
			CompressionRatio:   0.5,
			QualityScore:       0.9,
		},
	}

	data, err := json.Marshal(ctx)
	require.NoError(t, err)

	var decoded ConversationContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "conv-json", decoded.ConversationID)
	assert.Len(t, decoded.Messages, 1)
	assert.Len(t, decoded.Entities, 1)
	assert.True(t, decoded.Compressed)
	assert.NotNil(t, decoded.CompressionStats)
	assert.Equal(t, "adaptive", decoded.CompressionStats.Strategy)
}

func TestDebateCompletion_JSONSerialization(t *testing.T) {
	completion := &DebateCompletion{
		DebateID:       "d1",
		ConversationID: "c1",
		UserID:         "u1",
		Rounds:         5,
		Winner:         "deepseek/deepseek-v3",
		Confidence:     0.88,
		Participants: []DebateParticipant{
			{
				Provider: "deepseek", Model: "deepseek-v3",
				Position: "advocate", Won: true,
			},
			{
				Provider: "gemini", Model: "gemini-pro",
				Position: "critic", Won: false,
			},
		},
		Outcome: "successful",
	}

	data, err := json.Marshal(completion)
	require.NoError(t, err)

	var decoded DebateCompletion
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "d1", decoded.DebateID)
	assert.Len(t, decoded.Participants, 2)
	assert.True(t, decoded.Participants[0].Won)
	assert.False(t, decoded.Participants[1].Won)
}
