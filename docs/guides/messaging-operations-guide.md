# HelixAgent Messaging Operations Guide

This guide covers day-to-day operations, maintenance, and troubleshooting for the HelixAgent messaging infrastructure.

## Table of Contents

1. [Daily Operations](#daily-operations)
2. [Monitoring Dashboard](#monitoring-dashboard)
3. [Scaling Operations](#scaling-operations)
4. [Backup and Recovery](#backup-and-recovery)
5. [Maintenance Procedures](#maintenance-procedures)
6. [Incident Response](#incident-response)
7. [Performance Tuning](#performance-tuning)

---

## Daily Operations

### Morning Health Check

Run this daily to verify messaging system health:

```bash
#!/bin/bash
# morning-health-check.sh

echo "=== HelixAgent Messaging Health Check ==="
echo "Date: $(date)"
echo ""

# Check RabbitMQ
echo "RabbitMQ Status:"
curl -s http://localhost:15672/api/health/checks/alarms -u helixagent:helixagent123 | jq .
echo ""

# Check Kafka
echo "Kafka Topics:"
docker exec helixagent-kafka kafka-topics --bootstrap-server localhost:9092 --list | wc -l
echo "topics configured"
echo ""

# Check consumer lag
echo "Consumer Lag:"
docker exec helixagent-kafka kafka-consumer-groups \
  --bootstrap-server localhost:9092 \
  --describe --group helixagent-workers 2>/dev/null | head -20
echo ""

# Check DLQ
echo "DLQ Status:"
curl -s http://localhost:7061/v1/messaging/dlq | jq '.count'
echo "messages in DLQ"
echo ""

# Check queue depths
echo "Queue Depths:"
curl -s http://localhost:15672/api/queues -u helixagent:helixagent123 | \
  jq '.[] | select(.name | startswith("helixagent")) | {name, messages}'

echo "=== Health Check Complete ==="
```

### Queue Monitoring Commands

```bash
# List all queues with message counts
rabbitmqadmin -u helixagent -p helixagent123 list queues name messages

# Check specific queue
rabbitmqadmin -u helixagent -p helixagent123 get queue=helixagent.tasks.llm count=5

# Purge a queue (CAUTION: deletes all messages)
rabbitmqadmin -u helixagent -p helixagent123 purge queue name=helixagent.tasks.notifications
```

### Kafka Topic Management

```bash
# List topics
kafka-topics --bootstrap-server localhost:9092 --list

# Describe topic
kafka-topics --bootstrap-server localhost:9092 \
  --describe --topic helixagent.events.audit

# Check topic offsets
kafka-run-class kafka.tools.GetOffsetShell \
  --bootstrap-server localhost:9092 \
  --topic helixagent.events.audit

# Consumer group status
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe --group helixagent-workers
```

---

## Monitoring Dashboard

### Key Metrics to Watch

| Metric | Warning Threshold | Critical Threshold | Action |
|--------|------------------|-------------------|--------|
| Queue Depth | > 1,000 | > 10,000 | Scale consumers |
| Consumer Lag | > 1,000 | > 10,000 | Check consumers |
| DLQ Count | > 10 | > 100 | Investigate failures |
| P95 Latency | > 100ms | > 1s | Check infrastructure |
| Error Rate | > 1% | > 5% | Investigate errors |

### Grafana Alerts

Configure these alerts in Grafana:

```yaml
# alerts/messaging-alerts.yml
apiVersion: 1
groups:
  - orgId: 1
    name: Messaging Alerts
    folder: HelixAgent
    interval: 1m
    rules:
      - uid: high-queue-depth
        title: High Queue Depth
        condition: A
        data:
          - refId: A
            queryType: ''
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: sum(messaging_queue_depth) > 10000
        for: 5m
        annotations:
          summary: Queue depth exceeds 10,000 messages
        labels:
          severity: warning

      - uid: dlq-accumulation
        title: DLQ Accumulation
        condition: A
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              expr: messaging_dlq_messages_total > 100
        for: 10m
        annotations:
          summary: DLQ has more than 100 messages
        labels:
          severity: critical
```

---

## Scaling Operations

### Horizontal Scaling - RabbitMQ

```bash
# Add RabbitMQ node to cluster
docker run -d --name helixagent-rabbitmq-2 \
  --network helixagent-network \
  -e RABBITMQ_ERLANG_COOKIE=helixagent-secret-cookie \
  rabbitmq:3.12-management

# Join cluster
docker exec helixagent-rabbitmq-2 rabbitmqctl stop_app
docker exec helixagent-rabbitmq-2 rabbitmqctl join_cluster rabbit@helixagent-rabbitmq
docker exec helixagent-rabbitmq-2 rabbitmqctl start_app

# Verify cluster status
docker exec helixagent-rabbitmq rabbitmqctl cluster_status
```

### Horizontal Scaling - Kafka

```yaml
# Add Kafka broker to docker-compose
kafka-2:
  image: confluentinc/cp-kafka:7.5.0
  container_name: helixagent-kafka-2
  environment:
    KAFKA_BROKER_ID: 2
    KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
    KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka-2:29092
```

### Scaling Consumers

```bash
# Scale HelixAgent workers
docker-compose up -d --scale helixagent=3

# Kubernetes scaling
kubectl scale deployment helixagent --replicas=5 -n helixagent
```

### Partition Reassignment

```bash
# Generate reassignment plan
kafka-reassign-partitions --bootstrap-server localhost:9092 \
  --topics-to-move-json-file topics.json \
  --broker-list "1,2,3" \
  --generate

# Execute reassignment
kafka-reassign-partitions --bootstrap-server localhost:9092 \
  --reassignment-json-file reassignment.json \
  --execute

# Verify reassignment
kafka-reassign-partitions --bootstrap-server localhost:9092 \
  --reassignment-json-file reassignment.json \
  --verify
```

---

## Backup and Recovery

### RabbitMQ Backup

```bash
#!/bin/bash
# backup-rabbitmq.sh

BACKUP_DIR="/backups/rabbitmq/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# Export definitions
docker exec helixagent-rabbitmq rabbitmqctl export_definitions - > $BACKUP_DIR/definitions.json

# Backup data directory
docker cp helixagent-rabbitmq:/var/lib/rabbitmq $BACKUP_DIR/data

echo "RabbitMQ backup completed: $BACKUP_DIR"
```

### RabbitMQ Restore

```bash
#!/bin/bash
# restore-rabbitmq.sh

BACKUP_DIR=$1

# Import definitions
docker exec -i helixagent-rabbitmq rabbitmqctl import_definitions - < $BACKUP_DIR/definitions.json

echo "RabbitMQ restore completed from: $BACKUP_DIR"
```

### Kafka Backup

```bash
#!/bin/bash
# backup-kafka.sh

BACKUP_DIR="/backups/kafka/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# List topics to backup
TOPICS=$(kafka-topics --bootstrap-server localhost:9092 --list | grep helixagent)

# Backup each topic
for topic in $TOPICS; do
  echo "Backing up topic: $topic"
  kafka-console-consumer --bootstrap-server localhost:9092 \
    --topic $topic \
    --from-beginning \
    --timeout-ms 10000 \
    > $BACKUP_DIR/${topic}.json 2>/dev/null || true
done

# Backup topic configurations
kafka-configs --bootstrap-server localhost:9092 \
  --entity-type topics \
  --describe --all > $BACKUP_DIR/topic-configs.txt

echo "Kafka backup completed: $BACKUP_DIR"
```

### Kafka Restore

```bash
#!/bin/bash
# restore-kafka.sh

BACKUP_DIR=$1

# Restore each topic
for file in $BACKUP_DIR/helixagent.*.json; do
  topic=$(basename $file .json)
  echo "Restoring topic: $topic"

  # Create topic if not exists
  kafka-topics --bootstrap-server localhost:9092 \
    --create --if-not-exists \
    --topic $topic \
    --partitions 6 \
    --replication-factor 1

  # Replay messages
  kafka-console-producer --bootstrap-server localhost:9092 \
    --topic $topic \
    < $file
done

echo "Kafka restore completed from: $BACKUP_DIR"
```

---

## Maintenance Procedures

### Rolling Restart - RabbitMQ

```bash
#!/bin/bash
# rolling-restart-rabbitmq.sh

NODES=("helixagent-rabbitmq" "helixagent-rabbitmq-2" "helixagent-rabbitmq-3")

for node in "${NODES[@]}"; do
  echo "Restarting $node..."

  # Stop accepting new connections
  docker exec $node rabbitmqctl stop_app

  # Wait for messages to drain
  sleep 30

  # Restart
  docker restart $node

  # Wait for node to rejoin cluster
  sleep 60

  # Verify node is healthy
  docker exec $node rabbitmqctl cluster_status

  echo "$node restarted successfully"
done

echo "Rolling restart complete"
```

### Rolling Restart - Kafka

```bash
#!/bin/bash
# rolling-restart-kafka.sh

BROKERS=("helixagent-kafka" "helixagent-kafka-2" "helixagent-kafka-3")

for broker in "${BROKERS[@]}"; do
  echo "Restarting $broker..."

  # Get broker ID
  BROKER_ID=$(docker exec $broker cat /var/lib/kafka/data/meta.properties | grep broker.id | cut -d= -f2)

  # Move partition leadership away
  kafka-preferred-replica-election --bootstrap-server localhost:9092

  # Wait for leadership to transfer
  sleep 60

  # Restart broker
  docker restart $broker

  # Wait for broker to rejoin
  sleep 90

  # Verify broker is ISR
  kafka-metadata --bootstrap-server localhost:9092 --describe

  echo "$broker restarted successfully"
done

echo "Rolling restart complete"
```

### Log Rotation

```bash
# /etc/logrotate.d/helixagent-messaging
/var/log/helixagent/messaging/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    create 0640 helixagent helixagent
    postrotate
        docker-compose exec -T helixagent kill -USR1 1
    endscript
}
```

---

## Incident Response

### High Queue Depth

**Symptoms:** Queue depth > 10,000, increasing latency

**Diagnosis:**
```bash
# Check queue details
rabbitmqadmin -u helixagent -p helixagent123 list queues \
  name messages consumers

# Check consumer status
rabbitmqadmin -u helixagent -p helixagent123 list consumers
```

**Resolution:**
1. Scale consumers: `docker-compose up -d --scale helixagent=5`
2. Check for slow consumers in logs
3. Verify network connectivity
4. Consider increasing prefetch count

### Consumer Lag

**Symptoms:** Growing consumer lag, delayed event processing

**Diagnosis:**
```bash
# Check lag
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe --group helixagent-workers

# Check consumer status
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe --group helixagent-workers --members
```

**Resolution:**
1. Add more consumer instances
2. Check for processing bottlenecks
3. Increase partition count for parallelism
4. Review consumer configuration

### DLQ Overflow

**Symptoms:** DLQ count > 100, repeated failures

**Diagnosis:**
```bash
# List DLQ messages
curl http://localhost:7061/v1/messaging/dlq?limit=10 | jq .

# Check failure reasons
rabbitmqadmin -u helixagent -p helixagent123 get queue=helixagent.dlq count=10
```

**Resolution:**
1. Identify failure patterns
2. Fix root cause
3. Reprocess valid messages: `POST /v1/messaging/dlq/{id}/reprocess`
4. Discard invalid messages: `DELETE /v1/messaging/dlq/{id}`

### Broker Unavailable

**Symptoms:** Connection refused, publish/consume failures

**Diagnosis:**
```bash
# Check broker status
docker-compose ps
docker logs helixagent-rabbitmq --tail 100
docker logs helixagent-kafka --tail 100
```

**Resolution:**
1. Restart broker: `docker-compose restart rabbitmq`
2. Check disk space: `df -h`
3. Check memory: `free -m`
4. Review broker logs for errors

---

## Performance Tuning

### RabbitMQ Tuning

```bash
# Increase file descriptors
docker exec helixagent-rabbitmq rabbitmqctl eval 'file_handle_cache:info().'

# Set memory high watermark
docker exec helixagent-rabbitmq rabbitmqctl set_vm_memory_high_watermark 0.8

# Enable lazy queues for large backlogs
rabbitmqadmin -u helixagent -p helixagent123 declare queue \
  name=helixagent.tasks.background \
  arguments='{"x-queue-mode": "lazy"}'
```

### Kafka Tuning

```bash
# Increase retention for high-throughput topics
kafka-configs --bootstrap-server localhost:9092 \
  --alter --entity-type topics \
  --entity-name helixagent.stream.tokens \
  --add-config retention.ms=3600000

# Increase partitions for parallelism
kafka-topics --bootstrap-server localhost:9092 \
  --alter --topic helixagent.events.llm.responses \
  --partitions 12

# Producer tuning
# Set in application configuration:
# batch.size=65536
# linger.ms=5
# compression.type=lz4
```

### Consumer Tuning

```go
// Optimal consumer configuration
config := messaging.ConsumerConfig{
    PrefetchCount:      100,      // RabbitMQ prefetch
    MaxPollRecords:     500,      // Kafka max poll
    SessionTimeout:     30000,    // Kafka session timeout
    HeartbeatInterval:  10000,    // Kafka heartbeat
    AutoCommit:         false,    // Manual commit for reliability
}
```

---

## Runbooks

### Runbook: Complete System Restart

```bash
#!/bin/bash
# runbook-full-restart.sh

echo "Starting full system restart..."

# 1. Stop consumers gracefully
docker-compose stop helixagent

# 2. Wait for in-flight messages
sleep 30

# 3. Restart messaging infrastructure
docker-compose restart zookeeper
sleep 30
docker-compose restart kafka
sleep 60
docker-compose restart rabbitmq
sleep 30

# 4. Verify infrastructure health
./morning-health-check.sh

# 5. Start consumers
docker-compose up -d helixagent

echo "Full restart complete"
```

### Runbook: Emergency Queue Purge

```bash
#!/bin/bash
# runbook-emergency-purge.sh
# WARNING: This deletes all messages in the specified queue

QUEUE=$1

if [ -z "$QUEUE" ]; then
    echo "Usage: $0 <queue-name>"
    exit 1
fi

echo "WARNING: This will delete ALL messages in queue: $QUEUE"
read -p "Are you sure? (yes/no): " confirm

if [ "$confirm" = "yes" ]; then
    rabbitmqadmin -u helixagent -p helixagent123 purge queue name=$QUEUE
    echo "Queue $QUEUE purged"
else
    echo "Aborted"
fi
```

---

## Contact and Escalation

| Issue Type | First Response | Escalation |
|------------|---------------|------------|
| High queue depth | On-call engineer | Team lead after 30min |
| Broker down | On-call engineer | Team lead immediately |
| DLQ overflow | Morning review | Team lead if > 500 |
| Performance degradation | Monitoring team | Engineering if > 1hr |
