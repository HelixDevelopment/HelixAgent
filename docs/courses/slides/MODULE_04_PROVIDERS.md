# Module 4: LLM Provider Integration

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module 4: LLM Provider Integration
- Duration: 75 minutes
- Mastering Multi-Provider AI

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Integrate all supported LLM providers
- Understand provider capabilities and limitations
- Implement reliable fallback chains
- Optimize provider selection for your use case

---

## Slide 3: Provider Architecture

**LLMProvider Interface:**

```go
type LLMProvider interface {
    // Generate completion
    Complete(ctx context.Context, req *Request) (*Response, error)

    // Stream completion
    CompleteStream(ctx context.Context, req *Request) (chan StreamChunk, error)

    // Health verification
    HealthCheck(ctx context.Context) error

    // Capabilities info
    GetCapabilities() *Capabilities

    // Config validation
    ValidateConfig() error
}
```

---

## Slide 4: Supported Providers Overview

**7 Providers Available:**

| Provider | Best For | Cost Tier |
|----------|----------|-----------|
| Claude | Reasoning, Analysis | Medium-High |
| DeepSeek | Code, Technical | Low |
| Gemini | Multimodal, Scientific | Medium |
| Qwen | Multilingual | Low-Medium |
| Ollama | Private, Local | Free |
| OpenRouter | Provider Access | Varies |
| ZAI | Specialized | Medium |

---

## Slide 5: Claude Integration

**Anthropic Claude:**

```yaml
providers:
  claude:
    api_key: ${CLAUDE_API_KEY}
    model: claude-3-5-sonnet-20241022
    base_url: https://api.anthropic.com/v1
```

**Available Models:**
- claude-3-opus-20240229 (Most capable)
- claude-3-5-sonnet-20241022 (Balanced)
- claude-3-haiku-20240307 (Fast)

---

## Slide 6: Claude Capabilities

**Claude Strengths:**

- Complex reasoning and analysis
- Long context windows (200K tokens)
- Strong coding abilities
- Detailed explanations
- Safety and alignment

**Best Use Cases:**
- Document analysis
- Code review
- Research synthesis
- Complex problem solving

---

## Slide 7: DeepSeek Integration

**DeepSeek:**

```yaml
providers:
  deepseek:
    api_key: ${DEEPSEEK_API_KEY}
    model: deepseek-coder
    base_url: https://api.deepseek.com/v1
    temperature: 0.1
```

**Available Models:**
- deepseek-coder (Code-optimized)
- deepseek-chat (General conversation)

---

## Slide 8: DeepSeek Capabilities

**DeepSeek Strengths:**

- Excellent code generation
- Technical documentation
- Cost-effective processing
- Good debugging abilities

**Best Use Cases:**
- Code completion
- Bug fixing
- Technical writing
- API documentation

---

## Slide 9: Gemini Integration

**Google Gemini:**

```yaml
providers:
  gemini:
    api_key: ${GEMINI_API_KEY}
    model: gemini-pro
    base_url: https://generativelanguage.googleapis.com
```

**Available Models:**
- gemini-pro (Text)
- gemini-pro-vision (Multimodal)
- gemini-ultra (Most capable)

---

## Slide 10: Gemini Capabilities

**Gemini Strengths:**

- Multimodal understanding
- Scientific reasoning
- Math and logic
- Image analysis

**Best Use Cases:**
- Image + text analysis
- Scientific research
- Mathematical problems
- Data interpretation

---

## Slide 11: Qwen Integration

**Alibaba Qwen:**

```yaml
providers:
  qwen:
    api_key: ${QWEN_API_KEY}
    model: qwen-turbo
    base_url: https://dashscope.aliyuncs.com/api/v1
```

**Available Models:**
- qwen-turbo (Fast)
- qwen-plus (Balanced)
- qwen-max (Most capable)

---

## Slide 12: Qwen Capabilities

**Qwen Strengths:**

- Excellent multilingual support
- Chinese language expertise
- Good value for cost
- Strong reasoning

**Best Use Cases:**
- Multilingual applications
- Translation tasks
- Asian market content
- Cost-sensitive applications

---

## Slide 13: Ollama Integration

**Local LLM with Ollama:**

```yaml
providers:
  ollama:
    enabled: true
    base_url: http://localhost:11434
    model: llama2
    timeout: 60s
```

**No API key required - runs locally!**

---

## Slide 14: Ollama Capabilities

**Ollama Advantages:**

- Complete data privacy
- No API costs
- Low latency (local)
- Customizable models

**Best Use Cases:**
- Sensitive data processing
- Offline environments
- Development/testing
- Cost-conscious deployments

---

## Slide 15: OpenRouter Integration

**Meta-Provider Access:**

```yaml
providers:
  openrouter:
    api_key: ${OPENROUTER_API_KEY}
    model: anthropic/claude-3-opus
    base_url: https://openrouter.ai/api/v1
```

