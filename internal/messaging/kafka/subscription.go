// Package kafka provides an Apache Kafka message broker implementation.
package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"dev.helix.agent/internal/messaging"
)

// Subscription represents a Kafka subscription.
type Subscription struct {
	id       string
	topic    string
	broker   *Broker
	reader   *kafka.Reader
	handler  messaging.MessageHandler
	options  *messaging.SubscribeOptions
	active   bool
	stopCh   chan struct{}
	mu       sync.RWMutex
}

// ID returns the subscription ID.
func (s *Subscription) ID() string {
	return s.id
}

// Topic returns the subscribed topic.
func (s *Subscription) Topic() string {
	return s.topic
}

// IsActive returns true if the subscription is active.
func (s *Subscription) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// Unsubscribe cancels the subscription.
func (s *Subscription) Unsubscribe() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return nil
	}

	s.active = false
	close(s.stopCh)

	// Close reader
	if s.reader != nil {
		s.reader.Close()
	}

	// Remove from broker's subscriber list
	s.broker.mu.Lock()
	delete(s.broker.subscribers, s.id)
	delete(s.broker.readers, s.id)
	s.broker.metrics.RecordUnsubscription()
	s.broker.mu.Unlock()

	return nil
}

// consume processes incoming messages.
func (s *Subscription) consume(ctx context.Context) {
	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			s.cancel()
			return
		default:
			s.fetchAndProcess(ctx)
		}
	}
}

// fetchAndProcess fetches a message and processes it.
func (s *Subscription) fetchAndProcess(ctx context.Context) {
	// Create a context with timeout for fetch
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	kafkaMsg, err := s.reader.FetchMessage(fetchCtx)
	if err != nil {
		if ctx.Err() != nil || fetchCtx.Err() != nil {
			return
		}
		// Log error but continue
		s.broker.metrics.RecordError()
		return
	}

	s.processMessage(ctx, kafkaMsg)
}

// processMessage processes a single Kafka message.
func (s *Subscription) processMessage(ctx context.Context, kafkaMsg kafka.Message) {
	start := time.Now()

	// Convert Kafka message to Message
	msg := s.kafkaMessageToMessage(kafkaMsg)

	// Record consumption
	s.broker.metrics.RecordConsume(int64(len(kafkaMsg.Value)), time.Since(start), true)

	// Process with handler
	var err error
	if s.options.Timeout > 0 {
		handlerCtx, cancel := context.WithTimeout(ctx, s.options.Timeout)
		err = s.handler(handlerCtx, msg)
		cancel()
	} else {
		err = s.handler(ctx, msg)
	}

	// Handle result
	if err != nil {
		s.handleError(ctx, kafkaMsg, msg, err)
	} else {
		s.handleSuccess(ctx, kafkaMsg, msg)
	}
}

// kafkaMessageToMessage converts a Kafka message to a Message.
func (s *Subscription) kafkaMessageToMessage(kafkaMsg kafka.Message) *messaging.Message {
	msg := &messaging.Message{
		Payload:   kafkaMsg.Value,
		Headers:   make(map[string]string),
		Timestamp: kafkaMsg.Time,
		Partition: int32(kafkaMsg.Partition),
		Offset:    kafkaMsg.Offset,
	}

	// Extract headers
	for _, h := range kafkaMsg.Headers {
		msg.Headers[h.Key] = string(h.Value)
	}

	// Extract known headers
	if id := getHeaderValue(kafkaMsg.Headers, "message_id"); id != "" {
		msg.ID = id
	}
	if msgType := getHeaderValue(kafkaMsg.Headers, "message_type"); msgType != "" {
		msg.Type = msgType
	}
	if traceID := getHeaderValue(kafkaMsg.Headers, "trace_id"); traceID != "" {
		msg.TraceID = traceID
	}

	return msg
}

// handleSuccess handles successful message processing.
func (s *Subscription) handleSuccess(ctx context.Context, kafkaMsg kafka.Message, msg *messaging.Message) {
	// Commit the message
	if !s.options.AutoAck {
		if err := s.reader.CommitMessages(ctx, kafkaMsg); err != nil {
			s.broker.metrics.RecordError()
		} else {
			s.broker.metrics.RecordAck()
		}
	}
	s.broker.metrics.RecordProcessed()
}

// handleError handles message processing errors.
func (s *Subscription) handleError(ctx context.Context, kafkaMsg kafka.Message, msg *messaging.Message, err error) {
	s.broker.metrics.RecordError()
	s.broker.metrics.RecordFailed()

	// Check if retry is enabled
	if s.options.RetryOnError && msg.RetryCount < s.options.MaxRetries {
		msg.RetryCount++

		// Retry delay
		if s.options.RetryDelay > 0 {
			time.Sleep(s.options.RetryDelay)
		}

		// Re-process the message
		handlerCtx := ctx
		if s.options.Timeout > 0 {
			var cancel context.CancelFunc
			handlerCtx, cancel = context.WithTimeout(ctx, s.options.Timeout)
			defer cancel()
		}

		retryErr := s.handler(handlerCtx, msg)
		if retryErr == nil {
			s.handleSuccess(ctx, kafkaMsg, msg)
			return
		}

		s.broker.metrics.RecordRetry()
	}

	// Max retries exceeded or retry disabled - commit anyway to move forward
	if !s.options.AutoAck {
		if err := s.reader.CommitMessages(ctx, kafkaMsg); err != nil {
			s.broker.metrics.RecordError()
		}
	}
}

// cancel cancels the subscription.
func (s *Subscription) cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false

	select {
	case <-s.stopCh:
		// Already closed
	default:
		close(s.stopCh)
	}
}
