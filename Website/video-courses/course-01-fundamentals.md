# HelixAgent Fundamentals - Complete Video Course Script

**Total Duration: 60 minutes**
**Level: Beginner**
**Prerequisites: Basic understanding of APIs and REST concepts**

---

## Module 1: Introduction to HelixAgent (10 minutes)

### Opening Slide
**Title:** Welcome to HelixAgent
**Duration:** 30 seconds

---

### Section 1.1: What is HelixAgent? (3 minutes)

#### Narration Script:

Welcome to the HelixAgent Fundamentals course. I'm excited to introduce you to HelixAgent - an AI-powered ensemble LLM service that revolutionizes how you interact with multiple language models.

HelixAgent is written in Go 1.23+ and provides a unified interface for working with multiple AI providers. Instead of managing separate integrations for each AI service, HelixAgent acts as your intelligent orchestrator, combining responses from multiple models using sophisticated aggregation strategies.

#### Key Points to Cover:
- Unified interface for 43+ LLM providers
- Ensemble orchestration capabilities
- OpenAI-compatible API design
- Written in Go for high performance
- Production-ready with circuit breakers and health monitoring

#### Slide Content:
```
HELIXAGENT AT A GLANCE
- Multi-LLM Orchestration Platform
- 43 Supported Providers (including):
  - Claude (Anthropic)
  - DeepSeek
  - Gemini (Google)
  - Qwen (Alibaba)
  - ZAI
  - Ollama (Local)
  - OpenRouter (Gateway to 200+ models)
- OpenAI-Compatible REST & gRPC APIs
- Built-in High Availability
```

---

### Section 1.2: Key Features and Benefits (3 minutes)

#### Narration Script:

What makes HelixAgent different from simply calling one AI model? Let me walk you through the key features that make HelixAgent a powerful platform.

First, ensemble orchestration. HelixAgent can query multiple AI models simultaneously and intelligently combine their responses. This means you get better quality outputs by leveraging the strengths of different models.

Second, the AI Debate system. This unique feature allows multiple AI participants to engage in structured debates, reaching consensus through multi-round discussions. We'll cover this in depth in Course 2.

Third, fault tolerance. With built-in circuit breakers, automatic retries, and health monitoring, HelixAgent ensures your applications remain resilient even when individual providers experience issues.

#### Key Points to Cover:
- Ensemble strategies: confidence-weighted voting, majority vote
- AI Debate system for complex reasoning
- Circuit breaker pattern implementation
- Provider health monitoring
- Automatic failover between providers
- Response quality scoring

#### Slide Content:
```
KEY FEATURES

[Ensemble Orchestration]
- Parallel execution across providers
- Confidence-weighted response selection
- Quality scoring algorithms

[AI Debate System]
- Multi-agent collaboration
- Consensus building
- Role-based participation

[Fault Tolerance]
- Circuit breakers per provider
- Automatic retry with exponential backoff
- Health check monitoring
- Graceful degradation
```

#### Demo Scenario:
Show a diagram of request flow:
```
Client Request --> HelixAgent --> [Provider A] --> Response A
                              --> [Provider B] --> Response B
                              --> [Provider C] --> Response C
                                    |
                                    v
                              Ensemble Engine
                                    |
                                    v
                              Best Response
```

---

### Section 1.3: Use Cases and Applications (3.5 minutes)

#### Narration Script:

HelixAgent excels in scenarios where reliability, quality, and intelligent orchestration matter. Let me share some common use cases.

For enterprise AI applications, you need consistent, high-quality responses. HelixAgent's ensemble approach ensures that if one model provides a subpar response, others can compensate.

For research and analysis, the AI Debate feature allows you to set up structured discussions between different AI models, each with different perspectives or roles. This leads to more thorough analysis of complex topics.

For cost optimization, you can route simpler queries to more economical local models via Ollama while reserving premium providers for complex tasks.

#### Key Points to Cover:
- Enterprise chat applications requiring high availability
- Research platforms needing diverse AI perspectives
- Content generation with quality validation
- Code analysis and review systems
- Complex decision support systems
- Cost-optimized AI routing

