# Video Course 09: Production Operations

## Course Overview

**Duration:** 5 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 03 (Deployment)

Master production operations for HelixAgent, including high availability, scaling, monitoring, incident response, and operational best practices.

---

## Module 1: Production Architecture

### Video 1.1: High Availability Design (30 min)

**Topics:**
- HA architecture patterns
- Redundancy strategies
- Failover mechanisms
- Geographic distribution

**HA Architecture:**
```
┌─────────────────────────────────────────────────────────────────────┐
│                         Load Balancer (HA)                          │
│                      (HAProxy / NGINX / ALB)                        │
└─────────────────────────┬───────────────────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│  HelixAgent   │ │  HelixAgent   │ │  HelixAgent   │
│   Instance 1  │ │   Instance 2  │ │   Instance 3  │
│ (Primary Zone)│ │ (Primary Zone)│ │(Secondary Zone)│
└───────┬───────┘ └───────┬───────┘ └───────┬───────┘
        │                 │                 │
        └────────┬────────┴────────┬────────┘
                 │                 │
        ┌────────▼────────┐ ┌──────▼──────┐
        │   PostgreSQL    │ │    Redis    │
        │    (Primary)    │ │  (Cluster)  │
        │   + Replicas    │ │             │
        └─────────────────┘ └─────────────┘
```

### Video 1.2: Database High Availability (25 min)

**Topics:**
- PostgreSQL replication
- Connection pooling
- Automatic failover
- Backup strategies

**PostgreSQL HA Setup:**
```yaml
# docker-compose-ha.yml
services:
  postgres-primary:
    image: postgres:15
    environment:
      POSTGRES_USER: helixagent
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: helixagent_db
    volumes:
      - postgres-primary:/var/lib/postgresql/data
    command: |
      postgres
      -c wal_level=replica
      -c max_wal_senders=10
      -c max_replication_slots=10
      -c hot_standby=on

  postgres-replica:
    image: postgres:15
    environment:
      POSTGRES_USER: helixagent
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    depends_on:
      - postgres-primary
    command: |
      bash -c "
      pg_basebackup -h postgres-primary -D /var/lib/postgresql/data -U replicator -P -R
      postgres
      "

  pgpool:
    image: bitnami/pgpool:4
    environment:
      PGPOOL_BACKEND_NODES: "0:postgres-primary:5432,1:postgres-replica:5432"
      PGPOOL_ENABLE_LOAD_BALANCING: "yes"
      PGPOOL_SR_CHECK_USER: replicator
    ports:
      - "5432:5432"
```

### Video 1.3: Redis Cluster Setup (25 min)

**Topics:**
- Redis Cluster configuration
- Sentinel for failover
- Session persistence
- Cache strategies

**Redis Cluster:**
```yaml
# redis-cluster.yml
services:
  redis-node-1:
    image: redis:7
    command: redis-server --cluster-enabled yes --cluster-config-file nodes.conf
    ports:
      - "7001:6379"

  redis-node-2:
    image: redis:7
    command: redis-server --cluster-enabled yes --cluster-config-file nodes.conf
    ports:
      - "7002:6379"

  redis-node-3:
    image: redis:7
    command: redis-server --cluster-enabled yes --cluster-config-file nodes.conf
    ports:
      - "7003:6379"

  # After startup, create cluster:
  # redis-cli --cluster create redis-node-1:6379 redis-node-2:6379 redis-node-3:6379
```

---

## Module 2: Scaling Strategies

### Video 2.1: Horizontal Scaling (30 min)

**Topics:**
- Stateless design
- Session management
- Load distribution
- Auto-scaling rules

**Kubernetes HPA:**
```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: helixagent-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: helixagent
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
    - type: Pods
      pods:
        metric:
          name: http_requests_per_second
        target:
          type: AverageValue
          averageValue: "100"
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
        - type: Pods
          value: 4
          periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Percent
          value: 10
          periodSeconds: 60
```

