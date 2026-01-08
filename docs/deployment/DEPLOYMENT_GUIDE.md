# HelixAgent Deployment Guide

This guide covers production deployment of HelixAgent across different environments.

## Prerequisites

- Docker 24.0+ and Docker Compose V2
- Kubernetes 1.28+ (for K8s deployment)
- PostgreSQL 15+
- Redis 7+
- At least one LLM provider configured (Ollama for free testing)

## Quick Start

### Local Docker Deployment

```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Copy and configure environment
cp .env.example .env
# Edit .env with your API keys and configuration

# Start all services
docker-compose --profile full up -d

# Verify deployment
curl http://localhost:7061/health
```

### Environment Configuration

Create a `.env` file with the following configuration:

```bash
# Server Configuration
PORT=8080
GIN_MODE=release
JWT_SECRET=your-secure-jwt-secret-min-32-chars

# Database Configuration
DB_HOST=postgres
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=secure-database-password
DB_NAME=helixagent_db
DB_SSL_MODE=require

# Redis Configuration
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=secure-redis-password

# LLM Provider Configuration (at least one required)
OLLAMA_ENABLED=true
OLLAMA_BASE_URL=http://ollama:11434
OLLAMA_MODEL=llama2

# Optional Paid Providers
CLAUDE_API_KEY=sk-ant-your-key
DEEPSEEK_API_KEY=sk-your-key
GEMINI_API_KEY=your-key
OPENROUTER_API_KEY=your-key
```

## Docker Compose Profiles

### Available Profiles

| Profile | Description | Services |
|---------|-------------|----------|
| `core` | Core services only | HelixAgent, PostgreSQL, Redis |
| `ai` | Core + AI services | Core + Ollama |
| `monitoring` | Core + monitoring | Core + Prometheus, Grafana |
| `full` | All services | Core + AI + Monitoring + ChromaDB |
| `optimization` | LLM optimization | Core + SGLang, LangChain, LlamaIndex |

### Starting Profiles

```bash
# Core services
docker-compose --profile core up -d

# Full stack with monitoring
docker-compose --profile full up -d

# AI services only
docker-compose --profile ai up -d

# Stop all services
docker-compose --profile full down

# View logs
docker-compose logs -f helixagent
```

## Kubernetes Deployment

### Prerequisites

- kubectl configured for your cluster
- Kustomize installed
- Secrets created for database and API credentials

### Staging Deployment

```bash
# Create namespace
kubectl create namespace helixagent-staging

# Apply staging configuration
kustomize build k8s/staging | kubectl apply -f -

# Verify deployment
kubectl get pods -n helixagent-staging
kubectl get svc -n helixagent-staging
```

### Production Deployment

```bash
# Create namespace
kubectl create namespace helixagent-production

# Create secrets from GitHub Secrets or environment
kubectl create secret generic helixagent-secrets \
  --namespace=helixagent-production \
  --from-literal=db-host=your-db-host \
  --from-literal=db-user=helixagent \
  --from-literal=db-password=secure-password \
  --from-literal=db-name=helixagent_production \
  --from-literal=redis-host=your-redis-host \
  --from-literal=jwt-secret=your-jwt-secret

# Apply production configuration
kustomize build k8s/production | kubectl apply -f -

# Monitor rollout
kubectl rollout status deployment/prod-helixagent -n helixagent-production
```

### Kubernetes Resources

Production deployment includes:

- **Deployment**: 3 replicas with rolling updates
- **HorizontalPodAutoscaler**: 3-20 replicas based on CPU/memory
- **PodDisruptionBudget**: Minimum 1 pod available
- **NetworkPolicy**: Ingress/egress restrictions
- **Service**: ClusterIP for internal access
- **Ingress**: TLS-terminated external access
- **ServiceAccount**: Minimal RBAC permissions

## Cloud Provider Deployment

### AWS (EKS)

```bash
# Create EKS cluster
eksctl create cluster --name helixagent --region us-east-1

# Configure kubectl
aws eks update-kubeconfig --name helixagent --region us-east-1

# Deploy
kustomize build k8s/production | kubectl apply -f -
```

