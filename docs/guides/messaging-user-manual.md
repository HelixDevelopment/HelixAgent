# HelixAgent Messaging System User Manual

## Table of Contents

1. [Introduction](#introduction)
2. [Architecture Overview](#architecture-overview)
3. [Getting Started](#getting-started)
4. [RabbitMQ Task Queue](#rabbitmq-task-queue)
5. [Kafka Event Streaming](#kafka-event-streaming)
6. [Dead Letter Queue (DLQ) Processing](#dead-letter-queue-dlq-processing)
7. [Message Replay](#message-replay)
8. [Monitoring and Metrics](#monitoring-and-metrics)
9. [Production Deployment](#production-deployment)
10. [Troubleshooting](#troubleshooting)
11. [API Reference](#api-reference)

---

## Introduction

The HelixAgent Messaging System provides a hybrid messaging architecture combining:

- **RabbitMQ** for low-latency task queuing and job orchestration
- **Apache Kafka** for high-throughput event streaming and audit logs

This dual-broker approach ensures optimal performance for different workloads while maintaining reliability and scalability.

### Key Features

- Unified messaging abstraction layer
- Automatic failover and circuit breakers
- Dead letter queue processing with retry logic
- Message replay from Kafka for debugging and recovery
- Comprehensive monitoring via Prometheus/Grafana
- Production-ready Kubernetes deployments

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        HelixAgent Application                       │
├─────────────────────────────────────────────────────────────────────┤
│                        Messaging Hub                                 │
│  ┌─────────────────┐     ┌─────────────────┐     ┌───────────────┐ │
│  │  Task Publisher  │     │ Event Publisher │     │   In-Memory   │ │
│  │   (RabbitMQ)     │     │    (Kafka)      │     │   Fallback    │ │
│  └────────┬────────┘     └────────┬────────┘     └───────────────┘ │
└───────────┼──────────────────────┼───────────────────────────────────┘
            │                      │
            ▼                      ▼
┌───────────────────┐    ┌───────────────────────┐
│     RabbitMQ      │    │      Apache Kafka     │
│   ┌───────────┐   │    │   ┌───────────────┐   │
│   │   Queue   │   │    │   │    Topic      │   │
│   │ (Tasks)   │   │    │   │  (Events)     │   │
│   └───────────┘   │    │   └───────────────┘   │
│   ┌───────────┐   │    │   ┌───────────────┐   │
│   │    DLQ    │   │    │   │  Consumer     │   │
│   │           │   │    │   │   Groups      │   │
│   └───────────┘   │    │   └───────────────┘   │
└───────────────────┘    └───────────────────────┘
```

### When to Use Each Broker

| Use Case | Recommended Broker | Reason |
|----------|-------------------|--------|
| Background tasks | RabbitMQ | Low latency, acknowledgments |
| LLM request queuing | RabbitMQ | Priority queues, TTL support |
| Event streaming | Kafka | High throughput, retention |
| Audit logs | Kafka | Immutable log, replay capability |
| Real-time notifications | RabbitMQ | Fanout exchanges |
| Analytics pipeline | Kafka | Stream processing |

---

## Getting Started

### Prerequisites

- Docker and Docker Compose (or Podman)
- Go 1.24+
- At least 8GB RAM for full messaging stack

### Quick Start

1. **Start the messaging infrastructure:**

```bash
# Create network if not exists
docker network create helixagent-network 2>/dev/null || true

# Start messaging services
docker-compose -f docker-compose.yml -f docker-compose.messaging.yml --profile messaging up -d
```

2. **Verify services are running:**

```bash
# Check RabbitMQ
curl http://localhost:15672/api/health/checks/alarms -u helixagent:helixagent123

# Check Kafka
docker exec helixagent-kafka kafka-topics --bootstrap-server localhost:9092 --list
```

3. **Initialize topics (automatic with kafka-init container):**

```bash
docker logs helixagent-kafka-init
```

### Configuration

Set these environment variables in your `.env` file:

```bash
# RabbitMQ Configuration
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=helixagent
RABBITMQ_PASSWORD=helixagent123
RABBITMQ_VHOST=/

# Kafka Configuration
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=helixagent-workers

# Messaging Hub
MESSAGING_ENABLED=true
MESSAGING_FALLBACK_ENABLED=true
```

---

## RabbitMQ Task Queue

### Queue Topology

HelixAgent uses these RabbitMQ queues:

| Queue | Purpose | Priority | TTL |
|-------|---------|----------|-----|
| `helixagent.tasks.background` | Background processing | 10 | 24h |
| `helixagent.tasks.llm` | LLM API requests | 10 | 5min |
| `helixagent.tasks.debate` | AI debate rounds | 10 | 10min |
| `helixagent.tasks.verification` | Provider verification | 5 | 5min |
| `helixagent.tasks.notifications` | Notification delivery | 5 | 1min |
| `helixagent.dlq` | Dead letter queue | - | 30 days |

### Publishing Tasks

```go
import "dev.helix.agent/internal/messaging"

// Create a task
task := &messaging.Task{
    ID:       uuid.New().String(),
    Type:     "llm.request",
    Payload:  payload,
    Priority: 5,
    Deadline: time.Now().Add(5 * time.Minute),
}

// Publish to RabbitMQ queue
err := hub.PublishTask(ctx, "helixagent.tasks.llm", task)
```

### Consuming Tasks

```go
// Register task handler
hub.SubscribeTasks(ctx, "helixagent.tasks.llm", func(ctx context.Context, task *messaging.Task) error {
    // Process the task
    result, err := processLLMRequest(ctx, task.Payload)
    if err != nil {
        return err // Task will be retried or sent to DLQ
    }
    return nil // Task acknowledged
})
```

### Priority Queues

Tasks support priorities from 0-10 (higher = more urgent):

```go
task := &messaging.Task{
    Priority: 10, // Highest priority
}
```

---

## Kafka Event Streaming

### Topic Topology

| Topic | Partitions | Purpose | Retention |
|-------|------------|---------|-----------|
| `helixagent.events.llm.responses` | 6 | LLM responses | 7 days |
| `helixagent.events.debate.rounds` | 6 | Debate round events | 7 days |
| `helixagent.events.verification.results` | 3 | Verification results | 30 days |
| `helixagent.events.provider.health` | 3 | Provider health checks | 1 day |
| `helixagent.events.audit` | 6 | Audit log | 90 days |
| `helixagent.events.metrics` | 3 | System metrics | 1 day |
| `helixagent.events.errors` | 3 | Error events | 30 days |
| `helixagent.stream.tokens` | 12 | Token streaming | 1 hour |
| `helixagent.dlq.tasks` | 3 | Task DLQ | 30 days |
| `helixagent.dlq.events` | 3 | Event DLQ | 30 days |

### Publishing Events

```go
import "dev.helix.agent/internal/messaging"

// Create an event
event := &messaging.Event{
    ID:        uuid.New().String(),
    Type:      "verification.completed",
    Source:    "verifier",
    Subject:   providerID,
    Data:      resultJSON,
    Timestamp: time.Now(),
}

// Publish to Kafka topic
err := hub.PublishEvent(ctx, "helixagent.events.verification.results", event)
```

### Consumer Groups

Events are consumed by consumer groups for parallel processing:

```go
// Subscribe with consumer group
hub.SubscribeEvents(ctx, "helixagent.events.verification.results", func(ctx context.Context, event *messaging.Event) error {
    // Process the event
    return processVerificationResult(ctx, event)
}, messaging.WithConsumerGroup("analytics-workers"))
```

---

## Dead Letter Queue (DLQ) Processing

### How DLQ Works

1. Messages that fail processing are sent to the DLQ
2. The DLQ processor periodically attempts to reprocess messages
3. Messages exceeding max retries are discarded or archived

### DLQ Configuration

```go
import "dev.helix.agent/internal/messaging/dlq"

config := dlq.ProcessorConfig{
    MaxRetries:             3,           // Max retry attempts
    RetryDelay:             1 * time.Second,
    RetryBackoffMultiplier: 2.0,         // Exponential backoff
    MaxRetryDelay:          30 * time.Second,
    ProcessingTimeout:      30 * time.Second,
    BatchSize:              10,
    PollInterval:           5 * time.Second,
}

processor := dlq.NewProcessor(broker, config, logger)
```

### Starting the DLQ Processor

```go
// Start processing
err := processor.Start(ctx)
if err != nil {
    log.Fatal(err)
}

// Stop gracefully
defer processor.Stop()
```

### Custom Retry Handlers

```go
// Register handler for specific message type
processor.RegisterHandler("llm.request", func(ctx context.Context, msg *dlq.DeadLetterMessage) error {
    // Custom retry logic for LLM requests
    return retryLLMRequest(ctx, msg.OriginalMessage)
})
```

### Manual DLQ Operations

```go
// Manually reprocess a message
processor.ReprocessMessage(ctx, "message-id")

// Discard a message
processor.DiscardMessage(ctx, "message-id", "invalid payload")

// List DLQ messages
messages, _ := processor.ListMessages(ctx, 100, 0)
```

### DLQ Metrics

```go
metrics := processor.GetMetrics()
fmt.Printf("Processed: %d, Retried: %d, Discarded: %d\n",
    metrics.MessagesProcessed,
    metrics.MessagesRetried,
    metrics.MessagesDiscarded)
```

---

## Message Replay

### When to Use Replay

- Debugging issues with specific messages
- Recovering from consumer failures
- Testing with historical data
- Backfilling after system updates

### Starting a Replay

```go
import "dev.helix.agent/internal/messaging/replay"

handler := replay.NewHandler(broker, replay.DefaultReplayConfig(), logger)

request := &replay.ReplayRequest{
    ID:          "replay-001",
    Topic:       "helixagent.events.llm.responses",
    FromTime:    time.Now().Add(-24 * time.Hour),
    ToTime:      time.Now(),
    TargetTopic: "helixagent.events.llm.responses.replay",
    Filter: &replay.ReplayFilter{
        MessageTypes: []string{"llm.response"},
        Headers:      map[string]string{"provider": "gemini"},
    },
    Options: &replay.ReplayOptions{
        BatchSize:      100,
        DelayBetween:   10 * time.Millisecond,
        DryRun:         false,
        SkipDuplicates: true,
    },
}

progress, err := handler.StartReplay(ctx, request)
```

### Monitoring Replay Progress

```go
// Get progress
progress, err := handler.GetProgress("replay-001")
fmt.Printf("Status: %s, Replayed: %d/%d (%.2f%%)\n",
    progress.Status,
    progress.ReplayedCount,
    progress.TotalMessages,
    float64(progress.ReplayedCount)/float64(progress.TotalMessages)*100)
```

### Canceling a Replay

```go
err := handler.CancelReplay("replay-001")
```

### Replay API Endpoints

```bash
# Start replay
POST /v1/messaging/replay
{
    "topic": "helixagent.events.audit",
    "from_time": "2024-01-01T00:00:00Z",
    "to_time": "2024-01-02T00:00:00Z",
    "options": {
        "dry_run": true
    }
}

# Get replay status
GET /v1/messaging/replay/{replay_id}

# Cancel replay
DELETE /v1/messaging/replay/{replay_id}

# List all replays
GET /v1/messaging/replay
```

---

## Monitoring and Metrics

### Prometheus Metrics

The messaging system exposes these Prometheus metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `messaging_messages_published_total` | Counter | Total messages published |
| `messaging_messages_consumed_total` | Counter | Total messages consumed |
| `messaging_queue_depth` | Gauge | Current queue depth |
| `messaging_delivery_latency_seconds` | Histogram | Message delivery latency |
| `messaging_dlq_messages_total` | Gauge | Messages in DLQ |
| `messaging_dlq_reprocessed_total` | Counter | DLQ messages reprocessed |
| `messaging_replay_active` | Gauge | Active replay operations |

### Grafana Dashboard

Import the messaging dashboard:

```bash
# Dashboard location
monitoring/dashboards/messaging-dashboard.json
```

Features:
- Real-time message throughput
- Queue depth monitoring
- Consumer lag tracking
- DLQ status
- Latency percentiles

### Alerting Rules

Configure alerts in Prometheus:

```yaml
groups:
  - name: messaging
    rules:
      - alert: HighQueueDepth
        expr: messaging_queue_depth > 10000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Queue depth is high"

      - alert: DLQAccumulation
        expr: messaging_dlq_messages_total > 100
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "DLQ is accumulating messages"
```

---

## Production Deployment

### Kubernetes Deployment

Deploy the messaging infrastructure to Kubernetes:

```bash
# Install Strimzi Kafka Operator
kubectl apply -f https://strimzi.io/install/latest?namespace=helixagent-messaging

# Deploy messaging infrastructure
kubectl apply -f deployments/kubernetes/messaging/
```

### Security Configuration

1. **TLS/SSL:**
```bash
# Generate certificates
./scripts/generate-certs.sh

# Mount certificates
volumes:
  - name: ssl-certs
    secret:
      secretName: messaging-tls
```

2. **Authentication:**
```yaml
# RabbitMQ
RABBITMQ_DEFAULT_USER: your-secure-user
RABBITMQ_DEFAULT_PASS: your-secure-password

# Kafka SASL
KAFKA_SASL_ENABLED_MECHANISMS: SCRAM-SHA-512
```

3. **Authorization:**
```yaml
# Kafka ACLs
kafka-acls --add --allow-principal User:helixagent \
  --operation Read --operation Write \
  --topic 'helixagent.*'
```

### High Availability

For production HA:

- RabbitMQ: 3-node cluster with mirrored queues
- Kafka: 3+ brokers with replication factor 3
- Zookeeper: 3-node ensemble

```yaml
# docker-compose.production.yml
services:
  kafka:
    deploy:
      replicas: 3
    environment:
      KAFKA_DEFAULT_REPLICATION_FACTOR: 3
      KAFKA_MIN_INSYNC_REPLICAS: 2
```

---

## Troubleshooting

### Common Issues

#### 1. Connection Refused

```
Error: dial tcp 127.0.0.1:5672: connect: connection refused
```

**Solution:** Ensure RabbitMQ is running:
```bash
docker-compose ps rabbitmq
docker-compose logs rabbitmq
```

#### 2. Consumer Lag Growing

**Check consumer lag:**
```bash
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe --group helixagent-workers
```

**Solutions:**
- Increase consumer instances
- Check for slow consumers
- Verify network connectivity

#### 3. Messages in DLQ

**View DLQ messages:**
```bash
# RabbitMQ
rabbitmqadmin get queue=helixagent.dlq count=10

# Check failure reasons
GET /v1/messaging/dlq?limit=10
```

#### 4. Kafka Consumer Rebalancing

**Symptoms:** Frequent rebalancing, duplicate processing

**Solutions:**
```yaml
# Increase session timeout
KAFKA_SESSION_TIMEOUT_MS: 30000
KAFKA_HEARTBEAT_INTERVAL_MS: 10000
```

### Debug Mode

Enable debug logging:

```bash
export LOG_LEVEL=debug
export MESSAGING_DEBUG=true
```

### Health Checks

```bash
# Check all services
curl http://localhost:7061/health

# RabbitMQ health
curl http://localhost:15672/api/health/checks/alarms -u helixagent:helixagent123

# Kafka broker health
kafka-broker-api-versions --bootstrap-server localhost:9092
```

---

## API Reference

### Messaging Hub

```go
// Initialize hub
hub := messaging.NewHub(config, logger)
err := hub.Initialize(ctx)

// Publish task (RabbitMQ)
err := hub.PublishTask(ctx, queue, task)

// Publish event (Kafka)
err := hub.PublishEvent(ctx, topic, event)

// Subscribe to tasks
hub.SubscribeTasks(ctx, queue, handler)

// Subscribe to events
hub.SubscribeEvents(ctx, topic, handler)

// Health check
err := hub.HealthCheck(ctx)

// Get metrics
metrics := hub.GetMetrics()
```

### HTTP Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/messaging/health` | GET | Health status |
| `/v1/messaging/metrics` | GET | Prometheus metrics |
| `/v1/messaging/dlq` | GET | List DLQ messages |
| `/v1/messaging/dlq/{id}/reprocess` | POST | Reprocess DLQ message |
| `/v1/messaging/dlq/{id}/discard` | DELETE | Discard DLQ message |
| `/v1/messaging/replay` | POST | Start replay |
| `/v1/messaging/replay/{id}` | GET | Get replay status |
| `/v1/messaging/replay/{id}` | DELETE | Cancel replay |
| `/v1/messaging/replay` | GET | List all replays |

---

## Additional Resources

- [Messaging Architecture Guide](./messaging-architecture.md)
- [RabbitMQ Best Practices](./rabbitmq-integration.md)
- [Kafka Configuration Guide](./kafka-integration.md)
- [Migration Guide](./migration-guide.md)
- [Performance Tuning](./performance-tuning.md)
