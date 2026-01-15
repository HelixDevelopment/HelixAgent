// Package replay provides message replay functionality for the messaging system.
package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"dev.helix.agent/internal/messaging"
	"go.uber.org/zap"
)

// ReplayConfig configures the replay handler
type ReplayConfig struct {
	// MaxConcurrentReplays limits concurrent replay operations
	MaxConcurrentReplays int
	// DefaultBatchSize is the default number of messages per batch
	DefaultBatchSize int
	// MaxBatchSize is the maximum allowed batch size
	MaxBatchSize int
	// ReplayTimeout is the timeout for a single replay operation
	ReplayTimeout time.Duration
	// BufferSize is the size of the message buffer
	BufferSize int
}

// DefaultReplayConfig returns sensible defaults
func DefaultReplayConfig() ReplayConfig {
	return ReplayConfig{
		MaxConcurrentReplays: 5,
		DefaultBatchSize:     100,
		MaxBatchSize:         1000,
		ReplayTimeout:        5 * time.Minute,
		BufferSize:           10000,
	}
}

// ReplayRequest represents a request to replay messages
type ReplayRequest struct {
	ID          string            `json:"id"`
	Topic       string            `json:"topic"`
	FromTime    time.Time         `json:"from_time"`
	ToTime      time.Time         `json:"to_time,omitempty"`
	TargetTopic string            `json:"target_topic,omitempty"`
	Filter      *ReplayFilter     `json:"filter,omitempty"`
	Options     *ReplayOptions    `json:"options,omitempty"`
}

