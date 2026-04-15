package streaming

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.agent/internal/testutil"
)

func TestConversationStreamProcessor_New(t *testing.T) {
	config := DefaultStreamProcessorConfig()
	logger := zap.NewNop()

	processor, err := NewConversationStreamProcessor(config, nil, logger)
	require.NoError(t, err)
	require.NotNil(t, processor)

	assert.Equal(t, config, processor.config)
	assert.NotNil(t, processor.stateStore)
	assert.NotNil(t, processor.aggregator)
	assert.NotNil(t, processor.entityExtractor)
	assert.NotNil(t, processor.analyticsSink)
}

func TestConversationStreamProcessor_NewWithRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Redis-dependent test in short mode")
	}
	testutil.RequireRedis(t)

	infraCfg := testutil.DefaultInfraConfig()
	config := DefaultStreamProcessorConfig()
	config.StateStoreType = "redis"
	config.RedisHost = infraCfg.RedisHost
	config.RedisPort = infraCfg.RedisPort
	config.RedisPassword = testutil.RedisPassword()

	processor, err := NewConversationStreamProcessor(config, nil, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, processor)

	defer func() { _ = processor.Stop() }()
}

func TestConversationStreamProcessor_HandleMessageAdded(t *testing.T) {
	config := DefaultStreamProcessorConfig()
	processor, err := NewConversationStreamProcessor(config, nil, zap.NewNop())
	require.NoError(t, err)

	ctx := context.Background()
	event := &ConversationEvent{
		EventID:        "evt-001",
		ConversationID: "conv-001",
		UserID:         "user-001",
		SessionID:      "session-001",
		EventType:      ConversationEventMessageAdded,
		Message: &MessageData{
			MessageID: "msg-001",
			Role:      "user",
			Content:   "Hello, I need help with Kafka Streams and Redis configuration.",
			Tokens:    15,
			Timestamp: time.Now(),
		},
		Timestamp: time.Now(),
	}

	err = processor.handleMessageAdded(ctx, event)
	assert.NoError(t, err)

	// Verify state was updated
	state, err := processor.stateStore.GetState(ctx, event.ConversationID)
	assert.NoError(t, err)
	assert.Equal(t, 1, state.MessageCount)
	assert.Equal(t, int64(15), state.TotalTokens)
	assert.True(t, state.EntityCount > 0, "Should extract entities from message")
}

func TestConversationStreamProcessor_HandleEntityExtracted(t *testing.T) {
	config := DefaultStreamProcessorConfig()
	processor, err := NewConversationStreamProcessor(config, nil, zap.NewNop())
	require.NoError(t, err)

	ctx := context.Background()
	event := &ConversationEvent{
		EventID:        "evt-002",
		ConversationID: "conv-002",
		UserID:         "user-001",
		SessionID:      "session-001",
		EventType:      ConversationEventEntityExtracted,
		Entities: []EntityData{
			{
				EntityID:   "entity-001",
				Name:       "PostgreSQL",
				Type:       "technology",
				Importance: 0.9,
			},
			{
				EntityID:   "entity-002",
				Name:       "Docker",
				Type:       "technology",
				Importance: 0.8,
			},
		},
		Timestamp: time.Now(),
	}

	err = processor.handleEntityExtracted(ctx, event)
	assert.NoError(t, err)

	// Verify entities were added to state
	state, err := processor.stateStore.GetState(ctx, event.ConversationID)
	assert.NoError(t, err)
	assert.Equal(t, 2, state.EntityCount)
	assert.Contains(t, state.Entities, "entity-001")
	assert.Contains(t, state.Entities, "entity-002")
}

func TestConversationStreamProcessor_HandleDebateRound(t *testing.T) {
	config := DefaultStreamProcessorConfig()
	processor, err := NewConversationStreamProcessor(config, nil, zap.NewNop())
	require.NoError(t, err)

	ctx := context.Background()
	event := &ConversationEvent{
		EventID:        "evt-003",
		ConversationID: "conv-003",
		UserID:         "user-001",
		SessionID:      "session-001",
		EventType:      ConversationEventDebateRound,
		DebateRound: &DebateRoundData{
			RoundID:         "round-001",
			Round:           1,
			Position:        1,
			Role:            "analyst",
			Provider:        "claude",
			Model:           "claude-3-opus",
			ResponseTimeMs:  450,
			TokensUsed:      200,
			ConfidenceScore: 0.92,
		},
		Timestamp: time.Now(),
	}

	err = processor.handleDebateRound(ctx, event)
	assert.NoError(t, err)

	// Verify state was updated
	state, err := processor.stateStore.GetState(ctx, event.ConversationID)
	assert.NoError(t, err)
	assert.Equal(t, 1, state.DebateRoundCount)
	assert.Equal(t, int64(200), state.TotalTokens)
	assert.Equal(t, 1, state.ProviderUsage["claude"])
}

