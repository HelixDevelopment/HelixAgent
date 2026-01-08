# HelixAgent Deployment Guide

Complete deployment instructions for HelixAgent across various platforms and environments.

## Quick Start with Docker

### Prerequisites
- Docker & Docker Compose
- 4GB RAM minimum, 8GB recommended
- Git

### Single Command Deployment

```bash
# Clone repository
git clone https://dev.helix.agent.git
cd helixagent

# Copy environment template
cp .env.example .env

# Edit configuration (API keys, database settings)
nano .env

# Start all services
make docker-full

# Verify deployment
curl http://localhost:7061/health
```

### What's Included
- HelixAgent API server
- PostgreSQL database
- Redis cache
- Prometheus monitoring
- Grafana dashboards
- Nginx load balancer

## Production Docker Deployment

### Environment Configuration

Create production environment file:

```bash
cp .env.example .env.prod
nano .env.prod
```

Essential production settings:

```bash
# Server Configuration
PORT=8080
GIN_MODE=release
LOG_LEVEL=info

# Database
DB_HOST=postgres
DB_PORT=5432
DB_USER=helixagent_prod
DB_PASSWORD=your_secure_password
DB_NAME=helixagent_prod

# Redis
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=your_redis_password

# API Keys (configure at least one provider)
CLAUDE_API_KEY=sk-ant-api03-...
DEEPSEEK_API_KEY=sk-...
OPENROUTER_API_KEY=sk-or-v1-...

# Security
JWT_SECRET=your_super_secure_jwt_secret_64_chars_minimum
API_KEY_SECRET=your_api_key_secret

# Monitoring
PROMETHEUS_ENABLED=true
GRAFANA_ENABLED=true
```

### Production Docker Compose

```yaml
version: '3.8'

services:
  helixagent:
    image: helixagent/helixagent:latest
    environment:
      - GIN_MODE=release
      - LOG_LEVEL=info
    env_file:
      - .env.prod
    ports:
      - "8080:7061"
    depends_on:
      - postgres
      - redis
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:7061/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: helixagent_prod
      POSTGRES_USER: helixagent_prod
      POSTGRES_PASSWORD: your_secure_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U helixagent_prod"]
      interval: 30s
      timeout: 10s
      retries: 3

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass your_redis_password
    volumes:
      - redis_data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 30s
      timeout: 10s
      retries: 3

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
    ports:
      - "9090:9090"
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    environment:
      GF_SECURITY_ADMIN_PASSWORD: your_grafana_password
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/grafana/dashboards:/var/lib/grafana/dashboards
      - ./monitoring/grafana/provisioning:/etc/grafana/provisioning
    ports:
      - "3000:3000"
    restart: unless-stopped

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf
      - ./nginx/ssl:/etc/nginx/ssl
    depends_on:
      - helixagent
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
  prometheus_data:
  grafana_data:
```

## Kubernetes Deployment

### Prerequisites
- Kubernetes cluster (1.19+)
- kubectl configured
- Helm 3.x
- 8GB RAM minimum per node

### Using Helm Chart

```bash
# Add HelixAgent Helm repository
helm repo add helixagent https://charts.helixagent.ai
helm repo update

# Install with default configuration
helm install helixagent helixagent/helixagent

# Install with custom values
helm install helixagent helixagent/helixagent \
  --set helixagent.apiKey="your-api-key" \
  --set postgresql.auth.password="your-db-password" \
  --set redis.auth.password="your-redis-password"
```

### Manual Kubernetes Deployment

Create namespace:
```bash
kubectl create namespace helixagent
```

Apply configurations:
```bash
kubectl apply -f deploy/kubernetes/
```

Monitor deployment:
```bash
kubectl get pods -n helixagent
kubectl logs -f deployment/helixagent -n helixagent
```

### Kubernetes Manifests Structure

```
deploy/kubernetes/
├── namespace.yaml
├── configmap.yaml
├── secret.yaml
├── database/
│   ├── postgresql-deployment.yaml
│   └── postgresql-service.yaml
├── cache/
│   ├── redis-deployment.yaml
│   └── redis-service.yaml
├── monitoring/
│   ├── prometheus-deployment.yaml
│   ├── prometheus-service.yaml
│   ├── grafana-deployment.yaml
│   └── grafana-service.yaml
├── api/
│   ├── helixagent-deployment.yaml
│   ├── helixagent-service.yaml
│   └── helixagent-ingress.yaml
└── rbac/
    ├── serviceaccount.yaml
    ├── clusterrole.yaml
    └── clusterrolebinding.yaml
```

## Cloud Deployments

### AWS ECS Fargate

#### Using AWS CLI

```bash
# Create cluster
aws ecs create-cluster --cluster-name helixagent-cluster

# Register task definition
aws ecs register-task-definition --cli-input-json file://aws/task-definition.json

# Create service
aws ecs create-service \
  --cluster helixagent-cluster \
  --service-name helixagent-service \
  --task-definition helixagent-task \
  --desired-count 2 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[subnet-12345,subnet-67890],securityGroups=[sg-12345],assignPublicIp=ENABLED}"
```

