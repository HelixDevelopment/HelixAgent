package messaging

import (
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// BrokerMetrics holds metrics for a message broker.
type BrokerMetrics struct {
	// Connection metrics
	ConnectionAttempts   atomic.Int64 `json:"connection_attempts"`
	ConnectionSuccesses  atomic.Int64 `json:"connection_successes"`
	ConnectionFailures   atomic.Int64 `json:"connection_failures"`
	ReconnectionAttempts atomic.Int64 `json:"reconnection_attempts"`
	CurrentConnections   atomic.Int64 `json:"current_connections"`

	// Publish metrics
	MessagesPublished    atomic.Int64 `json:"messages_published"`
	PublishSuccesses     atomic.Int64 `json:"publish_successes"`
	PublishFailures      atomic.Int64 `json:"publish_failures"`
	PublishConfirmations atomic.Int64 `json:"publish_confirmations"`
	PublishTimeouts      atomic.Int64 `json:"publish_timeouts"`
	BytesPublished       atomic.Int64 `json:"bytes_published"`
	BatchesPublished     atomic.Int64 `json:"batches_published"`

	// Subscribe/Consume metrics
	MessagesReceived     atomic.Int64 `json:"messages_received"`
	MessagesConsumed     atomic.Int64 `json:"messages_consumed"`
	MessagesProcessed    atomic.Int64 `json:"messages_processed"`
	MessagesFailed       atomic.Int64 `json:"messages_failed"`
	MessagesRetried      atomic.Int64 `json:"messages_retried"`
	MessagesDeadLettered atomic.Int64 `json:"messages_dead_lettered"`
	MessagesAcked        atomic.Int64 `json:"messages_acked"`
	MessagesNacked       atomic.Int64 `json:"messages_nacked"`
	BytesReceived        atomic.Int64 `json:"bytes_received"`
	BytesConsumed        atomic.Int64 `json:"bytes_consumed"`
	ActiveSubscriptions  atomic.Int64 `json:"active_subscriptions"`

	// Latency metrics (stored as nanoseconds)
	PublishLatencyTotal   atomic.Int64 `json:"publish_latency_total_ns"`
	PublishLatencyCount   atomic.Int64 `json:"publish_latency_count"`
	SubscribeLatencyTotal atomic.Int64 `json:"subscribe_latency_total_ns"`
	SubscribeLatencyCount atomic.Int64 `json:"subscribe_latency_count"`

	// Error metrics
	TotalErrors         atomic.Int64 `json:"total_errors"`
	ConnectionErrors    atomic.Int64 `json:"connection_errors"`
	PublishErrors       atomic.Int64 `json:"publish_errors"`
	SubscribeErrors     atomic.Int64 `json:"subscribe_errors"`
	SerializationErrors atomic.Int64 `json:"serialization_errors"`

	// Queue/Topic metrics
	QueuesDeclared atomic.Int64 `json:"queues_declared"`
	TopicsCreated  atomic.Int64 `json:"topics_created"`

	// Timestamps
	StartTime         time.Time `json:"start_time"`
	LastPublishTime   time.Time `json:"last_publish_time"`
	LastReceiveTime   time.Time `json:"last_receive_time"`
	LastErrorTime     time.Time `json:"last_error_time"`
	LastReconnectTime time.Time `json:"last_reconnect_time"`

	// Mutex for timestamp updates
	mu sync.RWMutex
}

// NewBrokerMetrics creates a new BrokerMetrics instance.
func NewBrokerMetrics() *BrokerMetrics {
	return &BrokerMetrics{
		StartTime: time.Now().UTC(),
	}
}

// RecordConnectionAttempt records a connection attempt.
func (m *BrokerMetrics) RecordConnectionAttempt() {
	m.ConnectionAttempts.Add(1)
}

// RecordConnectionSuccess records a successful connection.
func (m *BrokerMetrics) RecordConnectionSuccess() {
	m.ConnectionSuccesses.Add(1)
	m.CurrentConnections.Add(1)
}

// RecordConnectionFailure records a failed connection.
func (m *BrokerMetrics) RecordConnectionFailure() {
	m.ConnectionFailures.Add(1)
	m.ConnectionErrors.Add(1)
	m.TotalErrors.Add(1)
}

// RecordDisconnection records a disconnection.
func (m *BrokerMetrics) RecordDisconnection() {
	m.CurrentConnections.Add(-1)
}

// RecordReconnectionAttempt records a reconnection attempt.
func (m *BrokerMetrics) RecordReconnectionAttempt() {
	m.ReconnectionAttempts.Add(1)
	m.mu.Lock()
	m.LastReconnectTime = time.Now().UTC()
	m.mu.Unlock()
}

// RecordPublish records a publish operation.
func (m *BrokerMetrics) RecordPublish(bytes int64, latency time.Duration, success bool) {
	m.MessagesPublished.Add(1)
	if success {
		m.PublishSuccesses.Add(1)
		m.BytesPublished.Add(bytes)
	} else {
		m.PublishFailures.Add(1)
		m.PublishErrors.Add(1)
		m.TotalErrors.Add(1)
	}
	m.PublishLatencyTotal.Add(int64(latency))
	m.PublishLatencyCount.Add(1)
	m.mu.Lock()
	m.LastPublishTime = time.Now().UTC()
	m.mu.Unlock()
}

// RecordBatchPublish records a batch publish operation.
func (m *BrokerMetrics) RecordBatchPublish(count int, bytes int64, latency time.Duration, success bool) {
	m.BatchesPublished.Add(1)
	m.MessagesPublished.Add(int64(count))
	if success {
		m.PublishSuccesses.Add(int64(count))
		m.BytesPublished.Add(bytes)
	} else {
		m.PublishFailures.Add(int64(count))
		m.PublishErrors.Add(1)
		m.TotalErrors.Add(1)
	}
	m.PublishLatencyTotal.Add(int64(latency))
	m.PublishLatencyCount.Add(1)
	m.mu.Lock()
	m.LastPublishTime = time.Now().UTC()
	m.mu.Unlock()
}

// RecordPublishConfirmation records a publisher confirmation.
func (m *BrokerMetrics) RecordPublishConfirmation() {
	m.PublishConfirmations.Add(1)
}

// RecordPublishTimeout records a publish timeout.
func (m *BrokerMetrics) RecordPublishTimeout() {
	m.PublishTimeouts.Add(1)
	m.PublishErrors.Add(1)
	m.TotalErrors.Add(1)
}

// RecordReceive records a message receive operation.
func (m *BrokerMetrics) RecordReceive(bytes int64, latency time.Duration) {
	m.MessagesReceived.Add(1)
	m.BytesReceived.Add(bytes)
	m.SubscribeLatencyTotal.Add(int64(latency))
	m.SubscribeLatencyCount.Add(1)
	m.mu.Lock()
	m.LastReceiveTime = time.Now().UTC()
	m.mu.Unlock()
}

// RecordConsume records a message consume operation.
func (m *BrokerMetrics) RecordConsume(bytes int64, latency time.Duration, success bool) {
	m.MessagesConsumed.Add(1)
	if success {
		m.BytesConsumed.Add(bytes)
	}
	m.SubscribeLatencyTotal.Add(int64(latency))
	m.SubscribeLatencyCount.Add(1)
	m.mu.Lock()
	m.LastReceiveTime = time.Now().UTC()
	m.mu.Unlock()
}

// RecordAck records a message acknowledgment.
func (m *BrokerMetrics) RecordAck() {
	m.MessagesAcked.Add(1)
}

// RecordNack records a message negative acknowledgment.
func (m *BrokerMetrics) RecordNack() {
	m.MessagesNacked.Add(1)
}

// RecordProcessed records a successfully processed message.
func (m *BrokerMetrics) RecordProcessed() {
	m.MessagesProcessed.Add(1)
}

// RecordFailed records a failed message processing.
func (m *BrokerMetrics) RecordFailed() {
	m.MessagesFailed.Add(1)
	m.SubscribeErrors.Add(1)
	m.TotalErrors.Add(1)
	m.mu.Lock()
	m.LastErrorTime = time.Now().UTC()
	m.mu.Unlock()
}

// RecordRetry records a message retry.
func (m *BrokerMetrics) RecordRetry() {
	m.MessagesRetried.Add(1)
}

// RecordDeadLettered records a dead-lettered message.
func (m *BrokerMetrics) RecordDeadLettered() {
	m.MessagesDeadLettered.Add(1)
}

// RecordSubscription records a new subscription.
func (m *BrokerMetrics) RecordSubscription() {
	m.ActiveSubscriptions.Add(1)
}

// RecordUnsubscription records an unsubscription.
func (m *BrokerMetrics) RecordUnsubscription() {
	m.ActiveSubscriptions.Add(-1)
}

// RecordQueueDeclared records a queue declaration.
func (m *BrokerMetrics) RecordQueueDeclared() {
	m.QueuesDeclared.Add(1)
}

// RecordTopicCreated records a topic creation.
func (m *BrokerMetrics) RecordTopicCreated() {
	m.TopicsCreated.Add(1)
}

// RecordSerializationError records a serialization error.
func (m *BrokerMetrics) RecordSerializationError() {
	m.SerializationErrors.Add(1)
	m.TotalErrors.Add(1)
}

// RecordError records a general error.
func (m *BrokerMetrics) RecordError() {
	m.TotalErrors.Add(1)
	m.mu.Lock()
	m.LastErrorTime = time.Now().UTC()
	m.mu.Unlock()
}

// GetAveragePublishLatency returns the average publish latency.
func (m *BrokerMetrics) GetAveragePublishLatency() time.Duration {
	count := m.PublishLatencyCount.Load()
	if count == 0 {
		return 0
	}
	return time.Duration(m.PublishLatencyTotal.Load() / count)
}

// GetAverageSubscribeLatency returns the average subscribe latency.
func (m *BrokerMetrics) GetAverageSubscribeLatency() time.Duration {
	count := m.SubscribeLatencyCount.Load()
	if count == 0 {
		return 0
	}
	return time.Duration(m.SubscribeLatencyTotal.Load() / count)
}

// GetUptime returns the broker uptime.
func (m *BrokerMetrics) GetUptime() time.Duration {
	return time.Since(m.StartTime)
}

// GetPublishSuccessRate returns the publish success rate (0-1).
func (m *BrokerMetrics) GetPublishSuccessRate() float64 {
	total := m.MessagesPublished.Load()
	if total == 0 {
		return 1.0
	}
	return float64(m.PublishSuccesses.Load()) / float64(total)
}

// GetProcessingSuccessRate returns the message processing success rate (0-1).
func (m *BrokerMetrics) GetProcessingSuccessRate() float64 {
	received := m.MessagesReceived.Load()
	if received == 0 {
		return 1.0
	}
	return float64(m.MessagesProcessed.Load()) / float64(received)
}

// GetLastPublishTime returns the last publish time.
func (m *BrokerMetrics) GetLastPublishTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.LastPublishTime
}

