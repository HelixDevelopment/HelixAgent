# 🚀 Advanced AI Debate Configuration System - Deployment Guide

## 📋 Deployment Overview

This guide provides comprehensive instructions for deploying the Advanced AI Debate Configuration System in production environments.

## 🏗️ System Architecture

### Component Overview
```
┌─────────────────────────────────────────────────────────────────┐
│                    Load Balancer / API Gateway                  │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │   Debate    │ │  Monitoring │ │  Security   │ │  Reporting  ││
│  │   Service   │ │   Service   │ │   Service   │ │   Service   ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │  Cognee AI  │ │Performance  │ │   History   │ │ Resilience  ││
│  │ Integration │ │Optimization │ │ Management  │ │   Service   ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
├─────────────────────────────────────────────────────────────────┤
│                    Shared Infrastructure                        │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│ │ PostgreSQL  │ │    Redis    │ │Message Queue│ │Object Storage││
│  │   Database  │ │    Cache    │ │ (RabbitMQ)  │ │   (S3/MinIO) ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Deployment Modes

#### 1. Single-Node Deployment
- **Use Case**: Development, testing, small-scale production
- **Resources**: 8 CPU cores, 16GB RAM, 100GB storage
- **Components**: All services on single node

#### 2. Multi-Node Deployment
- **Use Case**: Medium-scale production
- **Resources**: 3+ nodes, 4 CPU cores each, 8GB RAM each
- **Components**: Services distributed across nodes

#### 3. High-Availability Deployment
- **Use Case**: Large-scale production
- **Resources**: 5+ nodes, load balancer, redundant storage
- **Components**: Full redundancy and failover

## 🔧 Prerequisites

### System Requirements

#### Minimum Requirements
```yaml
# Single Node Deployment
CPU: 8 cores (16 threads)
RAM: 16GB (32GB recommended)
Storage: 100GB SSD (500GB recommended)
Network: 1Gbps connection
OS: Ubuntu 20.04+ / CentOS 8+ / RHEL 8+
```

#### Recommended Requirements
```yaml
# Multi-Node Deployment
CPU: 16+ cores per node
RAM: 32GB+ per node
Storage: 500GB+ NVMe SSD per node
Network: 10Gbps connection
OS: Ubuntu 22.04+ / RHEL 9+
Load Balancer: HAProxy / NGINX Plus
```

### Software Dependencies

#### Core Dependencies
```bash
# Install Go 1.21+
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

#### Database Dependencies
```bash
# Install PostgreSQL 14+
sudo apt update
sudo apt install postgresql postgresql-contrib
sudo systemctl enable postgresql

# Install Redis 7+
sudo apt install redis-server
sudo systemctl enable redis-server
```

#### Message Queue Dependencies
```bash
# Install RabbitMQ
sudo apt install rabbitmq-server
sudo systemctl enable rabbitmq-server
sudo rabbitmq-plugins enable rabbitmq_management
```

## 📦 Installation

### 1. Download and Prepare

```bash
# Clone the repository
git clone https://github.com/superagent/superagent.git
cd superagent

# Checkout the advanced features branch
git checkout advanced-features-complete

# Build the application
go mod download
go build -o superagent-advanced ./cmd/main.go
```

### 2. Configuration Setup

#### Create Configuration Directory
```bash
sudo mkdir -p /etc/superagent/advanced
sudo mkdir -p /var/log/superagent
sudo mkdir -p /var/lib/superagent
sudo chown -R $USER:$USER /etc/superagent /var/log/superagent /var/lib/superagent
```

