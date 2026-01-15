package messaging_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/messaging/inmemory"
)

// BenchmarkInMemoryPublish benchmarks message publishing throughput
func BenchmarkInMemoryPublish(b *testing.B) {
	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	msg := &messaging.Message{
		ID:        "bench-msg",
		Type:      "benchmark",
		Payload:   []byte(`{"test": "data", "timestamp": 1234567890}`),
		Headers:   map[string]string{"correlation-id": "bench-001"},
		Timestamp: time.Now(),
		Priority:  1,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := broker.Publish(ctx, "benchmark.topic", msg); err != nil {
				b.Errorf("Publish failed: %v", err)
			}
		}
	})
}

// BenchmarkInMemoryBatchPublish benchmarks batch message publishing
func BenchmarkInMemoryBatchPublish(b *testing.B) {
	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	batchSizes := []int{10, 50, 100, 500}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			messages := make([]*messaging.Message, batchSize)
			for i := 0; i < batchSize; i++ {
				messages[i] = &messaging.Message{
					ID:        fmt.Sprintf("bench-msg-%d", i),
					Type:      "benchmark",
					Payload:   []byte(`{"test": "data"}`),
					Timestamp: time.Now(),
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := broker.PublishBatch(ctx, "benchmark.topic", messages); err != nil {
					b.Errorf("Batch publish failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkInMemoryPubSub benchmarks full publish-subscribe cycle
func BenchmarkInMemoryPubSub(b *testing.B) {
	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	var received int64
	handler := func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&received, 1)
		return nil
	}

	if _, err := broker.Subscribe(ctx, "benchmark.topic", handler); err != nil {
		b.Fatalf("Subscribe failed: %v", err)
	}

	msg := &messaging.Message{
		ID:        "bench-msg",
		Type:      "benchmark",
		Payload:   []byte(`{"test": "data"}`),
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := broker.Publish(ctx, "benchmark.topic", msg); err != nil {
			b.Errorf("Publish failed: %v", err)
		}
	}

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)
}

// BenchmarkMessageSerialization benchmarks JSON message serialization
func BenchmarkMessageSerialization(b *testing.B) {
	msg := &messaging.Message{
		ID:        "test-msg-123",
		Type:      "benchmark.event",
		Payload:   []byte(`{"user_id": 12345, "action": "test", "metadata": {"key": "value"}}`),
		Headers:   map[string]string{"correlation-id": "bench-001", "trace-id": "trace-123"},
		Timestamp: time.Now(),
		Priority:  5,
		TraceID:   "trace-123",
	}

	b.Run("Marshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(msg)
			if err != nil {
				b.Errorf("Marshal failed: %v", err)
			}
		}
	})

	data, _ := json.Marshal(msg)
	b.Run("Unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var m messaging.Message
			if err := json.Unmarshal(data, &m); err != nil {
				b.Errorf("Unmarshal failed: %v", err)
			}
		}
	})
}

// BenchmarkConcurrentPublishers benchmarks concurrent message publishing
func BenchmarkConcurrentPublishers(b *testing.B) {
	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	publisherCounts := []int{1, 4, 8, 16, 32}

	for _, count := range publisherCounts {
		b.Run(fmt.Sprintf("Publishers_%d", count), func(b *testing.B) {
			msg := &messaging.Message{
				ID:        "bench-msg",
				Type:      "benchmark",
				Payload:   []byte(`{"test": "data"}`),
				Timestamp: time.Now(),
			}

			messagesPerPublisher := b.N / count
			if messagesPerPublisher == 0 {
				messagesPerPublisher = 1
			}

			var wg sync.WaitGroup
			wg.Add(count)

			b.ResetTimer()
			for p := 0; p < count; p++ {
				go func() {
					defer wg.Done()
					for i := 0; i < messagesPerPublisher; i++ {
						broker.Publish(ctx, "benchmark.topic", msg)
					}
				}()
			}
			wg.Wait()
		})
	}
}

