# User Manual 08: Troubleshooting Guide

## Overview

This guide helps you diagnose and resolve common issues with HelixAgent. Issues are organized by category for quick reference.

## Quick Diagnostics

### Health Check

```bash
# Basic health check
curl http://localhost:8080/health

# Detailed health status
curl http://localhost:8080/health/detailed

# Provider status
curl http://localhost:8080/v1/providers
```

### Log Inspection

```bash
# View recent logs
docker logs helixagent --tail 100

# Follow logs
docker logs helixagent -f

# Filter by level
docker logs helixagent 2>&1 | grep -i error
```

## Startup Issues

### Service Won't Start

**Symptoms:**
- Container exits immediately
- "Address already in use" error
- Missing configuration error

**Solutions:**

1. **Check port availability:**
```bash
# Check if port is in use
lsof -i :8080

# Kill process using port
kill -9 $(lsof -t -i:8080)
```

2. **Verify configuration:**
```bash
# Check config file syntax
cat config/production.yaml | yq .

# Validate environment variables
env | grep -E '^(DB_|REDIS_|CLAUDE_|DEEPSEEK_)'
```

3. **Check database connectivity:**
```bash
# Test PostgreSQL connection
psql -h localhost -U helixagent -d helixagent_db -c "SELECT 1"

# Test Redis connection
redis-cli -h localhost ping
```

### Provider Verification Fails

**Symptoms:**
- "No providers verified" message
- All providers score 0
- Startup hangs during verification

**Solutions:**

1. **Check API keys:**
```bash
# Verify API key format
echo $CLAUDE_API_KEY | head -c 20

# Test API key directly
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $CLAUDE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-3-haiku-20240307","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'
```

2. **Check network connectivity:**
```bash
# Test API endpoints
curl -I https://api.anthropic.com
curl -I https://api.deepseek.com
curl -I https://api.openai.com
```

3. **Review verification logs:**
```bash
docker logs helixagent 2>&1 | grep -i "verif"
```

## API Issues

### Requests Return 500 Error

**Symptoms:**
- Internal server error response
- Requests work intermittently
- Timeout errors

**Solutions:**

1. **Check provider health:**
```bash
curl http://localhost:8080/health/detailed | jq '.providers'
```

2. **Review request format:**
```bash
# Validate JSON
echo '{"model":"helix-ensemble","messages":[{"role":"user","content":"hi"}]}' | jq .
```

3. **Check rate limits:**
```bash
curl -I http://localhost:8080/v1/chat/completions
# Look for X-RateLimit-* headers
```

### Streaming Not Working

**Symptoms:**
- SSE connection drops
- No streaming tokens received
- Connection timeout

**Solutions:**

1. **Check SSE headers:**
```bash
curl -N http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"model":"helix-ensemble","stream":true,"messages":[{"role":"user","content":"hi"}]}'
```

2. **Verify proxy configuration:**
```nginx
# nginx.conf - ensure these are set
proxy_buffering off;
proxy_cache off;
proxy_set_header Connection '';
proxy_http_version 1.1;
chunked_transfer_encoding off;
```

3. **Check WebSocket upgrade:**
```bash
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  http://localhost:8080/v1/ws/tasks/test
```

### AI Debate Not Responding

**Symptoms:**
- Debate stuck in "running" state
- No responses from participants
- Timeout during debate

**Solutions:**

1. **Check debate team status:**
```bash
curl http://localhost:8080/v1/debates/status
```

2. **Verify participant availability:**
```bash
curl http://localhost:8080/v1/providers | jq '.[] | select(.status == "healthy")'
```

3. **Review debate logs:**
```bash
docker logs helixagent 2>&1 | grep -i "debate"
```

## Database Issues

### Connection Failed

**Symptoms:**
- "Connection refused" error
- "Too many connections" error
- Query timeouts

**Solutions:**

1. **Check database status:**
```bash
# PostgreSQL
pg_isready -h localhost -p 5432

# Check connections
psql -c "SELECT count(*) FROM pg_stat_activity"
```

2. **Verify credentials:**
```bash
psql "host=localhost port=5432 user=$DB_USER password=$DB_PASSWORD dbname=$DB_NAME" -c "SELECT 1"
```

3. **Reset connections:**
```bash
# Restart PostgreSQL
docker restart postgres

# Or force disconnect
psql -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'helixagent_db'"
```

### Migration Failures

**Symptoms:**
- Schema version mismatch
- Missing tables
- Column not found errors

**Solutions:**

1. **Check migration status:**
```bash
psql -d helixagent_db -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5"
```

