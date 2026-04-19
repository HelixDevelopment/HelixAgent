package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// StateStore defines the interface for storing stream processing state
type StateStore interface {
	// GetState retrieves conversation state by ID
	GetState(ctx context.Context, conversationID string) (*ConversationState, error)

	// SaveState saves conversation state
	SaveState(ctx context.Context, conversationID string, state *ConversationState) error

	// DeleteState deletes conversation state
	DeleteState(ctx context.Context, conversationID string) error

	// ListStates lists all conversation states
	ListStates(ctx context.Context) ([]*ConversationState, error)

	// GetWindowedAnalytics retrieves windowed analytics by key
	GetWindowedAnalytics(ctx context.Context, key string) (*WindowedAnalytics, error)

	// SaveWindowedAnalytics saves windowed analytics
	SaveWindowedAnalytics(ctx context.Context, key string, analytics *WindowedAnalytics) error

	// Close closes the state store
	Close() error
}

// ============================================================================
// In-Memory State Store
// ============================================================================

// InMemoryStateStore implements StateStore using in-memory maps.
//
// Concurrent-safe by construction (CONST-029): two independent
// safe.Stores. Save/Get clone the value, so the pointer stored is
// owned by the Store and is never mutated through the Store.
type InMemoryStateStore struct {
	conversationStates *safe.Store[string, *ConversationState]
	windowedAnalytics  *safe.Store[string, *WindowedAnalytics]
}

// NewInMemoryStateStore creates a new in-memory state store
func NewInMemoryStateStore() *InMemoryStateStore {
	return &InMemoryStateStore{
		conversationStates: safe.NewStore[string, *ConversationState](),
		windowedAnalytics:  safe.NewStore[string, *WindowedAnalytics](),
	}
}

// GetState retrieves conversation state
func (s *InMemoryStateStore) GetState(ctx context.Context, conversationID string) (*ConversationState, error) {
	state, exists := s.conversationStates.Get(conversationID)
	if !exists {
		return nil, fmt.Errorf("conversation state not found: %s", conversationID)
	}
	// Return a deep copy to prevent concurrent modifications
	return s.cloneState(state), nil
}

// SaveState saves conversation state
func (s *InMemoryStateStore) SaveState(ctx context.Context, conversationID string, state *ConversationState) error {
	s.conversationStates.Put(conversationID, s.cloneState(state))
	return nil
}

// DeleteState deletes conversation state
func (s *InMemoryStateStore) DeleteState(ctx context.Context, conversationID string) error {
	s.conversationStates.Delete(conversationID)
	return nil
}

// ListStates lists all conversation states
func (s *InMemoryStateStore) ListStates(ctx context.Context) ([]*ConversationState, error) {
	snapshot := s.conversationStates.Values()
	states := make([]*ConversationState, 0, len(snapshot))
	for _, state := range snapshot {
		states = append(states, s.cloneState(state))
	}
	return states, nil
}

// GetWindowedAnalytics retrieves windowed analytics
func (s *InMemoryStateStore) GetWindowedAnalytics(ctx context.Context, key string) (*WindowedAnalytics, error) {
	analytics, exists := s.windowedAnalytics.Get(key)
	if !exists {
		return nil, fmt.Errorf("windowed analytics not found: %s", key)
	}
	return s.cloneAnalytics(analytics), nil
}

// SaveWindowedAnalytics saves windowed analytics
func (s *InMemoryStateStore) SaveWindowedAnalytics(ctx context.Context, key string, analytics *WindowedAnalytics) error {
	s.windowedAnalytics.Put(key, s.cloneAnalytics(analytics))
	return nil
}

// Close closes the state store (no-op for in-memory)
func (s *InMemoryStateStore) Close() error {
	return nil
}

// cloneState creates a deep copy of conversation state
func (s *InMemoryStateStore) cloneState(state *ConversationState) *ConversationState {
	if state == nil {
		return nil
	}

	clone := &ConversationState{
		ConversationID:      state.ConversationID,
		UserID:              state.UserID,
		SessionID:           state.SessionID,
		MessageCount:        state.MessageCount,
		EntityCount:         state.EntityCount,
		TotalTokens:         state.TotalTokens,
		DebateRoundCount:    state.DebateRoundCount,
		TotalResponseTimeMs: state.TotalResponseTimeMs,
		StartedAt:           state.StartedAt,
		LastUpdatedAt:       state.LastUpdatedAt,
		Version:             state.Version,
		Entities:            make(map[string]EntityData),
		ProviderUsage:       make(map[string]int),
	}

	for k, v := range state.Entities {
		clone.Entities[k] = v
	}

	for k, v := range state.ProviderUsage {
		clone.ProviderUsage[k] = v
	}

	if state.CompressedContext != nil {
		clone.CompressedContext = make(map[string]interface{})
		for k, v := range state.CompressedContext {
			clone.CompressedContext[k] = v
		}
	}

	return clone
}