// GetLastReceiveTime returns the last receive time.
func (m *BrokerMetrics) GetLastReceiveTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.LastReceiveTime
}

// GetLastErrorTime returns the last error time.
func (m *BrokerMetrics) GetLastErrorTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.LastErrorTime
}

// Reset resets all metrics.
func (m *BrokerMetrics) Reset() {
	m.ConnectionAttempts.Store(0)
	m.ConnectionSuccesses.Store(0)
	m.ConnectionFailures.Store(0)
	m.ReconnectionAttempts.Store(0)
	m.CurrentConnections.Store(0)
	m.MessagesPublished.Store(0)
	m.PublishSuccesses.Store(0)
	m.PublishFailures.Store(0)
	m.PublishConfirmations.Store(0)
	m.PublishTimeouts.Store(0)
	m.BytesPublished.Store(0)
	m.BatchesPublished.Store(0)
	m.MessagesReceived.Store(0)
	m.MessagesConsumed.Store(0)
	m.MessagesProcessed.Store(0)
	m.MessagesFailed.Store(0)
	m.MessagesRetried.Store(0)
	m.MessagesDeadLettered.Store(0)
	m.MessagesAcked.Store(0)
	m.MessagesNacked.Store(0)
	m.BytesReceived.Store(0)
	m.BytesConsumed.Store(0)
	m.ActiveSubscriptions.Store(0)
	m.PublishLatencyTotal.Store(0)
	m.PublishLatencyCount.Store(0)
	m.SubscribeLatencyTotal.Store(0)
	m.SubscribeLatencyCount.Store(0)
	m.TotalErrors.Store(0)
	m.ConnectionErrors.Store(0)
	m.PublishErrors.Store(0)
	m.SubscribeErrors.Store(0)
	m.SerializationErrors.Store(0)
	m.QueuesDeclared.Store(0)
	m.TopicsCreated.Store(0)
	m.mu.Lock()
	m.StartTime = time.Now().UTC()
	m.LastPublishTime = time.Time{}
	m.LastReceiveTime = time.Time{}
	m.LastErrorTime = time.Time{}
	m.LastReconnectTime = time.Time{}
	m.mu.Unlock()
}

