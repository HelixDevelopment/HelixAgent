// Package dlq provides dead letter queue processing for the messaging system.
package dlq

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

// ProcessorConfig configures the DLQ processor
type ProcessorConfig struct {
	// MaxRetries is the maximum number of times to retry a message
	MaxRetries int
	// RetryDelay is the initial delay between retries
	RetryDelay time.Duration
	// RetryBackoffMultiplier multiplies the delay on each retry
	RetryBackoffMultiplier float64
	// MaxRetryDelay caps the maximum retry delay
	MaxRetryDelay time.Duration
	// ProcessingTimeout is the timeout for processing a single message
	ProcessingTimeout time.Duration
	// BatchSize is the number of messages to process in a batch
	BatchSize int
	// PollInterval is how often to poll the DLQ for messages
	PollInterval time.Duration
}

// DefaultConfig returns sensible defaults for DLQ processing
func DefaultConfig() ProcessorConfig {
	return ProcessorConfig{
		MaxRetries:             3,
		RetryDelay:             1 * time.Second,
		RetryBackoffMultiplier: 2.0,
		MaxRetryDelay:          30 * time.Second,
		ProcessingTimeout:      30 * time.Second,
		BatchSize:              10,
		PollInterval:           5 * time.Second,
	}
}

// DeadLetterMessage represents a message in the dead letter queue
type DeadLetterMessage struct {
	ID              string                 `json:"id"`
	OriginalQueue   string                 `json:"original_queue"`
	OriginalTopic   string                 `json:"original_topic"`
	OriginalMessage *messaging.Message     `json:"original_message"`
	FailureReason   string                 `json:"failure_reason"`
	FailureDetails  map[string]interface{} `json:"failure_details,omitempty"`
	RetryCount      int                    `json:"retry_count"`
	FirstFailure    time.Time              `json:"first_failure"`
	LastFailure     time.Time              `json:"last_failure"`
	NextRetry       time.Time              `json:"next_retry,omitempty"`
	Status          DLQStatus              `json:"status"`
}

// DLQStatus represents the status of a DLQ message
type DLQStatus string

const (
	StatusPending    DLQStatus = "pending"
	StatusRetrying   DLQStatus = "retrying"
	StatusProcessed  DLQStatus = "processed"
	StatusDiscarded  DLQStatus = "discarded"
	StatusExpired    DLQStatus = "expired"
)

// ProcessorMetrics tracks DLQ processor metrics
type ProcessorMetrics struct {
	MessagesProcessed  int64
	MessagesRetried    int64
	MessagesDiscarded  int64
	MessagesExpired    int64
	ProcessingErrors   int64
	CurrentQueueDepth  int64
	AverageProcessTime time.Duration
}

// Processor handles dead letter queue processing
type Processor struct {
	config  ProcessorConfig
	broker  messaging.MessageBroker
	logger  *zap.Logger
	metrics ProcessorMetrics

	handlers map[string]RetryHandler
	mu       sync.RWMutex

	running   atomic.Bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// RetryHandler handles retry logic for specific message types
type RetryHandler func(ctx context.Context, msg *DeadLetterMessage) error

// NewProcessor creates a new DLQ processor
func NewProcessor(broker messaging.MessageBroker, config ProcessorConfig, logger *zap.Logger) *Processor {
	return &Processor{
		config:   config,
		broker:   broker,
		logger:   logger,
		handlers: make(map[string]RetryHandler),
	}
}

// RegisterHandler registers a retry handler for a specific message type
func (p *Processor) RegisterHandler(messageType string, handler RetryHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[messageType] = handler
}

// Start begins processing the dead letter queue
func (p *Processor) Start(ctx context.Context) error {
	if p.running.Load() {
		return fmt.Errorf("processor already running")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.running.Store(true)

	p.wg.Add(1)
	go p.processLoop()

	p.logger.Info("DLQ processor started",
		zap.Int("max_retries", p.config.MaxRetries),
		zap.Duration("poll_interval", p.config.PollInterval),
		zap.Int("batch_size", p.config.BatchSize))

	return nil
}

// Stop gracefully stops the processor
func (p *Processor) Stop() error {
	if !p.running.Load() {
		return nil
	}

	p.cancel()
	p.wg.Wait()
	p.running.Store(false)

	p.logger.Info("DLQ processor stopped",
		zap.Int64("messages_processed", atomic.LoadInt64(&p.metrics.MessagesProcessed)),
		zap.Int64("messages_retried", atomic.LoadInt64(&p.metrics.MessagesRetried)))

	return nil
}

// processLoop is the main processing loop
func (p *Processor) processLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.processBatch()
		}
	}
}

