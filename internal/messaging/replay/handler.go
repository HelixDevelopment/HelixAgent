// Package replay provides message replay functionality for the messaging system.
package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

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
	ID          string         `json:"id"`
	Topic       string         `json:"topic"`
	FromTime    time.Time      `json:"from_time"`
	ToTime      time.Time      `json:"to_time,omitempty"`
	TargetTopic string         `json:"target_topic,omitempty"`
	Filter      *ReplayFilter  `json:"filter,omitempty"`
	Options     *ReplayOptions `json:"options,omitempty"`
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
	RequestID       string       `json:"request_id"`
	Status          ReplayStatus `json:"status"`
	TotalMessages   int64        `json:"total_messages"`
	ReplayedCount   int64        `json:"replayed_count"`
	SkippedCount    int64        `json:"skipped_count"`
	FailedCount     int64        `json:"failed_count"`
	StartTime       time.Time    `json:"start_time,omitempty"`
	EndTime         time.Time    `json:"end_time,omitempty"`
	CurrentOffset   int64        `json:"current_offset"`
	ErrorMessage    string       `json:"error_message,omitempty"`
	LastProcessedID string       `json:"last_processed_id,omitempty"`
	Rate            float64      `json:"rate_per_second"`
}

// Handler handles message replay operations.
//
// Concurrency model (CONST-029): activeReplays is a *safe.Store.
// Per-entry ReplayProgress mutations under Store.Update's lock
// preserve the pre-migration read-modify-write semantics. No survivor
// mutex.
type Handler struct {
	config ReplayConfig
	broker messaging.MessageBroker
	logger *zap.Logger

	activeReplays *safe.Store[string, *ReplayProgress]

	semaphore chan struct{}
}

// NewHandler creates a new replay handler
func NewHandler(broker messaging.MessageBroker, config ReplayConfig, logger *zap.Logger) *Handler {
	return &Handler{
		config:        config,
		broker:        broker,
		logger:        logger,
		activeReplays: safe.NewStore[string, *ReplayProgress](),
		semaphore:     make(chan struct{}, config.MaxConcurrentReplays),
	}
}

// StartReplay initiates a new replay operation
func (h *Handler) StartReplay(ctx context.Context, req *ReplayRequest) (*ReplayProgress, error) {
	// Validate request
	if err := h.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid replay request: %w", err)
	}

	// Check if replay with this ID already exists. Use PutIfAbsent for
	// atomic "reject if present else install" semantics.
	progress := &ReplayProgress{
		RequestID: req.ID,
		Status:    ReplayStatusPending,
		StartTime: time.Now(),
	}
	if _, stored := h.activeReplays.PutIfAbsent(req.ID, progress); !stored {
		return nil, fmt.Errorf("replay with ID %s already exists", req.ID)
	}

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

	// Update status with proper synchronization
	h.updateStatus(req.ID, ReplayStatusRunning, "")

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

	h.logger.Info("Executing replay",
		zap.String("request_id", req.ID),
		zap.String("source_topic", req.Topic),
		zap.String("target_topic", targetTopic),
		zap.Bool("dry_run", options.DryRun))

	var batch []*messaging.Message
	processed := int64(0)

	for {
		select {
		case <-replayCtx.Done():
			h.updateStatus(req.ID, ReplayStatusCancelled, "replay cancelled")
			return
		default:
		}

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
			lastID := msg.ID

			// Add delay between messages if configured
			if options.DelayBetween > 0 {
				time.Sleep(options.DelayBetween)
			}

			// Update progress fields with proper synchronization
			elapsed := time.Since(startTime).Seconds()
			rate := float64(0)
			if elapsed > 0 {
				rate = float64(processed) / elapsed
			}
			h.updateProgressFields(req.ID, lastID, rate)
		}

		atomic.AddInt64(&progress.CurrentOffset, int64(len(batch)))
	}

	// Mark as completed using updateStatus for proper synchronization
	h.updateStatus(req.ID, ReplayStatusCompleted, "")

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

