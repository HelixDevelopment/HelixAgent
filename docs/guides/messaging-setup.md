# Messaging System Setup Guide

## Overview

HelixAgent supports multiple messaging backends for distributed task processing, event streaming, and inter-service communication:

- **In-Memory**: Default, no configuration needed
- **Redis Pub/Sub**: Lightweight messaging with Redis
- **Apache Kafka**: High-throughput event streaming
- **RabbitMQ**: Enterprise message queuing

## Quick Start

### In-Memory (Default)

No configuration needed. Suitable for:
- Development
- Single-instance deployments
- Low-volume workloads

### Redis Pub/Sub

```bash
# Environment variables
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=your-password
export MESSAGING_BACKEND=redis
```

### Kafka

```bash
# Environment variables
export KAFKA_BROKERS=localhost:9092
export KAFKA_TOPIC_PREFIX=helixagent
export MESSAGING_BACKEND=kafka
```

### RabbitMQ

```bash
# Environment variables
export RABBITMQ_URL=amqp://user:pass@localhost:5672/
export RABBITMQ_EXCHANGE=helixagent
export MESSAGING_BACKEND=rabbitmq
```

## Kafka Configuration

### Docker Compose Setup

```yaml
version: '3.8'
services:
  zookeeper:
    image: confluentinc/cp-zookeeper:7.5.0
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181

  kafka:
    image: confluentinc/cp-kafka:7.5.0
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1

  helixagent:
    image: helixagent:latest
    depends_on:
      - kafka
    environment:
      KAFKA_BROKERS: kafka:9092
      MESSAGING_BACKEND: kafka
```

### Topic Configuration

HelixAgent creates these topics automatically:

| Topic | Purpose | Partitions |
|-------|---------|------------|
| `helixagent.tasks` | Task queue | 4 |
| `helixagent.events` | Event streaming | 8 |
| `helixagent.completions` | LLM completions | 4 |
| `helixagent.debates` | Debate events | 2 |

### Producer Configuration

```go
import "dev.helix.agent/internal/messaging/kafka"

producer := kafka.NewProducer(&kafka.ProducerConfig{
    Brokers:     []string{"localhost:9092"},
    TopicPrefix: "helixagent",
    Async:       true,
    BatchSize:   100,
    LingerMs:    10,
})
```

### Consumer Configuration

```go
consumer := kafka.NewConsumer(&kafka.ConsumerConfig{
    Brokers:       []string{"localhost:9092"},
    GroupID:       "helixagent-worker-1",
    Topics:        []string{"helixagent.tasks"},
    AutoCommit:    true,
    CommitInterval: 5 * time.Second,
})
```

## RabbitMQ Configuration

### Docker Compose Setup

```yaml
version: '3.8'
services:
  rabbitmq:
    image: rabbitmq:3.12-management
    ports:
      - "5672:5672"
      - "15672:15672"  # Management UI
    environment:
      RABBITMQ_DEFAULT_USER: helixagent
      RABBITMQ_DEFAULT_PASS: password

  helixagent:
    image: helixagent:latest
    depends_on:
      - rabbitmq
    environment:
      RABBITMQ_URL: amqp://helixagent:password@rabbitmq:5672/
      MESSAGING_BACKEND: rabbitmq
```

### Exchange Configuration

HelixAgent uses a topic exchange:

```go
import "dev.helix.agent/internal/messaging/rabbitmq"

broker := rabbitmq.NewBroker(&rabbitmq.Config{
    URL:          "amqp://localhost:5672/",
    Exchange:     "helixagent",
    ExchangeType: "topic",
    Durable:      true,
})
```

### Queue Binding

```go
// Bind queue to routing keys
broker.BindQueue("worker-queue", []string{
    "tasks.*",
    "completions.#",
})
```

## Message Formats

### Task Message

```json
{
  "id": "task-123",
  "type": "completion",
  "priority": 1,
  "payload": {
    "model": "helix-ensemble",
    "messages": [...]
  },
  "created_at": "2026-01-16T10:00:00Z"
}
```

### Event Message

```json
{
  "id": "event-456",
  "type": "debate.round.completed",
  "source": "debate-service",
  "data": {
    "debate_id": "debate-789",
    "round": 2,
    "responses": 5
  },
  "timestamp": "2026-01-16T10:05:00Z"
}
```

## Monitoring

### Kafka Metrics

Access Kafka metrics via Prometheus:

```bash
curl http://localhost:8080/metrics | grep kafka
```

Available metrics:
- `helixagent_kafka_messages_sent_total`
- `helixagent_kafka_messages_received_total`
- `helixagent_kafka_consumer_lag`
- `helixagent_kafka_producer_errors`

### RabbitMQ Metrics

Access RabbitMQ management UI at `http://localhost:15672`

Or via API:
```bash
curl -u helixagent:password http://localhost:15672/api/queues
```

## High Availability

### Kafka HA

```yaml
# Multiple brokers for HA
KAFKA_BROKERS: kafka1:9092,kafka2:9092,kafka3:9092

# Replication settings
KAFKA_REPLICATION_FACTOR: 3
KAFKA_MIN_INSYNC_REPLICAS: 2
```

### RabbitMQ HA

```yaml
# Cluster configuration
RABBITMQ_CLUSTER_NODES: rabbit@node1,rabbit@node2,rabbit@node3
RABBITMQ_HA_MODE: all
```

## Troubleshooting

### Connection Refused

```bash
# Check if service is running
docker ps | grep kafka
docker ps | grep rabbitmq

# Check logs
docker logs kafka
docker logs rabbitmq
```

### Consumer Lag

```bash
# Kafka: Check consumer lag
kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --group helixagent-worker --describe

# Add more consumers or increase partitions
```

### Message Loss

1. Enable durable queues/topics
2. Configure proper acknowledgments
3. Set up dead letter queues

## Related Documentation

- [Architecture](/docs/architecture/messaging-architecture.md)
- [Kafka Integration](/docs/architecture/kafka-integration.md)
- [RabbitMQ Integration](/docs/architecture/rabbitmq-integration.md)
