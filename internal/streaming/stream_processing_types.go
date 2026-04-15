package streaming

import (
	"context"
	"sync"
	"time"
)

// Ensure sync/context are available for channel lifecycle management.
var (
	_ context.Context
	_ sync.Locker
)

// ConversationEvent represents a conversation event from Kafka
type ConversationEvent struct {
	EventID        string                 `json:"event_id"`
	ConversationID string                 `json:"conversation_id"`
	UserID         string                 `json:"user_id"`
	SessionID      string                 `json:"session_id"`
	EventType      ConversationEventType  `json:"event_type"`
	Message        *MessageData           `json:"message,omitempty"`
	Entities       []EntityData           `json:"entities,omitempty"`
	DebateRound    *DebateRoundData       `json:"debate_round,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ConversationEventType represents the type of conversation event
type ConversationEventType string

const (
	ConversationEventMessageAdded    ConversationEventType = "message.added"
	ConversationEventEntityExtracted ConversationEventType = "entity.extracted"
	ConversationEventDebateRound     ConversationEventType = "debate.round"
	ConversationEventCompleted       ConversationEventType = "conversation.completed"
)

// MessageData represents message data in the event
type MessageData struct {
	MessageID string    `json:"message_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Tokens    int       `json:"tokens"`
	Timestamp time.Time `json:"timestamp"`
}

// EntityData represents extracted entity data
type EntityData struct {
	EntityID   string                 `json:"entity_id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Importance float64                `json:"importance"`
}

// DebateRoundData represents debate round data
type DebateRoundData struct {
	RoundID         string  `json:"round_id"`
	Round           int     `json:"round"`
	Position        int     `json:"position"`
	Role            string  `json:"role"`
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	ResponseTimeMs  int64   `json:"response_time_ms"`
	TokensUsed      int     `json:"tokens_used"`
	ConfidenceScore float64 `json:"confidence_score"`
}

// ConversationState represents aggregated conversation state
type ConversationState struct {
	ConversationID      string                 `json:"conversation_id"`
	UserID              string                 `json:"user_id"`
	SessionID           string                 `json:"session_id"`
	MessageCount        int                    `json:"message_count"`
	EntityCount         int                    `json:"entity_count"`
	TotalTokens         int64                  `json:"total_tokens"`
	DebateRoundCount    int                    `json:"debate_round_count"`
	TotalResponseTimeMs int64                  `json:"total_response_time_ms"`
	StartedAt           time.Time              `json:"started_at"`
	LastUpdatedAt       time.Time              `json:"last_updated_at"`
	Entities            map[string]EntityData  `json:"entities"`
	ProviderUsage       map[string]int         `json:"provider_usage"`
	CompressedContext   map[string]interface{} `json:"compressed_context,omitempty"`
	Version             int64                  `json:"version"` // Optimistic locking
}

// WindowedAnalytics represents analytics for a time window
type WindowedAnalytics struct {
	WindowStart          time.Time      `json:"window_start"`
	WindowEnd            time.Time      `json:"window_end"`
	ConversationID       string         `json:"conversation_id,omitempty"`
	TotalMessages        int            `json:"total_messages"`
	LLMCalls             int            `json:"llm_calls"`
	DebateRounds         int            `json:"debate_rounds"`
	AvgResponseTimeMs    float64        `json:"avg_response_time_ms"`
	EntityGrowth         int            `json:"entity_growth"`
	KnowledgeDensity     float64        `json:"knowledge_density"` // Entities per message
	ProviderDistribution map[string]int `json:"provider_distribution"`
	CreatedAt            time.Time      `json:"created_at"`
}

// EntityExtractionResult represents entity extraction result
type EntityExtractionResult struct {
	ConversationID string       `json:"conversation_id"`
	MessageID      string       `json:"message_id"`
	Entities       []EntityData `json:"entities"`
	ExtractedAt    time.Time    `json:"extracted_at"`
}

// StreamProcessorConfig holds configuration for stream processing
type StreamProcessorConfig struct {
	// Kafka configuration
	KafkaBrokers    []string `json:"kafka_brokers"`
	ConsumerGroupID string   `json:"consumer_group_id"`
	InputTopic      string   `json:"input_topic"`
	OutputTopics    []string `json:"output_topics"`

	// Processing configuration
	WindowDuration time.Duration `json:"window_duration"`
	GracePeriod    time.Duration `json:"grace_period"`
	FlushInterval  time.Duration `json:"flush_interval"`

	// State store configuration
	StateStoreType string `json:"state_store_type"` // "memory" or "redis"
	RedisHost      string `json:"redis_host,omitempty"`
	RedisPort      string `json:"redis_port,omitempty"`
	RedisDB        int    `json:"redis_db,omitempty"`
	RedisPassword  string `json:"redis_password,omitempty"`

	// Performance tuning
	MaxConcurrentMessages int           `json:"max_concurrent_messages"`
	BatchSize             int           `json:"batch_size"`
	CommitInterval        time.Duration `json:"commit_interval"`
}

// DefaultStreamProcessorConfig returns default configuration
func DefaultStreamProcessorConfig() *StreamProcessorConfig {
	return &StreamProcessorConfig{
		KafkaBrokers:    []string{"localhost:9092"},
		ConsumerGroupID: "helixagent-stream-processor",
		InputTopic:      "helixagent.conversations",
		OutputTopics: []string{
			"helixagent.memory.updates",
			"helixagent.analytics.debates",
		},
		WindowDuration:        5 * time.Minute,
		GracePeriod:           1 * time.Minute,
		FlushInterval:         10 * time.Second,
		StateStoreType:        "memory",
		MaxConcurrentMessages: 100,
		BatchSize:             100,
		CommitInterval:        5 * time.Second,
	}
}

// StreamTopology represents the stream processing topology
type StreamTopology struct {
	// Input streams
	ConversationStream <-chan *ConversationEvent

	// Intermediate streams
	EntityStream    chan<- *EntityExtractionResult
	DebateStream    chan<- *DebateRoundData
	AnalyticsStream chan<- *WindowedAnalytics

	// State stores
	ConversationStates map[string]*ConversationState
	WindowedStates     map[string]*WindowedAnalytics
}

// AggregationResult represents the result of aggregation
type AggregationResult struct {
	Key   string
	State *ConversationState
	Error error
}

// TopicConfig holds Kafka topic configuration
const (
	// Input topics
	TopicConversations = "helixagent.conversations"

	// Output topics
	TopicMemoryUpdates    = "helixagent.memory.updates"
	TopicAnalyticsDebates = "helixagent.analytics.debates"
	TopicEntitiesUpdates  = "helixagent.entities.updates"
	TopicProviderMetrics  = "helixagent.provider.metrics"
)
