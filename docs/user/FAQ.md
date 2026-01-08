# HelixAgent Frequently Asked Questions

## General Questions

### What is HelixAgent?

HelixAgent is an AI-powered ensemble LLM service that combines responses from multiple language models using intelligent aggregation strategies. It provides OpenAI-compatible APIs, supports 7+ LLM providers, and includes advanced features like AI debates and model verification.

### How is HelixAgent different from using a single LLM provider?

HelixAgent offers several advantages over using a single provider:
- **Better Accuracy**: Ensemble mode aggregates responses from multiple models
- **Higher Availability**: Automatic failover between providers
- **Cost Optimization**: Route to the most cost-effective provider
- **Feature Comparison**: Test and compare models with built-in verification
- **AI Debates**: Get multiple perspectives on complex questions

### Is HelixAgent compatible with OpenAI's API?

Yes, HelixAgent provides full OpenAI API compatibility. You can use existing OpenAI client libraries by changing the base URL to your HelixAgent server.

### What LLM providers are supported?

HelixAgent supports:
- OpenAI (GPT-4, GPT-3.5)
- Anthropic (Claude 3)
- Google (Gemini)
- DeepSeek
- Qwen
- ZAI
- Ollama (local models)
- OpenRouter (100+ models)
- AWS Bedrock
- GCP Vertex AI
- Azure OpenAI

### Can I run HelixAgent locally without cloud providers?

Yes, you can use Ollama for completely local inference:
```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull a model
ollama pull llama2

# Enable in HelixAgent
OLLAMA_ENABLED=true
OLLAMA_BASE_URL=http://localhost:11434
```

---

## Setup and Configuration

### What are the system requirements?

**Minimum**:
- 2GB RAM
- 2 CPU cores
- 1GB storage
- Docker or Go 1.23+

**Recommended**:
- 4GB+ RAM
- 4+ CPU cores
- 10GB storage (for logs/cache)
- SSD storage

### How do I configure multiple providers?

Add API keys to your `.env` file:
```bash
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
```

Or configure in `configs/production.yaml`:
```yaml
providers:
  openai:
    enabled: true
    api_key: ${OPENAI_API_KEY}
  anthropic:
    enabled: true
    api_key: ${ANTHROPIC_API_KEY}
```

### How do I set up high availability?

For production deployments:
1. Run multiple HelixAgent instances
2. Use a load balancer (nginx, HAProxy)
3. Configure shared PostgreSQL and Redis
4. Enable health checks

```yaml
# In each instance
server:
  instance_id: helixagent-1  # Unique per instance

database:
  host: shared-postgres.example.com

cache:
  host: shared-redis.example.com
```

### Can I use HelixAgent with Kubernetes?

Yes, see `docs/deployment/kubernetes-deployment.md` for:
- Helm charts
- Deployment manifests
- Horizontal Pod Autoscaler configuration
- Service mesh integration

---

## Ensemble Mode

### What ensemble strategies are available?

| Strategy | Description | Best For |
|----------|-------------|----------|
| `confidence_weighted` | Weight by provider confidence | General use |
| `majority_vote` | Most common response wins | Fact checking |
| `best_of_n` | Highest scored response | Quality focus |
| `fastest` | First response wins | Low latency |

### How do I customize ensemble weights?

```json
{
  "ensemble_config": {
    "strategy": "confidence_weighted",
    "weights": {
      "openai": 1.0,
      "anthropic": 0.9,
      "google": 0.8
    }
  }
}
```

### Can I exclude specific providers from ensemble?

Yes, use `preferred_providers` or `excluded_providers`:
```json
{
  "ensemble_config": {
    "preferred_providers": ["openai", "anthropic"],
    "excluded_providers": ["google"]
  }
}
```

### What happens if a provider fails during ensemble?

HelixAgent automatically handles failures:
1. Retries failed requests (configurable)
2. Proceeds with remaining providers
3. Returns result if `min_providers` threshold met
4. Falls back to single provider if necessary

---

## AI Debate System

### What is the AI Debate system?

The AI Debate system enables multiple AI models to debate a topic across several rounds. This helps explore different perspectives and reach well-reasoned conclusions.

### When should I use AI Debate vs Ensemble?

| Use Case | Feature |
|----------|---------|
| Quick answers | Ensemble |
| Complex decisions | AI Debate |
| Fact checking | Ensemble (majority) |
| Exploring perspectives | AI Debate |
| Low latency needed | Ensemble |

### How long does a debate take?

Typical debate duration:
- 2 participants, 3 rounds: 30-60 seconds
- 3 participants, 5 rounds: 60-120 seconds

Configure timeout:
```json
{
  "timeout": 300,
  "max_rounds": 3
}
```

### Can I have more than 2 debate participants?

Yes, debates support 2-5 participants:
```json
{
  "participants": [
    {"provider": "openai", "model": "gpt-4"},
    {"provider": "anthropic", "model": "claude-3-opus"},
    {"provider": "google", "model": "gemini-pro"}
  ]
}
```

---

## Model Verification

### What does LLMsVerifier test?

- **Existence**: Model is available and accessible
- **Responsiveness**: Response time and reliability
- **Streaming**: Streaming capability works
- **Code Visibility**: Model understands code context
- **Consistency**: Responses are stable

### How are model scores calculated?

Default scoring weights:
- Response Speed: 30%
- Model Efficiency: 20%
- Cost Effectiveness: 25%
- Capability: 15%
- Recency: 10%

