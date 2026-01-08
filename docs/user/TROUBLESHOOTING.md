# HelixAgent Troubleshooting Guide

Quick solutions for common issues when running HelixAgent.

## Table of Contents

1. [Installation Issues](#installation-issues)
2. [Startup Problems](#startup-problems)
3. [Provider Errors](#provider-errors)
4. [Database Issues](#database-issues)
5. [Cache Issues](#cache-issues)
6. [Performance Problems](#performance-problems)
7. [Authentication Errors](#authentication-errors)
8. [Ensemble Issues](#ensemble-issues)
9. [Streaming Problems](#streaming-problems)
10. [Container Issues](#container-issues)

---

## Installation Issues

### Go Version Too Old

**Error**: `go: requires go1.23 or later`

**Solution**:
```bash
# Check current version
go version

# Update Go (Linux)
sudo rm -rf /usr/local/go
wget https://go.dev/dl/go1.23.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.linux-amd64.tar.gz

# Update PATH
export PATH=$PATH:/usr/local/go/bin
```

### Missing Dependencies

**Error**: `cannot find package "github.com/..."`

**Solution**:
```bash
# Download dependencies
go mod download

# Or tidy modules
go mod tidy
```

### Docker Not Found

**Error**: `docker: command not found`

**Solution**:
```bash
# Install Docker
curl -fsSL https://get.docker.com | sh

# Add user to docker group
sudo usermod -aG docker $USER

# Restart shell or run
newgrp docker
```

### Docker Compose Not Found

**Error**: `docker-compose: command not found`

**Solution**:
```bash
# Install Docker Compose V2
sudo apt-get update
sudo apt-get install docker-compose-plugin

# Or use the docker compose command (with space)
docker compose up -d
```

---

## Startup Problems

### Port Already in Use

**Error**: `listen tcp :7061: bind: address already in use`

**Solution**:
```bash
# Find process using port
lsof -i :7061
# Or
netstat -tlnp | grep 8080

# Kill process
kill -9 <PID>

# Or use different port
export PORT=8081
./bin/helixagent
```

### Configuration File Not Found

**Error**: `config file not found`

**Solution**:
```bash
# Ensure config exists
ls configs/

# Copy sample config
cp configs/development.yaml.example configs/development.yaml

# Or specify config path
./bin/helixagent --config /path/to/config.yaml
```

### Environment Variables Not Set

**Error**: `API key not configured` or empty response

**Solution**:
```bash
# Create .env from example
cp .env.example .env

# Edit .env file
nano .env

# Verify environment
env | grep API_KEY

# Source .env manually
export $(cat .env | xargs)
```

### Permission Denied

**Error**: `permission denied`

**Solution**:
```bash
# Make binary executable
chmod +x ./bin/helixagent

# Fix Docker socket permissions
sudo chmod 666 /var/run/docker.sock

# Or add to docker group
sudo usermod -aG docker $USER
```

---

## Provider Errors

### OpenAI API Key Invalid

**Error**: `401 Unauthorized` or `Invalid API key`

**Checklist**:
1. Key starts with `sk-`
2. No extra whitespace
3. Key not revoked
4. Organization has available credits

**Test**:
```bash
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

### Anthropic API Errors

**Error**: `401 Unauthorized` from Anthropic

**Checklist**:
1. Key starts with `sk-ant-`
2. Check account status at console.anthropic.com
3. Verify model access (claude-3 requires specific access)

**Test**:
```bash
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-haiku-20240307","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'
```

### Google Gemini Errors

**Error**: `PERMISSION_DENIED` or `API key not valid`

**Checklist**:
1. Enable Generative AI API in GCP Console
2. Key has correct project
3. Billing enabled

**Test**:
```bash
curl "https://generativelanguage.googleapis.com/v1/models?key=$GEMINI_API_KEY"
```

### Ollama Connection Refused

**Error**: `dial tcp [::1]:11434: connect: connection refused`

**Solution**:
```bash
# Check Ollama is running
ollama list

# Start Ollama
ollama serve

# Or with Docker
docker run -d -v ollama:/root/.ollama -p 11434:11434 ollama/ollama
```

### Rate Limit Exceeded

**Error**: `429 Too Many Requests`

**Solution**:
```bash
# Increase retry delays in config
retry:
  count: 5
  wait_time: 1s
  max_wait: 30s
  exponential_backoff: true

# Or implement request queuing
rate_limit:
  requests_per_minute: 30
```

### Provider Timeout

**Error**: `context deadline exceeded` or `timeout`

**Solution**:
```yaml
# Increase timeout in config
server:
  request_timeout: 120s

providers:
  openai:
    timeout: 60s
```

---

## Database Issues

### Connection Refused

**Error**: `dial tcp 127.0.0.1:5432: connect: connection refused`

**Solution**:
```bash
# Start PostgreSQL
docker-compose up -d postgres

# Check status
docker-compose ps postgres

# Check logs
docker-compose logs postgres
```

### Authentication Failed

**Error**: `password authentication failed for user`

**Solution**:
```bash
# Verify credentials in .env
cat .env | grep DB_

# Reset password (Docker)
docker-compose exec postgres psql -U postgres -c "ALTER USER helixagent PASSWORD 'newpassword';"

# Update .env
DB_PASSWORD=newpassword
```

### Database Does Not Exist

**Error**: `database "helixagent" does not exist`

**Solution**:
```bash
# Create database
docker-compose exec postgres psql -U postgres -c "CREATE DATABASE helixagent;"

# Or run migrations
make db-migrate
```

### Migration Errors

**Error**: `migration failed`

**Solution**:
```bash
# Check current migration status
make db-status

# Reset and re-run migrations
make db-reset
make db-migrate

# Manual fix
docker-compose exec postgres psql -U helixagent -d helixagent
```

### Too Many Connections

**Error**: `too many connections for role`

**Solution**:
```yaml
# Reduce pool size in config
database:
  max_connections: 10
  min_connections: 2
```

```bash
# Check current connections
docker-compose exec postgres psql -U postgres -c "SELECT count(*) FROM pg_stat_activity;"

# Kill idle connections
docker-compose exec postgres psql -U postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle' AND pid <> pg_backend_pid();"
```

---

## Cache Issues

### Redis Connection Refused

**Error**: `dial tcp 127.0.0.1:6379: connect: connection refused`

**Solution**:
```bash
# Start Redis
docker-compose up -d redis

# Verify running
redis-cli ping

# Check logs
docker-compose logs redis
```

### Redis Authentication Failed

**Error**: `NOAUTH Authentication required`

**Solution**:
```bash
# Check password in .env
cat .env | grep REDIS_PASSWORD

# Test connection
redis-cli -a "$REDIS_PASSWORD" ping
```

### Cache Full

**Error**: `OOM command not allowed when used memory > 'maxmemory'`

**Solution**:
```bash
# Check memory usage
redis-cli INFO memory | grep used_memory_human

# Clear cache
redis-cli FLUSHALL

# Increase max memory
docker-compose exec redis redis-cli CONFIG SET maxmemory 512mb

# Set eviction policy
docker-compose exec redis redis-cli CONFIG SET maxmemory-policy allkeys-lru
```

### Cache Miss Issues

**Symptom**: High cache miss rate

**Solution**:
```yaml
# Adjust cache settings
cache:
  ttl: 24h                     # Increase TTL
  similarity_threshold: 0.80   # Lower threshold for more hits
  max_entries: 50000           # Increase capacity
```

---

## Performance Problems

### High Latency

**Symptom**: Slow response times (>5s)

**Diagnosis**:
```bash
# Check provider latency
curl -w "@curl-timing.txt" -o /dev/null http://localhost:7061/v1/chat/completions -d '...'

# Check system resources
docker stats

# Check database queries
docker-compose exec postgres psql -U helixagent -c "SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 5;"
```

**Solutions**:
1. Enable caching
2. Use faster models
3. Reduce ensemble providers
4. Optimize database indexes
5. Increase connection pool

### Memory Leak

**Symptom**: Memory usage grows over time

**Diagnosis**:
```bash
# Monitor memory
docker stats --no-stream helixagent

# Check Go runtime stats
curl http://localhost:7061/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Solution**:
```yaml
# Limit cache size
cache:
  max_entries: 10000
  eviction_policy: lru

# Set Go GC percentage
GOGC=50 ./bin/helixagent
```

### CPU Spike

**Symptom**: High CPU usage

**Diagnosis**:
```bash
# Profile CPU
curl http://localhost:7061/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

**Solution**:
```yaml
# Limit concurrent requests
rate_limit:
  enabled: true
  requests_per_minute: 100

# Reduce workers
server:
  max_workers: 4
```

### Connection Pool Exhaustion

**Error**: `connection pool exhausted`

**Solution**:
```yaml
database:
  max_connections: 50
  max_idle_connections: 25
  connection_max_lifetime: 5m

http_client:
  max_idle_conns: 200
  max_idle_conns_per_host: 20
```

---

## Authentication Errors

### JWT Token Invalid

**Error**: `token is invalid` or `signature is invalid`

**Solution**:
```bash
# Check JWT secret
echo $JWT_SECRET

# Ensure secret matches between services
# All services must use same JWT_SECRET

# Regenerate token
curl -X POST http://localhost:7061/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'
```

### Token Expired

**Error**: `token has expired`

**Solution**:
```bash
# Get new token
curl -X POST http://localhost:7061/auth/refresh \
  -H "Authorization: Bearer $OLD_TOKEN"
```

```yaml
# Increase token lifetime in config
auth:
  jwt_expiry: 24h
  refresh_expiry: 7d
```

### CORS Errors

**Error**: `Access-Control-Allow-Origin` blocked

**Solution**:
```yaml
cors:
  enabled: true
  allowed_origins:
    - "http://localhost:3000"
    - "https://yourdomain.com"
  allowed_methods:
    - GET
    - POST
    - PUT
    - DELETE
  allowed_headers:
    - Authorization
    - Content-Type
```

---

## Ensemble Issues

### No Providers Available

**Error**: `no providers available for ensemble`

**Solution**:
```bash
# Check provider health
curl http://localhost:7061/api/v1/verifier/health/providers

# Verify API keys are set
env | grep API_KEY

# Check provider configuration
cat configs/development.yaml | grep -A5 providers
```

### Ensemble Timeout

**Error**: `ensemble timeout: not enough responses`

**Solution**:
```yaml
ensemble:
  timeout: 120s           # Increase timeout
  min_providers: 1        # Reduce minimum
  async: true             # Enable async mode
```

### Inconsistent Results

**Symptom**: Different responses for same query

**Solution**:
```yaml
ensemble:
  strategy: best_of_n     # More deterministic
  temperature: 0          # Reduce randomness
  seed: 12345            # Set seed for reproducibility
```

---

## Streaming Problems

### Stream Disconnects

**Symptom**: Stream stops mid-response

**Solution**:
```yaml
server:
  read_timeout: 0         # Disable read timeout for streams
  write_timeout: 0        # Disable write timeout for streams
  keep_alive: true

streaming:
  buffer_size: 4096
  heartbeat_interval: 30s
```

### No Data Received

**Symptom**: Stream opens but no events

**Diagnosis**:
```bash
# Test with verbose curl
curl -v -N -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"stream":true}'
```

**Solution**:
```bash
# Ensure Accept header is correct
-H "Accept: text/event-stream"

# Disable buffering in curl
-N or --no-buffer
```

### Proxy Buffering Issues

**Symptom**: Events arrive in batches

**Solution (nginx)**:
```nginx
location /v1/chat/completions {
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 600s;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
}
```

---

## Container Issues

### Docker Build Fails

**Error**: `failed to solve: process "/bin/sh -c..." did not complete successfully`

**Solution**:
```bash
# Build with no cache
docker build --no-cache -t helixagent .

# Check Dockerfile syntax
docker build --progress=plain -t helixagent .

# Use BuildKit
DOCKER_BUILDKIT=1 docker build -t helixagent .
```

### Container Won't Start

**Error**: Container exits immediately

**Solution**:
```bash
# Check logs
docker-compose logs helixagent

# Run interactively
docker-compose run --rm helixagent /bin/sh

# Check entrypoint
docker inspect helixagent | grep -A5 Entrypoint
```

### Podman Compatibility

**Error**: Various Podman-specific errors

**Solution**:
```bash
# Enable Podman socket
systemctl --user enable --now podman.socket

# Set DOCKER_HOST
export DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock

# Use podman-compose
pip install podman-compose
podman-compose up -d
```

### Volume Permission Issues

**Error**: `permission denied` on mounted volumes

**Solution**:
```bash
# Fix ownership
sudo chown -R $(id -u):$(id -g) ./data

# Or use namespaced volumes in Podman
podman run --userns=keep-id -v ./data:/data ...
```

### Network Issues Between Containers

**Error**: Containers can't communicate

**Solution**:
```bash
# Check network
docker network ls
docker network inspect helixagent_default

# Recreate network
docker-compose down
docker network prune
docker-compose up -d
```

---

## Quick Diagnostic Commands

```bash
# System health check
curl http://localhost:7061/health

# Provider status
curl http://localhost:7061/api/v1/verifier/health/providers

# Database connection
docker-compose exec postgres pg_isready

# Redis connection
redis-cli ping

# Container status
docker-compose ps

# Application logs
docker-compose logs -f helixagent

# Resource usage
docker stats
```

---

## Getting More Help

If these solutions don't resolve your issue:

1. **Enable Debug Logging**:
   ```yaml
   logging:
     level: debug
   ```

2. **Collect Diagnostics**:
   ```bash
   # System info
   uname -a
   docker version
   go version

   # Logs
   docker-compose logs > logs.txt

   # Configuration (redact secrets)
   cat configs/development.yaml
   ```

3. **Open Issue**:
   - GitHub: https://dev.helix.agent/issues
   - Include: error message, logs, steps to reproduce

4. **Community Support**:
   - Discord: https://discord.gg/helixagent
   - Documentation: https://helixagent.ai/docs