### GCP (GKE)

```bash
# Create GKE cluster
gcloud container clusters create helixagent \
  --zone us-central1-a \
  --num-nodes 3

# Get credentials
gcloud container clusters get-credentials helixagent --zone us-central1-a

# Deploy
kustomize build k8s/production | kubectl apply -f -
```

### Azure (AKS)

```bash
# Create AKS cluster
az aks create --resource-group helixagent-rg \
  --name helixagent \
  --node-count 3

# Get credentials
az aks get-credentials --resource-group helixagent-rg --name helixagent

# Deploy
kustomize build k8s/production | kubectl apply -f -
```

## Health Checks

### Application Health

```bash
# Basic health check
curl http://localhost:7061/health

# Detailed health with provider status
curl http://localhost:7061/v1/health

# Provider-specific health
curl http://localhost:7061/v1/providers/ollama/health
```

### Database Health

```bash
# PostgreSQL health
docker-compose exec postgres pg_isready -U helixagent

# Redis health
docker-compose exec redis redis-cli ping
```

## Monitoring

### Prometheus Metrics

Available at `http://localhost:9090` (or your Prometheus endpoint):

- `helixagent_requests_total` - Request counter by method/endpoint/provider
- `helixagent_response_time_seconds` - Response time histogram
- `helixagent_errors_total` - Error counter by type
- `helixagent_provider_health` - Provider health status

### Grafana Dashboards

Access at `http://localhost:3000` (default credentials: admin/admin123):

- HelixAgent Overview Dashboard
- Provider Performance Dashboard
- Error Rate Dashboard
- Resource Usage Dashboard

### Log Aggregation

```bash
# View application logs
docker-compose logs -f helixagent

# View all service logs
docker-compose logs -f

# Kubernetes logs
kubectl logs -f deployment/helixagent -n helixagent-production
```

## Scaling

### Horizontal Scaling

```bash
# Docker Compose
docker-compose up -d --scale helixagent=3

# Kubernetes
kubectl scale deployment/helixagent --replicas=5 -n helixagent-production
```

### Vertical Scaling

Update resource limits in `k8s/production/deployment-patch.yaml`:

```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "1000m"
  limits:
    memory: "4Gi"
    cpu: "4000m"
```

## Backup and Recovery

### Database Backup

```bash
# Create backup
docker-compose exec postgres pg_dump -U helixagent helixagent_db > backup.sql

# Restore from backup
docker-compose exec -T postgres psql -U helixagent helixagent_db < backup.sql
```

### Configuration Backup

```bash
# Backup secrets (Kubernetes)
kubectl get secrets -n helixagent-production -o yaml > secrets-backup.yaml
```

## Troubleshooting

### Common Issues

1. **Database Connection Failed**
   ```bash
   # Check database connectivity
   docker-compose exec helixagent nc -zv postgres 5432
   ```

2. **Provider Authentication Failed**
   ```bash
   # Verify API keys
   docker-compose exec helixagent env | grep API_KEY
   ```

3. **High Memory Usage**
   ```bash
   # Check container stats
   docker stats helixagent

   # Restart with limits
   docker-compose restart helixagent
   ```

4. **Slow Response Times**
   ```bash
   # Check provider health
   curl http://localhost:7061/v1/providers

   # View metrics
   curl http://localhost:9090/metrics | grep response_time
   ```

## Security Considerations

1. **Secrets Management**: Use Kubernetes Secrets or external secret managers
2. **Network Policies**: Restrict pod-to-pod communication
3. **TLS**: Enable TLS for all external endpoints
4. **Rate Limiting**: Configure per-user rate limits
5. **Audit Logging**: Enable request logging for compliance

## Next Steps

- [Kubernetes Deployment Details](./kubernetes-deployment.md)
- [Monitoring Setup Guide](../guides/ANALYTICS_CONFIGURATION_GUIDE.md)
- [API Documentation](../api/api-documentation.md)
- [Troubleshooting Guide](../guides/POST_IMPLEMENTATION_GUIDE.md)
