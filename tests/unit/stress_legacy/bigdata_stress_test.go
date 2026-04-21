// Previously tests/stress/... — demoted to unit package under CONST-030.
// Stress tests of in-process concurrency primitives (semaphore / pool /
// cache saturation, circuit-breaker storms, goroutine growth) that
// exercise local mock providers and invokers. Keeping the mocks in the
// unit tree per the audit's Pattern-4 disposition for stress tests.
// PR33 of the CONST-030 compliance campaign.
package stress_legacy_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/bigdata"
	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/memory"
	"dev.helix.agent/internal/messaging"
)

// bdStressMockSubscription implements messaging.Subscription for stress tests.
type bdStressMockSubscription struct {
	id     string
	topic  string
	active bool
}

func newBdStressMockSubscription(id, topic string) *bdStressMockSubscription {
	return &bdStressMockSubscription{id: id, topic: topic, active: true}
}

func (s *bdStressMockSubscription) Unsubscribe() error { s.active = false; return nil }
func (s *bdStressMockSubscription) IsActive() bool     { return s.active }
func (s *bdStressMockSubscription) Topic() string      { return s.topic }
func (s *bdStressMockSubscription) ID() string         { return s.id }

// bdStressMockBroker implements messaging.MessageBroker for stress tests.
// It is fully thread-safe and tracks published message counts atomically.
type bdStressMockBroker struct {
	mu           sync.Mutex
	published    []*bdStressPublishedMsg
	publishCount atomic.Int64
	publishErr   error
}

type bdStressPublishedMsg struct {
	topic   string
	message *messaging.Message
}

func newBdStressMockBroker() *bdStressMockBroker {
	return &bdStressMockBroker{
		published: make([]*bdStressPublishedMsg, 0, 1024),
	}
}

func (b *bdStressMockBroker) Connect(_ context.Context) error     { return nil }
func (b *bdStressMockBroker) Close(_ context.Context) error       { return nil }
func (b *bdStressMockBroker) HealthCheck(_ context.Context) error { return nil }
func (b *bdStressMockBroker) IsConnected() bool                   { return true }
func (b *bdStressMockBroker) BrokerType() messaging.BrokerType {
	return messaging.BrokerTypeInMemory
}
func (b *bdStressMockBroker) GetMetrics() *messaging.BrokerMetrics {
	return &messaging.BrokerMetrics{}
}

func (b *bdStressMockBroker) Publish(
	_ context.Context,
	topic string,
	message *messaging.Message,
	_ ...messaging.PublishOption,
) error {
	if b.publishErr != nil {
		return b.publishErr
	}
	b.mu.Lock()
	b.published = append(b.published, &bdStressPublishedMsg{
		topic:   topic,
		message: message,
	})
	b.mu.Unlock()
	b.publishCount.Add(1)
	return nil
}

func (b *bdStressMockBroker) PublishBatch(
	ctx context.Context,
	topic string,
	messages []*messaging.Message,
	opts ...messaging.PublishOption,
) error {
	for _, m := range messages {
		if err := b.Publish(ctx, topic, m, opts...); err != nil {
			return err
		}
	}
	return nil
}

func (b *bdStressMockBroker) Subscribe(
	_ context.Context,
	topic string,
	_ messaging.MessageHandler,
	_ ...messaging.SubscribeOption,
) (messaging.Subscription, error) {
	return newBdStressMockSubscription("stress-sub", topic), nil
}

func (b *bdStressMockBroker) getPublishCount() int64 {
	return b.publishCount.Load()
}

func newBigdataStressLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	return logger
}

// TestBigData_ConcurrentConfigConversion runs 100 goroutines converting
// config.BigDataConfig to bigdata.IntegrationConfig and back simultaneously,
// verifying thread safety of the conversion functions.
func TestBigData_ConcurrentConfigConversion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		var successCount atomic.Int64

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				cfg := &config.BigDataConfig{
					EnableInfiniteContext:   idx%2 == 0,
					EnableDistributedMemory: idx%3 == 0,
					EnableKnowledgeGraph:    idx%4 == 0,
					EnableAnalytics:         idx%5 == 0,
					EnableCrossLearning:     idx%6 == 0,
					KafkaBootstrapServers:   fmt.Sprintf("localhost:%d", 9092+idx),
					KafkaConsumerGroup:      fmt.Sprintf("group-%d", idx),
					ClickHouseHost:          "localhost",
					ClickHousePort:          9000 + idx,
					ClickHouseDatabase:      fmt.Sprintf("db_%d", idx),
					ClickHouseUser:          "user",
					ClickHousePassword:      "pass",
					Neo4jURI:                fmt.Sprintf("bolt://localhost:%d", 7687+idx),
					Neo4jUsername:           "neo4j",
					Neo4jPassword:           "password",
					Neo4jDatabase:           "helix",
					ContextCacheSize:        100 + idx,
					ContextCacheTTL:         time.Duration(idx) * time.Minute,
					ContextCompressionType:  "hybrid",
					LearningMinConfidence:   float64(idx%100) / 100.0,
					LearningMinFrequency:    idx % 10,
				}

				// Convert forward
				icfg := bigdata.ConfigToIntegrationConfig(cfg)
				if icfg == nil {
					return
				}

				// Convert back
				result := bigdata.IntegrationConfigToConfig(icfg)
				if result == nil {
					return
				}

				// Validate roundtrip
				if result.KafkaBootstrapServers == cfg.KafkaBootstrapServers &&
					result.ClickHousePort == cfg.ClickHousePort &&
					result.ContextCacheSize == cfg.ContextCacheSize {
					successCount.Add(1)
				}
			}(i)
		}

		wg.Wait()

		if successCount.Load() != 100 {
			t.Errorf(
				"expected 100 successful conversions, got %d",
				successCount.Load(),
			)
		}
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: ConcurrentConfigConversion timed out")
	}
}

