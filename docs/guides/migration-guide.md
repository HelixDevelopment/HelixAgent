# Migration Guide

This guide covers migrating from PostgreSQL-based task queuing to the new RabbitMQ/Kafka messaging system.

## Overview

HelixAgent supports three migration modes:

| Mode | Description | Use Case |
|------|-------------|----------|
| `Legacy` | PostgreSQL only | Current behavior |
| `DualWrite` | Both PostgreSQL and RabbitMQ | Migration testing |
| `Messaging` | RabbitMQ/Kafka only | Target state |

## Pre-Migration Checklist

Before migrating, ensure:

- [ ] RabbitMQ and Kafka infrastructure deployed
- [ ] All challenge scripts pass (`./challenges/scripts/run_all_challenges.sh`)
- [ ] Database backup completed
- [ ] Monitoring dashboards configured
- [ ] Rollback plan documented
- [ ] Stakeholders notified

## Migration Steps

### Step 1: Deploy Infrastructure

```bash
# Start messaging infrastructure
docker-compose -f docker-compose.messaging.yml --profile messaging up -d

# Verify health
docker exec helixagent-rabbitmq rabbitmqctl ping
docker exec helixagent-kafka kafka-broker-api-versions --bootstrap-server localhost:9092
```

### Step 2: Run Validation Challenges

```bash
# Run all challenges to validate setup
./challenges/scripts/messaging_migration_challenge.sh

# Expected output: 21/21 tests passed
```

### Step 3: Enable Dual-Write Mode

Update your configuration:

```yaml
# configs/messaging.yaml
migration:
  mode: dual_write  # legacy | dual_write | messaging
  verify_consistency: true
  log_discrepancies: true
```

Or via environment:

```bash
export MIGRATION_MODE=dual_write
make run
```

In dual-write mode:
- Tasks written to both PostgreSQL and RabbitMQ
- Consumers read from PostgreSQL (primary)
- Discrepancies logged for analysis

### Step 4: Monitor Dual-Write

Watch for consistency issues:

```bash
# Check logs for discrepancies
tail -f logs/helixagent.log | grep "migration"

# Monitor RabbitMQ queue depth
watch -n 5 'docker exec helixagent-rabbitmq rabbitmqctl list_queues name messages'

# Monitor PostgreSQL task count
watch -n 5 'psql -c "SELECT status, COUNT(*) FROM tasks GROUP BY status"'
```

Expected behavior:
- Queue depths should remain low
- Task counts should match
- No discrepancy warnings

### Step 5: Gradual Traffic Migration

Once dual-write is stable, gradually shift consumers:

```yaml
migration:
  mode: dual_write
  consumer_traffic_split:
    postgresql: 90  # Percent reading from PostgreSQL
    rabbitmq: 10    # Percent reading from RabbitMQ
```

Increase RabbitMQ percentage over time:

```bash
# Week 1: 10%
export CONSUMER_RABBITMQ_PERCENT=10

# Week 2: 50%
export CONSUMER_RABBITMQ_PERCENT=50

# Week 3: 90%
export CONSUMER_RABBITMQ_PERCENT=90

# Week 4: 100%
export CONSUMER_RABBITMQ_PERCENT=100
```

### Step 6: Full Migration

Once 100% traffic is on RabbitMQ:

```yaml
migration:
  mode: messaging  # Full messaging mode
```

### Step 7: Cleanup

After successful migration:

```bash
# Archive PostgreSQL task queue
pg_dump -t tasks helixagent_db > tasks_backup_final.sql

# Optional: Truncate old task table
# psql -c "TRUNCATE TABLE tasks"
```

## Rollback Procedure

If issues occur, rollback immediately:

### Automatic Rollback

```yaml
migration:
  auto_rollback:
    enabled: true
    error_threshold: 10  # Errors per minute
    latency_threshold: 5000  # ms
```

### Manual Rollback

```bash
# Emergency rollback to PostgreSQL
export MIGRATION_MODE=legacy
make restart

# Or via API
curl -X POST http://localhost:8080/admin/migration/rollback
```

### Rollback Verification

```bash
# Verify PostgreSQL consumers active
curl http://localhost:8080/health | jq '.task_queue'

# Check pending tasks are processing
psql -c "SELECT COUNT(*) FROM tasks WHERE status = 'pending'"
```

## Backward Compatibility

The messaging system maintains 100% backward compatibility:

### CLI Agents