**Access 100+ models through one API!**

---

## Slide 16: OpenRouter Capabilities

**OpenRouter Advantages:**

- Access to many providers
- Single API interface
- Automatic failover
- Competitive pricing

**Available Models:**
- All major providers (Claude, GPT, Gemini)
- Open-source models
- Specialized models

---

## Slide 17: ZAI Integration

**ZAI Provider:**

```yaml
providers:
  zai:
    api_key: ${ZAI_API_KEY}
    model: zai-default
    base_url: https://api.zai.ai/v1
```

**Specialized for specific use cases**

---

## Slide 18: Provider Selection Strategy

**Choosing the Right Provider:**

| Task Type | Recommended | Fallback |
|-----------|-------------|----------|
| Analysis | Claude | Gemini |
| Code | DeepSeek | Claude |
| Multilingual | Qwen | Gemini |
| Private Data | Ollama | N/A |
| Cost-Sensitive | DeepSeek | Qwen |

---

## Slide 19: Fallback Chain Design

**Implementing Reliable Fallbacks:**

```yaml
providers:
  fallback_chain:
    - claude      # Primary - best quality
    - gemini      # Secondary - good alternative
    - deepseek    # Tertiary - cost-effective
    - ollama      # Last resort - always available

  fallback_policy:
    trigger_on:
      - timeout
      - rate_limit
      - error_5xx
    max_attempts: 3
```

---

## Slide 20: Provider Health Monitoring

**Health Check Implementation:**

```go
func (p *ClaudeProvider) HealthCheck(ctx context.Context) error {
    resp, err := p.client.Get("/health")
    if err != nil {
        return fmt.Errorf("health check failed: %w", err)
    }
    if resp.StatusCode != 200 {
        return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
    }
    return nil
}
```

---

## Slide 21: Health Check API

**Monitoring Provider Health:**

```bash
# Check all providers
curl http://localhost:7061/v1/providers/health

# Response
{
  "providers": {
    "claude": {
      "status": "healthy",
      "latency_ms": 145,
      "last_check": "2024-01-01T12:00:00Z"
    },
    "gemini": {
      "status": "healthy",
      "latency_ms": 198
    }
  }
}
```

---

## Slide 22: Adding a New Provider

**Steps to Add a Provider:**

1. Create package: `internal/llm/providers/<name>/`
2. Implement `LLMProvider` interface
3. Register in `provider_registry.go`
4. Add environment variables to `.env.example`
5. Write tests in `<name>_test.go`

---

## Slide 23: Provider Implementation Example

**Basic Provider Structure:**

```go
package myprovider

type MyProvider struct {
    apiKey  string
    client  *http.Client
    config  *Config
}

func (p *MyProvider) Complete(
    ctx context.Context,
    req *Request,
) (*Response, error) {
    // Implementation
}
```

---

## Slide 24: Rate Limiting Per Provider

**Managing API Limits:**

```yaml
providers:
  claude:
    rate_limit:
      requests_per_second: 10
      tokens_per_minute: 100000
      concurrent_requests: 5

  deepseek:
    rate_limit:
      requests_per_second: 15
      tokens_per_minute: 150000
```

---

## Slide 25: Cost Optimization

**Reducing API Costs:**

| Strategy | Implementation |
|----------|----------------|
| Caching | Cache similar queries |
| Routing | Use cheaper providers when possible |
| Tokens | Optimize prompt length |
| Fallback | Use local Ollama as backup |

---

## Slide 26: Performance Comparison

**Provider Performance Metrics:**

| Provider | Avg Latency | Cost/1K tokens |
|----------|-------------|----------------|
| Claude Sonnet | 800ms | $0.003 |
| Gemini Pro | 600ms | $0.002 |
| DeepSeek | 500ms | $0.001 |
| Ollama | 300ms | Free |

*Actual values vary by region and load*

---

## Slide 27: Hands-On Lab

**Lab Exercise 4.1: Multi-Provider Setup**

Tasks:
1. Configure at least 3 providers
2. Implement a fallback chain
3. Test provider health checks
4. Compare response quality
5. Measure latency differences

Time: 30 minutes

---

## Slide 28: Module Summary

**Key Takeaways:**

- 7 providers with distinct capabilities
- LLMProvider interface for consistency
- Fallback chains for reliability
- Health monitoring essential
- Choose providers based on use case
- Cost optimization through routing

**Next: Module 5 - Ensemble Strategies**

---

## Speaker Notes

### Slide 4 Notes
Go through each provider and discuss when to use each. Emphasize that provider selection depends on the specific use case.

### Slide 19 Notes
Demonstrate fallback in action by simulating a provider failure. Show how HelixAgent automatically routes to the next provider.

### Slide 25 Notes
Discuss cost implications in real-world scenarios. Many organizations can save 30-50% by intelligent routing.