### Video 2.2: Vertical Scaling (20 min)

**Topics:**
- Resource limits
- Memory optimization
- CPU allocation
- Tuning guidelines

**Resource Configuration:**
```yaml
# k8s/deployment.yaml
spec:
  containers:
    - name: helixagent
      resources:
        requests:
          cpu: "500m"
          memory: "512Mi"
        limits:
          cpu: "2000m"
          memory: "2Gi"
      env:
        - name: GOMAXPROCS
          valueFrom:
            resourceFieldRef:
              resource: limits.cpu
        - name: GOMEMLIMIT
          value: "1800MiB"  # 90% of limit
```

### Video 2.3: Database Scaling (25 min)

**Topics:**
- Read replicas
- Connection pooling
- Query optimization
- Partitioning strategies

**PgBouncer Configuration:**
```ini
# pgbouncer.ini
[databases]
helixagent = host=postgres-primary port=5432 dbname=helixagent_db
helixagent_ro = host=postgres-replica port=5432 dbname=helixagent_db

[pgbouncer]
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 20
min_pool_size = 5
reserve_pool_size = 5
reserve_pool_timeout = 3
max_db_connections = 50
```

---

## Module 3: Monitoring and Observability

### Video 3.1: Metrics Collection (30 min)

**Topics:**
- Prometheus setup
- Custom metrics
- Labels and dimensions
- Retention policies

**Key Metrics:**
```go
// internal/metrics/metrics.go
var (
    RequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "helixagent_requests_total",
            Help: "Total number of requests",
        },
        []string{"method", "endpoint", "status"},
    )

    RequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "helixagent_request_duration_seconds",
            Help:    "Request duration in seconds",
            Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
        },
        []string{"method", "endpoint"},
    )

    ProviderScore = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "helixagent_provider_score",
            Help: "Provider verification score",
        },
        []string{"provider"},
    )

    DebateConfidence = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "helixagent_debate_confidence",
            Help:    "Debate consensus confidence",
            Buckets: []float64{0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 1.0},
        },
    )
)
```

### Video 3.2: Grafana Dashboards (25 min)

**Topics:**
- Dashboard design
- Panel types
- Variables and templating
- Alerting

**Dashboard JSON:**
```json
{
  "dashboard": {
    "title": "HelixAgent Overview",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "sum(rate(helixagent_requests_total[5m])) by (endpoint)",
            "legendFormat": "{{endpoint}}"
          }
        ]
      },
      {
        "title": "Provider Scores",
        "type": "gauge",
        "targets": [
          {
            "expr": "helixagent_provider_score",
            "legendFormat": "{{provider}}"
          }
        ],
        "options": {
          "minValue": 0,
          "maxValue": 10,
          "thresholds": [
            {"value": 5, "color": "red"},
            {"value": 7, "color": "yellow"},
            {"value": 8, "color": "green"}
          ]
        }
      },
      {
        "title": "Error Rate",
        "type": "stat",
        "targets": [
          {
            "expr": "sum(rate(helixagent_requests_total{status=~\"5..\"}[5m])) / sum(rate(helixagent_requests_total[5m])) * 100"
          }
        ]
      }
    ]
  }
}
```

### Video 3.3: Distributed Tracing (25 min)

**Topics:**
- OpenTelemetry setup
- Trace context propagation
- Span attributes
- Jaeger integration

**Tracing Setup:**
```go
// internal/tracing/setup.go
func SetupTracing(serviceName string) (*sdktrace.TracerProvider, error) {
    exporter, err := otlptrace.New(
        context.Background(),
        otlptracegrpc.NewClient(
            otlptracegrpc.WithEndpoint("jaeger:4317"),
            otlptracegrpc.WithInsecure(),
        ),
    )
    if err != nil {
        return nil, err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(serviceName),
            semconv.ServiceVersion("1.0.0"),
        )),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)), // 10% sampling
    )

    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    return tp, nil
}
```

---

