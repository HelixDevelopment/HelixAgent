# SuperAgent Deployment Guide

## Overview

This guide provides comprehensive instructions for deploying SuperAgent in production environments. SuperAgent is a production-ready LLM facade system with multi-provider support, ensemble voting, and enterprise-grade features.

## Prerequisites

### System Requirements
- **OS**: Linux (Ubuntu 20.04+, CentOS 7+, RHEL 8+)
- **CPU**: 4+ cores (8+ recommended)
- **RAM**: 8GB minimum (16GB+ recommended)
- **Storage**: 50GB SSD minimum
- **Network**: 1Gbps connection minimum

### Software Dependencies
- **Go**: 1.21+ (compiled binary deployment)
- **PostgreSQL**: 13+ with pgx driver
- **Redis**: 6.0+ for caching
- **Nginx/HAProxy**: For load balancing (optional)
- **Docker**: For containerized deployment
- **Prometheus/Grafana**: For monitoring

### External Services
- **LLM Provider API Keys**:
  - Anthropic Claude API key
  - DeepSeek API key
  - Google Gemini API key
- **Cognee API Key** (optional, for memory enhancement)
- **SSL Certificate** (for HTTPS)

## Deployment Options

### Option 1: Docker Deployment (Recommended)

#### 1. Clone Repository
```bash
git clone https://github.com/superagent/superagent.git
cd superagent
```

#### 2. Environment Configuration
Create `.env` file:
```bash
# Server Configuration
PORT=8080
SUPERAGENT_API_KEY=your-super-secret-api-key

# JWT Configuration
JWT_SECRET=your-jwt-secret-key-change-in-production

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=superagent
DB_PASSWORD=your-db-password
DB_NAME=superagent_db

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your-redis-password

# LLM Provider API Keys
CLAUDE_API_KEY=your-claude-api-key
DEEPSEEK_API_KEY=your-deepseek-api-key
GEMINI_API_KEY=your-gemini-api-key

# Cognee Configuration (Optional)
COGNEE_BASE_URL=http://localhost:8000
COGNEE_API_KEY=your-cognee-api-key
COGNEE_AUTO_COGNIFY=true

# Plugin Configuration
PLUGIN_WATCH_PATHS=./plugins
```

#### 3. Docker Compose Setup
```yaml
# docker-compose.yml
version: '3.8'

services:
  superagent:
    build: .
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - DB_HOST=postgres
      - REDIS_HOST=redis
    depends_on:
      - postgres
      - redis
    volumes:
      - ./plugins:/app/plugins
    restart: unless-stopped

  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: superagent_db
      POSTGRES_USER: superagent
      POSTGRES_PASSWORD: your-db-password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass your-redis-password
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  prometheus:
    image: prom/prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'

  grafana:
    image: grafana/grafana
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./docs/monitoring/grafana-dashboard.json:/etc/grafana/provisioning/dashboards/superagent.json

volumes:
  postgres_data:
  redis_data:
  grafana_data:
```

#### 4. Build and Deploy
```bash
# Build the application
docker build -t superagent .

# Start all services
docker-compose up -d

# Check logs
docker-compose logs -f superagent
```

### Option 2: Binary Deployment

#### 1. Build Binary
```bash
# Clone and build
git clone https://github.com/superagent/superagent.git
cd superagent

# Build for production
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o superagent ./cmd/superagent

# Or build with optimizations
go build -ldflags="-w -s" -o superagent ./cmd/superagent
```

#### 2. System Setup
```bash
# Create system user
sudo useradd -r -s /bin/false superagent

# Create directories
sudo mkdir -p /opt/superagent
sudo mkdir -p /var/log/superagent
sudo mkdir -p /etc/superagent/plugins

# Set permissions
sudo chown -R superagent:superagent /opt/superagent
sudo chown -R superagent:superagent /var/log/superagent
sudo chown -R superagent:superagent /etc/superagent
```

#### 3. Configuration Files
```bash
# /etc/superagent/config.env
PORT=8080
SUPERAGENT_API_KEY=your-super-secret-api-key
JWT_SECRET=your-jwt-secret-key-change-in-production

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=superagent
DB_PASSWORD=your-db-password
DB_NAME=superagent_db

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your-redis-password

# LLM Providers
CLAUDE_API_KEY=your-claude-api-key
DEEPSEEK_API_KEY=your-deepseek-api-key
GEMINI_API_KEY=your-gemini-api-key

# Cognee (Optional)
COGNEE_BASE_URL=http://localhost:8000
COGNEE_API_KEY=your-cognee-api-key
COGNEE_AUTO_COGNIFY=true
```

