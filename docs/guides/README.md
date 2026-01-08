# HelixAgent User Guide

## Table of Contents

1. [Getting Started](#getting-started)
2. [Basic Configuration](#basic-configuration)
3. [Creating Debates](#creating-debates)
4. [Advanced Features](#advanced-features)
5. [Monitoring and Analytics](#monitoring-and-analytics)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)
8. [FAQ](#faq)

---

## Getting Started

### What is HelixAgent?

HelixAgent is a comprehensive AI debate platform that enables intelligent discussions between multiple AI participants using different LLM providers. It features:

- **Multi-Provider Support**: Claude, DeepSeek, Gemini, Qwen, Zai, and Ollama
- **Cognee AI Enhancement**: Advanced semantic analysis and insights
- **Real-time Monitoring**: Live debate tracking and analytics
- **Consensus Building**: Intelligent agreement detection
- **Performance Optimization**: Efficient resource utilization
- **Security & Compliance**: Enterprise-grade security features

### System Requirements

- **Go**: 1.23 or higher
- **Memory**: Minimum 2GB RAM
- **Storage**: 1GB available space
- **Network**: Internet access for LLM providers
- **Database**: PostgreSQL 12+ (optional for history)

### Installation

#### Using Docker (Recommended)

```bash
# Pull the latest image
docker pull helixagent/helixagent:latest

# Run with basic configuration
docker run -d \
  --name helixagent \
  -p 8080:7061 \
  -e HELIXAGENT_API_KEY=your-api-key \
  helixagent/helixagent:latest
```

#### Manual Installation

```bash
# Clone the repository
git clone https://dev.helix.agent.git
cd helixagent

# Build the application
make build

# Run tests
make test

# Start the server
./bin/helixagent-server
```

---

## Basic Configuration

### Configuration File Structure

Create a `config.yaml` file:

```yaml
# Basic server configuration
server:
  host: "0.0.0.0"
  port: 8080
  debug: false

# LLM Provider configurations
providers:
  claude:
    api_key: "your-claude-api-key"
    model: "claude-3-opus-20240229"
    enabled: true
    weight: 1.0
  
  deepseek:
    api_key: "your-deepseek-api-key"
    model: "deepseek-chat"
    enabled: true
    weight: 1.0
  
  gemini:
    api_key: "your-gemini-api-key"
    model: "gemini-pro"
    enabled: true
    weight: 1.0

# Debate settings
debate:
  max_rounds: 5
  timeout: 600  # seconds
  consensus_threshold: 0.75
  enable_cognee: true

# Monitoring settings
monitoring:
  enabled: true
  metrics_port: 9090
  
# Security settings
security:
  jwt_secret: "your-jwt-secret"
  api_key_required: true
```

### Environment Variables

Set these environment variables:

```bash
export HELIXAGENT_API_KEY=your-api-key
export JWT_SECRET=your-jwt-secret
export COGNEE_API_KEY=your-cognee-api-key
export POSTGRES_URL=postgres://user:pass@localhost/helixagent
export REDIS_URL=redis://localhost:6379
```

### Provider API Keys

Configure provider API keys:

| Provider | Environment Variable | Required |
|----------|---------------------|----------|
| Claude | `CLAUDE_API_KEY` | Yes |
| DeepSeek | `DEEPSEEK_API_KEY` | Yes |
| Gemini | `GEMINI_API_KEY` | Yes |
| Qwen | `QWEN_API_KEY` | Yes |
| Zai | `ZAI_API_KEY` | Yes |
| Ollama | `OLLAMA_BASE_URL` | Yes |

---

## Creating Debates

### Basic Debate

```python
import requests

# Create a simple debate
response = requests.post(
    "http://localhost:7061/v1/debates",
    headers={"Authorization": "Bearer your-api-key"},
    json={
        "debateId": "basic-debate-001",
        "topic": "Is remote work better than office work?",
        "participants": [
            {
                "participantId": "remote-advocate",
                "name": "Remote Work Advocate",
                "role": "proponent",
                "llmProvider": "claude",
                "llmModel": "claude-3-sonnet-20240229",
                "maxRounds": 3,
                "timeout": 120,
                "weight": 1.0
            },
            {
                "participantId": "office-advocate",
                "name": "Office Work Advocate",
                "role": "opponent",
                "llmProvider": "deepseek",
                "llmModel": "deepseek-chat",
                "maxRounds": 3,
                "timeout": 120,
                "weight": 1.0
            }
        ],
        "maxRounds": 3,
        "timeout": 600,
        "strategy": "structured"
    }
)

debate = response.json()
print(f"Debate created: {debate['debateId']}")
```

### Advanced Debate with Cognee

```python
# Create an advanced debate with Cognee enhancement
response = requests.post(
    "http://localhost:7061/v1/debates",
    headers={"Authorization": "Bearer your-api-key"},
    json={
        "debateId": "advanced-debate-001",
        "topic": "Should AI be regulated by governments?",
        "participants": [
            {
                "participantId": "ai-regulation-pro",
                "name": "AI Regulation Proponent",
                "role": "proponent",
                "llmProvider": "claude",
                "llmModel": "claude-3-opus-20240229",
                "maxRounds": 5,
                "timeout": 180,
                "weight": 1.2
            },
            {
                "participantId": "ai-regulation-con",
                "name": "AI Regulation Opponent",
                "role": "opponent",
                "llmProvider": "gemini",
                "llmModel": "gemini-pro",
                "maxRounds": 5,
                "timeout": 180,
                "weight": 1.0
            },
            {
                "participantId": "neutral-observer",
                "name": "Neutral Observer",
                "role": "neutral",
                "llmProvider": "qwen",
                "llmModel": "qwen-turbo",
                "maxRounds": 5,
                "timeout": 150,
                "weight": 0.8
            }
        ],
        "maxRounds": 5,
        "timeout": 900,
        "strategy": "socratic",
        "enableCognee": true,
        "consensusThreshold": 0.8,
        "metadata": {
            "category": "policy",
            "difficulty": "expert",
            "tags": ["ai", "regulation", "government"]
        }
    }
)

debate = response.json()
print(f"Advanced debate created: {debate['debateId']}")
```

### Multi-Provider Debate

```python
# Create a debate using multiple providers
response = requests.post(
    "http://localhost:7061/v1/debates",
    headers={"Authorization": "Bearer your-api-key"},
    json={
        "debateId": "multi-provider-001",
        "topic": "What is the future of renewable energy?",
        "participants": [
            {
                "participantId": "claude-analyst",
                "name": "Claude Policy Analyst",
                "role": "proponent",
                "llmProvider": "claude",
                "llmModel": "claude-3-opus-20240229",
                "maxRounds": 4,
                "timeout": 150,
                "weight": 1.0
            },
            {
                "participantId": "deepseek-engineer",
                "name": "DeepSeek Technical Expert",
                "role": "opponent",
                "llmProvider": "deepseek",
                "llmModel": "deepseek-coder",
                "maxRounds": 4,
                "timeout": 150,
                "weight": 1.0
            },
            {
                "participantId": "gemini-researcher",
                "name": "Gemini Research Analyst",
                "role": "neutral",
                "llmProvider": "gemini",
                "llmModel": "gemini-pro",
                "maxRounds": 4,
                "timeout": 150,
                "weight": 1.0
            },
            {
                "participantId": "qwen-strategist",
                "name": "Qwen Strategic Planner",
                "role": "moderator",
                "llmProvider": "qwen",
                "llmModel": "qwen-turbo",
                "maxRounds": 4,
                "timeout": 150,
                "weight": 0.9
            }
        ],
        "maxRounds": 4,
        "timeout": 600,
        "strategy": "round_robin",
        "enableCognee": true
    }
)

debate = response.json()
print(f"Multi-provider debate created: {debate['debateId']}")
```

---

## Advanced Features

### Consensus Building

```python
# Monitor consensus building
response = requests.get(
    "http://localhost:7061/v1/debates/climate-debate-001/results",
    headers={"Authorization": "Bearer your-api-key"}
)

results = response.json()
consensus = results['consensus']

if consensus['achieved']:
    print(f"Consensus achieved with {consensus['confidence']:.2%} confidence")
    print(f"Final position: {consensus['finalPosition']}")
    print(f"Key points: {', '.join(consensus['keyPoints'])}")
else:
    print("No consensus reached")
    print(f"Disagreements: {', '.join(consensus['disagreements'])}")
```

### Real-time Monitoring

```python
# Monitor debate progress
import time

def monitor_debate(debate_id):
    while True:
        response = requests.get(
            f"http://localhost:7061/v1/debates/{debate_id}/status",
            headers={"Authorization": "Bearer your-api-key"}
        )
        
        status = response.json()
        print(f"Status: {status['status']}, Round: {status['currentRound']}/{status['totalRounds']}")
        
        for participant in status['participants']:
            print(f"  {participant['name']}: {participant['status']}")
        
        if status['status'] == 'completed':
            break
        
        time.sleep(5)  # Poll every 5 seconds

# Monitor the debate
monitor_debate("climate-debate-001")
```

### Performance Analytics

```python
# Get performance metrics
response = requests.get(
    "http://localhost:7061/v1/metrics?timeRange=24h",
    headers={"Authorization": "Bearer your-api-key"}
)

metrics = response.json()
print(f"Total debates: {metrics['totalDebates']}")
print(f"Average quality score: {metrics['averageQualityScore']:.2f}")
print(f"Success rate: {metrics['successRate']:.2%}")

# Get provider-specific metrics
for provider in metrics['providerMetrics']:
    print(f"{provider['providerId']}: {provider['successRate']:.2%} success rate")
```

### Cognee AI Enhancement

```python
# Enable Cognee for semantic analysis
debate_config = {
    "debateId": "cognee-enhanced-001",
    "topic": "The ethics of AI in healthcare",
    "participants": [
        {
            "participantId": "medical-ethicist",
            "name": "Medical Ethicist",
            "role": "proponent",
            "llmProvider": "claude",
            "llmModel": "claude-3-opus-20240229"
        },
        {
            "participantId": "privacy-advocate",
            "name": "Privacy Advocate",
            "role": "opponent",
            "llmProvider": "deepseek",
            "llmModel": "deepseek-chat"
        }
    ],
    "maxRounds": 5,
    "timeout": 900,
    "strategy": "structured",
    "enableCognee": True,
    "consensusThreshold": 0.85
}

# The debate will automatically include Cognee insights
# Access insights in the final results
results_response = requests.get(
    f"http://localhost:7061/v1/debates/{debate_config['debateId']}/results",
    headers={"Authorization": "Bearer your-api-key"}
)

results = results_response.json()
if 'cogneeInsights' in results:
    insights = results['cogneeInsights']
    print(f"Cognee Dataset: {insights['datasetName']}")
    print(f"Enhancement Time: {insights['enhancementTime']}ms")
    print(f"Recommendations: {insights['recommendations']}")
```

---

## Monitoring and Analytics

### Debate History

```python
# Get debate history
response = requests.get(
    "http://localhost:7061/v1/history?limit=10&startTime=2024-01-01T00:00:00Z",
    headers={"Authorization": "Bearer your-api-key"}
)

history = response.json()
for debate in history['debates']:
    print(f"{debate['debateId']}: {debate['topic']}")
    print(f"  Quality Score: {debate['qualityScore']}")
    print(f"  Consensus: {'Yes' if debate['consensus'] else 'No'}")
```

### Provider Health Monitoring

```python
# Check provider health
response = requests.get(
    "http://localhost:7061/v1/providers",
    headers={"Authorization": "Bearer your-api-key"}
)

providers = response.json()['providers']
for provider in providers:
    print(f"{provider['name']}: {provider['status']}")
    print(f"  Success Rate: {provider['capabilities']['successRate']:.2%}")
    print(f"  Last Health Check: {provider['lastHealthCheck']}")
```

### Custom Metrics

```python
# Create custom metrics tracking
import time
from datetime import datetime

def track_debate_metrics(debate_id):
    start_time = time.time()
    
    # Monitor debate progress
    while True:
        response = requests.get(
            f"http://localhost:7061/v1/debates/{debate_id}/status",
            headers={"Authorization": "Bearer your-api-key"}
        )
        
        status = response.json()
        elapsed = time.time() - start_time
        
        print(f"[{datetime.now().isoformat()}] Round {status['currentRound']}/{status['totalRounds']} - {elapsed:.1f}s")
        
        if status['status'] == 'completed':
            break
        
        time.sleep(10)
    
    # Get final metrics
    results_response = requests.get(
        f"http://localhost:7061/v1/debates/{debate_id}/results",
        headers={"Authorization": "Bearer your-api-key"}
    )
    
    results = results_response.json()
    print(f"Total Duration: {results['duration']}s")
    print(f"Quality Score: {results['qualityScore']}")
    print(f"Consensus Achieved: {results['consensus']['achieved']}")

track_debate_metrics("your-debate-id")
```

---

## Best Practices

### 1. Debate Configuration

```yaml
# Optimal configuration for different scenarios
basic_debate:
  maxRounds: 3
  timeout: 600
  strategy: "structured"
  consensusThreshold: 0.75

advanced_debate:
  maxRounds: 5
  timeout: 900
  strategy: "socratic"
  consensusThreshold: 0.85
  enableCognee: true

expert_debate:
  maxRounds: 7
  timeout: 1200
  strategy: "adversarial"
  consensusThreshold: 0.9
  enableCognee: true
```

### 2. Provider Selection

```python
# Choose providers based on topic
def select_providers(topic):
    if "code" in topic.lower():
        return ["deepseek", "claude"]  # Strong coding capabilities
    elif "science" in topic.lower():
        return ["claude", "gemini"]    # Strong analytical capabilities
    elif "policy" in topic.lower():
        return ["claude", "qwen"]      # Strong reasoning capabilities
    else:
        return ["claude", "deepseek", "gemini"]  # Balanced approach

providers = select_providers("Machine learning ethics")
```

### 3. Timeout Configuration

```python
def calculate_timeout(topic_complexity, participant_count):
    base_timeout = 300  # 5 minutes
    
    # Adjust based on complexity
    if "expert" in topic_complexity.lower():
        base_timeout += 300
    elif "advanced" in topic_complexity.lower():
        base_timeout += 150
    
    # Adjust based on participant count
    timeout = base_timeout + (participant_count - 2) * 60
    
    return min(timeout, 1800)  # Max 30 minutes

timeout = calculate_timeout("Expert level AI ethics", 4)
```

### 4. Error Handling

```python
def safe_debate_operation(operation, max_retries=3):
    for attempt in range(max_retries):
        try:
            return operation()
        except requests.exceptions.RequestException as e:
            print(f"Attempt {attempt + 1} failed: {e}")
            if attempt < max_retries - 1:
                time.sleep(2 ** attempt)  # Exponential backoff
            else:
                raise

def create_debate_safe(config):
    return safe_debate_operation(
        lambda: requests.post(
            "http://localhost:7061/v1/debates",
            headers={"Authorization": "Bearer your-api-key"},
            json=config
        ).json()
    )
```

---

## Troubleshooting

### Common Issues

#### 1. Provider Authentication Errors

**Issue**: `PROVIDER_ERROR` - Authentication failed

**Solution**:
```python
# Verify API keys
import os

required_keys = [
    'CLAUDE_API_KEY',
    'DEEPSEEK_API_KEY',
    'GEMINI_API_KEY',
    'QWEN_API_KEY',
    'ZAI_API_KEY'
]

missing_keys = [key for key in required_keys if not os.getenv(key)]
if missing_keys:
    print(f"Missing API keys: {missing_keys}")
```

#### 2. Timeout Issues

**Issue**: Debate times out frequently

**Solution**:
```python
# Increase timeouts for complex topics
def adjust_timeouts(topic_length, complexity):
    base_timeout = 600
    
    # Adjust based on topic length
    if len(topic_length) > 100:
        base_timeout += 300
    
    # Adjust based on complexity
    complexity_multiplier = {"basic": 1.0, "advanced": 1.5, "expert": 2.0}
    multiplier = complexity_multiplier.get(complexity, 1.0)
    
    return int(base_timeout * multiplier)
```

#### 3. Consensus Not Reaching

**Issue**: Consensus threshold too high

**Solution**:
```python
def dynamic_consensus_threshold(round_number, total_rounds):
    # Start high, gradually decrease
    initial_threshold = 0.8
    final_threshold = 0.6
    
    # Linear decrease
    decrease_per_round = (initial_threshold - final_threshold) / total_rounds
    current_threshold = initial_threshold - (decrease_per_round * round_number)
    
    return max(current_threshold, final_threshold)
```

#### 4. Provider Rate Limits

**Issue**: Rate limit exceeded errors

**Solution**:
```python
import time
from collections import defaultdict

class RateLimiter:
    def __init__(self, max_requests=100, window_seconds=3600):
        self.max_requests = max_requests
        self.window_seconds = window_seconds
        self.requests = defaultdict(list)
    
    def is_allowed(self, provider):
        now = time.time()
        # Remove old requests
        self.requests[provider] = [
            req_time for req_time in self.requests[provider]
            if now - req_time < self.window_seconds
        ]
        
        # Check if under limit
        if len(self.requests[provider]) < self.max_requests:
            self.requests[provider].append(now)
            return True
        
        return False

rate_limiter = RateLimiter(max_requests=50, window_seconds=3600)
```

### Debugging Tools

#### 1. Comprehensive Logging

```python
import logging
import json

logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger(__name__)

def log_debate_request(request_data):
    logger.info(f"Debate Request: {json.dumps(request_data, indent=2)}")

def log_debate_response(response_data):
    logger.info(f"Debate Response: {json.dumps(response_data, indent=2)}")

def log_provider_metrics(metrics):
    logger.info(f"Provider Metrics: {json.dumps(metrics, indent=2)}")
```

#### 2. Request Tracing

```python
import uuid

def trace_debate_operation(operation_name, debate_id=None):
    trace_id = str(uuid.uuid4())
    
    def traced_operation(*args, **kwargs):
        print(f"[{trace_id}] Starting {operation_name}")
        if debate_id:
            print(f"[{trace_id}] Debate ID: {debate_id}")
        
        try:
            result = operation(*args, **kwargs)
            print(f"[{trace_id}] Completed {operation_name}")
            return result
        except Exception as e:
            print(f"[{trace_id}] Failed {operation_name}: {e}")
            raise
    
    return traced_operation
```

#### 3. Health Checking

```python
def comprehensive_health_check():
    health_status = {}
    
    # Check providers
    providers_response = requests.get(
        "http://localhost:7061/v1/providers",
        headers={"Authorization": "Bearer your-api-key"}
    )
    
    if providers_response.status_code == 200:
        providers = providers_response.json()['providers']
        health_status['providers'] = {
            p['providerId']: p['status'] for p in providers
        }
    
    # Check system metrics
    metrics_response = requests.get(
        "http://localhost:7061/v1/metrics",
        headers={"Authorization": "Bearer your-api-key"}
    )
    
    if metrics_response.status_code == 200:
        health_status['metrics'] = metrics_response.json()
    
    # Check database (if configured)
    try:
        response = requests.get(
            "http://localhost:7061/v1/health",
            headers={"Authorization": "Bearer your-api-key"}
        )
        health_status['system'] = response.json()
    except:
        health_status['system'] = {"status": "unknown"}
    
    return health_status

# Run health check
health = comprehensive_health_check()
print(json.dumps(health, indent=2))
```

---

## FAQ

### Q: How many participants can I have in a debate?

**A**: Between 2 and 10 participants. For best results, use 2-4 participants.

### Q: What's the maximum debate duration?

**A**: 1 hour (3600 seconds) per debate, with individual participant timeouts up to 10 minutes.

### Q: Can I use multiple providers in the same debate?

**A**: Yes! Mix and match providers to get diverse perspectives and capabilities.

### Q: How do I know if consensus was reached?

**A**: Check the `consensus.achieved` field in the debate results. If `true`, review `consensus.confidence` and `consensus.finalPosition`.

### Q: What happens if a provider fails?

**A**: The system automatically retries and falls back to other providers. Check the debate status for error details.

### Q: How can I improve debate quality?

**A**:
1. Enable Cognee AI enhancement
2. Use appropriate strategies (structured, socratic, etc.)
3. Set reasonable consensus thresholds
4. Choose providers based on topic expertise
5. Allow sufficient time for responses

### Q: Is there a rate limit?

**A**: Yes, 1000 requests per hour for authenticated users, 50 debate creations per hour.

### Q: Can I export debate results?

**A**: Yes, use the `/debates/{id}/report` endpoint with format options (json, pdf, html).

### Q: How do I monitor system health?

**A**: Use the `/providers` and `/metrics` endpoints for real-time monitoring.

---

## Next Steps

1. **Try the Examples**: Start with basic debates and gradually explore advanced features
2. **Configure Providers**: Set up your preferred LLM providers with API keys
3. **Enable Monitoring**: Set up metrics and monitoring for production use
4. **Explore Advanced Features**: Try Cognee enhancement, custom strategies, and analytics
5. **Scale Up**: Configure for high-volume usage with proper timeouts and limits

## Support

- **Documentation**: https://docs.helixagent.ai
- **API Reference**: `/docs/api`
- **Examples**: `/docs/examples`
- **Community**: https://community.helixagent.ai
- **Support**: support@helixagent.ai

---

*Last Updated: January 2026*
*Version: 1.0.0*