#### Generate Configuration Files
```bash
# Create main configuration
cat > /etc/superagent/advanced/config.yaml << 'EOF'
# Advanced AI Debate Configuration System
server:
  host: 0.0.0.0
  port: 8080
  mode: production
  tls:
    enabled: true
    cert_file: /etc/superagent/certs/server.crt
    key_file: /etc/superagent/certs/server.key

database:
  host: localhost
  port: 5432
  name: superagent_advanced
  user: superagent
  password: ${DB_PASSWORD}
  ssl_mode: require
  max_connections: 100
  connection_timeout: 30s

cache:
  type: redis
  host: localhost
  port: 6379
  password: ${REDIS_PASSWORD}
  database: 0
  max_retries: 3
  dial_timeout: 5s

message_queue:
  type: rabbitmq
  host: localhost
  port: 5672
  user: superagent
  password: ${RABBITMQ_PASSWORD}
  vhost: /
  ssl: false

ai_debate:
  enabled: true
  maximal_repeat_rounds: 5
  debate_timeout: 300000  # 5 minutes
  consensus_threshold: 0.75
  max_response_time: 30000  # 30 seconds
  max_context_length: 32000
  quality_threshold: 0.7
  enable_cognee: true
  
  cognee_config:
    enabled: true
    enhance_responses: true
    analyze_consensus: true
    generate_insights: true
    dataset_name: "ai_debate_enhancement"
    max_enhancement_time: 10000  # 10 seconds
    enhancement_strategy: "hybrid"
    memory_integration: true
    contextual_analysis: true
  
  participants:
    - name: "Analyst1"
      role: "Primary Analyst"
      enabled: true
      llms:
        - name: "Primary LLM"
          provider: "claude"
          model: "claude-3-5-sonnet-20241022"
          enabled: true
          api_key: "${CLAUDE_API_KEY}"
          temperature: 0.1
          max_tokens: 2000
          weight: 1.0
          timeout: 30000
        - name: "Fallback LLM"
          provider: "deepseek"
          model: "deepseek-coder"
          enabled: true
          api_key: "${DEEPSEEK_API_KEY}"
          temperature: 0.1
          max_tokens: 2000
          weight: 0.9
          timeout: 30000
  
  debate_strategy: "adaptive"
  voting_strategy: "weighted_consensus"
  
  # Advanced Features
  monitoring_enabled: true
  performance_optimization_enabled: true
  performance_optimization_level: "advanced"
  history_enabled: true
  history_retention_policy: "30_days"
  history_archival_strategy: "compress_and_encrypt"
  max_history_size: 1073741824  # 1GB
  resilience_enabled: true
  resilience_level: "advanced"
  recovery_timeout: 300000
  max_retry_attempts: 5
  threat_prevention_enabled: true
  reporting_enabled: true
  reporting_level: "comprehensive"
  max_report_size: 10485760  # 10MB
  report_retention_policy: "90_days"
  security_enabled: true
  security_level: "advanced"
  encryption_enabled: true
  audit_enabled: true

# Advanced Monitoring Configuration
monitoring:
  enabled: true
  update_interval: 5s
  retention_period: 30d
  alerting:
    enabled: true
    channels:
      - type: email
        enabled: true
        smtp_host: ${SMTP_HOST}
        smtp_port: ${SMTP_PORT}
        smtp_user: ${SMTP_USER}
        smtp_password: ${SMTP_PASSWORD}
        from_address: alerts@superagent.com
        to_addresses: ["admin@company.com"]
      - type: slack
        enabled: true
        webhook_url: ${SLACK_WEBHOOK_URL}
        channel: "#alerts"
    rules:
      - name: "high_error_rate"
        metric: "error_rate"
        threshold: 0.05
        duration: 5m
        severity: "warning"
      - name: "low_consensus_rate"
        metric: "consensus_rate"
        threshold: 0.6
        duration: 10m
        severity: "critical"

# Performance Optimization Configuration
performance:
  auto_tuning:
    enabled: true
    interval: 1h
    metrics:
      - "response_time"
      - "throughput"
      - "error_rate"
      - "consensus_level"
  optimization_targets:
    - "efficiency"
    - "quality"
    - "reliability"
  constraints:
    max_cpu_usage: 0.8
    max_memory_usage: 0.8
    max_response_time: 30s

# Resilience Configuration
resilience:
  circuit_breaker:
    enabled: true
    failure_threshold: 5
    success_threshold: 2
    timeout: 60s
    half_open_max_calls: 3
  retry:
    enabled: true
    max_attempts: 5
    backoff_strategy: "exponential"
    initial_interval: 1s
    max_interval: 30s
    multiplier: 2.0
  timeout:
    enabled: true
    default_timeout: 30s
    circuit_breaker_timeout: 60s
  health_check:
    enabled: true
    interval: 30s
    timeout: 5s
    failure_threshold: 3

# Security Configuration
security:
  authentication:
    enabled: true
    methods: ["basic", "oauth2", "api_key"]
    session_timeout: 24h
    max_sessions_per_user: 5
    mfa:
      enabled: true
      methods: ["totp", "sms", "email"]
  authorization:
    enabled: true
    model: "rbac"
    permissions:
      - "debate:create"
      - "debate:read"
      - "debate:update"
      - "debate:delete"
      - "report:generate"
      - "report:export"
      - "admin:manage"
  encryption:
    enabled: true
    algorithm: "aes-256-gcm"
    key_rotation_interval: 30d
    key_derivation_function: "pbkdf2"
  audit:
    enabled: true
    level: "detailed"
    retention_period: 365d
    encryption: true
    tamper_protection: true
  threat_detection:
    enabled: true
    update_interval: 1h
    detection_rules:
      - "brute_force_attack"
      - "sql_injection"
      - "xss_attack"
      - "rate_limit_exceeded"

logging:
  level: "info"
  format: "json"
  output: "file"
  file: "/var/log/superagent/advanced/superagent.log"
  max_size: 100MB
  max_backups: 10
  max_age: 30d
  compress: true

EOF

# Create environment file
cat > /etc/superagent/advanced/.env << 'EOF'
# Database Configuration
DB_PASSWORD=your_secure_database_password
DB_HOST=localhost
DB_PORT=5432

# Redis Configuration
REDIS_PASSWORD=your_secure_redis_password
REDIS_HOST=localhost
REDIS_PORT=6379

# RabbitMQ Configuration
RABBITMQ_PASSWORD=your_secure_rabbitmq_password
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672

# API Keys
CLAUDE_API_KEY=your_claude_api_key
DEEPSEEK_API_KEY=your_deepseek_api_key
OPENAI_API_KEY=your_openai_api_key

# Email Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your_email@gmail.com
SMTP_PASSWORD=your_email_password

# Slack Configuration
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK

# TLS Certificates
TLS_CERT_FILE=/etc/superagent/certs/server.crt
TLS_KEY_FILE=/etc/superagent/certs/server.key
EOF

# Set proper permissions
chmod 600 /etc/superagent/advanced/.env
chmod 644 /etc/superagent/advanced/config.yaml
```

