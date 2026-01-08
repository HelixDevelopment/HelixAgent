# Deployment Guide

This guide covers deploying HelixAgent in various environments.

## 🚀 Quick Start

### Docker Deployment (Recommended)
```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Configure environment
cp .env.example .env
# Edit .env with your configuration

# Start full stack
make docker-full
```

### Kubernetes Deployment
```bash
# Apply Kubernetes manifests
kubectl apply -f deploy/kubernetes/

# Check deployment status
kubectl get pods -n helixagent

# Port forward for local access
kubectl port-forward svc/helixagent-api 8080:7061 -n helixagent
```

## 🏗 Production Deployment

### Prerequisites
- Docker 20.10+
- Kubernetes 1.24+ (for K8s deployment)
- PostgreSQL 14+ (if using external DB)
- Redis 6+ (if using external cache)

### Environment Configuration
Copy `.env.example` to `.env.prod` and configure:

#### Required Variables
```bash
# Server
PORT=8080
HELIXAGENT_API_KEY=your-production-api-key
JWT_SECRET=your-production-jwt-secret-32-chars

# Database
DB_HOST=your-db-host
DB_USER=helixagent
DB_PASSWORD=your-db-password
DB_NAME=helixagent_db
```

#### Optional Variables
```bash
# LLM Providers (Ollama is free)
OLLAMA_ENABLED=true
OLLAMA_BASE_URL=http://ollama:11434

# Paid Providers (optional)
CLAUDE_API_KEY=sk-your-claude-key
DEEPSEEK_API_KEY=sk-your-deepseek-key
GEMINI_API_KEY=your-gemini-key

# Monitoring
METRICS_ENABLED=true
GRAFANA_PASSWORD=your-grafana-password
```

### Docker Compose Production
```bash
# Production deployment
docker-compose --profile prod up -d

# With custom environment file
docker-compose --profile prod --env-file .env.prod up -d

# Scale services
docker-compose --profile prod up -d --scale helixagent=3
```

### Kubernetes Production
```yaml
# deploy/kubernetes/helixagent-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helixagent
  namespace: helixagent
spec:
  replicas: 3
  selector:
    matchLabels:
      app: helixagent
  template:
    metadata:
      labels:
        app: helixagent
    spec:
      containers:
      - name: helixagent
        image: helixagent:latest
        ports:
        - containerPort: 8080
        env:
        - name: PORT
          value: "8080"
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: helixagent-secrets
              key: db-host
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## 🔒 Security Configuration

### TLS/SSL
```bash
# Enable HTTPS
export TLS_ENABLED=true
export TLS_CERT_PATH=/path/to/cert.pem
export TLS_KEY_PATH=/path/to/key.pem

# Or use Let's Encrypt
export CERT_MANAGER=true
```

### Network Security
```bash
# Configure trusted origins
export CORS_ORIGINS="https://yourdomain.com,https://app.yourdomain.com"

# Enable request validation
export REQUEST_VALIDATION=true

# Rate limiting
export RATE_LIMIT_REQUESTS=100
export RATE_LIMIT_WINDOW=1m
```

### Secrets Management
```bash
# Using Kubernetes secrets
kubectl create secret generic helixagent-secrets \
  --from-literal=db-host=your-db-host \
  --from-literal=db-password=your-db-password \
  --from-literal=jwt-secret=your-jwt-secret

# Using Docker secrets
docker secret create db-password your-db-password
docker-compose --profile prod up -d --use-secrets
```

## 📊 Monitoring & Observability

### Prometheus Configuration
```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'helixagent'
    static_configs:
      - targets: ['helixagent:7061']
    metrics_path: '/metrics'
    scrape_interval: 5s
```

### Grafana Dashboards
```bash
# Import pre-configured dashboards
curl -X POST \
  -H "Authorization: Bearer admin:admin123" \
  -H "Content-Type: application/json" \
  -d @monitoring/grafana-dashboard.json \
  http://localhost:3000/api/dashboards/db