Customize with:
```bash
curl -X PUT http://localhost:7061/api/v1/verifier/scores/weights \
  -H "Content-Type: application/json" \
  -d '{"response_speed": 0.4, "capability": 0.6}'
```

### How often should I verify models?

- Production critical: Daily
- Regular use: Weekly
- Development: Before deployment

Automate with cron:
```bash
0 6 * * * curl -X POST http://localhost:7061/api/v1/verifier/batch-verify -d '...'
```

---

## Performance and Caching

### How does semantic caching work?

HelixAgent caches responses based on semantic similarity. Similar queries hit the cache instead of calling providers.

Configure similarity threshold:
```yaml
cache:
  similarity_threshold: 0.85  # 85% similar = cache hit
  ttl: 24h
```

### What cache hit rate should I expect?

Typical hit rates:
- FAQ/support: 40-60%
- Code assistance: 20-30%
- Creative tasks: 5-10%

Monitor with:
```bash
curl http://localhost:7061/metrics | grep cache_hit
```

### How do I improve response latency?

1. **Enable caching**: Semantic cache for repeated queries
2. **Use faster models**: gpt-3.5-turbo instead of gpt-4
3. **Reduce ensemble size**: Fewer providers = faster
4. **Geographic proximity**: Choose nearby provider regions
5. **Connection pooling**: Reuse HTTP connections

### What's the memory footprint?

Base: ~100MB
With cache (10k entries): ~500MB
Full stack (with DB/Redis): ~2GB

---

## Security

### How is authentication handled?

HelixAgent supports:
- JWT tokens (default)
- API keys
- OAuth 2.0
- Custom middleware

Configure in production:
```yaml
security:
  jwt_enabled: true
  jwt_secret: ${JWT_SECRET}  # 256-bit min
  api_key_enabled: true
```

### Are API keys stored securely?

Provider API keys are:
- Stored in environment variables or secrets manager
- Never logged or exposed in responses
- Encrypted in database if stored
- Excluded from JSON serialization

### Does HelixAgent support rate limiting?

Yes, configurable per user/IP:
```yaml
rate_limit:
  enabled: true
  requests_per_minute: 60
  burst: 10
  by_user: true
```

### How do I secure a production deployment?

1. Use HTTPS (TLS termination at load balancer)
2. Enable authentication
3. Configure CORS for your domains
4. Set up rate limiting
5. Use secrets manager for API keys
6. Enable audit logging
7. Regular security updates

---

## SDKs and Integration

### Which SDKs are available?

- Python: `pip install helixagent-py`
- Go: `go get dev.helix.agent-go`
- JavaScript: `npm install helixagent-js`
- iOS: Swift Package Manager
- Android: Maven/Gradle

### Can I use OpenAI's official SDK?

Yes, just change the base URL:

**Python**:
```python
from openai import OpenAI
client = OpenAI(
    api_key="your-key",
    base_url="http://localhost:7061/v1"
)
```

**JavaScript**:
```javascript
import OpenAI from 'openai';
const client = new OpenAI({
  apiKey: 'your-key',
  baseURL: 'http://localhost:7061/v1',
});
```

### How do I handle streaming in my application?

All SDKs support streaming:
```python
stream = client.chat.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}],
    stream=True
)
for chunk in stream:
    print(chunk.choices[0].delta.content, end="")
```

---

## Troubleshooting

### Why am I getting 401 Unauthorized?

Common causes:
1. Missing Authorization header
2. Invalid or expired token
3. Wrong API key format
4. Provider key revoked

Debug:
```bash
# Test without auth (if enabled)
curl http://localhost:7061/health

# Test with auth
curl -H "Authorization: Bearer $TOKEN" http://localhost:7061/v1/models
```

### Why are ensemble requests slow?

Check:
1. Provider latencies: `GET /api/v1/verifier/health/providers`
2. Number of providers: Reduce `min_providers`
3. Network issues: Check connectivity to providers
4. Timeout settings: Increase if needed

### Why is my cache not working?

Verify:
1. Redis is running: `redis-cli ping`
2. Cache enabled in config
3. TTL not too short
4. Similarity threshold appropriate

Test:
```bash
# Same query twice
curl -X POST ... -d '{"messages":[{"role":"user","content":"Hello"}]}'
# Second should be faster (cache hit)
```

### How do I report a bug?

1. Check existing issues: https://dev.helix.agent/issues
2. Enable debug logging
3. Collect: error message, logs, steps to reproduce
4. Open issue with template

---

## Pricing and Licensing

### What is the license?

HelixAgent is open source under the MIT License. You can use it freely for personal and commercial projects.

### What are the costs?

HelixAgent itself is free. Costs come from:
- LLM provider API usage (OpenAI, Anthropic, etc.)
- Infrastructure (servers, databases)
- Optional: Managed cloud hosting

### Is there a managed/hosted version?

Check https://helixagent.ai for managed hosting options with:
- No infrastructure management
- Automatic scaling
- 24/7 support
- SLA guarantees

---

## Getting Help

### Where can I get support?

- Documentation: https://helixagent.ai/docs
- GitHub Issues: https://dev.helix.agent/issues
- Discord: https://discord.gg/helixagent
- Email: support@helixagent.ai

### How do I contribute?

1. Fork the repository
2. Create feature branch
3. Make changes with tests
4. Submit pull request
5. See CONTRIBUTING.md for details

### Where can I find more examples?

- `examples/` directory in repository
- API documentation: `docs/api/`
- SDK documentation: `docs/sdk/`
- Tutorial videos: https://helixagent.ai/tutorials