### 3. Database Setup

#### PostgreSQL Database Creation
```bash
# Create database and user
sudo -u postgres psql << 'EOF'
CREATE DATABASE superagent_advanced;
CREATE USER superagent WITH ENCRYPTED PASSWORD 'your_secure_database_password';
GRANT ALL PRIVILEGES ON DATABASE superagent_advanced TO superagent;
ALTER DATABASE superagent_advanced OWNER TO superagent;
\c superagent_advanced;

-- Create advanced debate tables
CREATE SCHEMA advanced;
SET search_path TO advanced;

-- Debate sessions table
CREATE TABLE debate_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    debate_strategy VARCHAR(100) NOT NULL,
    voting_strategy VARCHAR(100) NOT NULL,
    consensus_threshold DECIMAL(3,2) NOT NULL DEFAULT 0.75,
    max_participants INTEGER NOT NULL DEFAULT 10,
    debate_timeout INTEGER NOT NULL DEFAULT 300000,
    max_response_time INTEGER NOT NULL DEFAULT 30000,
    max_context_length INTEGER NOT NULL DEFAULT 32000,
    quality_threshold DECIMAL(3,2) NOT NULL DEFAULT 0.7,
    enable_cognee BOOLEAN NOT NULL DEFAULT true,
    monitoring_enabled BOOLEAN NOT NULL DEFAULT true,
    performance_tracking BOOLEAN NOT NULL DEFAULT true,
    history_enabled BOOLEAN NOT NULL DEFAULT true,
    resilience_enabled BOOLEAN NOT NULL DEFAULT true,
    reporting_enabled BOOLEAN NOT NULL DEFAULT true,
    security_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}'
);

-- Participants table
CREATE TABLE debate_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES debate_sessions(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

-- LLM configurations table
CREATE TABLE llm_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    participant_id UUID NOT NULL REFERENCES debate_participants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(100) NOT NULL,
    model VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    temperature DECIMAL(3,2) NOT NULL DEFAULT 0.1,
    max_tokens INTEGER NOT NULL DEFAULT 2000,
    weight DECIMAL(3,2) NOT NULL DEFAULT 1.0,
    timeout INTEGER NOT NULL DEFAULT 30000,
    api_key_encrypted TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Performance metrics table
CREATE TABLE performance_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES debate_sessions(id) ON DELETE CASCADE,
    metric_name VARCHAR(255) NOT NULL,
    metric_value DECIMAL(10,6) NOT NULL,
    metric_unit VARCHAR(50),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

-- Security audit log table
CREATE TABLE security_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    user_id VARCHAR(255),
    session_id UUID,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    action VARCHAR(100) NOT NULL,
    result VARCHAR(50) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    description TEXT,
    severity VARCHAR(20) NOT NULL DEFAULT 'info',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_debate_sessions_status ON debate_sessions(status);
CREATE INDEX idx_debate_sessions_created_at ON debate_sessions(created_at);
CREATE INDEX idx_debate_participants_session_id ON debate_participants(session_id);
CREATE INDEX idx_performance_metrics_session_id ON performance_metrics(session_id);
CREATE INDEX idx_performance_metrics_timestamp ON performance_metrics(timestamp);
CREATE INDEX idx_security_audit_log_event_type ON security_audit_log(event_type);
CREATE INDEX idx_security_audit_log_timestamp ON security_audit_log(created_at);
CREATE INDEX idx_security_audit_log_user_id ON security_audit_log(user_id);

-- Create update trigger for updated_at columns
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_debate_sessions_updated_at 
    BEFORE UPDATE ON debate_sessions 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_debate_participants_updated_at 
    BEFORE UPDATE ON debate_participants 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_llm_configurations_updated_at 
    BEFORE UPDATE ON llm_configurations 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Grant permissions
GRANT USAGE ON SCHEMA advanced TO superagent;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA advanced TO superagent;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA advanced TO superagent;

EOF

# Create Redis configuration
cat > /etc/redis/conf.d/superagent.conf << 'EOF'
# Redis configuration for SuperAgent Advanced
maxmemory 2gb
maxmemory-policy allkeys-lru
save 900 1
save 300 10
save 60 10000
rdbcompression yes
rdbchecksum yes
appendonly yes
appendfsync everysec
EOF

# Create RabbitMQ configuration
cat > /etc/rabbitmq/conf.d/superagent.conf << 'EOF'
# RabbitMQ configuration for SuperAgent Advanced
loopback_users.guest = false
listeners.tcp.default = 5672
management.tcp.port = 15672
management.load_definitions = /etc/rabbitmq/definitions.json
EOF
```