#### Using AWS CDK

```typescript
import * as cdk from 'aws-cdk-lib';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as ec2 from 'aws-cdk-lib/aws-ec2';

export class HelixAgentStack extends cdk.Stack {
  constructor(scope: cdk.App, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // VPC
    const vpc = new ec2.Vpc(this, 'HelixAgentVpc', {
      maxAzs: 2,
    });

    // ECS Cluster
    const cluster = new ecs.Cluster(this, 'HelixAgentCluster', {
      vpc,
    });

    // Task Definition
    const taskDefinition = new ecs.FargateTaskDefinition(this, 'HelixAgentTask', {
      memoryLimitMiB: 2048,
      cpu: 1024,
    });

    // Container
    taskDefinition.addContainer('HelixAgentContainer', {
      image: ecs.ContainerImage.fromRegistry('helixagent/helixagent:latest'),
      environment: {
        GIN_MODE: 'release',
        // Add other environment variables
      },
      logging: ecs.LogDrivers.awsLogs({ streamPrefix: 'helixagent' }),
    });

    // Service
    new ecs.FargateService(this, 'HelixAgentService', {
      cluster,
      taskDefinition,
      desiredCount: 2,
    });
  }
}
```

### Google Cloud Run

```bash
# Build and push container
gcloud builds submit --tag gcr.io/YOUR_PROJECT/helixagent

# Deploy to Cloud Run
gcloud run deploy helixagent \
  --image gcr.io/YOUR_PROJECT/helixagent \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars "GIN_MODE=release" \
  --memory 2Gi \
  --cpu 1 \
  --max-instances 10 \
  --concurrency 80
```

### Azure Container Instances

```bash
# Create resource group
az group create --name helixagent-rg --location eastus

# Create container instance
az container create \
  --resource-group helixagent-rg \
  --name helixagent-container \
  --image helixagent/helixagent:latest \
  --ports 8080 \
  --environment-variables GIN_MODE=release \
  --memory 2 \
  --cpu 1 \
  --dns-name-label helixagent-api \
  --os-type Linux
```

## On-Premise Deployment

### Bare Metal Linux

#### System Requirements
- Ubuntu 20.04+ or RHEL 8+
- 8GB RAM minimum
- 4 CPU cores minimum
- 50GB disk space

#### Installation Steps

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/download/v2.20.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Clone repository
git clone https://dev.helix.agent.git
cd helixagent

# Configure environment
cp .env.example .env
nano .env  # Edit configuration

# Start services
docker-compose up -d

# Enable systemd service
sudo cp deploy/systemd/helixagent.service /etc/systemd/system/
sudo systemctl enable helixagent
sudo systemctl start helixagent
```

### Red Hat Enterprise Linux

```bash
# Install Podman (alternative to Docker)
sudo dnf install -y podman podman-compose

# Clone and configure
git clone https://dev.helix.agent.git
cd helixagent

# Use Podman instead of Docker
export DOCKER=podman
export COMPOSE=podman-compose

# Start services
podman-compose up -d
```

## High Availability Setup

### Load Balancing with Nginx

```nginx
upstream helixagent_backend {
    least_conn;
    server helixagent-1:7061;
    server helixagent-2:7061;
    server helixagent-3:7061;
}

server {
    listen 80;
    server_name api.helixagent.ai;

    location / {
        proxy_pass http://helixagent_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeout settings
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://helixagent_backend;
        access_log off;
    }
}
```

### Database Clustering

#### PostgreSQL Streaming Replication

Master configuration (`postgresql.conf`):
```ini
wal_level = replica
max_wal_senders = 3
wal_keep_segments = 64
```

Standby configuration:
```bash
# On standby server
pg_basebackup -h master-server -D /var/lib/postgresql/data -U replicator -P -v -R
systemctl restart postgresql
```

### Redis Clustering

```yaml
version: '3.8'
services:
  redis-1:
    image: redis:7-alpine
    command: redis-server /etc/redis/redis.conf
    volumes:
      - ./redis/redis-1.conf:/etc/redis/redis.conf
      - redis-1-data:/data
    ports:
      - "6379:6379"

  redis-2:
    image: redis:7-alpine
    command: redis-server /etc/redis/redis.conf
    volumes:
      - ./redis/redis-2.conf:/etc/redis/redis.conf
      - redis-2-data:/data
    ports:
      - "6380:6379"

  redis-3:
    image: redis:7-alpine
    command: redis-server /etc/redis/redis.conf
    volumes:
      - ./redis/redis-3.conf:/etc/redis/redis.conf
      - redis-3-data:/data
    ports:
      - "6381:6379"
```

## Monitoring and Observability

### Prometheus Configuration

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

scrape_configs:
  - job_name: 'helixagent'
    static_configs:
      - targets: ['helixagent:7061']
    scrape_interval: 5s
    metrics_path: '/metrics'

  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres:9187']
    scrape_interval: 10s

  - job_name: 'redis'
    static_configs:
      - targets: ['redis:9121']
    scrape_interval: 30s
```

