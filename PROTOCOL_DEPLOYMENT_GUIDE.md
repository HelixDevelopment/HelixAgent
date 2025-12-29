# SuperAgent Advanced Protocol Enhancement - Deployment Guide

## Overview

SuperAgent has been enhanced with comprehensive protocol support, advanced caching, security, monitoring, and performance optimizations. This guide covers deployment, configuration, and usage of the enhanced system.

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   REST API      │────│  Unified Manager │────│ Protocol Clients│
│   Endpoints     │    │  (Orchestration) │    │  (MCP, LSP,    │
└─────────────────┘    └──────────────────┘    │   ACP, etc.)   │
         │                       │            └─────────────────┘
         ▼                       ▼                       │
┌─────────────────┐    ┌──────────────────┐            │
│   Security      │────│   Monitoring     │◄───────────┘
│   & Auth        │    │   & Alerting     │
└─────────────────┘    └──────────────────┘
         │                       │
         ▼                       ▼
┌─────────────────┐    ┌──────────────────┐
│   Caching       │    │   Rate Limiting  │
│   (Redis)       │    │                  │
└─────────────────┘    └──────────────────┘
```

## Prerequisites

### System Requirements
- Go 1.21+
- Redis (optional, for advanced caching)
- PostgreSQL (for data persistence)
- 2GB RAM minimum
- Linux/macOS/Windows

### Dependencies
```bash
# Install Go dependencies
go mod tidy
go mod download

# Optional: Install Redis for advanced caching
# macOS: brew install redis
# Ubuntu: sudo apt install redis-server
# CentOS: sudo yum install redis
```

## Quick Start Deployment

### 1. Clone and Build
```bash
git clone <repository-url>
cd superagent
go build -o superagent cmd/superagent/main.go
```

### 2. Configuration
Create `config.yaml`:
```yaml
server:
  port: 8080
  host: "0.0.0.0"

database:
  url: "postgres://user:password@localhost/superagent?sslmode=disable"

redis:
  enabled: true
  url: "redis://localhost:6379"

protocols:
  enabled: true
  security:
    enabled: true
    jwt_secret: "your-secret-key"
  caching:
    enabled: true
    ttl: "1h"
    max_size: 1000
  monitoring:
    enabled: true
    alert_rules:
      - name: "High Error Rate"
        protocol: "mcp"
        condition: "error_rate_above"
        threshold: 0.1
        severity: "error"
  rate_limiting:
    enabled: true
    requests_per_minute: 100

# Protocol-specific configurations
mcp:
  enabled: true
  servers:
    - name: "filesystem-tools"
      command: ["node", "/path/to/mcp-filesystem/dist/index.js"]
      args: []
    - name: "web-scraper"
      command: ["python", "/path/to/web-scraper/main.py"]
      args: ["--port", "3001"]

lsp:
  enabled: true
  servers:
    - name: "typescript-lsp"
      language: "typescript"
      command: "typescript-language-server"
      args: ["--stdio"]

acp:
  enabled: true
  servers:
    - name: "opencode-agent"
      url: "ws://localhost:8080/agent"
      capabilities: ["code_execution", "file_operations"]

embeddings:
  enabled: true
  provider: "openai"
  model: "text-embedding-ada-002"
  api_key: "${OPENAI_API_KEY}"
```

### 3. Environment Variables
```bash
export DATABASE_URL="postgres://user:password@localhost/superagent?sslmode=disable"
export REDIS_URL="redis://localhost:6379"
export JWT_SECRET="your-super-secret-jwt-key"
export OPENAI_API_KEY="sk-your-openai-key"
export LOG_LEVEL="info"
```

### 4. Database Setup
```bash
# Create database
createdb superagent

# Run migrations
go run cmd/superagent/main.go --migrate

# Optional: Load sample data
go run cmd/superagent/main.go --seed
```

### 5. Start the Server
```bash
./superagent --config config.yaml
```

## API Usage Examples

### Authentication
```bash
# Get API key (admin key created during initialization)
curl http://localhost:8080/v1/security/keys

# Use API key in requests
curl -H "Authorization: Bearer sk-admin-key" \
     http://localhost:8080/v1/protocols/servers
```

### Protocol Operations

#### Execute MCP Tool
```bash
curl -X POST http://localhost:8080/v1/protocols/execute \
  -H "Authorization: Bearer sk-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "protocolType": "mcp",
    "serverId": "filesystem-tools",
    "toolName": "read_file",
    "arguments": {
      "path": "/etc/hosts"
    }
  }'
```

#### Generate Embeddings
```bash
curl -X POST http://localhost:8080/v1/embeddings/generate \
  -H "Authorization: Bearer sk-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Hello, world! This is a test document for embedding."
  }'
```

#### Get Monitoring Metrics
```bash
curl http://localhost:8080/v1/monitoring/metrics \
  -H "Authorization: Bearer sk-admin-key"
```

## Docker Deployment

### Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o superagent cmd/superagent/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/superagent .
COPY --from=builder /app/config.yaml .

EXPOSE 8080
CMD ["./superagent", "--config", "config.yaml"]
```