### 4. TLS Certificate Setup

```bash
# Create certificate directory
sudo mkdir -p /etc/superagent/certs
sudo chmod 700 /etc/superagent/certs

# Generate self-signed certificates (for development)
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/superagent/certs/server.key \
  -out /etc/superagent/certs/server.crt \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=superagent.local"

# Set proper permissions
sudo chmod 600 /etc/superagent/certs/server.key
sudo chmod 644 /etc/superagent/certs/server.crt

# For production, use Let's Encrypt or commercial certificates
# sudo certbot certonly --standalone -d your-domain.com
```

## 🚀 Deployment Execution

### 1. Database Initialization

```bash
# Start PostgreSQL
sudo systemctl start postgresql
sudo systemctl enable postgresql

# Start Redis
sudo systemctl start redis-server
sudo systemctl enable redis-server

# Start RabbitMQ
sudo systemctl start rabbitmq-server
sudo systemctl enable rabbitmq-server

# Create RabbitMQ user
sudo rabbitmqctl add_user superagent your_secure_rabbitmq_password
sudo rabbitmqctl set_user_tags superagent administrator
sudo rabbitmqctl set_permissions -p / superagent ".*" ".*" ".*"
```

### 2. Application Deployment

#### Option A: Systemd Service (Recommended)

```bash
# Create systemd service file
sudo tee /etc/systemd/system/superagent-advanced.service > /dev/null << 'EOF'
[Unit]
Description=SuperAgent Advanced AI Debate Configuration System
Documentation=https://github.com/superagent/superagent
After=network.target postgresql.service redis-server.service rabbitmq-server.service
Wants=postgresql.service redis-server.service rabbitmq-server.service

[Service]
Type=simple
User=superagent
Group=superagent
WorkingDirectory=/opt/superagent/advanced
ExecStart=/opt/superagent/advanced/superagent-advanced --config /etc/superagent/advanced/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30

# Environment
Environment=SUPERAGENT_CONFIG_PATH=/etc/superagent/advanced
Environment=SUPERAGENT_LOG_PATH=/var/log/superagent/advanced
EnvironmentFile=/etc/superagent/advanced/.env

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/superagent/advanced /var/lib/superagent/advanced
AmbientCapabilities=CAP_NET_BIND_SERVICE

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096
MemoryMax=4G
CPUQuota=400%

# Logging
StandardOutput=append:/var/log/superagent/advanced/service.log
StandardError=append:/var/log/superagent/advanced/error.log
SyslogIdentifier=superagent-advanced

[Install]
WantedBy=multi-user.target
EOF

# Create application user
sudo useradd -r -s /bin/false -d /opt/superagent superagent
sudo mkdir -p /opt/superagent/advanced
sudo chown superagent:superagent /opt/superagent/advanced

# Copy application binary
sudo cp superagent-advanced /opt/superagent/advanced/
sudo chmod +x /opt/superagent/advanced/superagent-advanced
sudo chown superagent:superagent /opt/superagent/advanced/superagent-advanced

# Create log directory
sudo mkdir -p /var/log/superagent/advanced
sudo chown superagent:superagent /var/log/superagent/advanced

# Create data directory
sudo mkdir -p /var/lib/superagent/advanced
sudo chown superagent:superagent /var/lib/superagent/advanced

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable superagent-advanced
sudo systemctl start superagent-advanced
```

