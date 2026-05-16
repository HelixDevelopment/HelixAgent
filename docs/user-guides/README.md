# User Guides

This directory contains user-facing guides, how-to articles, and troubleshooting documentation for HelixAgent.

## Overview

These guides are designed for end users, developers integrating with HelixAgent, and operators managing HelixAgent deployments. Each guide provides step-by-step instructions with practical examples.

## Documentation Index

### Protocol Guides

| Document | Description |
|----------|-------------|
| [Protocols Comprehensive Guide](./PROTOCOLS_COMPREHENSIVE_GUIDE.md) | Complete guide covering MCP, LSP, ACP, Embeddings, Vision, and Cognee protocols |

## How-To Articles

### Getting Started

1. **Initial Setup**
   - Install HelixAgent binary or Docker image
   - Configure environment variables
   - Start the server and verify health

2. **First API Call**
   ```bash
   curl -X POST http://localhost:7061/v1/chat/completions \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer your-api-key" \
     -d '{
       "model": "helixagent-ensemble",
       "messages": [{"role": "user", "content": "Hello, HelixAgent!"}]
     }'
   ```

3. **Configure LLM Providers**
   - Set API keys for desired providers
   - Verify provider health via `/v1/monitoring/status`

### Common Tasks

#### Using Ensemble Mode

Combine responses from multiple LLMs for better quality:

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-ensemble",
    "messages": [{"role": "user", "content": "Your question"}],
    "ensemble_config": {
      "strategy": "confidence_weighted",
      "providers": ["claude", "deepseek", "gemini"]
    }
  }'
```

#### Running AI Debates

For complex questions requiring multi-perspective analysis:

```bash
curl -X POST http://localhost:7061/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "helixagent-debate",
    "messages": [{"role": "user", "content": "Should we use microservices?"}],
    "debate_config": {
      "rounds": 3,
      "participants": 5,
      "strategy": "structured"
    }
  }'
```

#### Using MCP Tools

Access external tools via MCP protocol:

```bash
# List available tools
curl http://localhost:7061/v1/mcp/tools

# Execute a tool
curl -X POST http://localhost:7061/v1/mcp/execute \
  -H "Content-Type: application/json" \
  -d '{
    "tool": "filesystem",
    "action": "read_file",
    "params": {"path": "/path/to/file"}
  }'
```

#### Generating Embeddings

Create vector embeddings for semantic search:

```bash
curl -X POST http://localhost:7061/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-ada-002",
    "input": "Your text to embed"
  }'
```

### Configuration Guides

#### Environment Variables

Essential environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No | Server port (default: 7061) |
| `GIN_MODE` | No | Gin mode: debug/release |
| `JWT_SECRET` | Yes | JWT signing secret |
| `DB_HOST`, `DB_PORT`, etc. | Yes | Database connection |
| `REDIS_HOST`, `REDIS_PORT` | Yes | Redis connection |
| `*_API_KEY` | Varies | Provider API keys |

#### Configuration Files

Main configuration files:
- `configs/development.yaml` - Development settings
- `configs/production.yaml` - Production settings

## Troubleshooting

### Common Issues

#### Provider Connection Failures

**Symptoms**: Requests fail with "provider unavailable" errors

**Solutions**:
1. Verify API key is set correctly
2. Check provider health: `curl http://localhost:7061/v1/monitoring/provider-health`
3. Review circuit breaker state: `curl http://localhost:7061/v1/monitoring/circuit-breakers`
4. Check network connectivity to provider endpoints

#### High Latency

**Symptoms**: Slow response times

**Solutions**:
1. Enable caching for repeated queries
2. Reduce ensemble provider count
3. Check MCP server performance
4. Review worker pool configuration

#### Memory Issues

**Symptoms**: High memory usage or OOM errors

**Solutions**:
1. Reduce cache sizes
2. Limit concurrent connections
3. Enable memory profiling to identify leaks
4. Configure swap or increase memory

#### Database Connection Errors

**Symptoms**: Database query failures

**Solutions**:
1. Verify database credentials
2. Check connection pool settings
3. Ensure database is accessible from HelixAgent
4. Review connection timeout settings

### Diagnostic Commands

```bash
# Check system health
curl http://localhost:7061/v1/monitoring/status

# View provider status
curl http://localhost:7061/v1/monitoring/provider-health

# Check circuit breakers
curl http://localhost:7061/v1/monitoring/circuit-breakers

# View recent errors
curl http://localhost:7061/v1/monitoring/errors

# Run health challenge
./challenges/scripts/full_system_boot_challenge.sh
```

## FAQ

### General Questions

**Q: What LLM providers are supported?**

A: HelixAgent supports 10 providers: Claude, DeepSeek, Gemini, Mistral, OpenRouter, Qwen, ZAI, Zen, Cerebras, and Ollama.

**Q: Is HelixAgent OpenAI API compatible?**

A: Yes, HelixAgent provides 100% OpenAI API compatibility at `/v1/chat/completions`.

**Q: Can I use HelixAgent with existing OpenAI SDKs?**

A: Yes, simply change the base URL to your HelixAgent instance.

### Performance Questions

**Q: How fast is the ensemble response?**

A: Typical ensemble latency is 500ms-1200ms depending on provider count and response length.

**Q: Does caching work with streaming responses?**

A: Streaming responses are cached after completion for subsequent non-streaming requests.

### Security Questions

**Q: How are API keys stored?**

A: API keys are stored in environment variables and never logged or exposed in responses.

**Q: Does HelixAgent support rate limiting?**

A: Yes, configurable rate limiting is available per client and globally.

### Integration Questions

**Q: Can I run HelixAgent in Kubernetes?**

A: Yes, Helm charts and Kubernetes manifests are provided in `deploy/k8s/`.

**Q: Does HelixAgent support horizontal scaling?**

A: Yes, with Redis for distributed caching and PostgreSQL for shared state.

## Getting Help

- **Documentation**: Check the [full documentation](../README.md)
- **GitHub Issues**: Report bugs at [GitHub Issues](https://github.com/HelixDevelopment/helix_agent/issues)
- **Discussions**: Ask questions in [GitHub Discussions](https://github.com/HelixDevelopment/helix_agent/discussions)

## Related Documentation

- [API Reference](../api/README.md)
- [Configuration Guide](../guides/configuration-guide.md)
- [Deployment Guide](../deployment/README.md)
- [SDK Documentation](../sdk/README.md)