2. **Run pending migrations:**
```bash
make migrate-up
```

3. **Fix broken migration:**
```bash
# Roll back last migration
make migrate-down

# Check and fix migration file, then re-run
make migrate-up
```

## Redis Issues

### Cache Not Working

**Symptoms:**
- High latency on cached requests
- "Connection refused" to Redis
- Memory issues

**Solutions:**

1. **Check Redis connectivity:**
```bash
redis-cli -h localhost ping
redis-cli -h localhost info memory
```

2. **Clear cache if corrupted:**
```bash
redis-cli FLUSHDB
```

3. **Check memory usage:**
```bash
redis-cli INFO memory | grep used_memory_human
```

## Performance Issues

### Slow Response Times

**Symptoms:**
- Requests take > 10 seconds
- CPU usage high
- Memory growing

**Solutions:**

1. **Enable profiling:**
```bash
curl http://localhost:8080/debug/pprof/profile?seconds=30 > profile.out
go tool pprof profile.out
```

2. **Check resource usage:**
```bash
docker stats helixagent
```

3. **Review slow queries:**
```sql
SELECT query, calls, total_time/calls as avg_time
FROM pg_stat_statements
ORDER BY total_time DESC
LIMIT 10;
```

### Memory Leaks

**Symptoms:**
- Memory usage grows over time
- OOM kills
- Degrading performance

**Solutions:**

1. **Check Go memory stats:**
```bash
curl http://localhost:8080/debug/pprof/heap > heap.out
go tool pprof heap.out
```

2. **Set memory limits:**
```yaml
# docker-compose.yaml
services:
  helixagent:
    deploy:
      resources:
        limits:
          memory: 2G
```

3. **Enable garbage collection logging:**
```bash
GODEBUG=gctrace=1 ./helixagent
```

## Protocol Issues

### MCP Tools Not Found

**Symptoms:**
- "Tool not available" error
- Empty tool list
- Tool execution fails

**Solutions:**

1. **Check tool registration:**
```bash
curl http://localhost:8080/v1/mcp/tools | jq '.tools[].name'
```

2. **Verify tool configuration:**
```yaml
# config/mcp.yaml
mcp:
  tools:
    - read_file
    - write_file
```

3. **Check tool permissions:**
```bash
# Verify file access
ls -la /path/to/allowed/directory
```

### LSP Server Not Responding

**Symptoms:**
- No completions returned
- "Server not initialized" error
- Timeout on LSP requests

**Solutions:**

1. **Check language server:**
```bash
# Verify server is installed
which gopls
gopls version
```

2. **Test server directly:**
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | gopls
```

3. **Check server logs:**
```bash
docker logs helixagent 2>&1 | grep -i "lsp"
```

## Common Error Codes

| Code | Meaning | Solution |
|------|---------|----------|
| 400 | Bad Request | Check request format |
| 401 | Unauthorized | Verify API key |
| 403 | Forbidden | Check permissions |
| 404 | Not Found | Verify endpoint |
| 429 | Rate Limited | Wait and retry |
| 500 | Server Error | Check logs |
| 502 | Bad Gateway | Check upstream |
| 503 | Unavailable | Check service health |
| 504 | Gateway Timeout | Increase timeout |

## Getting Help

### Collect Diagnostic Information

```bash
# Generate diagnostic bundle
./scripts/collect-diagnostics.sh > diagnostics.tar.gz

# This collects:
# - System info
# - HelixAgent logs
# - Configuration (sanitized)
# - Health check results
# - Resource usage
```

### Log Levels

```bash
# Set debug logging
LOG_LEVEL=debug ./helixagent

# Available levels:
# - debug: Verbose debugging
# - info: Normal operation
# - warn: Warnings only
# - error: Errors only
```

### Community Support

- GitHub Issues: https://github.com/helix-agent/issues
- Documentation: https://docs.helixagent.ai
- Discord: https://discord.gg/helixagent

## Preventive Measures

### Health Monitoring

```bash
# Set up health check cron
*/5 * * * * curl -s http://localhost:8080/health | grep -q '"status":"healthy"' || alert.sh
```

### Log Rotation

```yaml
# docker-compose.yaml
services:
  helixagent:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"
```

### Backup Configuration

```bash
# Daily config backup
0 0 * * * tar -czf /backups/config-$(date +%Y%m%d).tar.gz /etc/helixagent/
```

## Next Steps

- [Getting Started Guide](01-getting-started.md)
- [Provider Configuration](02-provider-configuration.md)
- [Deployment Guide](05-deployment-guide.md)