#### Option B: Docker Deployment

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o superagent-advanced ./cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy binary
COPY --from=builder /app/superagent-advanced .

# Copy configuration
COPY --from=builder /app/configs /etc/superagent/advanced

# Create user
RUN addgroup -g 1000 -S superagent && \
    adduser -u 1000 -S superagent -G superagent

# Create directories
RUN mkdir -p /var/log/superagent/advanced /var/lib/superagent/advanced && \
    chown -R superagent:superagent /var/log/superagent /var/lib/superagent

USER superagent

EXPOSE 8080 8443

ENTRYPOINT ["./superagent-advanced"]
CMD ["--config", "/etc/superagent/advanced/config.yaml"]
```

```yaml
# docker-compose.yml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    container_name: superagent-postgres
    environment:
      POSTGRES_DB: superagent_advanced
      POSTGRES_USER: superagent
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    ports:
      - "5432:5432"
    networks:
      - superagent-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U superagent"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: superagent-redis
    command: redis-server /etc/redis/conf.d/superagent.conf
    volumes:
      - ./redis.conf:/etc/redis/conf.d/superagent.conf
      - redis_data:/data
    ports:
      - "6379:6379"
    networks:
      - superagent-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5

  rabbitmq:
    image: rabbitmq:3.12-management-alpine
    container_name: superagent-rabbitmq
    environment:
      RABBITMQ_DEFAULT_USER: superagent
      RABBITMQ_DEFAULT_PASS: ${RABBITMQ_PASSWORD}
    volumes:
      - rabbitmq_data:/var/lib/rabbitmq
      - ./rabbitmq.conf:/etc/rabbitmq/conf.d/superagent.conf
    ports:
      - "5672:5672"
      - "15672:15672"
    networks:
      - superagent-network
    healthcheck:
      test: ["CMD", "rabbitmq-diagnostics", "ping"]
      interval: 30s
      timeout: 10s
      retries: 5

  superagent:
    build: .
    container_name: superagent-advanced
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      rabbitmq:
        condition: service_healthy
    environment:
      - SUPERAGENT_CONFIG_PATH=/etc/superagent/advanced
      - DB_PASSWORD=${DB_PASSWORD}
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - RABBITMQ_PASSWORD=${RABBITMQ_PASSWORD}
      - CLAUDE_API_KEY=${CLAUDE_API_KEY}
      - DEEPSEEK_API_KEY=${DEEPSEEK_API_KEY}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    volumes:
      - ./config.yaml:/etc/superagent/advanced/config.yaml:ro
      - ./certs:/etc/superagent/certs:ro
      - superagent_logs:/var/log/superagent/advanced
      - superagent_data:/var/lib/superagent/advanced
    ports:
      - "8080:8080"
      - "8443:8443"
    networks:
      - superagent-network
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s

volumes:
  postgres_data:
  redis_data:
  rabbitmq_data:
  superagent_logs:
  superagent_data:

networks:
  superagent-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

### 3. Kubernetes Deployment (Optional)

```yaml
# kubernetes/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: superagent-advanced
  labels:
    name: superagent-advanced
```

```yaml
# kubernetes/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: superagent-config
  namespace: superagent-advanced
data:
  config.yaml: |
    # Main configuration (same as above)
    server:
      host: 0.0.0.0
      port: 8080
      mode: production
      # ... rest of configuration
```

## 🔍 Health Checks and Monitoring

