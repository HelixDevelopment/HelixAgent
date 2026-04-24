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

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/memory"
	"dev.helix.agent/internal/messaging"
)

// --- Mock implementations for stress tests ---

// stressMockBroker implements messaging.MessageBroker for stress testing.
type stressMockBroker struct {
	mu        sync.Mutex
	published int64
}

func (b *stressMockBroker) Connect(_ context.Context) error  { return nil }
func (b *stressMockBroker) Close(_ context.Context) error    { return nil }
func (b *stressMockBroker) IsConnected() bool                { return true }
func (b *stressMockBroker) BrokerType() messaging.BrokerType { return messaging.BrokerTypeInMemory }
func (b *stressMockBroker) HealthCheck(_ context.Context) error {
	return nil
}
func (b *stressMockBroker) GetMetrics() *messaging.BrokerMetrics {
	return &messaging.BrokerMetrics{}
}

func (b *stressMockBroker) Publish(
	_ context.Context, _ string, _ *messaging.Message, _ ...messaging.PublishOption,
) error {
	b.mu.Lock()
	b.published++
	b.mu.Unlock()
	return nil
}

func (b *stressMockBroker) PublishBatch(
	_ context.Context, _ string, msgs []*messaging.Message, _ ...messaging.PublishOption,
) error {
	b.mu.Lock()
	b.published += int64(len(msgs))
	b.mu.Unlock()
	return nil
}

func (b *stressMockBroker) Subscribe(
	_ context.Context, _ string, _ messaging.MessageHandler, _ ...messaging.SubscribeOption,
) (messaging.Subscription, error) {
	return &stressMockSubscription{}, nil
}

// stressMockSubscription implements messaging.Subscription.
type stressMockSubscription struct{}

func (s *stressMockSubscription) Unsubscribe() error { return nil }
func (s *stressMockSubscription) IsActive() bool     { return true }
func (s *stressMockSubscription) Topic() string      { return "test" }
func (s *stressMockSubscription) ID() string         { return "stress-sub" }

// stressMockEventLog implements memory.EventLog for stress testing.
type stressMockEventLog struct {
	mu     sync.Mutex
	events []*memory.MemoryEvent
}

func (l *stressMockEventLog) Append(event *memory.MemoryEvent) error {
	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()
	return nil
}

func (l *stressMockEventLog) GetEvents(_ string) ([]*memory.MemoryEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]*memory.MemoryEvent, len(l.events))
	copy(result, l.events)
	return result, nil
}

func (l *stressMockEventLog) GetEventsSince(_ time.Time) ([]*memory.MemoryEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]*memory.MemoryEvent, len(l.events))
	copy(result, l.events)
	return result, nil
}

func (l *stressMockEventLog) GetEventsForUser(_ string) ([]*memory.MemoryEvent, error) {
	return nil, nil
}

func (l *stressMockEventLog) GetEventsFromNode(_ string) ([]*memory.MemoryEvent, error) {
	return nil, nil
}

// --- Stress Tests ---

// TestMemory_ConcurrentCRDTOperations tests that CRDTResolver handles concurrent
// DetectConflict and Merge calls without panics or data corruption.
// 100 goroutines simultaneously perform conflict detection and resolution.
func TestMemory_ConcurrentCRDTOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	resolver := memory.NewCRDTResolver(memory.ConflictStrategyLastWriteWins)

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				local := &memory.Memory{
					ID:         fmt.Sprintf("mem-%d", idx),
					UserID:     "user1",
					Content:    fmt.Sprintf("local content %d", idx),
					Importance: float64(idx) / 100.0,
					UpdatedAt:  time.Now().Add(-time.Duration(idx) * time.Second),
					Metadata: map[string]interface{}{
						"tags": []string{fmt.Sprintf("tag-%d", idx)},
					},
				}

				remote := memory.NewMemoryEvent(
					memory.MemoryEventUpdated,
					fmt.Sprintf("node-%d", idx%5),
					local.ID,
					"user1",
				)
				remote.Content = fmt.Sprintf("remote content %d", idx)
				remote.Importance = float64(idx+1) / 100.0
				remote.Tags = []string{fmt.Sprintf("remote-tag-%d", idx)}

				// Perform DetectConflict
				for j := 0; j < 50; j++ {
					resolver.DetectConflict(local, remote)
				}

				// Perform Merge
				for j := 0; j < 50; j++ {
					result := resolver.Merge(local, remote)
					if result == nil {
						t.Error("Merge returned nil")
						return
					}
				}

				// Perform ResolveWithReport
				for j := 0; j < 20; j++ {
					report := resolver.ResolveWithReport(local, remote)
					if report == nil {
						t.Error("ResolveWithReport returned nil")
						return
					}
				}
			}(i)
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// Success - no deadlock or panic
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: CRDT concurrent operations timed out")
	}
}