// processBatch processes a batch of DLQ messages
func (p *Processor) processBatch() {
	// Subscribe to DLQ and process messages
	ctx, cancel := context.WithTimeout(p.ctx, p.config.ProcessingTimeout*time.Duration(p.config.BatchSize))
	defer cancel()

	processed := 0
	messagesCh := make(chan *messaging.Message, p.config.BatchSize)

	// Create a temporary subscription to fetch messages
	sub, err := p.broker.Subscribe(ctx, "helixagent.dlq", func(ctx context.Context, msg *messaging.Message) error {
		select {
		case messagesCh <- msg:
		default:
			// Channel full, skip this message
		}
		return nil
	})
	if err != nil {
		p.logger.Error("Failed to subscribe to DLQ", zap.Error(err))
		return
	}
	defer sub.Unsubscribe()

	// Wait for messages or timeout
	timeout := time.After(p.config.PollInterval / 2)

	for processed < p.config.BatchSize {
		select {
		case msg := <-messagesCh:
			if err := p.processMessage(ctx, msg); err != nil {
				p.logger.Error("Failed to process DLQ message",
					zap.String("message_id", msg.ID),
					zap.Error(err))
				atomic.AddInt64(&p.metrics.ProcessingErrors, 1)
			} else {
				processed++
			}
		case <-timeout:
			return
		case <-ctx.Done():
			return
		}
	}
}

// processMessage processes a single DLQ message
func (p *Processor) processMessage(ctx context.Context, msg *messaging.Message) error {
	startTime := time.Now()

	// Parse DLQ message
	var dlqMsg DeadLetterMessage
	if err := json.Unmarshal(msg.Payload, &dlqMsg); err != nil {
		// If not a proper DLQ message, just log and discard
		p.logger.Warn("Invalid DLQ message format", zap.String("message_id", msg.ID))
		atomic.AddInt64(&p.metrics.MessagesDiscarded, 1)
		return nil
	}

	// Check retry count
	if dlqMsg.RetryCount >= p.config.MaxRetries {
		p.logger.Warn("Message exceeded max retries, discarding",
			zap.String("message_id", dlqMsg.ID),
			zap.Int("retry_count", dlqMsg.RetryCount),
			zap.String("failure_reason", dlqMsg.FailureReason))
		atomic.AddInt64(&p.metrics.MessagesDiscarded, 1)
		dlqMsg.Status = StatusDiscarded
		return p.updateDLQMessage(ctx, &dlqMsg)
	}

	// Check if retry is due
	if time.Now().Before(dlqMsg.NextRetry) {
		return nil // Not ready for retry yet
	}

	// Find handler for message type
	p.mu.RLock()
	handler, ok := p.handlers[dlqMsg.OriginalMessage.Type]
	p.mu.RUnlock()

	if !ok {
		// Use default retry handler
		handler = p.defaultRetryHandler
	}

	// Update status to retrying
	dlqMsg.Status = StatusRetrying
	dlqMsg.RetryCount++

	// Execute retry
	retryCtx, retryCancel := context.WithTimeout(ctx, p.config.ProcessingTimeout)
	defer retryCancel()

	if err := handler(retryCtx, &dlqMsg); err != nil {
		// Retry failed, schedule next retry
		delay := p.calculateRetryDelay(dlqMsg.RetryCount)
		dlqMsg.NextRetry = time.Now().Add(delay)
		dlqMsg.LastFailure = time.Now()
		dlqMsg.Status = StatusPending
		dlqMsg.FailureDetails["last_error"] = err.Error()

		p.logger.Warn("Retry failed, scheduling next attempt",
			zap.String("message_id", dlqMsg.ID),
			zap.Int("retry_count", dlqMsg.RetryCount),
			zap.Duration("next_retry_in", delay),
			zap.Error(err))

		atomic.AddInt64(&p.metrics.MessagesRetried, 1)
		return p.updateDLQMessage(ctx, &dlqMsg)
	}

	// Retry succeeded
	dlqMsg.Status = StatusProcessed
	atomic.AddInt64(&p.metrics.MessagesProcessed, 1)

	p.logger.Info("Successfully reprocessed DLQ message",
		zap.String("message_id", dlqMsg.ID),
		zap.Int("retry_count", dlqMsg.RetryCount),
		zap.Duration("processing_time", time.Since(startTime)))

	return p.updateDLQMessage(ctx, &dlqMsg)
}