#### Slide Content:
```
USE CASES

[Enterprise Applications]
- Customer support chatbots with failover
- Internal knowledge assistants
- Document analysis systems

[Research & Analysis]
- Multi-model fact verification
- Comparative AI responses
- Complex reasoning tasks

[Development]
- Code review and analysis
- Technical documentation
- Automated testing support

[Cost Optimization]
- Smart routing to appropriate models
- Local model fallback (Ollama)
- Pay-per-use optimization
```

---

## Module 2: Installation and Setup (15 minutes)

### Section 2.1: System Requirements (2 minutes)

#### Narration Script:

Before we install HelixAgent, let's review the system requirements. The good news is that HelixAgent is designed to be lightweight and flexible.

For development, you'll need Docker Desktop installed. If you prefer running natively, you'll need Go 1.23 or later. For the database and caching layers, we'll use PostgreSQL 15 and Redis 7, which our Docker setup handles automatically.

#### Key Points to Cover:
- Docker Desktop (recommended for quick start)
- Go 1.23+ (for native development)
- PostgreSQL 15
- Redis 7
- Minimum 2GB RAM
- Podman alternative supported

#### Slide Content:
```
SYSTEM REQUIREMENTS

[Container-Based Deployment]
- Docker Desktop 24.0+ or Podman 4.0+
- 4GB RAM recommended
- 10GB disk space

[Native Development]
- Go 1.23+
- PostgreSQL 15
- Redis 7
- Git

[Development Tools]
- curl or Postman for API testing
- Text editor or IDE
- Terminal access
```

---

### Section 2.2: Docker Installation (5 minutes)

#### Narration Script:

The fastest way to get HelixAgent running is with Docker. Let's walk through the complete setup process.

First, clone the repository. Then, we'll copy the environment example file and configure our API keys. Finally, we'll start the services using docker-compose.

#### Step-by-Step Script:

**Step 1: Clone the Repository**

```bash
# Clone the HelixAgent repository
git clone https://dev.helix.agent.git
cd helixagent
```

Narration: "First, we clone the HelixAgent repository. This contains everything you need including Docker configurations, configuration files, and the source code."

**Step 2: Configure Environment**

```bash
# Copy the environment template
cp .env.example .env

# Edit the environment file
nano .env
```

Narration: "Next, copy the environment template. This file contains all the configuration options. At minimum, you'll need to set your API keys for the providers you want to use."

#### Code Example - .env Configuration:
```bash
# Server Configuration
PORT=8080
GIN_MODE=release

# Database Configuration
DB_HOST=postgres
DB_PORT=5432
DB_USER=helixagent
DB_PASSWORD=your_secure_password
DB_NAME=helixagent

# Redis Configuration
REDIS_HOST=redis
REDIS_PORT=6379

# LLM Provider API Keys
ANTHROPIC_API_KEY=sk-ant-your-claude-key
DEEPSEEK_API_KEY=your-deepseek-key
GEMINI_API_KEY=your-gemini-key
QWEN_API_KEY=your-qwen-key
OPENROUTER_API_KEY=your-openrouter-key

# JWT Secret (generate a secure random string)
JWT_SECRET=your-jwt-secret-change-this
```

**Step 3: Start Core Services**

```bash
# Start the core services (PostgreSQL, Redis, HelixAgent)
docker-compose up -d

# Check the status
docker-compose ps
```

Narration: "Now we start the core services. The dash-d flag runs them in detached mode. Let's check that everything is running correctly."

**Step 4: Verify Installation**

```bash
# Check HelixAgent health
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","version":"1.0.0","services":{"database":"healthy","redis":"healthy"}}
```

Narration: "We can verify the installation by calling the health endpoint. A successful response shows all services are operational."

#### Slide Content:
```
DOCKER INSTALLATION STEPS

1. Clone Repository
   git clone https://dev.helix.agent.git

2. Configure Environment
   cp .env.example .env
   # Edit .env with your API keys

3. Start Services
   docker-compose up -d

4. Verify Health
   curl http://localhost:8080/health

[PROFILES AVAILABLE]
- default: Core services
- ai: Add Ollama support
- monitoring: Add Prometheus/Grafana
- optimization: Add LLM optimization tools
- full: Everything
```

---

### Section 2.3: Configuration Basics (5 minutes)

#### Narration Script:

HelixAgent uses YAML configuration files for detailed settings. Let's explore the main configuration file structure and the most important options.

The configuration is divided into logical sections: server settings, database connections, LLM providers, security, and monitoring. Let me show you the key sections.