### Docker Compose
```yaml
version: '3.8'

services:
  superagent:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://user:password@postgres/superagent?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - JWT_SECRET=your-secret-key
    depends_on:
      - postgres
      - redis
    volumes:
      - ./config.yaml:/root/config.yaml

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=superagent
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

### Deploy with Docker Compose
```bash
docker-compose up -d
```

## Kubernetes Deployment

### Deployment Manifest
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: superagent
spec:
  replicas: 3
  selector:
    matchLabels:
      app: superagent
  template:
    metadata:
      labels:
        app: superagent
    spec:
      containers:
      - name: superagent
        image: your-registry/superagent:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: superagent-secrets
              key: database-url
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: superagent-secrets
              key: redis-url
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: superagent-secrets
              key: jwt-secret
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /v1/monitoring/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /v1/monitoring/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Service Manifest
```yaml
apiVersion: v1
kind: Service
metadata:
  name: superagent-service
spec:
  selector:
    app: superagent
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

### Ingress (Optional)
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: superagent-ingress
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - api.superagent.example.com
    secretName: superagent-tls
  rules:
  - host: api.superagent.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: superagent-service
            port:
              number: 80
```

## Monitoring and Observability

### Prometheus Metrics
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'superagent'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/v1/monitoring/metrics'
```

### Grafana Dashboard
Import the provided dashboard JSON or create panels for:
- Protocol request rates
- Error rates by protocol
- Cache hit/miss ratios
- Response latency percentiles
- Active connections
- Resource usage (CPU, Memory)

### Alert Manager Rules
```yaml
groups:
  - name: superagent
    rules:
      - alert: HighErrorRate
        expr: rate(protocol_requests_total{status="error"}[5m]) / rate(protocol_requests_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }}% for protocol {{ $labels.protocol }}"

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(protocol_request_duration_seconds_bucket[5m])) > 5
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "95th percentile latency is {{ $value }}s"
```

## Security Configuration

### API Key Management
```bash
# Create a new API key
curl -X POST http://localhost:8080/v1/security/keys \
  -H "Authorization: Bearer sk-admin-key" \
  -d '{"name":"client-app","permissions":["mcp:read","embedding:execute"]}'

# List API keys
curl http://localhost:8080/v1/security/keys \
  -H "Authorization: Bearer sk-admin-key"

# Revoke API key
curl -X POST http://localhost:8080/v1/security/revoke \
  -H "Authorization: Bearer sk-admin-key" \
  -d '{"key":"sk-revoke-this-key"}'
```

### Rate Limiting Configuration
```yaml
# Configure rate limits
rate_limiting:
  enabled: true
  global_limit: 1000  # requests per minute globally
  per_client_limit: 100  # requests per minute per API key
  burst_limit: 50  # burst allowance
```

### CORS Configuration
```yaml
cors:
  enabled: true
  allowed_origins:
    - "https://yourapp.com"
    - "http://localhost:3000"
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
  allowed_headers:
    - "Authorization"
    - "Content-Type"
  allow_credentials: true
```

## Performance Tuning

### Cache Configuration
```yaml
caching:
  enabled: true
  redis_url: "redis://cluster:6379"
  ttl: "30m"
  max_size: 10000
  compression: true
  invalidation:
    enabled: true
    patterns:
      - "protocol:*:servers"
      - "protocol:*:tools:*"
```

### Connection Pooling
```yaml
protocols:
  mcp:
    connection_pool:
      max_connections: 10
      max_idle_time: "5m"
      health_check_interval: "30s"
  acp:
    connection_pool:
      max_connections: 5
      timeout: "10s"
```

### Resource Limits
```yaml
resources:
  max_memory: "2GB"
  max_cpu: "4"
  max_concurrent_requests: 1000
  request_timeout: "30s"
  shutdown_timeout: "10s"
```

## Troubleshooting

### Common Issues

#### Connection Refused
```
Error: dial tcp [::1]:8080: connect: connection refused
```
**Solution**: Check if the service is running and ports are open.

#### Authentication Failed
```
Error: invalid API key
```
**Solution**: Verify API key is correct and not expired.

#### Protocol Server Unavailable
```
Error: server not connected
```
**Solution**: Check protocol server configuration and connectivity.

#### High Latency
```
Warning: Response time > 5s
```
**Solution**: Check cache configuration, database performance, and network latency.

### Debug Mode
```bash
# Enable debug logging
export LOG_LEVEL=debug
export PROTOCOL_DEBUG=true

# Check logs
tail -f logs/superagent.log
```

### Health Checks
```bash
# Overall health
curl http://localhost:8080/v1/monitoring/health

# Protocol-specific health
curl http://localhost:8080/v1/mcp/health
curl http://localhost:8080/v1/lsp/health
```

## Scaling Considerations

### Horizontal Scaling
- Use Kubernetes deployments with multiple replicas
- Configure load balancer for request distribution
- Implement distributed caching (Redis Cluster)
- Use database connection pooling

### Vertical Scaling
- Increase CPU and memory limits based on load
- Optimize cache size and TTL settings
- Monitor and adjust rate limiting thresholds

### Database Scaling
- Use read replicas for read-heavy workloads
- Implement connection pooling
- Optimize queries and add appropriate indexes
- Consider database sharding for very high loads

This deployment guide provides comprehensive instructions for deploying SuperAgent with all its advanced protocol features in production environments.