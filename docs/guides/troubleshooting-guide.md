# HelixAgent Troubleshooting Guide

## 🚨 Quick Diagnosis

### First Steps
1. **Check if HelixAgent is running**: `curl http://localhost:7061/health`
2. **Check logs**: `docker-compose logs helixagent` or `tail -f logs/helixagent.log`
3. **Verify configuration**: `make validate-config`

---

## 🔍 Common Issues & Solutions

### 1. **HelixAgent Won't Start**

#### Symptoms:
- "Port already in use" error
- "Database connection failed"
- "Invalid configuration"

#### Solutions:

**Port Conflict:**
```bash
# Check what's using port 8080
sudo lsof -i :7061

# Kill the process or change HelixAgent port
export PORT=8081
make run-dev
```

**Database Connection:**
```bash
# Check PostgreSQL is running
docker-compose ps postgres

# Check PostgreSQL logs
docker-compose logs postgres

# Test database connection
psql -h localhost -U helixagent -d helixagent_db

# Reset database (development only)
make reset-db
```

**Configuration Errors:**
```bash
# Validate configuration
make validate-config

# Check environment variables
make check-env

# Generate configuration report
make config-report
```

### 2. **API Requests Fail**

#### Symptoms:
- "401 Unauthorized" errors
- "404 Not Found" for endpoints
- "500 Internal Server Error"

#### Solutions:

**Authentication Issues:**
```bash
# Test authentication endpoint
curl -X POST http://localhost:7061/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password"}'

# Check JWT secret in .env
echo $JWT_SECRET

# Generate new JWT secret
openssl rand -base64 32
```

**Endpoint Not Found:**
```bash
# List all available endpoints
curl http://localhost:7061/v1/health

# Check router configuration
make show-routes

# Verify API version in request
# Should be /v1/endpoint, not /endpoint
```

**Internal Server Errors:**
```bash
# Check application logs
tail -f logs/helixagent.log

# Enable debug logging
export LOG_LEVEL=debug
make restart

# Check for panic in logs
grep -i panic logs/helixagent.log
```

### 3. **LLM Provider Issues**

#### Symptoms:
- "Provider not available"
- "API key invalid"
- "Rate limit exceeded"
- "Timeout waiting for response"

#### Solutions:

**Provider Not Available:**
```bash
# List available providers
curl http://localhost:7061/v1/providers

# Check provider health
curl http://localhost:7061/v1/providers/claude/health

# Verify API keys in .env
cat .env | grep API_KEY

# Test provider connectivity
make test-providers
```

**Invalid API Keys:**
```bash
# Test API key directly with provider
curl -X POST https://api.anthropic.com/v1/messages \
  -H "x-api-key: $CLAUDE_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-3-sonnet-20240229","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}'

# Regenerate API keys from provider dashboards
# Claude: https://console.anthropic.com/
# DeepSeek: https://platform.deepseek.com/
# Gemini: https://makersuite.google.com/app/apikey
```

**Rate Limits:**
```bash
# Check provider rate limits
# Claude: 10,000 requests/month (free tier)
# DeepSeek: 1,000 requests/day (free tier)
# Gemini: 60 requests/minute

# Implement request queuing
export REQUEST_QUEUE_SIZE=100
export REQUEST_TIMEOUT=30

# Use multiple API keys (if available)
export CLAUDE_API_KEY_2=sk-ant-api03-your-second-key
```

**Timeouts:**
```bash
# Increase timeout settings in config
providers:
  claude:
    timeout: 60  # Increase from 30
    max_retries: 5
  
  ensemble:
    timeout_per_provider: 30
    max_total_timeout: 90

# Check network connectivity
ping api.anthropic.com
curl -I https://api.anthropic.com
```

### 4. **Database Issues**

#### Symptoms:
- "Database connection lost"
- "Migration failed"
- "Query timeout"

#### Solutions:

**Connection Issues:**
```bash
# Check PostgreSQL status
docker-compose exec postgres pg_isready

# Check connection pool
export DB_MAX_CONNECTIONS=100
export DB_CONNECTION_TIMEOUT=30

# Reset connection pool
make restart-db
```

**Migration Problems:**
```bash
# Run migrations manually
make migrate-up

# Check migration status
make migrate-status

# Rollback last migration
make migrate-down

# Reset all migrations (development only)
make migrate-reset
```

**Performance Issues:**
```bash
# Check database size
docker-compose exec postgres psql -U helixagent -c "SELECT pg_size_pretty(pg_database_size('helixagent_db'));"

# Check slow queries
docker-compose exec postgres psql -U helixagent -c "SELECT query, calls, total_time, mean_time FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"

# Add indexes for performance
make optimize-db
```

### 5. **Memory & Performance Issues**

#### Symptoms:
- High memory usage
- Slow response times
- Out of memory errors

