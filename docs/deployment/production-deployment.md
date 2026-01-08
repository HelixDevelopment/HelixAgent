# HelixAgent Production Deployment Guide

## 🚀 Quick Production Setup

### 1. Environment Configuration
```bash
# Copy and configure environment
cp .env.example .env.production

# Required environment variables
export SERVER_PORT=8080
export DB_HOST=your-production-db-host
export DB_PASSWORD=secure-production-password
export JWT_SECRET=your-32-char-jwt-secret-key

# LLM Provider Keys (choose one or more)
export DEEPSEEK_API_KEY=sk-your-deepseek-key
export QWEN_API_KEY=sk-your-qwen-key  
export OPENROUTER_API_KEY=sk-your-openrouter-key
```

### 2. Database Setup
```bash
# Initialize PostgreSQL with production schema
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -f scripts/init-db.sql

# Create production indexes
psql -h $DB_HOST -U $DB_USER -d $DB_NAME << EOF
CREATE INDEX CONCURRENTLY idx_llm_requests_user_created 
ON llm_requests(user_id, created_at);
CREATE INDEX CONCURRENTLY idx_sessions_user_active 
ON sessions(user_id) WHERE expires_at > NOW();
EOF
```

### 3. Production Deployment
```bash
# Docker Compose Production
docker-compose --profile prod up -d

# Kubernetes Production  
kubectl apply -f deploy/kubernetes/
kubectl set image deployment/helixagent helixagent=helixagent:latest

# Verify deployment
kubectl get pods -n helixagent
kubectl logs -f deployment/helixagent -n helixagent
```

## 🔒 Production Security

### 1. TLS/SSL Configuration
```bash
# Let's Encrypt (recommended)
export CERT_MANAGER=true
export LETSENCRYPT_EMAIL=admin@yourdomain.com

# Manual certificates
export TLS_CERT_PATH=/path/to/cert.pem
export TLS_KEY_PATH=/path/to/key.pem
```

### 2. API Security
```bash
# Rate limiting
export RATE_LIMIT_REQUESTS=1000
export RATE_LIMIT_WINDOW=1m

# CORS configuration
export CORS_ORIGINS="https://app.yourdomain.com,https://api.yourdomain.com"

# Security headers
export SECURITY_ENABLED=true
export CSP_ENABLED=true
```

### 3. Secrets Management
```bash
# Kubernetes secrets
kubectl create secret generic helixagent-prod-secrets \
  --from-literal=db-password=$DB_PASSWORD \
  --from-literal=jwt-secret=$JWT_SECRET \
  --from-literal=deepseek-api-key=$DEEPSEEK_API_KEY

# Docker secrets
docker secret create helixagent-db-password $DB_PASSWORD
docker secret create helixagent-jwt-secret $JWT_SECRET
```

## 📊 Production Monitoring

### 1. Prometheus Configuration
```yaml
# monitoring/prometheus-production.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "alert-rules.yml"

scrape_configs:
  - job_name: 'helixagent'
    static_configs:
      - targets: ['helixagent:7061']
    metrics_path: '/metrics'
    scrape_interval: 5s
    basic_auth:
      username: 'admin'
      password: '$GRAFANA_PASSWORD'

  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres-exporter:9187']

  - job_name: 'redis'
    static_configs:
      - targets: ['redis-exporter:9121']

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

### 2. Alert Rules
```yaml
# monitoring/alert-rules.yml
groups:
- name: helixagent-production
  rules:
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "High error rate detected"
      description: "Error rate is {{ $value | humanizePercentage }}"

  - alert: HighResponseTime
    expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 5
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High response time detected"
      description: "95th percentile response time is {{ $value }}s"

  - alert: LLMProviderDown
    expr: llm_providers_up == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "LLM provider is down"
      description: "Provider {{ $labels.provider }} has been down for more than 1 minute"

  - alert: DatabaseConnectionFailed
    expr: up{job="postgres"} == 0
    for: 30s
    labels:
      severity: critical
    annotations:
      summary: "Database connection failed"
      description: "PostgreSQL database has been down for more than 30 seconds"
```

### 3. Grafana Dashboard Import
```bash
# Import production dashboards
curl -X POST \
  http://admin:$GRAFANA_PASSWORD@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @monitoring/dashboards/helixagent-dashboard.json

# Configure alerts
curl -X POST \
  http://admin:$GRAFANA_PASSWORD@localhost:3000/api/alert-notifications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Slack Alerts",
    "type": "slack",
    "settings": {
      "url": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
    }
  }'
```

## 🔄 Production Scaling

### 1. Horizontal Scaling
```bash
# Docker Compose scaling
docker-compose --profile prod up -d --scale helixagent=5

# Kubernetes HPA
kubectl autoscale deployment helixagent \
  --cpu-percent=70 \
  --min=2 \
  --max=20 \
  -n helixagent