// Clone creates a snapshot of the current metrics.
func (m *BrokerMetrics) Clone() *BrokerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := &BrokerMetrics{
		StartTime:         m.StartTime,
		LastPublishTime:   m.LastPublishTime,
		LastReceiveTime:   m.LastReceiveTime,
		LastErrorTime:     m.LastErrorTime,
		LastReconnectTime: m.LastReconnectTime,
	}
	clone.ConnectionAttempts.Store(m.ConnectionAttempts.Load())
	clone.ConnectionSuccesses.Store(m.ConnectionSuccesses.Load())
	clone.ConnectionFailures.Store(m.ConnectionFailures.Load())
	clone.ReconnectionAttempts.Store(m.ReconnectionAttempts.Load())
	clone.CurrentConnections.Store(m.CurrentConnections.Load())
	clone.MessagesPublished.Store(m.MessagesPublished.Load())
	clone.PublishSuccesses.Store(m.PublishSuccesses.Load())
	clone.PublishFailures.Store(m.PublishFailures.Load())
	clone.PublishConfirmations.Store(m.PublishConfirmations.Load())
	clone.PublishTimeouts.Store(m.PublishTimeouts.Load())
	clone.BytesPublished.Store(m.BytesPublished.Load())
	clone.BatchesPublished.Store(m.BatchesPublished.Load())
	clone.MessagesReceived.Store(m.MessagesReceived.Load())
	clone.MessagesConsumed.Store(m.MessagesConsumed.Load())
	clone.MessagesProcessed.Store(m.MessagesProcessed.Load())
	clone.MessagesFailed.Store(m.MessagesFailed.Load())
	clone.MessagesRetried.Store(m.MessagesRetried.Load())
	clone.MessagesDeadLettered.Store(m.MessagesDeadLettered.Load())
	clone.MessagesAcked.Store(m.MessagesAcked.Load())
	clone.MessagesNacked.Store(m.MessagesNacked.Load())
	clone.BytesReceived.Store(m.BytesReceived.Load())
	clone.BytesConsumed.Store(m.BytesConsumed.Load())
	clone.ActiveSubscriptions.Store(m.ActiveSubscriptions.Load())
	clone.PublishLatencyTotal.Store(m.PublishLatencyTotal.Load())
	clone.PublishLatencyCount.Store(m.PublishLatencyCount.Load())
	clone.SubscribeLatencyTotal.Store(m.SubscribeLatencyTotal.Load())
	clone.SubscribeLatencyCount.Store(m.SubscribeLatencyCount.Load())
	clone.TotalErrors.Store(m.TotalErrors.Load())
	clone.ConnectionErrors.Store(m.ConnectionErrors.Load())
	clone.PublishErrors.Store(m.PublishErrors.Load())
	clone.SubscribeErrors.Store(m.SubscribeErrors.Load())
	clone.SerializationErrors.Store(m.SerializationErrors.Load())
	clone.QueuesDeclared.Store(m.QueuesDeclared.Load())
	clone.TopicsCreated.Store(m.TopicsCreated.Load())

	return clone
}