#### Code Example - development.yaml:
```yaml
# HelixAgent Configuration

server:
  port: 8080
  host: "0.0.0.0"
  environment: "development"
  log_level: "debug"

database:
  host: "localhost"
  port: 5432
  user: "helixagent"
  password: "${DB_PASSWORD}"
  name: "helixagent"
  max_open_connections: 10
  max_idle_connections: 2

redis:
  host: "localhost"
  port: 6379
  db: 0
  pool_size: 5

llm_providers:
  anthropic:
    enabled: true
    api_key: "${ANTHROPIC_API_KEY}"
    model: "claude-3-sonnet-20240229"
    timeout: "30s"
    weight: 1.0

  gemini:
    enabled: true
    api_key: "${GEMINI_API_KEY}"
    model: "gemini-pro"
    timeout: "30s"
    weight: 0.8

security:
  jwt:
    secret: "${JWT_SECRET}"
    expiration: "24h"
  rate_limit:
    requests_per_minute: 60
    burst: 10

monitoring:
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"
  health_check:
    enabled: true
    interval: "30s"
```

#### Key Points to Cover:
- Environment variable substitution with ${VAR_NAME}
- Provider weights for ensemble selection
- Timeout configurations
- Security settings importance
- Monitoring endpoints

#### Slide Content:
```
CONFIGURATION STRUCTURE

[Server Settings]
port, host, environment, log_level

[Database & Redis]
connection strings, pool sizes

[LLM Providers]
- Enable/disable per provider
- API keys (use env vars!)
- Model selection
- Timeouts and weights

[Security]
- JWT configuration
- Rate limiting
- CORS settings

[Monitoring]
- Prometheus metrics
- Health check intervals
```

---

### Section 2.4: First API Call (3 minutes)

#### Narration Script:

Let's make our first API call to HelixAgent. We'll use the completions endpoint, which follows the OpenAI-compatible format. This means if you have existing code that works with OpenAI's API, it should work with HelixAgent with minimal changes.

#### Code Example:
```bash
# Make your first completion request
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "messages": [
      {
        "role": "user",
        "content": "Explain what makes HelixAgent unique in one paragraph."
      }
    ],
    "max_tokens": 200,
    "temperature": 0.7
  }'
```

#### Expected Response:
```json
{
  "id": "resp-abc123",
  "object": "chat.completion",
  "created": 1699900000,
  "model": "claude-3-sonnet-20240229",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "HelixAgent is unique because it serves as an intelligent orchestration layer..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 85,
    "total_tokens": 100
  },
  "provider_id": "claude",
  "confidence": 0.92
}
```

Narration: "Notice that the response includes additional metadata like provider_id and confidence score. These HelixAgent-specific fields help you understand which provider handled the request and how confident the system is in the response quality."

---

## Module 3: Working with LLM Providers (20 minutes)

### Section 3.1: Provider Configuration (5 minutes)

#### Narration Script:

HelixAgent supports forty-three LLM providers out of the box. Each provider has its own strengths, and HelixAgent makes it easy to configure and use them together. Let's look at how to set up each provider.

#### Provider Setup Overview:

**Claude (Anthropic)**
```yaml
claude:
  enabled: true
  api_key: "${ANTHROPIC_API_KEY}"
  model: "claude-3-sonnet-20240229"
  base_url: "https://api.anthropic.com/v1/messages"
  timeout: "30s"
  max_retries: 3
  weight: 1.0
```

**DeepSeek**
```yaml
deepseek:
  enabled: true
  api_key: "${DEEPSEEK_API_KEY}"
  model: "deepseek-coder"
  base_url: "https://api.deepseek.com/v1"
  timeout: "30s"
  weight: 0.9
```

**Gemini (Google)**
```yaml
gemini:
  enabled: true
  api_key: "${GEMINI_API_KEY}"
  model: "gemini-pro"
  timeout: "30s"
  weight: 0.8
```

**Qwen (Alibaba)**
```yaml
qwen:
  enabled: true
  api_key: "${QWEN_API_KEY}"
  model: "qwen-turbo"
  base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
  timeout: "25s"
  weight: 0.8
```

**OpenRouter (Multi-model Gateway)**
```yaml
openrouter:
  enabled: true
  api_key: "${OPENROUTER_API_KEY}"
  model: "x-ai/grok-4"  # Access Grok, GPT-4, and 200+ models
  base_url: "https://openrouter.ai/api/v1"
  timeout: "35s"
  weight: 1.3
```