# Manual scaling
kubectl scale deployment helixagent --replicas=10 -n helixagent
```

### 2. Database Scaling
```sql
-- PostgreSQL optimization for production
ALTER SYSTEM SET max_connections = 200;
ALTER SYSTEM SET shared_buffers = '2GB';
ALTER SYSTEM SET effective_cache_size = '6GB';
ALTER SYSTEM SET maintenance_work_mem = '512MB';
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_buffers = '64MB';
SELECT pg_reload_conf();

-- Connection pooling configuration
CREATE USER pgbouncer WITH PASSWORD 'secure-pgbouncer-password';
GRANT CONNECT ON DATABASE helixagent_db TO pgbouncer;
```

### 3. Redis Clustering
```bash
# Redis cluster setup
redis-cli --cluster create \
  redis-1:6379 redis-2:6379 redis-3:6379 \
  redis-4:6379 redis-5:6379 redis-6:6379 \
  --cluster-replicas 1

# Redis configuration for production
echo "maxmemory 2gb" >> redis.conf
echo "maxmemory-policy allkeys-lru" >> redis.conf
echo "save 900 1" >> redis.conf
echo "save 300 10" >> redis.conf
```

## 🛠 Production Maintenance

### 1. Backup Procedures
```bash
# Database backup
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME \
  --no-password --clean --if-exists \
  --format=custom --compress=9 \
  > backup_$(date +%Y%m%d_%H%M%S).dump

# Automated backup script
#!/bin/bash
BACKUP_DIR="/backups/helixagent"
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR

pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME \
  --no-password --format=custom --compress=9 \
  > $BACKUP_DIR/helixagent_$DATE.dump

# Keep last 7 days of backups
find $BACKUP_DIR -name "*.dump" -mtime +7 -delete
```

### 2. Log Management
```bash
# Configure log rotation
cat > /etc/logrotate.d/helixagent << EOF
/var/log/helixagent/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 0644 helixagent helixagent
    postrotate
        docker-compose restart helixagent
    endscript
}
EOF

# Log aggregation with ELK
docker run -d \
  --name filebeat \
  -v /var/log/helixagent:/var/log/helixagent \
  -v ./filebeat.yml:/usr/share/filebeat/filebeat.yml \
  docker.elastic.co/beats/filebeat:8.5.0
```

### 3. Health Monitoring
```bash
# Production health check script
#!/bin/bash
HEALTH_ENDPOINT="https://api.yourdomain.com/health"
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" $HEALTH_ENDPOINT)

if [ $RESPONSE -eq 200 ]; then
    echo "✅ HelixAgent is healthy"
    exit 0
else
    echo "❌ HelixAgent is unhealthy (HTTP $RESPONSE)"
    # Send alert
    curl -X POST "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK" \
      -H 'Content-type: application/json' \
      --data "{\"text\":\"🚨 HelixAgent health check failed: HTTP $RESPONSE\"}"
    exit 1
fi
```

## 🚨 Production Troubleshooting

### 1. Common Issues
```bash
# High memory usage
docker stats --no-stream | grep helixagent
kubectl top pods -n helixagent

# Database connection issues
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT count(*) FROM pg_stat_activity;"

# LLM provider failures
curl -H "Authorization: Bearer $DEEPSEEK_API_KEY" \
  https://api.deepseek.com/v1/models

# Plugin issues
ls -la /app/plugins/
docker exec helixagent ls -la /app/plugins/
```

### 2. Performance Tuning
```bash
# Go performance profiling
curl "http://localhost:7061/debug/pprof/profile" > cpu.prof
curl "http://localhost:7061/debug/pprof/heap" > heap.prof

# Database performance analysis
SELECT query, calls, total_time, mean_time 
FROM pg_stat_statements 
ORDER BY total_time DESC 
LIMIT 10;

# Redis performance
redis-cli --latency-history
redis-cli info memory
```

### 3. Incident Response
```bash
# Immediate response checklist
1. Check service status: docker-compose ps
2. Review logs: docker-compose logs helixagent
3. Verify external services: curl $DB_HOST:5432
4. Check metrics: http://localhost:3000/d/helixagent-dashboard
5. Notify team: slack://alerts-channel

# Recovery procedures
docker-compose restart helixagent
kubectl rollout restart deployment/helixagent -n helixagent
```

## 📈 Production Metrics

### 1. Key Performance Indicators (KPIs)
- **Request Rate**: Target >1000 req/min
- **Response Time**: P95 <2s, P99 <5s  
- **Error Rate**: <1% of total requests
- **Uptime**: >99.9% availability
- **Provider Success Rate**: >95% per provider
- **Memory Usage**: <2GB per instance
- **CPU Usage**: <70% average utilization

### 2. Alert Thresholds
```yaml
Critical Alerts:
  - Service down > 1 minute
  - Error rate > 5% 
  - Response time P95 > 5s
  - Database connections > 80%
  - Memory usage > 90%

Warning Alerts:
  - Error rate > 1%
  - Response time P95 > 2s
  - CPU usage > 80%
  - Provider response time > 10s
```

---

This production guide provides everything needed to deploy HelixAgent at scale with proper monitoring, security, and maintenance procedures.