// QueueStats holds statistics for a queue.
type QueueStats struct {
	// Name is the queue name.
	Name string `json:"name"`
	// Messages is the number of messages in the queue.
	Messages int64 `json:"messages"`
	// Consumers is the number of consumers.
	Consumers int64 `json:"consumers"`
	// MessagesReady is the number of messages ready for delivery.
	MessagesReady int64 `json:"messages_ready"`
	// MessagesUnacked is the number of unacknowledged messages.
	MessagesUnacked int64 `json:"messages_unacked"`
	// MessageBytes is the total size of messages in bytes.
	MessageBytes int64 `json:"message_bytes"`
	// PublishRate is the publish rate (messages/second).
	PublishRate float64 `json:"publish_rate"`
	// DeliverRate is the deliver rate (messages/second).
	DeliverRate float64 `json:"deliver_rate"`
	// Timestamp is when the stats were collected.
	Timestamp time.Time `json:"timestamp"`
}

// TopicMetadata holds metadata for a Kafka topic.
type TopicMetadata struct {
	// Name is the topic name.
	Name string `json:"name"`
	// Partitions is the number of partitions.
	Partitions int `json:"partitions"`
	// ReplicationFactor is the replication factor.
	ReplicationFactor int `json:"replication_factor"`
	// RetentionMs is the retention period in milliseconds.
	RetentionMs int64 `json:"retention_ms"`
	// CleanupPolicy is the cleanup policy (delete, compact).
	CleanupPolicy string `json:"cleanup_policy"`
	// PartitionInfo contains per-partition information.
	PartitionInfo []PartitionInfo `json:"partition_info"`
	// Timestamp is when the metadata was collected.
	Timestamp time.Time `json:"timestamp"`
}