#### 4. Systemd Service
```bash
# /etc/systemd/system/superagent.service
[Unit]
Description=SuperAgent LLM Facade
After=network.target postgresql.service redis.service
Requires=postgresql.service redis.service

[Service]
Type=simple
User=superagent
Group=superagent
WorkingDirectory=/opt/superagent
ExecStart=/opt/superagent/superagent
EnvironmentFile=/etc/superagent/config.env
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=superagent

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/var/log/superagent
ProtectHome=yes

# Resource limits
MemoryLimit=2G
CPUQuota=200%

[Install]
WantedBy=multi-user.target
```

#### 5. Database Setup
```bash
# Install PostgreSQL
sudo apt update
sudo apt install postgresql postgresql-contrib

# Create database and user
sudo -u postgres psql
CREATE DATABASE superagent_db;
CREATE USER superagent WITH PASSWORD 'your-db-password';
GRANT ALL PRIVILEGES ON DATABASE superagent_db TO superagent;
\q

# Run migrations (SuperAgent will auto-migrate on startup)
```

#### 6. Redis Setup
```bash
# Install Redis
sudo apt install redis-server

# Configure Redis
sudo vim /etc/redis/redis.conf
# Set: requirepass your-redis-password
# Set: bind 127.0.0.1

# Restart Redis
sudo systemctl restart redis
sudo systemctl enable redis
```

#### 7. Deploy and Start
```bash
# Copy binary
sudo cp superagent /opt/superagent/
sudo chmod +x /opt/superagent/superagent

# Start service
sudo systemctl daemon-reload
sudo systemctl start superagent
sudo systemctl enable superagent

# Check status
sudo systemctl status superagent
journalctl -u superagent -f
```

## Load Balancing Setup

### Nginx Configuration
```nginx
# /etc/nginx/sites-available/superagent
upstream superagent_backend {
    server 127.0.0.1:8080;
    server 127.0.0.1:8081;
    server 127.0.0.1:8082;
}

server {
    listen 80;
    server_name api.superagent.ai;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.superagent.ai;

    ssl_certificate /etc/ssl/certs/superagent.crt;
    ssl_certificate_key /etc/ssl/private/superagent.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
    limit_req zone=api burst=20 nodelay;

    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains";

    location / {
        proxy_pass http://superagent_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support for streaming
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 300s;
    }

    location /metrics {
        proxy_pass http://127.0.0.1:9090;
        allow 10.0.0.0/8;
        deny all;
    }

    location /health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
}
```

## Monitoring Setup

### Prometheus Configuration
```yaml
# /etc/prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

scrape_configs:
  - job_name: 'superagent'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s

  - job_name: 'postgres'
    static_configs:
      - targets: ['localhost:9187']
    scrape_interval: 10s

  - job_name: 'redis'
    static_configs:
      - targets: ['localhost:9121']
    scrape_interval: 10s
```

### Grafana Setup
```bash
# Install Grafana
sudo apt install grafana

# Import dashboard
curl -X POST -H "Content-Type: application/json" \
  -d @docs/monitoring/grafana-dashboard.json \
  http://admin:admin@localhost:3000/api/dashboards/db
```

## Security Configuration

### SSL/TLS Setup
```bash
# Let's Encrypt (recommended)
sudo apt install certbot
sudo certbot certonly --standalone -d api.superagent.ai

# Or self-signed for testing
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

### Firewall Configuration
```bash
# UFW configuration
sudo ufw allow ssh
sudo ufw allow 80
sudo ufw allow 443
sudo ufw allow 8080  # SuperAgent port
sudo ufw --force enable
```

### API Key Management
```bash
# Generate secure API keys
openssl rand -hex 32

# Store in environment securely
echo "SUPERAGENT_API_KEY=$(openssl rand -hex 32)" >> /etc/superagent/config.env
```

## Scaling and High Availability

### Horizontal Scaling
```yaml
# docker-compose.scale.yml
version: '3.8'

services:
  superagent:
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '2.0'
          memory: 4G
        reservations:
          cpus: '1.0'
          memory: 2G
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
        window: 120s
```

### Database Clustering
```bash
# PostgreSQL streaming replication setup
# Master configuration
wal_level = replica
max_wal_senders = 3
wal_keep_segments = 64