// ReplayFilter allows filtering which messages to replay
type ReplayFilter struct {
	MessageTypes []string               `json:"message_types,omitempty"`
	Headers      map[string]string      `json:"headers,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ReplayOptions configures how replay should be performed
type ReplayOptions struct {
	BatchSize      int           `json:"batch_size,omitempty"`
	DelayBetween   time.Duration `json:"delay_between,omitempty"`
	DryRun         bool          `json:"dry_run,omitempty"`
	PreserveOrder  bool          `json:"preserve_order,omitempty"`
	SkipDuplicates bool          `json:"skip_duplicates,omitempty"`
}

// ReplayStatus represents the status of a replay operation
type ReplayStatus string

const (
	ReplayStatusPending   ReplayStatus = "pending"
	ReplayStatusRunning   ReplayStatus = "running"
	ReplayStatusCompleted ReplayStatus = "completed"
	ReplayStatusFailed    ReplayStatus = "failed"
	ReplayStatusCancelled ReplayStatus = "cancelled"
)

// ReplayProgress tracks the progress of a replay operation
type ReplayProgress struct {
	RequestID       string        `json:"request_id"`
	Status          ReplayStatus  `json:"status"`
	TotalMessages   int64         `json:"total_messages"`
	ReplayedCount   int64         `json:"replayed_count"`
	SkippedCount    int64         `json:"skipped_count"`
	FailedCount     int64         `json:"failed_count"`
	StartTime       time.Time     `json:"start_time,omitempty"`
	EndTime         time.Time     `json:"end_time,omitempty"`
	CurrentOffset   int64         `json:"current_offset"`
	ErrorMessage    string        `json:"error_message,omitempty"`
	LastProcessedID string        `json:"last_processed_id,omitempty"`
	Rate            float64       `json:"rate_per_second"`
}

// Handler handles message replay operations
type Handler struct {
	config  ReplayConfig
	broker  messaging.MessageBroker
	logger  *zap.Logger

	activeReplays map[string]*ReplayProgress
	mu            sync.RWMutex

	semaphore chan struct{}
}

// NewHandler creates a new replay handler
func NewHandler(broker messaging.MessageBroker, config ReplayConfig, logger *zap.Logger) *Handler {
	return &Handler{
		config:        config,
		broker:        broker,
		logger:        logger,
		activeReplays: make(map[string]*ReplayProgress),
		semaphore:     make(chan struct{}, config.MaxConcurrentReplays),
	}
}

// StartReplay initiates a new replay operation
func (h *Handler) StartReplay(ctx context.Context, req *ReplayRequest) (*ReplayProgress, error) {
	// Validate request
	if err := h.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid replay request: %w", err)
	}

	// Check if replay with this ID already exists
	h.mu.Lock()
	if _, exists := h.activeReplays[req.ID]; exists {
		h.mu.Unlock()
		return nil, fmt.Errorf("replay with ID %s already exists", req.ID)
	}

	// Create progress tracker
	progress := &ReplayProgress{
		RequestID: req.ID,
		Status:    ReplayStatusPending,
		StartTime: time.Now(),
	}
	h.activeReplays[req.ID] = progress
	h.mu.Unlock()

	// Try to acquire semaphore
	select {
	case h.semaphore <- struct{}{}:
	case <-ctx.Done():
		h.updateStatus(req.ID, ReplayStatusCancelled, "context cancelled before start")
		return progress, ctx.Err()
	}

	// Start replay in background
	go h.executeReplay(ctx, req, progress)

	h.logger.Info("Replay started",
		zap.String("request_id", req.ID),
		zap.String("topic", req.Topic),
		zap.Time("from_time", req.FromTime))

	return progress, nil
}

// executeReplay performs the actual replay operation
func (h *Handler) executeReplay(ctx context.Context, req *ReplayRequest, progress *ReplayProgress) {
	defer func() {
		<-h.semaphore // Release semaphore when done
	}()

	// Create timeout context
	replayCtx, cancel := context.WithTimeout(ctx, h.config.ReplayTimeout)
	defer cancel()

	progress.Status = ReplayStatusRunning

	// Apply options with defaults
	options := req.Options
	if options == nil {
		options = &ReplayOptions{}
	}
	if options.BatchSize == 0 || options.BatchSize > h.config.MaxBatchSize {
		options.BatchSize = h.config.DefaultBatchSize
	}

	// Get target topic
	targetTopic := req.TargetTopic
	if targetTopic == "" {
		targetTopic = req.Topic
	}

	// Track seen message IDs for deduplication
	seenIDs := make(map[string]bool)
	startTime := time.Now()

	// In a real implementation, this would read from Kafka with time-based offset seeking
	// For now, we simulate the replay operation
	h.logger.Info("Executing replay",
		zap.String("request_id", req.ID),
		zap.String("source_topic", req.Topic),
		zap.String("target_topic", targetTopic),
		zap.Bool("dry_run", options.DryRun))

	// Simulate processing messages
	var batch []*messaging.Message
	processed := int64(0)

	for {
		select {
		case <-replayCtx.Done():
			h.updateStatus(req.ID, ReplayStatusCancelled, "replay cancelled")
			return
		default:
		}

		// Simulate fetching a batch of messages
		// In production, this would read from Kafka using the FromTime/ToTime range
		batch = h.fetchMessageBatch(replayCtx, req, options.BatchSize)
		if len(batch) == 0 {
			break // No more messages
		}

		for _, msg := range batch {
			// Apply filters
			if !h.matchesFilter(msg, req.Filter) {
				atomic.AddInt64(&progress.SkippedCount, 1)
				continue
			}

			// Check for duplicates
			if options.SkipDuplicates {
				if seenIDs[msg.ID] {
					atomic.AddInt64(&progress.SkippedCount, 1)
					continue
				}
				seenIDs[msg.ID] = true
			}

			// Replay the message
			if !options.DryRun {
				if err := h.broker.Publish(replayCtx, targetTopic, msg); err != nil {
					atomic.AddInt64(&progress.FailedCount, 1)
					h.logger.Warn("Failed to replay message",
						zap.String("message_id", msg.ID),
						zap.Error(err))
					continue
				}
			}

			atomic.AddInt64(&progress.ReplayedCount, 1)
			processed++
			progress.LastProcessedID = msg.ID

			// Add delay between messages if configured
			if options.DelayBetween > 0 {
				time.Sleep(options.DelayBetween)
			}
		}

		atomic.AddInt64(&progress.CurrentOffset, int64(len(batch)))

		// Update rate
		elapsed := time.Since(startTime).Seconds()
		if elapsed > 0 {
			progress.Rate = float64(processed) / elapsed
		}
	}

	// Mark as completed
	progress.EndTime = time.Now()
	progress.Status = ReplayStatusCompleted

	h.logger.Info("Replay completed",
		zap.String("request_id", req.ID),
		zap.Int64("replayed", atomic.LoadInt64(&progress.ReplayedCount)),
		zap.Int64("skipped", atomic.LoadInt64(&progress.SkippedCount)),
		zap.Int64("failed", atomic.LoadInt64(&progress.FailedCount)),
		zap.Duration("duration", progress.EndTime.Sub(progress.StartTime)))
}

// validateRequest validates a replay request
func (h *Handler) validateRequest(req *ReplayRequest) error {
	if req.ID == "" {
		return fmt.Errorf("request ID is required")
	}
	if req.Topic == "" {
		return fmt.Errorf("topic is required")
	}
	if req.FromTime.IsZero() {
		return fmt.Errorf("from_time is required")
	}
	if !req.ToTime.IsZero() && req.ToTime.Before(req.FromTime) {
		return fmt.Errorf("to_time must be after from_time")
	}
	return nil
}

// fetchMessageBatch fetches a batch of messages for replay
// In production, this would read from Kafka
func (h *Handler) fetchMessageBatch(ctx context.Context, req *ReplayRequest, batchSize int) []*messaging.Message {
	// Simulate fetching messages - in production this would read from Kafka
	// using offset seeking based on timestamp
	return nil
}

// matchesFilter checks if a message matches the replay filter
func (h *Handler) matchesFilter(msg *messaging.Message, filter *ReplayFilter) bool {
	if filter == nil {
		return true
	}

	// Check message types
	if len(filter.MessageTypes) > 0 {
		found := false
		for _, t := range filter.MessageTypes {
			if msg.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check headers
	for key, value := range filter.Headers {
		if msg.Headers[key] != value {
			return false
		}
	}

	return true
}

// updateStatus updates the status of a replay operation
func (h *Handler) updateStatus(requestID string, status ReplayStatus, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if progress, ok := h.activeReplays[requestID]; ok {
		progress.Status = status
		progress.ErrorMessage = message
		if status == ReplayStatusCompleted || status == ReplayStatusFailed || status == ReplayStatusCancelled {
			progress.EndTime = time.Now()
		}
	}
}

// GetProgress returns the progress of a replay operation
func (h *Handler) GetProgress(requestID string) (*ReplayProgress, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	progress, ok := h.activeReplays[requestID]
	if !ok {
		return nil, fmt.Errorf("replay %s not found", requestID)
	}

	// Return a copy to avoid race conditions
	progressCopy := *progress
	return &progressCopy, nil
}

// CancelReplay cancels an ongoing replay operation
func (h *Handler) CancelReplay(requestID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	progress, ok := h.activeReplays[requestID]
	if !ok {
		return fmt.Errorf("replay %s not found", requestID)
	}

	if progress.Status != ReplayStatusRunning && progress.Status != ReplayStatusPending {
		return fmt.Errorf("replay %s is not running (status: %s)", requestID, progress.Status)
	}

	progress.Status = ReplayStatusCancelled
	progress.ErrorMessage = "cancelled by user"
	progress.EndTime = time.Now()

	h.logger.Info("Replay cancelled", zap.String("request_id", requestID))
	return nil
}

// ListReplays returns all active and recent replay operations
func (h *Handler) ListReplays() []*ReplayProgress {
	h.mu.RLock()
	defer h.mu.RUnlock()

	replays := make([]*ReplayProgress, 0, len(h.activeReplays))
	for _, progress := range h.activeReplays {
		progressCopy := *progress
		replays = append(replays, &progressCopy)
	}
	return replays
}

// CleanupOldReplays removes completed/failed replays older than maxAge
func (h *Handler) CleanupOldReplays(maxAge time.Duration) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, progress := range h.activeReplays {
		if progress.Status == ReplayStatusCompleted ||
			progress.Status == ReplayStatusFailed ||
			progress.Status == ReplayStatusCancelled {
			if !progress.EndTime.IsZero() && progress.EndTime.Before(cutoff) {
				delete(h.activeReplays, id)
				removed++
			}
		}
	}

	return removed
}

// ReplayHTTPHandler provides HTTP endpoints for replay operations
type ReplayHTTPHandler struct {
	handler *Handler
	logger  *zap.Logger
}

// NewReplayHTTPHandler creates a new HTTP handler for replay operations
func NewReplayHTTPHandler(handler *Handler, logger *zap.Logger) *ReplayHTTPHandler {
	return &ReplayHTTPHandler{
		handler: handler,
		logger:  logger,
	}
}

// StartReplayRequest is the request body for starting a replay
type StartReplayRequest struct {
	Topic       string        `json:"topic" binding:"required"`
	FromTime    string        `json:"from_time" binding:"required"`
	ToTime      string        `json:"to_time,omitempty"`
	TargetTopic string        `json:"target_topic,omitempty"`
	Filter      *ReplayFilter `json:"filter,omitempty"`
	Options     *ReplayOptions `json:"options,omitempty"`
}

// ReplayResponse is the response for replay operations
type ReplayResponse struct {
	Success  bool            `json:"success"`
	Message  string          `json:"message,omitempty"`
	Data     interface{}     `json:"data,omitempty"`
	Progress *ReplayProgress `json:"progress,omitempty"`
}

// SerializeProgress converts ReplayProgress to JSON
func (p *ReplayProgress) JSON() ([]byte, error) {
	return json.Marshal(p)
}