// PartitionInfo holds information about a Kafka partition.
type PartitionInfo struct {
	// ID is the partition ID.
	ID int32 `json:"id"`
	// Leader is the leader broker ID.
	Leader int32 `json:"leader"`
	// Replicas are the replica broker IDs.
	Replicas []int32 `json:"replicas"`
	// ISR (In-Sync Replicas) are the in-sync replica broker IDs.
	ISR []int32 `json:"isr"`
	// HighWatermark is the high watermark offset.
	HighWatermark int64 `json:"high_watermark"`
	// LowWatermark is the low watermark offset.
	LowWatermark int64 `json:"low_watermark"`
}

// MetricsSnapshot represents a point-in-time snapshot of metrics.
type MetricsSnapshot struct {
	BrokerType  BrokerType      `json:"broker_type"`
	Metrics     *BrokerMetrics  `json:"metrics"`
	QueueStats  []QueueStats    `json:"queue_stats,omitempty"`
	TopicStats  []TopicMetadata `json:"topic_stats,omitempty"`
	CollectedAt time.Time       `json:"collected_at"`
}

// NewMetricsSnapshot creates a new metrics snapshot.
func NewMetricsSnapshot(brokerType BrokerType, metrics *BrokerMetrics) *MetricsSnapshot {
	return &MetricsSnapshot{
		BrokerType:  brokerType,
		Metrics:     metrics.Clone(),
		CollectedAt: time.Now().UTC(),
	}
}

// WithQueueStats adds queue stats to the snapshot.
func (s *MetricsSnapshot) WithQueueStats(stats []QueueStats) *MetricsSnapshot {
	s.QueueStats = stats
	return s
}

// WithTopicStats adds topic stats to the snapshot.
func (s *MetricsSnapshot) WithTopicStats(stats []TopicMetadata) *MetricsSnapshot {
	s.TopicStats = stats
	return s
}

// maxCollectorSnapshots bounds the in-memory snapshot history kept by
// MetricsCollector.Start; older snapshots are dropped FIFO.
const maxCollectorSnapshots = 1000

// MetricsCollector collects metrics from multiple brokers.
//
// Concurrency model (CONST-029): brokers is a safe.Store, snapshots
// is a safe.Slice; no bare mu survives. Register/Unregister operate
// atomically on the store; Collect iterates a Snapshot of the store
// so new brokers registered mid-scan are either entirely included or
// entirely excluded. Start's tick handler appends snapshots and then
// does a trim-replace via Snapshot → slice-tail → Replace; the
// trim-replace pair is not atomic across concurrent Starts, but
// MetricsCollector is designed for a single Start per instance.
type MetricsCollector struct {
	brokers   *safe.Store[string, MessageBroker]
	snapshots *safe.Slice[*MetricsSnapshot]
	interval  time.Duration
	stopCh    chan struct{}
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		brokers:   safe.NewStore[string, MessageBroker](),
		snapshots: safe.NewSlice[*MetricsSnapshot](),
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

// Register registers a broker for metrics collection.
func (c *MetricsCollector) Register(name string, broker MessageBroker) {
	c.brokers.Put(name, broker)
}

// Unregister removes a broker from metrics collection.
func (c *MetricsCollector) Unregister(name string) {
	c.brokers.Delete(name)
}

// Collect collects metrics from all registered brokers.
func (c *MetricsCollector) Collect() []*MetricsSnapshot {
	brokers := c.brokers.Snapshot()
	snapshots := make([]*MetricsSnapshot, 0, len(brokers))
	for _, broker := range brokers {
		snapshot := NewMetricsSnapshot(broker.BrokerType(), broker.GetMetrics())
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

// Start starts periodic metrics collection.
func (c *MetricsCollector) Start() {
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				for _, s := range c.Collect() {
					c.snapshots.Append(s)
				}
				// Keep the last maxCollectorSnapshots entries.
				if c.snapshots.Len() > maxCollectorSnapshots {
					snap := c.snapshots.Snapshot()
					c.snapshots.Replace(snap[len(snap)-maxCollectorSnapshots:])
				}
			case <-c.stopCh:
				return
			}
		}
	}()
}

// Stop stops metrics collection.
func (c *MetricsCollector) Stop() {
	close(c.stopCh)
}

// GetSnapshots returns collected snapshots.
func (c *MetricsCollector) GetSnapshots() []*MetricsSnapshot {
	return c.snapshots.Snapshot()
}