**Ollama (Local Models)**
```yaml
ollama:
  enabled: true
  base_url: "http://localhost:11434"
  model: "llama2"
  timeout: "60s"
  weight: 0.5
```

#### Key Points to Cover:
- Each provider requires an API key (except Ollama)
- Weights influence ensemble selection
- Timeouts should match provider capabilities
- base_url can be customized for proxies

#### Slide Content:
```
PROVIDER CONFIGURATION

[Essential Settings per Provider]
- enabled: true/false
- api_key: use environment variables
- model: specific model identifier
- timeout: request timeout
- weight: ensemble priority (higher = preferred)
- max_retries: failure retry count

[Provider Weights Guide]
1.3 - Premium provider (use first)
1.0 - Standard priority
0.8 - Secondary choice
0.5 - Fallback only
```

---

### Section 3.2: Demo - Configuring Multiple Providers (8 minutes)

#### Narration Script:

Let me show you a live demo of configuring and testing multiple providers. We'll start with a simple setup and gradually add more providers.

#### Demo Steps:

**Step 1: Check Currently Registered Providers**
```bash
curl http://localhost:8080/v1/providers
```

Response:
```json
{
  "providers": [
    {
      "name": "claude",
      "enabled": true,
      "status": "healthy",
      "model": "claude-3-sonnet-20240229"
    },
    {
      "name": "gemini",
      "enabled": true,
      "status": "healthy",
      "model": "gemini-pro"
    }
  ]
}
```

**Step 2: Add a New Provider Dynamically**
```bash
curl -X POST http://localhost:8080/v1/providers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "deepseek",
    "type": "deepseek",
    "api_key": "your-deepseek-key",
    "model": "deepseek-coder",
    "enabled": true,
    "weight": 0.9
  }'
```

**Step 3: Test Provider Health**
```bash
# Check all provider health status
curl http://localhost:8080/v1/providers/health
```

Response:
```json
{
  "claude": {
    "status": "healthy",
    "response_time_ms": 245,
    "last_check": "2024-01-15T10:30:00Z"
  },
  "gemini": {
    "status": "healthy",
    "response_time_ms": 312,
    "last_check": "2024-01-15T10:30:00Z"
  },
  "deepseek": {
    "status": "healthy",
    "response_time_ms": 189,
    "last_check": "2024-01-15T10:30:00Z"
  }
}
```

**Step 4: Query Specific Provider**
```bash
# Request to specific provider
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "deepseek",
    "messages": [
      {"role": "user", "content": "Write a hello world function in Python"}
    ],
    "max_tokens": 100
  }'
```

**Step 5: Use Ensemble Mode**
```bash
# Ensemble request - queries all healthy providers
curl -X POST http://localhost:8080/v1/ensemble/completions \
  -H "Content-Type: application/json" \
  -d '{
    "strategy": "confidence_weighted",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "min_providers": 2
  }'
```

---

### Section 3.3: API Key Management (3 minutes)

#### Narration Script:

API key security is crucial. HelixAgent provides multiple ways to manage API keys securely. Never hardcode API keys in your configuration files - always use environment variables or a secrets manager.

#### Key Points to Cover:
- Environment variable injection
- Docker secrets support
- Key rotation without downtime
- Per-request key override capability

#### Code Example - Secure Key Management:
```bash
# Method 1: Environment Variables (recommended)
export ANTHROPIC_API_KEY="sk-ant-..."

# Method 2: Docker Secrets
echo "sk-ant-..." | docker secret create anthropic_api_key -

# Method 3: .env file (development only)
# Never commit .env to version control!
echo "ANTHROPIC_API_KEY=sk-ant-..." >> .env
```

#### Configuration Example:
```yaml
# In production.yaml - keys reference environment
llm_providers:
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    # Key can also be rotated dynamically via API
```

---

### Section 3.4: Health Monitoring (4 minutes)

#### Narration Script:

HelixAgent continuously monitors provider health. This enables automatic failover and ensures your application remains responsive even when providers experience issues.

#### Code Example - Health Check Implementation:

The health check system works in three layers:

**Layer 1: Basic Connectivity**
```bash
# Check if HelixAgent can reach the provider
curl http://localhost:8080/v1/providers/claude/health
```