#### Solutions:

**Memory Leaks:**
```bash
# Monitor memory usage
docker stats

# Set memory limits
export MEMORY_LIMIT_MB=1024
export GC_PERCENT=100

# Enable memory profiling
export ENABLE_PPROF=true
# Then access: http://localhost:7061/debug/pprof/heap
```

**Performance Bottlenecks:**
```bash
# Enable performance profiling
export ENABLE_TRACING=true
export TRACING_SAMPLE_RATE=0.1

# Check response times
curl -w "\\nTime: %{time_total}s\\n" http://localhost:7061/health

# Monitor with Prometheus
curl http://localhost:7061/metrics | grep helixagent
```

**Cache Issues:**
```bash
# Check Redis connectivity
docker-compose exec redis redis-cli ping

# Clear cache
make clear-cache

# Monitor cache hit rate
curl http://localhost:7061/metrics | grep cache
```

### 6. **Authentication & Security Issues**

#### Symptoms:
- JWT tokens not working
- Rate limiting too aggressive
- CORS errors

#### Solutions:

**JWT Token Problems:**
```bash
# Verify JWT secret
echo "JWT_SECRET length: ${#JWT_SECRET}"

# Decode JWT token (for debugging)
# Use https://jwt.io/ or:
echo "YOUR_JWT_TOKEN" | cut -d '.' -f 2 | base64 -d | jq

# Generate new tokens
curl -X POST http://localhost:7061/v1/auth/refresh \
  -H "Authorization: Bearer YOUR_REFRESH_TOKEN"
```

**Rate Limiting:**
```bash
# Adjust rate limits
export RATE_LIMIT_REQUESTS_PER_MINUTE=120
export RATE_LIMIT_BURST_SIZE=20

# Check current rate limits
curl -I http://localhost:7061/v1/chat/completions

# Implement client-side retry with exponential backoff
```

**CORS Issues:**
```bash
# Configure CORS in .env
export CORS_ALLOWED_ORIGINS="http://localhost:3000,https://yourdomain.com"
export CORS_ALLOWED_METHODS="GET,POST,PUT,DELETE,OPTIONS"

# Test CORS headers
curl -I -X OPTIONS http://localhost:7061/v1/chat/completions
```

### 7. **Deployment Issues**

#### Symptoms:
- Docker build fails
- Kubernetes pods crash
- Load balancer configuration issues

#### Solutions:

**Docker Build:**
```bash
# Clean build cache
docker system prune -a

# Build with no cache
docker-compose build --no-cache

# Check Dockerfile syntax
docker build --target builder -t helixagent-builder .

# Test multi-stage build
make docker-test
```

**Kubernetes:**
```bash
# Check pod status
kubectl get pods -n helixagent

# Check pod logs
kubectl logs -f deployment/helixagent -n helixagent

# Check resource limits
kubectl describe deployment helixagent -n helixagent

# Debug with exec
kubectl exec -it deployment/helixagent -n helixagent -- /bin/sh
```

**Load Balancer:**
```bash
# Check service endpoints
kubectl get endpoints -n helixagent

# Test load balancer
curl http://loadbalancer-ip/health

# Check ingress configuration
kubectl get ingress -n helixagent
kubectl describe ingress helixagent -n helixagent
```

---

## 🛠️ Diagnostic Tools

### Built-in Diagnostics

```bash
# Run comprehensive diagnostics
make diagnose

# Check system health
make health-check

# Generate diagnostic report
make diagnostic-report
```

### Log Analysis

```bash
# Tail logs in real-time
make logs

# Search for errors
grep -i error logs/helixagent.log

# Search for specific patterns
grep "provider.*failed" logs/helixagent.log

# Analyze log patterns
make analyze-logs
```

### Performance Testing

```bash
# Run load test
make load-test

# Run stress test
make stress-test

# Benchmark endpoints
make benchmark

# Generate performance report
make performance-report
```

---

## 📊 Monitoring & Metrics

### Key Metrics to Monitor

1. **Response Time**: `helixagent_request_duration_seconds`
2. **Error Rate**: `helixagent_request_errors_total`
3. **Provider Health**: `helixagent_provider_health`
4. **Memory Usage**: `process_resident_memory_bytes`
5. **Database Connections**: `db_connections_active`

### Access Metrics

```bash
# View Prometheus metrics
curl http://localhost:7061/metrics

# View health endpoint
curl http://localhost:7061/v1/health

# View detailed health
curl http://localhost:7061/v1/health/detailed
```

### Grafana Dashboards

Access: http://localhost:3000
- Username: `admin`
- Password: `admin`

Key dashboards:
1. **HelixAgent Overview**: Overall system health
2. **Provider Performance**: LLM provider metrics
3. **API Analytics**: Request/response statistics
4. **Database Metrics**: PostgreSQL performance
5. **System Resources**: CPU, memory, disk usage