## Module 4: Log Management

### Video 4.1: Structured Logging (25 min)

**Topics:**
- Log formats (JSON)
- Log levels
- Contextual logging
- Sensitive data handling

**Logger Configuration:**
```go
// internal/utils/logger.go
type Logger struct {
    logger *slog.Logger
}

func NewLogger(level string) *Logger {
    var logLevel slog.Level
    switch level {
    case "debug":
        logLevel = slog.LevelDebug
    case "info":
        logLevel = slog.LevelInfo
    case "warn":
        logLevel = slog.LevelWarn
    case "error":
        logLevel = slog.LevelError
    }

    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: logLevel,
        ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
            // Redact sensitive fields
            if a.Key == "api_key" || a.Key == "password" || a.Key == "token" {
                return slog.String(a.Key, "[REDACTED]")
            }
            return a
        },
    })

    return &Logger{logger: slog.New(handler)}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
    traceID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()
    return &Logger{
        logger: l.logger.With("trace_id", traceID),
    }
}
```

### Video 4.2: Log Aggregation (25 min)

**Topics:**
- ELK Stack setup
- Log shipping
- Index management
- Search and analysis

**Fluentd Configuration:**
```yaml
# fluentd.conf
<source>
  @type tail
  path /var/log/helixagent/*.log
  pos_file /var/log/fluentd/helixagent.pos
  tag helixagent.*
  <parse>
    @type json
    time_key timestamp
    time_format %Y-%m-%dT%H:%M:%S.%NZ
  </parse>
</source>

<filter helixagent.**>
  @type record_transformer
  <record>
    hostname "#{Socket.gethostname}"
    environment "#{ENV['ENVIRONMENT']}"
  </record>
</filter>

<match helixagent.**>
  @type elasticsearch
  host elasticsearch
  port 9200
  logstash_format true
  logstash_prefix helixagent
  <buffer>
    flush_interval 5s
    chunk_limit_size 10m
    retry_max_interval 30
  </buffer>
</match>
```

### Video 4.3: Log-Based Alerting (20 min)

**Topics:**
- Error pattern detection
- Alert rules
- Notification channels
- Escalation policies

**ElastAlert Rules:**
```yaml
# rules/high_error_rate.yaml
name: High Error Rate
type: frequency
index: helixagent-*
num_events: 100
timeframe:
  minutes: 5
filter:
  - term:
      level: error
alert:
  - slack
slack_webhook_url: ${SLACK_WEBHOOK}
slack_channel: "#alerts"
alert_text: |
  High error rate detected in HelixAgent
  Error count: {0}
  Time range: {1}
alert_text_args:
  - num_hits
  - timeframe
```

---

## Module 5: Incident Response

### Video 5.1: Runbook Creation (30 min)

**Topics:**
- Runbook structure
- Common scenarios
- Step-by-step procedures
- Automation opportunities

**Runbook Example:**
```markdown
# Runbook: Provider Degradation

## Symptoms
- Provider score drops below 7.0
- Error rate increases for specific provider
- Response latency spikes

## Impact
- Reduced response quality
- Potential debate failures
- User experience degradation

## Investigation Steps

1. Check provider status:
   ```bash
   curl http://localhost:8080/v1/providers/verification
   ```

2. Review provider metrics:
   ```promql
   helixagent_provider_score{provider="<name>"}
   rate(helixagent_provider_errors_total{provider="<name>"}[5m])
   ```

3. Check provider health:
   ```bash
   ./helixagent verify --provider <name> --verbose
   ```

## Resolution Steps

### If provider API is down:
1. Provider will auto-failover to fallback
2. Monitor fallback performance
3. Contact provider support if extended

### If rate limited:
1. Check current request rate
2. Adjust rate limiter:
   ```yaml
   providers:
     <name>:
       rate_limit:
         requests_per_minute: 30  # Reduce
   ```
3. Trigger config reload:
   ```bash
   curl -X POST http://localhost:8080/admin/reload
   ```

### If authentication failure:
1. Verify API key:
   ```bash
   ./helixagent verify --provider <name> --test auth
   ```
2. Rotate key if compromised
3. Update configuration

## Escalation
- If unresolved after 15 minutes: Page on-call engineer
- If affecting >10% of requests: Incident severity P2
```

