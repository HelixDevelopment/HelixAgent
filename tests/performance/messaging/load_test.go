package messaging_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/messaging/inmemory"
)

// LoadTestConfig defines configuration for load tests
type LoadTestConfig struct {
	Duration          time.Duration
	Publishers        int
	Subscribers       int
	MessagesPerSecond int
	MessageSize       int
	Topics            int
}

// LoadTestResult contains load test metrics
type LoadTestResult struct {
	TotalPublished   int64
	TotalReceived    int64
	AvgLatency       time.Duration
	P50Latency       time.Duration
	P95Latency       time.Duration
	P99Latency       time.Duration
	MaxLatency       time.Duration
	MinLatency       time.Duration
	Throughput       float64
	DeliveryRate     float64
	ErrorCount       int64
	DroppedMessages  int64
	StartTime        time.Time
	EndTime          time.Time
}

// TestLoadHighThroughput tests high message throughput scenarios
func TestLoadHighThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	config := LoadTestConfig{
		Duration:          10 * time.Second,
		Publishers:        4,
		Subscribers:       2,
		MessagesPerSecond: 10000,
		MessageSize:       1024,
		Topics:            1,
	}

	result := runLoadTest(t, config)

	t.Logf("Load Test Results:")
	t.Logf("  Published: %d messages", result.TotalPublished)
	t.Logf("  Received: %d messages", result.TotalReceived)
	t.Logf("  Throughput: %.2f msgs/sec", result.Throughput)
	t.Logf("  Delivery Rate: %.2f%%", result.DeliveryRate)
	t.Logf("  Avg Latency: %v", result.AvgLatency)
	t.Logf("  P95 Latency: %v", result.P95Latency)
	t.Logf("  P99 Latency: %v", result.P99Latency)
	t.Logf("  Errors: %d", result.ErrorCount)

	// Assert minimum performance requirements
	if result.DeliveryRate < 99.0 {
		t.Errorf("Delivery rate below 99%%: %.2f%%", result.DeliveryRate)
	}

	if result.P99Latency > 100*time.Millisecond {
		t.Errorf("P99 latency exceeds 100ms: %v", result.P99Latency)
	}
}

// TestLoadMultiTopic tests message routing across multiple topics
func TestLoadMultiTopic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	config := LoadTestConfig{
		Duration:          10 * time.Second,
		Publishers:        8,
		Subscribers:       4,
		MessagesPerSecond: 5000,
		MessageSize:       512,
		Topics:            10,
	}

	result := runLoadTest(t, config)

	t.Logf("Multi-Topic Load Test Results:")
	t.Logf("  Published: %d messages", result.TotalPublished)
	t.Logf("  Received: %d messages", result.TotalReceived)
	t.Logf("  Throughput: %.2f msgs/sec", result.Throughput)
	t.Logf("  Topics: %d", config.Topics)
}

// TestLoadLargeMessages tests handling of large message payloads
func TestLoadLargeMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	config := LoadTestConfig{
		Duration:          10 * time.Second,
		Publishers:        2,
		Subscribers:       2,
		MessagesPerSecond: 100,
		MessageSize:       1048576, // 1MB
		Topics:            1,
	}

	result := runLoadTest(t, config)

	t.Logf("Large Message Load Test Results:")
	t.Logf("  Published: %d messages", result.TotalPublished)
	t.Logf("  Received: %d messages", result.TotalReceived)
	t.Logf("  Message Size: %d bytes", config.MessageSize)
	t.Logf("  Data Throughput: %.2f MB/sec", float64(result.TotalPublished*int64(config.MessageSize))/(1024*1024*config.Duration.Seconds()))
}

// TestLoadBurstTraffic tests handling of traffic bursts
func TestLoadBurstTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	var received int64
	handler := func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&received, 1)
		return nil
	}

	if _, err := broker.Subscribe(ctx, "burst.topic", handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Send burst of messages
	burstSize := 10000
	payload := make([]byte, 256)
	msg := &messaging.Message{
		ID:        "burst-test",
		Type:      "burst",
		Payload:   payload,
		Timestamp: time.Now(),
	}

	startTime := time.Now()
	for i := 0; i < burstSize; i++ {
		broker.Publish(ctx, "burst.topic", msg)
	}
	publishDuration := time.Since(startTime)

	// Wait for processing
	time.Sleep(2 * time.Second)

	receivedCount := atomic.LoadInt64(&received)
	deliveryRate := float64(receivedCount) / float64(burstSize) * 100

	t.Logf("Burst Traffic Test Results:")
	t.Logf("  Burst Size: %d messages", burstSize)
	t.Logf("  Publish Duration: %v", publishDuration)
	t.Logf("  Publish Rate: %.2f msgs/sec", float64(burstSize)/publishDuration.Seconds())
	t.Logf("  Delivered: %d (%.2f%%)", receivedCount, deliveryRate)

	if deliveryRate < 99.0 {
		t.Errorf("Delivery rate below 99%%: %.2f%%", deliveryRate)
	}
}

// TestLoadSustainedTraffic tests sustained traffic over extended period
func TestLoadSustainedTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	config := LoadTestConfig{
		Duration:          30 * time.Second,
		Publishers:        4,
		Subscribers:       4,
		MessagesPerSecond: 1000,
		MessageSize:       256,
		Topics:            4,
	}

	result := runLoadTest(t, config)

	t.Logf("Sustained Traffic Test Results:")
	t.Logf("  Duration: %v", config.Duration)
	t.Logf("  Published: %d messages", result.TotalPublished)
	t.Logf("  Received: %d messages", result.TotalReceived)
	t.Logf("  Throughput: %.2f msgs/sec", result.Throughput)
	t.Logf("  Delivery Rate: %.2f%%", result.DeliveryRate)

	if result.ErrorCount > 0 {
		t.Errorf("Errors during sustained traffic: %d", result.ErrorCount)
	}
}