---

## 🔧 Advanced Troubleshooting

### Debug Mode

```bash
# Enable debug mode
export LOG_LEVEL=debug
export DEBUG=true
export ENABLE_PPROF=true

# Restart with debug
make restart

# Access debug endpoints
# Profiling: http://localhost:7061/debug/pprof/
# Metrics: http://localhost:7061/debug/vars
# Config: http://localhost:7061/debug/config
```

### Network Diagnostics

```bash
# Test external connectivity
make test-connectivity

# Check DNS resolution
nslookup api.anthropic.com

# Test SSL certificates
openssl s_client -connect api.anthropic.com:443

# Check firewall rules
sudo ufw status
```

### Database Diagnostics

```bash
# Run database diagnostics
make db-diagnose

# Check table sizes
make db-stats

# Analyze query performance
make db-analyze

# Backup before troubleshooting
make db-backup
```

### Provider-Specific Issues

**Claude:**
```bash
# Check Claude API status
curl https://status.anthropic.com/

# Test Claude API directly
curl -X POST https://api.anthropic.com/v1/messages \
  -H "x-api-key: $CLAUDE_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-3-haiku-20240307","max_tokens":10,"messages":[{"role":"user","content":"test"}]}'
```

**DeepSeek:**
```bash
# Check DeepSeek API status
curl https://status.deepseek.com/

# Test DeepSeek API
curl -X POST https://api.deepseek.com/chat/completions \
  -H "Authorization: Bearer $DEEPSEEK_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-chat","messages":[{"role":"user","content":"test"}]}'
```

**Gemini:**
```bash
# Check Gemini API status
curl https://status.cloud.google.com/

# Test Gemini API
curl -X POST https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=$GEMINI_API_KEY \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"test"}]}]}'
```

---

## 🚑 Emergency Procedures

### Service Unavailable

1. **Check all services**: `docker-compose ps` or `kubectl get pods`
2. **Restart services**: `make restart` or `kubectl rollout restart deployment`
3. **Check resource limits**: `docker stats` or `kubectl top pods`
4. **Review recent changes**: `git log --oneline -10`

### Data Corruption

1. **Stop services**: `make stop` or `kubectl scale deployment --replicas=0`
2. **Backup data**: `make backup` or database backup
3. **Restore from backup**: `make restore` or database restore
4. **Verify data integrity**: `make verify-data`

### Security Breach

1. **Rotate all secrets**: `make rotate-secrets`
2. **Review access logs**: `make audit-logs`
3. **Check for intrusions**: `make security-scan`
4. **Update dependencies**: `make update-deps`

---

## 📚 Prevention & Best Practices

### Regular Maintenance

```bash
# Daily checks
make daily-check

# Weekly maintenance
make weekly-maintenance

# Monthly optimization
make monthly-optimize
```

### Monitoring Setup

1. **Enable alerts** for critical metrics
2. **Set up log aggregation** (ELK stack, Loki)
3. **Implement APM** (OpenTelemetry, Datadog)
4. **Regular backup** of configurations and data

### Capacity Planning

1. **Monitor growth trends**
2. **Plan for scaling** (horizontal/vertical)
3. **Test failure scenarios**
4. **Document recovery procedures**

---

## 🆘 Getting Help

### Self-Help Resources

1. **Documentation**: `/docs/` directory
2. **API Reference**: `/docs/api-documentation.md`
3. **Examples**: `/docs/api-reference-examples.md`
4. **Configuration Guide**: `/docs/user/configuration-guide.md`

### Community Support

1. **GitHub Issues**: https://dev.helix.agent/issues
2. **Discord Community**: [Link in README]
3. **Stack Overflow**: Tag `helixagent`

### Professional Support

1. **Enterprise Support**: Contact sales@helixagent.ai
2. **Consulting Services**: Implementation and optimization
3. **Training**: Custom workshops and training

---

## 📋 Troubleshooting Checklist

### Quick Checklist
- [ ] HelixAgent running? `curl http://localhost:7061/health`
- [ ] Database connected? `docker-compose ps postgres`
- [ ] API keys valid? `make test-providers`
- [ ] Enough resources? `docker stats` or `kubectl top pods`
- [ ] Recent changes? `git log --oneline -5`
- [ ] Error logs? `tail -f logs/helixagent.log`

### Detailed Checklist
- [ ] Network connectivity
- [ ] DNS resolution
- [ ] SSL certificates
- [ ] Firewall rules
- [ ] Resource limits
- [ ] Configuration validation
- [ ] Dependency versions
- [ ] Security patches

---

**Remember**: Always backup before making significant changes, test in staging first, and document your troubleshooting steps for future reference.

**Next**: Review [Best Practices Guide](./best-practices-guide.md) to prevent issues before they occur.