### Video 5.2: Incident Management (25 min)

**Topics:**
- Incident classification
- Communication templates
- Post-mortem process
- Continuous improvement

**Incident Template:**
```markdown
# Incident Report: [INCIDENT-XXXX]

## Summary
- **Severity**: P1 / P2 / P3
- **Duration**: Start - End
- **Impact**: Description of user impact

## Timeline
| Time (UTC) | Event |
|------------|-------|
| HH:MM | Initial alert triggered |
| HH:MM | On-call acknowledged |
| HH:MM | Root cause identified |
| HH:MM | Mitigation applied |
| HH:MM | Incident resolved |

## Root Cause
Detailed explanation of what caused the incident.

## Resolution
Steps taken to resolve the incident.

## Action Items
| Item | Owner | Due Date | Status |
|------|-------|----------|--------|
| Improve monitoring | @engineer | YYYY-MM-DD | Open |
| Add runbook | @sre | YYYY-MM-DD | Open |

## Lessons Learned
What we learned and how we can prevent similar incidents.
```

### Video 5.3: On-Call Best Practices (20 min)

**Topics:**
- On-call rotations
- Alert fatigue prevention
- Escalation policies
- Mental health considerations

**PagerDuty Service:**
```yaml
# pagerduty-service.yaml
services:
  - name: helixagent-production
    escalation_policy: production-oncall
    alert_creation: create_alerts_and_incidents
    auto_resolve_timeout: 14400  # 4 hours

schedules:
  - name: production-oncall
    time_zone: UTC
    layers:
      - name: primary
        rotation_turn_length_seconds: 604800  # 1 week
        users:
          - user_id_1
          - user_id_2

escalation_policies:
  - name: production-oncall
    num_loops: 3
    rules:
      - escalation_delay_in_minutes: 5
        targets:
          - type: schedule_reference
            id: production-oncall
      - escalation_delay_in_minutes: 15
        targets:
          - type: user_reference
            id: engineering_manager
```

---

## Module 6: Backup and Recovery

### Video 6.1: Backup Strategies (25 min)

**Topics:**
- Database backups
- Configuration backups
- Secret management
- Backup validation

**Backup Script:**
```bash
#!/bin/bash
# backup.sh

set -euo pipefail

BACKUP_DIR="/backups/$(date +%Y-%m-%d)"
mkdir -p "$BACKUP_DIR"

# Database backup
echo "Backing up PostgreSQL..."
pg_dump -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" \
    --format=custom \
    --file="$BACKUP_DIR/helixagent_db.dump"

# Redis backup
echo "Backing up Redis..."
redis-cli -h "$REDIS_HOST" --rdb "$BACKUP_DIR/redis.rdb"

# Configuration backup
echo "Backing up configurations..."
tar -czf "$BACKUP_DIR/configs.tar.gz" /etc/helixagent/

# Encrypt backup
echo "Encrypting backup..."
gpg --symmetric --cipher-algo AES256 \
    --output "$BACKUP_DIR/backup.tar.gz.gpg" \
    --passphrase-file /etc/helixagent/backup-key \
    "$BACKUP_DIR"

# Upload to S3
echo "Uploading to S3..."
aws s3 cp "$BACKUP_DIR/backup.tar.gz.gpg" \
    "s3://helixagent-backups/$(date +%Y/%m/%d)/"

# Cleanup old local backups
find /backups -type d -mtime +7 -exec rm -rf {} +

echo "Backup completed successfully"
```

### Video 6.2: Disaster Recovery (30 min)

**Topics:**
- RTO and RPO
- Recovery procedures
- DR testing
- Multi-region setup