```

### Log Aggregation
```yaml
# filebeat.yml
filebeat.inputs:
- type: docker
  containers.ids:
  - helixagent
  processors:
  - add_docker_metadata:
      match_fields: ["container.name"]
  fields:
    service: helixagent
    environment: production

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
```

## 🔄 Scaling & High Availability

### Horizontal Scaling
```bash
# Docker Compose scaling
docker-compose --profile prod up -d --scale helixagent=5

# Kubernetes HPA
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: helixagent-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: helixagent
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Database Scaling
```sql
-- PostgreSQL connection pooling
ALTER SYSTEM SET max_connections = 200;
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';

-- Redis clustering
redis-cli --cluster create 127.0.0.1:7000 127.0.0.1:7001 127.0.0.1:7002
```

## 🌐 Cloud Deployment

### AWS ECS
```bash
# Build and push to ECR
aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-west-2.amazonaws.com
docker build -t helixagent:latest .
docker tag helixagent:latest 123456789012.dkr.ecr.us-west-2.amazonaws.com/helixagent:latest
docker push 123456789012.dkr.ecr.us-west-2.amazonaws.com/helixagent:latest

# Deploy to ECS
aws ecs create-cluster --cluster-name helixagent-cluster
aws ecs register-task-definition --cli-input-json file://task-definition.json
aws ecs create-service --cluster helixagent-cluster --service-name helixagent --task-definition helixagent:1
```

### Google Cloud Run
```bash
# Build and push to GCR
gcloud auth configure-docker
gcloud builds submit --tag gcr.io/PROJECT-ID/helixagent:latest
docker push gcr.io/PROJECT-ID/helixagent:latest

# Deploy to Cloud Run
gcloud run deploy helixagent \
  --image gcr.io/PROJECT-ID/helixagent:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --max-instances 1000
```

### Azure Container Instances
```bash
# Build and push to ACR
az acr login --name helixagent-registry
docker build -t helixagent-registry.azurecr.io/helixagent:latest .
docker push helixagent-registry.azurecr.io/helixagent:latest

# Deploy to ACI
az container create \
  --resource-group helixagent-rg \
  --name helixagent \
  --image helixagent-registry.azurecr.io/helixagent:latest \
  --cpu 1 \
  --memory 2 \
  --ports 8080
```

## 🔧 Maintenance

### Health Checks
```bash
# Comprehensive health check
curl http://localhost:7061/v1/health | jq '.'

# Provider-specific health
curl http://localhost:7061/v1/providers/health
```

### Updates & Rollbacks
```bash
# Zero-downtime deployment
docker-compose --profile prod up -d --no-deps helixagent

# Rollback to previous version
docker tag helixagent:previous helixagent:latest
docker-compose --profile prod up -d
```

### Backup & Recovery
```bash
# Database backup
docker-compose exec postgres pg_dump -U helixagent helixagent_db > backup_$(date +%Y%m%d_%H%M%S).sql

# Redis backup
docker-compose exec redis redis-cli BGSAVE

# Configuration backup
kubectl get configmap helixagent-config -o yaml > config-backup.yaml
```

## 🚨 Troubleshooting

### Common Issues

#### Container Fails to Start
```bash
# Check logs
docker-compose logs helixagent

# Verify environment
docker-compose exec helixagent env | grep -E "(DB_HOST|REDIS_HOST)"

# Check resource usage
docker stats helixagent
```

#### Database Connection Issues
```bash
# Test database connectivity
docker-compose exec postgres pg_isready -U helixagent -d helixagent_db

# Check network connectivity
docker-compose exec helixagent ping postgres

# Review database logs
docker-compose logs postgres | tail -100
```

#### Performance Issues
```bash
# Monitor resource usage
docker stats --no-stream

# Check response times
curl -w "@{time_total}\n" -o /dev/null -s http://localhost:7061/health

# Profile application
docker-compose exec helixagent ./helixagent -cpuprofile=cpu.prof -memprofile=mem.prof
```

### Debug Commands
```bash
# Enable debug mode
export LOG_LEVEL=debug
export GIN_MODE=debug
docker-compose --profile dev up -d

# View detailed logs
docker-compose logs -f --tail=100 helixagent

# Interactive debugging
docker-compose exec helixagent /bin/sh
```

---

For more detailed information, see the [HelixAgent Documentation](https://docs.helixagent.ai).