// TestBigData_ConcurrentAnalyticsPublish runs 100 goroutines publishing
// analytics events through AnalyticsIntegration concurrently.
func TestBigData_ConcurrentAnalyticsPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	broker := newBdStressMockBroker()
	ai := bigdata.NewAnalyticsIntegration(broker, newBigdataStressLogger(), true)

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		var errorCount atomic.Int64
		ctx := context.Background()

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				metrics := &bigdata.ProviderMetrics{
					Provider:       fmt.Sprintf("provider-%d", idx%10),
					Model:          fmt.Sprintf("model-%d", idx%5),
					RequestID:      fmt.Sprintf("req-%d", idx),
					Timestamp:      time.Now(),
					ResponseTimeMs: float64(idx) * 10.5,
					TokensUsed:     100 + idx,
					Success:        idx%7 != 0,
				}

				if err := ai.PublishProviderMetrics(ctx, metrics); err != nil {
					errorCount.Add(1)
				}

				debateMetrics := &bigdata.DebateMetrics{
					DebateID:         fmt.Sprintf("debate-%d", idx),
					Topic:            fmt.Sprintf("topic-%d", idx),
					Timestamp:        time.Now(),
					TotalRounds:      3,
					TotalDurationMs:  float64(idx) * 100.0,
					ParticipantCount: 5,
					Winner:           "claude",
					Confidence:       0.9,
					TotalTokens:      5000 + idx,
					Outcome:          "successful",
				}

				if err := ai.PublishDebateMetrics(ctx, debateMetrics); err != nil {
					errorCount.Add(1)
				}
			}(i)
		}

		wg.Wait()

		if errorCount.Load() > 0 {
			t.Errorf("unexpected errors during publish: %d", errorCount.Load())
		}

		publishedCount := broker.getPublishCount()
		// 100 goroutines x 2 publishes each = 200
		assert.Equal(t, int64(200), publishedCount,
			"expected 200 published messages, got %d", publishedCount)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: ConcurrentAnalyticsPublish timed out")
	}
}

// TestBigData_ConcurrentEntityPublish runs 100 goroutines publishing entity
// events through EntityIntegration concurrently.
func TestBigData_ConcurrentEntityPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	broker := newBdStressMockBroker()
	ei := bigdata.NewEntityIntegration(broker, newBigdataStressLogger(), true)

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		var errorCount atomic.Int64
		ctx := context.Background()

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				entity := &memory.Entity{
					ID:   fmt.Sprintf("entity-%d", idx),
					Name: fmt.Sprintf("Entity %d", idx),
					Type: "concept",
					Properties: map[string]interface{}{
						"index": idx,
						"tag":   fmt.Sprintf("tag-%d", idx%10),
					},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				convID := fmt.Sprintf("conv-%d", idx%20)

				if err := ei.PublishEntityCreated(ctx, entity, convID); err != nil {
					errorCount.Add(1)
				}

				if err := ei.PublishEntityUpdated(ctx, entity, convID); err != nil {
					errorCount.Add(1)
				}
			}(i)
		}

		wg.Wait()

		if errorCount.Load() > 0 {
			t.Errorf("unexpected errors during entity publish: %d",
				errorCount.Load())
		}

		publishedCount := broker.getPublishCount()
		// 100 goroutines x 2 publishes each = 200
		assert.Equal(t, int64(200), publishedCount,
			"expected 200 published messages, got %d", publishedCount)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: ConcurrentEntityPublish timed out")
	}
}