func TestConversationStreamProcessor_HandleConversationCompleted(t *testing.T) {
	config := DefaultStreamProcessorConfig()
	processor, err := NewConversationStreamProcessor(config, nil, zap.NewNop())
	require.NoError(t, err)

	ctx := context.Background()
	conversationID := "conv-004"

	// Setup initial state
	state := &ConversationState{
		ConversationID:   conversationID,
		UserID:           "user-001",
		SessionID:        "session-001",
		MessageCount:     10,
		EntityCount:      5,
		TotalTokens:      500,
		DebateRoundCount: 3,
		Entities:         make(map[string]EntityData),
		ProviderUsage:    map[string]int{"claude": 2, "deepseek": 1},
		StartedAt:        time.Now().Add(-10 * time.Minute),
		LastUpdatedAt:    time.Now(),
	}

	err = processor.stateStore.SaveState(ctx, conversationID, state)
	require.NoError(t, err)

	event := &ConversationEvent{
		EventID:        "evt-004",
		ConversationID: conversationID,
		EventType:      ConversationEventCompleted,
		Timestamp:      time.Now(),
	}

	err = processor.handleConversationCompleted(ctx, event)
	assert.NoError(t, err)
}

func TestConversationStreamProcessor_GetMetrics(t *testing.T) {
	config := DefaultStreamProcessorConfig()
	processor, err := NewConversationStreamProcessor(config, nil, zap.NewNop())
	require.NoError(t, err)

	// Process some events to generate metrics
	processor.eventsProcessed = 100
	processor.eventsBuffered = 10
	processor.stateUpdates = 50
	processor.processingTimeMs = 5000

	metrics := processor.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, int64(100), metrics["events_processed"])
	assert.Equal(t, int64(10), metrics["events_buffered"])
	assert.Equal(t, int64(50), metrics["state_updates"])
	assert.Equal(t, float64(50), metrics["avg_processing_time_ms"]) // 5000 / 100
}

func TestConversationStreamProcessor_BuildTopology(t *testing.T) {
	config := DefaultStreamProcessorConfig()
	processor, err := NewConversationStreamProcessor(config, nil, zap.NewNop())
	require.NoError(t, err)

	topology := processor.BuildTopology()
	assert.NotNil(t, topology)
	assert.NotNil(t, topology.ConversationStates)
	assert.NotNil(t, topology.WindowedStates)
}

func TestStreamProcessorConfig_Defaults(t *testing.T) {
	config := DefaultStreamProcessorConfig()

	assert.Equal(t, []string{"localhost:9092"}, config.KafkaBrokers)
	assert.Equal(t, "helixagent-stream-processor", config.ConsumerGroupID)
	assert.Equal(t, "helixagent.conversations", config.InputTopic)
	assert.Equal(t, 5*time.Minute, config.WindowDuration)
	assert.Equal(t, 1*time.Minute, config.GracePeriod)
	assert.Equal(t, "memory", config.StateStoreType)
	assert.Equal(t, 100, config.MaxConcurrentMessages)
}

func TestConversationEvent_Serialization(t *testing.T) {
	event := &ConversationEvent{
		EventID:        "evt-001",
		ConversationID: "conv-001",
		UserID:         "user-001",
		SessionID:      "session-001",
		EventType:      ConversationEventMessageAdded,
		Message: &MessageData{
			MessageID: "msg-001",
			Role:      "user",
			Content:   "Test message",
			Tokens:    3,
			Timestamp: time.Now(),
		},
		Timestamp: time.Now(),
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Unmarshal back
	var decoded ConversationEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.EventID, decoded.EventID)
	assert.Equal(t, event.ConversationID, decoded.ConversationID)
	assert.Equal(t, event.EventType, decoded.EventType)
	assert.NotNil(t, decoded.Message)
	assert.Equal(t, event.Message.MessageID, decoded.Message.MessageID)
}

func TestConversationState_Serialization(t *testing.T) {
	state := &ConversationState{
		ConversationID: "conv-001",
		UserID:         "user-001",
		SessionID:      "session-001",
		MessageCount:   10,
		EntityCount:    5,
		TotalTokens:    500,
		Entities: map[string]EntityData{
			"entity-001": {
				EntityID:   "entity-001",
				Name:       "PostgreSQL",
				Type:       "technology",
				Importance: 0.9,
			},
		},
		ProviderUsage: map[string]int{
			"claude":   3,
			"deepseek": 2,
		},
		StartedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
		Version:       1,
	}

	// Marshal to JSON
	data, err := json.Marshal(state)
	require.NoError(t, err)

	// Unmarshal back
	var decoded ConversationState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, state.ConversationID, decoded.ConversationID)
	assert.Equal(t, state.MessageCount, decoded.MessageCount)
	assert.Equal(t, state.EntityCount, decoded.EntityCount)
	assert.Contains(t, decoded.Entities, "entity-001")
	assert.Equal(t, 3, decoded.ProviderUsage["claude"])
}

func BenchmarkConversationStreamProcessor_HandleMessageAdded(b *testing.B) {
	config := DefaultStreamProcessorConfig()
	processor, err := NewConversationStreamProcessor(config, nil, zap.NewNop())
	require.NoError(b, err)

	ctx := context.Background()
	event := &ConversationEvent{
		EventID:        "evt-bench",
		ConversationID: "conv-bench",
		UserID:         "user-bench",
		SessionID:      "session-bench",
		EventType:      ConversationEventMessageAdded,
		Message: &MessageData{
			MessageID: "msg-bench",
			Role:      "user",
			Content:   "Benchmark message with Kafka and PostgreSQL mentions.",
			Tokens:    10,
			Timestamp: time.Now(),
		},
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event.EventID = string(rune(i))
		_ = processor.handleMessageAdded(ctx, event)
	}
}