// fetchMessageBatch fetches a batch of messages for replay by subscribing to the
// source topic and collecting messages that fall within the requested time range.
// It uses a short-lived subscription with a batch-sized buffer. Returns nil when
// no more messages are available within the time range or context is cancelled.
func (h *Handler) fetchMessageBatch(ctx context.Context, req *ReplayRequest, batchSize int) []*messaging.Message {
	if h.broker == nil {
		return nil
	}

	batch := make([]*messaging.Message, 0, batchSize)
	collected := make(chan *messaging.Message, batchSize)

	// Create a short-lived subscription to collect messages
	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	handler := func(_ context.Context, msg *messaging.Message) error {
		// Filter by time range
		if msg.Timestamp.Before(req.FromTime) {
			return nil // skip messages before range
		}
		if !req.ToTime.IsZero() && msg.Timestamp.After(req.ToTime) {
			cancel() // past the end of range, stop collecting
			return nil
		}
		select {
		case collected <- msg:
		default:
			cancel() // buffer full
		}
		return nil
	}

	sub, err := h.broker.Subscribe(subCtx, req.Topic, handler)
	if err != nil {
		h.logger.Warn("Failed to subscribe for replay batch",
			zap.String("topic", req.Topic),
			zap.Error(err))
		return nil
	}
	defer func() {
		if sub != nil {
			_ = sub.Unsubscribe() //nolint:errcheck
		}
	}()

	// Wait for context to finish (timeout or cancel)
	<-subCtx.Done()
	close(collected)

	for msg := range collected {
		batch = append(batch, msg)
	}

	return batch
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
	h.activeReplays.Update(requestID, func(cur *ReplayProgress, ok bool) (*ReplayProgress, bool) {
		if !ok {
			return nil, false
		}
		cp := *cur
		cp.Status = status
		cp.ErrorMessage = message
		if status == ReplayStatusCompleted || status == ReplayStatusFailed || status == ReplayStatusCancelled {
			cp.EndTime = time.Now()
		}
		return &cp, true
	})
}

// updateProgressFields updates the LastProcessedID and Rate fields with proper synchronization
func (h *Handler) updateProgressFields(requestID string, lastProcessedID string, rate float64) {
	h.activeReplays.Update(requestID, func(cur *ReplayProgress, ok bool) (*ReplayProgress, bool) {
		if !ok {
			return nil, false
		}
		cp := *cur
		cp.LastProcessedID = lastProcessedID
		cp.Rate = rate
		return &cp, true
	})
}

// GetProgress returns the progress of a replay operation
func (h *Handler) GetProgress(requestID string) (*ReplayProgress, error) {
	progress, ok := h.activeReplays.Get(requestID)
	if !ok {
		return nil, fmt.Errorf("replay %s not found", requestID)
	}

	// Return a copy to avoid race conditions
	progressCopy := *progress
	return &progressCopy, nil
}

// CancelReplay cancels an ongoing replay operation
func (h *Handler) CancelReplay(requestID string) error {
	var notRunning bool
	var found bool
	h.activeReplays.Update(requestID, func(cur *ReplayProgress, ok bool) (*ReplayProgress, bool) {
		if !ok {
			return nil, false
		}
		found = true
		if cur.Status != ReplayStatusRunning && cur.Status != ReplayStatusPending {
			notRunning = true
			return cur, true
		}
		cp := *cur
		cp.Status = ReplayStatusCancelled
		cp.ErrorMessage = "cancelled by user"
		cp.EndTime = time.Now()
		return &cp, true
	})

	if !found {
		return fmt.Errorf("replay %s not found", requestID)
	}
	if notRunning {
		progress, _ := h.activeReplays.Get(requestID)
		return fmt.Errorf("replay %s is not running (status: %s)", requestID, progress.Status)
	}

	h.logger.Info("Replay cancelled", zap.String("request_id", requestID))
	return nil
}

// ListReplays returns all active and recent replay operations
func (h *Handler) ListReplays() []*ReplayProgress {
	replays := make([]*ReplayProgress, 0, h.activeReplays.Len())
	h.activeReplays.Range(func(_ string, progress *ReplayProgress) bool {
		progressCopy := *progress
		replays = append(replays, &progressCopy)
		return true
	})
	return replays
}

// CleanupOldReplays removes completed/failed replays older than maxAge
func (h *Handler) CleanupOldReplays(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	var toRemove []string

	h.activeReplays.Range(func(id string, progress *ReplayProgress) bool {
		if progress.Status == ReplayStatusCompleted ||
			progress.Status == ReplayStatusFailed ||
			progress.Status == ReplayStatusCancelled {
			if !progress.EndTime.IsZero() && progress.EndTime.Before(cutoff) {
				toRemove = append(toRemove, id)
			}
		}
		return true
	})
	for _, id := range toRemove {
		h.activeReplays.Delete(id)
	}

	return len(toRemove)
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
	Topic       string         `json:"topic" binding:"required"`
	FromTime    string         `json:"from_time" binding:"required"`
	ToTime      string         `json:"to_time,omitempty"`
	TargetTopic string         `json:"target_topic,omitempty"`
	Filter      *ReplayFilter  `json:"filter,omitempty"`
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