// TestLoadGracefulDegradation tests system behavior under overload
func TestLoadGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	var received int64
	// Slow consumer to simulate backpressure
	handler := func(ctx context.Context, msg *messaging.Message) error {
		time.Sleep(1 * time.Millisecond)
		atomic.AddInt64(&received, 1)
		return nil
	}

	if _, err := broker.Subscribe(ctx, "overload.topic", handler); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Fast producer
	var published int64
	var errors int64
	duration := 5 * time.Second
	deadline := time.Now().Add(duration)

	msg := &messaging.Message{
		ID:        "overload-test",
		Type:      "overload",
		Payload:   []byte(`{"test": "overload"}`),
		Timestamp: time.Now(),
	}

	for time.Now().Before(deadline) {
		if err := broker.Publish(ctx, "overload.topic", msg); err != nil {
			atomic.AddInt64(&errors, 1)
		} else {
			atomic.AddInt64(&published, 1)
		}
	}

	// Wait for processing
	time.Sleep(5 * time.Second)

	publishedCount := atomic.LoadInt64(&published)
	receivedCount := atomic.LoadInt64(&received)
	errorCount := atomic.LoadInt64(&errors)

	t.Logf("Graceful Degradation Test Results:")
	t.Logf("  Published: %d messages", publishedCount)
	t.Logf("  Received: %d messages", receivedCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Backlog: %d messages", publishedCount-receivedCount)

	// System should not crash and should continue processing
	if receivedCount == 0 {
		t.Error("No messages received - system may have crashed")
	}
}

// runLoadTest executes a load test with given configuration
func runLoadTest(t *testing.T, config LoadTestConfig) LoadTestResult {
	broker := inmemory.NewBroker()
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration+10*time.Second)
	defer cancel()

	if err := broker.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	result := LoadTestResult{
		StartTime:  time.Now(),
		MinLatency: time.Hour, // Initialize to large value
	}

	var wg sync.WaitGroup
	var receivedCount int64
	var latencySum int64
	var latencies []time.Duration
	var latMu sync.Mutex

	// Set up subscribers
	for i := 0; i < config.Subscribers; i++ {
		for j := 0; j < config.Topics; j++ {
			topic := fmt.Sprintf("load.test.topic.%d", j)
			handler := func(ctx context.Context, msg *messaging.Message) error {
				atomic.AddInt64(&receivedCount, 1)
				// Calculate latency from message timestamp
				latency := time.Since(msg.Timestamp)
				atomic.AddInt64(&latencySum, int64(latency))
				latMu.Lock()
				latencies = append(latencies, latency)
				latMu.Unlock()
				return nil
			}
			if _, err := broker.Subscribe(ctx, topic, handler); err != nil {
				t.Fatalf("Subscribe failed: %v", err)
			}
		}
	}

	// Give subscribers time to register
	time.Sleep(100 * time.Millisecond)

	// Start publishers
	var publishedCount int64
	var errorCount int64

	payload := make([]byte, config.MessageSize)
	for i := 0; i < config.MessageSize; i++ {
		payload[i] = byte(i % 256)
	}

	deadline := time.Now().Add(config.Duration)
	interval := time.Duration(float64(time.Second) / float64(config.MessagesPerSecond) * float64(config.Publishers))

	for p := 0; p < config.Publishers; p++ {
		wg.Add(1)
		go func(publisherID int) {
			defer wg.Done()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if time.Now().After(deadline) {
						return
					}

					topic := fmt.Sprintf("load.test.topic.%d", atomic.LoadInt64(&publishedCount)%int64(config.Topics))
					msg := &messaging.Message{
						ID:        fmt.Sprintf("load-%d-%d", publisherID, atomic.LoadInt64(&publishedCount)),
						Type:      "load.test",
						Payload:   payload,
						Timestamp: time.Now(),
					}

					if err := broker.Publish(ctx, topic, msg); err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&publishedCount, 1)
					}
				}
			}
		}(p)
	}

	wg.Wait()
	result.EndTime = time.Now()

	// Wait for messages to be processed
	time.Sleep(2 * time.Second)

	result.TotalPublished = atomic.LoadInt64(&publishedCount)
	result.TotalReceived = atomic.LoadInt64(&receivedCount)
	result.ErrorCount = atomic.LoadInt64(&errorCount)

	if result.TotalPublished > 0 {
		result.Throughput = float64(result.TotalPublished) / config.Duration.Seconds()
		result.DeliveryRate = float64(result.TotalReceived) / float64(result.TotalPublished) * 100
	}

	if result.TotalReceived > 0 {
		result.AvgLatency = time.Duration(atomic.LoadInt64(&latencySum) / result.TotalReceived)

		// Calculate percentiles
		latMu.Lock()
		if len(latencies) > 0 {
			// Sort latencies (simple insertion sort for small datasets)
			for i := 1; i < len(latencies); i++ {
				for j := i; j > 0 && latencies[j] < latencies[j-1]; j-- {
					latencies[j], latencies[j-1] = latencies[j-1], latencies[j]
				}
			}

			result.MinLatency = latencies[0]
			result.MaxLatency = latencies[len(latencies)-1]
			result.P50Latency = latencies[len(latencies)*50/100]
			result.P95Latency = latencies[len(latencies)*95/100]
			result.P99Latency = latencies[len(latencies)*99/100]
		}
		latMu.Unlock()
	}

	return result
}
