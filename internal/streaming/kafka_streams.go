package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/messaging/kafka"
	"go.uber.org/zap"
)

// ConversationStreamProcessor processes conversation events in real-time
type ConversationStreamProcessor struct {
	config      *StreamProcessorConfig
	logger      *zap.Logger
	kafkaBroker *kafka.Broker
	stateStore  StateStore
	running     bool
	mu          sync.RWMutex
	stopChan    chan struct{}
	stopOnce    sync.Once // Prevents double-close panic on stopChan
	wg          sync.WaitGroup

	// Stream processing workers
	aggregator      *ConversationAggregator
	entityExtractor *EntityExtractor
	analyticsSink   *AnalyticsSink

	// Metrics
	eventsProcessed  int64
	eventsBuffered   int64
	stateUpdates     int64
	processingTimeMs int64
}

// NewConversationStreamProcessor creates a new stream processor
func NewConversationStreamProcessor(
	config *StreamProcessorConfig,
	kafkaBroker *kafka.Broker,
	logger *zap.Logger,
) (*ConversationStreamProcessor, error) {
	if config == nil {
		config = DefaultStreamProcessorConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create state store
	var stateStore StateStore
	var err error
	switch config.StateStoreType {
	case "redis":
		stateStore, err = NewRedisStateStore(config.RedisHost, config.RedisPort, config.RedisDB, logger, config.RedisPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis state store: %w", err)
		}
	case "memory":
		stateStore = NewInMemoryStateStore()
	default:
		return nil, fmt.Errorf("unsupported state store type: %s", config.StateStoreType)
	}

	csp := &ConversationStreamProcessor{
		config:      config,
		logger:      logger,
		kafkaBroker: kafkaBroker,
		stateStore:  stateStore,
		stopChan:    make(chan struct{}),
	}

	// Create stream processing components
	csp.aggregator = NewConversationAggregator(stateStore, logger)
	csp.entityExtractor = NewEntityExtractor(logger)
	csp.analyticsSink = NewAnalyticsSink(logger)

	return csp, nil
}

// Start starts the stream processor
func (csp *ConversationStreamProcessor) Start(ctx context.Context) error {
	csp.mu.Lock()
	if csp.running {
		csp.mu.Unlock()
		return fmt.Errorf("stream processor already running")
	}
	csp.running = true
	csp.mu.Unlock()

	csp.logger.Info("Starting Kafka stream processor",
		zap.String("input_topic", csp.config.InputTopic),
		zap.Strings("output_topics", csp.config.OutputTopics))

	// Subscribe to input topic
	sub, err := csp.kafkaBroker.Subscribe(ctx, csp.config.InputTopic, csp.handleMessage)
	if err != nil {
		csp.mu.Lock()
		csp.running = false
		csp.mu.Unlock()
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	csp.logger.Info("Stream processor started successfully")

	// Wait for stop signal — unsubscribe on context cancellation.
	// Note: this goroutine does NOT call Stop() to avoid WaitGroup deadlock
	// (Stop calls wg.Wait which would wait for this goroutine).
	// Instead it just unsubscribes and signals via stopOnce.
	go func() {
		<-ctx.Done()
		if err := sub.Unsubscribe(); err != nil {
			csp.logger.Error("failed to unsubscribe", zap.Error(err))
		}
		csp.stopOnce.Do(func() {
			close(csp.stopChan)
		})
	}()

	return nil
}

// handleMessage processes incoming conversation events
func (csp *ConversationStreamProcessor) handleMessage(ctx context.Context, msg *messaging.Message) error {
	startTime := time.Now()

	// Parse message
	var event ConversationEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		csp.logger.Error("Failed to parse conversation event", zap.Error(err))
		return nil // Don't reprocess invalid messages
	}

	csp.logger.Debug("Processing conversation event",
		zap.String("event_id", event.EventID),
		zap.String("event_type", string(event.EventType)),
		zap.String("conversation_id", event.ConversationID))

	// Process event based on type
	switch event.EventType {
	case ConversationEventMessageAdded:
		if err := csp.handleMessageAdded(ctx, &event); err != nil {
			csp.logger.Error("Failed to handle message added event", zap.Error(err))
			return err
		}

	case ConversationEventEntityExtracted:
		if err := csp.handleEntityExtracted(ctx, &event); err != nil {
			csp.logger.Error("Failed to handle entity extracted event", zap.Error(err))
			return err
		}

	case ConversationEventDebateRound:
		if err := csp.handleDebateRound(ctx, &event); err != nil {
			csp.logger.Error("Failed to handle debate round event", zap.Error(err))
			return err
		}

	case ConversationEventCompleted:
		if err := csp.handleConversationCompleted(ctx, &event); err != nil {
			csp.logger.Error("Failed to handle conversation completed event", zap.Error(err))
			return err
		}
	}

	// Update metrics
	csp.eventsProcessed++
	csp.processingTimeMs += time.Since(startTime).Milliseconds()

	return nil
}

// handleMessageAdded processes message added events
func (csp *ConversationStreamProcessor) handleMessageAdded(ctx context.Context, event *ConversationEvent) error {
	if event.Message == nil {
		return fmt.Errorf("message data is nil")
	}

	// Update conversation state
	state, err := csp.aggregator.AddMessage(ctx, event.ConversationID, event.Message)
	if err != nil {
		return fmt.Errorf("failed to aggregate message: %w", err)
	}

	// Extract entities from message content
	entities := csp.entityExtractor.Extract(event.Message.Content)
	if len(entities) > 0 {
		// Publish entity extraction result
		result := &EntityExtractionResult{
			ConversationID: event.ConversationID,
			MessageID:      event.Message.MessageID,
			Entities:       entities,
			ExtractedAt:    time.Now(),
		}

		if err := csp.publishEntityExtraction(ctx, result); err != nil {
			csp.logger.Error("Failed to publish entity extraction", zap.Error(err))
		}

		// Update state with entities
		for _, entity := range entities {
			state.Entities[entity.EntityID] = entity
			state.EntityCount++
		}
	}

	// Save updated state
	return csp.stateStore.SaveState(ctx, event.ConversationID, state)
}

// handleEntityExtracted processes entity extracted events
func (csp *ConversationStreamProcessor) handleEntityExtracted(ctx context.Context, event *ConversationEvent) error {
	if len(event.Entities) == 0 {
		return nil
	}

	// Get current state
	state, err := csp.stateStore.GetState(ctx, event.ConversationID)
	if err != nil {
		// Initialize new state if doesn't exist
		state = &ConversationState{
			ConversationID: event.ConversationID,
			UserID:         event.UserID,
			SessionID:      event.SessionID,
			Entities:       make(map[string]EntityData),
			ProviderUsage:  make(map[string]int),
			StartedAt:      time.Now(),
			LastUpdatedAt:  time.Now(),
		}
	}

	// Add entities to state
	for _, entity := range event.Entities {
		state.Entities[entity.EntityID] = entity
	}
	state.EntityCount = len(state.Entities)
	state.LastUpdatedAt = time.Now()

	// Save state
	return csp.stateStore.SaveState(ctx, event.ConversationID, state)
}

// handleDebateRound processes debate round events
func (csp *ConversationStreamProcessor) handleDebateRound(ctx context.Context, event *ConversationEvent) error {
	if event.DebateRound == nil {
		return fmt.Errorf("debate round data is nil")
	}

	// Get current state
	state, err := csp.stateStore.GetState(ctx, event.ConversationID)
	if err != nil {
		state = &ConversationState{
			ConversationID: event.ConversationID,
			UserID:         event.UserID,
			SessionID:      event.SessionID,
			Entities:       make(map[string]EntityData),
			ProviderUsage:  make(map[string]int),
			StartedAt:      time.Now(),
			LastUpdatedAt:  time.Now(),
		}
	}

	// Update state with debate round info
	state.DebateRoundCount++
	state.TotalResponseTimeMs += event.DebateRound.ResponseTimeMs
	state.ProviderUsage[event.DebateRound.Provider]++
	state.TotalTokens += int64(event.DebateRound.TokensUsed)
	state.LastUpdatedAt = time.Now()

	// Save state
	if err := csp.stateStore.SaveState(ctx, event.ConversationID, state); err != nil {
		return err
	}

	// Publish debate analytics
	analytics := &WindowedAnalytics{
		WindowStart:          state.StartedAt,
		WindowEnd:            time.Now(),
		ConversationID:       event.ConversationID,
		TotalMessages:        state.MessageCount,
		DebateRounds:         state.DebateRoundCount,
		AvgResponseTimeMs:    float64(event.DebateRound.ResponseTimeMs),
		ProviderDistribution: state.ProviderUsage,
		CreatedAt:            time.Now(),
	}

	return csp.publishAnalytics(ctx, analytics)
}

// handleConversationCompleted processes conversation completed events
func (csp *ConversationStreamProcessor) handleConversationCompleted(ctx context.Context, event *ConversationEvent) error {
	// Get final state
	state, err := csp.stateStore.GetState(ctx, event.ConversationID)
	if err != nil {
		return fmt.Errorf("failed to get final state: %w", err)
	}

	// Calculate knowledge density
	var knowledgeDensity float64
	if state.MessageCount > 0 {
		knowledgeDensity = float64(state.EntityCount) / float64(state.MessageCount)
	}

	// Create final analytics
	analytics := &WindowedAnalytics{
		WindowStart:          state.StartedAt,
		WindowEnd:            time.Now(),
		ConversationID:       event.ConversationID,
		TotalMessages:        state.MessageCount,
		DebateRounds:         state.DebateRoundCount,
		KnowledgeDensity:     knowledgeDensity,
		ProviderDistribution: state.ProviderUsage,
		CreatedAt:            time.Now(),
	}

	// Publish final analytics
	if err := csp.publishAnalytics(ctx, analytics); err != nil {
		return err
	}

	csp.logger.Info("Conversation completed",
		zap.String("conversation_id", event.ConversationID),
		zap.Int("total_messages", state.MessageCount),
		zap.Int("entities_extracted", state.EntityCount),
		zap.Int("debate_rounds", state.DebateRoundCount))

	return nil
}

// publishEntityExtraction publishes entity extraction results to Kafka
func (csp *ConversationStreamProcessor) publishEntityExtraction(ctx context.Context, result *EntityExtractionResult) error {
	// Skip publishing if no broker configured (e.g., in tests)
	if csp.kafkaBroker == nil {
		return nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal entity extraction result: %w", err)
	}

	msg := messaging.NewMessage("entity.extracted", data)
	msg.SetHeader("conversation_id", result.ConversationID)

	return csp.kafkaBroker.Publish(ctx, TopicEntitiesUpdates, msg)
}

// publishAnalytics publishes analytics to Kafka
func (csp *ConversationStreamProcessor) publishAnalytics(ctx context.Context, analytics *WindowedAnalytics) error {
	// Skip publishing if no broker configured (e.g., in tests)
	if csp.kafkaBroker == nil {
		return nil
	}

	data, err := json.Marshal(analytics)
	if err != nil {
		return fmt.Errorf("failed to marshal analytics: %w", err)
	}

	msg := messaging.NewMessage("analytics.windowed", data)
	msg.SetHeader("conversation_id", analytics.ConversationID)

	return csp.kafkaBroker.Publish(ctx, TopicAnalyticsDebates, msg)
}

// Stop stops the stream processor. It is safe to call multiple times.
func (csp *ConversationStreamProcessor) Stop() error {
	csp.mu.Lock()
	defer csp.mu.Unlock()

	if !csp.running {
		return nil
	}

	csp.logger.Info("Stopping stream processor...")

	// Use sync.Once to prevent double-close panic on stopChan.
	// This can happen when both the context cancellation goroutine
	// (in Start) and an explicit Stop() call race.
	csp.stopOnce.Do(func() {
		close(csp.stopChan)
	})
	csp.wg.Wait()

	// Close state store
	if err := csp.stateStore.Close(); err != nil {
		csp.logger.Error("Failed to close state store", zap.Error(err))
	}

	csp.running = false
	csp.logger.Info("Stream processor stopped")

	return nil
}

// GetMetrics returns processing metrics
func (csp *ConversationStreamProcessor) GetMetrics() map[string]interface{} {
	csp.mu.RLock()
	defer csp.mu.RUnlock()

	avgProcessingTime := float64(0)
	if csp.eventsProcessed > 0 {
		avgProcessingTime = float64(csp.processingTimeMs) / float64(csp.eventsProcessed)
	}

	return map[string]interface{}{
		"events_processed":       csp.eventsProcessed,
		"events_buffered":        csp.eventsBuffered,
		"state_updates":          csp.stateUpdates,
		"avg_processing_time_ms": avgProcessingTime,
		"running":                csp.running,
	}
}

// BuildTopology builds the stream processing topology
func (csp *ConversationStreamProcessor) BuildTopology() *StreamTopology {
	// This is a simplified topology representation
	// In a full Kafka Streams implementation, this would be more complex

	return &StreamTopology{
		ConversationStates: make(map[string]*ConversationState),
		WindowedStates:     make(map[string]*WindowedAnalytics),
	}
}