**Layer 2: Response Quality**
```json
{
  "provider": "claude",
  "status": "healthy",
  "metrics": {
    "avg_response_time_ms": 523,
    "success_rate": 0.987,
    "error_rate": 0.013,
    "requests_last_hour": 1250,
    "circuit_breaker_state": "closed"
  }
}
```

**Layer 3: Circuit Breaker Status**

The circuit breaker protects your application from cascading failures:

```yaml
circuit_breaker:
  enabled: true
  failure_threshold: 5      # Open after 5 failures
  success_threshold: 2      # Close after 2 successes
  recovery_timeout: "60s"   # Wait before half-open
```

States explained:
- **Closed**: Normal operation, requests flow through
- **Open**: Provider failing, requests fail-fast
- **Half-Open**: Testing recovery, limited requests

#### Slide Content:
```
HEALTH MONITORING

[Automatic Health Checks]
- Periodic connectivity tests
- Response time tracking
- Success/error rate calculation

[Circuit Breaker Pattern]
Closed --> (failures) --> Open
                           |
                     (timeout)
                           |
                           v
                      Half-Open
                           |
              (success) <--+--> (failure)
                  |                 |
                  v                 v
               Closed            Open
```

---

## Module 4: Basic API Usage (15 minutes)

### Section 4.1: Completion Endpoints (5 minutes)

#### Narration Script:

HelixAgent provides OpenAI-compatible endpoints for chat completions. This means you can use existing OpenAI client libraries with HelixAgent. Let's explore the main completion endpoints.

#### Code Examples:

**Basic Chat Completion**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain quantum computing in simple terms."}
    ],
    "temperature": 0.7,
    "max_tokens": 500
  }'
```

**Streaming Response**
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "messages": [
      {"role": "user", "content": "Write a short story about a robot."}
    ],
    "stream": true,
    "max_tokens": 300
  }'
```

Streaming output:
```
data: {"id":"chatcmpl-123","choices":[{"delta":{"content":"Once"}}]}
data: {"id":"chatcmpl-123","choices":[{"delta":{"content":" upon"}}]}
data: {"id":"chatcmpl-123","choices":[{"delta":{"content":" a"}}]}
data: {"id":"chatcmpl-123","choices":[{"delta":{"content":" time"}}]}
...
data: [DONE]
```

**Using with OpenAI Python SDK**
```python
from openai import OpenAI

# Point to HelixAgent instead of OpenAI
client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="your-helixagent-key"
)

response = client.chat.completions.create(
    model="claude-3-sonnet-20240229",
    messages=[
        {"role": "user", "content": "Hello, HelixAgent!"}
    ]
)

print(response.choices[0].message.content)
```

---

### Section 4.2: Chat Interfaces (4 minutes)

#### Narration Script:

HelixAgent extends the standard chat interface with additional features like conversation memory, context management, and multi-turn conversations. Let's see how to use these features effectively.

#### Code Examples:

**Multi-turn Conversation**
```bash
# Start a conversation with session tracking
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "session_id": "user-123-session-1",
    "messages": [
      {"role": "user", "content": "My name is Alice."}
    ]
  }'

# Continue the conversation
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "session_id": "user-123-session-1",
    "messages": [
      {"role": "user", "content": "What is my name?"}
    ]
  }'
```

**System Prompt Configuration**
```json
{
  "model": "claude-3-sonnet-20240229",
  "messages": [
    {
      "role": "system",
      "content": "You are a technical expert. Always provide code examples when explaining concepts. Be concise but thorough."
    },
    {
      "role": "user",
      "content": "How do I read a file in Go?"
    }
  ],
  "temperature": 0.3
}
```

---

### Section 4.3: Response Handling (3 minutes)

#### Narration Script:

Understanding response structure is essential for building robust applications. HelixAgent responses include both standard OpenAI fields and additional metadata specific to our platform.