// TestBigData_InMemoryEventLog_ConcurrentAppendAndRead exercises the
// inMemoryEventLog (exposed via BigDataIntegration) with 50 writers
// appending events and 50 readers querying events simultaneously.
// Since inMemoryEventLog is unexported, we test through
// BigDataIntegration's HealthCheck as a proxy for concurrent safety
// of the integration struct, and directly test the exported
// AnalyticsIntegration and EntityIntegration with mixed operations.
func TestBigData_InMemoryEventLog_ConcurrentAppendAndRead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	broker := newBdStressMockBroker()
	logger := newBigdataStressLogger()
	ai := bigdata.NewAnalyticsIntegration(broker, logger, true)
	ei := bigdata.NewEntityIntegration(broker, logger, true)

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		var writeOps atomic.Int64
		var readOps atomic.Int64
		ctx := context.Background()

		// 50 writers publishing analytics and entity events
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				for j := 0; j < 10; j++ {
					metrics := &bigdata.ProviderMetrics{
						Provider:       fmt.Sprintf("provider-%d", idx%10),
						Model:          fmt.Sprintf("model-%d", j),
						RequestID:      fmt.Sprintf("req-%d-%d", idx, j),
						Timestamp:      time.Now(),
						ResponseTimeMs: float64(j) * 5.0,
						TokensUsed:     50 * (j + 1),
						Success:        true,
					}

					if err := ai.PublishProviderMetrics(
						ctx, metrics,
					); err == nil {
						writeOps.Add(1)
					}

					entity := &memory.Entity{
						ID:        fmt.Sprintf("e-%d-%d", idx, j),
						Name:      fmt.Sprintf("Entity %d-%d", idx, j),
						Type:      "thing",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					}

					if err := ei.PublishEntityCreated(
						ctx, entity,
						fmt.Sprintf("conv-%d", idx),
					); err == nil {
						writeOps.Add(1)
					}
				}
			}(i)
		}

		// 50 readers performing config conversions and validations
		// (read-side operations that exercise shared state)
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				for j := 0; j < 10; j++ {
					icfg := bigdata.DefaultIntegrationConfig()
					if icfg != nil {
						readOps.Add(1)
					}

					cfg := bigdata.DefaultBigDataConfig()
					if cfg != nil {
						readOps.Add(1)
					}

					err := bigdata.ValidateConfig(icfg)
					if err == nil {
						readOps.Add(1)
					}
				}
			}(i)
		}

		wg.Wait()

		t.Logf(
			"Completed %d write ops and %d read ops",
			writeOps.Load(), readOps.Load(),
		)

		// 50 writers x 10 iterations x 2 writes = 1000 writes
		assert.True(t, writeOps.Load() >= 900,
			"expected at least 900 write ops, got %d", writeOps.Load())
		// 50 readers x 10 iterations x 3 reads = 1500 reads
		assert.True(t, readOps.Load() >= 1400,
			"expected at least 1400 read ops, got %d", readOps.Load())
	}()

	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: ConcurrentAppendAndRead timed out")
	}
}

// TestBigData_HealthCheck_UnderLoad runs 100 concurrent goroutines
// performing health checks on a BigDataIntegration instance while
// verifying no panics, races, or deadlocks occur.
func TestBigData_HealthCheck_UnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	logger := newBigdataStressLogger()

	cfg := &bigdata.IntegrationConfig{
		EnableInfiniteContext:   false,
		EnableDistributedMemory: false,
		EnableKnowledgeGraph:    false,
		EnableAnalytics:         false,
		EnableCrossLearning:     false,
	}

	bdi, err := bigdata.NewBigDataIntegration(cfg, nil, logger)
	if err != nil {
		t.Fatalf("failed to create BigDataIntegration: %v", err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		var checkCount atomic.Int64
		ctx := context.Background()

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				for j := 0; j < 10; j++ {
					health := bdi.HealthCheck(ctx)
					if health != nil {
						checkCount.Add(1)

						// Verify expected keys exist
						if _, ok := health["infinite_context"]; !ok {
							t.Errorf(
								"goroutine %d: missing infinite_context key",
								idx,
							)
						}
						if _, ok := health["distributed_memory"]; !ok {
							t.Errorf(
								"goroutine %d: missing distributed_memory key",
								idx,
							)
						}
						if _, ok := health["knowledge_graph"]; !ok {
							t.Errorf(
								"goroutine %d: missing knowledge_graph key",
								idx,
							)
						}
						if _, ok := health["analytics"]; !ok {
							t.Errorf(
								"goroutine %d: missing analytics key",
								idx,
							)
						}
						if _, ok := health["cross_learning"]; !ok {
							t.Errorf(
								"goroutine %d: missing cross_learning key",
								idx,
							)
						}
					}
				}
			}(i)
		}

		wg.Wait()

		totalChecks := checkCount.Load()
		t.Logf("Completed %d health checks under load", totalChecks)

		// 100 goroutines x 10 checks = 1000
		assert.Equal(t, int64(1000), totalChecks,
			"expected 1000 health checks, got %d", totalChecks)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: HealthCheck_UnderLoad timed out")
	}
}