# Replica configuration
hot_standby = on
```

### Redis Clustering
```redis.conf
# Redis cluster configuration
cluster-enabled yes
cluster-config-file nodes.conf
cluster-node-timeout 5000
appendonly yes
```

## Backup and Recovery

### Database Backup
```bash
# Automated backup script
#!/bin/bash
BACKUP_DIR="/var/backups/superagent"
DATE=$(date +%Y%m%d_%H%M%S)

# PostgreSQL backup
pg_dump -U superagent -h localhost superagent_db > $BACKUP_DIR/db_$DATE.sql

# Redis backup
redis-cli -a your-redis-password --rdb $BACKUP_DIR/redis_$DATE.rdb

# Compress and rotate
gzip $BACKUP_DIR/db_$DATE.sql
find $BACKUP_DIR -name "*.sql.gz" -mtime +30 -delete
find $BACKUP_DIR -name "*.rdb" -mtime +7 -delete
```

### Configuration Backup
```bash
# Backup configurations
tar -czf /var/backups/superagent/config_$DATE.tar.gz \
  /etc/superagent/ \
  /etc/nginx/sites-available/superagent \
  /etc/prometheus/prometheus.yml
```

## Troubleshooting

### Common Issues

#### 1. Database Connection Issues
```bash
# Check PostgreSQL status
sudo systemctl status postgresql

# Check connection
psql -U superagent -d superagent_db -h localhost

# View logs
sudo tail -f /var/log/postgresql/postgresql-*.log
```

#### 2. Redis Connection Issues
```bash
# Check Redis status
sudo systemctl status redis

# Test connection
redis-cli -a your-redis-password ping

# View logs
sudo tail -f /var/log/redis/redis-server.log
```

#### 3. High Memory Usage
```bash
# Check Go memory stats
curl http://localhost:8080/metrics | grep go_memstats

# Adjust GOGC if needed
export GOGC=50  # Lower GC threshold
```

#### 4. Slow Response Times
```bash
# Check provider health and circuit breaker status
curl http://localhost:8080/v1/providers/claude/health

# Monitor circuit breakers in metrics
curl http://localhost:8080/metrics | grep circuit_breaker

# Check provider response times
curl http://localhost:8080/metrics | grep llm_response_time
```

#### 5. Circuit Breaker Issues
```bash
# Check circuit breaker states
curl http://localhost:8080/metrics | grep circuit_breaker_state

# Reset circuit breaker manually (development only)
curl -X POST http://localhost:8080/v1/admin/providers/claude/reset-circuit-breaker
```

### Log Analysis
```bash
# View application logs
journalctl -u superagent -f

# Search for errors
journalctl -u superagent | grep ERROR

# Performance monitoring
curl http://localhost:8080/metrics | grep llm_response_time
```

## Performance Tuning

### Go Runtime Optimization
```bash
# Environment variables for performance
export GOGC=100          # GC target percentage
export GOMAXPROCS=8      # CPU cores to use
export GODEBUG=gctrace=1 # GC tracing (for debugging)
```

### Database Optimization
```sql
-- PostgreSQL performance settings
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET work_mem = '4MB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_buffers = '16MB';
ALTER SYSTEM SET default_statistics_target = 100;
```

### Redis Optimization
```redis.conf
# Redis performance settings
maxmemory 256mb
maxmemory-policy allkeys-lru
tcp-keepalive 300
timeout 300
databases 16
```

## Upgrade Procedure

### Rolling Updates
```bash
# Update binary
sudo systemctl stop superagent
sudo cp new-superagent /opt/superagent/superagent
sudo systemctl start superagent

# Verify health
curl http://localhost:8080/health

# Rollback if needed
sudo systemctl stop superagent
sudo cp backup-superagent /opt/superagent/superagent
sudo systemctl start superagent
```

### Database Migrations
```bash
# SuperAgent handles migrations automatically on startup
# For manual control:
psql -U superagent -d superagent_db -f migration.sql
```

## Support and Maintenance

### Health Checks
```bash
# Application health
curl http://localhost:8080/v1/health

# Database health
pg_isready -U superagent -d superagent_db

# Redis health
redis-cli -a your-redis-password ping
```

### Monitoring Alerts
- CPU usage > 80%
- Memory usage > 90%
- Error rate > 5%
- Response time > 30s
- Database connections > 90% of pool
- Redis memory > 80%

### Log Rotation
```bash
# Configure logrotate
echo "/var/log/superagent/*.log {
    daily
    rotate 30
    compress
    missingok
    notifempty
}" > /etc/logrotate.d/superagent
```

This deployment guide provides a comprehensive foundation for running SuperAgent in production. For specific environment requirements or advanced configurations, consult the project documentation or community forums.