#### Response Structure:
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1699900000,
  "model": "claude-3-sonnet-20240229",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The response content here..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 50,
    "completion_tokens": 150,
    "total_tokens": 200
  },

  // HelixAgent-specific fields
  "provider_id": "claude",
  "provider_name": "Claude",
  "confidence": 0.94,
  "response_time_ms": 1234,
  "metadata": {
    "input_tokens": 50,
    "model_version": "claude-3-sonnet-20240229"
  }
}
```

#### Key Fields Explained:
- `finish_reason`: "stop" (completed), "length" (max tokens), "content_filter"
- `confidence`: 0.0-1.0 quality score
- `provider_id`: which provider handled the request
- `response_time_ms`: actual response latency

---

### Section 4.4: Error Management (3 minutes)

#### Narration Script:

Proper error handling is crucial for production applications. HelixAgent provides detailed error responses that help you diagnose and handle issues gracefully.

#### Error Response Structure:
```json
{
  "error": {
    "message": "Rate limit exceeded for provider claude",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded",
    "param": null,
    "provider": "claude",
    "retry_after": 30
  }
}
```

#### Common Error Codes and Handling:

```python
import time
from openai import OpenAI, RateLimitError, APIError

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="your-key"
)

def make_request_with_retry(messages, max_retries=3):
    for attempt in range(max_retries):
        try:
            return client.chat.completions.create(
                model="claude-3-sonnet-20240229",
                messages=messages
            )
        except RateLimitError as e:
            retry_after = e.response.headers.get('Retry-After', 30)
            print(f"Rate limited. Waiting {retry_after}s...")
            time.sleep(int(retry_after))
        except APIError as e:
            if e.status_code >= 500:
                # Server error - retry with backoff
                wait_time = 2 ** attempt
                print(f"Server error. Retrying in {wait_time}s...")
                time.sleep(wait_time)
            else:
                raise  # Client error - don't retry

    raise Exception("Max retries exceeded")
```

#### Slide Content:
```
ERROR HANDLING BEST PRACTICES

[Error Types]
- rate_limit_error: Wait and retry
- authentication_error: Check API key
- invalid_request_error: Fix request
- server_error: Retry with backoff
- provider_unavailable: Automatic failover

[Retry Strategy]
1. Check error type
2. Extract retry-after header
3. Implement exponential backoff
4. Set maximum retry limit
5. Log for monitoring
```

---

## Course Wrap-up (1 minute)

#### Narration Script:

Congratulations! You've completed the HelixAgent Fundamentals course. You now understand what HelixAgent is, how to install and configure it, how to work with multiple LLM providers, and how to make API calls and handle responses.

In the next course, we'll dive deep into the AI Debate system - one of HelixAgent's most powerful features. We'll learn how to configure debate participants, run multi-round discussions, and leverage consensus building for complex reasoning tasks.

Thank you for learning with us, and we'll see you in Course 2: AI Debate System Mastery!

#### Slide Content:
```
COURSE COMPLETE!

What You Learned:
- HelixAgent architecture and benefits
- Installation with Docker
- Configuration and environment setup
- Working with multiple LLM providers
- Making API calls and handling responses
- Error management best practices

Next Steps:
- Course 2: AI Debate System Mastery
- Practice with the hands-on exercises
- Join our community forums

Resources:
- Documentation: docs.helixagent.ai
- GitHub: dev.helix.agent
- Support: support@helixagent.ai
```

---

## Supplementary Materials

### Hands-on Exercise 1: Provider Setup
Configure and test at least two LLM providers using Docker Compose.

### Hands-on Exercise 2: API Integration
Create a simple Python script that makes completion requests and handles errors.

### Hands-on Exercise 3: Ensemble Testing
Configure ensemble mode and compare responses from multiple providers.

---

## Related Courses

- **Course 70: DocProcessor Module** — Learn how HelixAgent automates documentation analysis and feature extraction, a key capability introduced alongside the fundamentals covered here.
- **Course 71: HelixQA Module** — Explore cross-platform autonomous QA with crash detection, built on the provider and ensemble concepts from this course.
- **Course 72: LLMOrchestrator Module** — Deep dive into headless CLI agent orchestration with circuit breakers, extending the provider management techniques introduced in Module 3.
- **Course 73: VisionEngine Module** — Discover computer vision UI analysis with NavigationGraph, adding visual capabilities to the HelixAgent platform you learned here.

### Quick Reference Card
```
ESSENTIAL COMMANDS

# Start HelixAgent
docker-compose up -d

# Check health
curl http://localhost:8080/health

# List providers
curl http://localhost:8080/v1/providers

# Make completion request
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-3-sonnet-20240229","messages":[{"role":"user","content":"Hello"}]}'

# View logs
docker-compose logs -f helixagent
```