// TestMemory_ConcurrentVectorClockUpdates tests that VectorClock operations
// do not cause panics when each goroutine operates on its own clocks.
// 100 goroutines performing Increment, Update, HappensBefore, and String.
func TestMemory_ConcurrentVectorClockUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				nodeID := fmt.Sprintf("node-%d", idx)

				for j := 0; j < 100; j++ {
					// Each goroutine works with its own clocks to avoid
					// data races on the underlying map
					vc1 := memory.NewVectorClock()
					vc2 := memory.NewVectorClock()

					// Increment
					vc1.Increment(nodeID)
					vc1.Increment(nodeID)
					vc2.Increment(fmt.Sprintf("other-node-%d", idx))

					// Update (merge)
					vc1.Update(vc2)

					// Compare
					_ = vc1.HappensBefore(vc2)
					_ = vc2.HappensBefore(vc1)
					_ = vc1.Concurrent(vc2)

					// String serialization
					s := vc1.String()
					if s == "" {
						t.Error("VectorClock.String() returned empty")
						return
					}

					// Parse back
					parsed, err := memory.ParseVectorClock(s)
					if err != nil {
						t.Errorf("ParseVectorClock failed: %v", err)
						return
					}
					if parsed == nil {
						t.Error("ParseVectorClock returned nil")
						return
					}
				}
			}(i)
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: VectorClock concurrent operations timed out")
	}
}

// TestMemory_ConcurrentEventStreamAppend tests that EventStream's CalculateStats
// operates correctly when event streams are built concurrently per goroutine.
// 50 goroutines each build their own EventStream with multiple events.
func TestMemory_ConcurrentEventStreamAppend(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		var totalEvents int64

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				stream := &memory.EventStream{
					StreamID:  fmt.Sprintf("stream-%d", idx),
					UserID:    fmt.Sprintf("user-%d", idx),
					StartTime: time.Now(),
					Events:    make([]*memory.MemoryEvent, 0, 100),
				}

				// Each goroutine appends events to its own stream
				for j := 0; j < 100; j++ {
					event := memory.NewMemoryEvent(
						memory.MemoryEventCreated,
						fmt.Sprintf("node-%d", idx%5),
						fmt.Sprintf("mem-%d-%d", idx, j),
						stream.UserID,
					)
					event.Content = fmt.Sprintf("content %d-%d", idx, j)
					event.Importance = float64(j) / 100.0

					stream.Events = append(stream.Events, event)
					atomic.AddInt64(&totalEvents, 1)
				}

				stream.EndTime = time.Now()
				stream.EventCount = len(stream.Events)

				// Calculate stats
				stats := stream.CalculateStats()
				if stats == nil {
					t.Error("CalculateStats returned nil")
					return
				}
				if stats.TotalEvents != 100 {
					t.Errorf(
						"Expected 100 events in stats, got %d",
						stats.TotalEvents,
					)
					return
				}
				if stats.MemoriesAffected < 1 {
					t.Error("Expected at least 1 memory affected")
					return
				}
			}(i)
		}
		wg.Wait()

		finalTotal := atomic.LoadInt64(&totalEvents)
		if finalTotal != 5000 {
			t.Errorf("Expected 5000 total events, got %d", finalTotal)
		}
	}()

	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: EventStream concurrent operations timed out")
	}
}

// TestMemory_DistributedManager_ConcurrentAdd tests that DistributedMemoryManager
// handles concurrent AddMemory calls safely with proper locking.
// 50 goroutines adding memories concurrently via DistributedMemoryManager.
func TestMemory_DistributedManager_ConcurrentAdd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	store := memory.NewInMemoryStore()
	localManager := memory.NewManager(store, nil, nil, nil, nil, nil)
	broker := &stressMockBroker{}
	eventLog := &stressMockEventLog{events: make([]*memory.MemoryEvent, 0)}
	resolver := memory.NewCRDTResolver(memory.ConflictStrategyLastWriteWins)

	dmm := memory.NewDistributedMemoryManager(
		localManager, "stress-node", eventLog, resolver, broker, nil,
	)

	done := make(chan struct{})
	ctx := context.Background()

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				for j := 0; j < 20; j++ {
					mem := &memory.Memory{
						ID:         fmt.Sprintf("mem-%d-%d", idx, j),
						UserID:     fmt.Sprintf("user-%d", idx%10),
						Content:    fmt.Sprintf("content from goroutine %d iteration %d", idx, j),
						Importance: float64(j) / 20.0,
						Metadata:   map[string]interface{}{},
					}

					err := dmm.AddMemory(ctx, mem)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}(i)
		}
		wg.Wait()

		successes := atomic.LoadInt64(&successCount)
		errors := atomic.LoadInt64(&errorCount)

		t.Logf(
			"Concurrent AddMemory: %d successes, %d errors out of %d total",
			successes, errors, 50*20,
		)

		// All operations should succeed since we use InMemoryStore
		assert.Equal(t, int64(0), errors, "Expected no errors during concurrent add")
		assert.Equal(t, int64(1000), successes, "Expected 1000 successful adds")

		// Verify vector clock was incremented correctly
		vc := dmm.GetVectorClock()
		assert.Equal(t, int64(1000), vc["stress-node"],
			"Vector clock should reflect all 1000 operations")

		// Verify broker received all messages
		broker.mu.Lock()
		publishedCount := broker.published
		broker.mu.Unlock()
		assert.Equal(t, int64(1000), publishedCount,
			"Broker should have received 1000 published messages")

		// Verify event log received all events
		eventLog.mu.Lock()
		eventCount := len(eventLog.events)
		eventLog.mu.Unlock()
		assert.Equal(t, 1000, eventCount,
			"Event log should contain 1000 events")
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(120 * time.Second):
		t.Fatal("DEADLOCK DETECTED: DistributedMemoryManager concurrent add timed out")
	}
}