**DR Plan:**
```markdown
# Disaster Recovery Plan

## Recovery Objectives
- **RPO** (Recovery Point Objective): 1 hour
- **RTO** (Recovery Time Objective): 4 hours

## Backup Schedule
- Database: Every 6 hours + continuous WAL archiving
- Redis: Every hour
- Configurations: On every change

## Recovery Procedures

### Full Region Failure

1. **Declare Incident** (5 min)
   - Notify stakeholders
   - Start incident channel

2. **Activate DR Region** (15 min)
   - Update DNS to DR region
   - Verify DR instances healthy

3. **Restore Database** (30 min)
   - Restore from latest backup
   - Apply WAL logs
   - Verify data integrity

4. **Restore Redis** (15 min)
   - Restore from RDB
   - Verify cache state

5. **Verify Services** (30 min)
   - Run health checks
   - Verify provider connectivity
   - Test critical flows

6. **Resume Operations** (15 min)
   - Enable traffic
   - Monitor error rates
   - Confirm recovery

## DR Testing Schedule
- Tabletop exercise: Monthly
- Partial failover test: Quarterly
- Full DR test: Annually
```

---

## Module 7: Performance Tuning

### Video 7.1: Application Tuning (25 min)

**Topics:**
- Go runtime tuning
- Connection pool sizing
- Buffer optimization
- Garbage collection

**Tuning Configuration:**
```go
// main.go
func init() {
    // Set GOMAXPROCS based on container limits
    if _, err := maxprocs.Set(); err != nil {
        log.Printf("Failed to set GOMAXPROCS: %v", err)
    }

    // Set memory limit (leave room for OS)
    if limit := os.Getenv("GOMEMLIMIT"); limit != "" {
        debug.SetMemoryLimit(parseMemoryLimit(limit))
    }

    // Tune GC
    if gcPercent := os.Getenv("GOGC"); gcPercent != "" {
        val, _ := strconv.Atoi(gcPercent)
        debug.SetGCPercent(val)
    }
}
```

### Video 7.2: Database Tuning (25 min)

**Topics:**
- Query optimization
- Index strategies
- Connection tuning
- Explain analysis

**PostgreSQL Tuning:**
```sql
-- postgresql.conf optimizations

-- Memory
shared_buffers = '4GB'              -- 25% of RAM
effective_cache_size = '12GB'       -- 75% of RAM
work_mem = '256MB'
maintenance_work_mem = '1GB'

-- Connections
max_connections = 200

-- WAL
wal_buffers = '64MB'
checkpoint_completion_target = 0.9
max_wal_size = '4GB'

-- Query planner
random_page_cost = 1.1              -- SSD
effective_io_concurrency = 200       -- SSD

-- Index creation
CREATE INDEX CONCURRENTLY idx_tasks_status_created
    ON tasks(status, created_at DESC)
    WHERE status NOT IN ('completed', 'failed');

CREATE INDEX CONCURRENTLY idx_debates_topic_trgm
    ON debates USING gin(topic gin_trgm_ops);
```

---

## Hands-on Labs

### Lab 1: HA Setup
Deploy HelixAgent with PostgreSQL and Redis HA.

### Lab 2: Monitoring Stack
Configure Prometheus, Grafana, and alerting.

### Lab 3: Incident Simulation
Run a game day with simulated failures.

### Lab 4: DR Test
Execute a disaster recovery procedure.

---

## Resources

- [Operations Runbooks](/docs/operations/runbooks/)
- [Monitoring Guide](/docs/guides/monitoring.md)
- [Backup Documentation](/docs/operations/backup.md)
- [HelixAgent GitHub](https://dev.helix.agent)

---

## Course Completion

Congratulations! You've completed the Production Operations course. You should now be able to:

- Design and implement high availability
- Scale HelixAgent effectively
- Set up comprehensive monitoring
- Respond to incidents efficiently
- Implement backup and recovery procedures