// defaultRetryHandler republishes the original message to its original queue
func (p *Processor) defaultRetryHandler(ctx context.Context, dlqMsg *DeadLetterMessage) error {
	// Republish to original topic/queue
	if dlqMsg.OriginalTopic != "" {
		return p.broker.Publish(ctx, dlqMsg.OriginalTopic, dlqMsg.OriginalMessage)
	}
	return fmt.Errorf("no original topic specified for message %s", dlqMsg.ID)
}

// calculateRetryDelay calculates the delay before next retry using exponential backoff
func (p *Processor) calculateRetryDelay(retryCount int) time.Duration {
	delay := p.config.RetryDelay
	for i := 1; i < retryCount; i++ {
		delay = time.Duration(float64(delay) * p.config.RetryBackoffMultiplier)
		if delay > p.config.MaxRetryDelay {
			delay = p.config.MaxRetryDelay
			break
		}
	}
	return delay
}

// updateDLQMessage updates a DLQ message (e.g., to track status)
func (p *Processor) updateDLQMessage(ctx context.Context, dlqMsg *DeadLetterMessage) error {
	// In a real implementation, this would update a persistent store
	// For now, we just log the update
	p.logger.Debug("DLQ message updated",
		zap.String("message_id", dlqMsg.ID),
		zap.String("status", string(dlqMsg.Status)),
		zap.Int("retry_count", dlqMsg.RetryCount))
	return nil
}

// GetMetrics returns current processor metrics
func (p *Processor) GetMetrics() ProcessorMetrics {
	return ProcessorMetrics{
		MessagesProcessed:  atomic.LoadInt64(&p.metrics.MessagesProcessed),
		MessagesRetried:    atomic.LoadInt64(&p.metrics.MessagesRetried),
		MessagesDiscarded:  atomic.LoadInt64(&p.metrics.MessagesDiscarded),
		MessagesExpired:    atomic.LoadInt64(&p.metrics.MessagesExpired),
		ProcessingErrors:   atomic.LoadInt64(&p.metrics.ProcessingErrors),
		CurrentQueueDepth:  atomic.LoadInt64(&p.metrics.CurrentQueueDepth),
	}
}

// ReprocessMessage manually triggers reprocessing of a specific DLQ message
func (p *Processor) ReprocessMessage(ctx context.Context, messageID string) error {
	p.logger.Info("Manual reprocess requested", zap.String("message_id", messageID))
	// In a real implementation, this would fetch and reprocess a specific message
	return nil
}

// DiscardMessage permanently discards a DLQ message
func (p *Processor) DiscardMessage(ctx context.Context, messageID string, reason string) error {
	p.logger.Info("Manual discard requested",
		zap.String("message_id", messageID),
		zap.String("reason", reason))
	atomic.AddInt64(&p.metrics.MessagesDiscarded, 1)
	return nil
}

// ListMessages returns a list of messages currently in the DLQ
func (p *Processor) ListMessages(ctx context.Context, limit, offset int) ([]*DeadLetterMessage, error) {
	// In a real implementation, this would query a persistent store
	return nil, nil
}