All 18 CLI agents continue working:
- OpenCode, Crush, HelixCode, Kiro
- Aider, ClaudeCode, Cline, CodenameGoose
- DeepSeekCLI, Forge, GeminiCLI, GPTEngineer
- KiloCode, MistralCode, OllamaCode, Plandex
- QwenCode, AmazonQ

### API Endpoints

All existing endpoints unchanged:
- `POST /v1/tasks` - Create task
- `GET /v1/tasks/:id` - Get task status
- `GET /v1/tasks/:id/events` - SSE updates
- `WS /v1/ws/tasks/:id` - WebSocket updates

### Background Task System

Background task interfaces unchanged:
- `TaskQueue.Enqueue()` works with both backends
- `WorkerPool` processes tasks from either source
- Task states and callbacks identical

## Zero-Downtime Migration

For zero-downtime migration:

### Blue-Green Deployment

1. Deploy new version with `dual_write` mode
2. Verify all health checks pass
3. Route 10% traffic to new version
4. Monitor for errors
5. Gradually increase traffic
6. Complete cutover
7. Decommission old version

### Rolling Update

```bash
# Update configuration
kubectl set env deployment/helixagent MIGRATION_MODE=dual_write

# Rolling restart
kubectl rollout restart deployment/helixagent

# Monitor rollout
kubectl rollout status deployment/helixagent
```

## Data Migration

### Migrating Pending Tasks

Existing PostgreSQL tasks are migrated automatically:

```go
// internal/messaging/migration.go
func (m *MigrationManager) MigratePendingTasks(ctx context.Context) error {
    // Read all pending tasks from PostgreSQL
    tasks, err := m.postgresQueue.GetPendingTasks(ctx)
    if err != nil {
        return err
    }

    // Publish to RabbitMQ
    for _, task := range tasks {
        if err := m.rabbitQueue.EnqueueTask(ctx, task); err != nil {
            m.logger.Error("Failed to migrate task", zap.String("id", task.ID))
            continue
        }
        // Mark as migrated in PostgreSQL
        m.postgresQueue.MarkMigrated(ctx, task.ID)
    }

    return nil
}
```

### Handling In-Flight Tasks

In-flight tasks (status=running) are handled by:

1. Allow current workers to complete
2. New tasks go to RabbitMQ
3. Monitor until all in-flight tasks complete

```sql
-- Check in-flight tasks
SELECT id, type, started_at
FROM tasks
WHERE status = 'running'
ORDER BY started_at;
```

## Monitoring During Migration

### Key Metrics

| Metric | Normal | Alert |
|--------|--------|-------|
| `migration_discrepancies` | 0 | > 0 |
| `rabbitmq_queue_depth` | < 1000 | > 10000 |
| `task_completion_rate` | > 99% | < 95% |
| `task_latency_p99` | < 100ms | > 500ms |

### Dashboards

Import Grafana dashboards:
- `configs/grafana/messaging-migration.json`

### Alerts

Configure alerts in `configs/prometheus/alerts.yml`:

```yaml
groups:
  - name: migration
    rules:
      - alert: MigrationDiscrepancy
        expr: migration_discrepancies > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: Migration consistency error detected
```

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Queue depth growing | Slow consumers | Scale workers |
| Discrepancies logged | Race condition | Increase dual-write timeout |
| Tasks stuck | Worker crash | Check worker logs |
| Connection errors | Infrastructure issue | Check broker health |

### Debug Commands

```bash
# Check migration status
curl http://localhost:8080/admin/migration/status

# Force task migration
curl -X POST http://localhost:8080/admin/migration/migrate-pending

# Check discrepancy log
tail -100 logs/migration-discrepancies.log

# RabbitMQ queue inspection
docker exec helixagent-rabbitmq rabbitmqctl list_queues name messages_ready consumers

# Kafka consumer lag
docker exec helixagent-kafka kafka-consumer-groups --bootstrap-server localhost:9092 --describe --group helixagent-group
```

## Post-Migration Validation

After completing migration:

```bash
# Run full test suite
make test

# Run all challenges
./challenges/scripts/run_all_challenges.sh

# Verify CLI agents
for agent in opencode crush helix kiro; do
    $agent --version
done

# Load test
go test ./tests/performance/... -bench=. -benchtime=60s
```

## Support

For migration assistance:
- Check logs: `logs/migration.log`
- Open issue: https://github.com/HelixDevelopment/helix_agent/issues
- Documentation: [Messaging Architecture](../architecture/messaging-architecture.md)
