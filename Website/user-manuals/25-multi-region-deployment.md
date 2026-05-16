# User Manual 25: Multi-Region Deployment

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Multi-Region Architecture](#multi-region-architecture)
4. [Kubernetes Deployment](#kubernetes-deployment)
5. [Database Replication](#database-replication)
6. [Redis Cluster Configuration](#redis-cluster-configuration)
7. [Load Balancing](#load-balancing)
8. [DNS and Geographic Routing](#dns-and-geographic-routing)
9. [Failover Configuration](#failover-configuration)
10. [Container Distribution with Containers Module](#container-distribution-with-containers-module)
11. [Configuration per Region](#configuration-per-region)
12. [Monitoring Across Regions](#monitoring-across-regions)
13. [Deployment Procedure](#deployment-procedure)
14. [Troubleshooting](#troubleshooting)
15. [Related Resources](#related-resources)

## Overview

HelixAgent supports multi-region deployment for high availability, geographic latency reduction, and regulatory compliance. This manual covers deploying HelixAgent across multiple regions using Kubernetes, configuring database replication, setting up geographic DNS routing, and implementing automatic failover.

The Containers module (`digital.vasic.containers`) handles container orchestration and remote distribution. When `CONTAINERS_REMOTE_ENABLED=true` in `containers/.env`, HelixAgent distributes all containers to remote hosts automatically.

## Prerequisites

- Kubernetes clusters in each target region (1.28+)
- PostgreSQL 15 with logical or streaming replication capability
- Redis 7 with cluster mode or Sentinel
- DNS provider with geographic routing support (Route 53, Cloudflare, etc.)
- SSH key-based access to all remote hosts
- TLS certificates for inter-region communication
- Container registry accessible from all regions

## Multi-Region Architecture

```
                    +--------------------+
                    |  Global DNS (Geo)  |
                    |  helixagent.io     |
                    +--------+-----------+
                             |
              +--------------+--------------+
              |                             |
     +--------v--------+          +--------v--------+
     |  Region: US-East |          |  Region: EU-West |
     +--------+---------+          +--------+---------+
              |                             |
     +--------v--------+          +--------v--------+
     |  Load Balancer   |          |  Load Balancer   |
     |  (L7 / Ingress) |          |  (L7 / Ingress) |
     +--------+---------+          +--------+---------+
              |                             |
     +--------v--------+          +--------v--------+
     |  HelixAgent x3  |          |  HelixAgent x3  |
     |  (K8s Pods)     |          |  (K8s Pods)     |
     +--------+---------+          +--------+---------+
              |                             |
     +--------v--------+          +--------v--------+
     |  PostgreSQL      |          |  PostgreSQL      |
     |  (Primary)       |<========>|  (Replica)       |
     +---------+--------+          +---------+--------+
              |                             |
     +--------v--------+          +--------v--------+
     |  Redis Sentinel  |          |  Redis Sentinel  |
     +------------------+          +------------------+
```

## Kubernetes Deployment

### Namespace Setup

```bash
kubectl create namespace helixagent
kubectl config set-context --current --namespace=helixagent
```

### HelixAgent Deployment Manifest

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helixagent
  namespace: helixagent
  labels:
    app: helixagent
spec:
  replicas: 3
  selector:
    matchLabels:
      app: helixagent
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  template:
    metadata:
      labels:
        app: helixagent
    spec:
      containers:
        - name: helixagent
          image: registry.example.com/helixagent:latest
          ports:
            - containerPort: 7061
              name: http
            - containerPort: 9090
              name: metrics
          envFrom:
            - secretRef:
                name: helixagent-secrets
            - configMapRef:
                name: helixagent-config
          resources:
            requests:
              cpu: "500m"
              memory: "512Mi"
            limits:
              cpu: "2000m"
              memory: "2Gi"
          livenessProbe:
            httpGet:
              path: /v1/monitoring/status
              port: 7061
            initialDelaySeconds: 120
            periodSeconds: 30
            timeoutSeconds: 10
          readinessProbe:
            httpGet:
              path: /v1/monitoring/status
              port: 7061
            initialDelaySeconds: 60
            periodSeconds: 10
            timeoutSeconds: 5
```

### Service and Ingress

```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: helixagent
  namespace: helixagent
spec:
  selector:
    app: helixagent
  ports:
    - name: http
      port: 7061
      targetPort: 7061
    - name: metrics
      port: 9090
      targetPort: 9090
  type: ClusterIP
---
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: helixagent
  namespace: helixagent
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - us-east.helixagent.io
      secretName: helixagent-tls
  rules:
    - host: us-east.helixagent.io
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: helixagent
                port:
                  number: 7061
```

### Horizontal Pod Autoscaler

```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: helixagent
  namespace: helixagent
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
```

## Database Replication

### PostgreSQL Streaming Replication

Configure the primary (US-East):

```
# postgresql.conf on primary
wal_level = replica
max_wal_senders = 5
wal_keep_size = 1GB
synchronous_standby_names = 'eu_west_replica'
```

```
# pg_hba.conf on primary
host replication replicator eu-west-db-ip/32 scram-sha-256
```

Configure the replica (EU-West):

```bash
# Initialize replica from primary
pg_basebackup -h us-east-db-host -U replicator -D /var/lib/postgresql/data -Fp -Xs -P -R
```

The `-R` flag creates `standby.signal` and sets `primary_conninfo` automatically.

### Failover Procedure

```bash
# On the replica, promote to primary
pg_ctl promote -D /var/lib/postgresql/data

# Update HelixAgent configuration to point to the new primary
# Update DNS or connection string in the secrets
kubectl -n helixagent edit secret helixagent-secrets
```

## Redis Cluster Configuration

### Redis Sentinel for HA

```yaml
# docker-compose.redis-sentinel.yml
services:
  redis-master:
    image: redis:7-alpine
    command: redis-server --requirepass helixagent123
    ports:
      - "6379:6379"

  redis-sentinel:
    image: redis:7-alpine
    command: >
      redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./configs/redis/sentinel.conf:/etc/redis/sentinel.conf
```

Sentinel configuration:

```
# configs/redis/sentinel.conf
sentinel monitor helixagent-master redis-master 6379 2
sentinel auth-pass helixagent-master helixagent123
sentinel down-after-milliseconds helixagent-master 5000
sentinel failover-timeout helixagent-master 10000
```

## Load Balancing

### Layer 7 Load Balancing

```yaml
# NGINX configuration for HelixAgent
upstream helixagent_backend {
    least_conn;
    server helixagent-1:7061 weight=5;
    server helixagent-2:7061 weight=5;
    server helixagent-3:7061 weight=5;
}

server {
    listen 443 ssl http2;
    server_name us-east.helixagent.io;

    ssl_certificate /etc/ssl/certs/helixagent.crt;
    ssl_certificate_key /etc/ssl/private/helixagent.key;

    location / {
        proxy_pass http://helixagent_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Request-ID $request_id;
        proxy_read_timeout 120s;
        proxy_send_timeout 120s;
    }

    location /v1/chat/completions {
        proxy_pass http://helixagent_backend;
        proxy_buffering off;  # Required for SSE streaming
        proxy_cache off;
    }
}
```

### Health Check Configuration

```yaml
# Health check for load balancer
location /health {
    proxy_pass http://helixagent_backend/v1/monitoring/status;
    proxy_connect_timeout 5s;
    proxy_read_timeout 5s;
}
```

## DNS and Geographic Routing

### Route 53 (AWS)

```
helixagent.io
+-- @ A record (Geo-routed)
|   +-- US traffic -> us-east ALB
|   +-- EU traffic -> eu-west ALB
|   +-- Default   -> us-east ALB
+-- us-east CNAME us-east.elb.amazonaws.com (weight: 100)
+-- eu-west CNAME eu-west.elb.amazonaws.com (weight: 100)
```

### Cloudflare

```json
{
    "type": "A",
    "name": "helixagent.io",
    "content": "us-east-ip",
    "proxied": true,
    "data": {
        "steering_policy": "geo",
        "pools": {
            "us-east": {"origins": [{"address": "us-east-ip"}]},
            "eu-west": {"origins": [{"address": "eu-west-ip"}]}
        }
    }
}
```

### Health-Based DNS Failover

Configure DNS health checks to automatically route traffic away from unhealthy regions:

```yaml
health_check:
  endpoint: /v1/monitoring/status
  interval: 10s
  threshold: 3     # Consecutive failures before failover
  timeout: 5s
```

## Failover Configuration

### Automatic Failover

1. DNS health checks monitor each region every 10 seconds
2. After 3 consecutive failures, DNS stops routing to the failed region
3. All traffic redirects to the healthy region(s)
4. When the failed region recovers, DNS gradually restores traffic

### Manual Failover

```bash
# Activate DR for a specific region
./scripts/dr-activate.sh region=us-west-2

# Verify the failover
curl -s https://helixagent.io/v1/monitoring/status | jq .

# Failback when primary is restored
./scripts/dr-failback.sh region=us-east-1
```

## Container Distribution with Containers Module

HelixAgent's Containers module can distribute containers to remote hosts:

```bash
# containers/.env
CONTAINERS_REMOTE_ENABLED=true
CONTAINERS_REMOTE_HOST_1=us-east-host.example.com
CONTAINERS_REMOTE_HOST_1_USER=deploy
CONTAINERS_REMOTE_HOST_2=eu-west-host.example.com
CONTAINERS_REMOTE_HOST_2_USER=deploy
```

When HelixAgent boots, it reads `containers/.env` and distributes all containers to the configured remote hosts via SSH. No manual container manipulation is needed.

## Configuration per Region

### Region-Specific Environment Variables

```bash
# US-East .env additions
REGION=us-east-1
DB_HOST=us-east-db.internal
REDIS_HOST=us-east-redis.internal
OTEL_EXPORTER_OTLP_ENDPOINT=http://us-east-otel:4317

# EU-West .env additions
REGION=eu-west-1
DB_HOST=eu-west-db.internal
REDIS_HOST=eu-west-redis.internal
OTEL_EXPORTER_OTLP_ENDPOINT=http://eu-west-otel:4317
```

### Kubernetes Secrets per Region

```bash
# Create region-specific secrets
kubectl -n helixagent create secret generic helixagent-secrets \
    --from-env-file=.env.us-east

# EU-West cluster
kubectl -n helixagent create secret generic helixagent-secrets \
    --from-env-file=.env.eu-west
```

## Monitoring Across Regions

### Centralized Monitoring

Use a central Prometheus federation or Thanos to aggregate metrics from all regions:

```yaml
# prometheus-federation.yml
scrape_configs:
  - job_name: 'federate-us-east'
    honor_labels: true
    metrics_path: '/federate'
    params:
      'match[]':
        - '{job="helixagent"}'
    static_configs:
      - targets: ['prometheus-us-east:9090']
        labels:
          region: 'us-east-1'

  - job_name: 'federate-eu-west'
    honor_labels: true
    metrics_path: '/federate'
    params:
      'match[]':
        - '{job="helixagent"}'
    static_configs:
      - targets: ['prometheus-eu-west:9090']
        labels:
          region: 'eu-west-1'
```

### Cross-Region Health Dashboard

Create a Grafana dashboard showing:

- Request rate per region
- Latency comparison between regions
- Error rate per region
- Database replication lag
- Active provider count per region

## Deployment Procedure

### Step-by-Step Multi-Region Deployment

1. **Build release artifacts** (container-based):
   ```bash
   make release
   ```

2. **Push images to registry**:
   ```bash
   docker push registry.example.com/helixagent:v1.0.0
   ```

3. **Deploy to primary region (US-East)**:
   ```bash
   kubectl --context=us-east apply -f k8s/
   kubectl --context=us-east rollout status deployment/helixagent -n helixagent
   ```

4. **Verify primary region**:
   ```bash
   curl -s https://us-east.helixagent.io/v1/monitoring/status | jq .
   ```

5. **Deploy to secondary region (EU-West)**:
   ```bash
   kubectl --context=eu-west apply -f k8s/
   kubectl --context=eu-west rollout status deployment/helixagent -n helixagent
   ```

6. **Verify both regions**:
   ```bash
   curl -s https://eu-west.helixagent.io/v1/monitoring/status | jq .
   ```

7. **Enable geographic DNS routing**.

## Troubleshooting

### Database Replication Lag

**Symptom:** EU-West reads return stale data.

**Solutions:**
1. Check replication lag: `SELECT pg_last_wal_receive_lsn() - pg_last_wal_replay_lsn()`
2. Increase `wal_keep_size` on the primary
3. Check network bandwidth between regions
4. Consider asynchronous replication if strict consistency is not required

### DNS Failover Not Triggering

**Symptom:** Traffic continues to route to a failed region.

**Solutions:**
1. Verify health check configuration in the DNS provider
2. Check that the health check endpoint (`/v1/monitoring/status`) is accessible from the DNS provider's health checkers
3. Lower the health check interval or failure threshold
4. Check DNS TTL values (lower TTL enables faster failover)

### Pods Failing Readiness Probes

**Symptom:** Pods stay in "NotReady" state after deployment.

**Solutions:**
1. Increase `initialDelaySeconds` on readiness probe (HelixAgent startup takes ~2 minutes for verification)
2. Check pod logs: `kubectl logs -n helixagent deployment/helixagent`
3. Verify infrastructure connectivity (database, Redis) from the pod
4. Check resource limits are sufficient

### Split-Brain During Failover

**Symptom:** Both regions think they are primary.

**Solutions:**
1. Use fencing/STONITH to prevent split-brain
2. Implement a distributed lock (etcd, Consul) for primary election
3. Configure `synchronous_standby_names` in PostgreSQL to prevent data loss
4. Use a witness node in a third location

## Related Resources

- [User Manual 24: Backup and Recovery](24-backup-recovery.md) -- Cross-region backup strategy
- [User Manual 29: Disaster Recovery](29-disaster-recovery.md) -- Failover and failback procedures
- [User Manual 30: Enterprise Architecture](30-enterprise-architecture.md) -- Enterprise deployment patterns
- [User Manual 18: Performance Monitoring](18-performance-monitoring.md) -- Monitoring multi-region metrics
- Containers module: `containers/`
- Container adapter: `internal/adapters/containers/adapter.go`
- Kubernetes documentation: https://kubernetes.io/docs/