// cloneAnalytics creates a deep copy of windowed analytics
func (s *InMemoryStateStore) cloneAnalytics(analytics *WindowedAnalytics) *WindowedAnalytics {
	if analytics == nil {
		return nil
	}

	clone := &WindowedAnalytics{
		WindowStart:          analytics.WindowStart,
		WindowEnd:            analytics.WindowEnd,
		ConversationID:       analytics.ConversationID,
		TotalMessages:        analytics.TotalMessages,
		LLMCalls:             analytics.LLMCalls,
		DebateRounds:         analytics.DebateRounds,
		AvgResponseTimeMs:    analytics.AvgResponseTimeMs,
		EntityGrowth:         analytics.EntityGrowth,
		KnowledgeDensity:     analytics.KnowledgeDensity,
		CreatedAt:            analytics.CreatedAt,
		ProviderDistribution: make(map[string]int),
	}

	for k, v := range analytics.ProviderDistribution {
		clone.ProviderDistribution[k] = v
	}

	return clone
}

// ============================================================================
// Redis State Store
// ============================================================================

// RedisStateStore implements StateStore using Redis
type RedisStateStore struct {
	client *redis.Client
	logger *zap.Logger
	ttl    time.Duration
}

// NewRedisStateStore creates a new Redis state store
func NewRedisStateStore(host, port string, db int, logger *zap.Logger, password ...string) (*RedisStateStore, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	var pwd string
	if len(password) > 0 {
		pwd = password[0]
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		DB:       db,
		Password: pwd,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Connected to Redis state store",
		zap.String("addr", fmt.Sprintf("%s:%s", host, port)),
		zap.Int("db", db))

	return &RedisStateStore{
		client: client,
		logger: logger,
		ttl:    24 * time.Hour, // Default 24 hour TTL for states
	}, nil
}

// GetState retrieves conversation state from Redis
func (r *RedisStateStore) GetState(ctx context.Context, conversationID string) (*ConversationState, error) {
	key := r.stateKey(conversationID)

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("conversation state not found: %s", conversationID)
		}
		return nil, fmt.Errorf("failed to get state from Redis: %w", err)
	}

	var state ConversationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

// SaveState saves conversation state to Redis
func (r *RedisStateStore) SaveState(ctx context.Context, conversationID string, state *ConversationState) error {
	key := r.stateKey(conversationID)

	// Update version for optimistic locking
	state.Version++
	state.LastUpdatedAt = time.Now()

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to save state to Redis: %w", err)
	}

	return nil
}

// DeleteState deletes conversation state from Redis
func (r *RedisStateStore) DeleteState(ctx context.Context, conversationID string) error {
	key := r.stateKey(conversationID)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete state from Redis: %w", err)
	}

	return nil
}

// ListStates lists all conversation states from Redis
func (r *RedisStateStore) ListStates(ctx context.Context) ([]*ConversationState, error) {
	pattern := "helixagent:stream:state:*"

	var cursor uint64
	var states []*ConversationState

	for {
		var keys []string
		var err error

		keys, cursor, err = r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan Redis keys: %w", err)
		}

		// Get all states for these keys
		for _, key := range keys {
			data, err := r.client.Get(ctx, key).Bytes()
			if err != nil {
				r.logger.Error("Failed to get state", zap.String("key", key), zap.Error(err))
				continue
			}

			var state ConversationState
			if err := json.Unmarshal(data, &state); err != nil {
				r.logger.Error("Failed to unmarshal state", zap.String("key", key), zap.Error(err))
				continue
			}

			states = append(states, &state)
		}

		if cursor == 0 {
			break
		}
	}

	return states, nil
}

// GetWindowedAnalytics retrieves windowed analytics from Redis
func (r *RedisStateStore) GetWindowedAnalytics(ctx context.Context, key string) (*WindowedAnalytics, error) {
	redisKey := r.analyticsKey(key)

	data, err := r.client.Get(ctx, redisKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("windowed analytics not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get analytics from Redis: %w", err)
	}

	var analytics WindowedAnalytics
	if err := json.Unmarshal(data, &analytics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal analytics: %w", err)
	}

	return &analytics, nil
}

// SaveWindowedAnalytics saves windowed analytics to Redis
func (r *RedisStateStore) SaveWindowedAnalytics(ctx context.Context, key string, analytics *WindowedAnalytics) error {
	redisKey := r.analyticsKey(key)

	data, err := json.Marshal(analytics)
	if err != nil {
		return fmt.Errorf("failed to marshal analytics: %w", err)
	}

	if err := r.client.Set(ctx, redisKey, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to save analytics to Redis: %w", err)
	}

	return nil
}

// Close closes the Redis connection
func (r *RedisStateStore) Close() error {
	return r.client.Close()
}

// stateKey generates Redis key for conversation state
func (r *RedisStateStore) stateKey(conversationID string) string {
	return fmt.Sprintf("helixagent:stream:state:%s", conversationID)
}

// analyticsKey generates Redis key for analytics
func (r *RedisStateStore) analyticsKey(key string) string {
	return fmt.Sprintf("helixagent:stream:analytics:%s", key)
}