// BenchmarkMessageThroughput measures messages per second
func BenchmarkMessageThroughput(b *testing.B) {
	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	var receivedCount int64
	handler := func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&receivedCount, 1)
		return nil
	}

	if _, err := broker.Subscribe(ctx, "throughput.topic", handler); err != nil {
		b.Fatalf("Subscribe failed: %v", err)
	}

	msg := &messaging.Message{
		ID:        "throughput-test",
		Type:      "throughput.benchmark",
		Payload:   []byte(`{"test": "throughput"}`),
		Timestamp: time.Now(),
	}

	duration := 5 * time.Second
	deadline := time.Now().Add(duration)
	var publishCount int64

	b.ResetTimer()
	for time.Now().Before(deadline) {
		if err := broker.Publish(ctx, "throughput.topic", msg); err == nil {
			atomic.AddInt64(&publishCount, 1)
		}
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	published := atomic.LoadInt64(&publishCount)
	received := atomic.LoadInt64(&receivedCount)
	throughput := float64(published) / duration.Seconds()

	b.ReportMetric(throughput, "msgs/sec")
	b.ReportMetric(float64(received)/float64(published)*100, "delivery_%")
}

// BenchmarkLatency measures message delivery latency
func BenchmarkLatency(b *testing.B) {
	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex
	receiveCh := make(chan time.Time, b.N)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		receiveCh <- time.Now()
		return nil
	}

	if _, err := broker.Subscribe(ctx, "latency.topic", handler); err != nil {
		b.Fatalf("Subscribe failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := &messaging.Message{
			ID:        fmt.Sprintf("latency-test-%d", i),
			Type:      "latency.benchmark",
			Payload:   []byte(`{"test": "latency"}`),
			Timestamp: time.Now(),
		}

		sendTime := time.Now()
		if err := broker.Publish(ctx, "latency.topic", msg); err != nil {
			b.Errorf("Publish failed: %v", err)
			continue
		}

		select {
		case receiveTime := <-receiveCh:
			mu.Lock()
			latencies = append(latencies, receiveTime.Sub(sendTime))
			mu.Unlock()
		case <-time.After(1 * time.Second):
			b.Errorf("Message not received within timeout")
		}
	}

	if len(latencies) > 0 {
		var total time.Duration
		for _, lat := range latencies {
			total += lat
		}
		avgLatency := total / time.Duration(len(latencies))
		b.ReportMetric(float64(avgLatency.Microseconds()), "avg_latency_us")
	}
}

// BenchmarkLargePayload benchmarks handling of large messages
func BenchmarkLargePayload(b *testing.B) {
	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	payloadSizes := []int{1024, 10240, 102400, 1048576} // 1KB, 10KB, 100KB, 1MB

	for _, size := range payloadSizes {
		b.Run(fmt.Sprintf("Payload_%dKB", size/1024), func(b *testing.B) {
			payload := make([]byte, size)
			for i := 0; i < size; i++ {
				payload[i] = byte(i % 256)
			}

			msg := &messaging.Message{
				ID:        "large-payload-test",
				Type:      "payload.benchmark",
				Payload:   payload,
				Timestamp: time.Now(),
			}

			b.ResetTimer()
			b.SetBytes(int64(size))
			for i := 0; i < b.N; i++ {
				if err := broker.Publish(ctx, "payload.topic", msg); err != nil {
					b.Errorf("Publish failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkTopicRouting benchmarks message routing to multiple topics
func BenchmarkTopicRouting(b *testing.B) {
	broker := inmemory.NewBroker()
	ctx := context.Background()

	if err := broker.Connect(ctx); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer broker.Close(ctx)

	topicCounts := []int{1, 10, 50, 100}

	for _, count := range topicCounts {
		b.Run(fmt.Sprintf("Topics_%d", count), func(b *testing.B) {
			topics := make([]string, count)
			for i := 0; i < count; i++ {
				topics[i] = fmt.Sprintf("routing.topic.%d", i)
			}

			msg := &messaging.Message{
				ID:        "routing-test",
				Type:      "routing.benchmark",
				Payload:   []byte(`{"test": "routing"}`),
				Timestamp: time.Now(),
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				topic := topics[i%count]
				if err := broker.Publish(ctx, topic, msg); err != nil {
					b.Errorf("Publish failed: %v", err)
				}
			}
		})
	}
}