### 1. Health Check Endpoints

```bash
# Basic health check
curl -f http://localhost:8080/health

# Detailed health check
curl -f http://localhost:8080/health/detailed

# Readiness check
curl -f http://localhost:8080/ready

# Metrics endpoint
curl -f http://localhost:8080/metrics
```

### 2. Monitoring Setup

```bash
# Install monitoring tools
sudo apt install prometheus-node-exporter
sudo systemctl enable prometheus-node-exporter

# Configure Prometheus
cat > /etc/prometheus/prometheus.yml << 'EOF'
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'superagent-advanced'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 10s
    
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['localhost:9100']
EOF

# Start Prometheus
sudo systemctl start prometheus
sudo systemctl enable prometheus
```

## 🔐 Security Hardening

### 1. Network Security

```bash
# Configure firewall
sudo ufw allow 8080/tcp
sudo ufw allow 8443/tcp
sudo ufw allow 5432/tcp  # PostgreSQL (internal only)
sudo ufw allow 6379/tcp  # Redis (internal only)
sudo ufw allow 5672/tcp  # RabbitMQ (internal only)
sudo ufw enable

# Configure fail2ban
sudo apt install fail2ban
sudo tee /etc/fail2ban/jail.local > /dev/null << 'EOF'
[superagent]
enabled = true
port = 8080,8443
filter = superagent
logpath = /var/log/superagent/advanced/error.log
maxretry = 5
bantime = 3600
findtime = 600
EOF

sudo systemctl restart fail2ban
```

### 2. Application Security

```bash
# Set up API rate limiting
# Configure in nginx/apache if using reverse proxy
# Or implement in application layer

# Enable security headers
cat > /etc/nginx/conf.d/security-headers.conf << 'EOF'
add_header X-Frame-Options DENY;
add_header X-Content-Type-Options nosniff;
add_header X-XSS-Protection "1; mode=block";
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';";
add_header Referrer-Policy "strict-origin-when-cross-origin";
EOF
```

## 📊 Verification and Testing

### 1. Deployment Verification

```bash
# Check service status
sudo systemctl status superagent-advanced

# Check logs
sudo journalctl -u superagent-advanced -f

# Test health endpoint
curl -f http://localhost:8080/health

# Test advanced debate endpoint
curl -X POST http://localhost:8080/api/v1/debate/advanced \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "topic": "AI Ethics in Autonomous Systems",
    "context": "Discuss the ethical implications",
    "strategy": "consensus_building",
    "participants": 3,
    "timeout": 300000
  }'
```

### 2. Performance Testing

```bash
# Install load testing tools
sudo apt install apache2-utils

# Test basic load
ab -n 1000 -c 10 http://localhost:8080/health

# Test debate endpoint load
ab -n 100 -c 5 -T 'application/json' -H 'Authorization: Bearer YOUR_API_KEY' \
  -p test-debate.json http://localhost:8080/api/v1/debate/advanced
```

## 🚨 Troubleshooting

### Common Issues

#### 1. Service Won't Start
```bash
# Check logs
sudo journalctl -u superagent-advanced -n 50

# Check configuration
sudo -u superagent /opt/superagent/advanced/superagent-advanced --config /etc/superagent/advanced/config.yaml --validate

# Check dependencies
sudo systemctl status postgresql redis-server rabbitmq-server
```

#### 2. Database Connection Issues
```bash
# Test database connection
sudo -u postgres psql -h localhost -U superagent -d superagent_advanced -c "SELECT 1;"

# Check PostgreSQL logs
sudo tail -f /var/log/postgresql/postgresql-*.log
```

#### 3. Performance Issues
```bash
# Check system resources
top -p $(pgrep superagent-advanced)
iostat -x 1
free -h

# Check application metrics
curl http://localhost:8080/metrics | grep -E "(cpu|memory|goroutine)"
```

## 📚 Additional Resources

### Documentation Links
- [API Documentation](https://docs.superagent.com/api)
- [Configuration Reference](https://docs.superagent.com/configuration)
- [Security Guide](https://docs.superagent.com/security)
- [Monitoring Guide](https://docs.superagent.com/monitoring)

### Support Contacts
- Technical Support: support@superagent.com
- Security Issues: security@superagent.com
- General Inquiries: info@superagent.com

---

**Next Steps**: Continue to the [Operational Guide](OPERATIONAL_GUIDE.md) for ongoing maintenance and operations procedures.