### Grafana Dashboards

Pre-configured dashboards available:
- HelixAgent API Performance
- Database Metrics
- Cache Performance
- Provider Health Status
- AI Debate Analytics

```bash
# Access Grafana
open http://localhost:3000
# Default credentials: admin/admin
```

## Security Hardening

### SSL/TLS Configuration

```nginx
server {
    listen 443 ssl http2;
    server_name api.helixagent.ai;

    ssl_certificate /etc/nginx/ssl/helixagent.crt;
    ssl_certificate_key /etc/nginx/ssl/helixagent.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;

    location / {
        proxy_pass http://helixagent_backend;
        # ... other proxy settings
    }
}
```

### Firewall Configuration

```bash
# UFW (Ubuntu)
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw --force enable

# firewalld (RHEL)
sudo firewall-cmd --permanent --add-port=80/tcp
sudo firewall-cmd --permanent --add-port=443/tcp
sudo firewall-cmd --permanent --add-port=22/tcp
sudo firewall-cmd --reload
```

### Secret Management

Using Docker secrets:
```yaml
version: '3.8'
secrets:
  api_key:
    file: ./secrets/api_key.txt
  db_password:
    file: ./secrets/db_password.txt

services:
  helixagent:
    secrets:
      - api_key
      - db_password
```

## Backup and Recovery

### Database Backup

```bash
# Daily PostgreSQL backup
0 2 * * * docker exec helixagent-postgres-1 pg_dump -U helixagent helixagent_db > /backup/helixagent_$(date +\%Y\%m\%d).sql

# Redis backup
0 3 * * * docker exec helixagent-redis-1 redis-cli --rdb /backup/redis_$(date +\%Y\%m\%d).rdb
```

### Automated Backups with Cron

```bash
# Install backup script
sudo cp scripts/backup.sh /usr/local/bin/helixagent-backup
sudo chmod +x /usr/local/bin/helixagent-backup

# Add to crontab
crontab -e
# Add: 0 2 * * * /usr/local/bin/helixagent-backup
```

## Troubleshooting

### Common Issues

#### Container Won't Start
```bash
# Check logs
docker-compose logs helixagent

# Check resource usage
docker stats

# Verify configuration
docker-compose config
```

#### Database Connection Issues
```bash
# Test database connectivity
docker-compose exec postgres pg_isready -U helixagent -d helixagent_db

# Check database logs
docker-compose logs postgres
```

#### High Memory Usage
```bash
# Monitor memory usage
docker stats

# Adjust container limits
helixagent:
  deploy:
    resources:
      limits:
        memory: 2G
      reservations:
        memory: 1G
```

#### Slow Response Times
```bash
# Check Redis connectivity
docker-compose exec redis redis-cli ping

# Monitor API response times
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:7061/health
```

## Performance Tuning

### Application Settings

```bash
# Environment variables for performance
GIN_MODE=release
MAX_WORKERS=10
REQUEST_TIMEOUT=30
CONNECTION_POOL_SIZE=20
CACHE_TTL=3600
```

### Database Optimization

```sql
-- Create indexes for better performance
CREATE INDEX idx_llm_requests_model ON llm_requests(model);
CREATE INDEX idx_llm_requests_created_at ON llm_requests(created_at);
CREATE INDEX idx_debates_status ON debates(status);

-- Analyze tables
ANALYZE llm_requests;
ANALYZE debates;
```

### Monitoring Queries

```sql
-- Slow queries
SELECT pid, now() - pg_stat_activity.query_start AS duration, query
FROM pg_stat_activity
WHERE state = 'active'
ORDER BY duration DESC;

-- Cache hit ratio
SELECT sum(blks_hit) * 100 / (sum(blks_hit) + sum(blks_read)) AS cache_hit_ratio
FROM pg_stat_database;
```

## Upgrade Procedures

### Rolling Updates

```bash
# Update images
docker-compose pull

# Rolling restart
docker-compose up -d --no-deps helixagent

# Verify health
curl http://localhost:7061/health
```

### Zero-Downtime Deployment

```bash
# Deploy new version alongside old
docker-compose up -d --scale helixagent=2

# Wait for new instances to be healthy
sleep 30

# Remove old instances
docker-compose up -d --scale helixagent=1
```

## Support and Resources

### Documentation
- [API Documentation](./api-documentation.md)
- [Configuration Guide](./configuration.md)
- [Troubleshooting Guide](./troubleshooting.md)

### Community Support
- [GitHub Issues](https://dev.helix.agent/issues)
- [Discussions](https://dev.helix.agent/discussions)

### Enterprise Support
- Email: enterprise@helixagent.ai
- SLA: 99.9% uptime guarantee
- Phone: +1 (555) 123-4567

---

**Deployment Checklist:**
- [ ] Environment configured
- [ ] API keys set
- [ ] Database initialized
- [ ] SSL certificates installed
- [ ] Firewall configured
- [ ] Monitoring enabled
- [ ] Backups scheduled
- [ ] Health checks passing